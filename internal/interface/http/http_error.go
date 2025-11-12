package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTTPError captures the metadata required to serialize an error response consistently.
type HTTPError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// NewHTTPError is a helper to build an HTTPError instance.
func NewHTTPError(status int, code, message string, err error) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message, Err: err}
}

func asHTTPError(err error) *HTTPError {
	if err == nil {
		return nil
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "something went wrong",
		Err:     err,
	}
}

func abortWithError(c *gin.Context, err *HTTPError) {
	if err == nil {
		return
	}
	_ = c.Error(err)
	c.Abort()
}
