package cmd

import (
	"log/slog"
	"orchid/internal/logger"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath string
	sshKeyPath string
	logLevel   string
	env        string
	dryRun     bool
	rootCmd    = &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Use:           "orchid",
		Short:         "Orchestrates on-premises applications for testing and deployment",
		Long:          `A CLI tool to manage the lifecycle of on-premises applications with dependencies.`,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "orchid.yml", "Path to configuration file")
	rootCmd.PersistentFlags().StringVarP(&sshKeyPath, "ssh-key", "k", "", "Path to SSH private key")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
	rootCmd.PersistentFlags().StringVarP(&env, "env", "e", "", "Environment to use from the config file")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate the operations without executing commands")
	rootCmd.MarkPersistentFlagRequired("ssh-key")
	rootCmd.MarkPersistentFlagRequired("env")

	cobra.OnInitialize(initLogger)

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
}

func initLogger() {
	if err := logger.InitLogger(logLevel); err != nil {
		slog.Error("Failed to initialize logger", "error", err)
		os.Exit(1)
	}
}
