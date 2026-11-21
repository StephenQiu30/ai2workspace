package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/usage"
)

func TestKnowledge_IngestionVersioningBudgetAndRevocation(t *testing.T) {
	ctx := context.Background()
	usageSvc := usage.NewService()

	// Access checker: only usr_buyer has access to course_123
	buyerHasAccess := true
	checker := func(ctx context.Context, userID, courseID string) (bool, error) {
		return buyerHasAccess && userID == "usr_buyer" && courseID == "course_123", nil
	}
	svc := knowledge.NewService(checker, usageSvc)

	// Create & update base
	kb, err := svc.CreateKnowledgeBase(ctx, "usr_instructor", "Go 高级微服务架构", "核心知识库", knowledge.VisibilityPrivate)
	if err != nil {
		t.Fatalf("failed to create base: %v", err)
	}
	newName := "Go 并发与微服务资料库"
	kb, err = svc.UpdateKnowledgeBase(ctx, kb.ID, "usr_instructor", false, 1, &newName, nil, nil)
	if err != nil || kb.Version != 2 || kb.Name != newName {
		t.Fatalf("failed to update kb: %+v, err=%v", kb, err)
	}

	// Register course text source
	v1Content := "Go 语言并发原语包括 Goroutine 与 Channel。\n\nGin 是一个轻量级高性能 HTTP Web 框架。"
	src, err := svc.RegisterKnowledgeSource(ctx, kb.ID, "usr_instructor", false, knowledge.KindCourseText, "course_123", "第1节: 并发与微服务", v1Content)
	if err != nil {
		t.Fatalf("failed to register source: %v", err)
	}

	// Start index job (consumes background sub-budget)
	job1, err := svc.StartIndexJob(ctx, src.ID)
	if err != nil || job1.State != "success" {
		t.Fatalf("index job failed: job=%+v err=%v", job1, err)
	}

	// Unindexed/unpublished content must NOT be retrievable
	preCits, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "并发", 5)
	if err != nil || len(preCits) != 0 {
		t.Fatalf("expected 0 citations before publish, got %d err=%v", len(preCits), err)
	}

	// Publish v1
	if err := svc.PublishKnowledgeSource(ctx, src.ID, 1); err != nil {
		t.Fatalf("failed to publish source: %v", err)
	}

	// 1. Authorized user retrieves chunks with full citation metadata
	citsBuyer, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "并发", 5)
	if err != nil {
		t.Fatalf("retrieve failed: %v", err)
	}
	if len(citsBuyer) == 0 {
		t.Fatalf("expected citations for authorized buyer, got 0")
	}
	if citsBuyer[0].SourceVersion != 1 || citsBuyer[0].ChunkIndex != 1 || citsBuyer[0].CourseID != "course_123" {
		t.Fatalf("incomplete citation metadata: %+v", citsBuyer[0])
	}

	// 2. Fix verification: short unrelated query (< 3 bytes) MUST NOT return everything!
	unrelated, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "zz", 5)
	if err != nil || len(unrelated) != 0 {
		t.Fatalf("expected 0 citations for unrelated short query 'zz', got %d", len(unrelated))
	}

	// 3. Unauthorized user cannot retrieve protected course chunks
	citsGuest, err := svc.RetrieveChunks(ctx, "usr_guest", []string{kb.ID}, "并发", 5)
	if err != nil || len(citsGuest) != 0 {
		t.Fatalf("expected 0 citations for unauthorized user, got %d err=%v", len(citsGuest), err)
	}

	// 4. Update source to v2: before v2 is indexed & published, v1 remains live!
	v2Content := "新版内容：Rust 与异步运行时 Tokio 调度机制。"
	srcV2, err := svc.UpdateKnowledgeSource(ctx, kb.ID, src.ID, "usr_instructor", false, "第1节: 新版异步架构", v2Content)
	if err != nil || srcV2.SourceVersion != 2 || srcV2.PublishedVersion != 1 {
		t.Fatalf("expected sourceVersion=2 publishedVersion=1, got %+v err=%v", srcV2, err)
	}
	stillV1, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "Goroutine", 5)
	if err != nil || len(stillV1) == 0 || stillV1[0].SourceVersion != 1 {
		t.Fatalf("expected v1 still retrievable while v2 is unindexed, got %+v err=%v", stillV1, err)
	}
	unpubV2, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "Tokio", 5)
	if err != nil || len(unpubV2) != 0 {
		t.Fatalf("expected unindexed v2 NOT retrievable, got %d", len(unpubV2))
	}

	// Index & publish v2 -> retrieval switches cleanly to v2
	job2, err := svc.StartIndexJob(ctx, src.ID)
	if err != nil || job2.State != "success" {
		t.Fatalf("v2 index job failed: %+v err=%v", job2, err)
	}
	if err := svc.PublishKnowledgeSource(ctx, src.ID, 2); err != nil {
		t.Fatalf("publish v2 failed: %v", err)
	}
	liveV2, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "Tokio", 5)
	if err != nil || len(liveV2) == 0 || liveV2[0].SourceVersion != 2 || !strings.Contains(liveV2[0].Snippet, "Tokio") {
		t.Fatalf("expected live v2 retrieval, got %+v err=%v", liveV2, err)
	}

	// 5. Revoke source: immediately purges chunks & forbids direct republish!
	if err := svc.RevokeKnowledgeSource(ctx, src.ID); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	afterRevoke, err := svc.RetrieveChunks(ctx, "usr_buyer", []string{kb.ID}, "Tokio", 5)
	if err != nil || len(afterRevoke) != 0 {
		t.Fatalf("expected 0 citations after revoke, got %d", len(afterRevoke))
	}
	if err := svc.PublishKnowledgeSource(ctx, src.ID, 2); !errors.Is(err, knowledge.ErrSourceRevoked) {
		t.Fatalf("expected ErrSourceRevoked when republishing revoked source, got %v", err)
	}
}

