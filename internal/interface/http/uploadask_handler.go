package http

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	uploadask "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// UploadDocument handles multipart upload and enqueues processing.
func (h *Handler) UploadDocument(c *gin.Context) {
	if h.uploadSvc == nil {
		abortWithError(c, NewHTTPError(http.StatusServiceUnavailable, "upload_disabled", "upload service unavailable", nil))
		return
	}
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", "file is required", err))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", "failed to read upload", err))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusInternalServerError, "upload_failed", "failed to read file", err))
		return
	}
	req := uploadask.UploadRequest{
		Filename: fileHeader.Filename,
		Title:    c.PostForm("title"),
		MimeType: fileHeader.Header.Get("Content-Type"),
		Content:  data,
	}
	resp, err := h.uploadSvc.Upload(c.Request.Context(), claims.UserID, req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "upload_failed"
		switch {
		case apperrors.IsCode(err, "invalid_input"):
			status = http.StatusBadRequest
			code = "invalid_request"
		case apperrors.IsCode(err, "unauthorized"):
			status = http.StatusUnauthorized
			code = "unauthorized"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusAccepted, resp)
}

// ListDocuments returns the user's uploads.
func (h *Handler) ListDocuments(c *gin.Context) {
	if h.uploadSvc == nil {
		abortWithError(c, NewHTTPError(http.StatusServiceUnavailable, "upload_disabled", "upload service unavailable", nil))
		return
	}
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	statuses := parseStatuses(c.Query("status"))
	filter := uploadask.DocumentFilter{Statuses: statuses}
	docs, err := h.uploadSvc.ListDocuments(c.Request.Context(), claims.UserID, filter)
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusInternalServerError, "fetch_failed", errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": docs})
}

// GetDocument returns a single document's metadata.
func (h *Handler) GetDocument(c *gin.Context) {
	if h.uploadSvc == nil {
		abortWithError(c, NewHTTPError(http.StatusServiceUnavailable, "upload_disabled", "upload service unavailable", nil))
		return
	}
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", "invalid document id", err))
		return
	}
	doc, err := h.uploadSvc.GetDocument(c.Request.Context(), claims.UserID, id)
	if err != nil {
		status := http.StatusInternalServerError
		code := "fetch_failed"
		if apperrors.IsCode(err, "not_found") {
			status = http.StatusNotFound
			code = "not_found"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, doc)
}

type askPayload struct {
	Query       string   `json:"query"`
	SessionID   *string  `json:"sessionId"`
	DocumentIDs []string `json:"documentIds"`
	TopK        int      `json:"topK"`
}

// AskQuestion performs retrieval augmented question answering.
func (h *Handler) AskQuestion(c *gin.Context) {
	if h.uploadSvc == nil {
		abortWithError(c, NewHTTPError(http.StatusServiceUnavailable, "upload_disabled", "upload service unavailable", nil))
		return
	}
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	var req askPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", errMessage(err), err))
		return
	}
	var sessionID *uuid.UUID
	if req.SessionID != nil {
		parsed, err := uuid.Parse(*req.SessionID)
		if err != nil {
			abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", "invalid sessionId", err))
			return
		}
		sessionID = &parsed
	}
	docIDs := make([]uuid.UUID, 0, len(req.DocumentIDs))
	for _, raw := range req.DocumentIDs {
		if raw == "" {
			continue
		}
		parsed, err := uuid.Parse(raw)
		if err != nil {
			abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", "invalid documentIds entry", err))
			return
		}
		docIDs = append(docIDs, parsed)
	}
	resp, err := h.uploadSvc.Ask(c.Request.Context(), claims.UserID, uploadask.AskRequest{
		Query:       req.Query,
		SessionID:   sessionID,
		DocumentIDs: docIDs,
		TopK:        req.TopK,
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "query_failed"
		switch {
		case apperrors.IsCode(err, "invalid_input"):
			status = http.StatusBadRequest
			code = "invalid_request"
		case apperrors.IsCode(err, "unauthorized"):
			status = http.StatusUnauthorized
			code = "unauthorized"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListSessions returns QA sessions for the current user.
func (h *Handler) ListSessions(c *gin.Context) {
	if h.uploadSvc == nil {
		abortWithError(c, NewHTTPError(http.StatusServiceUnavailable, "upload_disabled", "upload service unavailable", nil))
		return
	}
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	sessions, err := h.uploadSvc.ListSessions(c.Request.Context(), claims.UserID)
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusInternalServerError, "fetch_failed", errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// ListSessionLogs returns Q&A history for a session.
func (h *Handler) ListSessionLogs(c *gin.Context) {
	if h.uploadSvc == nil {
		abortWithError(c, NewHTTPError(http.StatusServiceUnavailable, "upload_disabled", "upload service unavailable", nil))
		return
	}
	claims, ok := getClaims(c)
	if !ok {
		abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing token", nil))
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		abortWithError(c, NewHTTPError(http.StatusBadRequest, "invalid_request", "invalid session id", err))
		return
	}
	logs, err := h.uploadSvc.ListSessionLogs(c.Request.Context(), claims.UserID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "fetch_failed"
		if apperrors.IsCode(err, "not_found") {
			status = http.StatusNotFound
			code = "not_found"
		}
		abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func parseStatuses(raw string) []uploadask.DocumentStatus {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]uploadask.DocumentStatus, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch strings.ToLower(part) {
		case "pending":
			out = append(out, uploadask.DocumentStatusPending)
		case "processing":
			out = append(out, uploadask.DocumentStatusProcessing)
		case "processed":
			out = append(out, uploadask.DocumentStatusProcessed)
		case "failed":
			out = append(out, uploadask.DocumentStatusFailed)
		}
	}
	return out
}
