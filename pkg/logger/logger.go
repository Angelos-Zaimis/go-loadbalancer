package logger

import (
	"log/slog"
	"os"
	"strings"
)

func New(lvl string, addSource bool, enviroment string) *slog.Logger {

	level := parseLevel(lvl)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}
	var handler slog.Handler

	if strings.ToLower(enviroment) == "prod" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler).With(
		slog.String("environment", enviroment),
	)
}

func parseLevel(level string) slog.Level {

	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
