package knowledge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
)

// UserAccessChecker evaluates whether userID currently has knowledge access to courseID.
// A non-nil error indicates the permission service is unavailable (fail closed).
type UserAccessChecker func(ctx context.Context, userID, courseID string) (bool, error)

// Service manages knowledge bases, versioned sources, budget-governed indexing, and authorized retrieval.
type Service struct {
	mu           sync.RWMutex
	seq          atomic.Int64
	bases        map[string]*knowledgedomain.KnowledgeBase
	sources      map[string]*knowledgedomain.KnowledgeSource
	liveChunks   map[string][]*knowledgedomain.KnowledgeChunk         // sourceID -> currently published version's chunks
	stagedChunks map[string]map[int][]*knowledgedomain.KnowledgeChunk // sourceID -> version -> indexed chunks awaiting publish
	jobs         map[string]*knowledgedomain.IndexJob

	accessChecker UserAccessChecker
	usageService  *usageapp.Service
	store         Store
}

// NewService creates an in-memory knowledge service.
// If checker is nil, course access checks FAIL CLOSED rather than defaulting to allow.
func NewService(checker UserAccessChecker, usageSvc *usageapp.Service) *Service {
	return NewServiceWithStore(checker, usageSvc, nil)
}

// NewServiceWithStore creates a knowledge service backed by optional PostgreSQL persistence.
func NewServiceWithStore(checker UserAccessChecker, usageSvc *usageapp.Service, store Store) *Service {
	if checker == nil {
		checker = func(ctx context.Context, userID, courseID string) (bool, error) {
			return false, knowledgedomain.ErrAccessCheckerUnavailable
		}
	}
	s := &Service{
		bases:         make(map[string]*knowledgedomain.KnowledgeBase),
		sources:       make(map[string]*knowledgedomain.KnowledgeSource),
		liveChunks:    make(map[string][]*knowledgedomain.KnowledgeChunk),
		stagedChunks:  make(map[string]map[int][]*knowledgedomain.KnowledgeChunk),
		jobs:          make(map[string]*knowledgedomain.IndexJob),
		accessChecker: checker,
		usageService:  usageSvc,
	}
	if store != nil {
		s.store = store
		s.restore(store.Load(context.Background()))
	}
	return s
}

