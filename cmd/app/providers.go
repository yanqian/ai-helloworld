package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	"github.com/yanqian/ai-helloworld/internal/infra/faqrepo"
	"github.com/yanqian/ai-helloworld/internal/infra/faqstore"
	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
	uploadchunker "github.com/yanqian/ai-helloworld/internal/infra/uploadask/chunker"
	uploadembedder "github.com/yanqian/ai-helloworld/internal/infra/uploadask/embedder"
	uploadllm "github.com/yanqian/ai-helloworld/internal/infra/uploadask/llm"
	uploadqueue "github.com/yanqian/ai-helloworld/internal/infra/uploadask/queue"
	uploadrepo "github.com/yanqian/ai-helloworld/internal/infra/uploadask/repo"
	uploadstorage "github.com/yanqian/ai-helloworld/internal/infra/uploadask/storage"
	"github.com/yanqian/ai-helloworld/internal/infra/userrepo"
	"github.com/yanqian/ai-helloworld/internal/infra/uv/datagov"
)

func provideSummaryConfig(cfg *config.Config) summarizer.Config {
	return summarizer.Config{
		MaxSummaryLen: cfg.Summary.MaxSummaryLen,
		MaxKeywords:   cfg.Summary.MaxKeywords,
		DefaultPrompt: cfg.Summary.DefaultPrompt,
		Model:         cfg.LLM.Model,
		Temperature:   cfg.LLM.Temperature,
	}
}

func provideChatGPTClient(cfg *config.Config) (*chatgpt.Client, error) {
	return chatgpt.NewClient(cfg.LLM.APIKey, cfg.LLM.BaseURL)
}

func provideUVAdvisorConfig(cfg *config.Config) uvadvisor.Config {
	return uvadvisor.Config{
		Model:       cfg.LLM.Model,
		Temperature: cfg.LLM.Temperature,
		Prompt:      cfg.UVAdvisor.Prompt,
		SourceURL:   cfg.UVAdvisor.APIBaseURL,
	}
}

func provideUVClient(cfg *config.Config) *datagov.Client {
	return datagov.NewClient(cfg.UVAdvisor.APIBaseURL)
}

func provideAuthConfig(cfg *config.Config) auth.Config {
	return auth.Config{
		Secret:          cfg.Auth.JWTSecret,
		TokenTTL:        cfg.Auth.AccessTokenTTL,
		RefreshTokenTTL: cfg.Auth.RefreshTokenTTL,
	}
}

func provideFAQConfig(cfg *config.Config) faq.Config {
	return faq.Config{
		Model:               cfg.LLM.Model,
		EmbeddingModel:      cfg.LLM.EmbeddingModel,
		Temperature:         cfg.LLM.Temperature,
		Prompt:              cfg.FAQ.Prompt,
		CacheTTL:            cfg.FAQ.CacheTTL,
		TopRecommendations:  cfg.FAQ.TopRecommendations,
		SimilarityThreshold: cfg.FAQ.SimilarityThreshold,
	}
}

func provideFAQRepository(cfg *config.Config, logger *slog.Logger) faq.QuestionRepository {
	fallback := faqrepo.NewMemoryRepository()
	dsn := strings.TrimSpace(cfg.FAQ.Postgres.DSN)
	if dsn == "" {
		logger.Info("faq postgres dsn not set, using memory repository")
		return fallback
	}
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Error("invalid postgres dsn, using memory repository", "error", err)
		return fallback
	}
	if cfg.FAQ.Postgres.MaxConns > 0 {
		poolConfig.MaxConns = cfg.FAQ.Postgres.MaxConns
	}
	if cfg.FAQ.Postgres.MinConns > 0 {
		poolConfig.MinConns = cfg.FAQ.Postgres.MinConns
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		logger.Error("failed to initialize postgres pool, using memory repository", "error", err)
		return fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("postgres ping failed, using memory repository", "error", err)
		pool.Close()
		return fallback
	}
	logger.Info("faq postgres repository enabled")
	return faqrepo.NewPostgresRepository(pool)
}

func provideFAQStore(cfg *config.Config, logger *slog.Logger) faq.Store {
	if cfg.FAQ.Redis.Enabled {
		opt, err := buildValkeyOptions(cfg.FAQ.Redis.Addr)
		if err != nil {
			logger.Error("invalid valkey configuration, falling back to memory store", "error", err)
			return faqstore.NewMemoryStore()
		}
		client, err := valkey.NewClient(opt)
		if err != nil {
			logger.Error("failed to create valkey client, falling back to memory store", "error", err)
			return faqstore.NewMemoryStore()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
			logger.Error("valkey ping failed, falling back to memory store", "error", err)
		} else {
			logger.Info("faq valkey store enabled", "addr", cfg.FAQ.Redis.Addr)
			return faqstore.NewValkeyStore(client, "faq")
		}
	}
	return faqstore.NewMemoryStore()
}

