package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/interaction"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/moderation"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/notification"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/server"
)

const testServiceToken = "test-community-service-token"

type testEnv struct {
	postStore     *posts.MemoryStore
	interactStore *interaction.MemoryStore
	modStore      *moderation.MemoryStore
	notifStore    *notification.MemoryStore
	router        http.Handler
	sessions      map[string]*auth.Actor
	courseAccess  map[string]bool // "courseID:userID" -> allowed
}

func setupTestEnv() *testEnv {
	env := &testEnv{
		postStore:     posts.NewMemoryStore(),
		interactStore: interaction.NewMemoryStore(),
		modStore:      moderation.NewMemoryStore(),
		notifStore:    notification.NewMemoryStore(),
		sessions: map[string]*auth.Actor{
			"sess_alice": {
				UserID:        "usr_alice",
				Nickname:      "爱丽丝",
				Email:         "alice@example.com",
				EmailVerified: true,
				Roles:         []string{"student"},
				AccountStatus: "active",
			},
			"sess_bob": {
				UserID:        "usr_bob",
				Nickname:      "鲍勃",
				Email:         "bob@example.com",
				EmailVerified: true,
				Roles:         []string{"student"},
				AccountStatus: "active",
			},
			"sess_unverified": {
				UserID:        "usr_unverified",
				Nickname:      "未验证用户",
				Email:         "unverified@example.com",
				EmailVerified: false,
				Roles:         []string{"student"},
				AccountStatus: "active",
			},
			"sess_suspended": {
				UserID:        "usr_suspended",
				Nickname:      "停用用户",
				Email:         "suspended@example.com",
				EmailVerified: true,
				Roles:         []string{"student"},
				AccountStatus: "suspended",
			},
			"sess_admin": {
				UserID:        "usr_admin",
				Nickname:      "社区管理员",
				Email:         "admin@example.com",
				EmailVerified: true,
				Roles:         []string{"student", "admin"},
				AccountStatus: "active",
			},
		},
		courseAccess: map[string]bool{
			"crs_101:usr_alice": true,
		},
	}

	env.router = server.NewWithDeps(server.Deps{
		SwaggerEnabled:    false,
		ServiceToken:      testServiceToken,
		PostStore:         env.postStore,
		InteractionStore:  env.interactStore,
		ModerationStore:   env.modStore,
		NotificationStore: env.notifStore,
		SessionResolver: func(ctx context.Context, opaqueSession string) (*auth.Actor, error) {
			actor, ok := env.sessions[opaqueSession]
			if !ok {
				return nil, auth.ErrUnauthenticated
			}
			cp := *actor
			return &cp, nil
		},
		CourseAccessChecker: func(ctx context.Context, courseID, userID string) (bool, error) {
			return env.courseAccess[courseID+":"+userID], nil
		},
	})
	return env
}

