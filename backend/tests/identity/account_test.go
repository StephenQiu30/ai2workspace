package identity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	accountshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/accounts"
	sessionshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/sessions"
	accessapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/access"
	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	sessionsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/sessions"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	"github.com/gin-gonic/gin"
)

type failingMailer struct {
	err error
}

func (f failingMailer) Send(ctx context.Context, msg accountsapp.MailMessage) error {
	return f.err
}

func setupTestApp() (*accountsapp.MemoryStore, *sessionsapp.MemoryStore, *accessapp.MemoryStore, http.Handler) {
	return setupTestAppWithOptions(identity.Options{})
}

func setupTestAppWithOptions(opts identity.Options) (*accountsapp.MemoryStore, *sessionsapp.MemoryStore, *accessapp.MemoryStore, http.Handler) {
	accStore := accountsapp.NewMemoryStore()
	sessStore := sessionsapp.NewMemoryStore()
	invStore := accessapp.NewMemoryStore()
	handler := gin.New()
	identity.NewWithStores(accStore, sessStore, invStore, opts).RegisterRoutes(handler)
	return accStore, sessStore, invStore, handler
}

func latestMailToken(t *testing.T, accStore accountsapp.Store, email string, purpose accountsdomain.TokenPurpose) string {
	t.Helper()
	tasks := accStore.GetRecentMailTasks(t.Context(), email)
	for i := len(tasks) - 1; i >= 0; i-- {
		if tasks[i].Purpose == purpose && tasks[i].Token != "" {
			return tasks[i].Token
		}
	}
	t.Fatalf("no mail task found for %s with purpose %s", email, purpose)
	return ""
}

func TestRegisterDuplicateEmailAndNoTokenLeakByDefault(t *testing.T) {
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

	var firstResult accountshttp.RegisterResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &firstResult); err != nil {
		t.Fatal(err)
	}
	if !firstResult.Accepted || firstResult.NextStep != "verify_email" {
		t.Fatalf("unexpected first register response: %+v", firstResult)
	}
	if firstResult.VerificationToken != "" {
		t.Fatalf("verificationToken leaked in default mode: %q", firstResult.VerificationToken)
	}

	// Second registration with duplicate email must match exact structure & status code
	reqDup := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	respDup := httptest.NewRecorder()
	handler.ServeHTTP(respDup, reqDup)

	if respDup.Code != resp.Code {
		t.Fatalf("duplicate register status = %d, want %d", respDup.Code, resp.Code)
	}
	if respDup.Body.String() != resp.Body.String() {
		t.Fatalf("duplicate response %s differs from first response %s", respDup.Body.String(), resp.Body.String())
	}

	if count := accStore.CountAccounts(req.Context()); count != 1 {
		t.Fatalf("expected 1 account, got %d", count)
	}

	// Mail task should be tracked internally with status sent and token available for ops/tests
	tasks := accStore.GetRecentMailTasks(t.Context(), "user@example.com")
	if len(tasks) != 1 || tasks[0].Token == "" || tasks[0].Status != "sent" {
		t.Fatalf("unexpected mail tasks: %+v", tasks)
	}
}

