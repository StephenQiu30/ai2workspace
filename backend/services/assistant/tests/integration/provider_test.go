package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/provider"
)

func TestProvider_FourProvidersRealHTTPChatCompletions(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		providerID string
		modelID    string
		apiKey     string
	}{
		{"deepseek", "deepseek-chat", "ds-secret"},
		{"openrouter", "anthropic/claude-3.5-haiku", "or-secret"},
		{"volcengine", "doubao-pro-32k", "vc-secret"},
		{"openai", "gpt-4o-mini", "oa-secret"},
	}

	for _, tc := range cases {
		t.Run(tc.providerID, func(t *testing.T) {
			var gotAuth, gotModel string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				gotAuth = r.Header.Get("Authorization")
				var payload struct {
					Model string `json:"model"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				gotModel = payload.Model

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "chatcmpl_" + tc.providerID,
					"choices": []map[string]any{
						{"message": map[string]any{"role": "assistant", "content": "real reply from " + tc.providerID}},
					},
					"usage": map[string]any{
						"prompt_tokens":     1200,
						"completion_tokens": 300,
						"total_tokens":      1500,
					},
				})
			}))
			defer srv.Close()

			cli := provider.NewClientWithOptions(provider.ClientOptions{
				Endpoints: map[string]config.ProviderEndpoint{
					tc.providerID: {APIKey: tc.apiKey, BaseURL: srv.URL},
				},
				AllowLocalSimulation: false,
			})

			res, err := cli.Generate(ctx, &provider.GenerateRequest{
				ProviderID: tc.providerID,
				ModelID:    tc.modelID,
				Messages:   []provider.ModelMessage{{Role: "user", Content: "你好"}},
				MaxTokens:  1000,
			})
			if err != nil {
				t.Fatalf("generate failed: %v", err)
			}
			if gotAuth != "Bearer "+tc.apiKey {
				t.Fatalf("expected Bearer auth header, got %q", gotAuth)
			}
			if gotModel != tc.modelID {
				t.Fatalf("expected model %q, got %q", tc.modelID, gotModel)
			}
			if res.Simulated || res.PromptTokens != 1200 || res.CompletionTokens != 300 || res.CostCents <= 0 {
				t.Fatalf("unexpected real response metadata: %+v", res)
			}
		})
	}
}

func TestProvider_UnconfiguredRejectsWhenSimulationDisabledAndPriceExpiration(t *testing.T) {
	ctx := context.Background()

	// Simulation disabled -> ErrProviderNotConfigured
	cli := provider.NewClientWithOptions(provider.ClientOptions{
		AllowLocalSimulation: false,
	})
	_, err := cli.Generate(ctx, &provider.GenerateRequest{
		ProviderID: "deepseek",
		ModelID:    "deepseek-chat",
		Messages:   []provider.ModelMessage{{Role: "user", Content: "hi"}},
	})
	if !errors.Is(err, provider.ErrProviderNotConfigured) {
		t.Fatalf("expected ErrProviderNotConfigured, got %v", err)
	}

	// Simulation enabled -> clearly marked simulated response
	simCli := provider.NewClientWithOptions(provider.ClientOptions{
		AllowLocalSimulation: true,
	})
	simRes, err := simCli.Generate(ctx, &provider.GenerateRequest{
		ProviderID: "deepseek",
		ModelID:    "deepseek-chat",
		Messages:   []provider.ModelMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil || !simRes.Simulated || !strings.Contains(simRes.Content, "本地仿真响应") {
		t.Fatalf("expected explicitly marked simulated response, got %+v err=%v", simRes, err)
	}

	// Expire model pricing -> ValidateModelForCall & Generate reject with ErrModelPriceExpired
	expiredAt := time.Now().Add(-1 * time.Hour)
	if _, err := simCli.UpdateModel(ctx, "deepseek", "deepseek-chat", provider.ModelUpdate{
		ExpiresAt: &expiredAt,
	}); err != nil {
		t.Fatalf("update model failed: %v", err)
	}
	if _, err := simCli.Generate(ctx, &provider.GenerateRequest{
		ProviderID: "deepseek",
		ModelID:    "deepseek-chat",
		Messages:   []provider.ModelMessage{{Role: "user", Content: "hi"}},
	}); !errors.Is(err, provider.ErrModelPriceExpired) {
		t.Fatalf("expected ErrModelPriceExpired, got %v", err)
	}
}

func TestSearchEngine_RealHTTPAndRetryPolicy(t *testing.T) {
	ctx := context.Background()

	// 1. Transient 503 then 200 -> retries once and succeeds
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"title": "Go Docs", "url": "https://go.dev", "snippet": "Go concurrency docs", "score": 0.99},
			},
		})
	}))
	defer srv.Close()

	eng := provider.NewSearchEngineWithOptions(provider.SearchEngineOptions{
		APIURL: srv.URL,
		APIKey: "search-secret",
	})
	res, err := eng.Search(ctx, "go concurrency")
	if err != nil || res.Status != "success" || res.Attempts != 2 || len(res.Results) != 1 {
		t.Fatalf("expected retry success on attempt 2, got res=%+v err=%v", res, err)
	}

	// 2. 401 Auth failure -> NO retry (attempts == 1)
	var authAttempts atomic.Int32
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authAttempts.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer authSrv.Close()

	authEng := provider.NewSearchEngineWithOptions(provider.SearchEngineOptions{APIURL: authSrv.URL})
	_, err = authEng.Search(ctx, "go")
	if !errors.Is(err, provider.ErrSearchAuthFailed) || authAttempts.Load() != 1 {
		t.Fatalf("expected single-attempt ErrSearchAuthFailed, got attempts=%d err=%v", authAttempts.Load(), err)
	}

	// 3. Invalid JSON -> failed (NOT zero_results!)
	badJSONSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-valid-json"))
	}))
	defer badJSONSrv.Close()
	badEng := provider.NewSearchEngineWithOptions(provider.SearchEngineOptions{APIURL: badJSONSrv.URL})
	badRes, err := badEng.Search(ctx, "go")
	if err == nil || badRes.Status != "failed" {
		t.Fatalf("expected invalid JSON to fail, got res=%+v err=%v", badRes, err)
	}

	// 4. Unconfigured without simulation -> ErrSearchNotConfigured
	unconfEng := provider.NewSearchEngineWithOptions(provider.SearchEngineOptions{AllowLocalSimulation: false})
	if _, err := unconfEng.Search(ctx, "go"); !errors.Is(err, provider.ErrSearchNotConfigured) {
		t.Fatalf("expected ErrSearchNotConfigured, got %v", err)
	}
}
