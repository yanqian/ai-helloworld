package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

func TestRouter_SummarizeSuccess(t *testing.T) {
	resp := summarizer.Response{Summary: "short summary", Keywords: []string{"go", "backend"}}
	svc := &stubSummarizer{
		summarizeFn: func(ctx context.Context, req summarizer.Request) (summarizer.Response, error) {
			require.Equal(t, "hello world", req.Text)
			return resp, nil
		},
	}

	recorder := performRequest("/api/v1/summaries", `{"text":"hello world"}`, newRouterUnderTest(t, svc))
	require.Equal(t, http.StatusOK, recorder.Code)

	var got summarizer.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, resp, got)
}

func TestRouter_SummarizeInvalidJSON(t *testing.T) {
	svc := &stubSummarizer{}

	recorder := performRequest("/api/v1/summaries", `{"text":123}`, newRouterUnderTest(t, svc))
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "invalid_request", errBody["error"]["code"])
	require.NotEmpty(t, errBody["error"]["message"])
}

func TestRouter_SummarizeInvalidInput(t *testing.T) {
	svc := &stubSummarizer{
		summarizeFn: func(ctx context.Context, req summarizer.Request) (summarizer.Response, error) {
			return summarizer.Response{}, apperrors.Wrap("invalid_input", "text cannot be empty", nil)
		},
	}

	recorder := performRequest("/api/v1/summaries", `{"text":""}`, newRouterUnderTest(t, svc))
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "summarize_failed", errBody["error"]["code"])
	require.Contains(t, errBody["error"]["message"], "text cannot be empty")
}

func TestRouter_SummarizeStreamSuccess(t *testing.T) {
	chunks := []summarizer.StreamChunk{
		{PartialSummary: "first"},
		{PartialSummary: "second", Completed: true, Keywords: []string{"go"}},
	}
	svc := &stubSummarizer{
		streamSummaryFn: func(ctx context.Context, req summarizer.Request) (<-chan summarizer.StreamChunk, error) {
			require.Equal(t, "stream me", req.Text)
			stream := make(chan summarizer.StreamChunk, len(chunks))
			go func() {
				defer close(stream)
				for _, chunk := range chunks {
					stream <- chunk
				}
			}()
			return stream, nil
		},
	}

	recorder := performRequest("/api/v1/summaries/stream", `{"text":"stream me"}`, newRouterUnderTest(t, svc))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))

	payload := strings.TrimSpace(recorder.Body.String())
	frames := strings.Split(payload, "\n\n")
	require.Len(t, frames, len(chunks))

	for i, frame := range frames {
		require.True(t, strings.HasPrefix(frame, "data: "))
		encoded := strings.TrimPrefix(frame, "data: ")
		var got summarizer.StreamChunk
		require.NoError(t, json.Unmarshal([]byte(encoded), &got))
		require.Equal(t, chunks[i], got)
	}
}

func TestRouter_SummarizeStreamInvalidInput(t *testing.T) {
	svc := &stubSummarizer{
		streamSummaryFn: func(ctx context.Context, req summarizer.Request) (<-chan summarizer.StreamChunk, error) {
			return nil, apperrors.Wrap("invalid_input", "text cannot be empty", nil)
		},
	}

	recorder := performRequest("/api/v1/summaries/stream", `{"text":""}`, newRouterUnderTest(t, svc))
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "summarize_failed", errBody["error"]["code"])
	require.Contains(t, errBody["error"]["message"], "text cannot be empty")
}

func performRequest(path, body string, server *http.Server) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	return rec
}

func newRouterUnderTest(t *testing.T, svc summarizer.Service) *http.Server {
	t.Helper()
	handler := NewSummaryHandler(svc, newTestLogger())
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Address:      ":0",
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
	}
	return NewRouter(cfg, handler)
}

func newTestLogger() *slog.Logger {
	handler := slog.NewTextHandler(io.Discard, nil)
	return slog.New(handler)
}

type stubSummarizer struct {
	summarizeFn     func(ctx context.Context, req summarizer.Request) (summarizer.Response, error)
	streamSummaryFn func(ctx context.Context, req summarizer.Request) (<-chan summarizer.StreamChunk, error)
}

func (s *stubSummarizer) Summarize(ctx context.Context, req summarizer.Request) (summarizer.Response, error) {
	if s.summarizeFn != nil {
		return s.summarizeFn(ctx, req)
	}
	return summarizer.Response{}, nil
}

func (s *stubSummarizer) StreamSummary(ctx context.Context, req summarizer.Request) (<-chan summarizer.StreamChunk, error) {
	if s.streamSummaryFn != nil {
		return s.streamSummaryFn(ctx, req)
	}
	stream := make(chan summarizer.StreamChunk)
	close(stream)
	return stream, nil
}

func decodeErrorBody(t *testing.T, raw []byte) map[string]map[string]string {
	t.Helper()
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}
