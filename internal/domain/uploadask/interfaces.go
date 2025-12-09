package uploadask

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// ObjectStorage abstracts blob storage (R2/S3/Supabase/local).
type ObjectStorage interface {
	Put(ctx context.Context, key string, data []byte, mimeType string) (StoredObject, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// StoredObject captures persisted blob metadata.
type StoredObject struct {
	Key      string
	Size     int64
	MimeType string
	ETag     string
}

// Embedder produces embeddings for free form text.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// LLM generates answers for a question and context.
type LLM interface {
	Chat(ctx context.Context, messages []LLMMessage) (string, error)
}

// LLMMessage mirrors a simplified chat payload.
type LLMMessage struct {
	Role    string
	Content string
}

// Retriever performs similarity search across stored chunks.
type Retriever interface {
	Search(ctx context.Context, userID int64, embedding []float32, filter DocumentFilter) ([]RetrievedChunk, error)
}

// DocumentRepository persists document metadata.
type DocumentRepository interface {
	Create(ctx context.Context, doc Document) error
	UpdateStatus(ctx context.Context, docID uuid.UUID, status DocumentStatus, failureReason *string) error
	Get(ctx context.Context, docID uuid.UUID, userID int64) (Document, bool, error)
	List(ctx context.Context, userID int64, filter DocumentFilter) ([]Document, error)
}

// FileObjectRepository persists uploaded file metadata.
type FileObjectRepository interface {
	Create(ctx context.Context, file FileObject) error
	FindByDocument(ctx context.Context, docID uuid.UUID) (FileObject, bool, error)
}

// ChunkRepository stores embedded chunks.
type ChunkRepository interface {
	InsertBatch(ctx context.Context, chunks []DocumentChunk) error
	SearchSimilar(ctx context.Context, userID int64, embedding []float32, filter DocumentFilter) ([]RetrievedChunk, error)
}

// QASessionRepository persists user sessions.
type QASessionRepository interface {
	Create(ctx context.Context, session QASession) error
	Find(ctx context.Context, id uuid.UUID, userID int64) (QASession, bool, error)
	List(ctx context.Context, userID int64) ([]QASession, error)
}

// QueryLogRepository records question/answer pairs.
type QueryLogRepository interface {
	Append(ctx context.Context, log QueryLog) error
	ListBySession(ctx context.Context, sessionID uuid.UUID, userID int64) ([]QueryLog, error)
}

// MessageLog persists conversational turns for a session.
type MessageLog interface {
	Append(ctx context.Context, msg ConversationMessage) error
	ListRecent(ctx context.Context, userID int64, sessionID uuid.UUID, maxTokens int, maxMessages int) ([]ConversationMessage, error)
}

// MemoryStore manages long-term memories for a user/session.
type MemoryStore interface {
	Upsert(ctx context.Context, mem MemoryRecord) error
	Search(ctx context.Context, userID int64, sessionID uuid.UUID, embedding []float32, k int) ([]RetrievedMemory, error)
	Prune(ctx context.Context, userID int64, sessionID *uuid.UUID, limit int) error
}

// JobQueue enqueues processing tasks.
type JobQueue interface {
	Enqueue(ctx context.Context, name string, payload any) error
}

// Chunker splits raw text into contextual pieces.
type Chunker interface {
	Chunk(text string) []ChunkCandidate
}

// ChunkCandidate is produced by the chunker before embedding.
type ChunkCandidate struct {
	Index      int
	Content    string
	TokenCount int
}

// DocumentFilter restricts scope to a set of documents or statuses.
type DocumentFilter struct {
	DocumentIDs []uuid.UUID
	Statuses    []DocumentStatus
}

// RetrievedChunk bundles the chunk and score.
type RetrievedChunk struct {
	Chunk     DocumentChunk
	Document  Document
	Score     float64
	CreatedAt time.Time
}
