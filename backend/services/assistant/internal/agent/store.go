package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

type agentStore struct {
	pool *pgxpool.Pool
}

func newAgentStore(pool *pgxpool.Pool) *agentStore {
	return &agentStore{pool: pool}
}

func (s *Service) bootstrapFromStore(ctx context.Context) {
	if s.store == nil {
		return
	}

	// Ensure default agents exist in DB, then load all persisted agents.
	for _, ag := range s.agents {
		kbsJSON, _ := json.Marshal(ag.KnowledgeBaseIDs)
		_, _ = s.store.pool.Exec(ctx, `
			INSERT INTO agent_configurations (
				id, name, slug, role_instruction, provider_id, model_id, knowledge_base_ids,
				draft_version, published_version, state, disabled_reason, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (id) DO NOTHING`,
			ag.ID, ag.Name, ag.Slug, ag.RoleInstruction, ag.ProviderID, ag.ModelID, kbsJSON,
			ag.DraftVersion, ag.PublishedVersion, ag.State, ag.DisabledReason, ag.CreatedAt, ag.UpdatedAt,
		)
	}

	agRows, err := s.store.pool.Query(ctx, `
		SELECT id, name, slug, role_instruction, provider_id, model_id, knowledge_base_ids,
		       draft_version, published_version, state, disabled_reason, created_at, updated_at
		FROM agent_configurations`)
	if err == nil {
		defer agRows.Close()
		for agRows.Next() {
			var ag Configuration
			var kbsJSON []byte
			if scanErr := agRows.Scan(
				&ag.ID, &ag.Name, &ag.Slug, &ag.RoleInstruction, &ag.ProviderID, &ag.ModelID, &kbsJSON,
				&ag.DraftVersion, &ag.PublishedVersion, &ag.State, &ag.DisabledReason, &ag.CreatedAt, &ag.UpdatedAt,
			); scanErr == nil {
				_ = json.Unmarshal(kbsJSON, &ag.KnowledgeBaseIDs)
				if ag.KnowledgeBaseIDs == nil {
					ag.KnowledgeBaseIDs = []string{}
				}
				s.agents[ag.ID] = &ag
				s.agentsBySlug[ag.Slug] = &ag
			}
		}
	}

	sessRows, err := s.store.pool.Query(ctx, `
		SELECT id, agent_id, user_id, course_id, title, created_at, updated_at
		FROM agent_sessions`)
	if err == nil {
		defer sessRows.Close()
		for sessRows.Next() {
			var sess Session
			if scanErr := sessRows.Scan(
				&sess.ID, &sess.AgentID, &sess.UserID, &sess.CourseID, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt,
			); scanErr == nil {
				s.sessions[sess.ID] = &sess
			}
		}
	}

	msgRows, err := s.store.pool.Query(ctx, `
		SELECT id, session_id, run_id, sender, content, citations, search_queries, search_status, token_usage, created_at
		FROM agent_messages
		ORDER BY created_at ASC`)
	if err == nil {
		defer msgRows.Close()
		for msgRows.Next() {
			var m Message
			var citsJSON, sqJSON, usageJSON []byte
			if scanErr := msgRows.Scan(
				&m.ID, &m.SessionID, &m.RunID, &m.Sender, &m.Content,
				&citsJSON, &sqJSON, &m.SearchStatus, &usageJSON, &m.CreatedAt,
			); scanErr == nil {
				_ = json.Unmarshal(citsJSON, &m.Citations)
				_ = json.Unmarshal(sqJSON, &m.SearchQueries)
				_ = json.Unmarshal(usageJSON, &m.TokenUsage)
				if m.Citations == nil {
					m.Citations = []knowledge.Citation{}
				}
				if m.SearchQueries == nil {
					m.SearchQueries = []provider.SearchResultItem{}
				}
				s.messages[m.SessionID] = append(s.messages[m.SessionID], &m)
				s.messagesByID[m.ID] = &m
			}
		}
	}

	runRows, err := s.store.pool.Query(ctx, `
		SELECT id, session_id, agent_id, user_id, idempotency_key, question_hash,
		       state, phase, message_id, error_code, error_message, created_at, updated_at
		FROM agent_runs`)
	if err == nil {
		defer runRows.Close()
		for runRows.Next() {
			var r Run
			if scanErr := runRows.Scan(
				&r.ID, &r.SessionID, &r.AgentID, &r.UserID, &r.IdempotencyKey, &r.QuestionHash,
				&r.State, &r.Phase, &r.MessageID, &r.ErrorCode, &r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt,
			); scanErr == nil {
				s.runs[r.ID] = &r
				if r.IdempotencyKey != "" {
					s.runsByIdemKey[r.UserID+":"+r.IdempotencyKey] = &r
				}
			}
		}
	}
}

