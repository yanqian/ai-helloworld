package repo

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	sqliteinfra "github.com/yanqian/ai-helloworld/internal/infra/sqlite"
)

func TestSQLiteUploadAskRepositoriesPersistAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploadask.db")
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	userID := int64(41)
	docID := uuid.New()
	otherDocID := uuid.New()
	sessionID := uuid.New()

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	docs := NewSQLiteDocumentRepository(db)
	files := NewSQLiteFileRepository(db)
	chunks := NewSQLiteChunkRepository(db)
	sessions := NewSQLiteQASessionRepository(db)
	logs := NewSQLiteQueryLogRepository(db)

	require.NoError(t, docs.Create(ctx, domain.Document{
		ID:        docID,
		UserID:    userID,
		Title:     "Local handbook",
		Source:    domain.DocumentSourceUpload,
		Status:    domain.DocumentStatusProcessed,
		CreatedAt: now,
		UpdatedAt: now,
	}))
	require.NoError(t, docs.Create(ctx, domain.Document{
		ID:        otherDocID,
		UserID:    userID,
		Title:     "Pending notes",
		Source:    domain.DocumentSourceUpload,
		Status:    domain.DocumentStatusPending,
		CreatedAt: now.Add(time.Minute),
		UpdatedAt: now.Add(time.Minute),
	}))
	require.NoError(t, files.Create(ctx, domain.FileObject{
		ID:         uuid.New(),
		DocumentID: docID,
		StorageKey: "uploads/local-handbook.txt",
		SizeBytes:  128,
		MimeType:   "text/plain",
		ETag:       "etag-1",
		CreatedAt:  now.Add(time.Second),
	}))
	require.NoError(t, chunks.InsertBatch(ctx, []domain.DocumentChunk{
		{
			ID:         uuid.New(),
			DocumentID: docID,
			ChunkIndex: 0,
			Content:    "SQLite keeps local upload ask data.",
			TokenCount: 7,
			Embedding:  []float32{1, 0, 0},
			CreatedAt:  now.Add(2 * time.Second),
		},
		{
			ID:         uuid.New(),
			DocumentID: otherDocID,
			ChunkIndex: 0,
			Content:    "This pending document should be filtered.",
			TokenCount: 8,
			Embedding:  []float32{0, 1, 0},
			CreatedAt:  now.Add(3 * time.Second),
		},
	}))
	require.NoError(t, sessions.Create(ctx, domain.QASession{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now.Add(4 * time.Second),
	}))
	require.NoError(t, logs.Append(ctx, domain.QueryLog{
		ID:           uuid.New(),
		SessionID:    sessionID,
		QueryText:    "Where is upload ask data stored?",
		ResponseText: "In SQLite for local mode.",
		LatencyMs:    23,
		Sources: []domain.ChunkSource{
			{DocumentID: docID, ChunkIndex: 0, Score: 0.99, Preview: "SQLite keeps local"},
		},
		CreatedAt: now.Add(5 * time.Second),
	}))
	require.NoError(t, db.Close())

	db, err = sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()
	reopenedDocs := NewSQLiteDocumentRepository(db)
	reopenedFiles := NewSQLiteFileRepository(db)
	reopenedChunks := NewSQLiteChunkRepository(db)
	reopenedSessions := NewSQLiteQASessionRepository(db)
	reopenedLogs := NewSQLiteQueryLogRepository(db)

	doc, found, err := reopenedDocs.Get(ctx, docID, userID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "Local handbook", doc.Title)

	listed, err := reopenedDocs.List(ctx, userID, domain.DocumentFilter{
		Statuses: []domain.DocumentStatus{domain.DocumentStatusProcessed},
	})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, docID, listed[0].ID)

	file, found, err := reopenedFiles.FindByDocument(ctx, docID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "uploads/local-handbook.txt", file.StorageKey)

	results, err := reopenedChunks.SearchSimilar(ctx, userID, []float32{0.9, 0.1, 0}, domain.DocumentFilter{
		Statuses: []domain.DocumentStatus{domain.DocumentStatusProcessed},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, docID, results[0].Document.ID)
	require.Equal(t, 0, results[0].Chunk.ChunkIndex)
	require.Greater(t, results[0].Score, 0.9)

	session, found, err := reopenedSessions.Find(ctx, sessionID, userID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, sessionID, session.ID)

	userSessions, err := reopenedSessions.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, userSessions, 1)

	entries, err := reopenedLogs.ListBySession(ctx, sessionID, userID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Where is upload ask data stored?", entries[0].QueryText)
	require.Equal(t, docID, entries[0].Sources[0].DocumentID)
}

