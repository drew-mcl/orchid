// main.go
package main

import (
	"fmt"
	"os"
	"time"

	"orchid/internal/config"
	"orchid/internal/orchestrator"

	"log/slog"

	"github.com/spf13/cobra"
)

func main() {
	var (
		cfgFile          string
		env              string
		force            bool
		dryRun           bool
		handleDeps       bool
		stopDeps         bool
		healthCheckWait  time.Duration
		healthCheckRetry time.Duration
		operationTimeout time.Duration
		logLevel         string
		jsonLog          bool
	)

	rootCmd := &cobra.Command{
		Use: "orchid",
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (required)")
	rootCmd.PersistentFlags().StringVarP(&env, "environment", "e", "", "environment to deploy (required)")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "force action")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "dry run mode")
	rootCmd.PersistentFlags().BoolVar(&handleDeps, "handle-deps", false, "handle dependencies (start/stop)")
	rootCmd.PersistentFlags().BoolVar(&stopDeps, "stop-deps", false, "stop dependencies in down command")
	rootCmd.PersistentFlags().DurationVar(&healthCheckWait, "health-check-timeout", 60*time.Second, "Health check timeout")
	rootCmd.PersistentFlags().DurationVar(&healthCheckRetry, "health-check-interval", 2*time.Second, "Health check retry interval")
	rootCmd.PersistentFlags().DurationVar(&operationTimeout, "operation-timeout", 5*time.Minute, "Operation timeout")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&jsonLog, "json", false, "Output logs in JSON format")

	rootCmd.MarkPersistentFlagRequired("config")
	rootCmd.MarkPersistentFlagRequired("environment")

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Start services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				return err
			}

			logger := setupLogger(logLevel, jsonLog)

			opts := orchestrator.Options{
				Config:      cfg,
				Environment: env,
				Force:       force,
				DryRun:      dryRun,
				Logger:      logger,
				HandleDeps:  handleDeps,
			}
			o, err := orchestrator.New(opts)
			if err != nil {
				return err
			}
			return o.Up()
		},
	}

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Stop services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				return err
			}

			logger := setupLogger(logLevel, jsonLog)

			opts := orchestrator.Options{
				Config:      cfg,
				Environment: env,
				Force:       force,
				DryRun:      dryRun,
				Logger:      logger,
				StopDeps:    stopDeps,
			}
			o, err := orchestrator.New(opts)
			if err != nil {
				return err
			}
			return o.Down()
		},
	}

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setupLogger(logLevel string, jsonLog bool) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	if jsonLog {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