func TestDevExposeTokensFlagControlsResponseTokens(t *testing.T) {
	_, _, _, handler := setupTestAppWithOptions(identity.Options{
		DevExposeTokens: true,
	})

	body, _ := json.Marshal(map[string]string{
		"email":    "devmode@example.com",
		"password": "Password123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	var regResult accountshttp.RegisterResponse
	_ = json.Unmarshal(resp.Body.Bytes(), &regResult)
	if regResult.VerificationToken == "" {
		t.Fatal("expected verificationToken when DevExposeTokens=true")
	}

	// Password reset also returns token in dev mode
	resetBody, _ := json.Marshal(map[string]string{"email": "devmode@example.com"})
	resetReq := httptest.NewRequest(http.MethodPost, "/api/identity/password-reset-requests", bytes.NewReader(resetBody))
	resetReq.Header.Set("Content-Type", "application/json")
	resetResp := httptest.NewRecorder()
	handler.ServeHTTP(resetResp, resetReq)

	var resetResult accountshttp.PasswordResetResponse
	_ = json.Unmarshal(resetResp.Body.Bytes(), &resetResult)
	if resetResult.ResetToken == "" {
		t.Fatal("expected resetToken when DevExposeTokens=true")
	}
}

func TestMailDeliveryFailureReturnsErrorAndMarksTaskFailed(t *testing.T) {
	accStore, _, _, handler := setupTestAppWithOptions(identity.Options{
		Mailer: failingMailer{err: errors.New("smtp unreachable")},
	})

	body, _ := json.Marshal(map[string]string{
		"email":    "smtp_fail@example.com",
		"password": "Password123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on SMTP failure, got %d: %s", resp.Code, resp.Body)
	}

	// Account should still exist (not rolled back) and stay unverified
	acc, err := accStore.GetAccountByEmail(t.Context(), "smtp_fail@example.com")
	if err != nil {
		t.Fatalf("expected account to remain created after SMTP failure: %v", err)
	}
	if acc.EmailVerified {
		t.Fatal("account must not be marked verified when mail delivery failed")
	}

	tasks := accStore.GetRecentMailTasks(t.Context(), "smtp_fail@example.com")
	if len(tasks) != 1 || tasks[0].Status != "failed" {
		t.Fatalf("expected failed mail task record, got %+v", tasks)
	}
}

func TestLoginRejectsWrongPasswordAndSetsSecureCookieAttributes(t *testing.T) {
	_, _, _, handler := setupTestAppWithOptions(identity.Options{
		CookieSecure: true,
	})

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

	// Wrong password
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
		if cookie.Name == sessionsapp.SessionCookieName && cookie.Value != "" {
			t.Fatalf("session cookie issued on wrong password: %v", cookie)
		}
	}

	// Correct password
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
	var sessionCookie *http.Cookie
	for _, cookie := range correctResp.Result().Cookies() {
		if cookie.Name == sessionsapp.SessionCookieName && cookie.Value != "" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not found on successful login")
	}
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie security flags mismatch: HttpOnly=%v Secure=%v SameSite=%v", sessionCookie.HttpOnly, sessionCookie.Secure, sessionCookie.SameSite)
	}

	// Logout clears cookie with matching security flags
	logoutReq := httptest.NewRequest(http.MethodDelete, "/api/identity/sessions/current", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutResp := httptest.NewRecorder()
	handler.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResp.Code)
	}
	var clearedCookie *http.Cookie
	for _, cookie := range logoutResp.Result().Cookies() {
		if cookie.Name == sessionsapp.SessionCookieName {
			clearedCookie = cookie
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 || !clearedCookie.HttpOnly || !clearedCookie.Secure || clearedCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected logout cookie: %+v", clearedCookie)
	}
}

