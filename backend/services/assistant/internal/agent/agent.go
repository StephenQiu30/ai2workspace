// Package agent provides configuration, version publication, and answer orchestration for assistant agents.
package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/provider"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/usage"
)

var (
	ErrAgentNotFound      = errors.New("agent configuration not found")
	ErrAgentDisabled      = errors.New("agent is disabled or not published")
	ErrSessionNotFound    = errors.New("agent session not found")
	ErrUnauthorizedAccess = errors.New("unauthorized to access this session")
	ErrEmptyQuestion      = errors.New("question cannot be empty")
)

// AgentState defines configuration state.
type AgentState string

const (
	StateDraft     AgentState = "draft"
	StatePublished AgentState = "published"
	StateDisabled  AgentState = "disabled"
)

// Configuration defines an agent's runtime parameters.
type Configuration struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	RoleInstruction  string     `json:"roleInstruction"`
	ProviderID       string     `json:"providerId"`
	ModelID          string     `json:"modelId"`
	KnowledgeBaseIDs []string   `json:"knowledgeBaseIds"`
	DraftVersion     int        `json:"draftVersion"`
	PublishedVersion int        `json:"publishedVersion"`
	State            AgentState `json:"state"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// Session represents a chat conversation.
type Session struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentId"`
	UserID    string    `json:"userId"`
	CourseID  string    `json:"courseId,omitempty"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Message represents a single message in the conversation.
type Message struct {
	ID            string                      `json:"id"`
	SessionID     string                      `json:"sessionId"`
	Sender        string                      `json:"sender"` // "user", "assistant"
	Content       string                      `json:"content"`
	Citations     []knowledge.Citation        `json:"citations"`
	SearchQueries []provider.SearchResultItem `json:"searchQueries"`
	TokenUsage    map[string]int              `json:"tokenUsage,omitempty"`
	CreatedAt     time.Time                   `json:"createdAt"`
}

// Service manages agents and chat execution.
type Service struct {
	mu           sync.RWMutex
	agents       map[string]*Configuration
	agentsBySlug map[string]*Configuration
	sessions     map[string]*Session
	messages     map[string][]*Message

	usageService     *usage.Service
	knowledgeService *knowledge.Service
	modelClient      provider.ModelClient
	searchEngine     provider.SearchEngine
}

// NewService creates a new agent orchestration service.
func NewService(
	usageSvc *usage.Service,
	knowledgeSvc *knowledge.Service,
	modelCli provider.ModelClient,
	searchEng provider.SearchEngine,
) *Service {
	s := &Service{
		agents:           make(map[string]*Configuration),
		agentsBySlug:     make(map[string]*Configuration),
		sessions:         make(map[string]*Session),
		messages:         make(map[string][]*Message),
		usageService:     usageSvc,
		knowledgeService: knowledgeSvc,
		modelClient:      modelCli,
		searchEngine:     searchEng,
	}

	// Initialize default published agents: customer_support and course_tutor
	now := time.Now()
	csAgent := &Configuration{
		ID:               "agent_cs_01",
		Name:             "学屿官方服务助手",
		Slug:             "customer_support",
		RoleInstruction:  "你是学屿学习社区的官方服务助手。基于官方指南与实时搜索信息为用户解答平台使用、订单、学习流程与常见问题。",
		ProviderID:       "deepseek",
		ModelID:          "deepseek-chat",
		KnowledgeBaseIDs: []string{},
		DraftVersion:     1,
		PublishedVersion: 1,
		State:            StatePublished,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	tutorAgent := &Configuration{
		ID:               "agent_tutor_01",
		Name:             "学屿课程专属助教",
		Slug:             "course_tutor",
		RoleInstruction:  "你是学屿课程的专业 AI 助教。结合当前课程授权资料与实时搜索，用专业严谨、鼓励学员的方式解答课程疑难与技术实践。",
		ProviderID:       "openai",
		ModelID:          "gpt-4o-mini",
		KnowledgeBaseIDs: []string{},
		DraftVersion:     1,
		PublishedVersion: 1,
		State:            StatePublished,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.agents[csAgent.ID] = csAgent
	s.agentsBySlug[csAgent.Slug] = csAgent
	s.agents[tutorAgent.ID] = tutorAgent
	s.agentsBySlug[tutorAgent.Slug] = tutorAgent

	return s
}

// CreateAgent creates or drafts a new agent configuration.
func (s *Service) CreateAgent(ctx context.Context, name, slug, instruction, providerID, modelID string, kbIDs []string) (*Configuration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("agent_%d", now.UnixNano())
	cfg := &Configuration{
		ID:               id,
		Name:             name,
		Slug:             slug,
		RoleInstruction:  instruction,
		ProviderID:       providerID,
		ModelID:          modelID,
		KnowledgeBaseIDs: kbIDs,
		DraftVersion:     1,
		PublishedVersion: 0,
		State:            StateDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.agents[id] = cfg
	s.agentsBySlug[slug] = cfg
	return cfg, nil
}

// PublishAgent publishes draft version.
func (s *Service) PublishAgent(ctx context.Context, agentID string) (*Configuration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, exists := s.agents[agentID]
	if !exists {
		return nil, ErrAgentNotFound
	}
	cfg.PublishedVersion = cfg.DraftVersion
	cfg.State = StatePublished
	cfg.UpdatedAt = time.Now()
	return cfg, nil
}

// ListAgents returns all agents.
func (s *Service) ListAgents(ctx context.Context) []*Configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Configuration, 0, len(s.agents))
	for _, a := range s.agents {
		res = append(res, a)
	}
	return res
}

// GetAgentBySlug retrieves agent by slug.
func (s *Service) GetAgentBySlug(ctx context.Context, slug string) (*Configuration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, exists := s.agentsBySlug[slug]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return cfg, nil
}

// CreateSession creates a conversation session.
func (s *Service) CreateSession(ctx context.Context, agentSlug, userID, courseID, title string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agentsBySlug[agentSlug]
	if !exists {
		return nil, ErrAgentNotFound
	}
	if agent.State != StatePublished {
		return nil, ErrAgentDisabled
	}

	if title == "" {
		title = "新对话"
	}

	now := time.Now()
	sessID := fmt.Sprintf("sess_%d_%s", now.UnixNano(), userID)
	sess := &Session{
		ID:        sessID,
		AgentID:   agent.ID,
		UserID:    userID,
		CourseID:  courseID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.sessions[sessID] = sess
	s.messages[sessID] = []*Message{}
	return sess, nil
}

// GetSession retrieves session.
func (s *Service) GetSession(ctx context.Context, sessionID, userID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	if sess.UserID != userID {
		return nil, ErrUnauthorizedAccess
	}
	return sess, nil
}

// ListMessages returns history of a session.
func (s *Service) ListMessages(ctx context.Context, sessionID, userID string) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	if sess.UserID != userID {
		return nil, ErrUnauthorizedAccess
	}

	msgs := s.messages[sessionID]
	return msgs, nil
}

// Ask orchestrates full question answering with mandatory search, RAG and budget settlement.
func (s *Service) Ask(ctx context.Context, sessionID, userID, question string) (*Message, error) {
	trimmedQ := question
	if len(trimmedQ) == 0 {
		return nil, ErrEmptyQuestion
	}

	s.mu.RLock()
	sess, exists := s.sessions[sessionID]
	if !exists {
		s.mu.RUnlock()
		return nil, ErrSessionNotFound
	}
	if sess.UserID != userID {
		s.mu.RUnlock()
		return nil, ErrUnauthorizedAccess
	}
	agent, exists := s.agents[sess.AgentID]
	if !exists || agent.State != StatePublished {
		s.mu.RUnlock()
		return nil, ErrAgentDisabled
	}
	s.mu.RUnlock()

	// 1. Budget reservation (estimate 10 cents hold)
	reservation, err := s.usageService.Reserve(ctx, userID, usage.PurposeAgentChat, 10)
	if err != nil {
		return nil, err
	}

	// 2. Mandatory live web search (max 8s per try, max 1 retry)
	searchRes, err := s.searchEngine.Search(ctx, trimmedQ)
	if err != nil {
		// Mandatory search failed: release reservation and abort model call!
		_ = s.usageService.Release(ctx, reservation.ID)
		return nil, fmt.Errorf("mandatory web search failed: %w", err)
	}

	// 3. Authorized RAG retrieval from bound knowledge bases
	citations, err := s.knowledgeService.RetrieveChunks(ctx, userID, agent.KnowledgeBaseIDs, trimmedQ, 3)
	if err != nil {
		_ = s.usageService.Release(ctx, reservation.ID)
		return nil, fmt.Errorf("knowledge retrieval failed: %w", err)
	}

	// 4. Model generation
	systemPrompt := fmt.Sprintf("%s\n\n【实时网络证据】: 找到 %d 条外部事实；\n【授权资料引用】: 检索到 %d 条可用片段。",
		agent.RoleInstruction, len(searchRes.Results), len(citations))

	modelReq := &provider.GenerateRequest{
		ProviderID: agent.ProviderID,
		ModelID:    agent.ModelID,
		Messages: []provider.ModelMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: trimmedQ},
		},
		MaxTokens:   1500,
		Temperature: 0.7,
	}

	genRes, err := s.modelClient.Generate(ctx, modelReq)
	if err != nil {
		_ = s.usageService.Release(ctx, reservation.ID)
		return nil, fmt.Errorf("model generation failed: %w", err)
	}

	// 5. Budget settlement
	steps := []usage.SettlementStep{
		{
			Step:      "search",
			Provider:  "internal_search",
			ModelID:   "web-search",
			CostCents: searchRes.CostCents,
		},
		{
			Step:             "llm",
			Provider:         agent.ProviderID,
			ModelID:          agent.ModelID,
			PromptTokens:     genRes.PromptTokens,
			CompletionTokens: genRes.CompletionTokens,
			CostCents:        genRes.CostCents,
		},
	}
	_, _ = s.usageService.Settle(ctx, reservation.ID, steps)

	// 6. Record user and assistant messages
	now := time.Now()
	userMsg := &Message{
		ID:        fmt.Sprintf("msg_%d_u", now.UnixNano()),
		SessionID: sessionID,
		Sender:    "user",
		Content:   trimmedQ,
		CreatedAt: now,
	}

	assistantMsg := &Message{
		ID:            fmt.Sprintf("msg_%d_a", now.UnixNano()+1),
		SessionID:     sessionID,
		Sender:        "assistant",
		Content:       genRes.Content,
		Citations:     citations,
		SearchQueries: searchRes.Results,
		TokenUsage: map[string]int{
			"promptTokens":     genRes.PromptTokens,
			"completionTokens": genRes.CompletionTokens,
			"costCents":        genRes.CostCents + searchRes.CostCents,
		},
		CreatedAt: now.Add(time.Millisecond * 10),
	}

	s.mu.Lock()
	s.messages[sessionID] = append(s.messages[sessionID], userMsg, assistantMsg)
	sess.UpdatedAt = now
	s.mu.Unlock()

	return assistantMsg, nil
}
