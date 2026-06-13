package repo

import (
	"context"
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