func TestSQLiteUploadAskRepositoriesParseDatabaseStyleTimestamps(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploadask.db")
	userID := int64(51)
	docID := uuid.New()
	fileID := uuid.New()
	chunkID := uuid.New()
	sessionID := uuid.New()
	queryID := uuid.New()
	legacyTime := "2025-12-05 15:06:46.339153+00"

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO upload_documents (id, user_id, title, source, status, failure_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, docID.String(), userID, "Legacy timestamps", string(domain.DocumentSourceUpload), string(domain.DocumentStatusProcessed), nil, legacyTime, legacyTime)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO upload_file_objects (id, document_id, storage_key, size_bytes, mime_type, etag, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, fileID.String(), docID.String(), "uploads/legacy.txt", 32, "text/plain", "etag", legacyTime)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO upload_document_chunks (id, document_id, chunk_index, content, token_count, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, chunkID.String(), docID.String(), 0, "legacy timestamp chunk", 3, `[1,0,0]`, legacyTime)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO upload_qa_sessions (id, user_id, created_at)
		VALUES (?, ?, ?)
	`, sessionID.String(), userID, legacyTime)
	require.NoError(t, err)
	sources, err := json.Marshal([]domain.ChunkSource{
		{DocumentID: docID, ChunkIndex: 0, Score: 0.98, Preview: "legacy timestamp"},
	})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO upload_query_logs (id, session_id, query_text, response_text, latency_ms, sources, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, queryID.String(), sessionID.String(), "What changed?", "Timestamp parsing.", 12, string(sources), legacyTime)
	require.NoError(t, err)

	docs := NewSQLiteDocumentRepository(db)
	files := NewSQLiteFileRepository(db)
	chunks := NewSQLiteChunkRepository(db)
	sessions := NewSQLiteQASessionRepository(db)
	logs := NewSQLiteQueryLogRepository(db)

	doc, found, err := docs.Get(ctx, docID, userID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, time.Date(2025, 12, 5, 15, 6, 46, 339153000, time.UTC), doc.CreatedAt)
	require.Equal(t, doc.CreatedAt, doc.UpdatedAt)

	listed, err := docs.List(ctx, userID, domain.DocumentFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)

	file, found, err := files.FindByDocument(ctx, docID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, fileID, file.ID)
	require.Equal(t, doc.CreatedAt, file.CreatedAt)

	foundChunks, err := chunks.SearchSimilar(ctx, userID, []float32{1, 0, 0}, domain.DocumentFilter{})
	require.NoError(t, err)
	require.Len(t, foundChunks, 1)
	require.Equal(t, chunkID, foundChunks[0].Chunk.ID)
	require.Equal(t, doc.CreatedAt, foundChunks[0].Chunk.CreatedAt)
	require.Equal(t, doc.CreatedAt, foundChunks[0].Document.CreatedAt)

	session, found, err := sessions.Find(ctx, sessionID, userID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, doc.CreatedAt, session.CreatedAt)

	sessionList, err := sessions.List(ctx, userID)
	require.NoError(t, err)
	require.Len(t, sessionList, 1)

	entries, err := logs.ListBySession(ctx, sessionID, userID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, queryID, entries[0].ID)
	require.Equal(t, doc.CreatedAt, entries[0].CreatedAt)
}

func TestSQLiteUploadAskRepositoryRejectsInvalidTimestamp(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploadask.db")
	userID := int64(52)
	docID := uuid.New()

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO upload_documents (id, user_id, title, source, status, failure_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, docID.String(), userID, "Bad timestamp", string(domain.DocumentSourceUpload), string(domain.DocumentStatusProcessed), nil, "not-a-time", "not-a-time")
	require.NoError(t, err)

	docs := NewSQLiteDocumentRepository(db)
	_, found, err := docs.Get(ctx, docID, userID)
	require.Error(t, err)
	require.False(t, found)
	require.Contains(t, err.Error(), "parse sqlite uploadask time")
}
