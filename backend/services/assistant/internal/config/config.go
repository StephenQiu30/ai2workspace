// Package config provides configuration management for the assistant service.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration options.
type Config struct {
	HTTPAddr           string
	HTTPTimeout        time.Duration
	SwaggerEnabled     bool
	DatabaseURL        string
	ServiceToken       string
	IdentityServiceURL string
	LearningServiceURL string
}

// Load loads configuration from environment variables with defaults.
func Load() (*Config, error) {
	httpAddr := os.Getenv("ASSISTANT_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":3127"
	}

	httpTimeout := 30 * time.Second
	if val := os.Getenv("ASSISTANT_HTTP_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			httpTimeout = d
		}
	}

	swaggerEnabled := true
	if val := os.Getenv("ASSISTANT_SWAGGER_ENABLED"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			swaggerEnabled = b
		}
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ai2workspace_assistant?sslmode=disable"
	}

	serviceToken := os.Getenv("ASSISTANT_SERVICE_TOKEN")
	if serviceToken == "" {
		serviceToken = "assistant-service-secret"
	}

	identityURL := os.Getenv("IDENTITY_SERVICE_URL")
	if identityURL == "" {
		identityURL = "http://identity:3123"
	}

	learningURL := os.Getenv("LEARNING_SERVICE_URL")
	if learningURL == "" {
		learningURL = "http://learning:3124"
	}

	return &Config{
		HTTPAddr:           httpAddr,
		HTTPTimeout:        httpTimeout,
		SwaggerEnabled:     swaggerEnabled,
		DatabaseURL:        dbURL,
		ServiceToken:       serviceToken,
		IdentityServiceURL: identityURL,
		LearningServiceURL: learningURL,
	}, nil
}
