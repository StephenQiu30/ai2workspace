package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/platform"
	"github.com/StephenQiu30/ai2workspace/backend/internal/server"
)

func TestWorkspaceAccess(t *testing.T) {
	dsn := os.Getenv("STUDIO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("STUDIO_TEST_DATABASE_URL is required for the PostgreSQL integration test")
	}
	db, pool, err := platform.OpenDatabase(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	workspaceA := t.Name() + time.Now().UTC().Format(time.RFC3339Nano)
	workspaceB := workspaceA + "-other"
	tx := db.WithContext(t.Context()).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.Exec("INSERT INTO identity.workspaces (id) VALUES (?), (?)", workspaceA, workspaceB).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`INSERT INTO identity.workspace_members (workspace_id, user_id, role)
		VALUES (?, 'maker', 'owner'), (?, 'maker', 'member')`, workspaceA, workspaceB).Error; err != nil {
		t.Fatal(err)
	}

	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/identity/me" {
			http.NotFound(w, r)
			return
		}
		cookie, err := r.Cookie("ai2_session")
		if err != nil || cookie.Value == "expired" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if cookie.Value == "unavailable" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"userId": cookie.Value, "accountStatus": "active"})
	}))
	t.Cleanup(legacy.Close)
	session, err := identity.NewSessionClient(legacy.URL)
	if err != nil {
		t.Fatal(err)
	}
	router := server.NewRouter(identity.NewHandler(tx, session), false)

	for _, tt := range []struct {
		name, workspace, cookie, role string
		status                        int
	}{
		{"owner", workspaceA, "maker", "owner", http.StatusOK},
		{"member", workspaceB, "maker", "member", http.StatusOK},
		{"foreign workspace", "unknown", "maker", "", http.StatusForbidden},
		{"nonmember", workspaceA, "visitor", "", http.StatusForbidden},
		{"no session", workspaceA, "", "", http.StatusUnauthorized},
		{"expired session", workspaceA, "expired", "", http.StatusUnauthorized},
		{"unavailable identity", workspaceA, "unavailable", "", http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checkAccess(t, router, tt.workspace, tt.cookie, tt.role, tt.status)
		})
	}

	if err := tx.Exec("UPDATE identity.workspace_members SET revoked_at = now() WHERE workspace_id = ? AND user_id = 'maker'", workspaceB).Error; err != nil {
		t.Fatal(err)
	}
	checkAccess(t, router, workspaceB, "maker", "", http.StatusForbidden)
	checkAccess(t, router, workspaceA, "maker", "owner", http.StatusOK)
}

func checkAccess(t *testing.T, router http.Handler, workspace, cookie, role string, status int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/studio/workspaces/"+workspace+"/access", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "ai2_session", Value: cookie})
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != status || resp.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("got %d %s, want %d and no-store", resp.Code, resp.Body.String(), status)
	}
	if status == http.StatusOK {
		var access identity.AccessResponse
		if err := json.NewDecoder(resp.Body).Decode(&access); err != nil {
			t.Fatal(err)
		}
		if access.WorkspaceID != workspace || access.Role != role {
			t.Fatalf("got %+v, want workspace %q and role %q", access, workspace, role)
		}
	}
}
