package embedder

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

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
	var (
		out            [][]float32
		batch          []string
		batchTokens    int
		maxBatchTokens = 200_000 // stay well below provider's 300k cap
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		req := chatgpt.EmbeddingRequest{
			Model: e.model,
			Input: batch,
		}
		resp, err := e.client.CreateEmbedding(ctx, req)
		if err != nil {
			return fmt.Errorf("create embedding: %w", err)
		}
		for _, item := range resp.Data {
			vec := make([]float32, len(item.Embedding))
			copy(vec, item.Embedding)
			out = append(out, vec)
		}
		if len(resp.Data) != len(batch) {
			e.logger.Warn("embedding result count mismatch", "expected", len(batch), "got", len(resp.Data))
		}
		batch = batch[:0]
		batchTokens = 0
		return nil
	}

	for _, text := range texts {
		tokens := estimateTokens(text)
		if tokens > maxBatchTokens {
			return nil, fmt.Errorf("text too large for embedding request: estimated tokens=%d", tokens)
		}
		if batchTokens+tokens > maxBatchTokens && len(batch) > 0 {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		batch = append(batch, text)
		batchTokens += tokens
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

var _ interface {
	Embed(context.Context, []string) ([][]float32, error)
} = (*ChatGPTEmbedder)(nil)

// estimateTokens provides a rough, upper-biased token count without external dependencies.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := utf8.RuneCountInString(text)
	words := len(strings.Fields(text))
	// Over-estimate to stay under provider caps: assume ~1 token per 2 runes and never below word count.
	byRunes := (runes + 1) / 2
	if byRunes < words {
		return words
	}
	return byRunes
}
