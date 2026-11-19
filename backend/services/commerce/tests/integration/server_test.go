package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/server"
)

func TestHealth(t *testing.T) {
	router := server.New(false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/commerce/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["service"] != "commerce" || result["status"] != "ok" {
		t.Fatalf("unexpected health response: %v", result)
	}
}

func TestSwaggerMatchesAPIRoutes(t *testing.T) {
	router := server.New(true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/commerce/doc.json", nil))
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
		// Ignore internal routes and health from strict public swagger checking if needed, or check api
		if strings.HasPrefix(route.Path, "/api/commerce") && route.Path != "/api/commerce/health" {
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
