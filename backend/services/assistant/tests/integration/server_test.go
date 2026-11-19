package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/server"
)

func TestServerEndpoints(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	srv := server.New(cfg)
	router := srv.Router()

	// 1. Health check
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// 2. Governance summary
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/assistant/governance/summary", nil)
	req.Header.Set("X-User-ID", "usr_test")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for governance summary, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Models catalog
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/assistant/models", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for models list, got %d", w.Code)
	}

	// 4. Create Session & Ask
	sessBody, _ := json.Marshal(map[string]any{
		"agentSlug": "customer_support",
		"title":     "测试会话",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/assistant/sessions", bytes.NewReader(sessBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "usr_test")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for session create, got %d: %s", w.Code, w.Body.String())
	}

	var sessResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &sessResp)

	// Ask question
	askBody, _ := json.Marshal(map[string]any{
		"question": "平台支持哪些支付方式？",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/assistant/sessions/"+sessResp.ID+"/ask", bytes.NewReader(askBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "usr_test")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for ask question, got %d: %s", w.Code, w.Body.String())
	}
}