func TestKnowledge_IndexJobPausedWhenBudgetExhausted(t *testing.T) {
	ctx := context.Background()
	usageSvc := usage.NewService()
	_, _ = usageSvc.SetPaused(ctx, true, "budget frozen")

	svc := knowledge.NewService(func(ctx context.Context, userID, courseID string) (bool, error) {
		return true, nil
	}, usageSvc)

	kb, _ := svc.CreateKnowledgeBase(ctx, "usr_admin", "Test KB", "", knowledge.VisibilityPublic)
	src, _ := svc.RegisterKnowledgeSource(ctx, kb.ID, "usr_admin", true, knowledge.KindManualDoc, "", "Doc", "Body content")

	job, err := svc.StartIndexJob(ctx, src.ID)
	if !errors.Is(err, knowledge.ErrBudgetPaused) {
		t.Fatalf("expected ErrBudgetPaused, got %v", err)
	}
	if job == nil || job.State != "paused_budget" {
		t.Fatalf("expected paused_budget job status, got %+v", job)
	}

	fetched, err := svc.GetIndexJob(ctx, job.ID)
	if err != nil || fetched.State != "paused_budget" {
		t.Fatalf("expected GetIndexJob to return paused job, got %+v err=%v", fetched, err)
	}
}

func TestKnowledge_NilAccessCheckerFailsClosed(t *testing.T) {
	ctx := context.Background()
	usageSvc := usage.NewService()
	svc := knowledge.NewService(nil, usageSvc)

	kb, _ := svc.CreateKnowledgeBase(ctx, "usr_admin", "Course KB", "", knowledge.VisibilityPublic)
	src, _ := svc.RegisterKnowledgeSource(ctx, kb.ID, "usr_admin", true, knowledge.KindCourseText, "crs_paid", "Paid Lesson", "Secret paid course content")
	_, _ = svc.StartIndexJob(ctx, src.ID)
	_ = svc.PublishKnowledgeSource(ctx, src.ID, 1)

	_, err := svc.RetrieveChunks(ctx, "usr_any", []string{kb.ID}, "Secret", 5)
	if !errors.Is(err, knowledge.ErrAccessCheckerUnavailable) {
		t.Fatalf("expected nil access checker to fail closed with ErrAccessCheckerUnavailable, got %v", err)
	}
}

func TestKnowledge_SyncCourseDeduplicatesAndBumpsVersion(t *testing.T) {
	ctx := context.Background()
	usageSvc := usage.NewService()
	svc := knowledge.NewService(func(ctx context.Context, userID, courseID string) (bool, error) {
		return true, nil
	}, usageSvc)

	kb, _ := svc.CreateKnowledgeBase(ctx, "usr_instructor", "Course KB", "", knowledge.VisibilityPrivate)

	// First sync creates v1 and publishes
	s1, _, err := svc.SyncCourseKnowledge(ctx, kb.ID, "crs_99", "课程标题", "第一版内容：微服务治理")
	if err != nil || s1.SourceVersion != 1 || s1.PublishedVersion != 1 {
		t.Fatalf("first sync failed: %+v err=%v", s1, err)
	}

	// Identical sync is idempotent (no version bump)
	sSame, jobSame, err := svc.SyncCourseKnowledge(ctx, kb.ID, "crs_99", "课程标题", "第一版内容：微服务治理")
	if err != nil || sSame.ID != s1.ID || sSame.SourceVersion != 1 || jobSame != nil {
		t.Fatalf("identical sync should be idempotent, got src=%+v job=%+v err=%v", sSame, jobSame, err)
	}

	// Updated content bumps same source to v2
	s2, _, err := svc.SyncCourseKnowledge(ctx, kb.ID, "crs_99", "课程标题", "第二版内容：分布式事务")
	if err != nil || s2.ID != s1.ID || s2.SourceVersion != 2 || s2.PublishedVersion != 2 {
		t.Fatalf("updated sync should bump version on same source, got %+v err=%v", s2, err)
	}
	if len(svc.ListSources(ctx, kb.ID)) != 1 {
		t.Fatalf("expected deduplicated single source for course, got %d", len(svc.ListSources(ctx, kb.ID)))
	}
}
