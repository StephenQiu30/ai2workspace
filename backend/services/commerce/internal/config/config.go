// Package config loads runtime configuration for the commerce service.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr             = ":3125"
	defaultHTTPTimeout          = 30 * time.Second
	defaultServiceToken         = "identity-service-secret"
	defaultPaymentWebhookSecret = "dev-commerce-webhook-secret"
)

// Config holds runtime configuration options for the commerce service.
type Config struct {
	HTTPAddr             string
	HTTPTimeout          time.Duration
	SwaggerEnabled       bool
	DatabaseURL          string
	ServiceToken         string
	IdentityServiceURL   string
	LearningServiceURL   string
	CommunityServiceURL  string
	PaymentWebhookSecret string
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	httpAddr := envFirst("COMMERCE_HTTP_ADDR", "HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	httpTimeout := defaultHTTPTimeout
	if val := envFirst("COMMERCE_HTTP_TIMEOUT", "HTTP_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			httpTimeout = d
		}
	}

	swaggerEnabled := true
	if val := envFirst("COMMERCE_SWAGGER_ENABLED", "SWAGGER_ENABLED"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			swaggerEnabled = b
		}
	}

	dbURL := envFirst("DATABASE_URL", "COMMERCE_DATABASE_URL")

	serviceToken := envFirst("COMMERCE_SERVICE_TOKEN", "SERVICE_TOKEN")
	if serviceToken == "" {
		serviceToken = defaultServiceToken
	}

	webhookSecret := envFirst("COMMERCE_PAYMENT_WEBHOOK_SECRET", "PAYMENT_WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = defaultPaymentWebhookSecret
	}

	return &Config{
		HTTPAddr:             httpAddr,
		HTTPTimeout:          httpTimeout,
		SwaggerEnabled:       swaggerEnabled,
		DatabaseURL:          dbURL,
		ServiceToken:         serviceToken,
		IdentityServiceURL:   strings.TrimRight(os.Getenv("IDENTITY_SERVICE_URL"), "/"),
		LearningServiceURL:   strings.TrimRight(os.Getenv("LEARNING_SERVICE_URL"), "/"),
		CommunityServiceURL:  strings.TrimRight(os.Getenv("COMMUNITY_SERVICE_URL"), "/"),
		PaymentWebhookSecret: webhookSecret,
	}, nil
}

func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
