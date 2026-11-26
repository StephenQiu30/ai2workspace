package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
)

// SearchEngineOptions configures DefaultSearchEngine.
type SearchEngineOptions struct {
	APIURL               string
	APIKey               string
	AllowLocalSimulation bool
	HTTPClient           *http.Client
	AttemptTimeout       time.Duration
}

// DefaultSearchEngine provides real HTTP web search execution with retry, Retry-After, and timeout governance.
type DefaultSearchEngine struct {
	APIURL               string
	APIKey               string
	AllowLocalSimulation bool
	HTTPClient           *http.Client
	AttemptTimeout       time.Duration
}

// NewSearchEngine creates a search engine from runtime configuration.
func NewSearchEngine(cfg *Config) *DefaultSearchEngine {
	opts := SearchEngineOptions{}
	if cfg != nil {
		opts.APIURL = cfg.SearchAPIURL
		opts.APIKey = cfg.SearchAPIKey
		opts.AllowLocalSimulation = cfg.AllowLocalSimulation
	}
	return NewSearchEngineWithOptions(opts)
}

// NewSearchEngineWithOptions creates a search engine with explicit options.
func NewSearchEngineWithOptions(opts SearchEngineOptions) *DefaultSearchEngine {
	cli := opts.HTTPClient
	if cli == nil {
		cli = &http.Client{Timeout: 10 * time.Second}
	}
	attemptTimeout := opts.AttemptTimeout
	if attemptTimeout <= 0 {
		attemptTimeout = 8 * time.Second
	}
	return &DefaultSearchEngine{
		APIURL:               strings.TrimSpace(opts.APIURL),
		APIKey:               strings.TrimSpace(opts.APIKey),
		AllowLocalSimulation: opts.AllowLocalSimulation,
		HTTPClient:           cli,
		AttemptTimeout:       attemptTimeout,
	}
}

type retryableSearchError struct {
	err        error
	retryAfter time.Duration
}

func (e *retryableSearchError) Error() string { return e.err.Error() }

func (e *retryableSearchError) Unwrap() error { return e.err }

// Search executes search with max 2 attempts (1 retry on network/timeout/429/5xx) and 8s timeout per attempt.
func (e *DefaultSearchEngine) Search(ctx context.Context, query string) (*providerdomain.SearchExecutionResult, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return &providerdomain.SearchExecutionResult{
			Query:      query,
			Status:     "failed",
			Results:    []providerdomain.SearchResultItem{},
			DurationMs: 0,
			Attempts:   0,
			CostCents:  0,
		}, fmt.Errorf("%w: empty search query", providerdomain.ErrSearchFailed)
	}

	if e.APIURL == "" && !e.AllowLocalSimulation {
		return &providerdomain.SearchExecutionResult{
			Query:      cleanQuery,
			Status:     "failed",
			Results:    []providerdomain.SearchResultItem{},
			DurationMs: 0,
			Attempts:   0,
			CostCents:  0,
		}, providerdomain.ErrSearchNotConfigured
	}

	start := time.Now()
	var lastErr error
	attempts := 0
	billedCost := 0

	for attempt := 1; attempt <= 2; attempt++ {
		if err := ctx.Err(); err != nil {
			lastErr = err
			break
		}
		attempts = attempt

		attemptCtx, cancel := context.WithTimeout(ctx, e.AttemptTimeout)
		var (
			res       []providerdomain.SearchResultItem
			simulated bool
			billable  bool
			err       error
		)
		if e.APIURL != "" {
			res, billable, err = e.doHTTPAttempt(attemptCtx, cleanQuery)
		} else {
			simulated = true
			res, billable, err = e.doSimulatedAttempt(attemptCtx, cleanQuery)
		}
		cancel()

		if billable {
			billedCost++
		}

		if err == nil {
			duration := int(time.Since(start).Milliseconds())
			status := "success"
			if len(res) == 0 {
				status = "zero_results"
			}
			return &providerdomain.SearchExecutionResult{
				Query:      cleanQuery,
				Status:     status,
				Results:    res,
				DurationMs: duration,
				Attempts:   attempts,
				CostCents:  billedCost,
				Simulated:  simulated,
			}, nil
		}

		lastErr = err

		var retryErr *retryableSearchError
		if !errors.As(err, &retryErr) || attempt >= 2 {
			break
		}

		// Respect Retry-After against remaining evidence deadline.
		if retryErr.retryAfter > 0 {
			if deadline, ok := ctx.Deadline(); ok {
				if time.Until(deadline) <= retryErr.retryAfter {
					break
				}
			}
			timer := time.NewTimer(retryErr.retryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				lastErr = ctx.Err()
				break
			case <-timer.C:
			}
		}
	}

	duration := int(time.Since(start).Milliseconds())
	out := &providerdomain.SearchExecutionResult{
		Query:      cleanQuery,
		Status:     "failed",
		Results:    []providerdomain.SearchResultItem{},
		DurationMs: duration,
		Attempts:   attempts,
		CostCents:  billedCost,
		Simulated:  e.APIURL == "",
	}
	if errors.Is(lastErr, providerdomain.ErrSearchAuthFailed) {
		return out, lastErr
	}
	if errors.Is(lastErr, context.DeadlineExceeded) {
		return out, fmt.Errorf("%w: %v", providerdomain.ErrSearchTimeout, lastErr)
	}
	return out, fmt.Errorf("%w: %v", providerdomain.ErrSearchFailed, lastErr)
}

type httpSearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type httpSearchResponse struct {
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Snippet string  `json:"snippet"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
	Data []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Snippet string  `json:"snippet"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"data"`
}

func (e *DefaultSearchEngine) doHTTPAttempt(ctx context.Context, query string) ([]providerdomain.SearchResultItem, bool, error) {
	bodyBytes, err := json.Marshal(httpSearchRequest{Query: query, MaxResults: 5})
	if err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.APIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		// Network / timeout errors are retryable.
		return nil, false, &retryableSearchError{err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, true, &retryableSearchError{err: err}
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, false, providerdomain.ErrSearchAuthFailed
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, false, &retryableSearchError{
			err:        fmt.Errorf("search provider returned HTTP %d", resp.StatusCode),
			retryAfter: retryAfter,
		}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, false, fmt.Errorf("search provider returned HTTP %d", resp.StatusCode)
	}

	var parsed httpSearchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// Unparseable response is a failure, NEVER disguised as zero_results!
		return nil, true, fmt.Errorf("invalid search response JSON: %w", err)
	}

	items := make([]providerdomain.SearchResultItem, 0, len(parsed.Results)+len(parsed.Data))
	for _, r := range parsed.Results {
		snippet := strings.TrimSpace(r.Snippet)
		if snippet == "" {
			snippet = strings.TrimSpace(r.Content)
		}
		items = append(items, providerdomain.SearchResultItem{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.URL),
			Snippet: snippet,
			Score:   r.Score,
		})
	}
	if len(items) == 0 {
		for _, r := range parsed.Data {
			snippet := strings.TrimSpace(r.Snippet)
			if snippet == "" {
				snippet = strings.TrimSpace(r.Content)
			}
			items = append(items, providerdomain.SearchResultItem{
				Title:   strings.TrimSpace(r.Title),
				URL:     strings.TrimSpace(r.URL),
				Snippet: snippet,
				Score:   r.Score,
			})
		}
	}
	return items, true, nil
}

func parseRetryAfter(val string) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func (e *DefaultSearchEngine) doSimulatedAttempt(ctx context.Context, query string) ([]providerdomain.SearchResultItem, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, &retryableSearchError{err: err}
	}
	lower := strings.ToLower(query)
	if strings.Contains(lower, "error_mock") {
		return nil, false, &retryableSearchError{err: errors.New("transient network 503: search provider unavailable")}
	}
	if strings.Contains(lower, "auth_fail_mock") {
		return nil, false, providerdomain.ErrSearchAuthFailed
	}
	if strings.Contains(lower, "empty_mock") {
		return []providerdomain.SearchResultItem{}, true, nil
	}

	escaped := url.QueryEscape(query)
	return []providerdomain.SearchResultItem{
		{
			Title:   fmt.Sprintf("[本地仿真] 关于「%s」的最新技术实践与规范", query),
			URL:     fmt.Sprintf("https://docs.ai2workspace.internal/search?q=%s", escaped),
			Snippet: fmt.Sprintf("本地仿真摘要：涵盖 %s 的定义、架构分层原则、接口幂等及权限隔离核心要点。", query),
			Score:   0.95,
		},
		{
			Title:   fmt.Sprintf("[本地仿真] %s 核心问答与设计取舍", query),
			URL:     fmt.Sprintf("https://wiki.ai2workspace.internal/knowledge?q=%s", escaped),
			Snippet: fmt.Sprintf("本地仿真摘要：详细解析 %s 在工程落地中的常见问题与最佳实践指引。", query),
			Score:   0.88,
		},
	}, true, nil
}