func TestNicknameDefaultAndUpdateMe(t *testing.T) {
	_, _, _, handler := setupTestApp()

	// Unauthenticated PATCH /api/identity/me rejected
	unauthReq := httptest.NewRequest(http.MethodPatch, "/api/identity/me", bytes.NewReader([]byte(`{"nickname":"Bob"}`)))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthResp := httptest.NewRecorder()
	handler.ServeHTTP(unauthResp, unauthReq)
	if unauthResp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated PATCH /me status = %d, want 401", unauthResp.Code)
	}

	// Register without nickname -> defaults to email prefix
	regBody, _ := json.Marshal(map[string]string{
		"email":    "alice.smith@example.com",
		"password": "Password123!",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp := httptest.NewRecorder()
	handler.ServeHTTP(regResp, regReq)
	if regResp.Code != http.StatusOK {
		t.Fatalf("register status = %d", regResp.Code)
	}

	// Login
	loginReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(regBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	handler.ServeHTTP(loginResp, loginReq)
	var cookie *http.Cookie
	for _, c := range loginResp.Result().Cookies() {
		if c.Name == sessionsapp.SessionCookieName {
			cookie = c
		}
	}

	// GET /api/identity/me returns id, email, nickname, emailVerified, status, roles, version
	meReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	meReq.AddCookie(cookie)
	meResp := httptest.NewRecorder()
	handler.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("GET /me status = %d: %s", meResp.Code, meResp.Body)
	}
	var meResult sessionshttp.CurrentUserResponse
	if err := json.Unmarshal(meResp.Body.Bytes(), &meResult); err != nil {
		t.Fatal(err)
	}
	if meResult.ID == "" || meResult.Nickname != "alice.smith" || meResult.Status != "active" || meResult.Version != 1 {
		t.Fatalf("unexpected /me response: %+v", meResult)
	}

	// PATCH /api/identity/me with whitespace-only nickname -> 400
	badPatchReq := httptest.NewRequest(http.MethodPatch, "/api/identity/me", bytes.NewReader([]byte(`{"nickname":"   "}`)))
	badPatchReq.Header.Set("Content-Type", "application/json")
	badPatchReq.AddCookie(cookie)
	badPatchResp := httptest.NewRecorder()
	handler.ServeHTTP(badPatchResp, badPatchReq)
	if badPatchResp.Code != http.StatusBadRequest {
		t.Fatalf("whitespace nickname status = %d, want 400", badPatchResp.Code)
	}

	// PATCH /api/identity/me updates nickname and increments version
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/identity/me", bytes.NewReader([]byte(`{"nickname":"Alice Chen"}`)))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.AddCookie(cookie)
	patchResp := httptest.NewRecorder()
	handler.ServeHTTP(patchResp, patchReq)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("PATCH /me status = %d: %s", patchResp.Code, patchResp.Body)
	}
	var patched accountshttp.ProfileResponse
	if err := json.Unmarshal(patchResp.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Nickname != "Alice Chen" || patched.Version != 2 {
		t.Fatalf("unexpected patched profile: %+v", patched)
	}

	updatedMeReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	updatedMeReq.AddCookie(cookie)
	updatedMeResp := httptest.NewRecorder()
	handler.ServeHTTP(updatedMeResp, updatedMeReq)
	var resolved sessionshttp.CurrentUserResponse
	if err := json.Unmarshal(updatedMeResp.Body.Bytes(), &resolved); err != nil {
		t.Fatal(err)
	}
	if resolved.Nickname != "Alice Chen" {
		t.Fatalf("expected resolved nickname Alice Chen, got %+v", resolved)
	}
}

func TestResendEmailVerificationRejectsSuspendedAndHidesToken(t *testing.T) {
	accStore, sessStore, _, handler := setupTestApp()

	acc, _, err := accStore.CreateAccount(t.Context(), "resend_user@example.com", "Password123!", "ResendUser")
	if err != nil {
		t.Fatal(err)
	}
	rawSess, _, err := sessStore.CreateSession(t.Context(), acc.ID, sessionsapp.DefaultSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionsapp.SessionCookieName, Value: rawSess}

	// Active unverified user resend succeeds without exposing token
	req := httptest.NewRequest(http.MethodPost, "/api/identity/email-verification-requests", nil)
	req.AddCookie(cookie)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("resend status = %d: %s", resp.Code, resp.Body)
	}
	var resendRes accountshttp.ResendVerificationResponse
	_ = json.Unmarshal(resp.Body.Bytes(), &resendRes)
	if !resendRes.Accepted || resendRes.VerificationToken != "" {
		t.Fatalf("unexpected resend response: %+v", resendRes)
	}

	// Suspend account directly while keeping session token intact -> resend must be 403
	if _, err := accStore.SetAccountStatus(t.Context(), acc.ID, accountsdomain.StatusSuspended, acc.Version); err != nil {
		t.Fatal(err)
	}
	suspReq := httptest.NewRequest(http.MethodPost, "/api/identity/email-verification-requests", nil)
	suspReq.AddCookie(cookie)
	suspResp := httptest.NewRecorder()
	handler.ServeHTTP(suspResp, suspReq)
	if suspResp.Code != http.StatusForbidden {
		t.Fatalf("suspended resend status = %d, want 403", suspResp.Code)
	}
}

