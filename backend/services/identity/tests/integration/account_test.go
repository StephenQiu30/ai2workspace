package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/access"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/accounts"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/server"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/sessions"
)

func setupTestApp() (*accounts.MemoryStore, *sessions.MemoryStore, *access.MemoryStore, http.Handler) {
	accStore := accounts.NewMemoryStore()
	sessStore := sessions.NewMemoryStore()
	invStore := access.NewMemoryStore()
	handler := server.NewWithStores(false, accStore, sessStore, invStore, "test-service-token")
	return accStore, sessStore, invStore, handler
}

func TestRegisterDuplicateEmail(t *testing.T) {
	accStore, _, _, handler := setupTestApp()

	body, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "Password123!",
	})

	// First registration
	req := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("first register status = %d: %s", resp.Code, resp.Body)
	}

	var firstResult accounts.RegisterResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &firstResult); err != nil {
		t.Fatal(err)
	}
	if !firstResult.Accepted || firstResult.NextStep != "verify_email" {
		t.Fatalf("unexpected first register response: %+v", firstResult)
	}
	if firstResult.VerificationToken == "" {
		t.Fatal("expected verificationToken in test environment")
	}

	// Second registration with duplicate email
	reqDup := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	respDup := httptest.NewRecorder()
	handler.ServeHTTP(respDup, reqDup)

	if respDup.Code != http.StatusOK {
		t.Fatalf("duplicate register status = %d: %s", respDup.Code, respDup.Body)
	}

	var dupResult accounts.RegisterResponse
	if err := json.Unmarshal(respDup.Body.Bytes(), &dupResult); err != nil {
		t.Fatal(err)
	}
	// Per AC-01: does not leak email existence, returns accepted
	if !dupResult.Accepted || dupResult.NextStep != "verify_email" {
		t.Fatalf("unexpected duplicate register response: %+v", dupResult)
	}
	// Assert account count is strictly 1
	if count := accStore.CountAccounts(req.Context()); count != 1 {
		t.Fatalf("expected 1 account, got %d", count)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	_, _, _, handler := setupTestApp()

	// Register account
	regBody, _ := json.Marshal(map[string]string{
		"email":    "login_test@example.com",
		"password": "CorrectPassword123",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp := httptest.NewRecorder()
	handler.ServeHTTP(regResp, regReq)
	if regResp.Code != http.StatusOK {
		t.Fatalf("register status = %d", regResp.Code)
	}

	// Attempt login with wrong password
	wrongBody, _ := json.Marshal(map[string]string{
		"email":    "login_test@example.com",
		"password": "WrongPassword456",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(wrongBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	handler.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want %d", loginResp.Code, http.StatusUnauthorized)
	}
	for _, cookie := range loginResp.Result().Cookies() {
		if cookie.Name == sessions.SessionCookieName && cookie.Value != "" {
			t.Fatalf("session cookie issued on wrong password: %v", cookie)
		}
	}

	// Attempt login with correct password
	correctBody, _ := json.Marshal(map[string]string{
		"email":    "login_test@example.com",
		"password": "CorrectPassword123",
	})
	correctReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(correctBody))
	correctReq.Header.Set("Content-Type", "application/json")
	correctResp := httptest.NewRecorder()
	handler.ServeHTTP(correctResp, correctReq)

	if correctResp.Code != http.StatusOK {
		t.Fatalf("correct login status = %d", correctResp.Code)
	}
	var foundSessionCookie bool
	for _, cookie := range correctResp.Result().Cookies() {
		if cookie.Name == sessions.SessionCookieName && cookie.Value != "" {
			foundSessionCookie = true
			if !cookie.HttpOnly {
				t.Error("expected session cookie to be HttpOnly")
			}
		}
	}
	if !foundSessionCookie {
		t.Fatal("session cookie not found on successful login")
	}
}

func TestAccountTokenSingleUse(t *testing.T) {
	_, _, _, handler := setupTestApp()

	// Register account to obtain verification token
	regBody, _ := json.Marshal(map[string]string{
		"email":    "token_test@example.com",
		"password": "StrongPassword123",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp := httptest.NewRecorder()
	handler.ServeHTTP(regResp, regReq)

	var regResult accounts.RegisterResponse
	_ = json.Unmarshal(regResp.Body.Bytes(), &regResult)
	token := regResult.VerificationToken
	if token == "" {
		t.Fatal("empty verification token")
	}

	// Verify email first time
	vBody, _ := json.Marshal(map[string]string{"token": token})
	vReq := httptest.NewRequest(http.MethodPost, "/api/identity/email-verifications", bytes.NewReader(vBody))
	vReq.Header.Set("Content-Type", "application/json")
	vResp := httptest.NewRecorder()
	handler.ServeHTTP(vResp, vReq)

	if vResp.Code != http.StatusOK {
		t.Fatalf("verify email status = %d: %s", vResp.Code, vResp.Body)
	}
	var vResult accounts.VerifyEmailResponse
	_ = json.Unmarshal(vResp.Body.Bytes(), &vResult)
	if !vResult.EmailVerified {
		t.Fatal("email should be marked verified")
	}

	// Re-verify with same token should be idempotent and not fail or rollback
	vReqDup := httptest.NewRequest(http.MethodPost, "/api/identity/email-verifications", bytes.NewReader(vBody))
	vReqDup.Header.Set("Content-Type", "application/json")
	vRespDup := httptest.NewRecorder()
	handler.ServeHTTP(vRespDup, vReqDup)

	if vRespDup.Code != http.StatusOK {
		t.Fatalf("duplicate verify email status = %d: %s", vRespDup.Code, vRespDup.Body)
	}

	// Attempt verify with bogus token
	bogusBody, _ := json.Marshal(map[string]string{"token": "bogus_token_12345"})
	bogusReq := httptest.NewRequest(http.MethodPost, "/api/identity/email-verifications", bytes.NewReader(bogusBody))
	bogusReq.Header.Set("Content-Type", "application/json")
	bogusResp := httptest.NewRecorder()
	handler.ServeHTTP(bogusResp, bogusReq)

	if bogusResp.Code != http.StatusBadRequest {
		t.Fatalf("bogus token status = %d, want 400", bogusResp.Code)
	}
}

func TestPasswordResetFlowAndSessionInvalidation(t *testing.T) {
	_, _, _, handler := setupTestApp()

	// 1. Register and login
	regBody, _ := json.Marshal(map[string]string{
		"email":    "reset_user@example.com",
		"password": "OldPassword123",
	})
	rReq := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(regBody))
	rReq.Header.Set("Content-Type", "application/json")
	rResp := httptest.NewRecorder()
	handler.ServeHTTP(rResp, rReq)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "reset_user@example.com",
		"password": "OldPassword123",
	})
	lReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(loginBody))
	lReq.Header.Set("Content-Type", "application/json")
	lResp := httptest.NewRecorder()
	handler.ServeHTTP(lResp, lReq)

	var cookie *http.Cookie
	for _, c := range lResp.Result().Cookies() {
		if c.Name == sessions.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("login session cookie missing")
	}

	// 2. Request password reset
	resetReqBody, _ := json.Marshal(map[string]string{"email": "reset_user@example.com"})
	resetReq := httptest.NewRequest(http.MethodPost, "/api/identity/password-reset-requests", bytes.NewReader(resetReqBody))
	resetReq.Header.Set("Content-Type", "application/json")
	resetResp := httptest.NewRecorder()
	handler.ServeHTTP(resetResp, resetReq)

	var resetResult accounts.PasswordResetResponse
	_ = json.Unmarshal(resetResp.Body.Bytes(), &resetResult)
	resetToken := resetResult.ResetToken
	if resetToken == "" {
		t.Fatal("expected reset token in test environment")
	}

	// 3. Complete password reset
	doResetBody, _ := json.Marshal(map[string]string{
		"token":       resetToken,
		"newPassword": "BrandNewPassword123",
	})
	doResetReq := httptest.NewRequest(http.MethodPost, "/api/identity/password-resets", bytes.NewReader(doResetBody))
	doResetReq.Header.Set("Content-Type", "application/json")
	doResetResp := httptest.NewRecorder()
	handler.ServeHTTP(doResetResp, doResetReq)

	if doResetResp.Code != http.StatusOK {
		t.Fatalf("reset password status = %d: %s", doResetResp.Code, doResetResp.Body)
	}

	// 4. Old session must now be invalidated!
	meReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	meReq.AddCookie(cookie)
	meResp := httptest.NewRecorder()
	handler.ServeHTTP(meResp, meReq)

	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session to be unauthorized after password reset, got %d", meResp.Code)
	}

	// 5. Old password rejected, new password accepted
	badLogin, _ := json.Marshal(map[string]string{
		"email":    "reset_user@example.com",
		"password": "OldPassword123",
	})
	blReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(badLogin))
	blReq.Header.Set("Content-Type", "application/json")
	blResp := httptest.NewRecorder()
	handler.ServeHTTP(blResp, blReq)
	if blResp.Code != http.StatusUnauthorized {
		t.Fatalf("old password accepted after reset: %d", blResp.Code)
	}

	goodLogin, _ := json.Marshal(map[string]string{
		"email":    "reset_user@example.com",
		"password": "BrandNewPassword123",
	})
	glReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(goodLogin))
	glReq.Header.Set("Content-Type", "application/json")
	glResp := httptest.NewRecorder()
	handler.ServeHTTP(glResp, glReq)
	if glResp.Code != http.StatusOK {
		t.Fatalf("new password login failed: %d", glResp.Code)
	}
}

