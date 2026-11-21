package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	keys := []string{
		"COMMERCE_HTTP_ADDR", "HTTP_ADDR",
		"COMMERCE_HTTP_TIMEOUT", "HTTP_TIMEOUT",
		"COMMERCE_SWAGGER_ENABLED", "SWAGGER_ENABLED",
		"DATABASE_URL", "COMMERCE_DATABASE_URL",
		"COMMERCE_SERVICE_TOKEN", "SERVICE_TOKEN",
		"IDENTITY_SERVICE_URL", "LEARNING_SERVICE_URL", "COMMUNITY_SERVICE_URL",
		"COMMERCE_PAYMENT_WEBHOOK_SECRET", "PAYMENT_WEBHOOK_SECRET",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

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
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty default DatabaseURL for memory fallback, got %s", cfg.DatabaseURL)
	}
	if cfg.PaymentWebhookSecret == "" {
		t.Errorf("expected default dev webhook secret")
	}
	if cfg.IdentityServiceURL != "" || cfg.LearningServiceURL != "" || cfg.CommunityServiceURL != "" {
		t.Errorf("expected empty default upstream service URLs")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("COMMERCE_HTTP_ADDR", ":4125")
	t.Setenv("COMMERCE_HTTP_TIMEOUT", "15s")
	t.Setenv("COMMERCE_SWAGGER_ENABLED", "false")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/commerce")
	t.Setenv("COMMERCE_SERVICE_TOKEN", "custom-token")
	t.Setenv("IDENTITY_SERVICE_URL", "http://identity:3123/")
	t.Setenv("LEARNING_SERVICE_URL", "http://learning:3124/")
	t.Setenv("COMMUNITY_SERVICE_URL", "http://community:3126/")
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_SECRET", "prod-webhook-secret")

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
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/commerce" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.ServiceToken != "custom-token" {
		t.Errorf("unexpected ServiceToken: %s", cfg.ServiceToken)
	}
	if cfg.IdentityServiceURL != "http://identity:3123" {
		t.Errorf("expected trimmed IdentityServiceURL, got %s", cfg.IdentityServiceURL)
	}
	if cfg.LearningServiceURL != "http://learning:3124" {
		t.Errorf("expected trimmed LearningServiceURL, got %s", cfg.LearningServiceURL)
	}
	if cfg.CommunityServiceURL != "http://community:3126" {
		t.Errorf("expected trimmed CommunityServiceURL, got %s", cfg.CommunityServiceURL)
	}
	if cfg.PaymentWebhookSecret != "prod-webhook-secret" {
		t.Errorf("unexpected PaymentWebhookSecret: %s", cfg.PaymentWebhookSecret)
	}
}
