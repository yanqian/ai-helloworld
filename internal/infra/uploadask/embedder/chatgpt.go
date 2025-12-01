package embedder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
)

// ChatGPTEmbedder calls OpenAI-compatible embeddings API.
type ChatGPTEmbedder struct {
	client *chatgpt.Client
	model  string
	logger *slog.Logger
}

// NewChatGPTEmbedder constructs an embedder backed by the ChatGPT client.
func NewChatGPTEmbedder(client *chatgpt.Client, model string, logger *slog.Logger) *ChatGPTEmbedder {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChatGPTEmbedder{
		client: client,
		model:  strings.TrimSpace(model),
		logger: logger.With("component", "uploadask.embedder.chatgpt"),
	}
}

// Embed requests embeddings for the given texts.
func (e *ChatGPTEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	req := chatgpt.EmbeddingRequest{
		Model: e.model,
		Input: texts,
	}
	resp, err := e.client.CreateEmbedding(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create embedding: %w", err)
	}
	out := make([][]float32, len(resp.Data))
	for i, item := range resp.Data {
		vec := make([]float32, len(item.Embedding))
		copy(vec, item.Embedding)
		out[i] = vec
	}
	if len(out) != len(texts) {
		e.logger.Warn("embedding result count mismatch", "expected", len(texts), "got", len(out))
	}
	return out, nil
}

var _ interface {
	Embed(context.Context, []string) ([][]float32, error)
} = (*ChatGPTEmbedder)(nil)