func (st *agentStore) upsertAgent(ctx context.Context, ag *Configuration) error {
	kbsJSON, _ := json.Marshal(ag.KnowledgeBaseIDs)
	_, err := st.pool.Exec(ctx, `
		INSERT INTO agent_configurations (
			id, name, slug, role_instruction, provider_id, model_id, knowledge_base_ids,
			draft_version, published_version, state, disabled_reason, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			role_instruction = EXCLUDED.role_instruction,
			provider_id = EXCLUDED.provider_id,
			model_id = EXCLUDED.model_id,
			knowledge_base_ids = EXCLUDED.knowledge_base_ids,
			draft_version = EXCLUDED.draft_version,
			published_version = EXCLUDED.published_version,
			state = EXCLUDED.state,
			disabled_reason = EXCLUDED.disabled_reason,
			updated_at = EXCLUDED.updated_at`,
		ag.ID, ag.Name, ag.Slug, ag.RoleInstruction, ag.ProviderID, ag.ModelID, kbsJSON,
		ag.DraftVersion, ag.PublishedVersion, ag.State, ag.DisabledReason, ag.CreatedAt, ag.UpdatedAt,
	)
	return err
}

func (st *agentStore) upsertSession(ctx context.Context, sess *Session) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO agent_sessions (id, agent_id, user_id, course_id, title, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			updated_at = EXCLUDED.updated_at`,
		sess.ID, sess.AgentID, sess.UserID, sess.CourseID, sess.Title, sess.CreatedAt, sess.UpdatedAt,
	)
	return err
}

func (st *agentStore) upsertRun(ctx context.Context, r *Run) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO agent_runs (
			id, session_id, agent_id, user_id, idempotency_key, question_hash,
			state, phase, message_id, error_code, error_message, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			state = EXCLUDED.state,
			phase = EXCLUDED.phase,
			message_id = EXCLUDED.message_id,
			error_code = EXCLUDED.error_code,
			error_message = EXCLUDED.error_message,
			updated_at = EXCLUDED.updated_at`,
		r.ID, r.SessionID, r.AgentID, r.UserID, r.IdempotencyKey, r.QuestionHash,
		r.State, r.Phase, r.MessageID, r.ErrorCode, r.ErrorMessage, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (st *agentStore) persistAskResult(
	ctx context.Context,
	sessionID string,
	run *Run,
	userMsg *Message,
	assistantMsg *Message,
	searchRes *provider.SearchExecutionResult,
) error {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if run != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE agent_runs SET state = $2, phase = $3, message_id = $4, updated_at = $5 WHERE id = $1`,
			run.ID, run.State, run.Phase, run.MessageID, run.UpdatedAt,
		); err != nil {
			return err
		}
	}

	for _, m := range []*Message{userMsg, assistantMsg} {
		citsJSON, _ := json.Marshal(m.Citations)
		sqJSON, _ := json.Marshal(m.SearchQueries)
		usageJSON, _ := json.Marshal(m.TokenUsage)
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_messages (
				id, session_id, run_id, sender, content, citations, search_queries, search_status, token_usage, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			m.ID, m.SessionID, m.RunID, m.Sender, m.Content, citsJSON, sqJSON, m.SearchStatus, usageJSON, m.CreatedAt,
		); err != nil {
			return err
		}
	}

	if searchRes != nil {
		logID := fmt.Sprintf("slog_%d", time.Now().UnixNano())
		runID := ""
		if run != nil {
			runID = run.ID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_search_logs (
				id, run_id, message_id, query, status, attempts, result_count, duration_ms, cost_cents, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			logID, runID, assistantMsg.ID, searchRes.Query, searchRes.Status,
			searchRes.Attempts, len(searchRes.Results), searchRes.DurationMs, searchRes.CostCents, assistantMsg.CreatedAt,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET updated_at = $2 WHERE id = $1`, sessionID, assistantMsg.CreatedAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
