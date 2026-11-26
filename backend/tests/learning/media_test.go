package learning_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	mediahttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/media"
	mediaapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/media"
	coursesdomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/courses"
	mediadomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/media"
)

func seedCourseWithTwoLessons(t *testing.T, env *testEnv) (*coursesdomain.Course, *coursesdomain.Lesson, *coursesdomain.Lesson) {
	t.Helper()
	c, err := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "媒体测试课", "", "", "engineering", false)
	if err != nil {
		t.Fatal(err)
	}
	sec, err := env.coursesStore.AddSection(t.Context(), c.ID, "第一章", 1)
	if err != nil {
		t.Fatal(err)
	}
	l1, err := env.coursesStore.AddLesson(t.Context(), c.ID, sec.ID, "试看课时", "video", true, "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	l2, err := env.coursesStore.AddLesson(t.Context(), c.ID, sec.ID, "付费课时", "video", false, "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	cur, _ := env.coursesStore.GetCourse(t.Context(), c.ID)
	return cur, l1, l2
}

func TestUploadAuthAndR15Limits(t *testing.T) {
	env := newTestEnv()
	c, l1, l2 := seedCourseWithTwoLessons(t, env)

	// 1. Auth & ownership checks
	w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "intro.mp4", DeclaredBytes: 1024,
	}, "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("guest upload status = %d, want 401", w.Code)
	}
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "intro.mp4", DeclaredBytes: 1024,
	}, sessStudent1, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("student upload status = %d, want 403", w.Code)
	}
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "intro.mp4", DeclaredBytes: 1024,
	}, sessInst2, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner upload status = %d, want 403", w.Code)
	}

	// 2. Over maximum declared size (> 2 GiB) -> 413
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "intro.mp4", DeclaredBytes: 2147483649,
	}, sessInst1, nil)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Invalid non-mp4 format -> 400
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "intro.avi", DeclaredBytes: 1024 * 1024,
	}, sessInst1, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for .avi, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Max 2 concurrent sessions -> 3rd gets 429; cancelling one frees slot
	w1 := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "v1.mp4", DeclaredBytes: 1024 * 1024,
	}, sessInst1, nil)
	if w1.Code != http.StatusCreated {
		t.Fatalf("1st upload status = %d: %s", w1.Code, w1.Body.String())
	}
	var u1 mediahttp.UploadSessionResponse
	_ = json.Unmarshal(w1.Body.Bytes(), &u1)

	w2 := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l2.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "v2.mp4", DeclaredBytes: 1024 * 1024,
	}, sessInst1, nil)
	if w2.Code != http.StatusCreated {
		t.Fatalf("2nd upload status = %d: %s", w2.Code, w2.Body.String())
	}

	w3 := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l2.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "v3.mp4", DeclaredBytes: 1024 * 1024,
	}, sessInst1, nil)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd upload status = %d, want 429", w3.Code)
	}

	// Other instructor cannot cancel u1
	if w := doRequest(t, env.router, http.MethodDelete, "/api/learning/uploads/"+u1.ID, nil, sessInst2, nil); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner cancel status = %d, want 403", w.Code)
	}
	// Owner cancels u1 -> 3rd upload now succeeds
	if w := doRequest(t, env.router, http.MethodDelete, "/api/learning/uploads/"+u1.ID, nil, sessInst1, nil); w.Code != http.StatusNoContent {
		t.Fatalf("owner cancel status = %d, want 204", w.Code)
	}
	w4 := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "v4.mp4", DeclaredBytes: 1024 * 1024,
	}, sessInst1, nil)
	if w4.Code != http.StatusCreated {
		t.Fatalf("upload after cancel status = %d, want 201", w4.Code)
	}
}

