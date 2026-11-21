package config_test

import (
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"HTTP_ADDR", "HTTP_TIMEOUT", "SHUTDOWN_TIMEOUT", "SWAGGER_ENABLED",
		"DATABASE_URL", "SERVICE_TOKEN", "IDENTITY_SERVICE_URL", "COMMERCE_SERVICE_URL", "PLAYBACK_SIGNING_SECRET",
	} {
		t.Setenv(key, "")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":3124" || cfg.HTTPTimeout != 5*time.Second || cfg.ShutdownTimeout != 10*time.Second || cfg.SwaggerEnabled {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ServiceToken != "identity-service-secret" || cfg.PlaybackSigningSecret != "identity-service-secret" {
		t.Fatalf("unexpected default secrets: %+v", cfg)
	}
	if cfg.DatabaseURL != "" || cfg.IdentityServiceURL != "" || cfg.CommerceServiceURL != "" {
		t.Fatalf("unexpected default URLs: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"HTTP_ADDR", "invalid"}, {"HTTP_ADDR", ":0"}, {"HTTP_ADDR", ":65536"},
		{"HTTP_TIMEOUT", "0s"}, {"HTTP_TIMEOUT", "invalid"},
		{"SHUTDOWN_TIMEOUT", "-1s"}, {"SWAGGER_ENABLED", "invalid"},
		{"IDENTITY_SERVICE_URL", "not-a-url"}, {"IDENTITY_SERVICE_URL", "ftp://identity:3123"},
		{"COMMERCE_SERVICE_URL", "://bad"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := config.Load(); err == nil {
				t.Fatalf("Load accepted %s=%q", tc.key, tc.value)
			}
		})
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:3199")
	t.Setenv("HTTP_TIMEOUT", "3s")
	t.Setenv("SHUTDOWN_TIMEOUT", "4s")
	t.Setenv("SWAGGER_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/learning")
	t.Setenv("SERVICE_TOKEN", "custom-svc-token")
	t.Setenv("IDENTITY_SERVICE_URL", "http://identity:3123/")
	t.Setenv("COMMERCE_SERVICE_URL", "http://commerce:3125/")
	t.Setenv("PLAYBACK_SIGNING_SECRET", "custom-signing-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:3199" || cfg.HTTPTimeout != 3*time.Second || cfg.ShutdownTimeout != 4*time.Second || !cfg.SwaggerEnabled {
		t.Fatalf("configuration overrides not applied: %+v", cfg)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/learning" ||
		cfg.ServiceToken != "custom-svc-token" ||
		cfg.IdentityServiceURL != "http://identity:3123" ||
		cfg.CommerceServiceURL != "http://commerce:3125" ||
		cfg.PlaybackSigningSecret != "custom-signing-secret" {
		t.Fatalf("service overrides not applied: %+v", cfg)
	}
}
