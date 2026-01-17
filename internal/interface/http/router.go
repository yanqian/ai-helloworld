package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanqian/ai-helloworld/internal/infra/config"
)

// NewRouter wires up the HTTP handlers and returns a configured server.
func NewRouter(cfg *config.Config, handler *Handler) *http.Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(
		gin.Recovery(),
		errorHandlingMiddleware(handler.logger),
		requestLogger(handler.logger),
		corsMiddleware(cfg.HTTP.AllowedOrigins),
		rateLimitMiddleware(cfg.HTTP.RateLimit, handler.logger),
	)

	api := router.Group("/api/v1")
	{
		authRoutes := api.Group("/auth")
		{
			authRoutes.POST("/register", handler.Register)
			authRoutes.POST("/login", handler.Login)
			authRoutes.POST("/refresh", handler.Refresh)
			authRoutes.GET("/google/login", handler.GoogleLogin)
			authRoutes.GET("/google/callback", handler.GoogleCallback)
		}

		protected := api.Group("/")
		protected.Use(authMiddleware(handler.authSvc))
		{
			protected.POST("/auth/logout", handler.Logout)
			protected.POST("/summaries", handler.Summarize)
			protected.POST("/summaries/stream", handler.SummarizeStream)
			protected.POST("/uv-advice", handler.RecommendProtection)
			protected.POST("/faq/search", handler.SmartFAQ)
			protected.GET("/faq/trending", handler.TrendingFAQ)
			protected.GET("/auth/me", handler.Profile)
			uploadAsk := protected.Group("/upload-ask")
			{
				uploadAsk.POST("/documents", handler.UploadDocument)
				uploadAsk.GET("/documents", handler.ListDocuments)
				uploadAsk.GET("/documents/:id", handler.GetDocument)
				uploadAsk.POST("/qa/query", handler.AskQuestion)
				uploadAsk.GET("/qa/sessions", handler.ListSessions)
				uploadAsk.GET("/qa/sessions/:id/logs", handler.ListSessionLogs)
			}
		}
	}

	return &http.Server{
		Addr:           cfg.HTTP.Address,
		Handler:        withRetry(router, cfg.HTTP.Retry, handler.logger),
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
