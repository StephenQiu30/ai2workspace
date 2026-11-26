package provider

import (
	"context"

	providerdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

type catalogStore struct {
	pool *pgxpool.Pool
}

func newCatalogStore(pool *pgxpool.Pool) *catalogStore {
	return &catalogStore{pool: pool}
}

func (c *Client) bootstrapFromStore(ctx context.Context) {
	if c.store == nil {
		return
	}
	// Seed defaults if missing, then load persisted rows.
	for _, m := range c.catalog {
		_, _ = c.store.pool.Exec(ctx, `
			INSERT INTO ai_model_catalog (
				provider_id, model_id, display_name, endpoint_ref, price_version, currency,
				prompt_price_per_k, completion_price_per_k, max_input_tokens, max_output_tokens,
				effective_from, expires_at, enabled, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			ON CONFLICT (provider_id, model_id) DO NOTHING`,
			m.ProviderID, m.ModelID, m.DisplayName, m.EndpointRef, m.PriceVersion, m.Currency,
			m.PromptPricePerK, m.CompletionPricePerK, m.MaxInputTokens, m.MaxOutputTokens,
			m.EffectiveFrom, m.ExpiresAt, m.Enabled, m.UpdatedAt,
		)
	}

	rows, err := c.store.pool.Query(ctx, `
		SELECT provider_id, model_id, display_name, endpoint_ref, price_version, currency,
		       prompt_price_per_k, completion_price_per_k, max_input_tokens, max_output_tokens,
		       effective_from, expires_at, enabled, updated_at
		FROM ai_model_catalog`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var m providerdomain.ModelInfo
		if err := rows.Scan(
			&m.ProviderID, &m.ModelID, &m.DisplayName, &m.EndpointRef, &m.PriceVersion, &m.Currency,
			&m.PromptPricePerK, &m.CompletionPricePerK, &m.MaxInputTokens, &m.MaxOutputTokens,
			&m.EffectiveFrom, &m.ExpiresAt, &m.Enabled, &m.UpdatedAt,
		); err == nil {
			c.catalog[catalogKey(m.ProviderID, m.ModelID)] = m
		}
	}
}

func (s *catalogStore) upsertModel(ctx context.Context, m providerdomain.ModelInfo) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_model_catalog (
			provider_id, model_id, display_name, endpoint_ref, price_version, currency,
			prompt_price_per_k, completion_price_per_k, max_input_tokens, max_output_tokens,
			effective_from, expires_at, enabled, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (provider_id, model_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			endpoint_ref = EXCLUDED.endpoint_ref,
			price_version = EXCLUDED.price_version,
			currency = EXCLUDED.currency,
			prompt_price_per_k = EXCLUDED.prompt_price_per_k,
			completion_price_per_k = EXCLUDED.completion_price_per_k,
			max_input_tokens = EXCLUDED.max_input_tokens,
			max_output_tokens = EXCLUDED.max_output_tokens,
			effective_from = EXCLUDED.effective_from,
			expires_at = EXCLUDED.expires_at,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at`,
		m.ProviderID, m.ModelID, m.DisplayName, m.EndpointRef, m.PriceVersion, m.Currency,
		m.PromptPricePerK, m.CompletionPricePerK, m.MaxInputTokens, m.MaxOutputTokens,
		m.EffectiveFrom, m.ExpiresAt, m.Enabled, m.UpdatedAt,
	)
	return err
}
