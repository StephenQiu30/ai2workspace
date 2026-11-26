package agent

import (
	providerapi "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/provider"
)

// Config configures the assistant's model and search providers.
type Config = providerapi.Config

// LoadConfig loads the assistant's runtime configuration.
func LoadConfig() (*Config, error) {
	return providerapi.LoadConfig()
}
