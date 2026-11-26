package provider

// SearchResultItem represents a single organic web search result.
type SearchResultItem struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// SearchExecutionResult holds output and billing metadata of the mandatory search step.
type SearchExecutionResult struct {
	Query      string             `json:"query"`
	Status     string             `json:"status"` // "success", "zero_results", "failed"
	Results    []SearchResultItem `json:"results"`
	DurationMs int                `json:"durationMs"`
	Attempts   int                `json:"attempts"`
	CostCents  int                `json:"costCents"`
	Simulated  bool               `json:"simulated,omitempty"`
}
