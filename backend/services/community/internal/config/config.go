// Package config provides configuration management for the community service.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration options.
type Config struct {
	HTTPAddr       string
	HTTPTimeout    time.Duration
	SwaggerEnabled bool
	DatabaseURL    string
	ServiceToken   string
}

// Load loads configuration from environment variables with defaults.
func Load() (*Config, error) {
	httpAddr := os.Getenv("COMMUNITY_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":3126"
	}

	httpTimeout := 30 * time.Second
	if val := os.Getenv("COMMUNITY_HTTP_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			httpTimeout = d
		}
	}

	swaggerEnabled := true
	if val := os.Getenv("COMMUNITY_SWAGGER_ENABLED"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			swaggerEnabled = b
		}
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ai2workspace_community?sslmode=disable"
	}

	serviceToken := os.Getenv("COMMUNITY_SERVICE_TOKEN")
	if serviceToken == "" {
		serviceToken = "community-service-secret"
	}

	return &Config{
		HTTPAddr:       httpAddr,
		HTTPTimeout:    httpTimeout,
		SwaggerEnabled: swaggerEnabled,
		DatabaseURL:    dbURL,
		ServiceToken:   serviceToken,
	}, nil
}
