package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/server"
)

func TestHealth(t *testing.T) {
	router := server.New(false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/community/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["service"] != "community" || result["status"] != "ok" {
		t.Fatalf("unexpected health response: %v", result)
	}
}

func TestSwaggerMatchesAPIRoutes(t *testing.T) {
	router := server.New(true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/community/doc.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("swagger status = %d: %s", response.Code, response.Body)
	}
	var document struct {
		Swagger string `json:"swagger"`
		Paths   map[string]map[string]struct {
			ID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Swagger != "2.0" {
		t.Fatalf("unexpected swagger version: %+v", document)
	}
	paramRe := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/community") || strings.HasPrefix(route.Path, "/internal/community") {
			swaggerPath := paramRe.ReplaceAllString(route.Path, "{$1}")
			if _, ok := document.Paths[swaggerPath][strings.ToLower(route.Method)]; !ok {
				t.Errorf("route missing from Swagger: %s %s (checked %s)", route.Method, route.Path, swaggerPath)
			}
		}
	}
	ids := make(map[string]bool)
	for path, methods := range document.Paths {
		for method, operation := range methods {
			if operation.ID == "" || ids[operation.ID] {
				t.Errorf("missing or duplicate operationId for %s %s (%s)", method, path, operation.ID)
			}
			ids[operation.ID] = true
		}
	}
}

func TestCrossServiceIdentityAndLearningIntegration(t *testing.T) {
	const expectedToken = "shared-internal-token"

	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/identity/sessions/resolve" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get(auth.ServiceTokenHeader) != expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			OpaqueSession string `json:"opaqueSession"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.OpaqueSession != "valid_sess_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(auth.Actor{
			UserID:        "usr_enrolled",
			Nickname:      "已购学员",
			Email:         "enrolled@example.com",
			EmailVerified: true,
			Roles:         []string{"student"},
			AccountStatus: "active",
		})
	}))
	defer identitySrv.Close()

	learningSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/learning/course-access/check" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get(auth.ServiceTokenHeader) != expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			UserID   string `json:"userId"`
			CourseID string `json:"courseId"`
			Purpose  string `json:"purpose"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Purpose != "discussion" {
			t.Errorf("expected purpose=discussion, got %q", req.Purpose)
		}
		allowed := req.UserID == "usr_enrolled" && req.CourseID == "crs_go_101"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"allowed":        allowed,
			"reason":         "checked",
			"accessVersion":  1,
			"contentVersion": 1,
		})
	}))
	defer learningSrv.Close()

	router, cleanup, err := server.NewWithConfig(context.Background(), &config.Config{
		HTTPTimeout:        5 * time.Second,
		SwaggerEnabled:     false,
		ServiceToken:       expectedToken,
		IdentityServiceURL: identitySrv.URL,
		LearningServiceURL: learningSrv.URL,
	})
	if err != nil {
		t.Fatalf("NewWithConfig failed: %v", err)
	}
	defer cleanup()

	// 1. Authorized student can create a course discussion post
	createPayload, _ := json.Marshal(posts.CreatePostRequest{
		Scope:         "course",
		CourseID:      "crs_go_101",
		Topic:         "course-qa",
		Title:         "并发通道最佳实践",
		PublicSummary: "探讨 Go channel 的关闭责任与取消模式。",
		Body:          "受课程权限保护的详细讨论正文。",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/community/posts", bytes.NewReader(createPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "valid_sess_token"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for enrolled student course post, got %d: %s", w.Code, w.Body.String())
	}

	var created posts.PostResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.AuthorNickname != "已购学员" || created.Topic != "course-qa" {
		t.Fatalf("unexpected created post metadata: %+v", created)
	}

	// 2. Denied course returns 403
	deniedPayload, _ := json.Marshal(posts.CreatePostRequest{
		Scope:         "course",
		CourseID:      "crs_other_999",
		Title:         "未购课程提问",
		PublicSummary: "公开摘要",
		Body:          "正文",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/community/posts", bytes.NewReader(deniedPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "valid_sess_token"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized course, got %d: %s", w.Code, w.Body.String())
	}
}
