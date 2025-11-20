package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
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
	authSvc       auth.Service
	logger        *slog.Logger
}

// NewHandler constructs the root HTTP handler.
func NewHandler(summarySvc summarizer.Service, advisorSvc uvadvisor.Service, faqSvc faq.Service, authSvc auth.Service, logger *slog.Logger) *Handler {
	return &Handler{
		summarizerSvc: summarySvc,
		advisorSvc:    advisorSvc,
		faqSvc:        faqSvc,
		authSvc:       authSvc,
		logger:        logger.With("component", "http.handler"),
	}
}

// Register handles account creation.
func (h *Handler) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}
	user, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "auth_failed"
		switch {
		case apperrors.IsCode(err, "invalid_input"):
			status = http.StatusBadRequest
			code = "invalid_request"
		case apperrors.IsCode(err, "email_exists"):
			status = http.StatusConflict
			code = "email_exists"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user":    user,
	})
}

// Login authenticates and issues a JWT.
func (h *Handler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}
	resp, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "auth_failed"
		switch {
		case apperrors.IsCode(err, "invalid_input"):
			status = http.StatusBadRequest
			code = "invalid_request"
		case apperrors.IsCode(err, "invalid_credentials"):
			status = http.StatusUnauthorized
			code = "invalid_credentials"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Refresh exchanges a refresh token for a new access token.
func (h *Handler) Refresh(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}
	resp, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		status := http.StatusInternalServerError
		code := "auth_failed"
		if apperrors.IsCode(err, "invalid_token") {
			status = http.StatusUnauthorized
			code = "invalid_token"
		}
		if apperrors.IsCode(err, "user_not_found") {
			status = http.StatusNotFound
			code = "user_not_found"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Profile returns the authenticated user's info.
func (h *Handler) Profile(c *gin.Context) {
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	user, err := h.authSvc.Profile(c.Request.Context(), claims.UserID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "auth_failed"
		if apperrors.IsCode(err, "user_not_found") {
			status = http.StatusNotFound
			code = "user_not_found"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Welcome to the private dashboard",
		"user":    user,
	})
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