func TestRateLimitingOnAuthAndMailEndpoints(t *testing.T) {
	accStore, sessStore, _, handler := setupTestAppWithOptions(identity.Options{
		AuthLimiter: accountshttp.NewRateLimiter(2, time.Minute),
		MailLimiter: accountshttp.NewRateLimiter(1, time.Minute),
	})

	// Register limit: 2 allowed, 3rd returns 429
	for i := range 3 {
		body, _ := json.Marshal(map[string]string{
			"email":    "rl" + string(rune('a'+i)) + "@example.com",
			"password": "Password123!",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.1:12345"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if i < 2 && resp.Code != http.StatusOK {
			t.Fatalf("register #%d status = %d, want 200", i, resp.Code)
		}
		if i == 2 {
			if resp.Code != http.StatusTooManyRequests {
				t.Fatalf("register #3 status = %d, want 429", resp.Code)
			}
			if resp.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After header on 429")
			}
		}
	}

	// Login limit on another IP: 2 attempts allowed, 3rd returns 429
	for i := range 3 {
		body, _ := json.Marshal(map[string]string{
			"email":    "rla@example.com",
			"password": "WrongPassword",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.2:12345"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if i < 2 && resp.Code != http.StatusUnauthorized {
			t.Fatalf("login #%d status = %d, want 401", i, resp.Code)
		}
		if i == 2 && resp.Code != http.StatusTooManyRequests {
			t.Fatalf("login #3 status = %d, want 429", resp.Code)
		}
	}

	// Password reset limit: 1 allowed, 2nd returns 429
	for i := range 2 {
		body, _ := json.Marshal(map[string]string{"email": "rla@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/api/identity/password-reset-requests", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.3:12345"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if i == 0 && resp.Code != http.StatusOK {
			t.Fatalf("pwdreset #1 status = %d, want 200", resp.Code)
		}
		if i == 1 && resp.Code != http.StatusTooManyRequests {
			t.Fatalf("pwdreset #2 status = %d, want 429", resp.Code)
		}
	}

	// Resend verification limit: 1 allowed, 2nd returns 429
	acc, err := accStore.GetAccountByEmail(t.Context(), "rla@example.com")
	if err != nil {
		t.Fatal(err)
	}
	rawSess, _, _ := sessStore.CreateSession(t.Context(), acc.ID, sessionsapp.DefaultSessionTTL)
	cookie := &http.Cookie{Name: sessionsapp.SessionCookieName, Value: rawSess}
	for i := range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/identity/email-verification-requests", nil)
		req.AddCookie(cookie)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if i == 0 && resp.Code != http.StatusOK {
			t.Fatalf("resend #1 status = %d, want 200", resp.Code)
		}
		if i == 1 && resp.Code != http.StatusTooManyRequests {
			t.Fatalf("resend #2 status = %d, want 429", resp.Code)
		}
	}
}

func TestBootstrapAdminIdempotentAndLogin(t *testing.T) {
	accStore, _, _, handler := setupTestApp()

	first, err := accStore.EnsureBootstrapAdmin(t.Context(), "Admin@AI2Workspace.local", "AdminSecret123")
	if err != nil {
		t.Fatal(err)
	}
	second, err := accStore.EnsureBootstrapAdmin(t.Context(), "admin@ai2workspace.local", "AdminSecret123")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || !second.EmailVerified || accStore.CountAccounts(t.Context()) != 1 {
		t.Fatalf("bootstrap admin not idempotent: first=%+v second=%+v count=%d", first, second, accStore.CountAccounts(t.Context()))
	}

	// Bootstrap admin can log in
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "admin@ai2workspace.local",
		"password": "AdminSecret123",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/identity/sessions", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	handler.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("bootstrap admin login status = %d: %s", loginResp.Code, loginResp.Body)
	}
}

