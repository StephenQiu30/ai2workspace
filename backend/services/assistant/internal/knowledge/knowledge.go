// Package knowledge provides knowledge base management, source ingestion, chunking, and permission-aware retrieval.
package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrKnowledgeBaseNotFound   = errors.New("knowledge base not found")
	ErrKnowledgeSourceNotFound = errors.New("knowledge source not found")
	ErrUnauthorizedSourceScope = errors.New("user is not authorized to register or manage this knowledge source")
	ErrSourceNotReady          = errors.New("knowledge source index is not ready for publication")
	ErrIndexJobNotFound        = errors.New("index job not found")
	ErrVersionMismatch         = errors.New("optimistic lock version mismatch")
)

// Visibility defines who can query the knowledge base.
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

// KnowledgeBase represents a grouped repository of documentation.
type KnowledgeBase struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Visibility  Visibility `json:"visibility"`
	OwnerID     string     `json:"ownerId"`
	Version     int        `json:"version"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// SourceKind defines what backs this source.
type SourceKind string

const (
	KindCourseText SourceKind = "course_text"
	KindManualDoc  SourceKind = "manual_doc"
)

// SourceState defines ingestion lifecycle.
type SourceState string

const (
	StateDraft     SourceState = "draft"
	StateIndexing  SourceState = "indexing"
	StateIndexed   SourceState = "indexed"
	StatePublished SourceState = "published"
	StateRevoked   SourceState = "revoked"
)

// KnowledgeSource represents a registered piece of content.
type KnowledgeSource struct {
	ID              string      `json:"id"`
	KnowledgeBaseID string      `json:"knowledgeBaseId"`
	Kind            SourceKind  `json:"kind"`
	SourceRef       string      `json:"sourceRef"` // e.g. course_id
	Title           string      `json:"title"`
	Content         string      `json:"content,omitempty"`
	SourceVersion   int         `json:"sourceVersion"`
	ReadyVersion    int         `json:"readyVersion"`
	State           SourceState `json:"state"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

// KnowledgeChunk represents an indexed slice of text.
type KnowledgeChunk struct {
	ID              string `json:"id"`
	SourceID        string `json:"sourceId"`
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	CourseID        string `json:"courseId,omitempty"`
	Title           string `json:"title"`
	ChunkIndex      int    `json:"chunkIndex"`
	Content         string `json:"content"`
	Version         int    `json:"version"`
}

