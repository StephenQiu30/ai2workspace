// Package config loads runtime configuration for the commerce service.
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

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	httpAddr := os.Getenv("COMMERCE_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":3125"
	}

	httpTimeout := 30 * time.Second
	if val := os.Getenv("COMMERCE_HTTP_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			httpTimeout = d
		}
	}

	swaggerEnabled := true
	if val := os.Getenv("COMMERCE_SWAGGER_ENABLED"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			swaggerEnabled = b
		}
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ai2workspace_commerce?sslmode=disable"
	}

	serviceToken := os.Getenv("COMMERCE_SERVICE_TOKEN")
	if serviceToken == "" {
		serviceToken = "commerce-service-secret"
	}

	return &Config{
		HTTPAddr:       httpAddr,
		HTTPTimeout:    httpTimeout,
		SwaggerEnabled: swaggerEnabled,
		DatabaseURL:    dbURL,
		ServiceToken:   serviceToken,
	}, nil
}
