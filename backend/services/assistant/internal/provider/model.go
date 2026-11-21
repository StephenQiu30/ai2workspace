// Package provider defines model catalog governance, real HTTP LLM adapters, and search adapters.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ModelClient performs text generation and catalog validation.
type ModelClient interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	ValidateModelForCall(providerID, modelID string) (ModelInfo, error)
}

// ClientOptions configures real HTTP provider credentials and optional simulation mode.
type ClientOptions struct {
	Endpoints            map[string]config.ProviderEndpoint
	AllowLocalSimulation bool
	HTTPClient           *http.Client
	DBPool               *pgxpool.Pool
	NowFunc              func() time.Time
}

// Client implements ModelClient with strict catalog validation and real HTTP adapters.
type Client struct {
	mu                   sync.RWMutex
	catalog              map[string]ModelInfo
	endpoints            map[string]config.ProviderEndpoint
	allowLocalSimulation bool
	httpClient           *http.Client
	store                *catalogStore
	nowFunc              func() time.Time
}

// NewClient creates a unified model client from runtime configuration.
func NewClient(cfg *config.Config) *Client {
	opts := ClientOptions{}
	if cfg != nil {
		opts.AllowLocalSimulation = cfg.AllowLocalSimulation
		opts.Endpoints = map[string]config.ProviderEndpoint{
			"deepseek":   cfg.DeepSeek,
			"openrouter": cfg.OpenRouter,
			"volcengine": cfg.Volcengine,
			"openai":     cfg.OpenAI,
		}
	}
	return NewClientWithOptions(opts)
}

// NewClientWithOptions creates a model client with explicit options (useful for tests & DB wiring).
func NewClientWithOptions(opts ClientOptions) *Client {
	nowFn := opts.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}
	httpCli := opts.HTTPClient
	if httpCli == nil {
		httpCli = &http.Client{Timeout: 65 * time.Second}
	}

	endpoints := map[string]config.ProviderEndpoint{
		"deepseek":   {BaseURL: "https://api.deepseek.com/v1"},
		"openrouter": {BaseURL: "https://openrouter.ai/api/v1"},
		"volcengine": {BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
		"openai":     {BaseURL: "https://api.openai.com/v1"},
	}
	for k, ep := range opts.Endpoints {
		if ep.BaseURL == "" {
			ep.BaseURL = endpoints[k].BaseURL
		}
		endpoints[k] = ep
	}

	cat := make(map[string]ModelInfo)
	for _, m := range DefaultCatalog(nowFn()) {
		cat[catalogKey(m.ProviderID, m.ModelID)] = m
	}

	c := &Client{
		catalog:              cat,
		endpoints:            endpoints,
		allowLocalSimulation: opts.AllowLocalSimulation,
		httpClient:           httpCli,
		nowFunc:              nowFn,
	}

	if opts.DBPool != nil {
		c.store = newCatalogStore(opts.DBPool)
		c.bootstrapFromStore(context.Background())
	}

	return c
}

func catalogKey(providerID, modelID string) string {
	return providerID + ":" + modelID
}

// SetNowFunc overrides the clock for deterministic expiration testing.
func (c *Client) SetNowFunc(fn func() time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fn == nil {
		c.nowFunc = time.Now
	} else {
		c.nowFunc = fn
	}
}

// ListCatalog returns allowed models. When includeDisabled is false, only enabled & unexpired models are returned.
func (c *Client) ListCatalog(includeDisabled bool) []ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := c.nowFunc()
	res := make([]ModelInfo, 0, len(c.catalog))
	for _, m := range c.catalog {
		if !includeDisabled {
			if !m.Enabled {
				continue
			}
			if !m.ExpiresAt.IsZero() && !now.Before(m.ExpiresAt) {
				continue
			}
			if !m.EffectiveFrom.IsZero() && now.Before(m.EffectiveFrom) {
				continue
			}
		}
		res = append(res, m)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].ProviderID != res[j].ProviderID {
			return res[i].ProviderID < res[j].ProviderID
		}
		return res[i].ModelID < res[j].ModelID
	})
	return res
}

// GetModel looks up a model entry by provider and model ID.
func (c *Client) GetModel(providerID, modelID string) (ModelInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.catalog[catalogKey(providerID, modelID)]
	return m, ok
}