func provideAuthRepository(cfg *config.Config, logger *slog.Logger) auth.Repository {
	fallback := userrepo.NewMemoryRepository()
	dsn := strings.TrimSpace(cfg.Auth.Postgres.DSN)
	if dsn == "" {
		logger.Info("auth postgres dsn not set, using memory repository")
		return fallback
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Error("invalid auth postgres dsn, using memory repository", "error", err)
		return fallback
	}
	if cfg.Auth.Postgres.MaxConns > 0 {
		poolConfig.MaxConns = cfg.Auth.Postgres.MaxConns
	}
	if cfg.Auth.Postgres.MinConns > 0 {
		poolConfig.MinConns = cfg.Auth.Postgres.MinConns
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		logger.Error("failed to initialize auth postgres pool, using memory repository", "error", err)
		return fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("auth postgres ping failed, using memory repository", "error", err)
		pool.Close()
		return fallback
	}
	logger.Info("auth postgres repository enabled")
	return userrepo.NewPostgresRepository(pool)
}

func provideUploadAskConfig(cfg *config.Config) uploadask.Config {
	return uploadask.Config{
		VectorDim:       cfg.UploadAsk.VectorDim,
		MaxFileBytes:    int64(cfg.UploadAsk.MaxFileMB) * 1024 * 1024,
		MaxRetrieved:    8,
		MaxPreviewChars: cfg.UploadAsk.MaxPreviewChars,
	}
}

func provideUploadStorage(cfg *config.Config, logger *slog.Logger) uploadask.ObjectStorage {
	endpoint := strings.TrimSpace(cfg.UploadAsk.Storage.Endpoint)
	accessKey := strings.TrimSpace(cfg.UploadAsk.Storage.AccessKey)
	secretKey := strings.TrimSpace(cfg.UploadAsk.Storage.SecretKey)
	bucket := strings.TrimSpace(cfg.UploadAsk.Storage.Bucket)
	region := strings.TrimSpace(cfg.UploadAsk.Storage.Region)

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		logger.Info("uploadask storage not fully configured, using memory storage")
		return uploadstorage.NewMemoryStorage()
	}
	r2, err := uploadstorage.NewR2Storage(endpoint, accessKey, secretKey, bucket, region, logger)
	if err != nil {
		logger.Error("failed to initialize r2 storage, using memory storage", "error", err)
		return uploadstorage.NewMemoryStorage()
	}
	logger.Info("uploadask r2 storage enabled", "endpoint", endpoint, "bucket", bucket)
	return r2
}

func provideUploadEmbedder(client *chatgpt.Client, cfg *config.Config, logger *slog.Logger) uploadask.Embedder {
	model := strings.TrimSpace(cfg.LLM.EmbeddingModel)
	if client != nil && model != "" {
		return uploadembedder.NewChatGPTEmbedder(client, model, logger)
	}
	logger.Warn("embedding client unavailable, using deterministic embedder")
	return uploadembedder.NewDeterministicEmbedder(cfg.UploadAsk.VectorDim)
}

func provideUploadChunker() uploadask.Chunker {
	return uploadchunker.NewSimpleChunker(800, 80)
}

func provideUploadDocumentRepository(cfg *config.Config, logger *slog.Logger) uploadask.DocumentRepository {
	pool := uploadPostgresPool(cfg, logger)
	if pool != nil {
		return uploadrepo.NewPostgresDocumentRepository(pool)
	}
	logger.Warn("uploadask document repository falling back to memory")
	return uploadrepo.NewMemoryDocumentRepository()
}

func provideUploadFileRepository(cfg *config.Config, logger *slog.Logger) uploadask.FileObjectRepository {
	pool := uploadPostgresPool(cfg, logger)
	if pool != nil {
		return uploadrepo.NewPostgresFileRepository(pool)
	}
	logger.Warn("uploadask file repository falling back to memory")
	return uploadrepo.NewMemoryFileRepository()
}

func provideUploadChunkRepository(cfg *config.Config, docRepo uploadask.DocumentRepository, logger *slog.Logger) uploadask.ChunkRepository {
	pool := uploadPostgresPool(cfg, logger)
	if pool != nil {
		return uploadrepo.NewPostgresChunkRepository(pool)
	}
	logger.Warn("uploadask chunk repository falling back to memory")
	return uploadrepo.NewMemoryChunkRepository(docRepo)
}

func provideUploadSessionRepository(cfg *config.Config, logger *slog.Logger) uploadask.QASessionRepository {
	pool := uploadPostgresPool(cfg, logger)
	if pool != nil {
		return uploadrepo.NewPostgresQASessionRepository(pool)
	}
	logger.Warn("uploadask session repository falling back to memory")
	return uploadrepo.NewMemoryQASessionRepository()
}

func provideUploadQueryLogRepository(cfg *config.Config, logger *slog.Logger) uploadask.QueryLogRepository {
	pool := uploadPostgresPool(cfg, logger)
	if pool != nil {
		return uploadrepo.NewPostgresQueryLogRepository(pool)
	}
	logger.Warn("uploadask query log repository falling back to memory")
	return uploadrepo.NewMemoryQueryLogRepository()
}

