package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/access"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/accounts"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/sessions"
)

func createAdminAndLogin(t *testing.T, accStore accounts.Store, sessStore sessions.Store) (*accounts.Account, *http.Cookie) {
	t.Helper()
	acc, _, err := accStore.CreateAccount(t.Context(), "admin@example.com", "AdminPassword123")
	if err != nil {
		t.Fatal(err)
	}
	adminAcc, err := accStore.AddRole(t.Context(), acc.ID, "admin")
	if err != nil {
		t.Fatal(err)
	}
	rawToken, _, err := sessStore.CreateSession(t.Context(), adminAcc.ID, sessions.DefaultSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{
		Name:  sessions.SessionCookieName,
		Value: rawToken,
	}
	return adminAcc, cookie
}

func createStudentAndLogin(t *testing.T, accStore accounts.Store, sessStore sessions.Store, email string) (*accounts.Account, *http.Cookie) {
	t.Helper()
	acc, _, err := accStore.CreateAccount(t.Context(), email, "StudentPassword123")
	if err != nil {
		t.Fatal(err)
	}
	rawToken, _, err := sessStore.CreateSession(t.Context(), acc.ID, sessions.DefaultSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{
		Name:  sessions.SessionCookieName,
		Value: rawToken,
	}
	return acc, cookie
}

func TestServiceIdentityRejectsForgedActor(t *testing.T) {
	accStore, sessStore, _, handler := setupTestApp()
	student, studentCookie := createStudentAndLogin(t, accStore, sessStore, "regular_student@example.com")

	// 1. Forged headers without authentication should be rejected
	forgedReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations", bytes.NewReader([]byte(`{"email":"target@example.com"}`)))
	forgedReq.Header.Set("Content-Type", "application/json")
	forgedReq.Header.Set("X-User-Id", "admin_user_id")
	forgedReq.Header.Set("X-User-Roles", "admin")
	forgedResp := httptest.NewRecorder()
	handler.ServeHTTP(forgedResp, forgedReq)

	if forgedResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for forged actor headers without session, got %d", forgedResp.Code)
	}

	// 2. Student trying to invite instructor should be rejected with 403 Forbidden
	inviteBody, _ := json.Marshal(map[string]string{"email": "prospect@example.com"})
	studentInviteReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations", bytes.NewReader(inviteBody))
	studentInviteReq.Header.Set("Content-Type", "application/json")
	studentInviteReq.AddCookie(studentCookie)
	// Even if student attempts to forge X-User-Roles: admin
	studentInviteReq.Header.Set("X-User-Roles", "admin")
	studentInviteResp := httptest.NewRecorder()
	handler.ServeHTTP(studentInviteResp, studentInviteReq)

	if studentInviteResp.Code != http.StatusForbidden {
		t.Fatalf("student invitation status = %d, want 403", studentInviteResp.Code)
	}

	// 3. Student trying to suspend another account should be rejected with 403 Forbidden
	suspendBody, _ := json.Marshal(map[string]any{
		"status":          "suspended",
		"expectedVersion": 1,
		"reason":          "student attempt",
	})
	suspendReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/identity/accounts/%s/status", student.ID), bytes.NewReader(suspendBody))
	suspendReq.Header.Set("Content-Type", "application/json")
	suspendReq.AddCookie(studentCookie)
	suspendResp := httptest.NewRecorder()
	handler.ServeHTTP(suspendResp, suspendReq)

	if suspendResp.Code != http.StatusForbidden {
		t.Fatalf("student suspend status = %d, want 403", suspendResp.Code)
	}
}

func TestRevokedSessionAndRole(t *testing.T) {
	accStore, sessStore, _, handler := setupTestApp()
	_, adminCookie := createAdminAndLogin(t, accStore, sessStore)
	candidateEmail := "instructor_candidate@example.com"
	candidate, candidateCookie := createStudentAndLogin(t, accStore, sessStore, candidateEmail)

	// 1. Admin invites instructor
	inviteBody, _ := json.Marshal(map[string]string{"email": candidateEmail})
	inviteReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations", bytes.NewReader(inviteBody))
	inviteReq.Header.Set("Content-Type", "application/json")
	inviteReq.AddCookie(adminCookie)
	inviteResp := httptest.NewRecorder()
	handler.ServeHTTP(inviteResp, inviteReq)

	if inviteResp.Code != http.StatusOK {
		t.Fatalf("invite status = %d: %s", inviteResp.Code, inviteResp.Body)
	}

	var inviteResult access.InviteInstructorResponse
	_ = json.Unmarshal(inviteResp.Body.Bytes(), &inviteResult)
	if inviteResult.InvitationToken == "" {
		t.Fatal("empty invitation token")
	}

	// 2. Candidate accepts invitation
	acceptBody, _ := json.Marshal(map[string]string{"token": inviteResult.InvitationToken})
	acceptReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations/accept", bytes.NewReader(acceptBody))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.AddCookie(candidateCookie)
	acceptResp := httptest.NewRecorder()
	handler.ServeHTTP(acceptResp, acceptReq)

	if acceptResp.Code != http.StatusOK {
		t.Fatalf("accept invitation status = %d: %s", acceptResp.Code, acceptResp.Body)
	}

	var acceptResult access.AcceptInvitationResponse
	_ = json.Unmarshal(acceptResp.Body.Bytes(), &acceptResult)
	var hasInstructor bool
	for _, r := range acceptResult.Roles {
		if r == "instructor" {
			hasInstructor = true
		}
	}
	if !hasInstructor {
		t.Fatalf("expected instructor role in accepted result: %v", acceptResult.Roles)
	}

	// Verify /api/identity/me now reflects instructor role
	meReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	meReq.AddCookie(candidateCookie)
	meResp := httptest.NewRecorder()
	handler.ServeHTTP(meResp, meReq)
	var meResult sessions.CurrentUserResponse
	_ = json.Unmarshal(meResp.Body.Bytes(), &meResult)
	hasInstructor = false
	for _, r := range meResult.Roles {
		if r == "instructor" {
			hasInstructor = true
		}
	}
	if !hasInstructor {
		t.Fatalf("expected instructor role in /me: %v", meResult.Roles)
	}

	// 3. Admin revokes instructor role with expected version
	// Current account version: candidate created (1), added role (2).
	currentCandidate, _ := accStore.GetAccountByID(t.Context(), candidate.ID)
	revokeReq := httptest.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/api/identity/accounts/%s/instructor-role?expectedVersion=%d", candidate.ID, currentCandidate.Version),
		nil,
	)
	revokeReq.AddCookie(adminCookie)
	revokeResp := httptest.NewRecorder()
	handler.ServeHTTP(revokeResp, revokeReq)

	if revokeResp.Code != http.StatusOK {
		t.Fatalf("revoke instructor status = %d: %s", revokeResp.Code, revokeResp.Body)
	}

	// Check version conflict on repeating with old version
	revokeConflictReq := httptest.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/api/identity/accounts/%s/instructor-role?expectedVersion=%d", candidate.ID, currentCandidate.Version),
		nil,
	)
	revokeConflictReq.AddCookie(adminCookie)
	revokeConflictResp := httptest.NewRecorder()
	handler.ServeHTTP(revokeConflictResp, revokeConflictReq)

	if revokeConflictResp.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d", revokeConflictResp.Code)
	}

	// 4. Admin suspends candidate account
	refreshedCandidate, _ := accStore.GetAccountByID(t.Context(), candidate.ID)
	suspendBody, _ := json.Marshal(map[string]any{
		"status":          "suspended",
		"expectedVersion": refreshedCandidate.Version,
		"reason":          "Admin action",
	})
	suspendReq := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/api/identity/accounts/%s/status", candidate.ID),
		bytes.NewReader(suspendBody),
	)
	suspendReq.Header.Set("Content-Type", "application/json")
	suspendReq.AddCookie(adminCookie)
	suspendResp := httptest.NewRecorder()
	handler.ServeHTTP(suspendResp, suspendReq)

	if suspendResp.Code != http.StatusOK {
		t.Fatalf("suspend status = %d: %s", suspendResp.Code, suspendResp.Body)
	}

	// 5. Candidate session must now be invalidated / forbidden
	candidateMeReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	candidateMeReq.AddCookie(candidateCookie)
	candidateMeResp := httptest.NewRecorder()
	handler.ServeHTTP(candidateMeResp, candidateMeReq)

	if candidateMeResp.Code != http.StatusUnauthorized && candidateMeResp.Code != http.StatusForbidden {
		t.Fatalf("expected suspended user request to be rejected, got %d", candidateMeResp.Code)
	}
}

