package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/infra/config"
)

// NewRouter wires up the HTTP handlers and returns a configured server.
func NewRouter(cfg *config.Config, handler *SummaryHandler) *http.Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(
		gin.Recovery(),
		requestLogger(handler.logger),
		corsMiddleware(),
	)

	api := router.Group("/api/v1")
	{
		api.POST("/summaries", handler.Summarize)
		api.POST("/summaries/stream", handler.SummarizeStream)
	}

	return &http.Server{
		Addr:           cfg.HTTP.Address,
		Handler:        router,
		ReadTimeout:    cfg.HTTP.ReadTimeout,
		WriteTimeout:   cfg.HTTP.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		logger.Info("http request", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "latency_ms", latency.Milliseconds())
	}
}
