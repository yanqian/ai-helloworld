package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

func authMiddleware(svc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "missing authorization header", nil))
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortWithError(c, NewHTTPError(http.StatusUnauthorized, "unauthorized", "invalid authorization header", nil))
			return
		}
		token := strings.TrimSpace(parts[1])
		claims, err := svc.ValidateToken(c.Request.Context(), token)
		if err != nil {
			status := http.StatusForbidden
			code := "invalid_token"
			if !apperrors.IsCode(err, "invalid_token") {
				status = http.StatusInternalServerError
				code = "auth_failed"
			}
			abortWithError(c, NewHTTPError(status, code, errMessage(err), err))
			return
		}
		setClaims(c, claims)
		c.Next()
	}
}
