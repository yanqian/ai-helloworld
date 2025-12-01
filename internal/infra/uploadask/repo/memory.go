package repo

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// MemoryDocumentRepository is a simple in-memory store for documents.
type MemoryDocumentRepository struct {
	mu   sync.RWMutex
	data map[uuid.UUID]domain.Document
}

// NewMemoryDocumentRepository constructs a document repository.
func NewMemoryDocumentRepository() *MemoryDocumentRepository {
	return &MemoryDocumentRepository{
		data: make(map[uuid.UUID]domain.Document),
	}
}

func (r *MemoryDocumentRepository) Create(_ context.Context, doc domain.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[doc.ID] = doc
	return nil
}

func (r *MemoryDocumentRepository) UpdateStatus(_ context.Context, docID uuid.UUID, status domain.DocumentStatus, failureReason *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	doc, ok := r.data[docID]
	if !ok {
		return nil
	}
	doc.Status = status
	doc.FailureReason = failureReason
	doc.UpdatedAt = time.Now()
	r.data[docID] = doc
	return nil
}

func (r *MemoryDocumentRepository) Get(_ context.Context, docID uuid.UUID, userID int64) (domain.Document, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	doc, ok := r.data[docID]
	if !ok || doc.UserID != userID {
		return domain.Document{}, false, nil
	}
	return doc, true, nil
}

func (r *MemoryDocumentRepository) List(_ context.Context, userID int64, filter domain.DocumentFilter) ([]domain.Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Document, 0)
	allowed := make(map[domain.DocumentStatus]bool)
	for _, st := range filter.Statuses {
		allowed[st] = true
	}
	for _, doc := range r.data {
		if doc.UserID != userID {
			continue
		}
		if len(allowed) > 0 && !allowed[doc.Status] {
			continue
		}
		if len(filter.DocumentIDs) > 0 {
			match := false
			for _, id := range filter.DocumentIDs {
				if id == doc.ID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, doc)
	}
	return out, nil
}

var _ domain.DocumentRepository = (*MemoryDocumentRepository)(nil)

// MemoryFileRepository stores file metadata.
type MemoryFileRepository struct {
	mu    sync.RWMutex
	files map[uuid.UUID]domain.FileObject
}

// NewMemoryFileRepository constructs a file repository.
func NewMemoryFileRepository() *MemoryFileRepository {
	return &MemoryFileRepository{
		files: make(map[uuid.UUID]domain.FileObject),
	}
}

func (r *MemoryFileRepository) Create(_ context.Context, file domain.FileObject) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[file.DocumentID] = file
	return nil
}

func (r *MemoryFileRepository) FindByDocument(_ context.Context, docID uuid.UUID) (domain.FileObject, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	file, ok := r.files[docID]
	return file, ok, nil
}

var _ domain.FileObjectRepository = (*MemoryFileRepository)(nil)

// MemoryChunkRepository stores embedded chunks for retrieval.
type MemoryChunkRepository struct {
	mu   sync.RWMutex
	data map[uuid.UUID][]domain.DocumentChunk
	docs domain.DocumentRepository
}

// NewMemoryChunkRepository constructs a chunk repository.
func NewMemoryChunkRepository(docs domain.DocumentRepository) *MemoryChunkRepository {
	return &MemoryChunkRepository{
		data: make(map[uuid.UUID][]domain.DocumentChunk),
		docs: docs,
	}
}

func (r *MemoryChunkRepository) InsertBatch(_ context.Context, chunks []domain.DocumentChunk) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, chunk := range chunks {
		r.data[chunk.DocumentID] = append(r.data[chunk.DocumentID], chunk)
	}
	return nil
}

func (r *MemoryChunkRepository) SearchSimilar(ctx context.Context, userID int64, embedding []float32, filter domain.DocumentFilter) ([]domain.RetrievedChunk, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allowedDocs := make(map[uuid.UUID]bool)
	for _, id := range filter.DocumentIDs {
		allowedDocs[id] = true
	}
	allowedStatuses := make(map[domain.DocumentStatus]bool)
	for _, st := range filter.Statuses {
		allowedStatuses[st] = true
	}

	results := make([]domain.RetrievedChunk, 0)
	for docID, chunks := range r.data {
		if len(allowedDocs) > 0 && !allowedDocs[docID] {
			continue
		}
		doc, found, _ := r.docs.Get(ctx, docID, userID)
		if !found {
			continue
		}
		if len(allowedStatuses) > 0 && !allowedStatuses[doc.Status] {
			continue
		}
		for _, chunk := range chunks {
			score := cosineSimilarity(embedding, chunk.Embedding)
			results = append(results, domain.RetrievedChunk{
				Chunk:     chunk,
				Document:  doc,
				Score:     score,
				CreatedAt: chunk.CreatedAt,
			})
		}
	}

	// simple sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results, nil
}

var _ domain.ChunkRepository = (*MemoryChunkRepository)(nil)

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	var magA float64
	var magB float64
	for i := 0; i < len(a); i++ {
		dot += float64(a[i] * b[i])
		magA += float64(a[i] * a[i])
		magB += float64(b[i] * b[i])
	}
	den := math.Sqrt(magA) * math.Sqrt(magB)
	if den == 0 {
		return 0
	}
	return dot / den
}

// MemoryQASessionRepository stores QA sessions.
type MemoryQASessionRepository struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]domain.QASession
}

// NewMemoryQASessionRepository constructs a session repository.
func NewMemoryQASessionRepository() *MemoryQASessionRepository {
	return &MemoryQASessionRepository{
		sessions: make(map[uuid.UUID]domain.QASession),
	}
}

func (r *MemoryQASessionRepository) Create(_ context.Context, session domain.QASession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}

func (r *MemoryQASessionRepository) Find(_ context.Context, id uuid.UUID, userID int64) (domain.QASession, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok || session.UserID != userID {
		return domain.QASession{}, false, nil
	}
	return session, true, nil
}

func (r *MemoryQASessionRepository) List(_ context.Context, userID int64) ([]domain.QASession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.QASession, 0)
	for _, session := range r.sessions {
		if session.UserID == userID {
			out = append(out, session)
		}
	}
	return out, nil
}

var _ domain.QASessionRepository = (*MemoryQASessionRepository)(nil)

// MemoryQueryLogRepository stores query logs.
type MemoryQueryLogRepository struct {
	mu   sync.RWMutex
	logs map[uuid.UUID][]domain.QueryLog
}

// NewMemoryQueryLogRepository constructs a query log repository.
func NewMemoryQueryLogRepository() *MemoryQueryLogRepository {
	return &MemoryQueryLogRepository{
		logs: make(map[uuid.UUID][]domain.QueryLog),
	}
}

func (r *MemoryQueryLogRepository) Append(_ context.Context, log domain.QueryLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs[log.SessionID] = append(r.logs[log.SessionID], log)
	return nil
}

func (r *MemoryQueryLogRepository) ListBySession(_ context.Context, sessionID uuid.UUID, userID int64) ([]domain.QueryLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	logs := r.logs[sessionID]
	out := make([]domain.QueryLog, 0, len(logs))
	for _, log := range logs {
		out = append(out, log)
	}
	return out, nil
}

var _ domain.QueryLogRepository = (*MemoryQueryLogRepository)(nil)
