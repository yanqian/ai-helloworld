package uploadask

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Config drives upload and query limits.
type Config struct {
	VectorDim       int
	MaxFileBytes    int64
	MaxRetrieved    int
	MaxPreviewChars int
}

// Service orchestrates the Upload-and-Ask workflows.
type Service struct {
	cfg      Config
	docs     DocumentRepository
	files    FileObjectRepository
	chunks   ChunkRepository
	sessions QASessionRepository
	logs     QueryLogRepository
	storage  ObjectStorage
	embedder Embedder
	llm      LLM
	chunker  Chunker
	queue    JobQueue
	logger   *slog.Logger
}

// NewService constructs a Service.
func NewService(cfg Config, docs DocumentRepository, files FileObjectRepository, chunks ChunkRepository, sessions QASessionRepository, logs QueryLogRepository, storage ObjectStorage, embedder Embedder, llm LLM, chunker Chunker, queue JobQueue, logger *slog.Logger) *Service {
	return &Service{
		cfg:      cfg,
		docs:     docs,
		files:    files,
		chunks:   chunks,
		sessions: sessions,
		logs:     logs,
		storage:  storage,
		embedder: embedder,
		llm:      llm,
		chunker:  chunker,
		queue:    queue,
		logger:   logger.With("component", "uploadask.service"),
	}
}

// UploadRequest captures a multipart submission.
type UploadRequest struct {
	Filename string
	Title    string
	MimeType string
	Content  []byte
}

// UploadResponse returns document metadata after enqueueing.
type UploadResponse struct {
	Document Document `json:"document"`
}

// AskRequest contains the question payload.
type AskRequest struct {
	Query       string
	SessionID   *uuid.UUID
	DocumentIDs []uuid.UUID
	TopK        int
}

// AskResponse is returned to the HTTP handler.
type AskResponse struct {
	SessionID uuid.UUID     `json:"sessionId"`
	Answer    string        `json:"answer"`
	Sources   []ChunkSource `json:"sources"`
	LatencyMs int64         `json:"latencyMs"`
}

