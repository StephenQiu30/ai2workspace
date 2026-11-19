package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/server"
)

func setupLearningApp() (*courses.MemoryStore, http.Handler) {
	store := courses.NewMemoryStore()
	handler := server.NewWithStore(false, store, "test-service-token")
	return store, handler
}

func TestCourseOwnershipAndPublication(t *testing.T) {
	_, handler := setupLearningApp()

	// 1. Instructor creates a draft course
	createBody, _ := json.Marshal(courses.CreateCourseRequest{
		Title:       "Next.js Fullstack Course",
		Description: "Comprehensive Next.js and TypeScript guide",
		CoverURL:    "https://example.com/cover.png",
		IsFree:      false,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/learning/courses", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Actor-Id", "instructor_1")
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)

	if createResp.Code != http.StatusOK {
		t.Fatalf("create course status = %d: %s", createResp.Code, createResp.Body)
	}

	var createResult courses.CourseMutationResponse
	_ = json.Unmarshal(createResp.Body.Bytes(), &createResult)
	courseID := createResult.CourseID
	if courseID == "" || createResult.State != "draft" {
		t.Fatalf("unexpected create course result: %+v", createResult)
	}

	// 2. Draft is not visible in public course list
	listReq := httptest.NewRequest(http.MethodGet, "/api/learning/courses", nil)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	var listResult courses.CourseListResponse
	_ = json.Unmarshal(listResp.Body.Bytes(), &listResult)
	if listResult.Total != 0 {
		t.Fatalf("expected 0 public courses, got %d", listResult.Total)
	}

	// 3. Different user attempting to update course draft is rejected with 403 Forbidden
	updateBody, _ := json.Marshal(courses.UpdateCourseRequest{
		Title:           "Malicious Update",
		ExpectedVersion: createResult.Version,
	})
	badUpdateReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/learning/courses/%s", courseID), bytes.NewReader(updateBody))
	badUpdateReq.Header.Set("Content-Type", "application/json")
	badUpdateReq.Header.Set("X-Actor-Id", "stranger_2")
	badUpdateResp := httptest.NewRecorder()
	handler.ServeHTTP(badUpdateResp, badUpdateReq)

	if badUpdateResp.Code != http.StatusForbidden {
		t.Fatalf("unauthorized update status = %d, want 403", badUpdateResp.Code)
	}

	// 4. Owner submits course for review
	submitBody, _ := json.Marshal(courses.SubmitCourseRequest{ExpectedVersion: createResult.Version})
	submitReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/learning/courses/%s/submissions", courseID), bytes.NewReader(submitBody))
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq.Header.Set("X-Actor-Id", "instructor_1")
	submitResp := httptest.NewRecorder()
	handler.ServeHTTP(submitResp, submitReq)

	if submitResp.Code != http.StatusOK {
		t.Fatalf("submit course status = %d: %s", submitResp.Code, submitResp.Body)
	}
	var submitResult courses.CourseMutationResponse
	_ = json.Unmarshal(submitResp.Body.Bytes(), &submitResult)
	if submitResult.State != "pending_review" {
		t.Fatalf("expected state pending_review, got %s", submitResult.State)
	}

	// 5. Admin reviews and approves the course
	reviewBody, _ := json.Marshal(courses.ReviewCourseRequest{
		Decision:        "approved",
		Reason:          "Meets curriculum standards",
		ExpectedVersion: submitResult.Version,
	})
	reviewReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/learning/courses/%s/reviews", courseID), bytes.NewReader(reviewBody))
	reviewReq.Header.Set("Content-Type", "application/json")
	reviewResp := httptest.NewRecorder()
	handler.ServeHTTP(reviewResp, reviewReq)

	if reviewResp.Code != http.StatusOK {
		t.Fatalf("review course status = %d: %s", reviewResp.Code, reviewResp.Body)
	}
	var reviewResult courses.CourseMutationResponse
	_ = json.Unmarshal(reviewResp.Body.Bytes(), &reviewResult)
	if reviewResult.State != "approved" {
		t.Fatalf("expected state approved, got %s", reviewResult.State)
	}

	// 6. Public list now discovers the approved course
	listResp2 := httptest.NewRecorder()
	handler.ServeHTTP(listResp2, listReq)
	var listResult2 courses.CourseListResponse
	_ = json.Unmarshal(listResp2.Body.Bytes(), &listResult2)
	if listResult2.Total != 1 || listResult2.Courses[0].ID != courseID {
		t.Fatalf("expected approved course in list, got: %+v", listResult2)
	}

	// 7. Optimistic concurrency: updating with stale version returns 409
	staleUpdateBody, _ := json.Marshal(courses.UpdateCourseRequest{
		Title:           "Stale Title",
		ExpectedVersion: 1, // Already incremented
	})
	staleReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/learning/courses/%s", courseID), bytes.NewReader(staleUpdateBody))
	staleReq.Header.Set("Content-Type", "application/json")
	staleReq.Header.Set("X-Actor-Id", "instructor_1")
	staleResp := httptest.NewRecorder()
	handler.ServeHTTP(staleResp, staleReq)

	if staleResp.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d", staleResp.Code)
	}
}