func provideUploadQueue(cfg *config.Config, logger *slog.Logger) uploadqueue.HandlerQueue {
	if cfg.UploadAsk.Redis.Enabled {
		opt, err := buildValkeyOptions(cfg.UploadAsk.Redis.Addr)
		if err != nil {
			logger.Error("invalid uploadask valkey configuration, falling back to in-memory queue", "error", err)
			return uploadqueue.NewImmediateQueue(nil)
		}
		client, err := valkey.NewClient(opt)
		if err != nil {
			logger.Error("failed to create uploadask valkey client, falling back to in-memory queue", "error", err)
			return uploadqueue.NewImmediateQueue(nil)
		}
		logger.Info("uploadask valkey queue enabled", "addr", cfg.UploadAsk.Redis.Addr)
		return uploadqueue.NewValkeyQueue(client, "uploadask:jobs", logger)
	}
	return uploadqueue.NewImmediateQueue(nil)
}

func provideUploadLLM(client *chatgpt.Client, cfg *config.Config, logger *slog.Logger) uploadask.LLM {
	if client == nil {
		logger.Warn("chatgpt client missing, falling back to echo llm")
		return uploadllm.EchoLLM{}
	}
	return uploadllm.NewChatGPTLLM(client, cfg.LLM.Model, cfg.LLM.Temperature)
}

func provideUploadService(appCfg uploadask.Config, docs uploadask.DocumentRepository, files uploadask.FileObjectRepository, chunks uploadask.ChunkRepository, sessions uploadask.QASessionRepository, logs uploadask.QueryLogRepository, storage uploadask.ObjectStorage, embedder uploadask.Embedder, llm uploadask.LLM, chunker uploadask.Chunker, queue uploadqueue.HandlerQueue, logger *slog.Logger) *uploadask.Service {
	svc := uploadask.NewService(appCfg, docs, files, chunks, sessions, logs, storage, embedder, llm, chunker, queue, logger)
	queue.SetHandler(func(ctx context.Context, name string, payload map[string]any) {
		if name != "process_document" {
			return
		}
		rawDocID, ok := payload["document_id"]
		if !ok {
			return
		}
		rawUserID, ok := payload["user_id"]
		if !ok {
			return
		}
		docID, err := parseUUID(rawDocID)
		if err != nil {
			logger.Warn("invalid document id in queue payload", "error", err)
			return
		}
		userID := parseUserID(rawUserID)
		if userID == 0 {
			return
		}
		if err := svc.ProcessDocument(ctx, docID, userID); err != nil {
			logger.Warn("process_document failed", "error", err)
		}
	})
	return svc
}

func buildValkeyOptions(addr string) (valkey.ClientOption, error) {
	var (
		opt valkey.ClientOption
		err error
	)
	addr = strings.TrimSpace(addr)
	if strings.Contains(addr, "://") {
		opt, err = valkey.ParseURL(addr)
	} else {
		opt = valkey.ClientOption{InitAddress: []string{addr}}
	}
	if err != nil {
		return valkey.ClientOption{}, err
	}
	return opt, nil
}

func parseUUID(value any) (uuid.UUID, error) {
	switch v := value.(type) {
	case string:
		return uuid.Parse(strings.TrimSpace(v))
	default:
		return uuid.Parse(strings.TrimSpace(fmt.Sprint(v)))
	}
}

func parseUserID(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed
	default:
		return 0
	}
}

var (
	uploadPoolOnce sync.Once
	uploadPool     *pgxpool.Pool
)

func uploadPostgresPool(cfg *config.Config, logger *slog.Logger) *pgxpool.Pool {
	uploadPoolOnce.Do(func() {
		dsn := strings.TrimSpace(cfg.UploadAsk.Postgres.DSN)
		if dsn == "" {
			logger.Info("uploadask postgres dsn not set, using memory repositories")
			return
		}
		poolConfig, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			logger.Error("invalid uploadask postgres dsn, using memory repositories", "error", err)
			return
		}
		registerPgVector(poolConfig, logger)
		if cfg.UploadAsk.Postgres.MaxConns > 0 {
			poolConfig.MaxConns = cfg.UploadAsk.Postgres.MaxConns
		}
		if cfg.UploadAsk.Postgres.MinConns > 0 {
			poolConfig.MinConns = cfg.UploadAsk.Postgres.MinConns
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err != nil {
			logger.Error("failed to initialize uploadask postgres pool, using memory repositories", "error", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			logger.Error("uploadask postgres ping failed, using memory repositories", "error", err)
			pool.Close()
			return
		}
		logger.Info("uploadask postgres repository enabled")
		uploadPool = pool
	})
	return uploadPool
}

func registerPgVector(poolConfig *pgxpool.Config, logger *slog.Logger) {
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		var oid uint32
		if err := conn.QueryRow(ctx, "SELECT 'vector'::regtype::oid").Scan(&oid); err != nil {
			logger.Error("failed to lookup pgvector oid", "error", err)
			return err
		}
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "vector",
			OID:   oid,
			Codec: pgtype.TextCodec{},
		})
		return nil
	}
}
