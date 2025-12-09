package uploadask

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Config drives upload and query limits.
type Config struct {
	VectorDim       int
	MaxFileBytes    int64
	MaxRetrieved    int
	MaxPreviewChars int
	Memory          MemoryConfig
}

// MemoryConfig controls conversational memory behavior.
type MemoryConfig struct {
	Enabled            bool
	TopKMems           int
	MaxHistoryTokens   int
	MemoryVectorDim    int
	SummaryEveryNTurns int
	PruneLimit         int
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
	messages MessageLog
	memories MemoryStore
	logger   *slog.Logger
}

// NewService constructs a Service.
func NewService(cfg Config, docs DocumentRepository, files FileObjectRepository, chunks ChunkRepository, sessions QASessionRepository, logs QueryLogRepository, messages MessageLog, memories MemoryStore, storage ObjectStorage, embedder Embedder, llm LLM, chunker Chunker, queue JobQueue, logger *slog.Logger) *Service {
	return &Service{
		cfg:      cfg,
		docs:     docs,
		files:    files,
		chunks:   chunks,
		sessions: sessions,
		logs:     logs,
		messages: messages,
		memories: memories,
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
	Query            string
	SessionID        *uuid.UUID
	DocumentIDs      []uuid.UUID
	TopK             int
	TopKMems         *int
	MaxHistoryTokens *int
	IncludeHistory   *bool
}

// AskResponse is returned to the HTTP handler.
type AskResponse struct {
	SessionID         uuid.UUID         `json:"sessionId"`
	Answer            string            `json:"answer"`
	Sources           []ChunkSource     `json:"sources"`
	Memories          []RetrievedMemory `json:"memories,omitempty"`
	UsedHistoryTokens int               `json:"usedHistoryTokens"`
	LatencyMs         int64             `json:"latencyMs"`
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
	s.logger.Info("process_document start", "document_id", docID, "user_id", userID)
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
	s.logger.Info("process_document complete", "document_id", docID, "user_id", userID, "chunks", len(chunks))
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
	topKDocs := req.TopK
	if topKDocs <= 0 {
		topKDocs = s.cfg.MaxRetrieved
		if topKDocs <= 0 {
			topKDocs = 8
		}
	}
	topKMems := s.resolveTopKMems(req.TopKMems)
	maxHistoryTokens := s.resolveMaxHistoryTokens(req.MaxHistoryTokens)
	includeHistory := s.shouldIncludeHistory(req.IncludeHistory)

	sessionID, err := s.ensureSession(ctx, userID, req.SessionID)
	if err != nil {
		return AskResponse{}, err
	}

	history, usedHistoryTokens := s.loadHistory(ctx, userID, sessionID, maxHistoryTokens, includeHistory)
	semanticQuery := s.buildSemanticQuery(query, history)
	embedding, err := s.embedText(ctx, semanticQuery)
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
	if len(results) > topKDocs {
		results = results[:topKDocs]
	}
	memories := s.searchMemories(ctx, userID, sessionID, embedding, topKMems)

	messages := s.buildPrompt(query, results, memories, history, includeHistory)
	start := time.Now()
	answer := s.answerWithPrompt(ctx, query, results, memories, messages)
	latency := time.Since(start).Milliseconds()

	sources := s.buildChunkSources(results)
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

	s.appendMessages(ctx, userID, sessionID, query, answer)
	s.persistTurnMemory(ctx, userID, sessionID, query, answer)
	s.maybeTriggerSummary(ctx, userID, sessionID, len(history)+2)

	return AskResponse{
		SessionID:         sessionID,
		Answer:            answer,
		Sources:           sources,
		Memories:          memories,
		UsedHistoryTokens: usedHistoryTokens,
		LatencyMs:         latency,
	}, nil
}

func (s *Service) resolveTopKMems(val *int) int {
	if val != nil {
		return *val
	}
	return s.cfg.Memory.TopKMems
}

func (s *Service) resolveMaxHistoryTokens(val *int) int {
	if val != nil {
		return *val
	}
	return s.cfg.Memory.MaxHistoryTokens
}

func (s *Service) shouldIncludeHistory(flag *bool) bool {
	if flag != nil {
		return *flag
	}
	return s.cfg.Memory.Enabled
}

func (s *Service) ensureSession(ctx context.Context, userID int64, requested *uuid.UUID) (uuid.UUID, error) {
	if requested != nil {
		session, found, err := s.sessions.Find(ctx, *requested, userID)
		if err != nil {
			return uuid.Nil, apperrors.Wrap("storage_error", "failed to load session", err)
		}
		if !found || session.UserID != userID {
			return uuid.Nil, apperrors.Wrap("not_found", "session not found", nil)
		}
		return session.ID, nil
	}
	id := uuid.New()
	session := QASession{
		ID:        id,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	_ = s.sessions.Create(ctx, session)
	return id, nil
}

func (s *Service) loadHistory(ctx context.Context, userID int64, sessionID uuid.UUID, maxTokens int, include bool) ([]ConversationMessage, int) {
	if s.messages == nil || !include {
		return nil, 0
	}
	msgs, err := s.messages.ListRecent(ctx, userID, sessionID, maxTokens, 50)
	if err != nil {
		s.logger.Warn("failed to list recent messages", "error", err)
		return nil, 0
	}
	return msgs, sumTokens(msgs)
}

func (s *Service) buildSemanticQuery(query string, history []ConversationMessage) string {
	summary := summarizeHistory(history, 3, 500)
	if summary == "" {
		return query
	}
	return fmt.Sprintf("%s\n\nRecent history: %s", query, summary)
}

func summarizeHistory(history []ConversationMessage, maxEntries int, maxChars int) string {
	if len(history) == 0 || maxEntries <= 0 {
		return ""
	}
	var builder strings.Builder
	count := 0
	for i := len(history) - 1; i >= 0 && count < maxEntries; i-- {
		line := fmt.Sprintf("%s: %s", history[i].Role, history[i].Content)
		if maxChars > 0 && builder.Len()+len(line) > maxChars {
			break
		}
		if builder.Len() > 0 {
			builder.WriteString(" | ")
		}
		builder.WriteString(line)
		count++
	}
	return builder.String()
}

func (s *Service) searchMemories(ctx context.Context, userID int64, sessionID uuid.UUID, embedding []float32, topK int) []RetrievedMemory {
	if !s.cfg.Memory.Enabled || s.memories == nil || topK <= 0 || len(embedding) == 0 {
		return nil
	}
	memories, err := s.memories.Search(ctx, userID, sessionID, embedding, topK)
	if err != nil {
		s.logger.Warn("memory search failed", "error", err)
		return nil
	}
	return memories
}

func (s *Service) buildPrompt(query string, chunks []RetrievedChunk, memories []RetrievedMemory, history []ConversationMessage, includeHistory bool) []LLMMessage {
	messages := []LLMMessage{
		{Role: "system", Content: "You are a helpful assistant that answers questions using the provided context. Cite document and chunk numbers when using document content."},
	}
	if ctx := s.buildContextBlock(chunks, memories); ctx != "" {
		messages = append(messages, LLMMessage{Role: "system", Content: "Context:\n" + ctx})
	}
	if includeHistory {
		for _, msg := range history {
			messages = append(messages, LLMMessage{Role: string(msg.Role), Content: msg.Content})
		}
	}
	messages = append(messages, LLMMessage{Role: "user", Content: query})
	return messages
}

func (s *Service) buildContextBlock(chunks []RetrievedChunk, memories []RetrievedMemory) string {
	var builder strings.Builder
	for _, rc := range chunks {
		chunk := rc.Chunk
		builder.WriteString(fmt.Sprintf("Doc %s chunk %d:\n%s\n\n", chunk.DocumentID.String(), chunk.ChunkIndex, chunk.Content))
	}
	if len(memories) > 0 {
		builder.WriteString("Memories:\n")
		for _, mem := range memories {
			builder.WriteString(fmt.Sprintf("- [%s] %s\n", mem.Memory.Source, mem.Memory.Content))
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func (s *Service) answerWithPrompt(ctx context.Context, query string, chunks []RetrievedChunk, memories []RetrievedMemory, messages []LLMMessage) string {
	answer, err := s.llm.Chat(ctx, messages)
	if err != nil || strings.TrimSpace(answer) == "" {
		s.logger.Warn("llm chat failed, falling back to heuristic answer", "error", err)
		if len(chunks) == 0 && len(memories) == 0 {
			return "No relevant context available to answer this question."
		}
		return fmt.Sprintf("%s\n\nBased on %d context items.", query, len(chunks)+len(memories))
	}
	return answer
}

func (s *Service) buildChunkSources(results []RetrievedChunk) []ChunkSource {
	sources := make([]ChunkSource, 0, len(results))
	for _, r := range results {
		sources = append(sources, ChunkSource{
			DocumentID: r.Chunk.DocumentID,
			ChunkIndex: r.Chunk.ChunkIndex,
			Score:      r.Score,
			Preview:    snippet(r.Chunk.Content, s.cfg.MaxPreviewChars),
		})
	}
	return sources
}

func (s *Service) appendMessages(ctx context.Context, userID int64, sessionID uuid.UUID, query, answer string) {
	if s.messages == nil {
		return
	}
	now := time.Now()
	for _, msg := range []ConversationMessage{
		{SessionID: sessionID, UserID: userID, Role: MessageRoleUser, Content: query, TokenCount: estimateTokens(query), CreatedAt: now},
		{SessionID: sessionID, UserID: userID, Role: MessageRoleAssistant, Content: answer, TokenCount: estimateTokens(answer), CreatedAt: now},
	} {
		if err := s.messages.Append(ctx, msg); err != nil {
			s.logger.Warn("failed to append conversation message", "role", msg.Role, "error", err)
		}
	}
}

func (s *Service) persistTurnMemory(ctx context.Context, userID int64, sessionID uuid.UUID, query, answer string) {
	if !s.cfg.Memory.Enabled || s.memories == nil {
		return
	}
	content := fmt.Sprintf("[Q]\n%s\n[Answer]\n%s", query, answer)
	if strings.TrimSpace(answer) == "" {
		return
	}
	embedding, err := s.embedText(ctx, content)
	if err != nil {
		s.logger.Warn("failed to embed qa turn memory", "error", err)
		return
	}
	mem := MemoryRecord{
		SessionID:  sessionID,
		UserID:     userID,
		Source:     MemorySourceQATurn,
		Content:    content,
		Embedding:  embedding,
		Importance: 0,
		CreatedAt:  time.Now(),
	}
	if err := s.memories.Upsert(ctx, mem); err != nil {
		s.logger.Warn("failed to upsert qa memory", "error", err)
	}
	if s.cfg.Memory.PruneLimit > 0 {
		if err := s.memories.Prune(ctx, userID, &sessionID, s.cfg.Memory.PruneLimit); err != nil {
			s.logger.Warn("memory prune failed", "error", err)
		}
	}
}

func (s *Service) maybeTriggerSummary(ctx context.Context, userID int64, sessionID uuid.UUID, turns int) {
	if !s.cfg.Memory.Enabled || s.cfg.Memory.SummaryEveryNTurns <= 0 || s.queue == nil {
		return
	}
	if turns%s.cfg.Memory.SummaryEveryNTurns == 0 {
		payload := map[string]any{
			"session_id": sessionID.String(),
			"user_id":    userID,
		}
		if err := s.queue.Enqueue(ctx, "summarize_session", payload); err != nil {
			s.logger.Warn("summary enqueue failed", "error", err)
			return
		}
		s.logger.Debug("summary job enqueued", "turns", turns, "session_id", sessionID)
	}
}

// SummarizeSession condenses recent history into a long-term memory via the LLM and stores it.
func (s *Service) SummarizeSession(ctx context.Context, userID int64, sessionID uuid.UUID) {
	if !s.cfg.Memory.Enabled || s.llm == nil || s.embedder == nil || s.memories == nil || s.messages == nil {
		return
	}
	maxTokens := s.cfg.Memory.MaxHistoryTokens
	if maxTokens <= 0 {
		maxTokens = 800
	}
	history, err := s.messages.ListRecent(ctx, userID, sessionID, maxTokens, 200)
	if err != nil {
		s.logger.Warn("failed to load history for summary", "error", err)
		return
	}
	if len(history) == 0 {
		return
	}
	var builder strings.Builder
	for _, msg := range history {
		builder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, strings.TrimSpace(msg.Content)))
	}

	prompt := []LLMMessage{
		{
			Role: "system",
			Content: "Summarize the conversation into a concise factual note (<=120 words) suitable for long-term recall. " +
				"Omit greetings or fluff; keep actionable facts, questions, and answers.",
		},
		{Role: "user", Content: builder.String()},
	}
	summary, err := s.llm.Chat(ctx, prompt)
	if err != nil {
		s.logger.Warn("summary llm call failed", "error", err)
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	embedding, err := s.embedText(ctx, summary)
	if err != nil {
		s.logger.Warn("summary embedding failed", "error", err)
		return
	}
	mem := MemoryRecord{
		SessionID:  sessionID,
		UserID:     userID,
		Source:     MemorySourceSummary,
		Content:    summary,
		Embedding:  embedding,
		Importance: 1,
		CreatedAt:  time.Now(),
	}
	if err := s.memories.Upsert(ctx, mem); err != nil {
		s.logger.Warn("failed to upsert summary memory", "error", err)
	}
	if s.cfg.Memory.PruneLimit > 0 {
		if err := s.memories.Prune(ctx, userID, &sessionID, s.cfg.Memory.PruneLimit); err != nil {
			s.logger.Warn("summary memory prune failed", "error", err)
		}
	}
}

func sumTokens(msgs []ConversationMessage) int {
	total := 0
	for _, msg := range msgs {
		if msg.TokenCount > 0 {
			total += msg.TokenCount
		}
	}
	return total
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	words := len(strings.Fields(trimmed))
	runes := utf8.RuneCountInString(trimmed)
	tokens := runes / 4
	if tokens < words {
		tokens = words
	}
	if tokens == 0 {
		tokens = 1
	}
	return tokens
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
