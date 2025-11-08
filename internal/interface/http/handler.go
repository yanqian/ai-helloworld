package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// SummaryHandler wires the summarizer service with HTTP transport.
type SummaryHandler struct {
	svc    summarizer.Service
	logger *slog.Logger
}

// NewSummaryHandler constructs a SummaryHandler instance.
func NewSummaryHandler(svc summarizer.Service, logger *slog.Logger) *SummaryHandler {
	return &SummaryHandler{svc: svc, logger: logger.With("component", "http.handler")}
}

// Summarize handles the sync summarization endpoint.
func (h *SummaryHandler) Summarize(c *gin.Context) {
	var req summarizer.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	resp, err := h.svc.Summarize(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if apperrors.IsCode(err, "invalid_input") {
			status = http.StatusBadRequest
		}
		h.writeError(c, status, "summarize_failed", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SummarizeStream streams partial summaries using Server-Sent Events.
func (h *SummaryHandler) SummarizeStream(c *gin.Context) {
	var req summarizer.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	stream, err := h.svc.StreamSummary(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if apperrors.IsCode(err, "invalid_input") {
			status = http.StatusBadRequest
		}
		h.writeError(c, status, "summarize_failed", err)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.writeError(c, http.StatusInternalServerError, "stream_unsupported", nil)
		return
	}

	for chunk := range stream {
		payload, err := json.Marshal(chunk)
		if err != nil {
			h.logger.Error("marshal chunk failed", "error", err)
			continue
		}
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(payload)
		c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}
}

func (h *SummaryHandler) writeError(c *gin.Context, status int, code string, err error) {
	if err != nil {
		h.logger.Error("request failed", "code", code, "error", err)
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": errMessage(err),
		},
	})
}

func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
