package usage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type usageStore struct {
	pool *pgxpool.Pool
}

func newUsageStore(pool *pgxpool.Pool) *usageStore {
	return &usageStore{pool: pool}
}

func (s *Service) bootstrapFromStore(ctx context.Context) {
	if s.store == nil {
		return
	}

	// Load or initialize budget config
	var cfg BudgetConfig
	err := s.store.pool.QueryRow(ctx, `
		SELECT version, user_daily_questions, user_per_minute, user_concurrency, platform_concurrency,
		       max_input_tokens, max_output_tokens, max_single_reserve_cents, user_daily_cents,
		       platform_daily_cents, platform_monthly_cents, bg_daily_cents, alert_threshold_pct,
		       paused, pause_reason, updated_at
		FROM ai_budget_configs WHERE id = 1`).Scan(
		&cfg.Version, &cfg.UserDailyQuestions, &cfg.UserPerMinute, &cfg.UserConcurrency, &cfg.PlatformConcurrency,
		&cfg.MaxInputTokens, &cfg.MaxOutputTokens, &cfg.MaxSingleReserveCents, &cfg.UserDailyCents,
		&cfg.PlatformDailyCents, &cfg.PlatformMonthlyCents, &cfg.BackgroundDailyCents, &cfg.AlertThresholdPct,
		&cfg.Paused, &cfg.PauseReason, &cfg.UpdatedAt,
	)
	if err == nil {
		s.config = cfg
	} else {
		_ = s.store.saveConfig(ctx, s.config)
	}

	// Rebuild confirmed/unknown totals from reservations & reconciliations
	rows, err := s.store.pool.Query(ctx, `
		SELECT id, user_id, purpose, period_day, period_month, reserved_cents, actual_cents,
		       unknown_cents, state, reconcile_due_at, created_at, updated_at
		FROM ai_budget_reservations`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r Reservation
			if scanErr := rows.Scan(
				&r.ID, &r.UserID, &r.Purpose, &r.PeriodDay, &r.PeriodMonth,
				&r.ReservedCents, &r.ActualCents, &r.UnknownCents, &r.State,
				&r.ReconcileDueAt, &r.CreatedAt, &r.UpdatedAt,
			); scanErr == nil {
				s.reservations[r.ID] = &r
				userKey := fmt.Sprintf("%s:%s", r.UserID, r.PeriodDay)
				if r.ActualCents > 0 {
					s.platformDailyCost[r.PeriodDay] += r.ActualCents
					s.platformMonthlyCost[r.PeriodMonth] += r.ActualCents
					if r.Purpose == PurposeAgentChat {
						s.userDailyCost[userKey] += r.ActualCents
					} else if r.Purpose == PurposeIndexJob {
						s.bgDailyCost[r.PeriodDay] += r.ActualCents
					}
				}
				if r.Purpose == PurposeAgentChat && (r.State == StateSettled || r.State == StateOverrun) && r.ActualCents > 0 {
					s.userDailyCount[userKey]++
				}
			}
		}
	}

	recRows, err := s.store.pool.Query(ctx, `
		SELECT id, reservation_id, user_id, purpose, period_day, period_month, unknown_cents,
		       resolved_cents, state, reason, resolution_note, due_at, resolved_at, created_at, updated_at
		FROM ai_budget_reconciliations`)
	if err == nil {
		defer recRows.Close()
		for recRows.Next() {
			var item ReconciliationItem
			if scanErr := recRows.Scan(
				&item.ID, &item.ReservationID, &item.UserID, &item.Purpose, &item.PeriodDay, &item.PeriodMonth,
				&item.UnknownCents, &item.ResolvedCents, &item.State, &item.Reason, &item.ResolutionNote,
				&item.DueAt, &item.ResolvedAt, &item.CreatedAt, &item.UpdatedAt,
			); scanErr == nil {
				s.reconciliations[item.ID] = &item
				if item.State != ReconciliationResolved {
					s.platformUnknownCents += item.UnknownCents
					if item.Purpose == PurposeAgentChat {
						s.userUnknownCents[item.UserID] += item.UnknownCents
					} else if item.Purpose == PurposeIndexJob {
						s.bgUnknownCents += item.UnknownCents
					}
				}
			}
		}
	}
}

