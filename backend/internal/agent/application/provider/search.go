package provider

import (
	"context"

	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
)

// SearchEngine executes live web searches.
type SearchEngine interface {
	Search(ctx context.Context, query string) (*providerdomain.SearchExecutionResult, error)
}
