package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// SQLiteDocumentRepository persists upload documents in SQLite.
type SQLiteDocumentRepository struct {
	db *sql.DB
}

// NewSQLiteDocumentRepository constructs a SQLite-backed document repository.
func NewSQLiteDocumentRepository(db *sql.DB) *SQLiteDocumentRepository {
	return &SQLiteDocumentRepository{db: db}
}

func (r *SQLiteDocumentRepository) Create(ctx context.Context, doc domain.Document) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO upload_documents (id, user_id, title, source, status, failure_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID.String(), doc.UserID, doc.Title, string(doc.Source), string(doc.Status), doc.FailureReason, formatSQLiteTime(doc.CreatedAt), formatSQLiteTime(doc.UpdatedAt))
	return err
}

func (r *SQLiteDocumentRepository) UpdateStatus(ctx context.Context, docID uuid.UUID, status domain.DocumentStatus, failureReason *string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE upload_documents
		SET status = ?, failure_reason = ?, updated_at = ?
		WHERE id = ?
	`, string(status), failureReason, formatSQLiteTime(time.Now().UTC()), docID.String())
	return err
}

func (r *SQLiteDocumentRepository) Get(ctx context.Context, docID uuid.UUID, userID int64) (domain.Document, bool, error) {
	return scanSQLiteDocument(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, source, status, failure_reason, created_at, updated_at
		FROM upload_documents
		WHERE id = ? AND user_id = ?
		LIMIT 1
	`, docID.String(), userID))
}