func TestLogout(t *testing.T) {
	_, _, _, handler := setupTestApp()

	// Register & login
	regBody, _ := json.Marshal(map[string]string{
		"email":    "logout_user@example.com",
		"password": "Password123!",
	})
	rReq := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(regBody))
	rReq.Header.Set("Content-Type", "application/json")
	rResp := httptest.NewRecorder()
	handler.ServeHTTP(rResp, rReq)

	lReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(regBody))
	lReq.Header.Set("Content-Type", "application/json")
	lResp := httptest.NewRecorder()
	handler.ServeHTTP(lResp, lReq)

	var cookie *http.Cookie
	for _, c := range lResp.Result().Cookies() {
		if c.Name == sessions.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("session cookie missing")
	}

	// Call logout
	logoutReq := httptest.NewRequest(http.MethodDelete, "/api/identity/sessions/current", nil)
	logoutReq.AddCookie(cookie)
	logoutResp := httptest.NewRecorder()
	handler.ServeHTTP(logoutResp, logoutReq)

	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResp.Code)
	}

	// Subsequent /me call with old cookie must fail
	meReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	meReq.AddCookie(cookie)
	meResp := httptest.NewRecorder()
	handler.ServeHTTP(meResp, meReq)

	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("me status after logout = %d, want 401", meResp.Code)
	}
}