func TestUploadCompletionValidationIdempotencyAndPublishSync(t *testing.T) {
	env := newTestEnv()
	c, l1, l2 := seedCourseWithTwoLessons(t, env)

	// Create upload on l1
	w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "lesson1.mp4", DeclaredBytes: 20 * 1024 * 1024,
	}, sessInst1, nil)
	var u mediahttp.UploadSessionResponse
	_ = json.Unmarshal(w.Body.Bytes(), &u)

	// Non-owner cannot complete upload
	compBody := mediahttp.CompleteUploadRequest{
		IdempotencyKey: "k-valid", Container: "mp4", Codec: "h264",
		DurationSeconds: 1800, Width: 1920, Height: 1080, FPS: 30, ActualBytes: 20 * 1024 * 1024,
	}
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+u.ID+"/completion", compBody, sessInst2, nil); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner complete status = %d, want 403", w.Code)
	}

	// HEVC rejected with 422
	badCodec := compBody
	badCodec.IdempotencyKey = "k-hevc"
	badCodec.Codec = "hevc"
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+u.ID+"/completion", badCodec, sessInst1, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("hevc status = %d, want 422", w.Code)
	}

	// 4K rejected with 422
	badRes := compBody
	badRes.IdempotencyKey = "k-4k"
	badRes.Width = 3840
	badRes.Height = 2160
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+u.ID+"/completion", badRes, sessInst1, nil); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("4K status = %d, want 422", w.Code)
	}

	// Valid completion -> 200 ready
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+u.ID+"/completion", compBody, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("valid completion status = %d: %s", w.Code, w.Body.String())
	}
	var mv1 mediahttp.MediaVersionResponse
	_ = json.Unmarshal(w.Body.Bytes(), &mv1)
	if mv1.Status != "ready" || len(mv1.HLSQualities) != 2 || mv1.HLSQualities[1] != "1080p" {
		t.Fatalf("unexpected media version: %+v", mv1)
	}

	// Idempotent replay with identical payload returns same media version ID
	wReplay := doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+u.ID+"/completion", compBody, sessInst1, nil)
	if wReplay.Code != http.StatusOK {
		t.Fatalf("idempotent replay status = %d", wReplay.Code)
	}
	var mvReplay mediahttp.MediaVersionResponse
	_ = json.Unmarshal(wReplay.Body.Bytes(), &mvReplay)
	if mvReplay.ID != mv1.ID {
		t.Fatalf("idempotent replay created duplicate version: %s vs %s", mvReplay.ID, mv1.ID)
	}

	// Same idempotency key with DIFFERENT payload -> 409 Conflict
	conflictBody := compBody
	conflictBody.DurationSeconds = 999
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+u.ID+"/completion", conflictBody, sessInst1, nil); w.Code != http.StatusConflict {
		t.Fatalf("idempotency conflict status = %d, want 409", w.Code)
	}

	// Publishing l1's media version onto l2 MUST be rejected (lesson mismatch -> 409)
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l2.ID+"/media-publication", mediahttp.PublishMediaRequest{
		MediaVersionID: mv1.ID,
	}, sessInst1, nil); w.Code != http.StatusConflict {
		t.Fatalf("cross-lesson publish status = %d, want 409", w.Code)
	}

	// Publishing onto l1 succeeds AND syncs courses.Store lesson.MediaVersionID
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/media-publication", mediahttp.PublishMediaRequest{
		MediaVersionID: mv1.ID,
	}, sessInst1, nil); w.Code != http.StatusOK {
		t.Fatalf("publish status = %d: %s", w.Code, w.Body.String())
	}
	updatedLesson, _, err := env.coursesStore.GetLesson(t.Context(), l1.ID)
	if err != nil || updatedLesson.MediaVersionID != mv1.ID {
		t.Fatalf("lesson mediaVersionId not synced in coursesStore: %+v (err=%v)", updatedLesson, err)
	}

	// GET upload status: owner allowed (200), non-owner denied (403)
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/uploads/"+u.ID, nil, sessInst1, nil); w.Code != http.StatusOK {
		t.Fatalf("owner get upload status = %d, want 200", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/uploads/"+u.ID, nil, sessInst2, nil); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner get upload status = %d, want 403", w.Code)
	}

	// Cancelling already-completed upload returns 409 Conflict
	if w := doRequest(t, env.router, http.MethodDelete, "/api/learning/uploads/"+u.ID, nil, sessInst1, nil); w.Code != http.StatusConflict {
		t.Fatalf("cancel completed upload status = %d, want 409", w.Code)
	}

	// Replacement failure protection: uploading a bad replacement video & trying to publish it fails (409) and preserves mv1
	wRep := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/uploads", mediahttp.CreateUploadRequest{
		CourseID: c.ID, FileName: "replace.mp4", DeclaredBytes: 1024,
	}, sessInst1, nil)
	var uRep mediahttp.UploadSessionResponse
	_ = json.Unmarshal(wRep.Body.Bytes(), &uRep)

	wBad := doRequest(t, env.router, http.MethodPost, "/api/learning/uploads/"+uRep.ID+"/completion", mediahttp.CompleteUploadRequest{
		IdempotencyKey: "k-bad-rep", Container: "mp4", Codec: "h264",
		DurationSeconds: 7201, Width: 1920, Height: 1080, FPS: 30, ActualBytes: 1024,
	}, sessInst1, nil)
	if wBad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("over-duration replacement status = %d, want 422", wBad.Code)
	}
	var mvFailed mediahttp.MediaVersionResponse
	_ = json.Unmarshal(wBad.Body.Bytes(), &mvFailed)

	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+l1.ID+"/media-publication", mediahttp.PublishMediaRequest{
		MediaVersionID: mvFailed.ID,
	}, sessInst1, nil); w.Code != http.StatusConflict {
		t.Fatalf("publishing failed replacement status = %d, want 409", w.Code)
	}
	currentVer, err := env.mediaStore.GetLessonMediaVersion(t.Context(), l1.ID)
	if err != nil || currentVer.ID != mv1.ID {
		t.Fatalf("existing published version was overwritten: %+v (err=%v)", currentVer, err)
	}
}

