package integration_test

import (
	"context"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/provider"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/usage"
)

func TestAgent_AskQuestionFlow(t *testing.T) {
	ctx := context.Background()

	usageSvc := usage.NewService()
	knowledgeSvc := knowledge.NewService(nil)
	modelCli := provider.NewClient()
	searchEng := provider.NewSearchEngine()

	agentSvc := agent.NewService(usageSvc, knowledgeSvc, modelCli, searchEng)

	// Create session with customer_support
	sess, err := agentSvc.CreateSession(ctx, "customer_support", "usr_1001", "", "咨询退款政策")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Ask question
	msg, err := agentSvc.Ask(ctx, sess.ID, "usr_1001", "请问平台的退款规则是什么？")
	if err != nil {
		t.Fatalf("failed to ask question: %v", err)
	}

	if msg.Content == "" {
		t.Fatalf("expected non-empty answer content")
	}
	if len(msg.SearchQueries) == 0 {
		t.Fatalf("expected mandatory web search evidence")
	}

	// Verify history
	history, err := agentSvc.ListMessages(ctx, sess.ID, "usr_1001")
	if err != nil || len(history) != 2 {
		t.Fatalf("expected 2 messages in session history, got %d (err: %v)", len(history), err)
	}

	// Verify isolation: another user cannot view this session
	_, err = agentSvc.ListMessages(ctx, sess.ID, "usr_other")
	if err != agent.ErrUnauthorizedAccess {
		t.Fatalf("expected ErrUnauthorizedAccess for different user, got: %v", err)
	}
}

func TestAgent_MandatorySearchFailureReleasesHold(t *testing.T) {
	ctx := context.Background()

	usageSvc := usage.NewService()
	knowledgeSvc := knowledge.NewService(nil)
	modelCli := provider.NewClient()
	searchEng := provider.NewSearchEngine()

	agentSvc := agent.NewService(usageSvc, knowledgeSvc, modelCli, searchEng)

	sess, err := agentSvc.CreateSession(ctx, "customer_support", "usr_fail", "", "测试搜索失败")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Query containing auth_fail_mock will trigger non-retryable search error
	_, err = agentSvc.Ask(ctx, sess.ID, "usr_fail", "auth_fail_mock")
	if err == nil {
		t.Fatalf("expected error from failed mandatory search")
	}

	// Verify that user concurrency is immediately freed (can reserve again)
	res, err := usageSvc.Reserve(ctx, "usr_fail", usage.PurposeAgentChat, 10)
	if err != nil {
		t.Fatalf("expected reservation to succeed since hold was released, got: %v", err)
	}
	_ = usageSvc.Release(ctx, res.ID)
}
