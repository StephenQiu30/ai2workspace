package agent_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	providerapi "github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/provider"
	agentapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/agent"
	knowledgeapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/knowledge"
	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
	agentdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/agent"
	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
)

type spyModelClient struct {
	mu          sync.Mutex
	calls       []*providerdomain.GenerateRequest
	failWith    error
	delay       time.Duration
	inner       *providerapi.Client
	callCounter int
}

func (s *spyModelClient) ValidateModelForCall(providerID, modelID string) (providerdomain.ModelInfo, error) {
	return s.inner.ValidateModelForCall(providerID, modelID)
}

func (s *spyModelClient) Generate(ctx context.Context, req *providerdomain.GenerateRequest) (*providerdomain.GenerateResponse, error) {
	s.mu.Lock()
	s.calls = append(s.calls, req)
	s.callCounter++
	failWith := s.failWith
	delay := s.delay
	s.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	if failWith != nil {
		return nil, failWith
	}
	return s.inner.Generate(ctx, req)
}

func TestAgent_AskInjectsRealRAGEvidenceAndWebSnippets(t *testing.T) {
	ctx := context.Background()

	usageSvc := usageapp.NewService()
	buyerAllowed := true
	knowledgeSvc := knowledgeapp.NewService(func(ctx context.Context, userID, courseID string) (bool, error) {
		return buyerAllowed && userID == "usr_1001" && courseID == "crs_go", nil
	}, usageSvc)

	kb, _ := knowledgeSvc.CreateKnowledgeBase(ctx, "usr_admin", "Go 课程库", "", knowledgedomain.VisibilityPrivate)
	src, _ := knowledgeSvc.RegisterKnowledgeSource(ctx, kb.ID, "usr_admin", true, knowledgedomain.KindCourseText, "crs_go", "并发原语讲义", "使用有缓冲 Channel 与 context.Context 可优雅取消 Goroutine，防止协程泄漏。")
	_, _ = knowledgeSvc.StartIndexJob(ctx, src.ID)
	_ = knowledgeSvc.PublishKnowledgeSource(ctx, src.ID, 1)

	baseModel := providerapi.NewClientWithOptions(providerapi.ClientOptions{AllowLocalSimulation: true})
	spy := &spyModelClient{inner: baseModel}
	searchEng := providerapi.NewSearchEngineWithOptions(providerapi.SearchEngineOptions{AllowLocalSimulation: true})
	agentSvc := agentapp.NewService(usageSvc, knowledgeSvc, spy, searchEng)

	ag, err := agentSvc.CreateAgent(ctx, "Go 助教", "go_helper", "你是 Go 课程助教", "deepseek", "deepseek-chat", []string{kb.ID})
	if err != nil {
		t.Fatalf("create agent failed: %v", err)
	}
	if _, err := agentSvc.PublishAgent(ctx, ag.ID); err != nil {
		t.Fatalf("publish agent failed: %v", err)
	}

	sess, err := agentSvc.CreateSession(ctx, "go_helper", "usr_1001", "crs_go", "协程问题")
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	msg, err := agentSvc.AskWithOptions(ctx, sess.ID, "usr_1001", "如何防止 Goroutine 泄漏？", "idem_001")
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if msg.Content == "" || len(msg.SearchQueries) == 0 || len(msg.Citations) == 0 {
		t.Fatalf("expected content, web search results, and RAG citations, got %+v", msg)
	}

	// Verify REAL RAG text and web snippet were injected into system prompt!
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 model call, got %d", len(spy.calls))
	}
	sysPrompt := spy.calls[0].Messages[0].Content
	if !strings.Contains(sysPrompt, "[K1]") || !strings.Contains(sysPrompt, "防止协程泄漏") {
		t.Fatalf("expected RAG chunk text injected into prompt, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "[W1]") || !strings.Contains(sysPrompt, "URL：") {
		t.Fatalf("expected web search snippet and URL injected into prompt, got: %s", sysPrompt)
	}

	// Idempotent replay with same key & question does NOT invoke model again!
	replay, err := agentSvc.AskWithOptions(ctx, sess.ID, "usr_1001", "如何防止 Goroutine 泄漏？", "idem_001")
	if err != nil || replay.ID != msg.ID || len(spy.calls) != 1 {
		t.Fatalf("expected idempotent replay without second model call, got replay=%+v calls=%d err=%v", replay, len(spy.calls), err)
	}
	// Same key with different question -> conflict
	if _, err := agentSvc.AskWithOptions(ctx, sess.ID, "usr_1001", "完全不同的问题", "idem_001"); !errors.Is(err, agentdomain.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}

	// Verify course access revocation blocks BOTH new retrieval AND historical ListMessages!
	buyerAllowed = false
	history, err := agentSvc.ListMessages(ctx, sess.ID, "usr_1001")
	if err != nil {
		t.Fatalf("list messages failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if !history[1].Redacted || !strings.Contains(history[1].Content, "受保护内容已隐藏") || len(history[1].Citations) != 0 {
		t.Fatalf("expected historical message redacted after losing course access, got %+v", history[1])
	}

	// Cross-user isolation
	if _, err := agentSvc.ListMessages(ctx, sess.ID, "usr_other"); !errors.Is(err, agentdomain.ErrUnauthorizedAccess) {
		t.Fatalf("expected ErrUnauthorizedAccess, got %v", err)
	}
}

func TestAgent_SearchSuccessModelFailureStillBillsSearchCost(t *testing.T) {
	ctx := context.Background()

	usageSvc := usageapp.NewService()
	knowledgeSvc := knowledgeapp.NewService(func(ctx context.Context, userID, courseID string) (bool, error) {
		return true, nil
	}, usageSvc)

	baseModel := providerapi.NewClientWithOptions(providerapi.ClientOptions{AllowLocalSimulation: true})
	spy := &spyModelClient{
		inner:    baseModel,
		failWith: errors.New("upstream 504 gateway timeout"),
	}
	searchEng := providerapi.NewSearchEngineWithOptions(providerapi.SearchEngineOptions{AllowLocalSimulation: true})
	agentSvc := agentapp.NewService(usageSvc, knowledgeSvc, spy, searchEng)

	sess, _ := agentSvc.CreateSession(ctx, "customer_support", "usr_bill", "", "测试部分失败计费")
	_, err := agentSvc.Ask(ctx, sess.ID, "usr_bill", "正常搜索但模型超时")
	if err == nil {
		t.Fatalf("expected model generation failure")
	}

	summary := usageSvc.GetSummary(ctx, "usr_bill", true)
	// Failed answer MUST NOT increment success count
	if summary.QuestionsCount != 0 {
		t.Fatalf("expected 0 success questions on failed model call, got %d", summary.QuestionsCount)
	}
	// Completed search cost (1 cent) MUST be billed and NOT erased!
	if summary.TotalCostCents < 1 || summary.PlatformCostCents < 1 {
		t.Fatalf("expected incurred search cost >= 1 cent to be billed, got user=%d platform=%d", summary.TotalCostCents, summary.PlatformCostCents)
	}
	// Unknown LLM cost MUST be retained for reconciliation!
	if summary.PlatformUnknownCents <= 0 || summary.PendingReconciliations != 1 {
		t.Fatalf("expected unknown LLM cost retained for reconciliation, got summary=%+v", summary)
	}
	// Concurrency slot must be released
	if summary.ActiveRequests != 0 {
		t.Fatalf("expected active requests released, got %d", summary.ActiveRequests)
	}
}

func TestAgent_MandatorySearchFailureAbortsBeforeModelCall(t *testing.T) {
	ctx := context.Background()

	usageSvc := usageapp.NewService()
	knowledgeSvc := knowledgeapp.NewService(nil, usageSvc)
	baseModel := providerapi.NewClientWithOptions(providerapi.ClientOptions{AllowLocalSimulation: true})
	spy := &spyModelClient{inner: baseModel}
	searchEng := providerapi.NewSearchEngineWithOptions(providerapi.SearchEngineOptions{AllowLocalSimulation: true})
	agentSvc := agentapp.NewService(usageSvc, knowledgeSvc, spy, searchEng)

	sess, _ := agentSvc.CreateSession(ctx, "customer_support", "usr_fail", "", "测试搜索失败")
	_, err := agentSvc.Ask(ctx, sess.ID, "usr_fail", "auth_fail_mock")
	if err == nil {
		t.Fatalf("expected error from failed mandatory search")
	}
	if spy.callCounter != 0 {
		t.Fatalf("expected 0 model calls when mandatory search fails, got %d", spy.callCounter)
	}
}

func TestAgent_ZeroSearchResultsExplicitMessageAndDisableLifecycle(t *testing.T) {
	ctx := context.Background()

	usageSvc := usageapp.NewService()
	knowledgeSvc := knowledgeapp.NewService(nil, usageSvc)
	modelCli := providerapi.NewClientWithOptions(providerapi.ClientOptions{AllowLocalSimulation: true})
	searchEng := providerapi.NewSearchEngineWithOptions(providerapi.SearchEngineOptions{AllowLocalSimulation: true})
	agentSvc := agentapp.NewService(usageSvc, knowledgeSvc, modelCli, searchEng)

	sess, _ := agentSvc.CreateSession(ctx, "customer_support", "usr_zero", "", "零结果测试")
	msg, err := agentSvc.Ask(ctx, sess.ID, "usr_zero", "empty_mock")
	if err != nil {
		t.Fatalf("expected zero-results search to succeed cleanly, got %v", err)
	}
	if msg.SearchStatus != "zero_results" || !strings.Contains(msg.Content, "未找到相关网页来源") {
		t.Fatalf("expected explicit zero-results indicator, got status=%s content=%s", msg.SearchStatus, msg.Content)
	}

	// Non-admin ListAgents hides internal systemPrompt & draft/disabled agents
	draftAg, err := agentSvc.CreateAgent(ctx, "草稿助手", "draft_bot", "秘密提示词", "deepseek", "deepseek-chat", nil)
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	// Duplicate slug rejected
	if _, err := agentSvc.CreateAgent(ctx, "重复", "draft_bot", "p", "deepseek", "deepseek-chat", nil); !errors.Is(err, agentdomain.ErrSlugAlreadyExists) {
		t.Fatalf("expected ErrSlugAlreadyExists, got %v", err)
	}
	// UpdateAgent increments draftVersion
	newName := "草稿助手 v2"
	updated, err := agentSvc.UpdateAgent(ctx, draftAg.ID, agentdomain.UpdateAgentInput{ExpectedVersion: 1, Name: &newName})
	if err != nil || updated.DraftVersion != 2 {
		t.Fatalf("expected draftVersion=2, got %+v err=%v", updated, err)
	}
	_, _ = agentSvc.PublishAgent(ctx, draftAg.ID)
	_, _ = agentSvc.DisableAgent(ctx, draftAg.ID, "maintenance")

	userList := agentSvc.ListAgents(ctx, false)
	for _, a := range userList {
		if a.Slug == "draft_bot" {
			t.Fatalf("disabled agent must not appear in non-admin list")
		}
		if a.RoleInstruction != "" {
			t.Fatalf("internal systemPrompt must be stripped for non-admin users")
		}
	}
	if _, err := agentSvc.CreateSession(ctx, "draft_bot", "usr_zero", "", "test"); !errors.Is(err, agentdomain.ErrAgentDisabled) {
		t.Fatalf("expected ErrAgentDisabled on disabled agent, got %v", err)
	}
}

func TestAgent_MultiTurnContextAndRunCancellation(t *testing.T) {
	ctx := context.Background()

	usageSvc := usageapp.NewService()
	knowledgeSvc := knowledgeapp.NewService(func(ctx context.Context, userID, courseID string) (bool, error) {
		return true, nil
	}, usageSvc)

	baseModel := providerapi.NewClientWithOptions(providerapi.ClientOptions{AllowLocalSimulation: true})
	spy := &spyModelClient{inner: baseModel}
	searchEng := providerapi.NewSearchEngineWithOptions(providerapi.SearchEngineOptions{AllowLocalSimulation: true})
	agentSvc := agentapp.NewService(usageSvc, knowledgeSvc, spy, searchEng)

	sess, _ := agentSvc.CreateSession(ctx, "customer_support", "usr_mt", "", "多轮对话")

	// Turn 1
	msg1, err := agentSvc.Ask(ctx, sess.ID, "usr_mt", "什么是学屿学习社区？")
	if err != nil {
		t.Fatalf("turn 1 failed: %v", err)
	}
	if msg1.RunID == "" {
		t.Fatalf("expected non-empty RunID on message")
	}

	// Turn 2: verify previous turn is included in model Messages
	if _, err := agentSvc.Ask(ctx, sess.ID, "usr_mt", "它的退款窗口是几天？"); err != nil {
		t.Fatalf("turn 2 failed: %v", err)
	}
	if len(spy.calls) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(spy.calls))
	}
	turn2Msgs := spy.calls[1].Messages
	if len(turn2Msgs) != 4 { // system + turn1 user + turn1 assistant + turn2 user
		t.Fatalf("expected 4 messages in multi-turn prompt, got %d", len(turn2Msgs))
	}
	if turn2Msgs[1].Role != "user" || !strings.Contains(turn2Msgs[1].Content, "学屿学习社区") {
		t.Fatalf("expected turn 1 user message in history, got %+v", turn2Msgs[1])
	}

	// Cancel already completed run is idempotent
	cancelledCompleted, err := agentSvc.CancelRun(ctx, sess.ID, msg1.RunID, "usr_mt")
	if err != nil || cancelledCompleted.State != agentdomain.RunStateCompleted {
		t.Fatalf("expected idempotent completed state on cancelling finished run, got %+v err=%v", cancelledCompleted, err)
	}

	// Cancel in-flight slow run via context cancellation releases concurrency slot and bills search cost
	spy.mu.Lock()
	spy.delay = 2 * time.Second
	spy.mu.Unlock()

	cancelCtx, cancel := context.WithCancel(ctx)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err = agentSvc.Ask(cancelCtx, sess.ID, "usr_mt", "慢速推理测试")
	if !errors.Is(err, agentdomain.ErrRunCancelled) {
		t.Fatalf("expected ErrRunCancelled, got %v", err)
	}
	summary := usageSvc.GetSummary(ctx, "usr_mt", true)
	if summary.ActiveRequests != 0 {
		t.Fatalf("expected concurrency slot released after cancel, got %d", summary.ActiveRequests)
	}
}
