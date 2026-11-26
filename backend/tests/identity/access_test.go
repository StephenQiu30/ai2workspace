package identity_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	accesshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/access"
	sessionshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/sessions"
	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	sessionsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/sessions"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
)

func createAdminAndLogin(t *testing.T, accStore accountsapp.Store, sessStore sessionsapp.Store) (*accountsdomain.Account, *http.Cookie) {
	t.Helper()
	adminAcc, err := accStore.EnsureBootstrapAdmin(t.Context(), "admin@example.com", "AdminPassword123")
	if err != nil {
		t.Fatal(err)
	}
	rawToken, _, err := sessStore.CreateSession(t.Context(), adminAcc.ID, sessionsapp.DefaultSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{
		Name:  sessionsapp.SessionCookieName,
		Value: rawToken,
	}
	return adminAcc, cookie
}

func createStudentAndLogin(t *testing.T, accStore accountsapp.Store, sessStore sessionsapp.Store, email string, verified bool) (*accountsdomain.Account, *http.Cookie) {
	t.Helper()
	acc, verifyToken, err := accStore.CreateAccount(t.Context(), email, "StudentPassword123", "")
	if err != nil {
		t.Fatal(err)
	}
	if verified {
		acc, err = accStore.VerifyEmailWithToken(t.Context(), verifyToken)
		if err != nil {
			t.Fatal(err)
		}
	}
	rawToken, _, err := sessStore.CreateSession(t.Context(), acc.ID, sessionsapp.DefaultSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{
		Name:  sessionsapp.SessionCookieName,
		Value: rawToken,
	}
	return acc, cookie
}

func TestServiceIdentityRejectsForgedActor(t *testing.T) {
	accStore, sessStore, _, handler := setupTestApp()
	student, studentCookie := createStudentAndLogin(t, accStore, sessStore, "regular_student@example.com", true)

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
	studentInviteReq.Header.Set("X-User-Roles", "admin")
	studentInviteResp := httptest.NewRecorder()
	handler.ServeHTTP(studentInviteResp, studentInviteReq)

	if studentInviteResp.Code != http.StatusForbidden {
		t.Fatalf("student invitation status = %d, want 403", studentInviteResp.Code)
	}

	// 3. Student trying to suspend another account should be rejected with 403 Forbidden
	suspendBody, _ := json.Marshal(map[string]any{
		"status":          "suspended",
		"expectedVersion": student.Version,
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

func TestAdminListAccountsAndInvitations(t *testing.T) {
	accStore, sessStore, _, handler := setupTestApp()
	_, adminCookie := createAdminAndLogin(t, accStore, sessStore)
	time.Sleep(2 * time.Millisecond)
	s1, s1Cookie := createStudentAndLogin(t, accStore, sessStore, "alpha@example.com", true)
	time.Sleep(2 * time.Millisecond)
	s2, _ := createStudentAndLogin(t, accStore, sessStore, "beta@example.com", false)
	if _, err := accStore.SetAccountStatus(t.Context(), s2.ID, accountsdomain.StatusSuspended, s2.Version); err != nil {
		t.Fatal(err)
	}

	// Student cannot list accounts or invitations
	for _, path := range []string{"/api/identity/accounts", "/api/identity/instructor-invitations"} {
		unauthResp := httptest.NewRecorder()
		handler.ServeHTTP(unauthResp, httptest.NewRequest(http.MethodGet, path, nil))
		if unauthResp.Code != http.StatusUnauthorized {
			t.Fatalf("unauth GET %s = %d, want 401", path, unauthResp.Code)
		}

		studentReq := httptest.NewRequest(http.MethodGet, path, nil)
		studentReq.AddCookie(s1Cookie)
		studentResp := httptest.NewRecorder()
		handler.ServeHTTP(studentResp, studentReq)
		if studentResp.Code != http.StatusForbidden {
			t.Fatalf("student GET %s = %d, want 403", path, studentResp.Code)
		}
	}

	// Admin lists all accounts sorted by createdAt DESC
	listReq := httptest.NewRequest(http.MethodGet, "/api/identity/accounts?page=1&pageSize=2", nil)
	listReq.AddCookie(adminCookie)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list accounts status = %d: %s", listResp.Code, listResp.Body)
	}
	var listRes accesshttp.ListAccountsResponse
	if err := json.Unmarshal(listResp.Body.Bytes(), &listRes); err != nil {
		t.Fatal(err)
	}
	if listRes.Total != 3 || len(listRes.Items) != 2 || listRes.Items[0].Email != "beta@example.com" || listRes.Items[1].Email != "alpha@example.com" {
		t.Fatalf("unexpected paginated accounts: %+v", listRes)
	}

	// Filter by status=suspended & q=beta
	filterReq := httptest.NewRequest(http.MethodGet, "/api/identity/accounts?status=suspended&q=beta", nil)
	filterReq.AddCookie(adminCookie)
	filterResp := httptest.NewRecorder()
	handler.ServeHTTP(filterResp, filterReq)
	var filterRes accesshttp.ListAccountsResponse
	_ = json.Unmarshal(filterResp.Body.Bytes(), &filterRes)
	if filterRes.Total != 1 || len(filterRes.Items) != 1 || filterRes.Items[0].ID != s2.ID {
		t.Fatalf("unexpected filtered accounts: %+v", filterRes)
	}

	// Filter by role=admin
	roleReq := httptest.NewRequest(http.MethodGet, "/api/identity/accounts?role=admin", nil)
	roleReq.AddCookie(adminCookie)
	roleResp := httptest.NewRecorder()
	handler.ServeHTTP(roleResp, roleReq)
	var roleRes accesshttp.ListAccountsResponse
	_ = json.Unmarshal(roleResp.Body.Bytes(), &roleRes)
	if roleRes.Total != 1 || roleRes.Items[0].Email != "admin@example.com" {
		t.Fatalf("unexpected role-filtered accounts: %+v", roleRes)
	}

	// Create two invitations and list them in createdAt DESC order
	for _, email := range []string{"first_inv@example.com", "second_inv@example.com"} {
		body, _ := json.Marshal(map[string]string{"email": email})
		req := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adminCookie)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("invite %s failed: %d %s", email, resp.Code, resp.Body)
		}
		time.Sleep(2 * time.Millisecond)
	}

	invListReq := httptest.NewRequest(http.MethodGet, "/api/identity/instructor-invitations", nil)
	invListReq.AddCookie(adminCookie)
	invListResp := httptest.NewRecorder()
	handler.ServeHTTP(invListResp, invListReq)
	if invListResp.Code != http.StatusOK {
		t.Fatalf("list invitations status = %d: %s", invListResp.Code, invListResp.Body)
	}
	var invListRes accesshttp.ListInvitationsResponse
	if err := json.Unmarshal(invListResp.Body.Bytes(), &invListRes); err != nil {
		t.Fatal(err)
	}
	if invListRes.Total != 2 || len(invListRes.Items) != 2 || invListRes.Items[0].Email != "second_inv@example.com" || invListRes.Items[1].Email != "first_inv@example.com" {
		t.Fatalf("unexpected invitations list: %+v", invListRes)
	}
	_ = s1
}

func TestUnverifiedEmailCannotAcceptInvitationAndAuditLogsRecorded(t *testing.T) {
	accStore, sessStore, invStore, handler := setupTestApp()
	admin, adminCookie := createAdminAndLogin(t, accStore, sessStore)
	candidateEmail := "instructor_candidate@example.com"
	candidate, candidateCookie := createStudentAndLogin(t, accStore, sessStore, candidateEmail, false)

	// 1. Admin invites instructor (default mode hides invitationToken from JSON)
	inviteBody, _ := json.Marshal(map[string]string{"email": candidateEmail})
	inviteReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations", bytes.NewReader(inviteBody))
	inviteReq.Header.Set("Content-Type", "application/json")
	inviteReq.AddCookie(adminCookie)
	inviteResp := httptest.NewRecorder()
	handler.ServeHTTP(inviteResp, inviteReq)

	if inviteResp.Code != http.StatusOK {
		t.Fatalf("invite status = %d: %s", inviteResp.Code, inviteResp.Body)
	}

	var inviteResult accesshttp.InviteInstructorResponse
	_ = json.Unmarshal(inviteResp.Body.Bytes(), &inviteResult)
	if inviteResult.InvitationToken != "" {
		t.Fatalf("invitationToken leaked in default mode: %q", inviteResult.InvitationToken)
	}
	inviteToken := latestMailToken(t, accStore, candidateEmail, accountsdomain.PurposeInstructorInvite)

	// 2. Unverified candidate attempting to accept invitation must be rejected with 403
	acceptBody, _ := json.Marshal(map[string]string{"token": inviteToken})
	unverifiedAcceptReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations/accept", bytes.NewReader(acceptBody))
	unverifiedAcceptReq.Header.Set("Content-Type", "application/json")
	unverifiedAcceptReq.AddCookie(candidateCookie)
	unverifiedAcceptResp := httptest.NewRecorder()
	handler.ServeHTTP(unverifiedAcceptResp, unverifiedAcceptReq)

	if unverifiedAcceptResp.Code != http.StatusForbidden {
		t.Fatalf("unverified accept status = %d, want 403: %s", unverifiedAcceptResp.Code, unverifiedAcceptResp.Body)
	}

	// Verify candidate email now
	verifyToken := latestMailToken(t, accStore, candidateEmail, accountsdomain.PurposeEmailVerification)
	if _, err := accStore.VerifyEmailWithToken(t.Context(), verifyToken); err != nil {
		t.Fatal(err)
	}

	// 3. Verified candidate accepts invitation successfully
	acceptReq := httptest.NewRequest(http.MethodPost, "/api/identity/instructor-invitations/accept", bytes.NewReader(acceptBody))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq.AddCookie(candidateCookie)
	acceptResp := httptest.NewRecorder()
	handler.ServeHTTP(acceptResp, acceptReq)

	if acceptResp.Code != http.StatusOK {
		t.Fatalf("accept invitation status = %d: %s", acceptResp.Code, acceptResp.Body)
	}

	var acceptResult accesshttp.AcceptInvitationResponse
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

	// Verify /api/identity/me reflects instructor role
	meReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	meReq.AddCookie(candidateCookie)
	meResp := httptest.NewRecorder()
	handler.ServeHTTP(meResp, meReq)
	var meResult sessionshttp.CurrentUserResponse
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

	// 4. Admin revokes instructor role with reason -> recorded in audit_logs
	currentCandidate, _ := accStore.GetAccountByID(t.Context(), candidate.ID)
	revokeReq := httptest.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/api/identity/accounts/%s/instructor-role?expectedVersion=%d&reason=course_policy", candidate.ID, currentCandidate.Version),
		nil,
	)
	revokeReq.AddCookie(adminCookie)
	revokeResp := httptest.NewRecorder()
	handler.ServeHTTP(revokeResp, revokeReq)

	if revokeResp.Code != http.StatusOK {
		t.Fatalf("revoke instructor status = %d: %s", revokeResp.Code, revokeResp.Body)
	}

	// Version conflict on stale expectedVersion
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

	// 5. Invalid account status must be rejected with 400
	refreshedCandidate, _ := accStore.GetAccountByID(t.Context(), candidate.ID)
	invalidStatusBody, _ := json.Marshal(map[string]any{
		"status":          "deleted",
		"expectedVersion": refreshedCandidate.Version,
		"reason":          "invalid status test",
	})
	invalidStatusReq := httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/api/identity/accounts/%s/status", candidate.ID),
		bytes.NewReader(invalidStatusBody),
	)
	invalidStatusReq.Header.Set("Content-Type", "application/json")
	invalidStatusReq.AddCookie(adminCookie)
	invalidStatusResp := httptest.NewRecorder()
	handler.ServeHTTP(invalidStatusResp, invalidStatusReq)
	if invalidStatusResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid status code = %d, want 400", invalidStatusResp.Code)
	}

	// 6. Admin suspends candidate account -> sessions revoked + audit log persisted
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

	// Verify audit logs for candidate contain both revoke_instructor_role and set_account_status
	logs, err := invStore.ListAuditLogs(t.Context(), candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sawStatusLog, sawRevokeLog bool
	for _, entry := range logs {
		if entry.ActorID != admin.ID {
			t.Fatalf("unexpected audit actor %q, want %q", entry.ActorID, admin.ID)
		}
		if entry.Action == "set_account_status" && entry.Reason == "Admin action" && entry.Detail == "suspended" {
			sawStatusLog = true
		}
		if entry.Action == "revoke_instructor_role" && entry.Reason == "course_policy" {
			sawRevokeLog = true
		}
	}
	if !sawStatusLog || !sawRevokeLog {
		t.Fatalf("missing expected audit logs: status=%v revoke=%v logs=%+v", sawStatusLog, sawRevokeLog, logs)
	}

	// 7. Candidate session must now be invalidated
	candidateMeReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	candidateMeReq.AddCookie(candidateCookie)
	candidateMeResp := httptest.NewRecorder()
	handler.ServeHTTP(candidateMeResp, candidateMeReq)

	if candidateMeResp.Code != http.StatusUnauthorized && candidateMeResp.Code != http.StatusForbidden {
		t.Fatalf("expected suspended user request to be rejected, got %d", candidateMeResp.Code)
	}
}
