package assistant

import (
	providerapi "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/provider"
	agentapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/agent"
	knowledgeapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/knowledge"
	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
)

// Handler serves the Agent HTTP API.
type Handler struct {
	sessionResolver  SessionResolver
	usageService     *usageapp.Service
	knowledgeService *knowledgeapp.Service
	modelClient      *providerapi.Client
	agentService     *agentapp.Service
}

// NewHandler connects the Agent services to the HTTP API.
func NewHandler(resolve SessionResolver, usage *usageapp.Service, knowledge *knowledgeapp.Service, model *providerapi.Client, agent *agentapp.Service) *Handler {
	return &Handler{
		sessionResolver:  resolve,
		usageService:     usage,
		knowledgeService: knowledge,
		modelClient:      model,
		agentService:     agent,
	}
}
