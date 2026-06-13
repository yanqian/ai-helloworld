package memory

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	sqliteinfra "github.com/yanqian/ai-helloworld/internal/infra/sqlite"
)

func TestSQLiteMessageLogAndMemoryStorePersistAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploadask-memory.db")
	now := time.Date(2026, 6, 13, 11, 0, 0, 0, time.UTC)
	userID := int64(42)
	sessionID := uuid.New()

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	insertSQLiteSession(t, ctx, db, sessionID, userID, now)

	messages := NewSQLiteMessageLog(db)
	memories := NewSQLiteMemoryStore(db)
	require.NoError(t, messages.Append(ctx, domain.ConversationMessage{
		SessionID:  sessionID,
		UserID:     userID,
		Role:       domain.MessageRoleSystem,
		Content:    "older context",
		TokenCount: 10,
		CreatedAt:  now,
	}))
	require.NoError(t, messages.Append(ctx, domain.ConversationMessage{
		SessionID:  sessionID,
		UserID:     userID,
		Role:       domain.MessageRoleUser,
		Content:    "question",
		TokenCount: 3,
		CreatedAt:  now.Add(time.Second),
	}))
	require.NoError(t, messages.Append(ctx, domain.ConversationMessage{
		SessionID:  sessionID,
		UserID:     userID,
		Role:       domain.MessageRoleAssistant,
		Content:    "answer",
		TokenCount: 4,
		CreatedAt:  now.Add(2 * time.Second),
	}))
	require.NoError(t, memories.Upsert(ctx, domain.MemoryRecord{
		SessionID:  sessionID,
		UserID:     userID,
		Source:     domain.MemorySourceQATurn,
		Content:    "prefers local sqlite",
		Embedding:  []float32{1, 0, 0},
		Importance: 2,
		CreatedAt:  now.Add(3 * time.Second),
	}))
	require.NoError(t, memories.Upsert(ctx, domain.MemoryRecord{
		SessionID:  sessionID,
		UserID:     userID,
		Source:     domain.MemorySourceSummary,
		Content:    "less relevant cloud note",
		Embedding:  []float32{0, 1, 0},
		Importance: 1,
		CreatedAt:  now.Add(4 * time.Second),
	}))
	require.NoError(t, db.Close())

	db, err = sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()
	reopenedMessages := NewSQLiteMessageLog(db)
	reopenedMemories := NewSQLiteMemoryStore(db)

	recent, err := reopenedMessages.ListRecent(ctx, userID, sessionID, 7, 10)
	require.NoError(t, err)
	require.Len(t, recent, 2)
	require.Equal(t, domain.MessageRoleUser, recent[0].Role)
	require.Equal(t, domain.MessageRoleAssistant, recent[1].Role)

	found, err := reopenedMemories.Search(ctx, userID, sessionID, []float32{0.95, 0.05, 0}, 1)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, "prefers local sqlite", found[0].Memory.Content)
	require.Greater(t, found[0].Score, 0.9)

	require.NoError(t, reopenedMemories.Upsert(ctx, domain.MemoryRecord{
		SessionID:  sessionID,
		UserID:     userID,
		Source:     domain.MemorySourceManual,
		Content:    "most important",
		Embedding:  []float32{0.8, 0.2, 0},
		Importance: 9,
		CreatedAt:  now.Add(5 * time.Second),
	}))
	require.NoError(t, reopenedMemories.Prune(ctx, userID, &sessionID, 1))

	remaining, err := reopenedMemories.Search(ctx, userID, sessionID, []float32{1, 0, 0}, 10)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, "most important", remaining[0].Memory.Content)
}

func insertSQLiteSession(t *testing.T, ctx context.Context, db *sql.DB, id uuid.UUID, userID int64, createdAt time.Time) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO upload_qa_sessions (id, user_id, created_at)
		VALUES (?, ?, ?)
	`, id.String(), userID, createdAt.UTC().Format(time.RFC3339Nano))
	require.NoError(t, err)
}
