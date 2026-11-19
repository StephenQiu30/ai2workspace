package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/media"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/server"
)

func TestProgressIsolationAndVersion(t *testing.T) {
	coursesStore := courses.NewMemoryStore()
	mediaStore := media.NewMemoryStore(nil)
	progressStore := progress.NewMemoryStore()
	r := server.NewWithStores(false, coursesStore, mediaStore, progressStore, "test-secret")

	// 1. User A saves progress at 120s
	reqA, _ := json.Marshal(progress.SaveProgressRequest{
		CourseID:        "crs_1",
		ContentVersion:  1,
		PositionSeconds: 120,
		ExpectedVersion: 0,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/learning/lessons/lsn_1/progress", bytes.NewReader(reqA))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_A")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 saving progress for user A, got %d: %s", w.Code, w.Body.String())
	}

	var respA progress.ProgressResponse
	_ = json.Unmarshal(w.Body.Bytes(), &respA)
	if respA.PositionSeconds != 120 || respA.Version != 1 {
		t.Fatalf("unexpected progress for user A: %+v", respA)
	}

	// 2. User B saves progress at 45s on the same lesson
	reqB, _ := json.Marshal(progress.SaveProgressRequest{
		CourseID:        "crs_1",
		ContentVersion:  1,
		PositionSeconds: 45,
		ExpectedVersion: 0,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/learning/lessons/lsn_1/progress", bytes.NewReader(reqB))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_B")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 saving progress for user B, got %d", w.Code)
	}

	// 3. User A queries progress - should still be 120s (isolated from user B)
	req = httptest.NewRequest(http.MethodGet, "/api/learning/lessons/lsn_1/progress", nil)
	req.Header.Set("X-User-ID", "user_A")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var queryA progress.ProgressResponse
	_ = json.Unmarshal(w.Body.Bytes(), &queryA)
	if queryA.PositionSeconds != 120 {
		t.Fatalf("expected user A position 120, got %d", queryA.PositionSeconds)
	}

	// 4. Stale update with mismatched expectedVersion -> 409 Conflict
	conflictReq, _ := json.Marshal(progress.SaveProgressRequest{
		CourseID:        "crs_1",
		ContentVersion:  1,
		PositionSeconds: 150,
		ExpectedVersion: 99, // Mismatch (current version is 1)
	})
	req = httptest.NewRequest(http.MethodPut, "/api/learning/lessons/lsn_1/progress", bytes.NewReader(conflictReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_A")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for stale progress version, got %d", w.Code)
	}
}

func TestPlaybackAuthorization(t *testing.T) {
	coursesStore := courses.NewMemoryStore()
	mediaStore := media.NewMemoryStore(nil)
	progressStore := progress.NewMemoryStore()
	r := server.NewWithStores(false, coursesStore, mediaStore, progressStore, "test-secret")

	// Create course and lessons
	crs, _ := coursesStore.CreateCourse(nil, "inst_1", "Paid Course", "Desc", "", false)
	sec, _ := coursesStore.AddSection(nil, crs.ID, "Chapter 1", 1)
	trialLesson, _ := coursesStore.AddLesson(nil, crs.ID, sec.ID, "Trial Lesson", "video", true, "", "", 1)
	paidLesson, _ := coursesStore.AddLesson(nil, crs.ID, sec.ID, "Paid Lesson", "video", false, "", "", 2)

	// Attach ready media to both lessons
	u1, _ := mediaStore.CreateUpload(nil, "inst_1", crs.ID, trialLesson.ID, "trial.mp4", 1000)
	v1, _ := mediaStore.CompleteUpload(nil, u1.ID, "k1", media.MediaMetadata{
		Container: "mp4", Codec: "h264", DurationSeconds: 60, Width: 1280, Height: 720, FPS: 30, ActualBytes: 1000,
	})
	_ = mediaStore.PublishLessonMedia(nil, trialLesson.ID, v1.ID, "inst_1")

	u2, _ := mediaStore.CreateUpload(nil, "inst_1", crs.ID, paidLesson.ID, "paid.mp4", 1000)
	v2, _ := mediaStore.CompleteUpload(nil, u2.ID, "k2", media.MediaMetadata{
		Container: "mp4", Codec: "h264", DurationSeconds: 60, Width: 1280, Height: 720, FPS: 30, ActualBytes: 1000,
	})
	_ = mediaStore.PublishLessonMedia(nil, paidLesson.ID, v2.ID, "inst_1")

	// 1. Trial lesson request with purpose=trial succeeds for anonymous user
	trialAuth, _ := json.Marshal(media.AuthorizePlaybackRequest{
		CourseID: crs.ID,
		Purpose:  "trial",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/learning/lessons/"+trialLesson.ID+"/playback-authorizations", bytes.NewReader(trialAuth))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for trial lesson authorization, got %d: %s", w.Code, w.Body.String())
	}

	var authResp media.PlaybackAuthorizationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &authResp)
	if authResp.Token == "" || authResp.MediaVersionID != v1.ID {
		t.Fatalf("invalid playback token issued: %+v", authResp)
	}

	// 2. Paid lesson request for anonymous user without trial purpose fails with 403
	paidAuth, _ := json.Marshal(media.AuthorizePlaybackRequest{
		CourseID: crs.ID,
		Purpose:  "playback",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", bytes.NewReader(paidAuth))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "anonymous")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for paid lesson without entitlement, got %d", w.Code)
	}
}
