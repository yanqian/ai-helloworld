package unit

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
)

func TestSummarizeUsesChatGPT(t *testing.T) {
	client := &stubChatClient{
		completionResp: chatgpt.ChatCompletionResponse{
			Choices: []struct {
				Message chatgpt.Message `json:"message"`
			}{
				{Message: chatgpt.Message{Content: "SUMMARY:\nConcise summary.\n\nKEYWORDS:\nalpha, beta"}},
			},
		},
	}

	svc := summarizer.NewService(testConfig(), client, newTestLogger())

	resp, err := svc.Summarize(context.Background(), summarizer.Request{Text: "Go makes backend services easier."})
	require.NoError(t, err)
	require.Equal(t, "Concise summary.", resp.Summary)
	require.Equal(t, []string{"alpha", "beta"}, resp.Keywords)

	require.NotEmpty(t, client.lastRequest.Messages)
	require.Contains(t, client.lastRequest.Messages[0].Content, "You are an expert")
}

func TestSummarizeRejectsEmptyText(t *testing.T) {
	client := &stubChatClient{}
	svc := summarizer.NewService(testConfig(), client, newTestLogger())

	_, err := svc.Summarize(context.Background(), summarizer.Request{Text: "   "})
	require.Error(t, err)
}

func TestSummarizeUsesCustomPromptWhenProvided(t *testing.T) {
	custom := "Custom prompt"
	client := &stubChatClient{
		completionResp: chatgpt.ChatCompletionResponse{
			Choices: []struct {
				Message chatgpt.Message `json:"message"`
			}{
				{Message: chatgpt.Message{Content: "SUMMARY:\nDone.\n\nKEYWORDS:\none"}},
			},
		},
	}
	svc := summarizer.NewService(testConfig(), client, newTestLogger())

	_, err := svc.Summarize(context.Background(), summarizer.Request{Text: "content", Prompt: custom})
	require.NoError(t, err)
	require.Equal(t, custom, client.lastRequest.Messages[0].Content)
}

func TestStreamSummaryReturnsChunks(t *testing.T) {
	client := &stubChatClient{
		streamChunks: []chatgpt.ChatCompletionStreamChunk{
			{
				Choices: []struct {
					Delta        chatgpt.Message `json:"delta"`
					FinishReason string          `json:"finish_reason"`
				}{
					{Delta: chatgpt.Message{Content: "SUMMARY:\nFirst"}},
				},
			},
			{
				Choices: []struct {
					Delta        chatgpt.Message `json:"delta"`
					FinishReason string          `json:"finish_reason"`
				}{
					{Delta: chatgpt.Message{Content: " sentence.\n\nKEYWORDS:\nalpha, beta"}},
				},
			},
		},
	}

	svc := summarizer.NewService(testConfig(), client, newTestLogger())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := svc.StreamSummary(ctx, summarizer.Request{Text: "Any"})
	require.NoError(t, err)

	var got []summarizer.StreamChunk
	for chunk := range stream {
		got = append(got, chunk)
	}

	require.NotEmpty(t, got)
	require.True(t, got[len(got)-1].Completed)
	require.Equal(t, []string{"alpha", "beta"}, got[len(got)-1].Keywords)
}

func testConfig() summarizer.Config {
	return summarizer.Config{
		MaxSummaryLen: 120,
		MaxKeywords:   3,
		DefaultPrompt: "You are an expert summarizer.",
		Model:         "test-model",
		Temperature:   0.1,
	}
}

func newTestLogger() *slog.Logger {
	handler := slog.NewTextHandler(io.Discard, nil)
	return slog.New(handler)
}

type stubChatClient struct {
	completionResp chatgpt.ChatCompletionResponse
	completionErr  error

	streamChunks []chatgpt.ChatCompletionStreamChunk
	streamErr    error

	lastRequest chatgpt.ChatCompletionRequest
}

func (s *stubChatClient) CreateChatCompletion(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.ChatCompletionResponse, error) {
	s.lastRequest = req
	if s.completionErr != nil {
		return chatgpt.ChatCompletionResponse{}, s.completionErr
	}
	return s.completionResp, nil
}

func (s *stubChatClient) CreateChatCompletionStream(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.Stream, error) {
	s.lastRequest = req
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	return &stubChatStream{chunks: s.streamChunks}, nil
}

type stubChatStream struct {
	chunks []chatgpt.ChatCompletionStreamChunk
	idx    int
	closed bool
}

func (s *stubChatStream) Recv() (chatgpt.ChatCompletionStreamChunk, error) {
	if s.idx >= len(s.chunks) {
		return chatgpt.ChatCompletionStreamChunk{}, io.EOF
	}
	chunk := s.chunks[s.idx]
	s.idx++
	return chunk, nil
}

func (s *stubChatStream) Close() error {
	s.closed = true
	return nil
}