func (e *testEnv) doRequest(method, path, sessionCookie string, payload any) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		bodyReader = bytes.NewReader(raw)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func TestGuestAndUnverifiedWriteRejected(t *testing.T) {
	env := setupTestEnv()

	postPayload := posts.CreatePostRequest{
		Scope:         "global",
		Topic:         "general",
		Title:         "测试标题",
		PublicSummary: "独立公开摘要",
		Body:          "完整正文",
	}

	// Forged X-User-ID without cookie must still be rejected 401 (no mock fallback!)
	raw, _ := json.Marshal(postPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/community/posts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "forged_admin")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no session cookie provided, got %d: %s", w.Code, w.Body.String())
	}

	// Unverified email user must be rejected 403 on write operations
	w = env.doRequest(http.MethodPost, "/api/community/posts", "sess_unverified", postPayload)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unverified user CreatePost, got %d: %s", w.Code, w.Body.String())
	}

	// Suspended user must be rejected 403
	w = env.doRequest(http.MethodPost, "/api/community/posts", "sess_suspended", postPayload)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for suspended user CreatePost, got %d: %s", w.Code, w.Body.String())
	}

	// Create a valid post as verified user to test comment/reaction/favorite/report write gates
	w = env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", postPayload)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for verified user CreatePost, got %d: %s", w.Code, w.Body.String())
	}
	var created posts.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// Guest and unverified cannot comment, react, favorite, or report
	checks := []struct {
		name    string
		method  string
		path    string
		payload any
	}{
		{"comment", http.MethodPost, "/api/community/posts/" + created.ID + "/comments", posts.CreateCommentRequest{Body: "hi"}},
		{"reaction", http.MethodPut, "/api/community/reactions", interaction.SetReactionRequest{TargetType: "post", TargetID: created.ID, Value: "like"}},
		{"favorite", http.MethodPut, "/api/community/favorites", interaction.SetFavoriteRequest{PostID: created.ID, Saved: true}},
		{"report", http.MethodPost, "/api/community/reports", moderation.CreateReportRequest{TargetType: "post", TargetID: created.ID, Reason: "spam"}},
	}
	for _, tc := range checks {
		if w := env.doRequest(tc.method, tc.path, "", tc.payload); w.Code != http.StatusUnauthorized {
			t.Errorf("guest %s expected 401, got %d: %s", tc.name, w.Code, w.Body.String())
		}
		if w := env.doRequest(tc.method, tc.path, "sess_unverified", tc.payload); w.Code != http.StatusForbidden {
			t.Errorf("unverified %s expected 403, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}
}

func TestIndependentPublicSummaryAndGuestVisibilityBoundary(t *testing.T) {
	env := setupTestEnv()

	// 1. Publishing without explicit publicSummary must fail (never auto-slice body!)
	w := env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", posts.CreatePostRequest{
		Scope:         "global",
		Topic:         "learning-method",
		Title:         "缺少公开摘要的帖子",
		PublicSummary: "   ",
		Body:          "这段受保护正文绝不能被自动截取为公开摘要。",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when publicSummary is empty on published post, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Draft may omit publicSummary, still never copying body, and is invisible to others
	w = env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", posts.CreatePostRequest{
		Scope:  "global",
		Topic:  "learning-method",
		Title:  "草稿帖子",
		Body:   "草稿私密正文",
		Status: "draft",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for draft post, got %d: %s", w.Code, w.Body.String())
	}
	var draft posts.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &draft)
	if draft.PublicSummary != "" {
		t.Fatalf("draft publicSummary must stay empty and never copy body, got %q", draft.PublicSummary)
	}
	if w := env.doRequest(http.MethodGet, "/api/community/posts/"+draft.ID, "sess_bob", nil); w.Code != http.StatusNotFound {
		t.Fatalf("non-author should not view draft, got %d", w.Code)
	}

	// 3. Create published post with independent summary and add a comment
	w = env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", posts.CreatePostRequest{
		Scope:         "global",
		Topic:         "learning-method",
		Title:         "独立摘要验证帖",
		PublicSummary: "这是人工维护的公开摘要。",
		Body:          "这是仅登录后可见的受保护正文 SECRET_BODY_TOKEN。",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var pub posts.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &pub)

	w = env.doRequest(http.MethodPost, "/api/community/posts/"+pub.ID+"/comments", "sess_bob", posts.CreateCommentRequest{
		Body: "SECRET_COMMENT_TOKEN",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for comment, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Guest list must never contain body field or secret text
	w = env.doRequest(http.MethodGet, "/api/community/posts", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for guest list, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "SECRET_BODY_TOKEN") || strings.Contains(w.Body.String(), `"body"`) {
		t.Fatalf("list response leaked body: %s", w.Body.String())
	}

	// 5. Guest detail gets public fields + accessState=login_required, empty body & comments
	w = env.doRequest(http.MethodGet, "/api/community/posts/"+pub.ID, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for guest detail, got %d", w.Code)
	}
	var guestDetail posts.PostDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &guestDetail)
	if guestDetail.AccessState != string(posts.AccessLoginRequired) {
		t.Fatalf("expected accessState=login_required, got %q", guestDetail.AccessState)
	}
	if guestDetail.Body != "" || len(guestDetail.Comments) != 0 {
		t.Fatalf("guest detail must not expose body or comments: %+v", guestDetail)
	}
	if guestDetail.PublicSummary != "这是人工维护的公开摘要。" || guestDetail.AuthorNickname != "爱丽丝" {
		t.Fatalf("guest detail missing public metadata: %+v", guestDetail)
	}

	// 6. Authenticated viewer gets full body & comments
	w = env.doRequest(http.MethodGet, "/api/community/posts/"+pub.ID, "sess_bob", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for logged-in detail, got %d", w.Code)
	}
	var authedDetail posts.PostDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &authedDetail)
	if authedDetail.AccessState != string(posts.AccessFull) || !strings.Contains(authedDetail.Body, "SECRET_BODY_TOKEN") || len(authedDetail.Comments) != 1 {
		t.Fatalf("authenticated viewer should see full body & comments: %+v", authedDetail)
	}
}

