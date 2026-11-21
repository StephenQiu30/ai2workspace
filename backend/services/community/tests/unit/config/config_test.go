package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"COMMUNITY_HTTP_ADDR",
		"COMMUNITY_HTTP_TIMEOUT",
		"COMMUNITY_SWAGGER_ENABLED",
		"DATABASE_URL",
		"COMMUNITY_SERVICE_TOKEN",
		"IDENTITY_SERVICE_URL",
		"IDENTITY_SERVICE_TOKEN",
		"LEARNING_SERVICE_URL",
		"LEARNING_SERVICE_TOKEN",
	} {
		_ = os.Unsetenv(key)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":3126" {
		t.Errorf("expected :3126, got %s", cfg.HTTPAddr)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.HTTPTimeout)
	}
	if !cfg.SwaggerEnabled {
		t.Errorf("expected swagger enabled by default")
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty DATABASE_URL by default, got %q", cfg.DatabaseURL)
	}
	if cfg.ServiceToken != "community-service-secret" {
		t.Errorf("unexpected default service token: %q", cfg.ServiceToken)
	}
	if cfg.IdentityServiceURL != "" || cfg.LearningServiceURL != "" {
		t.Errorf("expected empty upstream service URLs by default, got identity=%q learning=%q", cfg.IdentityServiceURL, cfg.LearningServiceURL)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("COMMUNITY_HTTP_ADDR", ":4126")
	t.Setenv("COMMUNITY_HTTP_TIMEOUT", "15s")
	t.Setenv("COMMUNITY_SWAGGER_ENABLED", "false")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/community_db?sslmode=disable")
	t.Setenv("COMMUNITY_SERVICE_TOKEN", "custom-comm-token")
	t.Setenv("IDENTITY_SERVICE_URL", "http://identity:3123/")
	t.Setenv("IDENTITY_SERVICE_TOKEN", "custom-id-token")
	t.Setenv("LEARNING_SERVICE_URL", "http://learning:3124/")
	t.Setenv("LEARNING_SERVICE_TOKEN", "custom-learn-token")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":4126" {
		t.Errorf("expected :4126, got %s", cfg.HTTPAddr)
	}
	if cfg.HTTPTimeout != 15*time.Second {
		t.Errorf("expected 15s timeout, got %v", cfg.HTTPTimeout)
	}
	if cfg.SwaggerEnabled {
		t.Errorf("expected swagger disabled")
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/community_db?sslmode=disable" {
		t.Errorf("unexpected DATABASE_URL: %q", cfg.DatabaseURL)
	}
	if cfg.ServiceToken != "custom-comm-token" {
		t.Errorf("unexpected ServiceToken: %q", cfg.ServiceToken)
	}
	if cfg.IdentityServiceURL != "http://identity:3123" {
		t.Errorf("expected trimmed IdentityServiceURL, got %q", cfg.IdentityServiceURL)
	}
	if cfg.IdentityServiceToken != "custom-id-token" {
		t.Errorf("unexpected IdentityServiceToken: %q", cfg.IdentityServiceToken)
	}
	if cfg.LearningServiceURL != "http://learning:3124" {
		t.Errorf("expected trimmed LearningServiceURL, got %q", cfg.LearningServiceURL)
	}
	if cfg.LearningServiceToken != "custom-learn-token" {
		t.Errorf("unexpected LearningServiceToken: %q", cfg.LearningServiceToken)
	}
}