// CreateKnowledgeBase creates a new knowledge base.
func (s *Service) CreateKnowledgeBase(ctx context.Context, ownerID, name, description string, vis knowledgedomain.Visibility) (*knowledgedomain.KnowledgeBase, error) {
	if vis != knowledgedomain.VisibilityPrivate && vis != knowledgedomain.VisibilityPublic {
		return nil, errors.New("visibility must be private or public")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	id := fmt.Sprintf("kb_%d_%d", now.UnixNano(), s.seq.Add(1))
	kb := &knowledgedomain.KnowledgeBase{
		ID:          id,
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Visibility:  vis,
		OwnerID:     ownerID,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if s.store != nil {
		if err := s.store.UpsertBase(ctx, kb); err != nil {
			return nil, err
		}
	}
	s.bases[id] = kb
	cp := *kb
	return &cp, nil
}

// UpdateKnowledgeBase updates name, description, or visibility with optimistic locking and ownership check.
func (s *Service) UpdateKnowledgeBase(
	ctx context.Context,
	kbID string,
	callerID string,
	isAdmin bool,
	expectedVersion int,
	name *string,
	description *string,
	vis *knowledgedomain.Visibility,
) (*knowledgedomain.KnowledgeBase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kb, exists := s.bases[kbID]
	if !exists {
		return nil, knowledgedomain.ErrKnowledgeBaseNotFound
	}
	if !isAdmin && kb.OwnerID != callerID {
		return nil, knowledgedomain.ErrUnauthorizedSourceScope
	}
	if expectedVersion > 0 && kb.Version != expectedVersion {
		return nil, knowledgedomain.ErrVersionMismatch
	}

	updated := *kb
	if name != nil && strings.TrimSpace(*name) != "" {
		updated.Name = strings.TrimSpace(*name)
	}
	if description != nil {
		updated.Description = strings.TrimSpace(*description)
	}
	if vis != nil {
		if *vis != knowledgedomain.VisibilityPrivate && *vis != knowledgedomain.VisibilityPublic {
			return nil, errors.New("visibility must be private or public")
		}
		updated.Visibility = *vis
	}
	updated.Version++
	updated.UpdatedAt = time.Now().UTC()

	if s.store != nil {
		if err := s.store.UpsertBase(ctx, &updated); err != nil {
			return nil, err
		}
	}
	*kb = updated
	cp := *kb
	return &cp, nil
}

// ListKnowledgeBases lists bases visible to the caller based on role and ownership.
func (s *Service) ListKnowledgeBases(ctx context.Context, callerID string, isAdmin bool) []*knowledgedomain.KnowledgeBase {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*knowledgedomain.KnowledgeBase, 0, len(s.bases))
	for _, kb := range s.bases {
		if isAdmin || kb.OwnerID == callerID || kb.Visibility == knowledgedomain.VisibilityPublic {
			cp := *kb
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// GetKnowledgeBase retrieves a base by ID.
func (s *Service) GetKnowledgeBase(ctx context.Context, id string) (*knowledgedomain.KnowledgeBase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kb, exists := s.bases[id]
	if !exists {
		return nil, knowledgedomain.ErrKnowledgeBaseNotFound
	}
	cp := *kb
	return &cp, nil
}

// CanManageKnowledgeBase checks whether callerID (with isAdmin flag) owns or administers kbID.
func (s *Service) CanManageKnowledgeBase(ctx context.Context, kbID, callerID string, isAdmin bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	kb, exists := s.bases[kbID]
	if !exists {
		return knowledgedomain.ErrKnowledgeBaseNotFound
	}
	if !isAdmin && kb.OwnerID != callerID {
		return knowledgedomain.ErrUnauthorizedSourceScope
	}
	return nil
}

// CanManageSource checks whether callerID (with isAdmin flag) owns or administers the source's knowledge base.
func (s *Service) CanManageSource(ctx context.Context, sourceID, callerID string, isAdmin bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, exists := s.sources[sourceID]
	if !exists {
		return knowledgedomain.ErrKnowledgeSourceNotFound
	}
	kb, kbExists := s.bases[src.KnowledgeBaseID]
	if !kbExists {
		return knowledgedomain.ErrKnowledgeBaseNotFound
	}
	if !isAdmin && kb.OwnerID != callerID {
		return knowledgedomain.ErrUnauthorizedSourceScope
	}
	return nil
}

// CanManageJob checks whether callerID (with isAdmin flag) can view the given index job.
func (s *Service) CanManageJob(ctx context.Context, jobID, callerID string, isAdmin bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[jobID]
	if !exists {
		return knowledgedomain.ErrIndexJobNotFound
	}
	src, srcExists := s.sources[job.SourceID]
	if !srcExists {
		return knowledgedomain.ErrKnowledgeSourceNotFound
	}
	kb, kbExists := s.bases[src.KnowledgeBaseID]
	if !kbExists {
		return knowledgedomain.ErrKnowledgeBaseNotFound
	}
	if !isAdmin && kb.OwnerID != callerID {
		return knowledgedomain.ErrUnauthorizedSourceScope
	}
	return nil
}

// RegisterKnowledgeSource registers a new content source in the base.
func (s *Service) RegisterKnowledgeSource(
	ctx context.Context,
	kbID string,
	callerID string,
	isAdmin bool,
	kind knowledgedomain.SourceKind,
	sourceRef string,
	title string,
	content string,
) (*knowledgedomain.KnowledgeSource, error) {
	if kind != knowledgedomain.KindCourseText && kind != knowledgedomain.KindManualDoc {
		return nil, knowledgedomain.ErrInvalidSourceKind
	}
	sourceRef = strings.TrimSpace(sourceRef)
	if kind == knowledgedomain.KindCourseText && sourceRef == "" {
		return nil, knowledgedomain.ErrInvalidSourceRef
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	kb, exists := s.bases[kbID]
	if !exists {
		return nil, knowledgedomain.ErrKnowledgeBaseNotFound
	}
	if !isAdmin && kb.OwnerID != callerID {
		return nil, knowledgedomain.ErrUnauthorizedSourceScope
	}

	now := time.Now().UTC()
	srcID := fmt.Sprintf("src_%d_%d", now.UnixNano(), s.seq.Add(1))
	src := &knowledgedomain.KnowledgeSource{
		ID:               srcID,
		KnowledgeBaseID:  kbID,
		Kind:             kind,
		SourceRef:        sourceRef,
		Title:            strings.TrimSpace(title),
		Content:          content,
		SourceVersion:    1,
		ReadyVersion:     0,
		PublishedVersion: 0,
		State:            knowledgedomain.StateDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if s.store != nil {
		if err := s.store.UpsertSource(ctx, src); err != nil {
			return nil, err
		}
	}
	s.sources[srcID] = src
	cp := *src
	return &cp, nil
}

// UpdateKnowledgeSource increments the source version with updated title/content while preserving
// any previously published version and its live chunks until the new version is indexed and published.
func (s *Service) UpdateKnowledgeSource(
	ctx context.Context,
	kbID string,
	sourceID string,
	callerID string,
	isAdmin bool,
	title string,
	content string,
) (*knowledgedomain.KnowledgeSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kb, kbExists := s.bases[kbID]
	if !kbExists {
		return nil, knowledgedomain.ErrKnowledgeBaseNotFound
	}
	if !isAdmin && kb.OwnerID != callerID {
		return nil, knowledgedomain.ErrUnauthorizedSourceScope
	}

	src, exists := s.sources[sourceID]
	if !exists || src.KnowledgeBaseID != kbID {
		return nil, knowledgedomain.ErrKnowledgeSourceNotFound
	}
	if src.State == knowledgedomain.StateRevoked {
		return nil, knowledgedomain.ErrSourceRevoked
	}

	updated := *src
	if strings.TrimSpace(title) != "" {
		updated.Title = strings.TrimSpace(title)
	}
	updated.Content = content
	updated.SourceVersion++
	// Keep StatePublished if a previous version is live; otherwise mark draft.
	if updated.PublishedVersion == 0 {
		updated.State = knowledgedomain.StateDraft
	}
	updated.UpdatedAt = time.Now().UTC()

	if s.store != nil {
		if err := s.store.UpsertSource(ctx, &updated); err != nil {
			return nil, err
		}
	}
	*src = updated
	cp := *src
	return &cp, nil
}

// StartIndexJob validates background AI budget, chunks and indexes the latest source version into staged chunks.
// Previously published chunks remain untouched until PublishKnowledgeSource is explicitly called.
func (s *Service) StartIndexJob(ctx context.Context, sourceID string) (*knowledgedomain.IndexJob, error) {
	s.mu.Lock()
	src, exists := s.sources[sourceID]
	if !exists {
		s.mu.Unlock()
		return nil, knowledgedomain.ErrKnowledgeSourceNotFound
	}
	if src.State == knowledgedomain.StateRevoked {
		s.mu.Unlock()
		return nil, knowledgedomain.ErrSourceRevoked
	}

	kb, kbExists := s.bases[src.KnowledgeBaseID]
	if !kbExists {
		s.mu.Unlock()
		return nil, knowledgedomain.ErrKnowledgeBaseNotFound
	}

	// Idempotent deduplication: if this exact sourceVersion already succeeded, return existing job without re-charging.
	for _, existing := range s.jobs {
		if existing.SourceID == sourceID && existing.SourceVersion == src.SourceVersion && existing.State == "success" {
			cp := *existing
			s.mu.Unlock()
			return &cp, nil
		}
	}

	targetVersion := src.SourceVersion
	title := src.Title
	content := src.Content
	kbID := src.KnowledgeBaseID
	kind := src.Kind
	sourceRef := src.SourceRef
	ownerID := kb.OwnerID
	s.mu.Unlock()

	// Estimate background indexing cost (at least 1 cent, capped at 50 cents per job).
	estCents := estimateIndexCostCents(content)
	var reservation *usagedomain.Reservation
	if s.usageService != nil {
		var err error
		reservation, err = s.usageService.Reserve(ctx, ownerID, usagedomain.PurposeIndexJob, estCents)
		if err != nil {
			now := time.Now().UTC()
			job := &knowledgedomain.IndexJob{
				ID:            fmt.Sprintf("job_%d_%d", now.UnixNano(), s.seq.Add(1)),
				SourceID:      sourceID,
				SourceVersion: targetVersion,
				Stage:         "budget_admission",
				State:         "paused_budget",
				Retryable:     true,
				ErrorCode:     "budget_paused",
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			s.mu.Lock()
			s.jobs[job.ID] = job
			if s.store != nil {
				_ = s.store.UpsertJob(ctx, job)
			}
			s.mu.Unlock()
			cp := *job
			return &cp, fmt.Errorf("%w: %v", knowledgedomain.ErrBudgetPaused, err)
		}
	}

	// Chunk content by paragraph (fallback to whole content if single block).
	chunks := buildChunks(sourceID, kbID, kind, sourceRef, title, content, targetVersion)

	// Settle background indexing cost.
	resID := ""
	actualCost := estCents
	if reservation != nil {
		resID = reservation.ID
		_, _ = s.usageService.Settle(ctx, reservation.ID, []usagedomain.SettlementStep{
			{
				Step:      "indexing",
				Provider:  "internal_indexer",
				ModelID:   "text-chunker-v1",
				CostCents: actualCost,
			},
		})
	}

	now := time.Now().UTC()
	job := &knowledgedomain.IndexJob{
		ID:            fmt.Sprintf("job_%d_%d", now.UnixNano(), s.seq.Add(1)),
		SourceID:      sourceID,
		SourceVersion: targetVersion,
		Stage:         "completed",
		State:         "success",
		Retryable:     false,
		CostCents:     actualCost,
		ReservationID: resID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check source wasn't revoked mid-job.
	curSrc, ok := s.sources[sourceID]
	if !ok || curSrc.State == knowledgedomain.StateRevoked {
		job.Stage = "aborted"
		job.State = "failed"
		job.ErrorCode = "source_revoked"
		s.jobs[job.ID] = job
		if s.store != nil {
			_ = s.store.UpsertJob(ctx, job)
		}
		cp := *job
		return &cp, knowledgedomain.ErrSourceRevoked
	}

	if s.stagedChunks[sourceID] == nil {
		s.stagedChunks[sourceID] = make(map[int][]*knowledgedomain.KnowledgeChunk)
	}
	s.stagedChunks[sourceID][targetVersion] = chunks
	if targetVersion > curSrc.ReadyVersion {
		curSrc.ReadyVersion = targetVersion
	}
	if curSrc.PublishedVersion == 0 {
		curSrc.State = knowledgedomain.StateIndexed
	}
	curSrc.UpdatedAt = now
	s.jobs[job.ID] = job

	if s.store != nil {
		if err := s.store.SaveStagedChunksAndJob(ctx, curSrc, targetVersion, chunks, job); err != nil {
			return nil, err
		}
	}

	cp := *job
	return &cp, nil
}

// GetIndexJob retrieves an index job by ID.
func (s *Service) GetIndexJob(ctx context.Context, jobID string) (*knowledgedomain.IndexJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, knowledgedomain.ErrIndexJobNotFound
	}
	cp := *job
	return &cp, nil
}

// ListIndexJobs lists index jobs for a given source.
func (s *Service) ListIndexJobs(ctx context.Context, sourceID string) []*knowledgedomain.IndexJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*knowledgedomain.IndexJob
	for _, j := range s.jobs {
		if j.SourceID == sourceID {
			cp := *j
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// PublishKnowledgeSource switches the live searchable chunks to expectedVersion once indexed.
// Revoked sources are strictly forbidden from being directly republished.
func (s *Service) PublishKnowledgeSource(ctx context.Context, sourceID string, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, exists := s.sources[sourceID]
	if !exists {
		return knowledgedomain.ErrKnowledgeSourceNotFound
	}
	if src.State == knowledgedomain.StateRevoked {
		return knowledgedomain.ErrSourceRevoked
	}
	if src.SourceVersion != expectedVersion {
		return knowledgedomain.ErrVersionMismatch
	}
	if src.ReadyVersion < expectedVersion {
		return knowledgedomain.ErrSourceNotReady
	}

	staged := s.stagedChunks[sourceID][expectedVersion]
	live := make([]*knowledgedomain.KnowledgeChunk, len(staged))
	copy(live, staged)

	now := time.Now().UTC()
	src.PublishedVersion = expectedVersion
	src.State = knowledgedomain.StatePublished
	src.UpdatedAt = now
	s.liveChunks[sourceID] = live

	// Clean up obsolete staged versions older than published version.
	for ver := range s.stagedChunks[sourceID] {
		if ver <= expectedVersion {
			delete(s.stagedChunks[sourceID], ver)
		}
	}

	if s.store != nil {
		if err := s.store.PublishSourceVersion(ctx, src, expectedVersion); err != nil {
			return err
		}
	}
	return nil
}

// RevokeKnowledgeSource immediately blocks the source and purges all derived chunks.
func (s *Service) RevokeKnowledgeSource(ctx context.Context, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, exists := s.sources[sourceID]
	if !exists {
		return knowledgedomain.ErrKnowledgeSourceNotFound
	}

	now := time.Now().UTC()
	src.State = knowledgedomain.StateRevoked
	src.PublishedVersion = 0
	src.RevokedAt = &now
	src.UpdatedAt = now

	// Immediately purge all live and staged derived chunks.
	delete(s.liveChunks, sourceID)
	delete(s.stagedChunks, sourceID)

	if s.store != nil {
		if err := s.store.RevokeSourceAndPurgeChunks(ctx, src); err != nil {
			return err
		}
	}
	return nil
}

// SyncCourseKnowledge deduplicates or version-bumps a course_text source by (knowledgeBaseID, courseID),
// runs budget-governed indexing, and publishes the new version on success.
func (s *Service) SyncCourseKnowledge(ctx context.Context, kbID, courseID, title, content string) (*knowledgedomain.KnowledgeSource, *knowledgedomain.IndexJob, error) {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return nil, nil, knowledgedomain.ErrInvalidSourceRef
	}
	title = strings.TrimSpace(title)

	s.mu.Lock()
	if _, exists := s.bases[kbID]; !exists {
		s.mu.Unlock()
		return nil, nil, knowledgedomain.ErrKnowledgeBaseNotFound
	}

	var existing *knowledgedomain.KnowledgeSource
	for _, src := range s.sources {
		if src.KnowledgeBaseID == kbID && src.Kind == knowledgedomain.KindCourseText && src.SourceRef == courseID && src.State != knowledgedomain.StateRevoked {
			existing = src
			break
		}
	}

	now := time.Now().UTC()
	var targetID string
	var targetVersion int
	if existing == nil {
		srcID := fmt.Sprintf("src_%d_%d", now.UnixNano(), s.seq.Add(1))
		created := &knowledgedomain.KnowledgeSource{
			ID:               srcID,
			KnowledgeBaseID:  kbID,
			Kind:             knowledgedomain.KindCourseText,
			SourceRef:        courseID,
			Title:            title,
			Content:          content,
			SourceVersion:    1,
			ReadyVersion:     0,
			PublishedVersion: 0,
			State:            knowledgedomain.StateDraft,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if s.store != nil {
			if err := s.store.UpsertSource(ctx, created); err != nil {
				s.mu.Unlock()
				return nil, nil, err
			}
		}
		s.sources[srcID] = created
		targetID = srcID
		targetVersion = 1
	} else {
		// Idempotent check: unchanged content & already published at latest version.
		if existing.Title == title && existing.Content == content &&
			existing.State == knowledgedomain.StatePublished && existing.PublishedVersion == existing.SourceVersion {
			cp := *existing
			s.mu.Unlock()
			return &cp, nil, nil
		}
		existing.Title = title
		existing.Content = content
		existing.SourceVersion++
		existing.UpdatedAt = now
		if s.store != nil {
			if err := s.store.UpsertSource(ctx, existing); err != nil {
				s.mu.Unlock()
				return nil, nil, err
			}
		}
		targetID = existing.ID
		targetVersion = existing.SourceVersion
	}
	s.mu.Unlock()

	job, err := s.StartIndexJob(ctx, targetID)
	if err != nil {
		s.mu.RLock()
		srcCopy := *s.sources[targetID]
		s.mu.RUnlock()
		return &srcCopy, job, err
	}

	if err := s.PublishKnowledgeSource(ctx, targetID, targetVersion); err != nil {
		s.mu.RLock()
		srcCopy := *s.sources[targetID]
		s.mu.RUnlock()
		return &srcCopy, job, err
	}

	s.mu.RLock()
	srcCopy := *s.sources[targetID]
	s.mu.RUnlock()
	return &srcCopy, job, nil
}

// ListSources lists sources for a knowledge base.
func (s *Service) ListSources(ctx context.Context, kbID string) []*knowledgedomain.KnowledgeSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*knowledgedomain.KnowledgeSource, 0)
	for _, src := range s.sources {
		if src.KnowledgeBaseID == kbID {
			cp := *src
			res = append(res, &cp)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res
}

type retrievalCandidate struct {
	sourceID      string
	sourceVersion int
	kind          knowledgedomain.SourceKind
	courseID      string
	chunks        []*knowledgedomain.KnowledgeChunk
}

// RetrieveChunks finds relevant chunks across allowed knowledge bases using tokenization + TF scoring,
// strictly enforcing live published status and active course access verification.
func (s *Service) RetrieveChunks(ctx context.Context, userID string, kbIDs []string, query string, maxResults int) ([]knowledgedomain.Citation, error) {
	if maxResults <= 0 {
		maxResults = 5
	}
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return []knowledgedomain.Citation{}, nil
	}
	queryLower := strings.ToLower(strings.TrimSpace(query))

	allowedKBs := make(map[string]bool, len(kbIDs))
	for _, id := range kbIDs {
		allowedKBs[id] = true
	}

	// Snapshot candidate published sources & live chunks under read lock.
	s.mu.RLock()
	var candidates []retrievalCandidate
	for srcID, chunks := range s.liveChunks {
		src, exists := s.sources[srcID]
		if !exists || src.State != knowledgedomain.StatePublished || src.PublishedVersion <= 0 {
			continue
		}
		if !allowedKBs[src.KnowledgeBaseID] {
			continue
		}
		var verChunks []*knowledgedomain.KnowledgeChunk
		for _, ch := range chunks {
			if ch.Version == src.PublishedVersion {
				cp := *ch
				verChunks = append(verChunks, &cp)
			}
		}
		if len(verChunks) == 0 {
			continue
		}
		courseID := ""
		if src.Kind == knowledgedomain.KindCourseText {
			courseID = src.SourceRef
		}
		candidates = append(candidates, retrievalCandidate{
			sourceID:      src.ID,
			sourceVersion: src.PublishedVersion,
			kind:          src.Kind,
			courseID:      courseID,
			chunks:        verChunks,
		})
	}
	s.mu.RUnlock()

	// Perform course access checks outside lock (caching per courseID within this single retrieval call).
	courseAccessCache := make(map[string]bool)
	var scored []knowledgedomain.Citation

	for _, cand := range candidates {
		if cand.kind == knowledgedomain.KindCourseText && cand.courseID != "" {
			allowed, checked := courseAccessCache[cand.courseID]
			if !checked {
				var err error
				allowed, err = s.accessChecker(ctx, userID, cand.courseID)
				if err != nil {
					// Fail closed: permission service outage must surface as retrieval failure, NOT empty evidence.
					return nil, fmt.Errorf("course access check failed for course %s: %w", cand.courseID, err)
				}
				courseAccessCache[cand.courseID] = allowed
			}
			if !allowed {
				continue
			}
		}

		for _, chunk := range cand.chunks {
			score := scoreChunkTF(queryLower, queryTokens, chunk.Title, chunk.Content)
			if score <= 0 {
				continue
			}
			scored = append(scored, knowledgedomain.Citation{
				ChunkID:       chunk.ID,
				SourceID:      cand.sourceID,
				SourceVersion: cand.sourceVersion,
				ChunkIndex:    chunk.ChunkIndex,
				CourseID:      cand.courseID,
				Title:         chunk.Title,
				Snippet:       truncateRunes(chunk.Content, 240),
				Content:       chunk.Content,
				Score:         math.Round(score*1000) / 1000,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		if scored[i].SourceID != scored[j].SourceID {
			return scored[i].SourceID < scored[j].SourceID
		}
		return scored[i].ChunkIndex < scored[j].ChunkIndex
	})

	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}
	return scored, nil
}

// VerifyCitations re-verifies every citation before answer delivery or on historical message reads.
// Any citation whose source was revoked/unpublished or whose course access is no longer valid is filtered out.
func (s *Service) VerifyCitations(ctx context.Context, userID string, citations []knowledgedomain.Citation) ([]knowledgedomain.Citation, int, error) {
	if len(citations) == 0 {
		return nil, 0, nil
	}

	type sourceCheck struct {
		published bool
		courseID  string
	}

	s.mu.RLock()
	sourceInfo := make(map[string]sourceCheck, len(citations))
	for _, cit := range citations {
		if _, ok := sourceInfo[cit.SourceID]; ok {
			continue
		}
		src, exists := s.sources[cit.SourceID]
		if !exists || src.State != knowledgedomain.StatePublished || src.PublishedVersion <= 0 {
			sourceInfo[cit.SourceID] = sourceCheck{published: false, courseID: cit.CourseID}
			continue
		}
		courseID := cit.CourseID
		if courseID == "" && src.Kind == knowledgedomain.KindCourseText {
			courseID = src.SourceRef
		}
		sourceInfo[cit.SourceID] = sourceCheck{published: true, courseID: courseID}
	}
	s.mu.RUnlock()

	courseCache := make(map[string]bool)
	valid := make([]knowledgedomain.Citation, 0, len(citations))
	redacted := 0

	for _, cit := range citations {
		info := sourceInfo[cit.SourceID]
		if !info.published {
			redacted++
			continue
		}
		if info.courseID != "" {
			allowed, checked := courseCache[info.courseID]
			if !checked {
				var err error
				allowed, err = s.accessChecker(ctx, userID, info.courseID)
				if err != nil {
					return nil, redacted, fmt.Errorf("course access re-verification failed: %w", err)
				}
				courseCache[info.courseID] = allowed
			}
			if !allowed {
				redacted++
				continue
			}
		}
		valid = append(valid, cit)
	}

	return valid, redacted, nil
}

func estimateIndexCostCents(content string) int {
	runes := len([]rune(content))
	cents := (runes + 999) / 1000
	if cents < 1 {
		cents = 1
	}
	if cents > 50 {
		cents = 50
	}
	return cents
}

func buildChunks(sourceID, kbID string, kind knowledgedomain.SourceKind, sourceRef, title, content string, version int) []*knowledgedomain.KnowledgeChunk {
	paragraphs := strings.Split(content, "\n\n")
	courseID := ""
	if kind == knowledgedomain.KindCourseText {
		courseID = sourceRef
	}
	var chunks []*knowledgedomain.KnowledgeChunk
	chunkNo := 0
	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		chunkNo++
		chunks = append(chunks, &knowledgedomain.KnowledgeChunk{
			ID:              fmt.Sprintf("chk_%s_v%d_%d", sourceID, version, chunkNo),
			SourceID:        sourceID,
			KnowledgeBaseID: kbID,
			CourseID:        courseID,
			Title:           title,
			ChunkIndex:      chunkNo,
			Content:         trimmed,
			Version:         version,
		})
	}
	if len(chunks) == 0 && strings.TrimSpace(content) != "" {
		chunks = append(chunks, &knowledgedomain.KnowledgeChunk{
			ID:              fmt.Sprintf("chk_%s_v%d_1", sourceID, version),
			SourceID:        sourceID,
			KnowledgeBaseID: kbID,
			CourseID:        courseID,
			Title:           title,
			ChunkIndex:      1,
			Content:         strings.TrimSpace(content),
			Version:         version,
		})
	}
	return chunks
}

// tokenize splits text into normalized ASCII word tokens and CJK unigram/bigram tokens.
func tokenize(text string) []string {
	var tokens []string
	var asciiBuf strings.Builder
	var cjkRunes []rune

	flushASCII := func() {
		if asciiBuf.Len() > 0 {
			w := strings.ToLower(asciiBuf.String())
			if len(w) > 0 {
				tokens = append(tokens, w)
			}
			asciiBuf.Reset()
		}
	}
	flushCJK := func() {
		if len(cjkRunes) == 0 {
			return
		}
		for i, r := range cjkRunes {
			tokens = append(tokens, string(r))
			if i+1 < len(cjkRunes) {
				tokens = append(tokens, string([]rune{r, cjkRunes[i+1]}))
			}
		}
		cjkRunes = cjkRunes[:0]
	}

	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			flushASCII()
			cjkRunes = append(cjkRunes, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			asciiBuf.WriteRune(unicode.ToLower(r))
		default:
			flushASCII()
			flushCJK()
		}
	}
	flushASCII()
	flushCJK()
	return tokens
}

// scoreChunkTF computes a TF-based relevance score over chunk content and title.
func scoreChunkTF(queryLower string, queryTokens []string, title, content string) float64 {
	contentTokens := tokenize(content)
	titleTokens := tokenize(title)
	if len(contentTokens) == 0 && len(titleTokens) == 0 {
		return 0
	}

	contentFreq := make(map[string]int, len(contentTokens))
	for _, t := range contentTokens {
		contentFreq[t]++
	}
	titleFreq := make(map[string]int, len(titleTokens))
	for _, t := range titleTokens {
		titleFreq[t]++
	}

	uniqueQuery := make(map[string]struct{}, len(queryTokens))
	for _, qt := range queryTokens {
		uniqueQuery[qt] = struct{}{}
	}

	matched := 0
	tfSum := 0.0
	docLen := float64(len(contentTokens) + 1)

	for qt := range uniqueQuery {
		cf := contentFreq[qt]
		tf := titleFreq[qt]
		if cf == 0 && tf == 0 {
			continue
		}
		matched++
		// Weight multi-rune tokens (ASCII words / CJK bigrams) higher than single CJK chars.
		weight := 1.0
		if len([]rune(qt)) >= 2 {
			weight = 2.0
		}
		termScore := (float64(cf) / docLen) * 10.0
		if tf > 0 {
			termScore += 0.5 * float64(tf)
		}
		tfSum += termScore * weight
	}

	if matched == 0 {
		return 0
	}

	coverage := float64(matched) / float64(len(uniqueQuery))
	score := tfSum * (0.5 + 0.5*coverage)

	// Exact phrase match bonus
	if queryLower != "" {
		if strings.Contains(strings.ToLower(content), queryLower) {
			score += 1.0
		}
		if strings.Contains(strings.ToLower(title), queryLower) {
			score += 0.5
		}
	}
	return score
}

func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
