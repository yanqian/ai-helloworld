package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Handler wires the HTTP transport to domain services.
type Handler struct {
	summarizerSvc summarizer.Service
	advisorSvc    uvadvisor.Service
	faqSvc        faq.Service
	logger        *slog.Logger
}

// NewHandler constructs the root HTTP handler.
func NewHandler(summarySvc summarizer.Service, advisorSvc uvadvisor.Service, faqSvc faq.Service, logger *slog.Logger) *Handler {
	return &Handler{
		summarizerSvc: summarySvc,
		advisorSvc:    advisorSvc,
		faqSvc:        faqSvc,
		logger:        logger.With("component", "http.handler"),
	}
}

// Summarize handles the sync summarization endpoint.
func (h *Handler) Summarize(c *gin.Context) {
	var req summarizer.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}

	resp, err := h.summarizerSvc.Summarize(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if apperrors.IsCode(err, "invalid_input") {
			status = http.StatusBadRequest
		}
		abortWithError(c, NewHTTPError(status, "summarize_failed", errMessage(err), err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SummarizeStream streams partial summaries using Server-Sent Events.
func (h *Handler) SummarizeStream(c *gin.Context) {
	var req summarizer.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}

	stream, err := h.summarizerSvc.StreamSummary(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if apperrors.IsCode(err, "invalid_input") {
			status = http.StatusBadRequest
		}
		abortWithError(c, NewHTTPError(status, "summarize_failed", errMessage(err), err))
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusInternalServerError, "stream_unsupported", "streaming not supported", nil))
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

// RecommendProtection returns AI generated clothing/protection suggestions.
func (h *Handler) RecommendProtection(c *gin.Context) {
	var req uvadvisor.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}

	resp, err := h.advisorSvc.Recommend(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "uv_advice_failed"
		switch {
		case apperrors.IsCode(err, "invalid_input"):
			status = http.StatusBadRequest
			code = "invalid_request"
		case apperrors.IsCode(err, "uv_data_error"):
			status = http.StatusBadGateway
		case apperrors.IsCode(err, "llm_error"):
			status = http.StatusBadGateway
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SmartFAQ answers frequently asked questions using search + caching strategies.
func (h *Handler) SmartFAQ(c *gin.Context) {
	var req faq.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}

	resp, err := h.faqSvc.Answer(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "faq_failed"
		if apperrors.IsCode(err, "invalid_input") {
			status = http.StatusBadRequest
			code = "invalid_request"
		}
		if apperrors.IsCode(err, "llm_error") {
			status = http.StatusBadGateway
			code = "llm_error"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// TrendingFAQ returns the most common search recommendations.
func (h *Handler) TrendingFAQ(c *gin.Context) {
	items, err := h.faqSvc.Trending(c.Request.Context())
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusInternalServerError, "faq_failed", errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": items})
}

func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
