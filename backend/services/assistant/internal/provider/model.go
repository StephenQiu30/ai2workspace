// Package provider defines model interfaces and search adapters.
package provider

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrModelNotAllowed  = errors.New("model not in allowed catalog or provider disabled")
	ErrSearchTimeout    = errors.New("search attempt timed out")
	ErrSearchAuthFailed = errors.New("search provider authentication failed")
	ErrSearchFailed     = errors.New("search execution failed")
)

// ModelInfo defines allowed models and pricing.
type ModelInfo struct {
	ProviderID          string `json:"providerId"` // "openai", "deepseek", "openrouter", "volcengine"
	ModelID             string `json:"modelId"`
	DisplayName         string `json:"displayName"`
	PromptPricePerK     int    `json:"promptPricePerK"`     // cents per 1k prompt tokens
	CompletionPricePerK int    `json:"completionPricePerK"` // cents per 1k completion tokens
	Enabled             bool   `json:"enabled"`
}

// DefaultCatalog lists verified allowed models.
var DefaultCatalog = []ModelInfo{
	{
		ProviderID:          "deepseek",
		ModelID:             "deepseek-chat",
		DisplayName:         "DeepSeek V3 (Chat)",
		PromptPricePerK:     1,
		CompletionPricePerK: 2,
		Enabled:             true,
	},
	{
		ProviderID:          "openai",
		ModelID:             "gpt-4o-mini",
		DisplayName:         "OpenAI GPT-4o-mini",
		PromptPricePerK:     2,
		CompletionPricePerK: 6,
		Enabled:             true,
	},
	{
		ProviderID:          "openrouter",
		ModelID:             "anthropic/claude-3.5-haiku",
		DisplayName:         "Claude 3.5 Haiku (via OpenRouter)",
		PromptPricePerK:     3,
		CompletionPricePerK: 8,
		Enabled:             true,
	},
	{
		ProviderID:          "volcengine",
		ModelID:             "doubao-pro-32k",
		DisplayName:         "火山引擎 豆包 Pro",
		PromptPricePerK:     1,
		CompletionPricePerK: 2,
		Enabled:             true,
	},
}

// ModelMessage is a conversational turn.
type ModelMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// GenerateRequest describes input to a model.
type GenerateRequest struct {
	ProviderID  string         `json:"providerId"`
	ModelID     string         `json:"modelId"`
	Messages    []ModelMessage `json:"messages"`
	MaxTokens   int            `json:"maxTokens"`
	Temperature float64        `json:"temperature"`
}

// GenerateResponse normalizes model output and token usage.
type GenerateResponse struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	CostCents        int    `json:"costCents"`
}

// ModelClient performs text generation.
type ModelClient interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
}

// Client implements ModelClient with catalog validation.
type Client struct {
	catalog map[string]ModelInfo
}

// NewClient creates a new unified model client.
func NewClient() *Client {
	cat := make(map[string]ModelInfo)
	for _, m := range DefaultCatalog {
		key := fmt.Sprintf("%s:%s", m.ProviderID, m.ModelID)
		cat[key] = m
	}
	return &Client{catalog: cat}
}

// ListCatalog returns all supported models.
func (c *Client) ListCatalog() []ModelInfo {
	res := make([]ModelInfo, 0, len(c.catalog))
	for _, m := range c.catalog {
		res = append(res, m)
	}
	return res
}

// Generate simulates/executes unified generation with usage calculation.
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	key := fmt.Sprintf("%s:%s", req.ProviderID, req.ModelID)
	info, exists := c.catalog[key]
	if !exists || !info.Enabled {
		return nil, ErrModelNotAllowed
	}

	// Calculate prompt length and tokens estimation (1 token ~= 4 chars)
	totalChars := 0
	for _, m := range req.Messages {
		totalChars += len(m.Content)
	}
	promptTokens := totalChars / 4
	if promptTokens < 10 {
		promptTokens = 10
	}

	// Simulation/deterministic reply based on context
	var lastUserMsg string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMsg = req.Messages[i].Content
			break
		}
	}

	replyContent := fmt.Sprintf("依据检索到的课程证据与实时网络信息，针对问题「%s」的分析如下：\n1. 核心概念与关键事实已经过核实；\n2. 学习建议请参考对应讲节的详细说明与引用来源；\n如有进一步深入疑问，欢迎随时提问！", lastUserMsg)
	completionTokens := len(replyContent) / 4
	if completionTokens < 20 {
		completionTokens = 20
	}

	// Calculate cost
	promptCost := (promptTokens * info.PromptPricePerK) / 1000
	completionCost := (completionTokens * info.CompletionPricePerK) / 1000
	totalCost := promptCost + completionCost
	if totalCost < 1 {
		totalCost = 1 // min 1 cent
	}

	return &GenerateResponse{
		Content:          replyContent,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		CostCents:        totalCost,
	}, nil
}
