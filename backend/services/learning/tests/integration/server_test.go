package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/media"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/server"
)

func TestHealth(t *testing.T) {
	router := server.New(false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/learning/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["service"] != "learning" || result["status"] != "ok" {
		t.Fatalf("unexpected health response: %v", result)
	}
}

func TestSwaggerMatchesAPIRoutes(t *testing.T) {
	router := server.New(true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/learning/doc.json", nil))
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
	if document.Swagger != "2.0" || document.Paths["/api/learning/health"]["get"].ID != "getLearningHealth" {
		t.Fatalf("unexpected swagger contract: %+v", document)
	}
	paramRe := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/") || strings.HasPrefix(route.Path, "/internal/") {
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
				t.Errorf("missing or duplicate operationId for %s %s: %q", method, path, operation.ID)
			}
			ids[operation.ID] = true
		}
	}
}

func TestUnknownRouteAndDisabledSwagger(t *testing.T) {
	router := server.New(false)
	for _, path := range []string{"/unknown", "/swagger/learning/doc.json"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d", path, response.Code)
		}
	}
}

func TestDefaultServerRejectsPlaintextIdentityHeaders(t *testing.T) {
	router := server.New(false)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"create course", http.MethodPost, "/api/learning/courses", `{"title":"Hack"}`},
		{"manage courses", http.MethodGet, "/api/learning/manage/courses", ""},
		{"my courses", http.MethodGet, "/api/learning/me/courses", ""},
		{"create upload", http.MethodPost, "/api/learning/lessons/lsn_1/uploads", `{"courseId":"crs_1","fileName":"a.mp4","declaredBytes":100}`},
		{"save progress", http.MethodPut, "/api/learning/lessons/lsn_1/progress", `{"courseId":"crs_1","contentVersion":1,"positionSeconds":10}`},
		{"get progress", http.MethodGet, "/api/learning/lessons/lsn_1/progress", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("X-User-ID", "spoofed_admin")
			req.Header.Set("X-Actor-Id", "spoofed_admin")
			req.Header.Set("X-Actor-Roles", "admin")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 when only plaintext headers provided, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestIdentityAndCommerceHTTPClients(t *testing.T) {
	const svcToken = "shared-prod-secret"

	// Mock identity server
	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/identity/sessions/resolve" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Service-Token") != svcToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			OpaqueSession string `json:"opaqueSession"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body.OpaqueSession {
		case "valid-buyer-cookie":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"userId":         "buyer_99",
				"email":          "buyer@example.com",
				"nickname":       "付费学员",
				"emailVerified":  true,
				"roles":          []string{"student"},
				"accountStatus":  "active",
				"sessionVersion": 1,
			})
		case "suspended-cookie":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer identitySrv.Close()

	coursesStore := courses.NewMemoryStore()
	paidCourse, err := coursesStore.CreateCourse(t.Context(), "inst_1", "张讲师", "Go 微服务架构", "生产实践", "", "engineering", false)
	if err != nil {
		t.Fatal(err)
	}
	sec, _ := coursesStore.AddSection(t.Context(), paidCourse.ID, "第一章", 1)
	lsn, _ := coursesStore.AddLesson(t.Context(), paidCourse.ID, sec.ID, "核心图文", "article", false, "VIP 正文内容", "", 1)
	// Submit & approve
	cCur, _ := coursesStore.GetCourse(t.Context(), paidCourse.ID)
	cSub, _ := coursesStore.SubmitCourse(t.Context(), paidCourse.ID, "inst_1", false, cCur.Version)
	_, _ = coursesStore.ReviewCourse(t.Context(), paidCourse.ID, "approved", "通过", cSub.Version)

	// Mock commerce server
	commerceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Service-Token") != svcToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/internal/commerce/entitlements/check" && r.Method == http.MethodPost {
			var req struct {
				UserID   string `json:"userId"`
				CourseID string `json:"courseId"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			allowed := req.UserID == "buyer_99" && req.CourseID == paidCourse.ID
			_ = json.NewEncoder(w).Encode(map[string]any{
				"allowed":      allowed,
				"grantVersion": 1,
				"reason":       "active_grant",
			})
			return
		}
		if r.URL.Path == "/internal/commerce/users/buyer_99/entitlements" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":          "ent_1",
					"userId":      "buyer_99",
					"courseId":    paidCourse.ID,
					"orderId":     "ord_1",
					"status":      "active",
					"activatedAt": time.Now().UTC().Format(time.RFC3339),
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer commerceSrv.Close()

	cfg := config.Config{
		HTTPTimeout:           5 * time.Second,
		ServiceToken:          svcToken,
		IdentityServiceURL:    identitySrv.URL,
		CommerceServiceURL:    commerceSrv.URL,
		PlaybackSigningSecret: svcToken,
	}
	router := server.NewFromConfig(cfg, coursesStore, media.NewMemoryStore(), progress.NewMemoryStore())

	// 1. Valid cookie resolves via identity -> commerce grants access -> sees protected article content!
	req := httptest.NewRequest(http.MethodGet, "/api/learning/courses/"+paidCourse.ID, nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "valid-buyer-cookie"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for entitled buyer, got %d: %s", w.Code, w.Body.String())
	}
	var detail courses.CourseResponse
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if len(detail.Sections) != 1 || len(detail.Sections[0].Lessons) != 1 || detail.Sections[0].Lessons[0].Content != "VIP 正文内容" {
		t.Fatalf("expected protected content visible to buyer, got %+v", detail.Sections)
	}

	// 2. /api/learning/me/courses merges purchased course from commerce
	myReq := httptest.NewRequest(http.MethodGet, "/api/learning/me/courses", nil)
	myReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "valid-buyer-cookie"})
	myW := httptest.NewRecorder()
	router.ServeHTTP(myW, myReq)
	if myW.Code != http.StatusOK {
		t.Fatalf("me/courses status = %d: %s", myW.Code, myW.Body.String())
	}
	var myList []courses.MyCourseResponse
	_ = json.Unmarshal(myW.Body.Bytes(), &myList)
	if len(myList) != 1 || myList[0].ID != paidCourse.ID || myList[0].EnrollmentType != "purchased" {
		t.Fatalf("expected purchased course in me/courses, got %+v", myList)
	}

	// 3. Suspended cookie rejected with 403 on protected progress endpoint
	progReq := httptest.NewRequest(http.MethodGet, "/api/learning/lessons/"+lsn.ID+"/progress", nil)
	progReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "suspended-cookie"})
	progW := httptest.NewRecorder()
	router.ServeHTTP(progW, progReq)
	if progW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for suspended cookie, got %d", progW.Code)
	}
}
