package provider

import (
	"context"

	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
)

// ModelClient performs text generation and catalog validation.
type ModelClient interface {
	Generate(ctx context.Context, req *providerdomain.GenerateRequest) (*providerdomain.GenerateResponse, error)
	ValidateModelForCall(providerID, modelID string) (providerdomain.ModelInfo, error)
}
