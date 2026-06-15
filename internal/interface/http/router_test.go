package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	uploadask "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	"github.com/yanqian/ai-helloworld/internal/infra/config"
	uploadchunker "github.com/yanqian/ai-helloworld/internal/infra/uploadask/chunker"
	uploadembedder "github.com/yanqian/ai-helloworld/internal/infra/uploadask/embedder"
	uploadllm "github.com/yanqian/ai-helloworld/internal/infra/uploadask/llm"
	uploadmemory "github.com/yanqian/ai-helloworld/internal/infra/uploadask/memory"
	uploadqueue "github.com/yanqian/ai-helloworld/internal/infra/uploadask/queue"
	uploadrepo "github.com/yanqian/ai-helloworld/internal/infra/uploadask/repo"
	uploadstorage "github.com/yanqian/ai-helloworld/internal/infra/uploadask/storage"
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

	recorder := performRequest("/api/v1/summaries", `{"text":"hello world"}`, newRouterUnderTest(t, svc, nil, nil, nil, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var got summarizer.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, resp, got)
}

func TestRouter_SummarizeInvalidJSON(t *testing.T) {
	svc := &stubSummarizer{}

	recorder := performRequest("/api/v1/summaries", `{"text":123}`, newRouterUnderTest(t, svc, nil, nil, nil, nil))
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

	recorder := performRequest("/api/v1/summaries", `{"text":""}`, newRouterUnderTest(t, svc, nil, nil, nil, nil))
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

	recorder := performRequest("/api/v1/summaries/stream", `{"text":"stream me"}`, newRouterUnderTest(t, svc, nil, nil, nil, nil))
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

	recorder := performRequest("/api/v1/summaries/stream", `{"text":""}`, newRouterUnderTest(t, svc, nil, nil, nil, nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "summarize_failed", errBody["error"]["code"])
	require.Contains(t, errBody["error"]["message"], "text cannot be empty")
}

func TestRouter_CORSPreflight(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/summaries", nil)
	recorder := httptest.NewRecorder()

	server.Handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, POST, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type, Authorization", recorder.Header().Get("Access-Control-Allow-Headers"))
}

func TestRouter_RegisterSuccess(t *testing.T) {
	authSvc := &stubAuth{
		registerFn: func(ctx context.Context, req auth.RegisterRequest) (auth.UserView, error) {
			require.Equal(t, "user@example.com", req.Email)
			require.Equal(t, "password123", req.Password)
			require.Equal(t, "Nickname", req.Nickname)
			return auth.UserView{ID: 42, Email: req.Email, Nickname: req.Nickname}, nil
		},
	}
	recorder := performRequest("/api/v1/auth/register", `{"email":"user@example.com","password":"password123","nickname":"Nickname"}`, newRouterUnderTest(t, &stubSummarizer{}, nil, nil, authSvc, nil))
	require.Equal(t, http.StatusCreated, recorder.Code)

	var body struct {
		Message string        `json:"message"`
		User    auth.UserView `json:"user"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "User registered successfully", body.Message)
	require.Equal(t, "user@example.com", body.User.Email)
	require.Equal(t, "Nickname", body.User.Nickname)
}

func TestRouter_LoginInvalidCredentials(t *testing.T) {
	authSvc := &stubAuth{
		loginFn: func(ctx context.Context, req auth.LoginRequest) (auth.LoginResponse, error) {
			return auth.LoginResponse{}, apperrors.Wrap("invalid_credentials", "invalid", nil)
		},
	}
	recorder := performRequest("/api/v1/auth/login", `{"email":"user@example.com","password":"wrong"}`, newRouterUnderTest(t, &stubSummarizer{}, nil, nil, authSvc, nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "invalid_credentials", errBody["error"]["code"])
}

func TestRouter_RefreshSuccess(t *testing.T) {
	authSvc := &stubAuth{
		refreshFn: func(ctx context.Context, token string) (auth.LoginResponse, error) {
			require.Equal(t, "refresh-token", token)
			return auth.LoginResponse{Token: "new-token", RefreshToken: "new-refresh", User: auth.UserView{Email: "user@example.com", Nickname: "Nick"}}, nil
		},
	}
	recorder := performRequest("/api/v1/auth/refresh", `{"refreshToken":"refresh-token"}`, newRouterUnderTest(t, &stubSummarizer{}, nil, nil, authSvc, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp auth.LoginResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "new-token", resp.Token)
	require.Equal(t, "new-refresh", resp.RefreshToken)
}

func TestRouter_RefreshInvalid(t *testing.T) {
	authSvc := &stubAuth{
		refreshFn: func(ctx context.Context, token string) (auth.LoginResponse, error) {
			return auth.LoginResponse{}, apperrors.Wrap("invalid_token", "expired", nil)
		},
	}
	recorder := performRequest("/api/v1/auth/refresh", `{"refreshToken":"bad"}`, newRouterUnderTest(t, &stubSummarizer{}, nil, nil, authSvc, nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	errBody := decodeErrorBody(t, recorder.Body.Bytes())
	require.Equal(t, "invalid_token", errBody["error"]["code"])
}

func TestRouter_ProtectedRequiresAuth(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, nil, nil)
	recorder := performJSONRequest(http.MethodPost, "/api/v1/summaries", `{"text":"hello"}`, server, withoutAuth())
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	errBody := assertStructuredError(t, recorder.Body.Bytes())
	require.Equal(t, "unauthorized", errBody["error"]["code"])
}

func TestRouter_ProtectedContractSmokeRejectsMissingAuth(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, nil, nil)
	documentID := "11111111-1111-1111-1111-111111111111"
	sessionID := "22222222-2222-2222-2222-222222222222"
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "auth profile", method: http.MethodGet, path: "/api/v1/auth/me"},
		{name: "auth logout", method: http.MethodPost, path: "/api/v1/auth/logout"},
		{name: "summarizer sync", method: http.MethodPost, path: "/api/v1/summaries", body: `{"text":"hello"}`},
		{name: "summarizer stream", method: http.MethodPost, path: "/api/v1/summaries/stream", body: `{"text":"hello"}`},
		{name: "uv advice", method: http.MethodPost, path: "/api/v1/uv-advice", body: `{"date":"2026-06-13"}`},
		{name: "faq search", method: http.MethodPost, path: "/api/v1/faq/search", body: `{"question":"hello"}`},
		{name: "faq trending", method: http.MethodGet, path: "/api/v1/faq/trending"},
		{name: "upload document", method: http.MethodPost, path: "/api/v1/upload-ask/documents"},
		{name: "upload document list", method: http.MethodGet, path: "/api/v1/upload-ask/documents"},
		{name: "upload document get", method: http.MethodGet, path: "/api/v1/upload-ask/documents/" + documentID},
		{name: "upload qa query", method: http.MethodPost, path: "/api/v1/upload-ask/qa/query", body: `{"query":"hello"}`},
		{name: "upload qa sessions", method: http.MethodGet, path: "/api/v1/upload-ask/qa/sessions"},
		{name: "upload qa session logs", method: http.MethodGet, path: "/api/v1/upload-ask/qa/sessions/" + sessionID + "/logs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performJSONRequest(tc.method, tc.path, tc.body, server, withoutAuth())
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			errBody := assertStructuredError(t, recorder.Body.Bytes())
			require.Equal(t, "unauthorized", errBody["error"]["code"])
		})
	}
}

func TestRouter_ProtectedContractSmokeRejectsInvalidBearerToken(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, nil, nil)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "auth profile", method: http.MethodGet, path: "/api/v1/auth/me"},
		{name: "summarizer sync", method: http.MethodPost, path: "/api/v1/summaries", body: `{"text":"hello"}`},
		{name: "uv advice", method: http.MethodPost, path: "/api/v1/uv-advice", body: `{"date":"2026-06-13"}`},
		{name: "faq search", method: http.MethodPost, path: "/api/v1/faq/search", body: `{"question":"hello"}`},
		{name: "upload qa query", method: http.MethodPost, path: "/api/v1/upload-ask/qa/query", body: `{"query":"hello"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performJSONRequest(tc.method, tc.path, tc.body, server, withAuthToken("invalid-token"))
			require.Equal(t, http.StatusForbidden, recorder.Code)
			errBody := assertStructuredError(t, recorder.Body.Bytes())
			require.Equal(t, "invalid_token", errBody["error"]["code"])
		})
	}
}

