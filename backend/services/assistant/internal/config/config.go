// Package config provides configuration management for the assistant service.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ProviderEndpoint holds API credentials and base URL for a model provider.
type ProviderEndpoint struct {
	APIKey  string
	BaseURL string
}

// Config holds runtime configuration options.
type Config struct {
	HTTPAddr             string
	HTTPTimeout          time.Duration
	SwaggerEnabled       bool
	DatabaseURL          string
	ServiceToken         string
	IdentityServiceURL   string
	LearningServiceURL   string
	DeepSeek             ProviderEndpoint
	OpenRouter           ProviderEndpoint
	Volcengine           ProviderEndpoint
	OpenAI               ProviderEndpoint
	SearchAPIURL         string
	SearchAPIKey         string
	AllowLocalSimulation bool
}

// Load loads configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	httpAddr := envOrDefault("ASSISTANT_HTTP_ADDR", ":3127")

	httpTimeout := 30 * time.Second
	if val := strings.TrimSpace(os.Getenv("ASSISTANT_HTTP_TIMEOUT")); val != "" {
		d, err := time.ParseDuration(val)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("invalid ASSISTANT_HTTP_TIMEOUT: %q", val)
		}
		httpTimeout = d
	}

	swaggerEnabled := true
	if val := strings.TrimSpace(os.Getenv("ASSISTANT_SWAGGER_ENABLED")); val != "" {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_SWAGGER_ENABLED: %q", val)
		}
		swaggerEnabled = b
	}

	allowSimulation := false
	if val := strings.TrimSpace(os.Getenv("AI_ALLOW_LOCAL_SIMULATION")); val != "" {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid AI_ALLOW_LOCAL_SIMULATION: %q", val)
		}
		allowSimulation = b
	}

	serviceToken := strings.TrimSpace(os.Getenv("ASSISTANT_SERVICE_TOKEN"))
	if serviceToken == "" {
		serviceToken = strings.TrimSpace(os.Getenv("SERVICE_TOKEN"))
	}
	if serviceToken == "" {
		serviceToken = "assistant-service-secret"
	}

	return &Config{
		HTTPAddr:           httpAddr,
		HTTPTimeout:        httpTimeout,
		SwaggerEnabled:     swaggerEnabled,
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ServiceToken:       serviceToken,
		IdentityServiceURL: strings.TrimRight(strings.TrimSpace(os.Getenv("IDENTITY_SERVICE_URL")), "/"),
		LearningServiceURL: strings.TrimRight(strings.TrimSpace(os.Getenv("LEARNING_SERVICE_URL")), "/"),
		DeepSeek: ProviderEndpoint{
			APIKey:  strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"), "/"),
		},
		OpenRouter: ProviderEndpoint{
			APIKey:  strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		},
		Volcengine: ProviderEndpoint{
			APIKey:  strings.TrimSpace(os.Getenv("VOLCENGINE_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("VOLCENGINE_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"), "/"),
		},
		OpenAI: ProviderEndpoint{
			APIKey:  strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		},
		SearchAPIURL:         strings.TrimSpace(os.Getenv("SEARCH_API_URL")),
		SearchAPIKey:         strings.TrimSpace(os.Getenv("SEARCH_API_KEY")),
		AllowLocalSimulation: allowSimulation,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}
