package logger

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogger initializes the default slog logger with a TextHandler.
// It sets the log level based on the provided logLevel string and adds contextual fields.
func InitLogger(logLevel string) error {
	level := parseLogLevel(logLevel)

	var attrs []slog.Attr

	orchidEnv := getOrDefault("ORCHID_ENV", "local")
	attrs = append(attrs, slog.String("orchid_env", orchidEnv))

	if pipelineID := os.Getenv("CI_PIPELINE_ID"); pipelineID != "" {
		attrs = append(attrs, slog.String("pipeline_id", pipelineID))
	}

	if commitRef := os.Getenv("CI_COMMIT_REF_NAME"); commitRef != "" {
		attrs = append(attrs, slog.String("commit_ref", commitRef))
	}

	if projectName := os.Getenv("CI_PROJECT_NAME"); projectName != "" {
		attrs = append(attrs, slog.String("project_name", projectName))
	}

	if environment := os.Getenv("CI_ENVIRONMENT_NAME"); environment != "" {
		attrs = append(attrs, slog.String("ci_environment", environment))
	}

	// Create a TextHandler with the specified log level
	var handler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	handler = handler.WithAttrs(attrs)

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

// getOrDefault retrieves the value of the environment variable named by the key.
// If the variable is not present, it returns the provided default value.
func getOrDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