func (r *SQLiteDocumentRepository) List(ctx context.Context, userID int64, filter domain.DocumentFilter) ([]domain.Document, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, title, source, status, failure_reason, created_at, updated_at
		FROM upload_documents
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	allowedDocs := uuidSet(filter.DocumentIDs)
	allowedStatuses := statusSet(filter.Statuses)
	out := make([]domain.Document, 0)
	for rows.Next() {
		doc, err := scanSQLiteDocumentRow(rows)
		if err != nil {
			return nil, err
		}
		if len(allowedDocs) > 0 && !allowedDocs[doc.ID] {
			continue
		}
		if len(allowedStatuses) > 0 && !allowedStatuses[doc.Status] {
			continue
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

var _ domain.DocumentRepository = (*SQLiteDocumentRepository)(nil)

// SQLiteFileRepository persists upload file metadata in SQLite.
type SQLiteFileRepository struct {
	db *sql.DB
}

// NewSQLiteFileRepository constructs a SQLite-backed file repository.
func NewSQLiteFileRepository(db *sql.DB) *SQLiteFileRepository {
	return &SQLiteFileRepository{db: db}
}

func (r *SQLiteFileRepository) Create(ctx context.Context, file domain.FileObject) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO upload_file_objects (id, document_id, storage_key, size_bytes, mime_type, etag, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, file.ID.String(), file.DocumentID.String(), file.StorageKey, file.SizeBytes, file.MimeType, file.ETag, formatSQLiteTime(file.CreatedAt))
	return err
}

func (r *SQLiteFileRepository) FindByDocument(ctx context.Context, docID uuid.UUID) (domain.FileObject, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, document_id, storage_key, size_bytes, mime_type, etag, created_at
		FROM upload_file_objects
		WHERE document_id = ?
		LIMIT 1
	`, docID.String())
	var (
		file      domain.FileObject
		id        string
		document  string
		createdAt string
	)
	if err := row.Scan(&id, &document, &file.StorageKey, &file.SizeBytes, &file.MimeType, &file.ETag, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.FileObject{}, false, nil
		}
		return domain.FileObject{}, false, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return domain.FileObject{}, false, err
	}
	parsedDocumentID, err := uuid.Parse(document)
	if err != nil {
		return domain.FileObject{}, false, err
	}
	created, err := parseSQLiteTime(createdAt)
	if err != nil {
		return domain.FileObject{}, false, err
	}
	file.ID = parsedID
	file.DocumentID = parsedDocumentID
	file.CreatedAt = created
	return file, true, nil
}

var _ domain.FileObjectRepository = (*SQLiteFileRepository)(nil)

// SQLiteChunkRepository stores chunks and performs local similarity search.
type SQLiteChunkRepository struct {
	db *sql.DB
}

// NewSQLiteChunkRepository constructs a SQLite-backed chunk repository.
func NewSQLiteChunkRepository(db *sql.DB) *SQLiteChunkRepository {
	return &SQLiteChunkRepository{db: db}
}

func (r *SQLiteChunkRepository) InsertBatch(ctx context.Context, chunks []domain.DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO upload_document_chunks (id, document_id, chunk_index, content, token_count, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, chunk := range chunks {
		payload, err := json.Marshal(chunk.Embedding)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, chunk.ID.String(), chunk.DocumentID.String(), chunk.ChunkIndex, chunk.Content, chunk.TokenCount, string(payload), formatSQLiteTime(chunk.CreatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteChunkRepository) SearchSimilar(ctx context.Context, userID int64, embedding []float32, filter domain.DocumentFilter) ([]domain.RetrievedChunk, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.id, c.document_id, c.chunk_index, c.content, c.token_count, c.embedding, c.created_at,
			d.id, d.user_id, d.title, d.source, d.status, d.failure_reason, d.created_at, d.updated_at
		FROM upload_document_chunks c
		JOIN upload_documents d ON d.id = c.document_id
		WHERE d.user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	allowedDocs := uuidSet(filter.DocumentIDs)
	allowedStatuses := statusSet(filter.Statuses)
	results := make([]domain.RetrievedChunk, 0)
	for rows.Next() {
		chunk, doc, err := scanSQLiteRetrievedChunk(rows)
		if err != nil {
			return nil, err
		}
		if len(allowedDocs) > 0 && !allowedDocs[doc.ID] {
			continue
		}
		if len(allowedStatuses) > 0 && !allowedStatuses[doc.Status] {
			continue
		}
		results = append(results, domain.RetrievedChunk{
			Chunk:     chunk,
			Document:  doc,
			Score:     cosineSimilarity(embedding, chunk.Embedding),
			CreatedAt: chunk.CreatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > 64 {
		results = results[:64]
	}
	return results, nil
}

var _ domain.ChunkRepository = (*SQLiteChunkRepository)(nil)

// SQLiteQASessionRepository persists QA sessions in SQLite.
type SQLiteQASessionRepository struct {
	db *sql.DB
}

// NewSQLiteQASessionRepository constructs a SQLite-backed session repository.
func NewSQLiteQASessionRepository(db *sql.DB) *SQLiteQASessionRepository {
	return &SQLiteQASessionRepository{db: db}
}

func (r *SQLiteQASessionRepository) Create(ctx context.Context, session domain.QASession) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO upload_qa_sessions (id, user_id, created_at)
		VALUES (?, ?, ?)
	`, session.ID.String(), session.UserID, formatSQLiteTime(session.CreatedAt))
	return err
}

func (r *SQLiteQASessionRepository) Find(ctx context.Context, id uuid.UUID, userID int64) (domain.QASession, bool, error) {
	return scanSQLiteSession(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, created_at
		FROM upload_qa_sessions
		WHERE id = ? AND user_id = ?
		LIMIT 1
	`, id.String(), userID))
}

func (r *SQLiteQASessionRepository) List(ctx context.Context, userID int64) ([]domain.QASession, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, created_at
		FROM upload_qa_sessions
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.QASession, 0)
	for rows.Next() {
		session, err := scanSQLiteSessionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

var _ domain.QASessionRepository = (*SQLiteQASessionRepository)(nil)

// SQLiteQueryLogRepository persists question and answer logs in SQLite.
type SQLiteQueryLogRepository struct {
	db *sql.DB
}

// NewSQLiteQueryLogRepository constructs a SQLite-backed query log repository.
func NewSQLiteQueryLogRepository(db *sql.DB) *SQLiteQueryLogRepository {
	return &SQLiteQueryLogRepository{db: db}
}

func (r *SQLiteQueryLogRepository) Append(ctx context.Context, log domain.QueryLog) error {
	sources, err := json.Marshal(log.Sources)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO upload_query_logs (id, session_id, query_text, response_text, latency_ms, sources, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, log.ID.String(), log.SessionID.String(), log.QueryText, log.ResponseText, log.LatencyMs, string(sources), formatSQLiteTime(log.CreatedAt))
	return err
}

func (r *SQLiteQueryLogRepository) ListBySession(ctx context.Context, sessionID uuid.UUID, userID int64) ([]domain.QueryLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT q.id, q.session_id, q.query_text, q.response_text, q.latency_ms, q.sources, q.created_at
		FROM upload_query_logs q
		JOIN upload_qa_sessions s ON s.id = q.session_id
		WHERE q.session_id = ? AND s.user_id = ?
		ORDER BY q.created_at DESC
	`, sessionID.String(), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.QueryLog, 0)
	for rows.Next() {
		entry, err := scanSQLiteQueryLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

var _ domain.QueryLogRepository = (*SQLiteQueryLogRepository)(nil)

type sqliteDocumentScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteDocument(row sqliteDocumentScanner) (domain.Document, bool, error) {
	doc, err := scanSQLiteDocumentRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Document{}, false, nil
		}
		return domain.Document{}, false, err
	}
	return doc, true, nil
}

func scanSQLiteDocumentRow(row sqliteDocumentScanner) (domain.Document, error) {
	var (
		doc           domain.Document
		id            string
		source        string
		status        string
		failureReason sql.NullString
		createdAt     string
		updatedAt     string
	)
	if err := row.Scan(&id, &doc.UserID, &doc.Title, &source, &status, &failureReason, &createdAt, &updatedAt); err != nil {
		return domain.Document{}, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return domain.Document{}, err
	}
	created, err := parseSQLiteTime(createdAt)
	if err != nil {
		return domain.Document{}, err
	}
	updated, err := parseSQLiteTime(updatedAt)
	if err != nil {
		return domain.Document{}, err
	}
	doc.ID = parsedID
	doc.Source = domain.DocumentSource(source)
	doc.Status = domain.DocumentStatus(status)
	doc.CreatedAt = created
	doc.UpdatedAt = updated
	if failureReason.Valid {
		reason := failureReason.String
		doc.FailureReason = &reason
	}
	return doc, nil
}

func scanSQLiteRetrievedChunk(row sqliteDocumentScanner) (domain.DocumentChunk, domain.Document, error) {
	var (
		chunk            domain.DocumentChunk
		doc              domain.Document
		chunkID          string
		chunkDocumentID  string
		rawEmbedding     string
		chunkCreatedAt   string
		docID            string
		docSource        string
		docStatus        string
		docFailureReason sql.NullString
		docCreatedAt     string
		docUpdatedAt     string
	)
	if err := row.Scan(
		&chunkID, &chunkDocumentID, &chunk.ChunkIndex, &chunk.Content, &chunk.TokenCount, &rawEmbedding, &chunkCreatedAt,
		&docID, &doc.UserID, &doc.Title, &docSource, &docStatus, &docFailureReason, &docCreatedAt, &docUpdatedAt,
	); err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	parsedChunkID, err := uuid.Parse(chunkID)
	if err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	parsedChunkDocID, err := uuid.Parse(chunkDocumentID)
	if err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	parsedDocID, err := uuid.Parse(docID)
	if err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	chunkCreated, err := parseSQLiteTime(chunkCreatedAt)
	if err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	docCreated, err := parseSQLiteTime(docCreatedAt)
	if err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	docUpdated, err := parseSQLiteTime(docUpdatedAt)
	if err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	var embedding []float32
	if err := json.Unmarshal([]byte(rawEmbedding), &embedding); err != nil {
		return domain.DocumentChunk{}, domain.Document{}, err
	}
	chunk.ID = parsedChunkID
	chunk.DocumentID = parsedChunkDocID
	chunk.Embedding = embedding
	chunk.CreatedAt = chunkCreated
	doc.ID = parsedDocID
	doc.Source = domain.DocumentSource(docSource)
	doc.Status = domain.DocumentStatus(docStatus)
	doc.CreatedAt = docCreated
	doc.UpdatedAt = docUpdated
	if docFailureReason.Valid {
		reason := docFailureReason.String
		doc.FailureReason = &reason
	}
	return chunk, doc, nil
}

type sqliteSessionScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteSession(row sqliteSessionScanner) (domain.QASession, bool, error) {
	session, err := scanSQLiteSessionRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.QASession{}, false, nil
		}
		return domain.QASession{}, false, err
	}
	return session, true, nil
}

