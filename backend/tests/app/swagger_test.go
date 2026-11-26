package app_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	_ "github.com/StephenQiu30/ai2workspace/backend/api"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
)

func assertSwaggerRoutes(t *testing.T, router *gin.Engine) {
	t.Helper()
	doc, err := swag.ReadDoc()
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]json.RawMessage
	}
	if err := json.Unmarshal([]byte(doc), &contract); err != nil {
		t.Fatal(err)
	}
	params := regexp.MustCompile(`[:*]([^/]+)`)
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/") {
			continue
		}
		path := params.ReplaceAllString(route.Path, "{$1}")
		method := strings.ToLower(route.Method)
		if contract.Paths[path][method] == nil {
			t.Errorf("missing Swagger operation for %s %s", route.Method, path)
		}
		delete(contract.Paths[path], method)
	}
	for path, operations := range contract.Paths {
		for method := range operations {
			t.Errorf("Swagger operation has no route: %s %s", method, path)
		}
	}
}
