package agent_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
)

func TestUsageGovernance_ReserveAndLimits(t *testing.T) {
	ctx := context.Background()
	svc := usageapp.NewService()

	// 1. Successful reservation
	res, err := svc.Reserve(ctx, "usr_1001", usagedomain.PurposeAgentChat, 20)
	if err != nil {
		t.Fatalf("expected successful reservation, got: %v", err)
	}
	if res.State != usagedomain.StateReserved {
		t.Fatalf("expected state reserved, got: %s", res.State)
	}

	// 2. Concurrency limit for same user (max 1 active request)
	_, err = svc.Reserve(ctx, "usr_1001", usagedomain.PurposeAgentChat, 20)
	if !errors.Is(err, usagedomain.ErrUserConcurrencyLimit) {
		t.Fatalf("expected ErrUserConcurrencyLimit, got: %v", err)
	}

	// 3. Settle previous reservation
	steps := []usagedomain.SettlementStep{
		{
			Step:             "llm",
			Provider:         "openai",
			ModelID:          "gpt-4o-mini",
			PromptTokens:     100,
			CompletionTokens: 50,
			CostCents:        5,
		},
	}
	settled, err := svc.Settle(ctx, res.ID, steps)
	if err != nil {
		t.Fatalf("expected successful settlement, got: %v", err)
	}
	if settled.State != usagedomain.StateSettled || settled.ActualCents != 5 {
		t.Fatalf("unexpected settled state: %+v", settled)
	}

	// 4. Concurrency is freed after settlement
	res2, err := svc.Reserve(ctx, "usr_1001", usagedomain.PurposeAgentChat, 20)
	if err != nil {
		t.Fatalf("expected successful reservation after settle, got: %v", err)
	}

	// 5. Release reservation
	if err := svc.Release(ctx, res2.ID); err != nil {
		t.Fatalf("expected successful release, got: %v", err)
	}
}

