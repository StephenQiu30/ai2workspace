package provider

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SearchResultItem represents an organic search result.
type SearchResultItem struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// SearchExecutionResult holds output of the mandatory search step.
type SearchExecutionResult struct {
	Query      string             `json:"query"`
	Status     string             `json:"status"` // "success", "zero_results", "failed"
	Results    []SearchResultItem `json:"results"`
	DurationMs int                `json:"durationMs"`
	Attempts   int                `json:"attempts"`
	CostCents  int                `json:"costCents"`
}

// SearchEngine executes web searches.
type SearchEngine interface {
	Search(ctx context.Context, query string) (*SearchExecutionResult, error)
}

// DefaultSearchEngine provides web search execution with retry and timeout rules.
type DefaultSearchEngine struct {
	Endpoint string
}

// NewSearchEngine creates a new search engine instance.
func NewSearchEngine() *DefaultSearchEngine {
	return &DefaultSearchEngine{
		Endpoint: "https://api.search.internal",
	}
}

// Search executes search with up to 1 retry on transient failure.
func (e *DefaultSearchEngine) Search(ctx context.Context, query string) (*SearchExecutionResult, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return &SearchExecutionResult{
			Query:      query,
			Status:     "zero_results",
			Results:    []SearchResultItem{},
			DurationMs: 5,
			Attempts:   1,
			CostCents:  0,
		}, nil
	}

	start := time.Now()
	var lastErr error
	attempts := 0

	for attempt := 1; attempt <= 2; attempt++ {
		attempts = attempt
		// Each attempt has max 8 seconds deadline
		attemptCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		res, err := e.doSingleSearch(attemptCtx, cleanQuery)
		cancel()

		if err == nil {
			duration := int(time.Since(start).Milliseconds())
			status := "success"
			if len(res) == 0 {
				status = "zero_results"
			}
			return &SearchExecutionResult{
				Query:      cleanQuery,
				Status:     status,
				Results:    res,
				DurationMs: duration,
				Attempts:   attempts,
				CostCents:  1, // 1 cent per search query
			}, nil
		}

		lastErr = err
		// Non-retryable error check (e.g. auth error)
		if strings.Contains(err.Error(), "auth") || strings.Contains(err.Error(), "unauthorized") {
			break
		}
	}

	duration := int(time.Since(start).Milliseconds())
	return &SearchExecutionResult{
		Query:      cleanQuery,
		Status:     "failed",
		Results:    []SearchResultItem{},
		DurationMs: duration,
		Attempts:   attempts,
		CostCents:  0,
	}, fmt.Errorf("%w: %v", ErrSearchFailed, lastErr)
}

func (e *DefaultSearchEngine) doSingleSearch(ctx context.Context, query string) ([]SearchResultItem, error) {
	// Simulated robust search result generator
	if strings.Contains(strings.ToLower(query), "error_mock") {
		return nil, fmt.Errorf("transient network 503: search provider unavailable")
	}
	if strings.Contains(strings.ToLower(query), "auth_fail_mock") {
		return nil, ErrSearchAuthFailed
	}
	if strings.Contains(strings.ToLower(query), "empty_mock") {
		return []SearchResultItem{}, nil
	}

	results := []SearchResultItem{
		{
			Title:   fmt.Sprintf("关于「%s」的最新技术实践与规范", query),
			URL:     fmt.Sprintf("https://docs.ai2workspace.internal/search?q=%s", query),
			Snippet: fmt.Sprintf("涵盖 %s 的定义、架构分层原则、接口幂等及权限隔离核心要点。", query),
			Score:   0.95,
		},
		{
			Title:   fmt.Sprintf("%s 核心问答与设计取舍", query),
			URL:     fmt.Sprintf("https://wiki.ai2workspace.internal/knowledge/%s", query),
			Snippet: fmt.Sprintf("详细解析 %s 在工程落地中的常见问题与最佳实践指引。", query),
			Score:   0.88,
		},
	}

	return results, nil
}
