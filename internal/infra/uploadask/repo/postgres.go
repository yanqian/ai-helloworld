package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// PostgresDocumentRepository persists documents in Postgres.
type PostgresDocumentRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresDocumentRepository constructs the repository.
func NewPostgresDocumentRepository(pool *pgxpool.Pool) *PostgresDocumentRepository {
	return &PostgresDocumentRepository{pool: pool}
}

func (r *PostgresDocumentRepository) Create(ctx context.Context, doc domain.Document) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO upload_documents (id, user_id, title, source, status, failure_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, doc.ID, doc.UserID, doc.Title, doc.Source, doc.Status, doc.FailureReason, doc.CreatedAt, doc.UpdatedAt)
	return err
}

func (r *PostgresDocumentRepository) UpdateStatus(ctx context.Context, docID uuid.UUID, status domain.DocumentStatus, failureReason *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE upload_documents
		SET status = $1, failure_reason = $2, updated_at = NOW()
		WHERE id = $3
	`, status, failureReason, docID)
	return err
}

func (r *PostgresDocumentRepository) Get(ctx context.Context, docID uuid.UUID, userID int64) (domain.Document, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, title, source, status, failure_reason, created_at, updated_at
		FROM upload_documents
		WHERE id = $1 AND user_id = $2
		LIMIT 1
	`, docID, userID)
	var doc domain.Document
	var failureReason *string
	if err := row.Scan(&doc.ID, &doc.UserID, &doc.Title, &doc.Source, &doc.Status, &failureReason, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.Document{}, false, nil
		}
		return domain.Document{}, false, err
	}
	doc.FailureReason = failureReason
	return doc, true, nil
}

func (r *PostgresDocumentRepository) List(ctx context.Context, userID int64, filter domain.DocumentFilter) ([]domain.Document, error) {
	query := `
		SELECT id, user_id, title, source, status, failure_reason, created_at, updated_at
		FROM upload_documents
		WHERE user_id = $1
	`
	args := []any{userID}
	argPos := 2
	if len(filter.Statuses) > 0 {
		query += ` AND status = ANY($` + itoa(argPos) + `)`
		args = append(args, filter.Statuses)
		argPos++
	}
	if len(filter.DocumentIDs) > 0 {
		query += ` AND id = ANY($` + itoa(argPos) + `)`
		args = append(args, filter.DocumentIDs)
		argPos++
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []domain.Document
	for rows.Next() {
		var doc domain.Document
		var failureReason *string
		if err := rows.Scan(&doc.ID, &doc.UserID, &doc.Title, &doc.Source, &doc.Status, &failureReason, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		doc.FailureReason = failureReason
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

var _ domain.DocumentRepository = (*PostgresDocumentRepository)(nil)

// PostgresFileRepository persists file metadata.
type PostgresFileRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresFileRepository constructs the repository.
func NewPostgresFileRepository(pool *pgxpool.Pool) *PostgresFileRepository {
	return &PostgresFileRepository{pool: pool}
}

func (r *PostgresFileRepository) Create(ctx context.Context, file domain.FileObject) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO upload_file_objects (id, document_id, storage_key, size_bytes, mime_type, etag, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, file.ID, file.DocumentID, file.StorageKey, file.SizeBytes, file.MimeType, file.ETag, file.CreatedAt)
	return err
}

func (r *PostgresFileRepository) FindByDocument(ctx context.Context, docID uuid.UUID) (domain.FileObject, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, document_id, storage_key, size_bytes, mime_type, etag, created_at
		FROM upload_file_objects
		WHERE document_id = $1
		LIMIT 1
	`, docID)
	var file domain.FileObject
	if err := row.Scan(&file.ID, &file.DocumentID, &file.StorageKey, &file.SizeBytes, &file.MimeType, &file.ETag, &file.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.FileObject{}, false, nil
		}
		return domain.FileObject{}, false, err
	}
	return file, true, nil
}

var _ domain.FileObjectRepository = (*PostgresFileRepository)(nil)

// PostgresChunkRepository stores chunks and supports similarity search via pgvector.
type PostgresChunkRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresChunkRepository constructs the chunk repository.
func NewPostgresChunkRepository(pool *pgxpool.Pool) *PostgresChunkRepository {
	return &PostgresChunkRepository{pool: pool}
}

func (r *PostgresChunkRepository) InsertBatch(ctx context.Context, chunks []domain.DocumentChunk) error {
	batch := &pgx.Batch{}
	for _, chunk := range chunks {
		batch.Queue(`
			INSERT INTO upload_document_chunks (id, document_id, chunk_index, content, token_count, embedding, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, chunk.ID, chunk.DocumentID, chunk.ChunkIndex, chunk.Content, chunk.TokenCount, pgvector.NewVector(chunk.Embedding), chunk.CreatedAt)
	}
	return r.pool.SendBatch(ctx, batch).Close()
}

func (r *PostgresChunkRepository) SearchSimilar(ctx context.Context, userID int64, embedding []float32, filter domain.DocumentFilter) ([]domain.RetrievedChunk, error) {
	query := `
		SELECT
			c.id, c.document_id, c.chunk_index, c.content, c.token_count, c.embedding, c.created_at,
			d.id, d.user_id, d.title, d.source, d.status, d.failure_reason, d.created_at, d.updated_at,
			(1.0 / (1.0 + (c.embedding <-> $1))) AS score
		FROM upload_document_chunks c
		JOIN upload_documents d ON d.id = c.document_id
		WHERE d.user_id = $2
	`
	args := []any{pgvector.NewVector(embedding), userID}
	argPos := 3
	if len(filter.Statuses) > 0 {
		query += ` AND d.status = ANY($` + itoa(argPos) + `)`
		args = append(args, filter.Statuses)
		argPos++
	}
	if len(filter.DocumentIDs) > 0 {
		query += ` AND c.document_id = ANY($` + itoa(argPos) + `)`
		args = append(args, filter.DocumentIDs)
		argPos++
	}
	query += ` ORDER BY (c.embedding <-> $1) ASC LIMIT 64`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.RetrievedChunk
	for rows.Next() {
		var (
			chunk         domain.DocumentChunk
			doc           domain.Document
			failureReason *string
			score         float64
			embeddingRaw  any
		)
		if err := rows.Scan(
			&chunk.ID, &chunk.DocumentID, &chunk.ChunkIndex, &chunk.Content, &chunk.TokenCount, &embeddingRaw, &chunk.CreatedAt,
			&doc.ID, &doc.UserID, &doc.Title, &doc.Source, &doc.Status, &failureReason, &doc.CreatedAt, &doc.UpdatedAt,
			&score,
		); err != nil {
			return nil, err
		}
		parsed, err := normalizeEmbedding(embeddingRaw)
		if err != nil {
			return nil, err
		}
		chunk.Embedding = parsed
		doc.FailureReason = failureReason
		results = append(results, domain.RetrievedChunk{
			Chunk:     chunk,
			Document:  doc,
			Score:     score,
			CreatedAt: chunk.CreatedAt,
		})
	}
	return results, rows.Err()
}

var _ domain.ChunkRepository = (*PostgresChunkRepository)(nil)

// PostgresQASessionRepository stores sessions.
type PostgresQASessionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresQASessionRepository constructs the repository.
func NewPostgresQASessionRepository(pool *pgxpool.Pool) *PostgresQASessionRepository {
	return &PostgresQASessionRepository{pool: pool}
}

func (r *PostgresQASessionRepository) Create(ctx context.Context, session domain.QASession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO upload_qa_sessions (id, user_id, created_at)
		VALUES ($1, $2, $3)
	`, session.ID, session.UserID, session.CreatedAt)
	return err
}

func (r *PostgresQASessionRepository) Find(ctx context.Context, id uuid.UUID, userID int64) (domain.QASession, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, created_at
		FROM upload_qa_sessions
		WHERE id = $1 AND user_id = $2
		LIMIT 1
	`, id, userID)
	var session domain.QASession
	if err := row.Scan(&session.ID, &session.UserID, &session.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.QASession{}, false, nil
		}
		return domain.QASession{}, false, err
	}
	return session, true, nil
}

