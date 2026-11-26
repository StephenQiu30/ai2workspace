package knowledge

import (
	"errors"
	"time"
)

var (
	ErrKnowledgeBaseNotFound    = errors.New("knowledge base not found")
	ErrKnowledgeSourceNotFound  = errors.New("knowledge source not found")
	ErrUnauthorizedSourceScope  = errors.New("user is not authorized to manage this knowledge resource")
	ErrSourceNotReady           = errors.New("knowledge source index is not ready for publication")
	ErrSourceRevoked            = errors.New("knowledge source has been revoked and cannot be published directly")
	ErrIndexJobNotFound         = errors.New("index job not found")
	ErrVersionMismatch          = errors.New("optimistic lock version mismatch")
	ErrInvalidSourceKind        = errors.New("unsupported knowledge source kind")
	ErrInvalidSourceRef         = errors.New("course_text source requires a non-empty courseId sourceRef")
	ErrBudgetPaused             = errors.New("index job paused due to AI budget limit")
	ErrAccessCheckerUnavailable = errors.New("course access verification service is not configured")
)

// Visibility defines who can see a knowledge base.
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

// KnowledgeSource represents a registered piece of content with independent draft/ready/published versions.
type KnowledgeSource struct {
	ID               string      `json:"id"`
	KnowledgeBaseID  string      `json:"knowledgeBaseId"`
	Kind             SourceKind  `json:"kind"`
	SourceRef        string      `json:"sourceRef"` // e.g. course_id
	Title            string      `json:"title"`
	Content          string      `json:"content,omitempty"`
	SourceVersion    int         `json:"sourceVersion"`
	ReadyVersion     int         `json:"readyVersion"`
	PublishedVersion int         `json:"publishedVersion"`
	State            SourceState `json:"state"`
	RevokedAt        *time.Time  `json:"revokedAt,omitempty"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

// KnowledgeChunk represents an indexed slice of text bound to a specific source version.
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

// IndexJob tracks background parsing, chunking, and indexing with budget governance.
type IndexJob struct {
	ID            string    `json:"id"`
	SourceID      string    `json:"sourceId"`
	SourceVersion int       `json:"sourceVersion"`
	Stage         string    `json:"stage"` // "budget_admission", "chunking", "indexing", "completed", "failed"
	State         string    `json:"state"` // "running", "success", "failed", "paused_budget"
	Retryable     bool      `json:"retryable"`
	ErrorCode     string    `json:"errorCode,omitempty"`
	CostCents     int       `json:"costCents,omitempty"`
	ReservationID string    `json:"reservationId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Citation is a traceable reference returned with agent answers and re-verified on historical reads.
type Citation struct {
	ChunkID       string  `json:"chunkId"`
	SourceID      string  `json:"sourceId"`
	SourceVersion int     `json:"sourceVersion"`
	ChunkIndex    int     `json:"chunkIndex"`
	CourseID      string  `json:"courseId,omitempty"`
	Title         string  `json:"title"`
	Snippet       string  `json:"snippet"`
	Content       string  `json:"-"`
	Score         float64 `json:"score"`
}
