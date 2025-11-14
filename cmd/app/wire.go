//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/yanqian/ai-helloworld/internal/bootstrap"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	httpiface "github.com/yanqian/ai-helloworld/internal/interface/http"
	"github.com/yanqian/ai-helloworld/pkg/logger"
)

func initializeApp() (*bootstrap.App, error) {
	wire.Build(
		config.Load,
		logger.New,
		provideSummaryConfig,
		provideUVAdvisorConfig,
		provideChatGPTClient,
		provideUVClient,
		summarizer.NewService,
		uvadvisor.NewService,
		httpiface.NewHandler,
		httpiface.NewRouter,
		bootstrap.NewApp,
	)
	return nil, nil
}
