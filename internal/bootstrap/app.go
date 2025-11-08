package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/yanqian/ai-helloworld/internal/infra/config"
)

// App encapsulates the HTTP server lifecycle.
type App struct {
	cfg    *config.Config
	logger *slog.Logger
	server *http.Server
}

// NewApp is used by Wire to build the runnable app.
func NewApp(cfg *config.Config, logger *slog.Logger, server *http.Server) *App {
	return &App{cfg: cfg, logger: logger.With("component", "bootstrap"), server: server}
}

// Run starts the HTTP server and blocks until shutdown.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.logger.Info("http server starting", "address", a.cfg.HTTP.Address)
		if err := a.server.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		a.logger.Info("shutdown signal received")
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
