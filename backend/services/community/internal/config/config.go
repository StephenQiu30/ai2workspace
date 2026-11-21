// Package config provides configuration management for the community service.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration options for the community service.
type Config struct {
	HTTPAddr             string
	HTTPTimeout          time.Duration
	SwaggerEnabled       bool
	DatabaseURL          string
	ServiceToken         string
	IdentityServiceURL   string
	IdentityServiceToken string
	LearningServiceURL   string
	LearningServiceToken string
}

// Load loads configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	httpAddr := strings.TrimSpace(os.Getenv("COMMUNITY_HTTP_ADDR"))
	if httpAddr == "" {
		httpAddr = ":3126"
	}

	httpTimeout := 30 * time.Second
	if val := strings.TrimSpace(os.Getenv("COMMUNITY_HTTP_TIMEOUT")); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			httpTimeout = d
		}
	}

	swaggerEnabled := true
	if val := strings.TrimSpace(os.Getenv("COMMUNITY_SWAGGER_ENABLED")); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			swaggerEnabled = b
		}
	}

	// DatabaseURL stays empty when unset so local runs and unit tests can use in-memory stores.
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))

	serviceToken := strings.TrimSpace(os.Getenv("COMMUNITY_SERVICE_TOKEN"))
	if serviceToken == "" {
		serviceToken = "community-service-secret"
	}

	identityURL := strings.TrimRight(strings.TrimSpace(os.Getenv("IDENTITY_SERVICE_URL")), "/")
	identityToken := strings.TrimSpace(os.Getenv("IDENTITY_SERVICE_TOKEN"))
	if identityToken == "" {
		identityToken = serviceToken
	}

	learningURL := strings.TrimRight(strings.TrimSpace(os.Getenv("LEARNING_SERVICE_URL")), "/")
	learningToken := strings.TrimSpace(os.Getenv("LEARNING_SERVICE_TOKEN"))
	if learningToken == "" {
		learningToken = serviceToken
	}

	return &Config{
		HTTPAddr:             httpAddr,
		HTTPTimeout:          httpTimeout,
		SwaggerEnabled:       swaggerEnabled,
		DatabaseURL:          dbURL,
		ServiceToken:         serviceToken,
		IdentityServiceURL:   identityURL,
		IdentityServiceToken: identityToken,
		LearningServiceURL:   learningURL,
		LearningServiceToken: learningToken,
	}, nil
}
