package knowledge

import (
	"context"

	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
)

// StoredState contains the records restored when the use case starts.
type StoredState struct {
	Bases        []*knowledgedomain.KnowledgeBase
	Sources      []*knowledgedomain.KnowledgeSource
	LiveChunks   []*knowledgedomain.KnowledgeChunk
	StagedChunks []*knowledgedomain.KnowledgeChunk
	Jobs         []*knowledgedomain.IndexJob
}

// Store persists knowledge bases, sources, and index state.
type Store interface {
	Load(ctx context.Context) StoredState
	UpsertBase(ctx context.Context, kb *knowledgedomain.KnowledgeBase) error
	UpsertSource(ctx context.Context, src *knowledgedomain.KnowledgeSource) error
	UpsertJob(ctx context.Context, job *knowledgedomain.IndexJob) error
	SaveStagedChunksAndJob(ctx context.Context, src *knowledgedomain.KnowledgeSource, version int, chunks []*knowledgedomain.KnowledgeChunk, job *knowledgedomain.IndexJob) error
	PublishSourceVersion(ctx context.Context, src *knowledgedomain.KnowledgeSource, version int) error
	RevokeSourceAndPurgeChunks(ctx context.Context, src *knowledgedomain.KnowledgeSource) error
}

func (s *Service) restore(state StoredState) {
	for _, base := range state.Bases {
		s.bases[base.ID] = base
	}
	for _, source := range state.Sources {
		s.sources[source.ID] = source
	}
	for _, chunk := range state.LiveChunks {
		s.liveChunks[chunk.SourceID] = append(s.liveChunks[chunk.SourceID], chunk)
	}
	for _, chunk := range state.StagedChunks {
		if s.stagedChunks[chunk.SourceID] == nil {
			s.stagedChunks[chunk.SourceID] = make(map[int][]*knowledgedomain.KnowledgeChunk)
		}
		s.stagedChunks[chunk.SourceID][chunk.Version] = append(s.stagedChunks[chunk.SourceID][chunk.Version], chunk)
	}
	for _, job := range state.Jobs {
		s.jobs[job.ID] = job
	}
}
