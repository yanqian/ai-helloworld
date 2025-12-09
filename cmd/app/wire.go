//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/yanqian/ai-helloworld/internal/bootstrap"
	"github.com/yanqian/ai-helloworld/internal/domain/auth"
	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
	"github.com/yanqian/ai-helloworld/internal/infra/uv/datagov"
	httpiface "github.com/yanqian/ai-helloworld/internal/interface/http"
	"github.com/yanqian/ai-helloworld/pkg/logger"
)

func initializeApp() (*bootstrap.App, error) {
	wire.Build(
		config.Load,
		logger.New,
		provideSummaryConfig,
		provideUVAdvisorConfig,
		provideFAQConfig,
		provideAuthConfig,
		provideChatGPTClient,
		provideUVClient,
		provideFAQRepository,
		provideFAQStore,
		provideAuthRepository,
		provideUploadAskConfig,
		provideUploadStorage,
		provideUploadEmbedder,
		provideUploadChunker,
		provideUploadDocumentRepository,
		provideUploadFileRepository,
		provideUploadChunkRepository,
		provideUploadSessionRepository,
		provideUploadQueryLogRepository,
		provideUploadMessageLog,
		provideUploadMemoryStore,
		provideUploadQueue,
		provideUploadLLM,
		provideUploadService,
		summarizer.NewService,
		uvadvisor.NewService,
		faq.NewService,
		auth.NewService,
		wire.Bind(new(summarizer.ChatClient), new(*chatgpt.Client)),
		wire.Bind(new(uvadvisor.ChatClient), new(*chatgpt.Client)),
		wire.Bind(new(uvadvisor.UVClient), new(*datagov.Client)),
		wire.Bind(new(faq.ChatClient), new(*chatgpt.Client)),
		httpiface.NewHandler,
		httpiface.NewRouter,
		bootstrap.NewApp,
	)
	return nil, nil
}