// ValidateModelForCall verifies that the model exists, is enabled, and has valid unexpired pricing.
func (c *Client) ValidateModelForCall(providerID, modelID string) (ModelInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info, exists := c.catalog[catalogKey(providerID, modelID)]
	if !exists || !info.Enabled {
		return ModelInfo{}, ErrModelNotAllowed
	}
	now := c.nowFunc()
	if !info.EffectiveFrom.IsZero() && now.Before(info.EffectiveFrom) {
		return ModelInfo{}, ErrModelPriceExpired
	}
	if !info.ExpiresAt.IsZero() && !now.Before(info.ExpiresAt) {
		return ModelInfo{}, ErrModelPriceExpired
	}
	if info.PromptPricePerK < 0 || info.CompletionPricePerK < 0 || (info.PromptPricePerK == 0 && info.CompletionPricePerK == 0) {
		return ModelInfo{}, ErrModelPriceExpired
	}
	return info, nil
}

// UpdateModel updates a model's enabled state, price version, unit prices, or validity window (admin only).
func (c *Client) UpdateModel(ctx context.Context, providerID, modelID string, update ModelUpdate) (ModelInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := catalogKey(providerID, modelID)
	info, exists := c.catalog[key]
	if !exists {
		return ModelInfo{}, ErrModelNotAllowed
	}
	if update.DisplayName != nil && strings.TrimSpace(*update.DisplayName) != "" {
		info.DisplayName = strings.TrimSpace(*update.DisplayName)
	}
	if update.PriceVersion != nil && strings.TrimSpace(*update.PriceVersion) != "" {
		info.PriceVersion = strings.TrimSpace(*update.PriceVersion)
	}
	if update.PromptPricePerK != nil {
		if *update.PromptPricePerK < 0 {
			return ModelInfo{}, errors.New("promptPricePerK must be non-negative")
		}
		info.PromptPricePerK = *update.PromptPricePerK
	}
	if update.CompletionPricePerK != nil {
		if *update.CompletionPricePerK < 0 {
			return ModelInfo{}, errors.New("completionPricePerK must be non-negative")
		}
		info.CompletionPricePerK = *update.CompletionPricePerK
	}
	if update.EffectiveFrom != nil {
		info.EffectiveFrom = update.EffectiveFrom.UTC()
	}
	if update.ExpiresAt != nil {
		info.ExpiresAt = update.ExpiresAt.UTC()
	}
	if update.Enabled != nil {
		info.Enabled = *update.Enabled
	}
	info.UpdatedAt = c.nowFunc().UTC()

	if c.store != nil {
		if err := c.store.upsertModel(ctx, info); err != nil {
			return ModelInfo{}, err
		}
	}
	c.catalog[key] = info
	return info, nil
}

type chatCompletionRequest struct {
	Model       string         `json:"model"`
	Messages    []ModelMessage `json:"messages"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	Stream      bool           `json:"stream"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

// Generate calls the configured provider's real Chat Completions API (no silent fallback or auto-retry).
// If the provider API key is not configured, it returns ErrProviderNotConfigured unless AllowLocalSimulation is true.
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: nil request", ErrModelGenerationFailed)
	}
	info, err := c.ValidateModelForCall(req.ProviderID, req.ModelID)
	if err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = info.MaxOutputTokens
	}
	if info.MaxOutputTokens > 0 && maxTokens > info.MaxOutputTokens {
		return nil, fmt.Errorf("%w: requested %d > limit %d", ErrOutputLimitExceeded, maxTokens, info.MaxOutputTokens)
	}

	c.mu.RLock()
	ep := c.endpoints[req.ProviderID]
	allowSim := c.allowLocalSimulation
	httpCli := c.httpClient
	c.mu.RUnlock()

	if strings.TrimSpace(ep.APIKey) != "" {
		return c.callChatCompletionsHTTP(ctx, httpCli, ep, info, req, maxTokens)
	}

	if !allowSim {
		return nil, fmt.Errorf("%w: provider=%s model=%s", ErrProviderNotConfigured, req.ProviderID, req.ModelID)
	}

	return c.simulateGenerate(ctx, info, req)
}

