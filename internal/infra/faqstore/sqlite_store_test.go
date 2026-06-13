package faqstore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	sqliteinfra "github.com/yanqian/ai-helloworld/internal/infra/sqlite"
)

func TestSQLiteStorePersistsAnswersAndTrendingAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "faq.db")

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	insertFAQQuestion(t, ctx, db, 42, "What is local mode?")
	store := NewSQLiteStore(db)
	err = store.SaveAnswer(ctx, faq.AnswerRecord{
		QuestionID: 42,
		Question:   "What is local mode?",
		Answer:     "It uses SQLite for local persistence.",
		CreatedAt:  time.Now().UTC(),
	}, time.Hour)
	require.NoError(t, err)
	require.NoError(t, store.IncrementQuery(ctx, "what is local mode", "What is local mode?"))
	require.NoError(t, store.IncrementQuery(ctx, "what is local mode", "What is local mode?"))
	require.NoError(t, store.IncrementQuery(ctx, "sqlite search", "SQLite search"))
	require.NoError(t, db.Close())

	db, err = sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()
	reopened := NewSQLiteStore(db)

	answer, found, err := reopened.GetAnswer(ctx, 42)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "It uses SQLite for local persistence.", answer.Answer)
	require.Equal(t, "What is local mode?", answer.Question)

	trending, err := reopened.TopQueries(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []faq.TrendingQuery{
		{Query: "What is local mode?", Count: 2},
		{Query: "SQLite search", Count: 1},
	}, trending)
}

func TestSQLiteStoreExpiresAnswers(t *testing.T) {
	ctx := context.Background()
	db, err := sqliteinfra.Open(ctx, filepath.Join(t.TempDir(), "faq.db"))
	require.NoError(t, err)
	defer db.Close()
	insertFAQQuestion(t, ctx, db, 7, "Expired?")
	store := NewSQLiteStore(db)

	require.NoError(t, store.SaveAnswer(ctx, faq.AnswerRecord{
		QuestionID: 7,
		Question:   "Expired?",
		Answer:     "yes",
		CreatedAt:  time.Now().UTC(),
	}, time.Nanosecond))
	time.Sleep(time.Millisecond)

	_, found, err := store.GetAnswer(ctx, 7)
	require.NoError(t, err)
	require.False(t, found)
}

func insertFAQQuestion(t *testing.T, ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, id int64, question string) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO faq_questions (id, question_text, embedding, created_at)
		VALUES (?, ?, ?, ?)
	`, id, question, "[]", time.Now().UTC().Format(time.RFC3339Nano))
	require.NoError(t, err)
}
