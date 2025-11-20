package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// corsMiddleware injects permissive CORS headers so the Cloud Run frontend can call the API.
func corsMiddleware(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Access-Control-Allow-Origin", resolveOrigin(c.GetHeader("Origin"), allowed))
		headers.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		headers.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func resolveOrigin(requestOrigin string, allowed []string) string {
	if len(allowed) == 0 {
		return "*"
	}
	for _, candidate := range allowed {
		if candidate == "*" {
			return "*"
		}
		if requestOrigin != "" && strings.EqualFold(candidate, requestOrigin) {
			return requestOrigin
		}
	}
	return allowed[0]
}
