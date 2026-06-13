package faqrepo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	sqliteinfra "github.com/yanqian/ai-helloworld/internal/infra/sqlite"
)

func TestSQLiteRepositoryPersistsFAQQuestionsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "faq.db")
	hash := uint64(1<<63 + 37)

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	repo := NewSQLiteRepository(db)
	first, err := repo.InsertQuestion(ctx, "What is local mode?", []float32{0.1, 0.2, 0.3}, &hash)
	require.NoError(t, err)
	second, err := repo.InsertQuestion(ctx, "How does SQLite search work?", []float32{0.9, 0.8, 0.7}, nil)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	db, err = sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()
	reopened := NewSQLiteRepository(db)

	exact, found, err := reopened.FindExact(ctx, "What is local mode?")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ID, exact.ID)
	require.NotNil(t, exact.SemanticHash)
	require.Equal(t, hash, *exact.SemanticHash)

	byHash, found, err := reopened.FindBySemanticHash(ctx, hash)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ID, byHash.ID)

	match, found, err := reopened.FindNearest(ctx, []float32{0.88, 0.79, 0.69})
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, second.ID, match.Question.ID)
	require.Less(t, match.Distance, 0.05)
}
