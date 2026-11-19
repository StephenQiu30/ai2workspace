package integration_test

import (
	"context"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/usage"
)

func TestUsageGovernance_ReserveAndLimits(t *testing.T) {
	ctx := context.Background()
	svc := usage.NewService()

	// 1. Successful reservation
	res, err := svc.Reserve(ctx, "usr_1001", usage.PurposeAgentChat, 10)
	if err != nil {
		t.Fatalf("expected successful reservation, got: %v", err)
	}
	if res.State != usage.StateReserved {
		t.Fatalf("expected state reserved, got: %s", res.State)
	}

	// 2. Concurrency limit for same user (max 1 active request)
	_, err = svc.Reserve(ctx, "usr_1001", usage.PurposeAgentChat, 10)
	if err != usage.ErrUserConcurrencyLimit {
		t.Fatalf("expected ErrUserConcurrencyLimit, got: %v", err)
	}

	// 3. Settle previous reservation
	steps := []usage.SettlementStep{
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
	if settled.State != usage.StateSettled || settled.ActualCents != 5 {
		t.Fatalf("unexpected settled state: %+v", settled)
	}

	// 4. Concurrency is freed after settlement
	res2, err := svc.Reserve(ctx, "usr_1001", usage.PurposeAgentChat, 10)
	if err != nil {
		t.Fatalf("expected successful reservation after settle, got: %v", err)
	}

	// 5. Release reservation
	if err := svc.Release(ctx, res2.ID); err != nil {
		t.Fatalf("expected successful release, got: %v", err)
	}
}

func TestUsageGovernance_DailyBudgetExceeded(t *testing.T) {
	ctx := context.Background()
	svc := usage.NewService()

	// Attempt reservation exceeding 200 cents
	_, err := svc.Reserve(ctx, "usr_expensive", usage.PurposeAgentChat, 250)
	if err != usage.ErrUserBudgetExceeded {
		t.Fatalf("expected ErrUserBudgetExceeded, got: %v", err)
	}
}
