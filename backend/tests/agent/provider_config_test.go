package agent_test

import (
	"testing"

	providerapi "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/provider"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AI_ALLOW_LOCAL_SIMULATION", "")
	cfg, err := providerapi.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AllowLocalSimulation {
		t.Errorf("expected AllowLocalSimulation false by default, got true")
	}
}

func TestLoadProviderEnv(t *testing.T) {
	t.Setenv("AI_ALLOW_LOCAL_SIMULATION", "")
	t.Setenv("AI_ALLOW_LOCAL_SIMULATION", "true")
	t.Setenv("DEEPSEEK_API_KEY", "ds-key")
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("VOLCENGINE_API_KEY", "vc-key")
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("SEARCH_API_URL", "https://search.example.com/query")
	t.Setenv("SEARCH_API_KEY", "search-key")

	cfg, err := providerapi.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AllowLocalSimulation {
		t.Fatalf("expected AllowLocalSimulation true")
	}
	if cfg.DeepSeek.APIKey != "ds-key" || cfg.OpenRouter.APIKey != "or-key" || cfg.Volcengine.APIKey != "vc-key" || cfg.OpenAI.APIKey != "oa-key" {
		t.Fatalf("expected all 4 provider API keys loaded")
	}
	if cfg.SearchAPIURL != "https://search.example.com/query" || cfg.SearchAPIKey != "search-key" {
		t.Fatalf("expected search config loaded")
	}
}
