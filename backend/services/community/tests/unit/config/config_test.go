package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("COMMUNITY_HTTP_ADDR")
	os.Unsetenv("COMMUNITY_HTTP_TIMEOUT")
	os.Unsetenv("COMMUNITY_SWAGGER_ENABLED")

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
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("COMMUNITY_HTTP_ADDR", ":4126")
	t.Setenv("COMMUNITY_HTTP_TIMEOUT", "15s")
	t.Setenv("COMMUNITY_SWAGGER_ENABLED", "false")

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
}
