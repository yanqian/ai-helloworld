package uploadask

import (
	"time"

	"github.com/google/uuid"
)

// DocumentStatus tracks pipeline progress.
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusProcessed  DocumentStatus = "processed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

// DocumentSource describes how the document was ingested.
type DocumentSource string

const (
	DocumentSourceUpload DocumentSource = "upload"
	DocumentSourceURL    DocumentSource = "url"
)

// Document represents a user scoped file submission.
type Document struct {
	ID            uuid.UUID      `json:"id"`
	UserID        int64          `json:"userId"`
	Title         string         `json:"title"`
	Source        DocumentSource `json:"source"`
	Status        DocumentStatus `json:"status"`
	FailureReason *string        `json:"failureReason,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// FileObject stores uploaded blob metadata.
type FileObject struct {
	ID         uuid.UUID `json:"id"`
	DocumentID uuid.UUID `json:"documentId"`
	StorageKey string    `json:"storageKey"`
	SizeBytes  int64     `json:"sizeBytes"`
	MimeType   string    `json:"mimeType"`
	ETag       string    `json:"etag"`
	CreatedAt  time.Time `json:"createdAt"`
}

// DocumentChunk contains an embedded slice of a document.
type DocumentChunk struct {
	ID         uuid.UUID `json:"id"`
	DocumentID uuid.UUID `json:"documentId"`
	ChunkIndex int       `json:"chunkIndex"`
	Content    string    `json:"content"`
	TokenCount int       `json:"tokenCount"`
	Embedding  []float32 `json:"embedding"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ChunkSource captures retrieval metadata returned to the client.
type ChunkSource struct {
	DocumentID uuid.UUID `json:"documentId"`
	ChunkIndex int       `json:"chunkIndex"`
	Score      float64   `json:"score"`
	Preview    string    `json:"preview"`
}

// QASession groups multiple questions from the same user.
type QASession struct {
	ID        uuid.UUID `json:"id"`
	UserID    int64     `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// QueryLog records a single question/answer exchange.
type QueryLog struct {
	ID           uuid.UUID     `json:"id"`
	SessionID    uuid.UUID     `json:"sessionId"`
	QueryText    string        `json:"queryText"`
	ResponseText string        `json:"responseText"`
	LatencyMs    int64         `json:"latencyMs"`
	Sources      []ChunkSource `json:"sources"`
	CreatedAt    time.Time     `json:"createdAt"`
}

// MessageRole enumerates chat roles stored in upload_qa_messages.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

// ConversationMessage captures a chat turn for an Upload & Ask session.
type ConversationMessage struct {
	ID         int64       `json:"id"`
	SessionID  uuid.UUID   `json:"sessionId"`
	UserID     int64       `json:"userId"`
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	TokenCount int         `json:"tokenCount"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// MemorySource indicates how a memory was produced.
type MemorySource string

const (
	MemorySourceQATurn  MemorySource = "qa_turn"
	MemorySourceSummary MemorySource = "summary"
	MemorySourceManual  MemorySource = "manual"
)

// MemoryRecord stores long-term conversational context.
type MemoryRecord struct {
	ID         int64        `json:"id"`
	SessionID  uuid.UUID    `json:"sessionId"`
	UserID     int64        `json:"userId"`
	Source     MemorySource `json:"source"`
	Content    string       `json:"content"`
	Embedding  []float32    `json:"embedding,omitempty"`
	Importance int16        `json:"importance"`
	CreatedAt  time.Time    `json:"createdAt"`
}

// RetrievedMemory includes the memory and similarity score.
type RetrievedMemory struct {
	Memory    MemoryRecord `json:"memory"`
	Score     float64      `json:"score"`
	CreatedAt time.Time    `json:"createdAt"`
}
