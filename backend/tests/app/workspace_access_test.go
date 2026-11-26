package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/app"
	"github.com/StephenQiu30/ai2workspace/backend/internal/creation/adapter/http/studio"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/infra/postgres"
)

func TestWorkspaceAccess(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for the PostgreSQL integration test")
	}
	pool, err := postgres.Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	accounts := identity.New(pool, identity.Config{})
	identityKey := time.Now().UTC().Format("20060102150405.000000000")
	email := identityKey + "@example.test"
	const password = "Test-password-123"
	accountID, err := accounts.BootstrapAdmin(t.Context(), email, password)
	if err != nil {
		t.Fatal(err)
	}
	workspaceA, workspaceB := "studio-"+identityKey, "studio-other-"+identityKey
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO workspaces (id) VALUES ($1), ($2)", workspaceA, workspaceB)
	exec("INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $3, 'owner'), ($2, $3, 'member')", workspaceA, workspaceB, accountID)
	defer exec("DELETE FROM workspaces WHERE id IN ($1, $2)", workspaceA, workspaceB)
	defer exec("DELETE FROM accounts WHERE id = $1", accountID)

	router := app.New(pool, accounts, app.Options{
		SwaggerEnabled:        true,
		PlaybackSigningSecret: "test-playback",
		PaymentWebhookSecret:  "test-webhook",
	})
	request := func(method, path string, body []byte, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	for _, path := range []string{"/api/health", "/swagger/doc.json", "/api/learning/courses", "/api/community/posts"} {
		if resp := request(http.MethodGet, path, nil, nil); resp.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", path, resp.Code, resp.Body)
		}
	}
	assertSwaggerRoutes(t, router)

	credentials, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	login := request(http.MethodPost, "/api/identity/sessions", credentials, nil)
	cookies := login.Result().Cookies()
	if login.Code != http.StatusOK || len(cookies) == 0 {
		t.Fatalf("login returned %d: %s", login.Code, login.Body)
	}
	session := cookies[0]
	for _, path := range []string{
		"/api/identity/me",
		"/api/commerce/orders",
		"/api/learning/me/courses",
		"/api/community/notifications",
		"/api/assistant/governance/summary",
	} {
		if resp := request(http.MethodGet, path, nil, session); resp.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", path, resp.Code, resp.Body)
		}
	}
	checkAccess := func(workspace string, cookie *http.Cookie, status int, role string) {
		t.Helper()
		resp := request(http.MethodGet, "/api/studio/workspaces/"+workspace+"/access", nil, cookie)
		if resp.Code != status || resp.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("workspace access returned %d, expected %d with no-store: %s", resp.Code, status, resp.Body)
		}
		if status == http.StatusOK {
			var access studio.AccessResponse
			if err := json.NewDecoder(resp.Body).Decode(&access); err != nil {
				t.Fatal(err)
			}
			if access.WorkspaceID != workspace || access.Role != role {
				t.Fatalf("unexpected workspace access: %+v", access)
			}
		}
	}
	checkAccess(workspaceA, nil, http.StatusUnauthorized, "")
	checkAccess("unrelated-workspace", session, http.StatusForbidden, "")
	checkAccess(workspaceA, session, http.StatusOK, "owner")
	checkAccess(workspaceB, session, http.StatusOK, "member")
	exec("UPDATE workspace_members SET revoked_at = now() WHERE workspace_id = $1 AND user_id = $2", workspaceB, accountID)
	checkAccess(workspaceB, session, http.StatusForbidden, "")
	checkAccess(workspaceA, session, http.StatusOK, "owner")
	if resp := request(http.MethodDelete, "/api/identity/sessions/current", nil, session); resp.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d: %s", resp.Code, resp.Body)
	}
	checkAccess(workspaceA, session, http.StatusUnauthorized, "")
	if resp := request(http.MethodPost, "/internal/identity/sessions/resolve", nil, nil); resp.Code != http.StatusNotFound {
		t.Fatalf("retired HTTP route returned %d", resp.Code)
	}
}
