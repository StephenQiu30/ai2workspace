package agent

import (
	"context"

	agentdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/agent"
	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
)

// StoredState contains the records restored when the use case starts.
type StoredState struct {
	Agents   []*agentdomain.Configuration
	Sessions []*agentdomain.Session
	Messages []*agentdomain.Message
	Runs     []*agentdomain.Run
}

// Store persists agent configurations and chat state.
type Store interface {
	Load(ctx context.Context, defaults []*agentdomain.Configuration) StoredState
	UpsertAgent(ctx context.Context, ag *agentdomain.Configuration) error
	UpsertSession(ctx context.Context, sess *agentdomain.Session) error
	UpsertRun(ctx context.Context, r *agentdomain.Run) error
	PersistAskResult(
		ctx context.Context,
		sessionID string,
		run *agentdomain.Run,
		userMsg *agentdomain.Message,
		assistantMsg *agentdomain.Message,
		searchRes *providerdomain.SearchExecutionResult,
	) error
}

func (s *Service) restore(state StoredState) {
	for _, agent := range state.Agents {
		s.agents[agent.ID] = agent
		s.agentsBySlug[agent.Slug] = agent
	}
	for _, session := range state.Sessions {
		s.sessions[session.ID] = session
	}
	for _, msg := range state.Messages {
		s.messages[msg.SessionID] = append(s.messages[msg.SessionID], msg)
		s.messagesByID[msg.ID] = msg
	}
	for _, run := range state.Runs {
		s.runs[run.ID] = run
		if run.IdempotencyKey != "" {
			s.runsByIdemKey[run.UserID+":"+run.IdempotencyKey] = run
		}
	}
}