// Upload persists the document metadata, stores the blob, and enqueues processing.
func (s *Service) Upload(ctx context.Context, userID int64, req UploadRequest) (UploadResponse, error) {
	if userID == 0 {
		return UploadResponse{}, apperrors.Wrap("unauthorized", "missing user", nil)
	}
	if len(req.Content) == 0 {
		return UploadResponse{}, apperrors.Wrap("invalid_input", "file content cannot be empty", nil)
	}
	if s.cfg.MaxFileBytes > 0 && int64(len(req.Content)) > s.cfg.MaxFileBytes {
		return UploadResponse{}, apperrors.Wrap("invalid_input", "file exceeds maximum allowed size", nil)
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = "document.txt"
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = filename
	}
	now := time.Now()
	doc := Document{
		ID:        uuid.New(),
		UserID:    userID,
		Title:     title,
		Source:    DocumentSourceUpload,
		Status:    DocumentStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.docs.Create(ctx, doc); err != nil {
		return UploadResponse{}, apperrors.Wrap("storage_error", "failed to persist document", err)
	}

	mime := req.MimeType
	if mime == "" {
		mime = http.DetectContentType(req.Content)
	}
	storageKey := fmt.Sprintf("uploads/%d/%s/%s", userID, doc.ID.String(), sanitizeFilename(filename))
	obj, err := s.storage.Put(ctx, storageKey, req.Content, mime)
	if err != nil {
		return UploadResponse{}, apperrors.Wrap("storage_error", "failed to store file", err)
	}

	file := FileObject{
		ID:         uuid.New(),
		DocumentID: doc.ID,
		StorageKey: obj.Key,
		SizeBytes:  obj.Size,
		MimeType:   obj.MimeType,
		ETag:       obj.ETag,
		CreatedAt:  now,
	}
	if err := s.files.Create(ctx, file); err != nil {
		return UploadResponse{}, apperrors.Wrap("storage_error", "failed to persist file metadata", err)
	}

	if s.queue != nil {
		payload := map[string]any{
			"document_id": doc.ID.String(),
			"user_id":     userID,
		}
		if err := s.queue.Enqueue(ctx, "process_document", payload); err != nil {
			s.logger.Warn("enqueue process_document failed", "error", err)
		}
	}

	return UploadResponse{Document: doc}, nil
}

// ProcessDocument extracts, chunks, embeds, and stores chunks.
func (s *Service) ProcessDocument(ctx context.Context, docID uuid.UUID, userID int64) error {
	doc, found, err := s.docs.Get(ctx, docID, userID)
	if err != nil {
		return apperrors.Wrap("storage_error", "failed to load document", err)
	}
	if !found {
		return apperrors.Wrap("not_found", "document not found", nil)
	}
	if doc.Status == DocumentStatusProcessed {
		return nil
	}

	if err := s.docs.UpdateStatus(ctx, docID, DocumentStatusProcessing, nil); err != nil {
		return apperrors.Wrap("storage_error", "failed to update status", err)
	}

	file, found, err := s.files.FindByDocument(ctx, docID)
	if err != nil {
		return apperrors.Wrap("storage_error", "failed to load file metadata", err)
	}
	if !found {
		return apperrors.Wrap("not_found", "file not found for document", nil)
	}

	reader, err := s.storage.Get(ctx, file.StorageKey)
	if err != nil {
		_ = s.docs.UpdateStatus(ctx, docID, DocumentStatusFailed, ptrString("failed to read storage"))
		return apperrors.Wrap("storage_error", "failed to fetch stored file", err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		_ = s.docs.UpdateStatus(ctx, docID, DocumentStatusFailed, ptrString("failed to read storage"))
		return apperrors.Wrap("storage_error", "failed to read stored file", err)
	}

	candidates := s.chunker.Chunk(string(raw))
	if len(candidates) == 0 {
		reason := "no content to process"
		_ = s.docs.UpdateStatus(ctx, docID, DocumentStatusFailed, &reason)
		return apperrors.Wrap("invalid_input", reason, nil)
	}

	texts := make([]string, 0, len(candidates))
	for _, c := range candidates {
		texts = append(texts, c.Content)
	}
	embeddings, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		_ = s.docs.UpdateStatus(ctx, docID, DocumentStatusFailed, ptrString("embedding failed"))
		return apperrors.Wrap("embedding_error", "failed to embed chunks", err)
	}
	now := time.Now()
	chunks := make([]DocumentChunk, 0, len(candidates))
	for i, c := range candidates {
		embedding := make([]float32, len(embeddings[i]))
		copy(embedding, embeddings[i])
		chunks = append(chunks, DocumentChunk{
			ID:         uuid.New(),
			DocumentID: docID,
			ChunkIndex: c.Index,
			Content:    c.Content,
			TokenCount: c.TokenCount,
			Embedding:  embedding,
			CreatedAt:  now,
		})
	}
	if err := s.chunks.InsertBatch(ctx, chunks); err != nil {
		_ = s.docs.UpdateStatus(ctx, docID, DocumentStatusFailed, ptrString("persisting chunks failed"))
		return apperrors.Wrap("storage_error", "failed to persist chunks", err)
	}
	if err := s.docs.UpdateStatus(ctx, docID, DocumentStatusProcessed, nil); err != nil {
		return apperrors.Wrap("storage_error", "failed to finalize document", err)
	}
	return nil
}

// Ask performs similarity search then calls the LLM to answer.
func (s *Service) Ask(ctx context.Context, userID int64, req AskRequest) (AskResponse, error) {
	if userID == 0 {
		return AskResponse{}, apperrors.Wrap("unauthorized", "missing user", nil)
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return AskResponse{}, apperrors.Wrap("invalid_input", "query cannot be empty", nil)
	}
	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.MaxRetrieved
		if topK <= 0 {
			topK = 8
		}
	}
	embedding, err := s.embedText(ctx, query)
	if err != nil {
		return AskResponse{}, err
	}
	filter := DocumentFilter{
		DocumentIDs: req.DocumentIDs,
		Statuses:    []DocumentStatus{DocumentStatusProcessed},
	}
	results, err := s.chunks.SearchSimilar(ctx, userID, embedding, filter)
	if err != nil {
		return AskResponse{}, apperrors.Wrap("storage_error", "search failed", err)
	}
	if len(results) > topK {
		results = results[:topK]
	}

	sessionID := uuid.New()
	if req.SessionID != nil {
		sessionID = *req.SessionID
	} else {
		session := QASession{
			ID:        sessionID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}
		_ = s.sessions.Create(ctx, session)
	}

	start := time.Now()
	answer := s.answerWithContext(ctx, query, results)
	latency := time.Since(start).Milliseconds()

	sources := make([]ChunkSource, 0, len(results))
	for _, r := range results {
		sources = append(sources, ChunkSource{
			DocumentID: r.Chunk.DocumentID,
			ChunkIndex: r.Chunk.ChunkIndex,
			Score:      r.Score,
			Preview:    snippet(r.Chunk.Content, s.cfg.MaxPreviewChars),
		})
	}
	log := QueryLog{
		ID:           uuid.New(),
		SessionID:    sessionID,
		QueryText:    query,
		ResponseText: answer,
		LatencyMs:    latency,
		Sources:      sources,
		CreatedAt:    time.Now(),
	}
	_ = s.logs.Append(ctx, log)

	return AskResponse{
		SessionID: sessionID,
		Answer:    answer,
		Sources:   sources,
		LatencyMs: latency,
	}, nil
}

