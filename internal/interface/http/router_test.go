package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
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

	recorder := performRequest("/api/v1/summaries", `{"text":"hello world"}`, newRouterUnderTest(t, svc, nil, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var got summarizer.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, resp, got)
}

func TestRouter_SummarizeInvalidJSON(t *testing.T) {
	svc := &stubSummarizer{}

	recorder := performRequest("/api/v1/summaries", `{"text":123}`, newRouterUnderTest(t, svc, nil, nil))
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

	recorder := performRequest("/api/v1/summaries", `{"text":""}`, newRouterUnderTest(t, svc, nil, nil))
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

	recorder := performRequest("/api/v1/summaries/stream", `{"text":"stream me"}`, newRouterUnderTest(t, svc, nil, nil))
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

	recorder := performRequest("/api/v1/summaries/stream", `{"text":""}`, newRouterUnderTest(t, svc, nil, nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "summarize_failed", errBody["error"]["code"])
	require.Contains(t, errBody["error"]["message"], "text cannot be empty")
}

func TestRouter_CORSPreflight(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/summaries", nil)
	recorder := httptest.NewRecorder()

	server.Handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "POST, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", recorder.Header().Get("Access-Control-Allow-Headers"))
}

func TestRouter_RetryOnTransientFailure(t *testing.T) {
	var calls int
	svc := &stubSummarizer{
		summarizeFn: func(ctx context.Context, req summarizer.Request) (summarizer.Response, error) {
			calls++
			if calls == 1 {
				return summarizer.Response{}, errors.New("temporary failure")
			}
			return summarizer.Response{Summary: "recovered"}, nil
		},
	}

	server := newRouterUnderTest(t, svc, nil, nil, func(cfg *config.Config) {
		cfg.HTTP.Retry.Enabled = true
		cfg.HTTP.Retry.MaxAttempts = 2
		cfg.HTTP.Retry.BaseBackoff = 0
	})

	recorder := performRequest("/api/v1/summaries", `{"text":"hello"}`, server)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 2, calls)
}

func TestRouter_RateLimitExceeded(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, func(cfg *config.Config) {
		cfg.HTTP.RateLimit.Enabled = true
		cfg.HTTP.RateLimit.RequestsPerMinute = 1
		cfg.HTTP.RateLimit.Burst = 1
	})

	first := performRequest("/api/v1/summaries", `{"text":"hello"}`, server)
	require.Equal(t, http.StatusOK, first.Code)

	second := performRequest("/api/v1/summaries", `{"text":"hello"}`, server)
	require.Equal(t, http.StatusTooManyRequests, second.Code)

	errBody := decodeErrorBody(t, second.Body.Bytes())
	require.Equal(t, "rate_limit_exceeded", errBody["error"]["code"])
}

func TestIPRateLimiterBasic(t *testing.T) {
	limiter := newIPRateLimiter(config.RateLimitConfig{RequestsPerMinute: 1, Burst: 1})
	require.True(t, limiter.allow("ip"))
	require.False(t, limiter.allow("ip"))
}

func TestRateLimitMiddlewareBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(errorHandlingMiddleware(newTestLogger()), rateLimitMiddleware(config.RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 1,
		Burst:             1,
	}, newTestLogger()))
	router.POST("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"text":"a"}`))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"text":"a"}`))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusTooManyRequests, rec2.Code)
}

func performRequest(path, body string, server *http.Server) *httptest.ResponseRecorder {
	return performJSONRequest(http.MethodPost, path, body, server)
}

func performJSONRequest(method, path, body string, server *http.Server) *httptest.ResponseRecorder {
	var payload io.Reader
	if body != "" {
		payload = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, payload)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.RemoteAddr = "203.0.113.1:1234"
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	return rec
}

