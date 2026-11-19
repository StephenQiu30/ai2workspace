package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("COMMERCE_HTTP_ADDR")
	os.Unsetenv("COMMERCE_HTTP_TIMEOUT")
	os.Unsetenv("COMMERCE_SWAGGER_ENABLED")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":3125" {
		t.Errorf("expected :3125, got %s", cfg.HTTPAddr)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.HTTPTimeout)
	}
	if !cfg.SwaggerEnabled {
		t.Errorf("expected swagger enabled by default")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("COMMERCE_HTTP_ADDR", ":4125")
	t.Setenv("COMMERCE_HTTP_TIMEOUT", "15s")
	t.Setenv("COMMERCE_SWAGGER_ENABLED", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":4125" {
		t.Errorf("expected :4125, got %s", cfg.HTTPAddr)
	}
	if cfg.HTTPTimeout != 15*time.Second {
		t.Errorf("expected 15s timeout, got %v", cfg.HTTPTimeout)
	}
	if cfg.SwaggerEnabled {
		t.Errorf("expected swagger disabled")
	}
}