func (st *usageStore) saveConfig(ctx context.Context, cfg BudgetConfig) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO ai_budget_configs (
			id, version, user_daily_questions, user_per_minute, user_concurrency, platform_concurrency,
			max_input_tokens, max_output_tokens, max_single_reserve_cents, user_daily_cents,
			platform_daily_cents, platform_monthly_cents, bg_daily_cents, alert_threshold_pct,
			paused, pause_reason, updated_at
		) VALUES (1,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (id) DO UPDATE SET
			version = EXCLUDED.version,
			user_daily_questions = EXCLUDED.user_daily_questions,
			user_per_minute = EXCLUDED.user_per_minute,
			user_concurrency = EXCLUDED.user_concurrency,
			platform_concurrency = EXCLUDED.platform_concurrency,
			max_input_tokens = EXCLUDED.max_input_tokens,
			max_output_tokens = EXCLUDED.max_output_tokens,
			max_single_reserve_cents = EXCLUDED.max_single_reserve_cents,
			user_daily_cents = EXCLUDED.user_daily_cents,
			platform_daily_cents = EXCLUDED.platform_daily_cents,
			platform_monthly_cents = EXCLUDED.platform_monthly_cents,
			bg_daily_cents = EXCLUDED.bg_daily_cents,
			alert_threshold_pct = EXCLUDED.alert_threshold_pct,
			paused = EXCLUDED.paused,
			pause_reason = EXCLUDED.pause_reason,
			updated_at = EXCLUDED.updated_at`,
		cfg.Version, cfg.UserDailyQuestions, cfg.UserPerMinute, cfg.UserConcurrency, cfg.PlatformConcurrency,
		cfg.MaxInputTokens, cfg.MaxOutputTokens, cfg.MaxSingleReserveCents, cfg.UserDailyCents,
		cfg.PlatformDailyCents, cfg.PlatformMonthlyCents, cfg.BackgroundDailyCents, cfg.AlertThresholdPct,
		cfg.Paused, cfg.PauseReason, cfg.UpdatedAt,
	)
	return err
}

func (st *usageStore) insertReservation(ctx context.Context, r *Reservation) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO ai_budget_reservations (
			id, user_id, purpose, period_day, period_month, reserved_cents,
			actual_cents, unknown_cents, state, reconcile_due_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		r.ID, r.UserID, r.Purpose, r.PeriodDay, r.PeriodMonth, r.ReservedCents,
		r.ActualCents, r.UnknownCents, r.State, r.ReconcileDueAt, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (st *usageStore) updateReservation(ctx context.Context, r *Reservation) error {
	_, err := st.pool.Exec(ctx, `
		UPDATE ai_budget_reservations
		SET actual_cents = $2, unknown_cents = $3, state = $4, reconcile_due_at = $5, updated_at = $6
		WHERE id = $1`,
		r.ID, r.ActualCents, r.UnknownCents, r.State, r.ReconcileDueAt, r.UpdatedAt,
	)
	return err
}

func (st *usageStore) persistSettlement(ctx context.Context, r *Reservation, steps []SettlementStep, rec *ReconciliationItem) error {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE ai_budget_reservations
		SET actual_cents = $2, unknown_cents = $3, state = $4, reconcile_due_at = $5, updated_at = $6
		WHERE id = $1`,
		r.ID, r.ActualCents, r.UnknownCents, r.State, r.ReconcileDueAt, r.UpdatedAt,
	); err != nil {
		return err
	}

	for idx, step := range steps {
		ledgerID := fmt.Sprintf("ldg_%s_%d", r.ID, idx+1)
		if _, err := tx.Exec(ctx, `
			INSERT INTO ai_budget_ledgers (
				id, user_id, reservation_id, step, provider, model_id,
				prompt_tokens, completion_tokens, cost_cents, price_version, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (id) DO NOTHING`,
			ledgerID, r.UserID, r.ID, step.Step, step.Provider, step.ModelID,
			step.PromptTokens, step.CompletionTokens, step.CostCents, step.PriceVersion, r.UpdatedAt,
		); err != nil {
			return err
		}
	}

	if rec != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ai_budget_reconciliations (
				id, reservation_id, user_id, purpose, period_day, period_month,
				unknown_cents, resolved_cents, state, reason, resolution_note,
				due_at, resolved_at, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			rec.ID, rec.ReservationID, rec.UserID, rec.Purpose, rec.PeriodDay, rec.PeriodMonth,
			rec.UnknownCents, rec.ResolvedCents, rec.State, rec.Reason, rec.ResolutionNote,
			rec.DueAt, rec.ResolvedAt, rec.CreatedAt, rec.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (st *usageStore) updateReconciliation(ctx context.Context, item *ReconciliationItem) error {
	_, err := st.pool.Exec(ctx, `
		UPDATE ai_budget_reconciliations
		SET resolved_cents = $2, state = $3, resolution_note = $4, resolved_at = $5, updated_at = $6
		WHERE id = $1`,
		item.ID, item.ResolvedCents, item.State, item.ResolutionNote, item.ResolvedAt, item.UpdatedAt,
	)
	return err
}