func TestSaleSnapshotDoesNotCheckEntitlement(t *testing.T) {
	store, handler := setupLearningApp()

	// Seed course directly in store
	course, err := store.CreateCourse(t.Context(), "instructor_1", "Snapshot Test Course", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = store.ReviewCourse(t.Context(), course.ID, "approved", "ok", course.Version)

	// Call internal sale snapshot endpoint
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/internal/learning/courses/%s/sale-snapshot", course.ID), nil)
	req.Header.Set("X-Service-Token", "test-service-token")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("sale snapshot status = %d: %s", resp.Code, resp.Body)
	}

	var snapshot courses.SaleSnapshotResponse
	_ = json.Unmarshal(resp.Body.Bytes(), &snapshot)
	if snapshot.CourseID != course.ID || snapshot.Title != "Snapshot Test Course" || !snapshot.Deliverable {
		t.Fatalf("unexpected snapshot response: %+v", snapshot)
	}
}

func TestDeliveryEvidenceKeepsEffectiveTime(t *testing.T) {
	store, handler := setupLearningApp()

	// Seed course and approve it
	course, _ := store.CreateCourse(t.Context(), "instructor_1", "Evidence Course", "", "", false)
	approvedCourse, _ := store.ReviewCourse(t.Context(), course.ID, "approved", "ok", course.Version)
	if approvedCourse.FirstDeliverableAt == nil {
		t.Fatal("expected FirstDeliverableAt to be set upon approval")
	}

	// Query delivery evidence via internal endpoint
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/internal/learning/courses/%s/delivery-evidence", course.ID), nil)
	req.Header.Set("X-Service-Token", "test-service-token")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("delivery evidence status = %d", resp.Code)
	}

	var ev courses.DeliveryEvidenceResponse
	_ = json.Unmarshal(resp.Body.Bytes(), &ev)
	if ev.FirstDeliverableAt == nil {
		t.Fatal("delivery evidence returned nil firstDeliverableAt")
	}

	// Repeated query retains the exact same timestamp
	resp2 := httptest.NewRecorder()
	handler.ServeHTTP(resp2, req)
	var ev2 courses.DeliveryEvidenceResponse
	_ = json.Unmarshal(resp2.Body.Bytes(), &ev2)

	if !ev.FirstDeliverableAt.Equal(*ev2.FirstDeliverableAt) {
		t.Fatalf("evidence time changed between queries: %v vs %v", ev.FirstDeliverableAt, ev2.FirstDeliverableAt)
	}
}

func TestFreeCourseEnrollment(t *testing.T) {
	store, handler := setupLearningApp()

	freeCourse, _ := store.CreateCourse(t.Context(), "instructor_1", "Free Introductory Course", "", "", true)
	_, _ = store.ReviewCourse(t.Context(), freeCourse.ID, "approved", "ok", freeCourse.Version)

	paidCourse, _ := store.CreateCourse(t.Context(), "instructor_1", "Paid Advanced Course", "", "", false)
	_, _ = store.ReviewCourse(t.Context(), paidCourse.ID, "approved", "ok", paidCourse.Version)

	// 1. Enrolling in free course succeeds
	freeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/learning/courses/%s/enrollments", freeCourse.ID), nil)
	freeReq.Header.Set("X-Actor-Id", "student_1")
	freeResp := httptest.NewRecorder()
	handler.ServeHTTP(freeResp, freeReq)

	if freeResp.Code != http.StatusOK {
		t.Fatalf("free enrollment status = %d: %s", freeResp.Code, freeResp.Body)
	}

	// 2. Enrolling in paid course directly fails
	paidReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/learning/courses/%s/enrollments", paidCourse.ID), nil)
	paidReq.Header.Set("X-Actor-Id", "student_1")
	paidResp := httptest.NewRecorder()
	handler.ServeHTTP(paidResp, paidReq)

	if paidResp.Code != http.StatusBadRequest {
		t.Fatalf("paid course enrollment status = %d, want 400", paidResp.Code)
	}

	// 3. User course list returns the free course
	myReq := httptest.NewRequest(http.MethodGet, "/api/learning/me/courses", nil)
	myReq.Header.Set("X-Actor-Id", "student_1")
	myResp := httptest.NewRecorder()
	handler.ServeHTTP(myResp, myReq)

	var myCourses []courses.CourseResponse
	_ = json.Unmarshal(myResp.Body.Bytes(), &myCourses)
	if len(myCourses) != 1 || myCourses[0].ID != freeCourse.ID {
		t.Fatalf("unexpected user courses: %+v", myCourses)
	}
}