func TestInternalSessionResolution(t *testing.T) {
	accStore, sessStore, _, handler := setupTestApp()
	student, studentCookie := createStudentAndLogin(t, accStore, sessStore, "resolved_student@example.com")

	resolveBody, _ := json.Marshal(map[string]string{
		"opaqueSession": studentCookie.Value,
	})

	// 1. Without service token header -> 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/internal/identity/sessions/resolve", bytes.NewReader(resolveBody))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthResp := httptest.NewRecorder()
	handler.ServeHTTP(unauthResp, unauthReq)

	if unauthResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without service token, got %d", unauthResp.Code)
	}

	// 2. With valid service token header -> 200
	validReq := httptest.NewRequest(http.MethodPost, "/internal/identity/sessions/resolve", bytes.NewReader(resolveBody))
	validReq.Header.Set("Content-Type", "application/json")
	validReq.Header.Set("X-Service-Token", "test-service-token")
	validResp := httptest.NewRecorder()
	handler.ServeHTTP(validResp, validReq)

	if validResp.Code != http.StatusOK {
		t.Fatalf("resolve status = %d: %s", validResp.Code, validResp.Body)
	}

	var res access.ResolveSessionResponse
	_ = json.Unmarshal(validResp.Body.Bytes(), &res)
	if res.UserID != student.ID || res.Email != "resolved_student@example.com" {
		t.Fatalf("unexpected resolve session result: %+v", res)
	}
}