func TestCourseDiscussionAccessControl(t *testing.T) {
	env := setupTestEnv()

	// Bob lacks access to crs_101 -> cannot create course post
	w := env.doRequest(http.MethodPost, "/api/community/posts", "sess_bob", posts.CreatePostRequest{
		Scope:         "course",
		CourseID:      "crs_101",
		Topic:         "course-qa",
		Title:         "未购学员尝试发帖",
		PublicSummary: "摘要",
		Body:          "正文",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized course post creation, got %d", w.Code)
	}

	// Alice has access to crs_101 -> creates course post
	w = env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", posts.CreatePostRequest{
		Scope:         "course",
		CourseID:      "crs_101",
		Topic:         "course-qa",
		Title:         "课程讨论：第3章问题",
		PublicSummary: "第3章作业思路讨论。",
		Body:          "COURSE_ONLY_BODY_CONTENT",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for enrolled student course post, got %d: %s", w.Code, w.Body.String())
	}
	var coursePost posts.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &coursePost)

	// Guest and Bob (no access) MUST receive 403 on course discussion list
	if w := env.doRequest(http.MethodGet, "/api/community/posts?scope=course&courseId=crs_101", "", nil); w.Code != http.StatusForbidden {
		t.Fatalf("guest listing course posts must get 403, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodGet, "/api/community/posts?scope=course&courseId=crs_101", "sess_bob", nil); w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized user listing course posts must get 403, got %d", w.Code)
	}

	// Alice can list course posts
	w = env.doRequest(http.MethodGet, "/api/community/posts?scope=course&courseId=crs_101", "sess_alice", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("enrolled user listing course posts expected 200, got %d", w.Code)
	}
	var listResp posts.PostListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 course post, got %+v", listResp)
	}

	// Bob viewing course post detail gets accessState=course_access_required with empty body & comments
	w = env.doRequest(http.MethodGet, "/api/community/posts/"+coursePost.ID, "sess_bob", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for course post detail with access gate, got %d", w.Code)
	}
	var gated posts.PostDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &gated)
	if gated.AccessState != string(posts.AccessCourseAccessRequired) || gated.Body != "" || len(gated.Comments) != 0 {
		t.Fatalf("unauthorized viewer must receive course_access_required without body/comments: %+v", gated)
	}

	// Bob cannot comment, react, or bookmark the course post
	if w := env.doRequest(http.MethodPost, "/api/community/posts/"+coursePost.ID+"/comments", "sess_bob", posts.CreateCommentRequest{Body: "sneak"}); w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized comment on course post expected 403, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodPut, "/api/community/reactions", "sess_bob", interaction.SetReactionRequest{TargetType: "post", TargetID: coursePost.ID, Value: "like"}); w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized reaction on course post expected 403, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodPut, "/api/community/favorites", "sess_bob", interaction.SetFavoriteRequest{PostID: coursePost.ID, Saved: true}); w.Code != http.StatusForbidden {
		t.Fatalf("unauthorized favorite on course post expected 403, got %d", w.Code)
	}
}

