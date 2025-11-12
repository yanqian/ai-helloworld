package http

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	"log/slog"

	"github.com/yanqian/ai-helloworld/internal/infra/config"
)

const retryBodyLimit = 1 << 20 // 1 MiB

var errBodyTooLarge = errors.New("request body exceeds retry limit")

func withRetry(handler http.Handler, cfg config.RetryConfig, logger *slog.Logger) http.Handler {
	if !cfg.Enabled || cfg.MaxAttempts <= 1 {
		return handler
	}
	exclusions := make(map[string]struct{}, len(cfg.Exclude))
	for _, path := range cfg.Exclude {
		exclusions[path] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, skip := exclusions[r.URL.Path]; skip || r.Method != http.MethodPost {
			handler.ServeHTTP(w, r)
			return
		}
		bodyBytes, err := readRequestBody(r)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errBodyTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, err.Error(), status)
			return
		}

		for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
			if attempt > 1 {
				delay := cfg.BaseBackoff * time.Duration(1<<(attempt-2))
				if delay > 0 {
					time.Sleep(delay)
				}
			}

			recorder := newRetryResponseRecorder(w)
			reqCopy := r.Clone(r.Context())
			reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqCopy.ContentLength = int64(len(bodyBytes))

			handler.ServeHTTP(recorder, reqCopy)
			if !recorder.retryable() || attempt == cfg.MaxAttempts {
				recorder.Commit()
				return
			}

			logger.Warn("transient failure, retrying request", "path", r.URL.Path, "status", recorder.statusCode, "attempt", attempt)
		}
	})
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	reader := io.LimitReader(r.Body, retryBodyLimit+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) > retryBodyLimit {
		return nil, errBodyTooLarge
	}
	return data, nil
}

type retryResponseRecorder struct {
	dst        http.ResponseWriter
	header     http.Header
	body       bytes.Buffer
	statusCode int
	wroteHead  bool
}

func newRetryResponseRecorder(dst http.ResponseWriter) *retryResponseRecorder {
	return &retryResponseRecorder{
		dst:        dst,
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (r *retryResponseRecorder) Header() http.Header {
	return r.header
}

func (r *retryResponseRecorder) WriteHeader(status int) {
	if r.wroteHead {
		return
	}
	r.statusCode = status
	r.wroteHead = true
}

func (r *retryResponseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *retryResponseRecorder) Commit() {
	dstHeader := r.dst.Header()
	for k := range dstHeader {
		dstHeader.Del(k)
	}
	for k, values := range r.header {
		copied := make([]string, len(values))
		copy(copied, values)
		dstHeader[k] = copied
	}
	if !r.wroteHead {
		r.statusCode = http.StatusOK
	}
	r.dst.WriteHeader(r.statusCode)
	if r.body.Len() > 0 {
		_, _ = r.dst.Write(r.body.Bytes())
	}
}

func (r *retryResponseRecorder) retryable() bool {
	return r.statusCode >= http.StatusInternalServerError
}

func (r *retryResponseRecorder) Flush() {}
