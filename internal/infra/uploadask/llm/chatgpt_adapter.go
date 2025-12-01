package llm

import (
	"context"
	"strings"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
)

// ChatGPTLLM adapts the existing ChatGPT client to the upload-ask domain.
type ChatGPTLLM struct {
	client      *chatgpt.Client
	model       string
	temperature float32
}

// NewChatGPTLLM constructs the adapter.
func NewChatGPTLLM(client *chatgpt.Client, model string, temperature float32) *ChatGPTLLM {
	return &ChatGPTLLM{client: client, model: model, temperature: temperature}
}

// Chat sends a chat completion request.
func (l *ChatGPTLLM) Chat(ctx context.Context, messages []domain.LLMMessage) (string, error) {
	req := chatgpt.ChatCompletionRequest{
		Model:       l.model,
		Temperature: l.temperature,
		Messages:    make([]chatgpt.Message, 0, len(messages)),
	}
	for _, msg := range messages {
		req.Messages = append(req.Messages, chatgpt.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	resp, err := l.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

var _ domain.LLM = (*ChatGPTLLM)(nil)

// EchoLLM returns a lightweight fallback without external calls.
type EchoLLM struct{}

// Chat returns a simple response that echoes the question.
func (EchoLLM) Chat(_ context.Context, messages []domain.LLMMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}
	return "Answer: " + messages[len(messages)-1].Content, nil
}

var _ domain.LLM = (*EchoLLM)(nil)
