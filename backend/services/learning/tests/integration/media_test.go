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
	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *courses.MemoryStore, *media.MemoryStore, *progress.MemoryStore) {
	coursesStore := courses.NewMemoryStore()
	mediaStore := media.NewMemoryStore(func(courseID, instructorID string) (bool, error) {
		return true, nil
	})
	progressStore := progress.NewMemoryStore()
	r := server.NewWithStores(false, coursesStore, mediaStore, progressStore, "test-secret")
	return r, coursesStore, mediaStore, progressStore
}

func TestUploadLimitsAndExpiry(t *testing.T) {
	_, coursesStore, mediaStore, progressStore := setupTestRouter()
	r := server.NewWithStores(false, coursesStore, mediaStore, progressStore, "test-secret")

	// 1. Over maximum declared size (> 2 GiB)
	reqBody, _ := json.Marshal(media.CreateUploadRequest{
		CourseID:      "crs_1",
		FileName:      "intro.mp4",
		DeclaredBytes: 2147483649, // 2 GiB + 1 byte
	})
	req := httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_1/uploads", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 Request Entity Too Large, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Invalid non-mp4 format
	reqBody, _ = json.Marshal(media.CreateUploadRequest{
		CourseID:      "crs_1",
		FileName:      "intro.avi",
		DeclaredBytes: 1024 * 1024 * 50,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_1/uploads", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_1")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-mp4 container, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Two concurrent sessions are allowed
	validReq, _ := json.Marshal(media.CreateUploadRequest{
		CourseID:      "crs_1",
		FileName:      "lesson1.mp4",
		DeclaredBytes: 1024 * 1024 * 10,
	})

	req = httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_1/uploads", bytes.NewReader(validReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_limit")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 1st upload created 201, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_2/uploads", bytes.NewReader(validReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_limit")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 2nd upload created 201, got %d: %s", w.Code, w.Body.String())
	}

	// 4. 3rd concurrent session must be rejected with 429
	req = httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_3/uploads", bytes.NewReader(validReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_limit")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 3rd upload rejected with 429 Too Many Requests, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadValidatesActualMedia(t *testing.T) {
	_, coursesStore, mediaStore, progressStore := setupTestRouter()
	r := server.NewWithStores(false, coursesStore, mediaStore, progressStore, "test-secret")

	// Create valid upload
	validReq, _ := json.Marshal(media.CreateUploadRequest{
		CourseID:      "crs_1",
		FileName:      "lesson_meta.mp4",
		DeclaredBytes: 1024 * 1024 * 20,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_meta/uploads", bytes.NewReader(validReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_meta")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var uploadResp media.UploadSessionResponse
	_ = json.Unmarshal(w.Body.Bytes(), &uploadResp)
	uploadID := uploadResp.ID

	// 1. Complete with non-H.264 codec (e.g. HEVC) -> rejected / failed
	compReq, _ := json.Marshal(media.CompleteUploadRequest{
		IdempotencyKey:  "key-1",
		Container:       "mp4",
		Codec:           "hevc",
		DurationSeconds: 600,
		Width:           1920,
		Height:          1080,
		FPS:             30,
		ActualBytes:     1024 * 1024 * 20,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/learning/uploads/"+uploadID+"/completion", bytes.NewReader(compReq))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid codec HEVC, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Complete with excessive resolution (>1920 long side) -> rejected / failed
	compReq, _ = json.Marshal(media.CompleteUploadRequest{
		IdempotencyKey:  "key-2",
		Container:       "mp4",
		Codec:           "h264",
		DurationSeconds: 600,
		Width:           3840,
		Height:          2160,
		FPS:             30,
		ActualBytes:     1024 * 1024 * 20,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/learning/uploads/"+uploadID+"/completion", bytes.NewReader(compReq))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for 4K resolution, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Complete with valid metadata -> 200 OK and ready
	compReq, _ = json.Marshal(media.CompleteUploadRequest{
		IdempotencyKey:  "key-valid",
		Container:       "mp4",
		Codec:           "h264",
		DurationSeconds: 1800, // 30 minutes
		Width:           1920,
		Height:          1080,
		FPS:             30,
		ActualBytes:     1024 * 1024 * 20,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/learning/uploads/"+uploadID+"/completion", bytes.NewReader(compReq))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid MP4 H.264, got %d: %s", w.Code, w.Body.String())
	}

	var verResp media.MediaVersionResponse
	_ = json.Unmarshal(w.Body.Bytes(), &verResp)
	if verResp.Status != "ready" {
		t.Fatalf("expected media version ready, got %s", verResp.Status)
	}
	if len(verResp.HLSQualities) != 2 || verResp.HLSQualities[1] != "1080p" {
		t.Fatalf("expected 720p and 1080p qualities, got %v", verResp.HLSQualities)
	}
}

func TestMediaVersionPublication(t *testing.T) {
	_, coursesStore, mediaStore, progressStore := setupTestRouter()
	r := server.NewWithStores(false, coursesStore, mediaStore, progressStore, "test-secret")

	// 1. Direct upload session and ready media
	session, err := mediaStore.CreateUpload(nil, "inst_pub", "crs_1", "lsn_pub", "valid.mp4", 1000)
	if err != nil {
		t.Fatalf("failed to create upload: %v", err)
	}

	readyVer, err := mediaStore.CompleteUpload(nil, session.ID, "idemp-1", media.MediaMetadata{
		Container:       "mp4",
		Codec:           "h264",
		DurationSeconds: 120,
		Width:           1280,
		Height:          720,
		FPS:             30,
		ActualBytes:     1000,
	})
	if err != nil {
		t.Fatalf("failed to complete upload: %v", err)
	}

	// 2. Publish ready media to lesson
	pubReq, _ := json.Marshal(media.PublishMediaRequest{
		MediaVersionID: readyVer.ID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_pub/media-publication", bytes.NewReader(pubReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_pub")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK publishing ready media, got %d: %s", w.Code, w.Body.String())
	}

	// Verify published version
	published, err := mediaStore.GetLessonMediaVersion(nil, "lsn_pub")
	if err != nil || published.ID != readyVer.ID {
		t.Fatalf("expected published media version %s, got %v", readyVer.ID, published)
	}

	// 3. Attempting to publish an unready or non-existent media version fails with 404 / 409
	fakeReq, _ := json.Marshal(media.PublishMediaRequest{
		MediaVersionID: "non_existent_mv",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/learning/lessons/lsn_pub/media-publication", bytes.NewReader(fakeReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "inst_pub")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent media version, got %d", w.Code)
	}

	// Old published version remains intact
	current, _ := mediaStore.GetLessonMediaVersion(nil, "lsn_pub")
	if current.ID != readyVer.ID {
		t.Fatalf("published version was corrupted after failed publish attempt")
	}
}
