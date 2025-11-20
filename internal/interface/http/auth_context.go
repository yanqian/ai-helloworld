package http

import (
	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
)

const authClaimsKey = "auth_claims"

func setClaims(c *gin.Context, claims auth.Claims) {
	c.Set(authClaimsKey, claims)
}

func getClaims(c *gin.Context) (auth.Claims, bool) {
	value, ok := c.Get(authClaimsKey)
	if !ok {
		return auth.Claims{}, false
	}
	claims, ok := value.(auth.Claims)
	return claims, ok
}
