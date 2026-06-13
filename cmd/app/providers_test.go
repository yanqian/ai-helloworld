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
	uploadmemory "github.com/yanqian/ai-helloworld/internal/infra/uploadask/memory"
	uploadrepo "github.com/yanqian/ai-helloworld/internal/infra/uploadask/repo"
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

func TestProvideUploadAskStorageUsesSQLiteInLocalMode(t *testing.T) {
	resetSQLiteProviderForTest(t)
	cfg := &config.Config{
		SQLite: config.SQLiteConfig{
			Enabled: true,
			Path:    filepath.Join(t.TempDir(), "local.db"),
		},
		UploadAsk: config.UploadAskConfig{
			Memory: config.UploadAskMemoryConfig{
				Enabled: true,
			},
		},
	}
	logger := slog.Default()

	docs := provideUploadDocumentRepository(cfg, logger)
	files := provideUploadFileRepository(cfg, logger)
	chunks := provideUploadChunkRepository(cfg, docs, logger)
	sessions := provideUploadSessionRepository(cfg, logger)
	logs := provideUploadQueryLogRepository(cfg, logger)
	messages := provideUploadMessageLog(cfg, logger)
	memories := provideUploadMemoryStore(cfg, logger)
	t.Cleanup(func() {
		if sqliteConn != nil {
			require.NoError(t, sqliteConn.Close())
		}
		sqliteConn = nil
		sqliteOnce = sync.Once{}
	})

	require.IsType(t, &uploadrepo.SQLiteDocumentRepository{}, docs)
	require.IsType(t, &uploadrepo.SQLiteFileRepository{}, files)
	require.IsType(t, &uploadrepo.SQLiteChunkRepository{}, chunks)
	require.IsType(t, &uploadrepo.SQLiteQASessionRepository{}, sessions)
	require.IsType(t, &uploadrepo.SQLiteQueryLogRepository{}, logs)
	require.IsType(t, &uploadmemory.SQLiteMessageLog{}, messages)
	require.IsType(t, &uploadmemory.SQLiteMemoryStore{}, memories)
}

func resetSQLiteProviderForTest(t *testing.T) {
	t.Helper()
	if sqliteConn != nil {
		require.NoError(t, sqliteConn.Close())
	}
	sqliteConn = nil
	sqliteOnce = sync.Once{}
}
