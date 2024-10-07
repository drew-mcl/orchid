package logger

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogger initializes the default slog logger with a TextHandler.
// It sets the log level based on the provided logLevel string.
func InitLogger(logLevel string) error {
	level := parseLogLevel(logLevel)

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
	return nil
}

// parseLogLevel converts a string log level to slog.Level.
// Defaults to slog.LevelInfo if the input is unrecognized.
func parseLogLevel(logLevel string) slog.Level {
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