func TestChronologicalPaginationAndTopics(t *testing.T) {
	env := setupTestEnv()

	topics := []string{"learning-method", "project-practice", "learning-method", "resources", "general"}
	createdIDs := make([]string, 0, len(topics))
	for i, topic := range topics {
		w := env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", posts.CreatePostRequest{
			Scope:         "global",
			Topic:         topic,
			Title:         fmt.Sprintf("帖子-%d", i+1),
			PublicSummary: fmt.Sprintf("公开摘要-%d", i+1),
			Body:          fmt.Sprintf("正文-%d", i+1),
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("failed creating post %d: %d %s", i, w.Code, w.Body.String())
		}
		var resp posts.PostResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		createdIDs = append(createdIDs, resp.ID)
	}

	// Reverse chronological order: newest first (createdIDs[4], [3], [2], [1], [0])
	for run := 0; run < 5; run++ {
		w1 := env.doRequest(http.MethodGet, "/api/community/posts?page=1&pageSize=2", "", nil)
		w2 := env.doRequest(http.MethodGet, "/api/community/posts?page=2&pageSize=2", "", nil)
		w3 := env.doRequest(http.MethodGet, "/api/community/posts?page=3&pageSize=2", "", nil)

		var p1, p2, p3 posts.PostListResponse
		_ = json.Unmarshal(w1.Body.Bytes(), &p1)
		_ = json.Unmarshal(w2.Body.Bytes(), &p2)
		_ = json.Unmarshal(w3.Body.Bytes(), &p3)

		if p1.Total != 5 || len(p1.Items) != 2 || len(p2.Items) != 2 || len(p3.Items) != 1 {
			t.Fatalf("unexpected page sizes: p1=%d p2=%d p3=%d", len(p1.Items), len(p2.Items), len(p3.Items))
		}
		got := []string{p1.Items[0].ID, p1.Items[1].ID, p2.Items[0].ID, p2.Items[1].ID, p3.Items[0].ID}
		want := []string{createdIDs[4], createdIDs[3], createdIDs[2], createdIDs[1], createdIDs[0]}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("run %d unstable order at %d: got %v, want %v", run, i, got, want)
			}
		}
	}

	// Filter by topic
	w := env.doRequest(http.MethodGet, "/api/community/posts?topic=learning-method", "", nil)
	var filtered posts.PostListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &filtered)
	if filtered.Total != 2 || filtered.Items[0].ID != createdIDs[2] || filtered.Items[1].ID != createdIDs[0] {
		t.Fatalf("unexpected topic filter result: %+v", filtered)
	}

	// Topic counts endpoint
	w = env.doRequest(http.MethodGet, "/api/community/topics", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/community/topics, got %d", w.Code)
	}
	var topicResp posts.TopicListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &topicResp)
	counts := make(map[string]int)
	for _, item := range topicResp.Items {
		counts[item.Slug] = item.PostCount
	}
	if counts["learning-method"] != 2 || counts["project-practice"] != 1 || counts["resources"] != 1 || counts["general"] != 1 || counts["course-qa"] != 0 {
		t.Fatalf("unexpected topic counts: %+v", counts)
	}
}

