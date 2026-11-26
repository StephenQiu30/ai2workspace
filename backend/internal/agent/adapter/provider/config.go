package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Endpoint holds API credentials and base URL for a model provider.
type Endpoint struct {
	APIKey  string
	BaseURL string
}

// Config holds runtime configuration options.
type Config struct {
	DeepSeek             Endpoint
	OpenRouter           Endpoint
	Volcengine           Endpoint
	OpenAI               Endpoint
	SearchAPIURL         string
	SearchAPIKey         string
	AllowLocalSimulation bool
}

// LoadConfig loads configuration from environment variables with safe defaults.
func LoadConfig() (*Config, error) {
	allowSimulation := false
	if val := strings.TrimSpace(os.Getenv("AI_ALLOW_LOCAL_SIMULATION")); val != "" {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid AI_ALLOW_LOCAL_SIMULATION: %q", val)
		}
		allowSimulation = b
	}

	return &Config{
		DeepSeek: Endpoint{
			APIKey:  strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"), "/"),
		},
		OpenRouter: Endpoint{
			APIKey:  strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		},
		Volcengine: Endpoint{
			APIKey:  strings.TrimSpace(os.Getenv("VOLCENGINE_API_KEY")),
			BaseURL: strings.TrimRight(envOrDefault("VOLCENGINE_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"), "/"),
		},
		OpenAI: Endpoint{
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