func TestRouter_PublicAuthContractSmoke(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, &stubAuth{
		loginFn: func(ctx context.Context, req auth.LoginRequest) (auth.LoginResponse, error) {
			return auth.LoginResponse{}, apperrors.Wrap("invalid_credentials", "invalid email or password", nil)
		},
		refreshFn: func(ctx context.Context, token string) (auth.LoginResponse, error) {
			return auth.LoginResponse{}, apperrors.Wrap("invalid_token", "expired", nil)
		},
	}, nil)
	cases := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "register invalid payload", path: "/api/v1/auth/register", body: `{"email":123}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "login invalid credentials", path: "/api/v1/auth/login", body: `{"email":"user@example.com","password":"wrong"}`, wantStatus: http.StatusUnauthorized, wantCode: "invalid_credentials"},
		{name: "refresh invalid token", path: "/api/v1/auth/refresh", body: `{"refreshToken":"expired"}`, wantStatus: http.StatusUnauthorized, wantCode: "invalid_token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performJSONRequest(http.MethodPost, tc.path, tc.body, server, withoutAuth())
			require.Equal(t, tc.wantStatus, recorder.Code)
			errBody := assertStructuredError(t, recorder.Body.Bytes())
			require.Equal(t, tc.wantCode, errBody["error"]["code"])
		})
	}
}

func TestRouter_UploadAskLocalContractSmoke(t *testing.T) {
	storage := newBlockingGetStorage()
	uploadSvc := newQueuedLocalUploadAskServiceForTest(t, storage)
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, nil, uploadSvc)

	uploadCtx, cancelUploadCtx := context.WithCancel(context.Background())
	upload := performMultipartUpload(t, "/api/v1/upload-ask/documents", server, "local-notes.txt", "Local Notes", "Local SQLite upload ask contract text with citation context.", withRequestContext(uploadCtx))
	cancelUploadCtx()
	require.Equal(t, http.StatusAccepted, upload.Code)

	var uploadBody struct {
		Document uploadask.Document `json:"document"`
	}
	require.NoError(t, json.Unmarshal(upload.Body.Bytes(), &uploadBody))
	require.NotEqual(t, uuid.Nil, uploadBody.Document.ID)
	require.Equal(t, int64(1), uploadBody.Document.UserID)
	require.Equal(t, "Local Notes", uploadBody.Document.Title)
	require.Equal(t, uploadask.DocumentSourceUpload, uploadBody.Document.Source)
	require.Equal(t, uploadask.DocumentStatusPending, uploadBody.Document.Status)
	require.False(t, uploadBody.Document.CreatedAt.IsZero())
	require.False(t, uploadBody.Document.UpdatedAt.IsZero())

	docPath := "/api/v1/upload-ask/documents/" + uploadBody.Document.ID.String()
	pending := performJSONRequest(http.MethodGet, docPath, "", server)
	require.Equal(t, http.StatusOK, pending.Code)
	var pendingDoc uploadask.Document
	require.NoError(t, json.Unmarshal(pending.Body.Bytes(), &pendingDoc))
	require.Contains(t, []uploadask.DocumentStatus{uploadask.DocumentStatusPending, uploadask.DocumentStatusProcessing}, pendingDoc.Status)

	storage.AllowGet()
	require.Eventually(t, func() bool {
		processed := performJSONRequest(http.MethodGet, docPath, "", server)
		if processed.Code != http.StatusOK {
			return false
		}
		var doc uploadask.Document
		if err := json.Unmarshal(processed.Body.Bytes(), &doc); err != nil {
			return false
		}
		return doc.Status == uploadask.DocumentStatusProcessed
	}, time.Second, 10*time.Millisecond)

	processed := performJSONRequest(http.MethodGet, docPath, "", server)
	require.Equal(t, http.StatusOK, processed.Code)
	var processedDoc uploadask.Document
	require.NoError(t, json.Unmarshal(processed.Body.Bytes(), &processedDoc))
	require.Equal(t, uploadask.DocumentStatusProcessed, processedDoc.Status)
	require.Nil(t, processedDoc.FailureReason)

	list := performJSONRequest(http.MethodGet, "/api/v1/upload-ask/documents?status=processed", "", server)
	require.Equal(t, http.StatusOK, list.Code)
	var listBody struct {
		Items []uploadask.Document `json:"items"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listBody))
	require.Len(t, listBody.Items, 1)
	require.Equal(t, uploadBody.Document.ID, listBody.Items[0].ID)

	ask := performJSONRequest(
		http.MethodPost,
		"/api/v1/upload-ask/qa/query",
		`{"query":"What does the local contract mention?","documentIds":["`+uploadBody.Document.ID.String()+`"],"topK":1}`,
		server,
	)
	require.Equal(t, http.StatusOK, ask.Code)
	var askBody uploadask.AskResponse
	require.NoError(t, json.Unmarshal(ask.Body.Bytes(), &askBody))
	require.NotEqual(t, uuid.Nil, askBody.SessionID)
	require.NotEmpty(t, askBody.Answer)
	require.GreaterOrEqual(t, askBody.LatencyMs, int64(0))
	require.GreaterOrEqual(t, askBody.UsedHistoryTokens, 0)
	require.Len(t, askBody.Sources, 1)
	require.Equal(t, uploadBody.Document.ID, askBody.Sources[0].DocumentID)
	require.Equal(t, 0, askBody.Sources[0].ChunkIndex)
	require.GreaterOrEqual(t, askBody.Sources[0].Score, 0.0)
	require.Contains(t, askBody.Sources[0].Preview, "Local SQLite")

	sessions := performJSONRequest(http.MethodGet, "/api/v1/upload-ask/qa/sessions", "", server)
	require.Equal(t, http.StatusOK, sessions.Code)
	var sessionsBody struct {
		Sessions []uploadask.QASession `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(sessions.Body.Bytes(), &sessionsBody))
	require.Len(t, sessionsBody.Sessions, 1)
	require.Equal(t, askBody.SessionID, sessionsBody.Sessions[0].ID)

	logs := performJSONRequest(http.MethodGet, "/api/v1/upload-ask/qa/sessions/"+askBody.SessionID.String()+"/logs", "", server)
	require.Equal(t, http.StatusOK, logs.Code)
	var logsBody struct {
		Logs []uploadask.QueryLog `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(logs.Body.Bytes(), &logsBody))
	require.Len(t, logsBody.Logs, 1)
	require.Equal(t, askBody.SessionID, logsBody.Logs[0].SessionID)
	require.Equal(t, "What does the local contract mention?", logsBody.Logs[0].QueryText)
	require.NotEmpty(t, logsBody.Logs[0].ResponseText)
	require.Len(t, logsBody.Logs[0].Sources, 1)
	require.Equal(t, uploadBody.Document.ID, logsBody.Logs[0].Sources[0].DocumentID)
}