func TestUsageGovernance_ConcurrentReserveNeverOverdraftsPlatformBudget(t *testing.T) {
	ctx := context.Background()
	svc := usageapp.NewService()

	// Tighten platform daily limit to 60 cents (only two 30-cent reservations can fit concurrently)
	dailyLimit := 60
	bgLimit := 20
	platConc := 20
	if _, err := svc.UpdateConfig(ctx, usagedomain.BudgetConfigUpdate{
		PlatformDailyCents:   &dailyLimit,
		BackgroundDailyCents: &bgLimit,
		PlatformConcurrency:  &platConc,
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	const workers = 20
	var admitted atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			uid := fmt.Sprintf("usr_conc_%d", idx)
			if _, err := svc.Reserve(ctx, uid, usagedomain.PurposeAgentChat, 30); err == nil {
				admitted.Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if admitted.Load() != 2 {
		t.Fatalf("expected exactly 2 concurrent admissions within 60-cent budget, got %d", admitted.Load())
	}

	summary := svc.GetSummary(ctx, "admin", true)
	if summary.PlatformDailyReservedCents != 60 {
		t.Fatalf("expected platformDailyReserved=60, got %d", summary.PlatformDailyReservedCents)
	}
	if !summary.DailyAlertTriggered {
		t.Fatalf("expected 80%% daily alert triggered at 60/60 cents")
	}
}

func TestUsageGovernance_DynamicEstimateAndHardLimits(t *testing.T) {
	svc := usageapp.NewService()

	// Input tokens > 8000 rejected
	if _, err := svc.EstimateAnswerReserveCents(8001, 2000, 1, 2, 2, 1); !errors.Is(err, usagedomain.ErrInputTokensExceeded) {
		t.Fatalf("expected ErrInputTokensExceeded, got %v", err)
	}
	// Output tokens > 2000 rejected
	if _, err := svc.EstimateAnswerReserveCents(4000, 2001, 1, 2, 2, 1); !errors.Is(err, usagedomain.ErrOutputTokensExceeded) {
		t.Fatalf("expected ErrOutputTokensExceeded, got %v", err)
	}
	// Single request reserve > 50 cents rejected (e.g. expensive model 5/15 -> (40+30+2)*1.2 = 87 > 50)
	if _, err := svc.EstimateAnswerReserveCents(8000, 2000, 5, 15, 2, 1); !errors.Is(err, usagedomain.ErrSingleRequestBudgetExceeded) {
		t.Fatalf("expected ErrSingleRequestBudgetExceeded, got %v", err)
	}
	// DeepSeek (1/2) at 8000 input + 2000 output + 2*1 search = 8+4+2=14; *1.2 = 17 cents
	est, err := svc.EstimateAnswerReserveCents(8000, 2000, 1, 2, 2, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est != 17 {
		t.Fatalf("expected 17 cents reserve with 20%% buffer, got %d", est)
	}
}

func TestUsageGovernance_BudgetRejectionDoesNotConsumeRateLimit(t *testing.T) {
	ctx := context.Background()
	svc := usageapp.NewService()

	// Set user daily budget to 40 cents
	userDaily := 40
	if _, err := svc.UpdateConfig(ctx, usagedomain.BudgetConfigUpdate{UserDailyCents: &userDaily}); err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	// Attempt 4 reservations of 45 cents (each rejected by user budget)
	for i := 0; i < 4; i++ {
		_, err := svc.Reserve(ctx, "usr_rate", usagedomain.PurposeAgentChat, 45)
		if !errors.Is(err, usagedomain.ErrUserBudgetExceeded) {
			t.Fatalf("attempt %d: expected ErrUserBudgetExceeded, got %v", i, err)
		}
	}

	// Now a valid 20-cent reservation must still succeed (rate limit was NOT consumed by budget rejections!)
	res, err := svc.Reserve(ctx, "usr_rate", usagedomain.PurposeAgentChat, 20)
	if err != nil {
		t.Fatalf("expected valid reservation to succeed without rate limit error, got %v", err)
	}
	_ = svc.Release(ctx, res.ID)
}

func TestUsageGovernance_BackgroundSubQuotaAndPlatformTotal(t *testing.T) {
	ctx := context.Background()
	svc := usageapp.NewService()

	// Lower platform daily limit to 100 and background sub-quota to 60
	platDaily := 100
	bgDaily := 60
	if _, err := svc.UpdateConfig(ctx, usagedomain.BudgetConfigUpdate{
		PlatformDailyCents:   &platDaily,
		BackgroundDailyCents: &bgDaily,
	}); err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	// Reserve & settle 40 cents for indexing
	r1, err := svc.Reserve(ctx, "usr_instructor", usagedomain.PurposeIndexJob, 40)
	if err != nil {
		t.Fatalf("expected bg reserve 1 to succeed, got %v", err)
	}
	_, _ = svc.Settle(ctx, r1.ID, []usagedomain.SettlementStep{{Step: "indexing", CostCents: 40}})

	// Another 30 cents exceeds bg sub-quota (40+30=70 > 60)
	if _, err := svc.Reserve(ctx, "usr_instructor", usagedomain.PurposeIndexJob, 30); !errors.Is(err, usagedomain.ErrBackgroundBudgetExceeded) {
		t.Fatalf("expected ErrBackgroundBudgetExceeded, got %v", err)
	}

	// User chat spends 50 cents -> platform total now 40+50=90
	r2, err := svc.Reserve(ctx, "usr_chat", usagedomain.PurposeAgentChat, 50)
	if err != nil {
		t.Fatalf("expected chat reserve to succeed, got %v", err)
	}
	_, _ = svc.Settle(ctx, r2.ID, []usagedomain.SettlementStep{{Step: "llm", CostCents: 50}})

	// Remaining bg sub-quota is 20 (60-40), but platform daily remaining is only 10 (100-90)!
	// Background task of 15 cents MUST be blocked by platform daily limit (proving bg sub-quota is inside platform total).
	if _, err := svc.Reserve(ctx, "usr_instructor", usagedomain.PurposeIndexJob, 15); !errors.Is(err, usagedomain.ErrPlatformBudget) {
		t.Fatalf("expected ErrPlatformBudget because bg sub-quota shares platform daily total, got %v", err)
	}
}

func TestUsageGovernance_UnknownCostPersistsAcrossDayAndMonthRollover(t *testing.T) {
	ctx := context.Background()
	svc := usageapp.NewService()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	current := time.Date(2026, 11, 30, 23, 50, 0, 0, loc)
	svc.SetNowFunc(func() time.Time { return current })

	// Set tight limits: platform daily 50, monthly 100, user daily 50
	platDaily := 50
	platMonthly := 100
	bgDaily := 20
	userDaily := 50
	if _, err := svc.UpdateConfig(ctx, usagedomain.BudgetConfigUpdate{
		PlatformDailyCents:   &platDaily,
		PlatformMonthlyCents: &platMonthly,
		BackgroundDailyCents: &bgDaily,
		UserDailyCents:       &userDaily,
	}); err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	// Reserve 40 cents on 2026-11-30, search succeeds (2 cents), LLM times out -> 38 cents unknown!
	res, err := svc.Reserve(ctx, "usr_unknown", usagedomain.PurposeAgentChat, 40)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	settled, err := svc.SettleFailed(ctx, res.ID, []usagedomain.SettlementStep{
		{Step: "search", Provider: "web_search", CostCents: 2},
	}, 38, "upstream timeout")
	if err != nil {
		t.Fatalf("settle failed error: %v", err)
	}
	if settled.State != usagedomain.StatePendingReconciliation || settled.UnknownCents != 38 {
		t.Fatalf("expected pending_reconciliation with 38 unknown cents, got %+v", settled)
	}

	// Advance clock across BOTH natural day AND month boundary into 2026-12-01!
	current = time.Date(2026, 12, 1, 10, 0, 0, 0, loc)

	// On 2026-12-01, unknown 38 cents MUST STILL consume platform & user available budget (50 - 38 = 12 left)!
	if _, err := svc.Reserve(ctx, "usr_unknown", usagedomain.PurposeAgentChat, 15); !errors.Is(err, usagedomain.ErrUserBudgetExceeded) {
		t.Fatalf("expected unknown hold to persist across day/month rollover and block 15 cents, got %v", err)
	}
	if _, err := svc.Reserve(ctx, "usr_other", usagedomain.PurposeAgentChat, 15); !errors.Is(err, usagedomain.ErrPlatformBudget) {
		t.Fatalf("expected platform unknown hold to persist across day/month rollover and block 15 cents, got %v", err)
	}

	// Advance past 24 hours -> reconciliation item auto-transitions to manual_review
	current = time.Date(2026, 12, 2, 1, 0, 0, 0, loc)
	recs := svc.ListReconciliations(ctx)
	if len(recs) != 1 || recs[0].State != usagedomain.ReconciliationManualReview {
		t.Fatalf("expected 1 reconciliation item in manual_review after 24h, got %+v", recs)
	}

	// Admin resolves reconciliation at actual 10 cents -> credited to ORIGINAL 2026-11-30 / 2026-11 period,
	// releasing the 38-cent risk hold so 2026-12-02 has full 50 cents available!
	resolved, err := svc.ResolveReconciliation(ctx, recs[0].ID, 10, "confirmed from provider invoice")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.State != usagedomain.ReconciliationResolved || resolved.ResolvedCents != 10 {
		t.Fatalf("unexpected resolved item: %+v", resolved)
	}

	// Now 45 cents on 2026-12-02 succeeds!
	resNext, err := svc.Reserve(ctx, "usr_unknown", usagedomain.PurposeAgentChat, 45)
	if err != nil {
		t.Fatalf("expected reservation to succeed after reconciliation resolved, got %v", err)
	}
	_ = svc.Release(ctx, resNext.ID)
}

func TestUsageGovernance_AdminPauseSwitch(t *testing.T) {
	ctx := context.Background()
	svc := usageapp.NewService()

	if _, err := svc.SetPaused(ctx, true, "emergency maintenance"); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if _, err := svc.Reserve(ctx, "usr_1", usagedomain.PurposeAgentChat, 10); !errors.Is(err, usagedomain.ErrAIPaused) {
		t.Fatalf("expected ErrAIPaused, got %v", err)
	}

	if _, err := svc.SetPaused(ctx, false, ""); err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	res, err := svc.Reserve(ctx, "usr_1", usagedomain.PurposeAgentChat, 10)
	if err != nil {
		t.Fatalf("expected reservation to succeed after resume, got %v", err)
	}
	_ = svc.Release(ctx, res.ID)
}