func (c *Client) callChatCompletionsHTTP(
	ctx context.Context,
	httpCli *http.Client,
	ep config.ProviderEndpoint,
	info ModelInfo,
	req *GenerateRequest,
	maxTokens int,
) (*GenerateResponse, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(ep.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: missing base URL for provider %s", ErrProviderNotConfigured, req.ProviderID)
	}
	url := baseURL + "/chat/completions"

	payload := chatCompletionRequest{
		Model:       req.ModelID,
		Messages:    req.Messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %v", ErrModelGenerationFailed, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrModelGenerationFailed, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+ep.APIKey)
	if req.ProviderID == "openrouter" {
		httpReq.Header.Set("X-Title", "ai2workspace")
	}

	resp, err := httpCli.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrModelGenerationFailed, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %w", ErrModelGenerationFailed, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errEnvelope chatCompletionResponse
		_ = json.Unmarshal(respBody, &errEnvelope)
		if errEnvelope.Error != nil && errEnvelope.Error.Message != "" {
			return nil, fmt.Errorf("%w: provider=%s status=%d msg=%s", ErrModelGenerationFailed, req.ProviderID, resp.StatusCode, errEnvelope.Error.Message)
		}
		return nil, fmt.Errorf("%w: provider=%s status=%d", ErrModelGenerationFailed, req.ProviderID, resp.StatusCode)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%w: invalid provider JSON: %v", ErrModelGenerationFailed, err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%w: provider returned zero choices", ErrModelGenerationFailed)
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: provider returned empty content", ErrModelGenerationFailed)
	}

	out := &GenerateResponse{
		Content:           content,
		ProviderRequestID: parsed.ID,
		PriceVersion:      info.PriceVersion,
		Simulated:         false,
	}

	if parsed.Usage == nil || (parsed.Usage.PromptTokens == 0 && parsed.Usage.CompletionTokens == 0) {
		out.UsageMissing = true
		return out, nil
	}

	out.PromptTokens = parsed.Usage.PromptTokens
	out.CompletionTokens = parsed.Usage.CompletionTokens
	out.CostCents = CalculateCostCents(out.PromptTokens, out.CompletionTokens, info.PromptPricePerK, info.CompletionPricePerK)
	return out, nil
}

func (c *Client) simulateGenerate(ctx context.Context, info ModelInfo, req *GenerateRequest) (*GenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrModelGenerationFailed, err)
	}

	totalRunes := 0
	var lastUserMsg string
	var systemMsg string
	for _, m := range req.Messages {
		totalRunes += len([]rune(m.Content))
		if m.Role == "user" {
			lastUserMsg = m.Content
		}
		if m.Role == "system" {
			systemMsg = m.Content
		}
	}

	promptTokens := EstimateTokensFromRunes(totalRunes)
	if promptTokens < 10 {
		promptTokens = 10
	}

	hasZeroWeb := strings.Contains(systemMsg, "未找到相关网页来源")
	hasZeroRAG := strings.Contains(systemMsg, "未检索到当前用户有权访问的相关资料")

	var b strings.Builder
	b.WriteString("[本地仿真响应] ")
	if hasZeroWeb && hasZeroRAG {
		b.WriteString("未找到相关网页来源，且未检索到当前有权访问的知识库依据，当前证据不足以给出确切结论，建议补充问题背景或检查课程授权。")
	} else {
		if hasZeroWeb {
			b.WriteString("本次联网搜索未找到相关网页来源，以下仅依据已授权知识库内容作答：")
		} else {
			b.WriteString(fmt.Sprintf("依据检索到的授权知识证据与实时联网信息，针对「%s」的说明如下：", lastUserMsg))
		}
		b.WriteString("\n1. 已结合提供的知识片段与联网摘要完成核对；\n2. 详细信息请核对附带的来源引用与链接。")
	}

	replyContent := b.String()
	completionTokens := EstimateTokensFromRunes(len([]rune(replyContent)))
	if completionTokens < 20 {
		completionTokens = 20
	}
	costCents := CalculateCostCents(promptTokens, completionTokens, info.PromptPricePerK, info.CompletionPricePerK)

	return &GenerateResponse{
		Content:           replyContent,
		ProviderRequestID: fmt.Sprintf("sim_%s_%d", info.ProviderID, c.nowFunc().UnixNano()),
		PriceVersion:      info.PriceVersion,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		CostCents:         costCents,
		Simulated:         true,
	}, nil
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
