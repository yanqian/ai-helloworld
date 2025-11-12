package http

import (
	"math"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/infra/config"
)

func errorHandlingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}

		httpErr := asHTTPError(c.Errors.Last().Err)
		message := httpErr.Message
		if message == "" {
			message = httpErr.Error()
		}

		if httpErr.Status >= http.StatusInternalServerError {
			logger.Error("request failed", "code", httpErr.Code, "status", httpErr.Status, "path", c.Request.URL.Path, "error", httpErr.Err)
		} else {
			logger.Warn("request failed", "code", httpErr.Code, "status", httpErr.Status, "path", c.Request.URL.Path, "error", httpErr.Err)
		}

		c.JSON(httpErr.Status, gin.H{
			"error": gin.H{
				"code":    httpErr.Code,
				"message": message,
			},
		})
	}
}

func rateLimitMiddleware(cfg config.RateLimitConfig, logger *slog.Logger) gin.HandlerFunc {
	if !cfg.Enabled || cfg.RequestsPerMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	limiter := newIPRateLimiter(cfg)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if limiter.allow(ip) {
			c.Next()
			return
		}
		logger.Warn("rate limit exceeded", "ip", ip, "path", c.Request.URL.Path)
		abortWithError(c, NewHTTPError(http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests", nil))
	}
}

type ipRateLimiter struct {
	visitors      map[string]*visitor
	mu            sync.Mutex
	ratePerMinute float64
	burst         float64
	ttl           time.Duration
}

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

func newIPRateLimiter(cfg config.RateLimitConfig) *ipRateLimiter {
	return &ipRateLimiter{
		visitors:      make(map[string]*visitor),
		ratePerMinute: float64(cfg.RequestsPerMinute),
		burst:         float64(cfg.Burst),
		ttl:           5 * time.Minute,
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{tokens: l.burst, lastSeen: now}
		l.visitors[ip] = v
	} else {
		elapsed := now.Sub(v.lastSeen).Minutes()
		if elapsed > 0 {
			refill := elapsed * l.ratePerMinute
			v.tokens = math.Min(l.burst, v.tokens+refill)
		}
		v.lastSeen = now
	}
	l.cleanupLocked(now)
	if v.tokens < 1 {
		return false
	}
	v.tokens -= 1
	return true
}

func (l *ipRateLimiter) cleanupLocked(now time.Time) {
	for ip, v := range l.visitors {
		if now.Sub(v.lastSeen) > l.ttl {
			delete(l.visitors, ip)
		}
	}
}
