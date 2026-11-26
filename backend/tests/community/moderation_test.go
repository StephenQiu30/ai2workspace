package community_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/internal/community"
	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/auth"
	moderationhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/moderation"
	notificationhttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/notification"
	postshttp "github.com/StephenQiu30/ai2workspace/backend/internal/community/adapter/http/posts"
	moderationapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/moderation"
	postsapp "github.com/StephenQiu30/ai2workspace/backend/internal/community/application/posts"
	moderationdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/moderation"
	postsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/community/domain/posts"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

func TestModerationAdminAuthHidingAndNotifications(t *testing.T) {
	env := setupTestEnv()

	// 1. Alice creates a violating post
	w := env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", postshttp.CreatePostRequest{
		Scope:         "global",
		Topic:         "general",
		Title:         "违规推广帖",
		PublicSummary: "表面普通的公开摘要",
		Body:          "SENSITIVE_VIOLATION_BODY_NEVER_LEAK",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create post: %d %s", w.Code, w.Body.String())
	}
	var p postshttp.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p)

	// 2. Report validation: invalid targetType -> 400, nonexistent target -> 404
	if w := env.doRequest(http.MethodPost, "/api/community/reports", "sess_bob", moderationhttp.CreateReportRequest{
		TargetType: "course", TargetID: p.ID, Reason: "invalid",
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid report targetType, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodPost, "/api/community/reports", "sess_bob", moderationhttp.CreateReportRequest{
		TargetType: "post", TargetID: "post_nonexistent", Reason: "missing",
	}); w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent report target, got %d", w.Code)
	}

	// 3. Bob submits a valid report; duplicate open report returns same report ID
	w = env.doRequest(http.MethodPost, "/api/community/reports", "sess_bob", moderationhttp.CreateReportRequest{
		TargetType: "post", TargetID: p.ID, Reason: "广告垃圾信息与违规内容",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating report, got %d: %s", w.Code, w.Body.String())
	}
	var rep moderationdomain.Report
	_ = json.Unmarshal(w.Body.Bytes(), &rep)

	wDup := env.doRequest(http.MethodPost, "/api/community/reports", "sess_bob", moderationhttp.CreateReportRequest{
		TargetType: "post", TargetID: p.ID, Reason: "重复提交",
	})
	var repDup moderationdomain.Report
	_ = json.Unmarshal(wDup.Body.Bytes(), &repDup)
	if repDup.ID != rep.ID {
		t.Fatalf("expected deduplicated pending report ID %s, got %s", rep.ID, repDup.ID)
	}

	// 4. Non-admin users & guests cannot list or review reports
	if w := env.doRequest(http.MethodGet, "/api/community/reports", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("guest listing reports must get 401, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodGet, "/api/community/reports", "sess_bob", nil); w.Code != http.StatusForbidden {
		t.Fatalf("student listing reports must get 403, got %d", w.Code)
	}
	if w := env.doRequest(http.MethodPost, "/api/community/reports/"+rep.ID+"/review", "sess_bob", moderationhttp.ModerateReportRequest{
		Decision: "approved", Reason: "越权处置",
	}); w.Code != http.StatusForbidden {
		t.Fatalf("student reviewing report must get 403, got %d", w.Code)
	}

	// 5. Admin submitting invalid decision must receive 400
	if w := env.doRequest(http.MethodPost, "/api/community/reports/"+rep.ID+"/review", "sess_admin", moderationhttp.ModerateReportRequest{
		Decision: "maybe_later", Reason: "无效状态",
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid moderation decision, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Admin approves report -> hides post and sends safe notifications to reporter & author
	w = env.doRequest(http.MethodPost, "/api/community/reports/"+rep.ID+"/review", "sess_admin", moderationhttp.ModerateReportRequest{
		Decision: "approved", Reason: "核实属违规推广，予以隐藏",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin approval, got %d: %s", w.Code, w.Body.String())
	}
	var reviewed moderationdomain.Report
	_ = json.Unmarshal(w.Body.Bytes(), &reviewed)
	if reviewed.Status != moderationdomain.StatusApproved || reviewed.ReviewerID != "usr_admin" {
		t.Fatalf("unexpected reviewed report: %+v", reviewed)
	}

	// Double review must return 409 Conflict
	if w := env.doRequest(http.MethodPost, "/api/community/reports/"+rep.ID+"/review", "sess_admin", moderationhttp.ModerateReportRequest{
		Decision: "rejected",
	}); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for re-reviewing resolved report, got %d", w.Code)
	}

	// 7. Directly querying hidden post returns 410 with status=hidden, message="内容已隐藏", zero leaked content
	w = env.doRequest(http.MethodGet, "/api/community/posts/"+p.ID, "sess_alice", nil)
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410 for hidden post, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "SENSITIVE_VIOLATION_BODY_NEVER_LEAK") || strings.Contains(w.Body.String(), "违规推广帖") {
		t.Fatalf("hidden post response leaked content: %s", w.Body.String())
	}
	var hiddenResp postshttp.PostUnavailableResponse
	_ = json.Unmarshal(w.Body.Bytes(), &hiddenResp)
	if hiddenResp.Status != "hidden" || hiddenResp.Message != "内容已隐藏" {
		t.Fatalf("unexpected hidden post response: %+v", hiddenResp)
	}

	// 8. Both reporter (Bob) and content author (Alice) received moderation notifications,
	//    with targetAccessible=false and zero hidden body leakage
	for _, sess := range []string{"sess_bob", "sess_alice"} {
		w = env.doRequest(http.MethodGet, "/api/community/notifications", sess, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 listing notifications for %s, got %d", sess, w.Code)
		}
		if strings.Contains(w.Body.String(), "SENSITIVE_VIOLATION_BODY_NEVER_LEAK") {
			t.Fatalf("notification leaked hidden body for %s: %s", sess, w.Body.String())
		}
		var nResp notificationhttp.NotificationListResponse
		_ = json.Unmarshal(w.Body.Bytes(), &nResp)
		if nResp.Total != 1 || nResp.Items[0].Type != "moderation" || nResp.Items[0].TargetAccessible {
			t.Fatalf("expected 1 moderation notification with targetAccessible=false for %s, got %+v", sess, nResp)
		}
	}
}

type failingHidePostStore struct {
	*postsapp.MemoryStore
}

func (f *failingHidePostStore) HidePost(ctx context.Context, postID string) error {
	return errors.New("db write failure while hiding post")
}

func TestModerationDoesNotSwallowHideError(t *testing.T) {
	baseStore := postsapp.NewMemoryStore()
	postStore := &failingHidePostStore{MemoryStore: baseStore}
	modStore := moderationapp.NewMemoryStore()

	p, err := baseStore.CreatePost(context.Background(), postsdomain.CreatePostInput{
		Scope:          postsdomain.ScopeGlobal,
		Topic:          "general",
		AuthorID:       "usr_alice",
		AuthorNickname: "爱丽丝",
		Title:          "待隐藏帖子",
		PublicSummary:  "摘要",
		Body:           "正文",
		Status:         postsdomain.StatusPublished,
	})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := modStore.CreateReport(context.Background(), "usr_bob", "post", p.ID, "spam")
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	community.NewWithDeps(community.Deps{
		PostStore:       postStore,
		ModerationStore: modStore,
		SessionResolver: func(ctx context.Context, token string) (*authhttp.Actor, error) {
			if token == "sess_admin" {
				return &authhttp.Actor{UserID: "usr_admin", EmailVerified: true, Roles: []string{"admin"}, AccountStatus: "active"}, nil
			}
			return nil, authhttp.ErrUnauthenticated
		},
	}).RegisterRoutes(router)

	raw, _ := json.Marshal(moderationhttp.ModerateReportRequest{Decision: "approved", Reason: "hide"})
	req := httptest.NewRequest(http.MethodPost, "/api/community/reports/"+rep.ID+"/review", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: "sess_admin"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when HidePost fails, got %d: %s", w.Code, w.Body.String())
	}

	// Report must remain pending when HidePost failed
	stillPending, _ := modStore.GetReport(context.Background(), rep.ID)
	if stillPending.Status != moderationdomain.StatusPending {
		t.Fatalf("report must remain pending after HidePost error, got %s", stillPending.Status)
	}
}

func TestNotificationDeduplicationAndIsolation(t *testing.T) {
	env := setupTestEnv()
	store := env.notifStore
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := store.AppendNotification(t.Context(), "payment-event", "student1", "payment", "Paid", "Payment received", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := store.ListNotifications(t.Context(), "student1", 1, 20)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("expected one notification after repeat delivery, got %d, %v", total, err)
	}
	_, total, err = store.ListNotifications(t.Context(), "student2", 1, 20)
	if err != nil || total != 0 {
		t.Fatalf("expected no notifications for another recipient, got %d, %v", total, err)
	}
}

func TestCommentModerationAndReportListPagination(t *testing.T) {
	env := setupTestEnv()

	w := env.doRequest(http.MethodPost, "/api/community/posts", "sess_alice", postshttp.CreatePostRequest{
		Scope:         "global",
		Topic:         "general",
		Title:         "正常讨论帖",
		PublicSummary: "公开摘要",
		Body:          "正常正文",
	})
	var p postshttp.PostResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p)

	w = env.doRequest(http.MethodPost, "/api/community/posts/"+p.ID+"/comments", "sess_bob", postshttp.CreateCommentRequest{
		Body: "BAD_COMMENT_CONTENT_TO_HIDE",
	})
	var cmt postshttp.CommentResponse
	_ = json.Unmarshal(w.Body.Bytes(), &cmt)

	// Create 2 reports (post first, then comment) to verify reverse chronological pagination
	w1 := env.doRequest(http.MethodPost, "/api/community/reports", "sess_bob", moderationhttp.CreateReportRequest{
		TargetType: "post", TargetID: p.ID, Reason: "first report",
	})
	var r1 moderationdomain.Report
	_ = json.Unmarshal(w1.Body.Bytes(), &r1)

	w2 := env.doRequest(http.MethodPost, "/api/community/reports", "sess_alice", moderationhttp.CreateReportRequest{
		TargetType: "comment", TargetID: cmt.ID, Reason: "second report",
	})
	var r2 moderationdomain.Report
	_ = json.Unmarshal(w2.Body.Bytes(), &r2)

	// Admin lists reports page 1 size 1 -> newest (r2) first
	wList1 := env.doRequest(http.MethodGet, "/api/community/reports?page=1&pageSize=1", "sess_admin", nil)
	var list1 moderationhttp.ReportListResponse
	_ = json.Unmarshal(wList1.Body.Bytes(), &list1)
	if list1.Total != 2 || len(list1.Items) != 1 || list1.Items[0].ID != r2.ID {
		t.Fatalf("expected newest report %s on page 1, got %+v", r2.ID, list1)
	}

	wList2 := env.doRequest(http.MethodGet, "/api/community/reports?page=2&pageSize=1", "sess_admin", nil)
	var list2 moderationhttp.ReportListResponse
	_ = json.Unmarshal(wList2.Body.Bytes(), &list2)
	if len(list2.Items) != 1 || list2.Items[0].ID != r1.ID {
		t.Fatalf("expected older report %s on page 2, got %+v", r1.ID, list2)
	}

	// Admin approves comment report -> hides comment, excludes it from post detail
	wRev := env.doRequest(http.MethodPost, "/api/community/reports/"+r2.ID+"/review", "sess_admin", moderationhttp.ModerateReportRequest{
		Decision: "approved", Reason: "违规评论隐藏",
	})
	if wRev.Code != http.StatusOK {
		t.Fatalf("expected 200 approving comment report, got %d: %s", wRev.Code, wRev.Body.String())
	}

	wPost := env.doRequest(http.MethodGet, "/api/community/posts/"+p.ID, "sess_alice", nil)
	if strings.Contains(wPost.Body.String(), "BAD_COMMENT_CONTENT_TO_HIDE") {
		t.Fatalf("hidden comment leaked in GetPost: %s", wPost.Body.String())
	}

	// Filter approved reports returns only r2
	wApproved := env.doRequest(http.MethodGet, "/api/community/reports?status=approved", "sess_admin", nil)
	var approvedList moderationhttp.ReportListResponse
	_ = json.Unmarshal(wApproved.Body.Bytes(), &approvedList)
	if approvedList.Total != 1 || approvedList.Items[0].ID != r2.ID {
		t.Fatalf("expected 1 approved report %s, got %+v", r2.ID, approvedList)
	}
}
