package config_test

import (
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/infra/config"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HTTP_ADDR",
		"HTTP_TIMEOUT",
		"SHUTDOWN_TIMEOUT",
		"SWAGGER_ENABLED",
		"DATABASE_URL",
		"PAYMENT_WEBHOOK_SECRET",
		"COOKIE_SECURE",
		"DEV_EXPOSE_TOKENS",
		"BOOTSTRAP_ADMIN_EMAIL",
		"BOOTSTRAP_ADMIN_PASSWORD",
		"SMTP_HOST",
		"SMTP_PORT",
		"SMTP_USER",
		"SMTP_PASSWORD",
		"SMTP_FROM",
		"RATE_LIMIT_AUTH_MAX",
		"RATE_LIMIT_MAIL_MAX",
		"RATE_LIMIT_WINDOW",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearConfigEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:3123" ||
		cfg.HTTPTimeout != 5*time.Second ||
		cfg.ShutdownTimeout != 10*time.Second ||
		cfg.SwaggerEnabled ||
		cfg.DatabaseURL != "" ||
		cfg.PaymentWebhookSecret != "" ||
		cfg.CookieSecure ||
		cfg.DevExposeTokens ||
		cfg.BootstrapAdminEmail != "" ||
		cfg.BootstrapAdminPassword != "" ||
		cfg.RateLimitAuthMax != 20 ||
		cfg.RateLimitMailMax != 10 ||
		cfg.RateLimitWindow != time.Minute {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{"HTTP_ADDR/invalid", map[string]string{"HTTP_ADDR": "invalid"}},
		{"HTTP_ADDR/:0", map[string]string{"HTTP_ADDR": ":0"}},
		{"HTTP_ADDR/:65536", map[string]string{"HTTP_ADDR": ":65536"}},
		{"HTTP_TIMEOUT/0s", map[string]string{"HTTP_TIMEOUT": "0s"}},
		{"HTTP_TIMEOUT/invalid", map[string]string{"HTTP_TIMEOUT": "invalid"}},
		{"SHUTDOWN_TIMEOUT/-1s", map[string]string{"SHUTDOWN_TIMEOUT": "-1s"}},
		{"SWAGGER_ENABLED/invalid", map[string]string{"SWAGGER_ENABLED": "invalid"}},
		{"COOKIE_SECURE/invalid", map[string]string{"COOKIE_SECURE": "notabool"}},
		{"DEV_EXPOSE_TOKENS/invalid", map[string]string{"DEV_EXPOSE_TOKENS": "nope"}},
		{"BOOTSTRAP_EMAIL_ONLY", map[string]string{"BOOTSTRAP_ADMIN_EMAIL": "admin@example.com"}},
		{"BOOTSTRAP_PASSWORD_ONLY", map[string]string{"BOOTSTRAP_ADMIN_PASSWORD": "AdminPassword123"}},
		{"BOOTSTRAP_PASSWORD_SHORT", map[string]string{"BOOTSTRAP_ADMIN_EMAIL": "admin@example.com", "BOOTSTRAP_ADMIN_PASSWORD": "short"}},
		{"RATE_LIMIT_AUTH_NEGATIVE", map[string]string{"RATE_LIMIT_AUTH_MAX": "-1"}},
		{"RATE_LIMIT_MAIL_INVALID", map[string]string{"RATE_LIMIT_MAIL_MAX": "abc"}},
		{"RATE_LIMIT_WINDOW_ZERO", map[string]string{"RATE_LIMIT_WINDOW": "0s"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if _, err := config.Load(); err == nil {
				t.Fatalf("Load accepted invalid config: %v", tc.env)
			}
		})
	}
}

func TestLoadOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_ADDR", "127.0.0.1:3124")
	t.Setenv("HTTP_TIMEOUT", "2s")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("SWAGGER_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/identity")
	t.Setenv("PAYMENT_WEBHOOK_SECRET", "custom-secret")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("DEV_EXPOSE_TOKENS", "true")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@ai2workspace.local")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "SuperSecure123")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("RATE_LIMIT_AUTH_MAX", "50")
	t.Setenv("RATE_LIMIT_MAIL_MAX", "5")
	t.Setenv("RATE_LIMIT_WINDOW", "30s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:3124" ||
		cfg.HTTPTimeout != 2*time.Second ||
		cfg.ShutdownTimeout != 3*time.Second ||
		!cfg.SwaggerEnabled ||
		cfg.DatabaseURL != "postgres://user:pass@localhost:5432/identity" ||
		cfg.PaymentWebhookSecret != "custom-secret" ||
		!cfg.CookieSecure ||
		!cfg.DevExposeTokens ||
		cfg.BootstrapAdminEmail != "admin@ai2workspace.local" ||
		cfg.BootstrapAdminPassword != "SuperSecure123" ||
		cfg.SMTPHost != "smtp.example.com" ||
		cfg.SMTPPort != "2525" ||
		cfg.RateLimitAuthMax != 50 ||
		cfg.RateLimitMailMax != 5 ||
		cfg.RateLimitWindow != 30*time.Second {
		t.Fatalf("configuration overrides not applied: %+v", cfg)
	}
}
