package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/server"
)

func TestHealth(t *testing.T) {
	router := server.New(false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/identity/v1/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["service"] != "identity" || result["status"] != "ok" {
		t.Fatalf("unexpected health response: %v", result)
	}
}

func TestSwaggerMatchesAPIRoutes(t *testing.T) {
	router := server.New(true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/identity/doc.json", nil))
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
	if document.Swagger != "2.0" || document.Paths["/api/identity/v1/health"]["get"].ID != "getIdentityHealth" {
		t.Fatalf("unexpected swagger contract: %+v", document)
	}
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/") {
			if _, ok := document.Paths[route.Path][strings.ToLower(route.Method)]; !ok {
				t.Errorf("route missing from Swagger: %s %s", route.Method, route.Path)
			}
		}
	}
	ids := make(map[string]bool)
	for path, methods := range document.Paths {
		for method, operation := range methods {
			if operation.ID == "" || ids[operation.ID] {
				t.Errorf("missing or duplicate operationId for %s %s", method, path)
			}
			ids[operation.ID] = true
		}
	}
}

func TestUnknownRouteAndDisabledSwagger(t *testing.T) {
	router := server.New(false)
	for _, path := range []string{"/unknown", "/swagger/identity/doc.json"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d", path, response.Code)
		}
	}
}
