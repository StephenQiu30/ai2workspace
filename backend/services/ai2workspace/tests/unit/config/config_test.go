package config_test

import (
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/ai2workspace/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"HTTP_ADDR", "HTTP_TIMEOUT", "SHUTDOWN_TIMEOUT", "SWAGGER_ENABLED"} {
		t.Setenv(key, "")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":3123" || cfg.HTTPTimeout != 5*time.Second || cfg.SwaggerEnabled {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"HTTP_ADDR", "invalid"}, {"HTTP_ADDR", ":0"}, {"HTTP_ADDR", ":65536"},
		{"HTTP_TIMEOUT", "0s"}, {"HTTP_TIMEOUT", "invalid"},
		{"SHUTDOWN_TIMEOUT", "-1s"}, {"SWAGGER_ENABLED", "invalid"},
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
	t.Setenv("HTTP_ADDR", "127.0.0.1:3124")
	t.Setenv("HTTP_TIMEOUT", "2s")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("SWAGGER_ENABLED", "true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:3124" || cfg.HTTPTimeout != 2*time.Second || cfg.ShutdownTimeout != 3*time.Second || !cfg.SwaggerEnabled {
		t.Fatalf("configuration overrides not applied: %+v", cfg)
	}
}
