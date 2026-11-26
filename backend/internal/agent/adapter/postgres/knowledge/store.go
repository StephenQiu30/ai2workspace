package knowledge

import (
	"context"

	knowledgeapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/knowledge"
	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists knowledge bases, sources, and index state in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore builds storage on the application pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Load reads knowledge bases, sources, and index state from PostgreSQL.
func (st *Store) Load(ctx context.Context) knowledgeapp.StoredState {
	var state knowledgeapp.StoredState

	baseRows, err := st.pool.Query(ctx, `
		SELECT id, name, description, visibility, owner_id, version, created_at, updated_at
		FROM knowledge_bases`)
	if err == nil {
		defer baseRows.Close()
		for baseRows.Next() {
			var kb knowledgedomain.KnowledgeBase
			if scanErr := baseRows.Scan(
				&kb.ID, &kb.Name, &kb.Description, &kb.Visibility, &kb.OwnerID,
				&kb.Version, &kb.CreatedAt, &kb.UpdatedAt,
			); scanErr == nil {
				state.Bases = append(state.Bases, &kb)
			}
		}
	}

	srcRows, err := st.pool.Query(ctx, `
		SELECT id, knowledge_base_id, kind, source_ref, title, content,
		       source_version, ready_version, published_version, state, revoked_at, created_at, updated_at
		FROM knowledge_sources`)
	if err == nil {
		defer srcRows.Close()
		for srcRows.Next() {
			var src knowledgedomain.KnowledgeSource
			if scanErr := srcRows.Scan(
				&src.ID, &src.KnowledgeBaseID, &src.Kind, &src.SourceRef, &src.Title, &src.Content,
				&src.SourceVersion, &src.ReadyVersion, &src.PublishedVersion, &src.State,
				&src.RevokedAt, &src.CreatedAt, &src.UpdatedAt,
			); scanErr == nil {
				state.Sources = append(state.Sources, &src)
			}
		}
	}

	chunkRows, err := st.pool.Query(ctx, `
		SELECT id, source_id, knowledge_base_id, course_id, title, chunk_index, content, version, is_live
		FROM knowledge_chunks
		ORDER BY source_id, version, chunk_index`)
	if err == nil {
		defer chunkRows.Close()
		for chunkRows.Next() {
			var ch knowledgedomain.KnowledgeChunk
			var isLive bool
			if scanErr := chunkRows.Scan(
				&ch.ID, &ch.SourceID, &ch.KnowledgeBaseID, &ch.CourseID, &ch.Title,
				&ch.ChunkIndex, &ch.Content, &ch.Version, &isLive,
			); scanErr == nil {
				if isLive {
					state.LiveChunks = append(state.LiveChunks, &ch)
				} else {
					state.StagedChunks = append(state.StagedChunks, &ch)
				}
			}
		}
	}

	jobRows, err := st.pool.Query(ctx, `
		SELECT id, source_id, source_version, stage, state, retryable, error_code, cost_cents, reservation_id, created_at, updated_at
		FROM knowledge_index_jobs`)
	if err == nil {
		defer jobRows.Close()
		for jobRows.Next() {
			var job knowledgedomain.IndexJob
			if scanErr := jobRows.Scan(
				&job.ID, &job.SourceID, &job.SourceVersion, &job.Stage, &job.State,
				&job.Retryable, &job.ErrorCode, &job.CostCents, &job.ReservationID,
				&job.CreatedAt, &job.UpdatedAt,
			); scanErr == nil {
				state.Jobs = append(state.Jobs, &job)
			}
		}
	}
	return state
}

func (st *Store) UpsertBase(ctx context.Context, kb *knowledgedomain.KnowledgeBase) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO knowledge_bases (id, name, description, visibility, owner_id, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			visibility = EXCLUDED.visibility,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at`,
		kb.ID, kb.Name, kb.Description, kb.Visibility, kb.OwnerID, kb.Version, kb.CreatedAt, kb.UpdatedAt,
	)
	return err
}

func (st *Store) UpsertSource(ctx context.Context, src *knowledgedomain.KnowledgeSource) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO knowledge_sources (
			id, knowledge_base_id, kind, source_ref, title, content,
			source_version, ready_version, published_version, state, revoked_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			content = EXCLUDED.content,
			source_version = EXCLUDED.source_version,
			ready_version = EXCLUDED.ready_version,
			published_version = EXCLUDED.published_version,
			state = EXCLUDED.state,
			revoked_at = EXCLUDED.revoked_at,
			updated_at = EXCLUDED.updated_at`,
		src.ID, src.KnowledgeBaseID, src.Kind, src.SourceRef, src.Title, src.Content,
		src.SourceVersion, src.ReadyVersion, src.PublishedVersion, src.State,
		src.RevokedAt, src.CreatedAt, src.UpdatedAt,
	)
	return err
}

