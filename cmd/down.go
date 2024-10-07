package cmd

import (
	"fmt"
	"log/slog"
	"orchid/internal/config"
	"orchid/internal/orchestrator"
	"orchid/internal/ssh"
	"os"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Bring down applications in the specified environment",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			slog.Error("Failed to load config", "error", err)
			return err
		}

		envConfig, exists := cfg.Environments[env]
		if !exists {
			slog.Error("Environment not found in config", "environment", env)
			return fmt.Errorf("environment '%s' not found in config", env)
		}

		sshUser := envConfig.RemoteUser
		if sshUser == "" {
			slog.Error("SSH user (remote_user) not specified in config for environment", "environment", env)
			return fmt.Errorf("SSH user (remote_user) not specified in config for environment '%s'", env)
		}

		if sshKeyPath == "" {
			slog.Error("SSH key path must be specified")
			return fmt.Errorf("SSH key path must be specified using --ssh-key flag")
		}

		knownHostsPath := os.Getenv("HOME") + "/.ssh/known_hosts"
		port := 22

		sshFactory := ssh.NewRealSSHFactory(sshUser, sshKeyPath, knownHostsPath, port)

		if dryRun {
			slog.Info("Running in dry-run mode. No commands will be executed.")
		}

		orch, err := orchestrator.NewOrchestrator(cfg, sshFactory, env, nil, dryRun)
		if err != nil {
			slog.Error("Failed to initialize orchestrator", "error", err)
			return err
		}

		if err := orch.BringDown(cmd.Context()); err != nil {
			slog.Error("Bring down failed", "error", err)
			return err
		}

		return nil
	},
}
