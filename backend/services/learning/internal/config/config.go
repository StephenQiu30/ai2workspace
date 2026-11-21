// Package config reads and validates learning service configuration.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains process-level configuration for the learning service.
type Config struct {
	HTTPAddr              string
	HTTPTimeout           time.Duration
	ShutdownTimeout       time.Duration
	SwaggerEnabled        bool
	DatabaseURL           string
	ServiceToken          string
	IdentityServiceURL    string
	CommerceServiceURL    string
	PlaybackSigningSecret string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	serviceToken := value("SERVICE_TOKEN", "identity-service-secret")
	cfg := Config{
		HTTPAddr:              value("HTTP_ADDR", ":3124"),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ServiceToken:          serviceToken,
		IdentityServiceURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("IDENTITY_SERVICE_URL")), "/"),
		CommerceServiceURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("COMMERCE_SERVICE_URL")), "/"),
		PlaybackSigningSecret: value("PLAYBACK_SIGNING_SECRET", serviceToken),
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR requires a port between 1 and 65535")
	}
	if cfg.HTTPTimeout, err = duration("HTTP_TIMEOUT", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", "10s"); err != nil {
		return Config{}, err
	}
	if cfg.SwaggerEnabled, err = strconv.ParseBool(value("SWAGGER_ENABLED", "false")); err != nil {
		return Config{}, fmt.Errorf("SWAGGER_ENABLED must be a boolean: %w", err)
	}
	if err := validateOptionalHTTPURL("IDENTITY_SERVICE_URL", cfg.IdentityServiceURL); err != nil {
		return Config{}, err
	}
	if err := validateOptionalHTTPURL("COMMERCE_SERVICE_URL", cfg.CommerceServiceURL); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateOptionalHTTPURL(key, raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s must be a valid http(s) URL", key)
	}
	return nil
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func duration(key, fallback string) (time.Duration, error) {
	s := value(key, fallback)
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return d, nil
}