func (st *Store) UpsertJob(ctx context.Context, job *knowledgedomain.IndexJob) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO knowledge_index_jobs (
			id, source_id, source_version, stage, state, retryable, error_code, cost_cents, reservation_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			stage = EXCLUDED.stage,
			state = EXCLUDED.state,
			retryable = EXCLUDED.retryable,
			error_code = EXCLUDED.error_code,
			cost_cents = EXCLUDED.cost_cents,
			reservation_id = EXCLUDED.reservation_id,
			updated_at = EXCLUDED.updated_at`,
		job.ID, job.SourceID, job.SourceVersion, job.Stage, job.State,
		job.Retryable, job.ErrorCode, job.CostCents, job.ReservationID,
		job.CreatedAt, job.UpdatedAt,
	)
	return err
}

func (st *Store) SaveStagedChunksAndJob(ctx context.Context, src *knowledgedomain.KnowledgeSource, version int, chunks []*knowledgedomain.KnowledgeChunk, job *knowledgedomain.IndexJob) error {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1 AND version = $2 AND is_live = FALSE`, src.ID, version); err != nil {
		return err
	}
	for _, ch := range chunks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_chunks (
				id, source_id, knowledge_base_id, course_id, title, chunk_index, content, version, is_live
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,FALSE)
			ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content, title = EXCLUDED.title`,
			ch.ID, ch.SourceID, ch.KnowledgeBaseID, ch.CourseID, ch.Title, ch.ChunkIndex, ch.Content, ch.Version,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE knowledge_sources SET ready_version = $2, state = $3, updated_at = $4 WHERE id = $1`,
		src.ID, src.ReadyVersion, src.State, src.UpdatedAt,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_index_jobs (
			id, source_id, source_version, stage, state, retryable, error_code, cost_cents, reservation_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			stage = EXCLUDED.stage, state = EXCLUDED.state, updated_at = EXCLUDED.updated_at`,
		job.ID, job.SourceID, job.SourceVersion, job.Stage, job.State,
		job.Retryable, job.ErrorCode, job.CostCents, job.ReservationID,
		job.CreatedAt, job.UpdatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (st *Store) PublishSourceVersion(ctx context.Context, src *knowledgedomain.KnowledgeSource, version int) error {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1 AND version <> $2`, src.ID, version); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE knowledge_chunks SET is_live = TRUE WHERE source_id = $1 AND version = $2`, src.ID, version); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE knowledge_sources
		SET published_version = $2, state = $3, updated_at = $4
		WHERE id = $1`,
		src.ID, src.PublishedVersion, src.State, src.UpdatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (st *Store) RevokeSourceAndPurgeChunks(ctx context.Context, src *knowledgedomain.KnowledgeSource) error {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1`, src.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE knowledge_sources
		SET state = $2, published_version = 0, revoked_at = $3, updated_at = $4
		WHERE id = $1`,
		src.ID, src.State, src.RevokedAt, src.UpdatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
