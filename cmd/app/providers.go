package main

import (
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
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