func TestRouter_UVAdviceSuccess(t *testing.T) {
	advice := uvadvisor.Response{Date: "2024-07-01", Category: "high", Summary: "Hot", Clothing: []string{"Hat"}}
	svc := &stubUVAdvisor{
		recommendFn: func(ctx context.Context, req uvadvisor.Request) (uvadvisor.Response, error) {
			require.Equal(t, "2024-07-01", req.Date)
			return advice, nil
		},
	}

	recorder := performRequest("/api/v1/uv-advice", `{"date":"2024-07-01"}`, newRouterUnderTest(t, &stubSummarizer{}, svc, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp uvadvisor.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, advice.Date, resp.Date)
	require.Equal(t, advice.Summary, resp.Summary)
}

func TestRouter_UVAdviceInvalidJSON(t *testing.T) {
	recorder := performRequest("/api/v1/uv-advice", `{"date":123}`, newRouterUnderTest(t, &stubSummarizer{}, &stubUVAdvisor{}, nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRouter_UVAdviceUpstreamError(t *testing.T) {
	svc := &stubUVAdvisor{
		recommendFn: func(ctx context.Context, req uvadvisor.Request) (uvadvisor.Response, error) {
			return uvadvisor.Response{}, apperrors.Wrap("uv_data_error", "upstream unavailable", nil)
		},
	}
	recorder := performRequest("/api/v1/uv-advice", `{}`, newRouterUnderTest(t, &stubSummarizer{}, svc, nil))
	require.Equal(t, http.StatusBadGateway, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "uv_advice_failed", errBody["error"]["code"])
}

func TestRouter_FAQSearchSuccess(t *testing.T) {
	expected := faq.Response{Answer: "The moon is about 384,400 km away.", Source: "cache"}
	faqSvc := &stubFAQ{
		answerFn: func(ctx context.Context, req faq.Request) (faq.Response, error) {
			require.Equal(t, "How far is the moon?", req.Question)
			return expected, nil
		},
	}
	recorder := performRequest("/api/v1/faq/search", `{"question":"How far is the moon?"}`, newRouterUnderTest(t, &stubSummarizer{}, nil, faqSvc))
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp faq.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, expected.Answer, resp.Answer)
	require.Equal(t, expected.Source, resp.Source)
}

func TestRouter_FAQTrending(t *testing.T) {
	faqSvc := &stubFAQ{
		trendingFn: func(ctx context.Context) ([]faq.TrendingQuery, error) {
			return []faq.TrendingQuery{{Query: "Question", Count: 3}}, nil
		},
	}
	recorder := performJSONRequest(http.MethodGet, "/api/v1/faq/trending", "", newRouterUnderTest(t, &stubSummarizer{}, nil, faqSvc))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Recommendations []faq.TrendingQuery `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Recommendations, 1)
	require.Equal(t, int64(3), body.Recommendations[0].Count)
}

func newRouterUnderTest(t *testing.T, summarySvc summarizer.Service, advisorSvc uvadvisor.Service, faqSvc faq.Service, overrides ...func(*config.Config)) *http.Server {
	t.Helper()
	if advisorSvc == nil {
		advisorSvc = &stubUVAdvisor{}
	}
	if faqSvc == nil {
		faqSvc = &stubFAQ{}
	}
	handler := NewHandler(summarySvc, advisorSvc, faqSvc, newTestLogger())
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Address:      ":0",
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			RateLimit: config.RateLimitConfig{
				Enabled: false,
			},
			Retry: config.RetryConfig{
				Enabled: false,
			},
		},
	}
	for _, override := range overrides {
		override(cfg)
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

type stubUVAdvisor struct {
	recommendFn func(ctx context.Context, req uvadvisor.Request) (uvadvisor.Response, error)
}

func (s *stubUVAdvisor) Recommend(ctx context.Context, req uvadvisor.Request) (uvadvisor.Response, error) {
	if s.recommendFn != nil {
		return s.recommendFn(ctx, req)
	}
	return uvadvisor.Response{}, nil
}

type stubFAQ struct {
	answerFn   func(ctx context.Context, req faq.Request) (faq.Response, error)
	trendingFn func(ctx context.Context) ([]faq.TrendingQuery, error)
}

func (s *stubFAQ) Answer(ctx context.Context, req faq.Request) (faq.Response, error) {
	if s.answerFn != nil {
		return s.answerFn(ctx, req)
	}
	return faq.Response{}, nil
}

func (s *stubFAQ) Trending(ctx context.Context) ([]faq.TrendingQuery, error) {
	if s.trendingFn != nil {
		return s.trendingFn(ctx)
	}
	return nil, nil
}

func decodeErrorBody(t *testing.T, raw []byte) map[string]map[string]string {
	t.Helper()
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}
