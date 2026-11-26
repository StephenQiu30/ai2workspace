package tests

import (
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/StephenQiu30/ai2workspace/backend/api"
	"github.com/swaggo/swag"
)

func TestStudioSwaggerRoutes(t *testing.T) {
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
	for path := range contract.Paths {
		if !strings.HasPrefix(path, "/api/studio/") {
			t.Errorf("pilot contract includes a route outside the pilot: %s", path)
		}
	}
	for _, path := range []string{"/api/studio/health", "/api/studio/workspaces/{workspaceId}/access"} {
		if contract.Paths[path]["get"] == nil {
			t.Errorf("missing GET %s", path)
		}
	}
}
