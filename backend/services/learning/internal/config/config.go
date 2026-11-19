// Package config reads and validates learning service configuration.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// Config contains process-level configuration for the learning service.
type Config struct {
	HTTPAddr        string
	HTTPTimeout     time.Duration
	ShutdownTimeout time.Duration
	SwaggerEnabled  bool
	ServiceToken    string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:     value("HTTP_ADDR", ":3124"),
		ServiceToken: value("SERVICE_TOKEN", "identity-service-secret"),
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
	return cfg, nil
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
