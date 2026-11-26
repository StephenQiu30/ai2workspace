package agent

import (
	"errors"
	"time"

	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
)

var (
	ErrAgentNotFound         = errors.New("agent configuration not found")
	ErrAgentDisabled         = errors.New("agent is disabled or not published")
	ErrSlugAlreadyExists     = errors.New("agent slug already exists")
	ErrInvalidModel          = errors.New("configured model is not allowed, disabled, or expired")
	ErrInvalidKnowledgeBase  = errors.New("bound knowledge base does not exist")
	ErrVersionMismatch       = errors.New("optimistic lock version mismatch")
	ErrSessionNotFound       = errors.New("agent session not found")
	ErrRunNotFound           = errors.New("agent run not found")
	ErrUnauthorizedAccess    = errors.New("unauthorized to access this session")
	ErrEmptyQuestion         = errors.New("question cannot be empty")
	ErrUnsafeSearchQuery     = errors.New("question cannot be sanitized into a safe outbound search query")
	ErrIdempotencyConflict   = errors.New("idempotency key reused with a different question")
	ErrRunInProgress         = errors.New("a run with this idempotency key is still in progress")
	ErrRunCancelled          = errors.New("agent run was cancelled")
	ErrCitationAccessRevoked = errors.New("knowledge source was revoked or course access expired before delivery")
)

// AgentState defines configuration lifecycle state.
type AgentState string

const (
	StateDraft     AgentState = "draft"
	StatePublished AgentState = "published"
	StateDisabled  AgentState = "disabled"
)

// RunState defines execution lifecycle state for an ask run.
type RunState string

const (
	RunStateAdmitted   RunState = "admitted"
	RunStateRetrieving RunState = "retrieving"
	RunStateGenerating RunState = "generating"
	RunStateCompleted  RunState = "completed"
	RunStateFailed     RunState = "failed"
	RunStateCancelled  RunState = "cancelled"
)

// Configuration defines an agent's runtime parameters.
type Configuration struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	RoleInstruction  string     `json:"roleInstruction,omitempty"`
	ProviderID       string     `json:"providerId"`
	ModelID          string     `json:"modelId"`
	KnowledgeBaseIDs []string   `json:"knowledgeBaseIds"`
	DraftVersion     int        `json:"draftVersion"`
	PublishedVersion int        `json:"publishedVersion"`
	State            AgentState `json:"state"`
	DisabledReason   string     `json:"disabledReason,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// UpdateAgentInput defines editable fields for an existing agent configuration.
type UpdateAgentInput struct {
	ExpectedVersion  int       `json:"expectedVersion"`
	Name             *string   `json:"name,omitempty"`
	Slug             *string   `json:"slug,omitempty"`
	RoleInstruction  *string   `json:"roleInstruction,omitempty"`
	ProviderID       *string   `json:"providerId,omitempty"`
	ModelID          *string   `json:"modelId,omitempty"`
	KnowledgeBaseIDs *[]string `json:"knowledgeBaseIds,omitempty"`
}

// Session represents a chat conversation belonging to a single user.
type Session struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentId"`
	UserID    string    `json:"userId"`
	CourseID  string    `json:"courseId,omitempty"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Run tracks a single question-answering execution, supporting cancellation and idempotency.
type Run struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"sessionId"`
	AgentID        string    `json:"agentId"`
	UserID         string    `json:"userId"`
	IdempotencyKey string    `json:"idempotencyKey,omitempty"`
	QuestionHash   string    `json:"questionHash,omitempty"`
	State          RunState  `json:"state"`
	Phase          string    `json:"phase"`
	MessageID      string    `json:"messageId,omitempty"`
	ErrorCode      string    `json:"errorCode,omitempty"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Message represents a single conversational message with traceable citations and web search evidence.
type Message struct {
	ID            string                            `json:"id"`
	SessionID     string                            `json:"sessionId"`
	RunID         string                            `json:"runId,omitempty"`
	Sender        string                            `json:"sender"` // "user", "assistant"
	Content       string                            `json:"content"`
	Redacted      bool                              `json:"redacted,omitempty"`
	Citations     []knowledgedomain.Citation        `json:"citations"`
	SearchQueries []providerdomain.SearchResultItem `json:"searchQueries"`
	SearchStatus  string                            `json:"searchStatus,omitempty"`
	TokenUsage    map[string]int                    `json:"tokenUsage,omitempty"`
	CreatedAt     time.Time                         `json:"createdAt"`
}
