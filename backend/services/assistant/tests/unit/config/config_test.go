package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":3127" {
		t.Errorf("expected HTTPAddr :3127, got %s", cfg.HTTPAddr)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("expected HTTPTimeout 30s, got %v", cfg.HTTPTimeout)
	}
	if !cfg.SwaggerEnabled {
		t.Errorf("expected SwaggerEnabled true, got %v", cfg.SwaggerEnabled)
	}
	if cfg.AllowLocalSimulation {
		t.Errorf("expected AllowLocalSimulation false by default, got true")
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty default DatabaseURL, got %q", cfg.DatabaseURL)
	}
}

func TestLoadProviderAndServiceEnv(t *testing.T) {
	os.Clearenv()
	t.Setenv("ASSISTANT_HTTP_ADDR", "127.0.0.1:4127")
	t.Setenv("ASSISTANT_HTTP_TIMEOUT", "45s")
	t.Setenv("ASSISTANT_SWAGGER_ENABLED", "false")
	t.Setenv("AI_ALLOW_LOCAL_SIMULATION", "true")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/assistant")
	t.Setenv("SERVICE_TOKEN", "shared-secret")
	t.Setenv("IDENTITY_SERVICE_URL", "http://identity:3123/")
	t.Setenv("LEARNING_SERVICE_URL", "http://learning:3124/")
	t.Setenv("DEEPSEEK_API_KEY", "ds-key")
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("VOLCENGINE_API_KEY", "vc-key")
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("SEARCH_API_URL", "https://search.example.com/query")
	t.Setenv("SEARCH_API_KEY", "search-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:4127" || cfg.HTTPTimeout != 45*time.Second || cfg.SwaggerEnabled {
		t.Fatalf("unexpected core config: %+v", cfg)
	}
	if !cfg.AllowLocalSimulation {
		t.Fatalf("expected AllowLocalSimulation true")
	}
	if cfg.ServiceToken != "shared-secret" {
		t.Fatalf("expected ServiceToken fallback to SERVICE_TOKEN, got %q", cfg.ServiceToken)
	}
	if cfg.IdentityServiceURL != "http://identity:3123" || cfg.LearningServiceURL != "http://learning:3124" {
		t.Fatalf("expected trimmed service URLs, got identity=%q learning=%q", cfg.IdentityServiceURL, cfg.LearningServiceURL)
	}
	if cfg.DeepSeek.APIKey != "ds-key" || cfg.OpenRouter.APIKey != "or-key" || cfg.Volcengine.APIKey != "vc-key" || cfg.OpenAI.APIKey != "oa-key" {
		t.Fatalf("expected all 4 provider API keys loaded")
	}
	if cfg.SearchAPIURL != "https://search.example.com/query" || cfg.SearchAPIKey != "search-key" {
		t.Fatalf("expected search config loaded")
	}
}
