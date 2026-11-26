package provider

import (
	"errors"
	"time"
)

var (
	ErrModelNotAllowed       = errors.New("model not in allowed catalog or provider disabled")
	ErrModelPriceExpired     = errors.New("model pricing is expired or not yet effective")
	ErrProviderNotConfigured = errors.New("model provider API key is not configured")
	ErrModelGenerationFailed = errors.New("model generation failed")
	ErrOutputLimitExceeded   = errors.New("requested maxTokens exceeds model output limit")
	ErrSearchNotConfigured   = errors.New("search provider URL is not configured")
	ErrSearchTimeout         = errors.New("search attempt timed out")
	ErrSearchAuthFailed      = errors.New("search provider authentication failed")
	ErrSearchFailed          = errors.New("search execution failed")
)

// ModelInfo defines an allowed model, its pricing version, validity window, and capabilities.
type ModelInfo struct {
	ProviderID          string    `json:"providerId"`
	ModelID             string    `json:"modelId"`
	DisplayName         string    `json:"displayName"`
	EndpointRef         string    `json:"endpointRef,omitempty"`
	PriceVersion        string    `json:"priceVersion"`
	Currency            string    `json:"currency"`
	PromptPricePerK     int       `json:"promptPricePerK"`     // cents per 1k prompt tokens
	CompletionPricePerK int       `json:"completionPricePerK"` // cents per 1k completion tokens
	MaxInputTokens      int       `json:"maxInputTokens"`
	MaxOutputTokens     int       `json:"maxOutputTokens"`
	EffectiveFrom       time.Time `json:"effectiveFrom"`
	ExpiresAt           time.Time `json:"expiresAt"`
	Enabled             bool      `json:"enabled"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// ModelUpdate defines mutable admin governance fields for a catalog model.
type ModelUpdate struct {
	DisplayName         *string    `json:"displayName,omitempty"`
	PriceVersion        *string    `json:"priceVersion,omitempty"`
	PromptPricePerK     *int       `json:"promptPricePerK,omitempty"`
	CompletionPricePerK *int       `json:"completionPricePerK,omitempty"`
	EffectiveFrom       *time.Time `json:"effectiveFrom,omitempty"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	Enabled             *bool      `json:"enabled,omitempty"`
}

// DefaultCatalog lists the baseline allowed models across the four designated providers.
func DefaultCatalog(now time.Time) []ModelInfo {
	effective := now.Add(-24 * time.Hour).UTC()
	expires := now.Add(365 * 24 * time.Hour).UTC()
	return []ModelInfo{
		{
			ProviderID:          "deepseek",
			ModelID:             "deepseek-chat",
			DisplayName:         "DeepSeek V3 (Chat)",
			EndpointRef:         "https://api.deepseek.com/v1/chat/completions",
			PriceVersion:        "v1-2026-11",
			Currency:            "CNY",
			PromptPricePerK:     1,
			CompletionPricePerK: 2,
			MaxInputTokens:      8000,
			MaxOutputTokens:     2000,
			EffectiveFrom:       effective,
			ExpiresAt:           expires,
			Enabled:             true,
			UpdatedAt:           now.UTC(),
		},
		{
			ProviderID:          "openai",
			ModelID:             "gpt-4o-mini",
			DisplayName:         "OpenAI GPT-4o-mini",
			EndpointRef:         "https://api.openai.com/v1/chat/completions",
			PriceVersion:        "v1-2026-11",
			Currency:            "CNY",
			PromptPricePerK:     2,
			CompletionPricePerK: 5,
			MaxInputTokens:      8000,
			MaxOutputTokens:     2000,
			EffectiveFrom:       effective,
			ExpiresAt:           expires,
			Enabled:             true,
			UpdatedAt:           now.UTC(),
		},
		{
			ProviderID:          "openrouter",
			ModelID:             "anthropic/claude-3.5-haiku",
			DisplayName:         "Claude 3.5 Haiku (via OpenRouter)",
			EndpointRef:         "https://openrouter.ai/api/v1/chat/completions",
			PriceVersion:        "v1-2026-11",
			Currency:            "CNY",
			PromptPricePerK:     2,
			CompletionPricePerK: 6,
			MaxInputTokens:      8000,
			MaxOutputTokens:     2000,
			EffectiveFrom:       effective,
			ExpiresAt:           expires,
			Enabled:             true,
			UpdatedAt:           now.UTC(),
		},
		{
			ProviderID:          "volcengine",
			ModelID:             "doubao-pro-32k",
			DisplayName:         "火山引擎 豆包 Pro",
			EndpointRef:         "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
			PriceVersion:        "v1-2026-11",
			Currency:            "CNY",
			PromptPricePerK:     1,
			CompletionPricePerK: 2,
			MaxInputTokens:      8000,
			MaxOutputTokens:     2000,
			EffectiveFrom:       effective,
			ExpiresAt:           expires,
			Enabled:             true,
			UpdatedAt:           now.UTC(),
		},
	}
}

// ModelMessage is a single conversational turn sent to an LLM provider.
type ModelMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// GenerateRequest describes input to a model call.
type GenerateRequest struct {
	ProviderID  string         `json:"providerId"`
	ModelID     string         `json:"modelId"`
	Messages    []ModelMessage `json:"messages"`
	MaxTokens   int            `json:"maxTokens"`
	Temperature float64        `json:"temperature"`
}

// GenerateResponse normalizes model output, token usage, and billing metadata.
type GenerateResponse struct {
	Content           string `json:"content"`
	ProviderRequestID string `json:"providerRequestId,omitempty"`
	PriceVersion      string `json:"priceVersion,omitempty"`
	PromptTokens      int    `json:"promptTokens"`
	CompletionTokens  int    `json:"completionTokens"`
	CostCents         int    `json:"costCents"`
	UsageMissing      bool   `json:"usageMissing,omitempty"`
	Simulated         bool   `json:"simulated,omitempty"`
}

// EstimateTokensFromRunes provides a conservative token estimate from rune count.
func EstimateTokensFromRunes(runeCount int) int {
	if runeCount <= 0 {
		return 0
	}
	// Conservative approximation (~1.5 runes per token for mixed Chinese/English).
	tokens := (runeCount*2 + 2) / 3
	if tokens < 1 {
		return 1
	}
	return tokens
}

// CalculateCostCents computes integer cent cost using ceiling division (minimum 1 cent when tokens > 0).
func CalculateCostCents(promptTokens, completionTokens, promptPricePerK, completionPricePerK int) int {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	promptCost := (promptTokens*promptPricePerK + 999) / 1000
	completionCost := (completionTokens*completionPricePerK + 999) / 1000
	total := promptCost + completionCost
	if total < 1 {
		total = 1
	}
	return total
}