func TestPlaybackAuthorizationSecurityAndSignedToken(t *testing.T) {
	env := newTestEnv()
	c, trialLesson, paidLesson := seedCourseWithTwoLessons(t, env)

	// Upload & publish ready media for both lessons
	publishReadyMedia := func(lessonID string) string {
		t.Helper()
		u, err := env.mediaStore.CreateUpload(t.Context(), "instructor_1", c.ID, lessonID, "video.mp4", 1024)
		if err != nil {
			t.Fatal(err)
		}
		mv, err := env.mediaStore.CompleteUpload(t.Context(), u.ID, "instructor_1", false, "k-"+lessonID, mediadomain.MediaMetadata{
			Container: "mp4", Codec: "h264", DurationSeconds: 120, Width: 1280, Height: 720, FPS: 30, ActualBytes: 1024,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := env.mediaStore.PublishLessonMedia(t.Context(), lessonID, mv.ID, "instructor_1", false); err != nil {
			t.Fatal(err)
		}
		_ = env.coursesStore.SetLessonMediaVersion(t.Context(), lessonID, mv.ID)
		return mv.ID
	}

	vTrial := publishReadyMedia(trialLesson.ID)
	vPaid := publishReadyMedia(paidLesson.ID)

	// Unpublished draft: student cannot play even trial lesson (404), owner CAN preview (200)
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+trialLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "trial",
	}, sessStudent1, nil); w.Code != http.StatusNotFound {
		t.Fatalf("draft playback for student status = %d, want 404", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "playback",
	}, sessInst1, nil); w.Code != http.StatusOK {
		t.Fatalf("draft preview for owner status = %d, want 200", w.Code)
	}

	// Publish the course
	sub, _ := env.coursesStore.SubmitCourse(t.Context(), c.ID, "instructor_1", false, c.Version)
	app, _ := env.coursesStore.ReviewCourse(t.Context(), c.ID, "approved", "ok", sub.Version)

	// 1. Trial lesson succeeds for anonymous guest and returns a verifiable HMAC-SHA256 signed token
	w := doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+trialLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "playback",
	}, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trial playback for guest status = %d: %s", w.Code, w.Body.String())
	}
	var trialResp mediahttp.PlaybackAuthorizationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &trialResp)
	if trialResp.MediaVersionID != vTrial || trialResp.Token == "" {
		t.Fatalf("invalid trial playback response: %+v", trialResp)
	}
	claims, err := mediaapp.VerifyPlaybackToken(trialResp.Token, testSigningSecret, time.Now().UTC())
	if err != nil || claims.LessonID != trialLesson.ID || claims.Purpose != "trial" {
		t.Fatalf("signed token verification failed: claims=%+v err=%v", claims, err)
	}
	// Tampered token MUST fail verification
	if _, err := mediaapp.VerifyPlaybackToken(trialResp.Token+"tamper", testSigningSecret, time.Now().UTC()); err == nil {
		t.Fatal("tampered playback token unexpectedly verified")
	}
	// Expired token MUST fail verification
	if _, err := mediaapp.VerifyPlaybackToken(trialResp.Token, testSigningSecret, time.Now().UTC().Add(2*time.Hour)); err == nil {
		t.Fatal("expired playback token unexpectedly verified")
	}

	// 2. CRITICAL: Paid non-trial lesson passing purpose="trial" MUST BE REJECTED for both guest and unpurchased student!
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "trial",
	}, "", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest spoofing purpose=trial on paid lesson status = %d, want 403", w.Code)
	}
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "trial",
	}, sessStudent1, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unpurchased student spoofing purpose=trial on paid lesson status = %d, want 403", w.Code)
	}

	// 3. Unauthenticated non-trial with purpose="playback" -> 401
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "playback",
	}, "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("guest paid playback status = %d, want 401", w.Code)
	}

	// 4. Purchased student gets signed playback token
	env.entitlements.Grant("student_1", c.ID)
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "playback",
	}, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("purchased student playback status = %d: %s", w.Code, w.Body.String())
	}
	var paidResp mediahttp.PlaybackAuthorizationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &paidResp)
	if paidResp.MediaVersionID != vPaid {
		t.Fatalf("expected %s, got %s", vPaid, paidResp.MediaVersionID)
	}

	// 5. Closing sales preserves playback for purchased student
	closed, _ := env.coursesStore.CloseSales(t.Context(), c.ID, "admin_1", "停售", app.Version)
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "playback",
	}, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("closed-sales purchased playback status = %d, want 200", w.Code)
	}

	// 6. Withdrawing course rejects playback even for purchased student (403)
	_, _ = env.coursesStore.WithdrawCourse(t.Context(), c.ID, "admin_1", "违规撤回", closed.Version)
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/lessons/"+paidLesson.ID+"/playback-authorizations", mediahttp.AuthorizePlaybackRequest{
		CourseID: c.ID, Purpose: "playback",
	}, sessStudent1, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("withdrawn course playback status = %d, want 403", w.Code)
	}
}
