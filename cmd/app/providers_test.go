package main

import (
	"log/slog"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/infra/config"
	"github.com/yanqian/ai-helloworld/internal/infra/faqrepo"
	"github.com/yanqian/ai-helloworld/internal/infra/faqstore"
)

func TestProvideFAQStorageUsesSQLiteInLocalMode(t *testing.T) {
	resetSQLiteProviderForTest(t)
	cfg := &config.Config{
		SQLite: config.SQLiteConfig{
			Enabled: true,
			Path:    filepath.Join(t.TempDir(), "local.db"),
		},
	}
	logger := slog.Default()

	repo := provideFAQRepository(cfg, logger)
	store := provideFAQStore(cfg, logger)
	t.Cleanup(func() {
		if sqliteConn != nil {
			require.NoError(t, sqliteConn.Close())
		}
		sqliteConn = nil
		sqliteOnce = sync.Once{}
	})

	require.IsType(t, &faqrepo.SQLiteRepository{}, repo)
	require.IsType(t, &faqstore.SQLiteStore{}, store)
}

func resetSQLiteProviderForTest(t *testing.T) {
	t.Helper()
	if sqliteConn != nil {
		require.NoError(t, sqliteConn.Close())
	}
	sqliteConn = nil
	sqliteOnce = sync.Once{}
}