func (r *PostgresQASessionRepository) List(ctx context.Context, userID int64) ([]domain.QASession, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, created_at
		FROM upload_qa_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []domain.QASession
	for rows.Next() {
		var session domain.QASession
		if err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

var _ domain.QASessionRepository = (*PostgresQASessionRepository)(nil)

// PostgresQueryLogRepository stores query logs.
type PostgresQueryLogRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresQueryLogRepository constructs the repository.
func NewPostgresQueryLogRepository(pool *pgxpool.Pool) *PostgresQueryLogRepository {
	return &PostgresQueryLogRepository{pool: pool}
}

func (r *PostgresQueryLogRepository) Append(ctx context.Context, log domain.QueryLog) error {
	sources, err := json.Marshal(log.Sources)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO upload_query_logs (id, session_id, query_text, response_text, latency_ms, sources, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, log.ID, log.SessionID, log.QueryText, log.ResponseText, log.LatencyMs, sources, log.CreatedAt)
	return err
}

func (r *PostgresQueryLogRepository) ListBySession(ctx context.Context, sessionID uuid.UUID, userID int64) ([]domain.QueryLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT q.id, q.session_id, q.query_text, q.response_text, q.latency_ms, q.sources, q.created_at
		FROM upload_query_logs q
		JOIN upload_qa_sessions s ON s.id = q.session_id
		WHERE q.session_id = $1 AND s.user_id = $2
		ORDER BY q.created_at DESC
	`, sessionID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.QueryLog
	for rows.Next() {
		var (
			entry   domain.QueryLog
			rawJSON []byte
		)
		if err := rows.Scan(&entry.ID, &entry.SessionID, &entry.QueryText, &entry.ResponseText, &entry.LatencyMs, &rawJSON, &entry.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawJSON, &entry.Sources)
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}

var _ domain.QueryLogRepository = (*PostgresQueryLogRepository)(nil)

func itoa(v int) string {
	return strconv.Itoa(v)
}

func normalizeEmbedding(raw any) ([]float32, error) {
	switch v := raw.(type) {
	case pgvector.Vector:
		return append([]float32(nil), v.Slice()...), nil
	case []float32:
		return append([]float32(nil), v...), nil
	case []float64:
		out := make([]float32, len(v))
		for i, f := range v {
			out[i] = float32(f)
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(v)
		trimmed = strings.TrimPrefix(trimmed, "[")
		trimmed = strings.TrimSuffix(trimmed, "]")
		if trimmed == "" {
			return nil, nil
		}
		parts := strings.Split(trimmed, ",")
		out := make([]float32, 0, len(parts))
		for _, p := range parts {
			numStr := strings.TrimSpace(p)
			if numStr == "" {
				continue
			}
			f, err := strconv.ParseFloat(numStr, 32)
			if err != nil {
				return nil, err
			}
			out = append(out, float32(f))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported embedding type %T", raw)
	}
}
