package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	"github.com/yanqian/ai-helloworld/internal/infra/faqrepo"
	"github.com/yanqian/ai-helloworld/internal/infra/faqstore"
	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
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
		opt, err := buildValkeyOptions(cfg)
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

func buildValkeyOptions(cfg *config.Config) (valkey.ClientOption, error) {
	var (
		opt valkey.ClientOption
		err error
	)
	if strings.Contains(cfg.FAQ.Redis.Addr, "://") {
		opt, err = valkey.ParseURL(cfg.FAQ.Redis.Addr)
	} else {
		opt = valkey.ClientOption{InitAddress: []string{cfg.FAQ.Redis.Addr}}
	}
	if err != nil {
		return valkey.ClientOption{}, err
	}
	return opt, nil
}