// IndexJob tracks background parsing and chunking.
type IndexJob struct {
	ID            string    `json:"id"`
	SourceID      string    `json:"sourceId"`
	SourceVersion int       `json:"sourceVersion"`
	Stage         string    `json:"stage"` // "parsing", "chunking", "indexing", "completed", "failed"
	State         string    `json:"state"` // "running", "success", "failed"
	ErrorCode     string    `json:"errorCode,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Citation is a safe reference returned with agent answers.
type Citation struct {
	ChunkID  string  `json:"chunkId"`
	SourceID string  `json:"sourceId"`
	Title    string  `json:"title"`
	Snippet  string  `json:"snippet"`
	Score    float64 `json:"score"`
}

// UserAccessChecker evaluates if the user can read a given course content.
type UserAccessChecker func(ctx context.Context, userID, courseID string) bool

// Service manages knowledge operations.
type Service struct {
	mu            sync.RWMutex
	bases         map[string]*KnowledgeBase
	sources       map[string]*KnowledgeSource
	chunks        map[string][]*KnowledgeChunk // sourceID -> chunks
	jobs          map[string]*IndexJob
	accessChecker UserAccessChecker
}

// NewService creates a new knowledge service.
func NewService(checker UserAccessChecker) *Service {
	if checker == nil {
		checker = func(ctx context.Context, userID, courseID string) bool {
			return true // default allow
		}
	}
	return &Service{
		bases:         make(map[string]*KnowledgeBase),
		sources:       make(map[string]*KnowledgeSource),
		chunks:        make(map[string][]*KnowledgeChunk),
		jobs:          make(map[string]*IndexJob),
		accessChecker: checker,
	}
}

// CreateKnowledgeBase creates a new base.
func (s *Service) CreateKnowledgeBase(ctx context.Context, ownerID, name, description string, vis Visibility) (*KnowledgeBase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("kb_%d", now.UnixNano())
	kb := &KnowledgeBase{
		ID:          id,
		Name:        name,
		Description: description,
		Visibility:  vis,
		OwnerID:     ownerID,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.bases[id] = kb
	return kb, nil
}

// ListKnowledgeBases lists bases visible to owner or public.
func (s *Service) ListKnowledgeBases(ctx context.Context, callerID string, isAdmin bool) []*KnowledgeBase {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*KnowledgeBase
	for _, kb := range s.bases {
		if isAdmin || kb.OwnerID == callerID || kb.Visibility == VisibilityPublic {
			result = append(result, kb)
		}
	}
	return result
}

// GetKnowledgeBase retrieves a base by ID.
func (s *Service) GetKnowledgeBase(ctx context.Context, id string) (*KnowledgeBase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kb, exists := s.bases[id]
	if !exists {
		return nil, ErrKnowledgeBaseNotFound
	}
	return kb, nil
}

// RegisterKnowledgeSource registers a new content source in the base.
func (s *Service) RegisterKnowledgeSource(ctx context.Context, kbID, ownerID string, kind SourceKind, sourceRef, title, content string) (*KnowledgeSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kb, exists := s.bases[kbID]
	if !exists {
		return nil, ErrKnowledgeBaseNotFound
	}
	if kb.OwnerID != ownerID && kb.Visibility != VisibilityPublic {
		return nil, ErrUnauthorizedSourceScope
	}

	now := time.Now()
	srcID := fmt.Sprintf("src_%d", now.UnixNano())
	src := &KnowledgeSource{
		ID:              srcID,
		KnowledgeBaseID: kbID,
		Kind:            kind,
		SourceRef:       sourceRef,
		Title:           title,
		Content:         content,
		SourceVersion:   1,
		ReadyVersion:    0,
		State:           StateDraft,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.sources[srcID] = src
	return src, nil
}

// StartIndexJob simulates background chunking and indexing of a source.
func (s *Service) StartIndexJob(ctx context.Context, sourceID string) (*IndexJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, exists := s.sources[sourceID]
	if !exists {
		return nil, ErrKnowledgeSourceNotFound
	}

	now := time.Now()
	jobID := fmt.Sprintf("job_%d", now.UnixNano())
	job := &IndexJob{
		ID:            jobID,
		SourceID:      sourceID,
		SourceVersion: src.SourceVersion,
		Stage:         "completed",
		State:         "success",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.jobs[jobID] = job

	// Simple paragraph chunker
	paragraphs := strings.Split(src.Content, "\n\n")
	var chunks []*KnowledgeChunk
	for idx, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		courseID := ""
		if src.Kind == KindCourseText {
			courseID = src.SourceRef
		}
		chunks = append(chunks, &KnowledgeChunk{
			ID:              fmt.Sprintf("chk_%s_%d", sourceID, idx+1),
			SourceID:        sourceID,
			KnowledgeBaseID: src.KnowledgeBaseID,
			CourseID:        courseID,
			Title:           src.Title,
			ChunkIndex:      idx + 1,
			Content:         trimmed,
			Version:         src.SourceVersion,
		})
	}
	s.chunks[sourceID] = chunks
	src.State = StateIndexed
	src.ReadyVersion = src.SourceVersion
	src.UpdatedAt = now

	return job, nil
}

// PublishKnowledgeSource publishes indexed content to make it available for retrieval.
func (s *Service) PublishKnowledgeSource(ctx context.Context, sourceID string, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, exists := s.sources[sourceID]
	if !exists {
		return ErrKnowledgeSourceNotFound
	}
	if src.SourceVersion != expectedVersion {
		return ErrVersionMismatch
	}
	if src.ReadyVersion < src.SourceVersion {
		return ErrSourceNotReady
	}

	src.State = StatePublished
	src.UpdatedAt = time.Now()
	return nil
}

// RevokeKnowledgeSource immediately blocks all chunks of this source from retrieval.
func (s *Service) RevokeKnowledgeSource(ctx context.Context, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, exists := s.sources[sourceID]
	if !exists {
		return ErrKnowledgeSourceNotFound
	}

	src.State = StateRevoked
	src.UpdatedAt = time.Now()
	return nil
}

// ListSources lists sources for a knowledge base.
func (s *Service) ListSources(ctx context.Context, kbID string) []*KnowledgeSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var res []*KnowledgeSource
	for _, src := range s.sources {
		if src.KnowledgeBaseID == kbID {
			res = append(res, src)
		}
	}
	return res
}

// RetrieveChunks finds relevant chunks across allowed knowledge bases with active course access verification.
func (s *Service) RetrieveChunks(ctx context.Context, userID string, kbIDs []string, query string, maxResults int) ([]Citation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowedKBs := make(map[string]bool)
	for _, id := range kbIDs {
		allowedKBs[id] = true
	}

	queryLower := strings.ToLower(query)
	var citations []Citation

	for srcID, chunks := range s.chunks {
		src, exists := s.sources[srcID]
		if !exists || src.State != StatePublished {
			// Revoked, draft, or not published sources are never returned
			continue
		}
		if !allowedKBs[src.KnowledgeBaseID] {
			continue
		}

		// Check permission if linked to a course
		if src.Kind == KindCourseText && src.SourceRef != "" {
			if !s.accessChecker(ctx, userID, src.SourceRef) {
				// User does not currently hold access to this course; skip!
				continue
			}
		}

		for _, chunk := range chunks {
			chunkLower := strings.ToLower(chunk.Content)
			if strings.Contains(chunkLower, queryLower) || len(queryLower) < 3 {
				snippet := chunk.Content
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				citations = append(citations, Citation{
					ChunkID:  chunk.ID,
					SourceID: src.ID,
					Title:    chunk.Title,
					Snippet:  snippet,
					Score:    0.92,
				})
				if len(citations) >= maxResults {
					return citations, nil
				}
			}
		}
	}

	return citations, nil
}
