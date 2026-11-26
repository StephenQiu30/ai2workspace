package learning_test

import (
	"encoding/json"
	"net/http"
	"testing"

	progresshttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/progress"
)

func TestProgressAuthzIsolationAndStrictOptimisticLock(t *testing.T) {
	env := newTestEnv()

	c, _ := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "付费进度测试课", "", "", "engineering", false)
	sec, _ := env.coursesStore.AddSection(t.Context(), c.ID, "第一章", 1)
	l1, _ := env.coursesStore.AddLesson(t.Context(), c.ID, sec.ID, "课时 1", "video", false, "", "", 1)
	cur, _ := env.coursesStore.GetCourse(t.Context(), c.ID)
	sub, _ := env.coursesStore.SubmitCourse(t.Context(), c.ID, "instructor_1", false, cur.Version)
	_, _ = env.coursesStore.ReviewCourse(t.Context(), c.ID, "approved", "ok", sub.Version)

	// 1. Unauthenticated cannot read or write progress -> 401
	if w := doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 10, ExpectedVersion: 0,
	}, "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("guest save progress status = %d, want 401", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/lessons/"+l1.ID+"/progress", nil, "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("guest get progress status = %d, want 401", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+c.ID+"/progress", nil, "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("guest list progress status = %d, want 401", w.Code)
	}

	// 2. Unpurchased student cannot read or write progress -> 403
	if w := doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 10, ExpectedVersion: 0,
	}, sessStudent1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("unpurchased save progress status = %d, want 403", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/lessons/"+l1.ID+"/progress", nil, sessStudent1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("unpurchased get progress status = %d, want 403", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+c.ID+"/progress", nil, sessStudent1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("unpurchased list progress status = %d, want 403", w.Code)
	}

	// 3. Grant entitlement to both student_1 and student_2
	env.entitlements.Grant("student_1", c.ID)
	env.entitlements.Grant("student_2", c.ID)

	// Initial save with expectedVersion=0 succeeds -> version=1
	w := doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 120, ExpectedVersion: 0,
	}, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("student_1 initial save status = %d: %s", w.Code, w.Body.String())
	}
	var p1 progresshttp.ProgressResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p1)
	if p1.PositionSeconds != 120 || p1.Version != 1 {
		t.Fatalf("unexpected progress p1: %+v", p1)
	}

	// Student 2 saves independently at 45s
	w = doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 45, ExpectedVersion: 0,
	}, sessStudent2, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("student_2 save status = %d: %s", w.Code, w.Body.String())
	}

	// User isolation: student_1 still sees 120s
	w = doRequest(t, env.router, http.MethodGet, "/api/learning/lessons/"+l1.ID+"/progress", nil, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("student_1 get progress status = %d", w.Code)
	}
	var got1 progresshttp.ProgressResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got1)
	if got1.PositionSeconds != 120 {
		t.Fatalf("expected isolated position 120, got %d", got1.PositionSeconds)
	}

	// 4. CRITICAL: Passing expectedVersion=0 when record already exists MUST return 409 Conflict!
	w = doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 5, ExpectedVersion: 0,
	}, sessStudent1, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when passing expectedVersion=0 on existing progress, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Updating with matching expectedVersion=1 succeeds -> version=2
	w = doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 200, ExpectedVersion: 1,
	}, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 updating progress with matching version 1, got %d: %s", w.Code, w.Body.String())
	}
	var p2 progresshttp.ProgressResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p2)
	if p2.PositionSeconds != 200 || p2.Version != 2 {
		t.Fatalf("unexpected updated progress p2: %+v", p2)
	}

	// 6. Stale update with expectedVersion=1 -> 409 Conflict
	w = doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 150, ExpectedVersion: 1,
	}, sessStudent1, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 on stale version 1, got %d", w.Code)
	}

	// 7. ListCourseProgress returns user's records
	w = doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+c.ID+"/progress", nil, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list course progress status = %d", w.Code)
	}
	var list []progresshttp.ProgressResponse
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].PositionSeconds != 200 {
		t.Fatalf("unexpected course progress list: %+v", list)
	}

	// 8. After refund (entitlement revoked), student_1 can no longer read/write progress -> 403
	env.entitlements.Revoke("student_1", c.ID)
	if w := doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progresshttp.SaveProgressRequest{
		CourseID: c.ID, ContentVersion: 1, PositionSeconds: 250, ExpectedVersion: 2,
	}, sessStudent1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("revoked student save progress status = %d, want 403", w.Code)
	}
}