func TestRouter_Profile(t *testing.T) {
	authSvc := &stubAuth{
		validateFn: func(ctx context.Context, token string) (auth.Claims, error) {
			return auth.Claims{UserID: 99, Email: "me@example.com", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		profileFn: func(ctx context.Context, userID int64) (auth.UserView, error) {
			return auth.UserView{ID: userID, Email: "me@example.com", Nickname: "MeNick"}, nil
		},
	}
	recorder := performJSONRequest(http.MethodGet, "/api/v1/auth/me", "", newRouterUnderTest(t, &stubSummarizer{}, nil, nil, authSvc, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		User auth.UserView `json:"user"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "MeNick", body.User.Nickname)
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

	server := newRouterUnderTest(t, svc, nil, nil, nil, nil, func(cfg *config.Config) {
		cfg.HTTP.Retry.Enabled = true
		cfg.HTTP.Retry.MaxAttempts = 2
		cfg.HTTP.Retry.BaseBackoff = 0
	})

	recorder := performRequest("/api/v1/summaries", `{"text":"hello"}`, server)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 2, calls)
}

func TestRouter_RateLimitExceeded(t *testing.T) {
	server := newRouterUnderTest(t, &stubSummarizer{}, nil, nil, nil, nil, func(cfg *config.Config) {
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

func performJSONRequest(method, path, body string, server *http.Server, opts ...requestOption) *httptest.ResponseRecorder {
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
	req.Header.Set("Authorization", "Bearer "+defaultAuthToken)
	for _, opt := range opts {
		opt(req)
	}
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	return rec
}

func performMultipartUpload(t *testing.T, path string, server *http.Server, filename string, title string, content string, opts ...requestOption) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	require.NoError(t, writer.WriteField("title", title))
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, &payload)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.RemoteAddr = "203.0.113.1:1234"
	req.Header.Set("Authorization", "Bearer "+defaultAuthToken)
	for _, opt := range opts {
		opt(req)
	}
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	return rec
}

const defaultAuthToken = "valid-token"

type requestOption func(req *http.Request)

func withoutAuth() requestOption {
	return func(req *http.Request) {
		req.Header.Del("Authorization")
	}
}

func withAuthToken(token string) requestOption {
	return func(req *http.Request) {
		if token == "" {
			req.Header.Del("Authorization")
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func withRequestContext(ctx context.Context) requestOption {
	return func(req *http.Request) {
		*req = *req.WithContext(ctx)
	}
}

func TestRouter_UVAdviceSuccess(t *testing.T) {
	advice := uvadvisor.Response{Date: "2024-07-01", Category: "high", Summary: "Hot", Clothing: []string{"Hat"}}
	svc := &stubUVAdvisor{
		recommendFn: func(ctx context.Context, req uvadvisor.Request) (uvadvisor.Response, error) {
			require.Equal(t, "2024-07-01", req.Date)
			return advice, nil
		},
	}

	recorder := performRequest("/api/v1/uv-advice", `{"date":"2024-07-01"}`, newRouterUnderTest(t, &stubSummarizer{}, svc, nil, nil, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp uvadvisor.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, advice.Date, resp.Date)
	require.Equal(t, advice.Summary, resp.Summary)
}

func TestRouter_UVAdviceInvalidJSON(t *testing.T) {
	recorder := performRequest("/api/v1/uv-advice", `{"date":123}`, newRouterUnderTest(t, &stubSummarizer{}, &stubUVAdvisor{}, nil, nil, nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRouter_UVAdviceUpstreamError(t *testing.T) {
	svc := &stubUVAdvisor{
		recommendFn: func(ctx context.Context, req uvadvisor.Request) (uvadvisor.Response, error) {
			return uvadvisor.Response{}, apperrors.Wrap("uv_data_error", "upstream unavailable", nil)
		},
	}
	recorder := performRequest("/api/v1/uv-advice", `{}`, newRouterUnderTest(t, &stubSummarizer{}, svc, nil, nil, nil))
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
	recorder := performRequest("/api/v1/faq/search", `{"question":"How far is the moon?"}`, newRouterUnderTest(t, &stubSummarizer{}, nil, faqSvc, nil, nil))
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
	recorder := performJSONRequest(http.MethodGet, "/api/v1/faq/trending", "", newRouterUnderTest(t, &stubSummarizer{}, nil, faqSvc, nil, nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Recommendations []faq.TrendingQuery `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Recommendations, 1)
	require.Equal(t, int64(3), body.Recommendations[0].Count)
}

func newRouterUnderTest(t *testing.T, summarySvc summarizer.Service, advisorSvc uvadvisor.Service, faqSvc faq.Service, authSvc auth.Service, uploadSvc *uploadask.Service, overrides ...func(*config.Config)) *http.Server {
	t.Helper()
	if summarySvc == nil {
		summarySvc = &stubSummarizer{}
	}
	if advisorSvc == nil {
		advisorSvc = &stubUVAdvisor{}
	}
	if faqSvc == nil {
		faqSvc = &stubFAQ{}
	}
	if authSvc == nil {
		authSvc = &stubAuth{
			validateFn: func(ctx context.Context, token string) (auth.Claims, error) {
				if token != defaultAuthToken {
					return auth.Claims{}, apperrors.Wrap("invalid_token", "invalid token", nil)
				}
				return auth.Claims{UserID: 1, Email: "tester@example.com", ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
			profileFn: func(ctx context.Context, userID int64) (auth.UserView, error) {
				return auth.UserView{ID: userID, Email: "tester@example.com", Nickname: "Tester"}, nil
			},
		}
	}
	handler := NewHandler(summarySvc, advisorSvc, faqSvc, authSvc, uploadSvc, newTestLogger())
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

func newLocalUploadAskServiceForTest() *uploadask.Service {
	return newLocalUploadAskServiceForTestWithQueueAndStorage(nil, uploadstorage.NewMemoryStorage())
}

func newQueuedLocalUploadAskServiceForTest(t *testing.T, storage uploadask.ObjectStorage) *uploadask.Service {
	t.Helper()
	queue := uploadqueue.NewImmediateQueue(nil)
	svc := newLocalUploadAskServiceForTestWithQueueAndStorage(queue, storage)
	queue.SetHandler(func(ctx context.Context, name string, payload map[string]any) {
		if name != "process_document" {
			return
		}
		rawDocID, ok := payload["document_id"].(string)
		if !ok {
			return
		}
		docID, err := uuid.Parse(rawDocID)
		if err != nil {
			return
		}
		userID, ok := payload["user_id"].(int64)
		if !ok {
			return
		}
		if err := svc.ProcessDocument(ctx, docID, userID); err != nil {
			t.Errorf("process document from queue: %v", err)
		}
	})
	return svc
}

func newLocalUploadAskServiceForTestWithQueueAndStorage(queue uploadask.JobQueue, storage uploadask.ObjectStorage) *uploadask.Service {
	docs := uploadrepo.NewMemoryDocumentRepository()
	files := uploadrepo.NewMemoryFileRepository()
	chunks := uploadrepo.NewMemoryChunkRepository(docs)
	sessions := uploadrepo.NewMemoryQASessionRepository()
	logs := uploadrepo.NewMemoryQueryLogRepository()
	return uploadask.NewService(
		uploadask.Config{
			VectorDim:       32,
			MaxFileBytes:    1024 * 1024,
			MaxRetrieved:    3,
			MaxPreviewChars: 80,
			Memory: uploadask.MemoryConfig{
				Enabled:          true,
				TopKMems:         2,
				MaxHistoryTokens: 200,
				MemoryVectorDim:  32,
				PruneLimit:       20,
			},
		},
		docs,
		files,
		chunks,
		sessions,
		logs,
		uploadmemory.NewMemoryMessageLog(),
		uploadmemory.NewMemoryStore(),
		storage,
		uploadembedder.NewDeterministicEmbedder(32),
		uploadllm.EchoLLM{},
		uploadchunker.NewSimpleChunker(120, 0),
		queue,
		newTestLogger(),
	)
}

type blockingGetStorage struct {
	mu       sync.Mutex
	objects  map[string]storedObject
	allowGet chan struct{}
}

type storedObject struct {
	data     []byte
	mimeType string
	etag     string
}

func newBlockingGetStorage() *blockingGetStorage {
	return &blockingGetStorage{
		objects:  make(map[string]storedObject),
		allowGet: make(chan struct{}),
	}
}

func (s *blockingGetStorage) Put(_ context.Context, key string, data []byte, mimeType string) (uploadask.StoredObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := append([]byte(nil), data...)
	obj := storedObject{data: copied, mimeType: mimeType, etag: "memory-etag"}
	s.objects[key] = obj
	return uploadask.StoredObject{Key: key, Size: int64(len(copied)), MimeType: mimeType, ETag: obj.etag}, nil
}

func (s *blockingGetStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	select {
	case <-s.allowGet:
	case <-time.After(time.Second):
		return nil, context.DeadlineExceeded
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[key]
	if !ok {
		return nil, apperrors.Wrap("not_found", "object not found", nil)
	}
	return io.NopCloser(bytes.NewReader(obj.data)), nil
}

func (s *blockingGetStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func (s *blockingGetStorage) AllowGet() {
	close(s.allowGet)
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

type stubAuth struct {
	registerFn   func(ctx context.Context, req auth.RegisterRequest) (auth.UserView, error)
	loginFn      func(ctx context.Context, req auth.LoginRequest) (auth.LoginResponse, error)
	googleAuthFn func(ctx context.Context, state, codeChallenge string) (string, error)
	googleCBFn   func(ctx context.Context, code, codeVerifier string) (auth.LoginResponse, error)
	refreshFn    func(ctx context.Context, token string) (auth.LoginResponse, error)
	validateFn   func(ctx context.Context, token string) (auth.Claims, error)
	profileFn    func(ctx context.Context, userID int64) (auth.UserView, error)
	logoutFn     func(ctx context.Context, userID int64) error
}

func (s *stubAuth) Register(ctx context.Context, req auth.RegisterRequest) (auth.UserView, error) {
	if s.registerFn != nil {
		return s.registerFn(ctx, req)
	}
	return auth.UserView{}, nil
}

func (s *stubAuth) Login(ctx context.Context, req auth.LoginRequest) (auth.LoginResponse, error) {
	if s.loginFn != nil {
		return s.loginFn(ctx, req)
	}
	return auth.LoginResponse{}, nil
}

func (s *stubAuth) GoogleAuthURL(ctx context.Context, state, codeChallenge string) (string, error) {
	if s.googleAuthFn != nil {
		return s.googleAuthFn(ctx, state, codeChallenge)
	}
	return "", nil
}

func (s *stubAuth) GoogleCallback(ctx context.Context, code, codeVerifier string) (auth.LoginResponse, error) {
	if s.googleCBFn != nil {
		return s.googleCBFn(ctx, code, codeVerifier)
	}
	return auth.LoginResponse{}, nil
}

func (s *stubAuth) Refresh(ctx context.Context, token string) (auth.LoginResponse, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx, token)
	}
	return auth.LoginResponse{}, nil
}

func (s *stubAuth) ValidateToken(ctx context.Context, token string) (auth.Claims, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, token)
	}
	return auth.Claims{}, nil
}

func (s *stubAuth) Profile(ctx context.Context, userID int64) (auth.UserView, error) {
	if s.profileFn != nil {
		return s.profileFn(ctx, userID)
	}
	return auth.UserView{}, nil
}

func (s *stubAuth) Logout(ctx context.Context, userID int64) error {
	if s.logoutFn != nil {
		return s.logoutFn(ctx, userID)
	}
	return nil
}

func decodeErrorBody(t *testing.T, raw []byte) map[string]map[string]string {
	t.Helper()
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}

func assertStructuredError(t *testing.T, raw []byte) map[string]map[string]string {
	t.Helper()
	body := decodeErrorBody(t, raw)
	require.Contains(t, body, "error")
	require.NotEmpty(t, body["error"]["code"])
	require.NotEmpty(t, body["error"]["message"])
	return body
}
