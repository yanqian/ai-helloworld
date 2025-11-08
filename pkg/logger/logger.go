package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New constructs a JSON slog logger that adheres to the Codex logging checklist.
func New() *slog.Logger {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With("service", "summarizer")
}

func parseLevel(level string) slog.Leveler {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
