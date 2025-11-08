//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/yanqian/ai-helloworld/internal/bootstrap"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	httpiface "github.com/yanqian/ai-helloworld/internal/interface/http"
	"github.com/yanqian/ai-helloworld/pkg/logger"
)

func initializeApp() (*bootstrap.App, error) {
	wire.Build(
		config.Load,
		logger.New,
		provideSummaryConfig,
		provideChatGPTClient,
		summarizer.NewService,
		httpiface.NewSummaryHandler,
		httpiface.NewRouter,
		bootstrap.NewApp,
	)
	return nil, nil
}