func TestAccountTokenSingleUse(t *testing.T) {
	accStore, _, _, handler := setupTestApp()

	regBody, _ := json.Marshal(map[string]string{
		"email":    "token_test@example.com",
		"password": "StrongPassword123",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/identity/accounts", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp := httptest.NewRecorder()
	handler.ServeHTTP(regResp, regReq)

	token := latestMailToken(t, accStore, "token_test@example.com", accountsdomain.PurposeEmailVerification)

	// Verify email first time
	vBody, _ := json.Marshal(map[string]string{"token": token})
	vReq := httptest.NewRequest(http.MethodPost, "/api/identity/email-verifications", bytes.NewReader(vBody))
	vReq.Header.Set("Content-Type", "application/json")
	vResp := httptest.NewRecorder()
	handler.ServeHTTP(vResp, vReq)

	if vResp.Code != http.StatusOK {
		t.Fatalf("verify email status = %d: %s", vResp.Code, vResp.Body)
	}
	var vResult accountshttp.VerifyEmailResponse
	_ = json.Unmarshal(vResp.Body.Bytes(), &vResult)
	if !vResult.EmailVerified {
		t.Fatal("email should be marked verified")
	}

	// Re-verify with same token is idempotent
	vReqDup := httptest.NewRequest(http.MethodPost, "/api/identity/email-verifications", bytes.NewReader(vBody))
	vReqDup.Header.Set("Content-Type", "application/json")
	vRespDup := httptest.NewRecorder()
	handler.ServeHTTP(vRespDup, vReqDup)

	if vRespDup.Code != http.StatusOK {
		t.Fatalf("duplicate verify email status = %d: %s", vRespDup.Code, vRespDup.Body)
	}

	// Bogus token rejected
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
	accStore, _, _, handler := setupTestApp()

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
		if c.Name == sessionsapp.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("login session cookie missing")
	}

	// 2. Request password reset (token hidden in JSON, tracked in mail_tasks)
	resetReqBody, _ := json.Marshal(map[string]string{"email": "reset_user@example.com"})
	resetReq := httptest.NewRequest(http.MethodPost, "/api/identity/password-reset-requests", bytes.NewReader(resetReqBody))
	resetReq.Header.Set("Content-Type", "application/json")
	resetResp := httptest.NewRecorder()
	handler.ServeHTTP(resetResp, resetReq)

	var resetResult accountshttp.PasswordResetResponse
	_ = json.Unmarshal(resetResp.Body.Bytes(), &resetResult)
	if !resetResult.Accepted || resetResult.ResetToken != "" {
		t.Fatalf("expected accepted response without exposed resetToken, got %+v", resetResult)
	}
	resetToken := latestMailToken(t, accStore, "reset_user@example.com", accountsdomain.PurposePasswordReset)

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

	// 4. Old session must now be invalidated
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
		if c.Name == sessionsapp.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("session cookie missing")
	}

	logoutReq := httptest.NewRequest(http.MethodDelete, "/api/identity/sessions/current", nil)
	logoutReq.AddCookie(cookie)
	logoutResp := httptest.NewRecorder()
	handler.ServeHTTP(logoutResp, logoutReq)

	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResp.Code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/identity/me", nil)
	meReq.AddCookie(cookie)
	meResp := httptest.NewRecorder()
	handler.ServeHTTP(meResp, meReq)

	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("me status after logout = %d, want 401", meResp.Code)
	}
}
