package integration_test

import (
	"context"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
)

func TestKnowledge_IngestionAndAccess(t *testing.T) {
	ctx := context.Background()

	// Access checker: only usr_buyer has access to course_123
	checker := func(ctx context.Context, userID, courseID string) bool {
		return userID == "usr_buyer" && courseID == "course_123"
	}
	svc := knowledge.NewService(checker)

	// Create base
	kb, err := svc.CreateKnowledgeBase(ctx, "usr_admin", "Go 高级微服务架构", "核心知识库", knowledge.VisibilityPublic)
	if err != nil {
		t.Fatalf("failed to create base: %v", err)
	}

	// Register course text source
	content := "Go 语言并发原语包括 Goroutine 与 Channel。\n\nGin 是一个轻量级高性能 HTTP Web 框架。"
	src, err := svc.RegisterKnowledgeSource(ctx, kb.ID, "usr_admin", knowledge.KindCourseText, "course_123", "第1节: 并发与微服务", content)
	if err != nil {
		t.Fatalf("failed to register source: %v", err)
	}

	// Start index job
	job, err := svc.StartIndexJob(ctx, src.ID)
	if err != nil || job.State != "success" {
		t.Fatalf("index job failed: %v", err)
	}

	// Publish source
	if err := svc.PublishKnowledgeSource(ctx, src.ID, 1); err != nil {
		t.Fatalf("failed to publish source: %v", err)
	}

	// 1. Authorized user retrieves chunks
	citsBuyer, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "并发", 5)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}
	if len(citsBuyer) == 0 {
		t.Fatalf("expected citations for authorized buyer, got 0")
	}

	// 2. Unauthorized user cannot retrieve protected course chunks
	citsGuest, err := svc.RetrieveChunks(ctx, "usr_guest", []string{kb.ID}, "并发", 5)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}
	if len(citsGuest) != 0 {
		t.Fatalf("expected 0 citations for unauthorized user, got %d", len(citsGuest))
	}

	// 3. Revoke source: even authorized user cannot retrieve chunks anymore
	if err := svc.RevokeKnowledgeSource(ctx, src.ID); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	citsRevoked, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "并发", 5)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}
	if len(citsRevoked) != 0 {
		t.Fatalf("expected 0 citations after revoke, got %d", len(citsRevoked))
	}
}
