package usage

import (
	"context"
	"fmt"

	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists usage limits and settlement state in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore builds storage on the application pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Load reads usage limits and settlement state from PostgreSQL.
func (st *Store) Load(ctx context.Context, defaults usagedomain.BudgetConfig) usageapp.StoredState {
	var state usageapp.StoredState

	// Load or initialize budget config
	var cfg usagedomain.BudgetConfig
	err := st.pool.QueryRow(ctx, `
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
		state.Config = cfg
	} else {
		state.Config = defaults
		_ = st.SaveConfig(ctx, defaults)
	}

	// Rebuild confirmed/unknown totals from reservations & reconciliations
	rows, err := st.pool.Query(ctx, `
		SELECT id, user_id, purpose, period_day, period_month, reserved_cents, actual_cents,
		       unknown_cents, state, reconcile_due_at, created_at, updated_at
		FROM ai_budget_reservations`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r usagedomain.Reservation
			if scanErr := rows.Scan(
				&r.ID, &r.UserID, &r.Purpose, &r.PeriodDay, &r.PeriodMonth,
				&r.ReservedCents, &r.ActualCents, &r.UnknownCents, &r.State,
				&r.ReconcileDueAt, &r.CreatedAt, &r.UpdatedAt,
			); scanErr == nil {
				state.Reservations = append(state.Reservations, &r)
			}
		}
	}

	recRows, err := st.pool.Query(ctx, `
		SELECT id, reservation_id, user_id, purpose, period_day, period_month, unknown_cents,
		       resolved_cents, state, reason, resolution_note, due_at, resolved_at, created_at, updated_at
		FROM ai_budget_reconciliations`)
	if err == nil {
		defer recRows.Close()
		for recRows.Next() {
			var item usagedomain.ReconciliationItem
			if scanErr := recRows.Scan(
				&item.ID, &item.ReservationID, &item.UserID, &item.Purpose, &item.PeriodDay, &item.PeriodMonth,
				&item.UnknownCents, &item.ResolvedCents, &item.State, &item.Reason, &item.ResolutionNote,
				&item.DueAt, &item.ResolvedAt, &item.CreatedAt, &item.UpdatedAt,
			); scanErr == nil {
				state.Reconciliations = append(state.Reconciliations, &item)
			}
		}
	}
	return state
}

func (st *Store) SaveConfig(ctx context.Context, cfg usagedomain.BudgetConfig) error {
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

func (st *Store) InsertReservation(ctx context.Context, r *usagedomain.Reservation) error {
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

func (st *Store) UpdateReservation(ctx context.Context, r *usagedomain.Reservation) error {
	_, err := st.pool.Exec(ctx, `
		UPDATE ai_budget_reservations
		SET actual_cents = $2, unknown_cents = $3, state = $4, reconcile_due_at = $5, updated_at = $6
		WHERE id = $1`,
		r.ID, r.ActualCents, r.UnknownCents, r.State, r.ReconcileDueAt, r.UpdatedAt,
	)
	return err
}

func (st *Store) PersistSettlement(ctx context.Context, r *usagedomain.Reservation, steps []usagedomain.SettlementStep, rec *usagedomain.ReconciliationItem) error {
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

func (st *Store) UpdateReconciliation(ctx context.Context, item *usagedomain.ReconciliationItem) error {
	_, err := st.pool.Exec(ctx, `
		UPDATE ai_budget_reconciliations
		SET resolved_cents = $2, state = $3, resolution_note = $4, resolved_at = $5, updated_at = $6
		WHERE id = $1`,
		item.ID, item.ResolvedCents, item.State, item.ResolutionNote, item.ResolvedAt, item.UpdatedAt,
	)
	return err
}
