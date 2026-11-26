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

	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ClientOptions configures real HTTP provider credentials and optional simulation mode.
type ClientOptions struct {
	Endpoints            map[string]Endpoint
	AllowLocalSimulation bool
	HTTPClient           *http.Client
	DBPool               *pgxpool.Pool
	NowFunc              func() time.Time
}

// Client implements ModelClient with strict catalog validation and real HTTP adapters.
type Client struct {
	mu                   sync.RWMutex
	catalog              map[string]providerdomain.ModelInfo
	endpoints            map[string]Endpoint
	allowLocalSimulation bool
	httpClient           *http.Client
	store                *catalogStore
	nowFunc              func() time.Time
}

// NewClient creates a unified model client from runtime configuration.
func NewClient(cfg *Config) *Client {
	opts := ClientOptions{}
	if cfg != nil {
		opts.AllowLocalSimulation = cfg.AllowLocalSimulation
		opts.Endpoints = map[string]Endpoint{
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

	endpoints := map[string]Endpoint{
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

	cat := make(map[string]providerdomain.ModelInfo)
	for _, m := range providerdomain.DefaultCatalog(nowFn()) {
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
func (c *Client) ListCatalog(includeDisabled bool) []providerdomain.ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := c.nowFunc()
	res := make([]providerdomain.ModelInfo, 0, len(c.catalog))
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
func (c *Client) GetModel(providerID, modelID string) (providerdomain.ModelInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.catalog[catalogKey(providerID, modelID)]
	return m, ok
}

// ValidateModelForCall verifies that the model exists, is enabled, and has valid unexpired pricing.
func (c *Client) ValidateModelForCall(providerID, modelID string) (providerdomain.ModelInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info, exists := c.catalog[catalogKey(providerID, modelID)]
	if !exists || !info.Enabled {
		return providerdomain.ModelInfo{}, providerdomain.ErrModelNotAllowed
	}
	now := c.nowFunc()
	if !info.EffectiveFrom.IsZero() && now.Before(info.EffectiveFrom) {
		return providerdomain.ModelInfo{}, providerdomain.ErrModelPriceExpired
	}
	if !info.ExpiresAt.IsZero() && !now.Before(info.ExpiresAt) {
		return providerdomain.ModelInfo{}, providerdomain.ErrModelPriceExpired
	}
	if info.PromptPricePerK < 0 || info.CompletionPricePerK < 0 || (info.PromptPricePerK == 0 && info.CompletionPricePerK == 0) {
		return providerdomain.ModelInfo{}, providerdomain.ErrModelPriceExpired
	}
	return info, nil
}

// UpdateModel updates a model's enabled state, price version, unit prices, or validity window (admin only).
func (c *Client) UpdateModel(ctx context.Context, providerID, modelID string, update providerdomain.ModelUpdate) (providerdomain.ModelInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := catalogKey(providerID, modelID)
	info, exists := c.catalog[key]
	if !exists {
		return providerdomain.ModelInfo{}, providerdomain.ErrModelNotAllowed
	}
	if update.DisplayName != nil && strings.TrimSpace(*update.DisplayName) != "" {
		info.DisplayName = strings.TrimSpace(*update.DisplayName)
	}
	if update.PriceVersion != nil && strings.TrimSpace(*update.PriceVersion) != "" {
		info.PriceVersion = strings.TrimSpace(*update.PriceVersion)
	}
	if update.PromptPricePerK != nil {
		if *update.PromptPricePerK < 0 {
			return providerdomain.ModelInfo{}, errors.New("promptPricePerK must be non-negative")
		}
		info.PromptPricePerK = *update.PromptPricePerK
	}
	if update.CompletionPricePerK != nil {
		if *update.CompletionPricePerK < 0 {
			return providerdomain.ModelInfo{}, errors.New("completionPricePerK must be non-negative")
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
			return providerdomain.ModelInfo{}, err
		}
	}
	c.catalog[key] = info
	return info, nil
}

type chatCompletionRequest struct {
	Model       string                        `json:"model"`
	Messages    []providerdomain.ModelMessage `json:"messages"`
	MaxTokens   int                           `json:"max_tokens,omitempty"`
	Temperature float64                       `json:"temperature,omitempty"`
	Stream      bool                          `json:"stream"`
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
func (c *Client) Generate(ctx context.Context, req *providerdomain.GenerateRequest) (*providerdomain.GenerateResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: nil request", providerdomain.ErrModelGenerationFailed)
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
		return nil, fmt.Errorf("%w: requested %d > limit %d", providerdomain.ErrOutputLimitExceeded, maxTokens, info.MaxOutputTokens)
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
		return nil, fmt.Errorf("%w: provider=%s model=%s", providerdomain.ErrProviderNotConfigured, req.ProviderID, req.ModelID)
	}

	return c.simulateGenerate(ctx, info, req)
}

func (c *Client) callChatCompletionsHTTP(
	ctx context.Context,
	httpCli *http.Client,
	ep Endpoint,
	info providerdomain.ModelInfo,
	req *providerdomain.GenerateRequest,
	maxTokens int,
) (*providerdomain.GenerateResponse, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(ep.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: missing base URL for provider %s", providerdomain.ErrProviderNotConfigured, req.ProviderID)
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
		return nil, fmt.Errorf("%w: marshal request: %v", providerdomain.ErrModelGenerationFailed, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", providerdomain.ErrModelGenerationFailed, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+ep.APIKey)
	if req.ProviderID == "openrouter" {
		httpReq.Header.Set("X-Title", "ai2workspace")
	}

	resp, err := httpCli.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", providerdomain.ErrModelGenerationFailed, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %w", providerdomain.ErrModelGenerationFailed, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errEnvelope chatCompletionResponse
		_ = json.Unmarshal(respBody, &errEnvelope)
		if errEnvelope.Error != nil && errEnvelope.Error.Message != "" {
			return nil, fmt.Errorf("%w: provider=%s status=%d msg=%s", providerdomain.ErrModelGenerationFailed, req.ProviderID, resp.StatusCode, errEnvelope.Error.Message)
		}
		return nil, fmt.Errorf("%w: provider=%s status=%d", providerdomain.ErrModelGenerationFailed, req.ProviderID, resp.StatusCode)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%w: invalid provider JSON: %v", providerdomain.ErrModelGenerationFailed, err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%w: provider returned zero choices", providerdomain.ErrModelGenerationFailed)
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: provider returned empty content", providerdomain.ErrModelGenerationFailed)
	}

	out := &providerdomain.GenerateResponse{
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
	out.CostCents = providerdomain.CalculateCostCents(out.PromptTokens, out.CompletionTokens, info.PromptPricePerK, info.CompletionPricePerK)
	return out, nil
}

func (c *Client) simulateGenerate(ctx context.Context, info providerdomain.ModelInfo, req *providerdomain.GenerateRequest) (*providerdomain.GenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", providerdomain.ErrModelGenerationFailed, err)
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

	promptTokens := providerdomain.EstimateTokensFromRunes(totalRunes)
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
	completionTokens := providerdomain.EstimateTokensFromRunes(len([]rune(replyContent)))
	if completionTokens < 20 {
		completionTokens = 20
	}
	costCents := providerdomain.CalculateCostCents(promptTokens, completionTokens, info.PromptPricePerK, info.CompletionPricePerK)

	return &providerdomain.GenerateResponse{
		Content:           replyContent,
		ProviderRequestID: fmt.Sprintf("sim_%s_%d", info.ProviderID, c.nowFunc().UnixNano()),
		PriceVersion:      info.PriceVersion,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		CostCents:         costCents,
		Simulated:         true,
	}, nil
}