func scanSQLiteSessionRow(row sqliteSessionScanner) (domain.QASession, error) {
	var (
		session domain.QASession
		id      string
		created string
	)
	if err := row.Scan(&id, &session.UserID, &created); err != nil {
		return domain.QASession{}, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return domain.QASession{}, err
	}
	createdAt, err := parseSQLiteTime(created)
	if err != nil {
		return domain.QASession{}, err
	}
	session.ID = parsedID
	session.CreatedAt = createdAt
	return session, nil
}

func scanSQLiteQueryLog(row sqliteDocumentScanner) (domain.QueryLog, error) {
	var (
		entry     domain.QueryLog
		id        string
		sessionID string
		sources   string
		created   string
	)
	if err := row.Scan(&id, &sessionID, &entry.QueryText, &entry.ResponseText, &entry.LatencyMs, &sources, &created); err != nil {
		return domain.QueryLog{}, err
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return domain.QueryLog{}, err
	}
	parsedSessionID, err := uuid.Parse(sessionID)
	if err != nil {
		return domain.QueryLog{}, err
	}
	createdAt, err := parseSQLiteTime(created)
	if err != nil {
		return domain.QueryLog{}, err
	}
	if err := json.Unmarshal([]byte(sources), &entry.Sources); err != nil {
		return domain.QueryLog{}, err
	}
	entry.ID = parsedID
	entry.SessionID = parsedSessionID
	entry.CreatedAt = createdAt
	return entry, nil
}

func uuidSet(values []uuid.UUID) map[uuid.UUID]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[uuid.UUID]bool, len(values))
	for _, id := range values {
		out[id] = true
	}
	return out
}

func statusSet(values []domain.DocumentStatus) map[domain.DocumentStatus]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[domain.DocumentStatus]bool, len(values))
	for _, status := range values {
		out[status] = true
	}
	return out
}

func formatSQLiteTime(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseSQLiteTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999-0700",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05-0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("parse sqlite uploadask time %q: %w", value, lastErr)
}