func TestCommentsUpdateOptimisticLockReplyNotificationsAndReactionsFavorites(t *testing.T) {
	env := setupTestEnv()

	// Alice creates a global post
	w := env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", posts.CreatePostRequest{
		Scope:         "global",
		Topic:         "general",
		Title:         "Alice 的讨论",
		PublicSummary: "关于评论与互动的测试帖。",
		Body:          "Alice 正文内容",
	})
	var p posts.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p)

	// Bob tries to edit Alice's post -> 403
	w = env.doRequest(http.MethodPatch, "/api/community/posts/"+p.ID, "sess_bob", posts.UpdatePostRequest{
		ExpectedVersion: 1,
		Title:           "恶意修改",
		PublicSummary:   "恶意摘要",
		Body:            "恶意正文",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-author post edit, got %d", w.Code)
	}

	// Alice edits with stale version -> 409
	w = env.doRequest(http.MethodPatch, "/api/community/posts/"+p.ID, "sess_alice", posts.UpdatePostRequest{
		ExpectedVersion: 99,
		Title:           "过期版本修改",
		PublicSummary:   "摘要",
		Body:            "正文",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for stale post version, got %d", w.Code)
	}

	// Bob adds top-level comment -> Alice receives reply notification
	w = env.doRequest(http.MethodPost, "/api/community/posts/"+p.ID+"/comments", "sess_bob", posts.CreateCommentRequest{
		Body: "第一条评论",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for comment, got %d: %s", w.Code, w.Body.String())
	}
	var c1 posts.CommentResponse
	_ = json.Unmarshal(w.Body.Bytes(), &c1)
	if c1.Version != 1 || c1.AuthorNickname != "鲍勃" {
		t.Fatalf("unexpected comment metadata: %+v", c1)
	}

	// Alice replies to Bob's comment -> Bob receives reply notification
	w = env.doRequest(http.MethodPost, "/api/community/posts/"+p.ID+"/comments", "sess_alice", posts.CreateCommentRequest{
		Body:            "回复鲍勃",
		ParentCommentID: c1.ID,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for single-level reply, got %d: %s", w.Code, w.Body.String())
	}
	var c2 posts.CommentResponse
	_ = json.Unmarshal(w.Body.Bytes(), &c2)

	// Nested reply to c2 -> 400
	w = env.doRequest(http.MethodPost, "/api/community/posts/"+p.ID+"/comments", "sess_bob", posts.CreateCommentRequest{
		Body:            "不允许二层回复",
		ParentCommentID: c2.ID,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 rejecting nested reply, got %d", w.Code)
	}

	// Check reply notifications delivered to Alice and Bob with targetType=post, targetId=p.ID
	w = env.doRequest(http.MethodGet, "/api/community/notifications", "sess_alice", nil)
	var aliceNotifs notification.NotificationListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &aliceNotifs)
	if aliceNotifs.Total != 1 || aliceNotifs.Items[0].TargetType != "post" || aliceNotifs.Items[0].TargetID != p.ID || !aliceNotifs.Items[0].TargetAccessible {
		t.Fatalf("expected reply notification for Alice, got %+v", aliceNotifs)
	}

	w = env.doRequest(http.MethodGet, "/api/community/notifications", "sess_bob", nil)
	var bobNotifs notification.NotificationListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &bobNotifs)
	if bobNotifs.Total != 1 || bobNotifs.Items[0].TargetType != "post" || bobNotifs.Items[0].TargetID != p.ID {
		t.Fatalf("expected reply notification for Bob, got %+v", bobNotifs)
	}

	// Comment update: Alice cannot edit Bob's comment (403); Bob with wrong version gets 409; Bob with version 1 succeeds
	if w := env.doRequest(http.MethodPatch, "/api/community/comments/"+c1.ID, "sess_alice", posts.UpdateCommentRequest{ExpectedVersion: 1, Body: "越权"}); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for editing another user's comment, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodPatch, "/api/community/comments/"+c1.ID, "sess_bob", posts.UpdateCommentRequest{ExpectedVersion: 2, Body: "冲突"}); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for comment version conflict, got %d", w.Code)
	}
	w = env.doRequest(http.MethodPatch, "/api/community/comments/"+c1.ID, "sess_bob", posts.UpdateCommentRequest{ExpectedVersion: 1, Body: "第一条评论（已修订）"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for comment update, got %d: %s", w.Code, w.Body.String())
	}
	var c1Updated posts.CommentResponse
	_ = json.Unmarshal(w.Body.Bytes(), &c1Updated)
	if c1Updated.Version != 2 || c1Updated.Body != "第一条评论（已修订）" {
		t.Fatalf("unexpected updated comment: %+v", c1Updated)
	}

	// Verify comments in GetPost are strictly ascending (c1 before c2)
	w = env.doRequest(http.MethodGet, "/api/community/posts/"+p.ID, "sess_alice", nil)
	var detail posts.PostDetailResponse
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if len(detail.Comments) != 2 || detail.Comments[0].ID != c1.ID || detail.Comments[1].ID != c2.ID {
		t.Fatalf("comments not sorted ascending: %+v", detail.Comments)
	}

	// Mutually exclusive reactions on BOTH post and comment
	for _, target := range []struct {
		tType string
		tID   string
	}{
		{"post", p.ID},
		{"comment", c1.ID},
	} {
		// Like twice -> idempotent
		w = env.doRequest(http.MethodPut, "/api/community/reactions", "sess_bob", interaction.SetReactionRequest{TargetType: target.tType, TargetID: target.tID, Value: "like"})
		if w.Code != http.StatusOK {
			t.Fatalf("like %s failed: %d %s", target.tType, w.Code, w.Body.String())
		}
		w = env.doRequest(http.MethodPut, "/api/community/reactions", "sess_bob", interaction.SetReactionRequest{TargetType: target.tType, TargetID: target.tID, Value: "like"})
		var sum interaction.ReactionSummary
		_ = json.Unmarshal(w.Body.Bytes(), &sum)
		if sum.Likes != 1 || sum.Dislikes != 0 || sum.MyReaction != interaction.ReactionLike {
			t.Fatalf("duplicate like failed for %s: %+v", target.tType, sum)
		}

		// Switch to dislike -> mutually exclusive
		w = env.doRequest(http.MethodPut, "/api/community/reactions", "sess_bob", interaction.SetReactionRequest{TargetType: target.tType, TargetID: target.tID, Value: "dislike"})
		_ = json.Unmarshal(w.Body.Bytes(), &sum)
		if sum.Likes != 0 || sum.Dislikes != 1 || sum.MyReaction != interaction.ReactionDislike {
			t.Fatalf("switch to dislike failed for %s: %+v", target.tType, sum)
		}

		// Cancel -> none
		w = env.doRequest(http.MethodPut, "/api/community/reactions", "sess_bob", interaction.SetReactionRequest{TargetType: target.tType, TargetID: target.tID, Value: "none"})
		_ = json.Unmarshal(w.Body.Bytes(), &sum)
		if sum.Likes != 0 || sum.Dislikes != 0 || sum.MyReaction != interaction.ReactionNone {
			t.Fatalf("cancel reaction failed for %s: %+v", target.tType, sum)
		}
	}

	// Invalid reaction value rejected 400
	if w := env.doRequest(http.MethodPut, "/api/community/reactions", "sess_bob", interaction.SetReactionRequest{TargetType: "post", TargetID: p.ID, Value: "love"}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid reaction value, got %d", w.Code)
	}

	// Post-only favorites: bookmarking a comment ID MUST be rejected 400 (ErrFavoritePostOnly)
	if w := env.doRequest(http.MethodPut, "/api/community/favorites", "sess_bob", interaction.SetFavoriteRequest{PostID: c1.ID, Saved: true}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 rejecting comment bookmark, got %d: %s", w.Code, w.Body.String())
	}
	if w := env.doRequest(http.MethodPut, "/api/community/favorites", "sess_bob", interaction.SetFavoriteRequest{TargetType: "comment", TargetID: c1.ID, Saved: true}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 rejecting comment targetType bookmark, got %d: %s", w.Code, w.Body.String())
	}
	// Non-existent post rejected 404
	if w := env.doRequest(http.MethodPut, "/api/community/favorites", "sess_bob", interaction.SetFavoriteRequest{PostID: "post_missing", Saved: true}); w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent post bookmark, got %d", w.Code)
	}

	// Bookmark valid post, query status, list, and unbookmark
	w = env.doRequest(http.MethodPut, "/api/community/favorites", "sess_bob", interaction.SetFavoriteRequest{PostID: p.ID, Saved: true})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 bookmarking post, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest(http.MethodGet, "/api/community/favorites/"+p.ID, "sess_bob", nil)
	var favStatus interaction.FavoriteResponse
	_ = json.Unmarshal(w.Body.Bytes(), &favStatus)
	if !favStatus.Saved || favStatus.PostID != p.ID {
		t.Fatalf("expected saved=true for bookmarked post, got %+v", favStatus)
	}
	w = env.doRequest(http.MethodGet, "/api/community/me/favorites", "sess_bob", nil)
	var favList []string
	_ = json.Unmarshal(w.Body.Bytes(), &favList)
	if len(favList) != 1 || favList[0] != p.ID {
		t.Fatalf("expected [%s] in my favorites, got %v", p.ID, favList)
	}

	w = env.doRequest(http.MethodPut, "/api/community/favorites", "sess_bob", interaction.SetFavoriteRequest{PostID: p.ID, Saved: false})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 unbookmarking post, got %d", w.Code)
	}
	w = env.doRequest(http.MethodGet, "/api/community/favorites/"+p.ID, "sess_bob", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &favStatus)
	if favStatus.Saved {
		t.Fatalf("expected saved=false after unbookmarking")
	}

	// Author deletes post -> GetPost returns 410 with status=deleted and message="作者已删除"
	if w := env.doRequest(http.MethodDelete, "/api/community/posts/"+p.ID, "sess_alice", nil); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting own post, got %d", w.Code)
	}
	w = env.doRequest(http.MethodGet, "/api/community/posts/"+p.ID, "sess_bob", nil)
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410 for deleted post, got %d: %s", w.Code, w.Body.String())
	}
	var unavail posts.PostUnavailableResponse
	_ = json.Unmarshal(w.Body.Bytes(), &unavail)
	if unavail.Status != "deleted" || unavail.Message != "作者已删除" {
		t.Fatalf("unexpected deleted post response: %+v", unavail)
	}

	// Earlier reply notification now dynamically reports targetAccessible=false with sanitized summary
	w = env.doRequest(http.MethodGet, "/api/community/notifications", "sess_alice", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &aliceNotifs)
	if len(aliceNotifs.Items) != 1 || aliceNotifs.Items[0].TargetAccessible || strings.Contains(aliceNotifs.Items[0].Summary, "Alice 的讨论") {
		t.Fatalf("deleted post notification must mark targetAccessible=false and sanitize summary: %+v", aliceNotifs.Items[0])
	}
}