// ListDocuments returns user scoped documents.
func (s *Service) ListDocuments(ctx context.Context, userID int64, filter DocumentFilter) ([]Document, error) {
	if userID == 0 {
		return nil, apperrors.Wrap("unauthorized", "missing user", nil)
	}
	return s.docs.List(ctx, userID, filter)
}

// GetDocument fetches a single document.
func (s *Service) GetDocument(ctx context.Context, userID int64, docID uuid.UUID) (Document, error) {
	doc, found, err := s.docs.Get(ctx, docID, userID)
	if err != nil {
		return Document{}, apperrors.Wrap("storage_error", "failed to fetch document", err)
	}
	if !found {
		return Document{}, apperrors.Wrap("not_found", "document not found", nil)
	}
	return doc, nil
}

// ListSessionLogs returns historical Q&A exchanges.
func (s *Service) ListSessionLogs(ctx context.Context, userID int64, sessionID uuid.UUID) ([]QueryLog, error) {
	session, found, err := s.sessions.Find(ctx, sessionID, userID)
	if err != nil {
		return nil, apperrors.Wrap("storage_error", "failed to load session", err)
	}
	if !found || session.UserID != userID {
		return nil, apperrors.Wrap("not_found", "session not found", nil)
	}
	return s.logs.ListBySession(ctx, sessionID, userID)
}

// ListSessions returns QA sessions for a user.
func (s *Service) ListSessions(ctx context.Context, userID int64) ([]QASession, error) {
	if userID == 0 {
		return nil, apperrors.Wrap("unauthorized", "missing user", nil)
	}
	return s.sessions.List(ctx, userID)
}

func (s *Service) embedText(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.embedder.Embed(ctx, []string{text})
	if err != nil {
		return nil, apperrors.Wrap("embedding_error", "failed to embed query", err)
	}
	if len(embeddings) == 0 {
		return nil, apperrors.Wrap("embedding_error", "no embedding returned", nil)
	}
	return embeddings[0], nil
}

func (s *Service) answerWithContext(ctx context.Context, query string, chunks []RetrievedChunk) string {
	var contextBuilder strings.Builder
	for _, rc := range chunks {
		chunk := rc.Chunk
		contextBuilder.WriteString(fmt.Sprintf("Doc %s chunk %d:\n%s\n\n", chunk.DocumentID.String(), chunk.ChunkIndex, chunk.Content))
	}
	messages := []LLMMessage{
		{Role: "system", Content: "You are a helpful assistant that answers questions using the provided context. Cite document and chunk numbers in the response."},
		{Role: "user", Content: fmt.Sprintf("Question: %s\n\nContext:\n%s", query, contextBuilder.String())},
	}
	answer, err := s.llm.Chat(ctx, messages)
	if err != nil || strings.TrimSpace(answer) == "" {
		s.logger.Warn("llm chat failed, falling back to heuristic answer", "error", err)
		if len(chunks) == 0 {
			return "No relevant context available to answer this question."
		}
		return fmt.Sprintf("%s\n\nBased on %d context chunks.", query, len(chunks))
	}
	return answer
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	if name == "" {
		return "file"
	}
	return name
}

func snippet(body string, max int) string {
	if max <= 0 || len(body) <= max {
		return body
	}
	return strings.TrimSpace(body[:max]) + "..."
}

func ptrString(val string) *string {
	return &val
}
