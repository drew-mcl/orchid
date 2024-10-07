package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"orchid/internal/config"
	"orchid/internal/ssh"
)

type Orchestrator struct {
	cfg         *config.Config
	sshFactory  ssh.SSHFactory
	environment string
	appStates   map[string]bool
	mutex       sync.Mutex
	cancelFunc  context.CancelFunc
	wg          sync.WaitGroup
	flagManager *FlagManager
	monitorChan chan error
	dryRun      bool
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator(cfg *config.Config, sshFactory ssh.SSHFactory, environment string, flagManager *FlagManager, dryRun bool) (*Orchestrator, error) {
	if environment == "" {
		return nil, fmt.Errorf("environment must be specified")
	}

	if _, exists := cfg.Environments[environment]; !exists {
		return nil, fmt.Errorf("environment '%s' not found in config", environment)
	}

	var fm *FlagManager
	if flagManager != nil {
		fm = flagManager
	} else {
		flagDir := filepath.Join(os.Getenv("HOME"), "sre", "flags")
		if !dryRun {
			if err := os.MkdirAll(flagDir, 0755); err != nil {
				return nil, fmt.Errorf("creating flag directory: %w", err)
			}
		}
		flagPath := filepath.Join(flagDir, fmt.Sprintf("%s.flag", environment))
		fm = NewFlagManager(flagPath)
	}

	return &Orchestrator{
		cfg:         cfg,
		sshFactory:  sshFactory,
		environment: environment,
		appStates:   make(map[string]bool),
		flagManager: fm,
		monitorChan: make(chan error, 1),
		dryRun:      dryRun,
	}, nil
}

// BringUp starts all applications in the specified environment.
func (o *Orchestrator) BringUp(ctx context.Context) error {
	if !o.dryRun {
		if err := o.flagManager.Acquire(); err != nil {
			slog.Error("Failed to acquire flag", "error", err, "flagPath", o.flagManager.flagPath)
			return err
		}
		defer func() {
			if err := o.flagManager.Release(); err != nil {
				slog.Warn("Failed to release flag", "error", err, "flagPath", o.flagManager.flagPath)
			}
		}()
	} else {
		slog.Info("[Dry-run] Skipping flag acquisition")
	}

	env := o.cfg.Environments[o.environment]

	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel
	defer cancel()

	if !o.dryRun {
		o.wg.Add(1)
		go o.monitorApps(ctx)
	}

	for _, app := range env.Applications {
		select {
		case <-ctx.Done():
			slog.Warn("Bring up operation canceled")
			return ctx.Err()
		case err := <-o.monitorChan:
			slog.Error("Application failed during bring up, initiating rollback", "error", err)
			o.cancelFunc()
			o.wg.Wait()
			return o.rollback()
		default:
		}

		client, err := o.sshFactory.GetClient(app.Host, o.dryRun)
		if err != nil {
			slog.Error("Failed to get SSH client", "host", app.Host, "error", err)
			o.cancelFunc()
			o.wg.Wait()
			return o.rollback()
		}

		// Ensure the app is down by running the check command
		if !o.dryRun {
			err = client.RunCommand(app.CheckCommand)
		} else {
			slog.Info("[Dry-run] Would run check command", "command", app.CheckCommand, "app", app.Name, "host", app.Host)
			err = fmt.Errorf("simulated check failure")
		}

		if err == nil {
			// If check command succeeds rc 0, app is running, attempt to stop it first
			slog.Info("App is already running, attempting to stop before starting", "app", app.Name, "host", app.Host)
			if !o.dryRun {
				if err := client.RunCommand(app.StopCommand); err != nil {
					slog.Error("Failed to stop app before starting", "app", app.Name, "host", app.Host, "error", err)
					o.cancelFunc()
					o.wg.Wait()
					return o.rollback()
				}
			} else {
				slog.Info("[Dry-run] Would execute stop command", "command", app.StopCommand, "app", app.Name, "host", app.Host)
			}
		} else {
			// If check command fails (rc != 0), assume app is not running and proceed
			slog.Info("App is not running, proceeding to start", "app", app.Name, "host", app.Host)
		}

		slog.Info("Starting app", "app", app.Name, "host", app.Host)
		if !o.dryRun {
			if err := client.RunCommand(app.StartCommand); err != nil {
				slog.Error("Failed to start app", "app", app.Name, "host", app.Host, "error", err)
				o.cancelFunc()
				o.wg.Wait()
				return o.rollback()
			}
		} else {
			slog.Info("[Dry-run] Would execute start command", "command", app.StartCommand, "app", app.Name, "host", app.Host)
		}

		o.mutex.Lock()
		o.appStates[app.Name] = true
		o.mutex.Unlock()

		// Wait for the check interval
		time.Sleep(time.Duration(app.CheckInterval) * time.Second)

		if !o.dryRun {
			err = client.RunCommand(app.CheckCommand)
		} else {
			slog.Info("[Dry-run] Assuming app started successfully", "app", app.Name, "host", app.Host)
			err = nil
		}

		if err != nil {
			// If check command fails, initiate rollback
			slog.Error("App failed to start correctly", "app", app.Name, "host", app.Host, "error", err)
			o.cancelFunc()
			o.wg.Wait()
			return o.rollback()
		}

		slog.Info("App started successfully", "app", app.Name, "host", app.Host)
	}

	// Wait for a short period to ensure monitoring has not detected any failures
	if !o.dryRun {
		select {
		case err := <-o.monitorChan:
			if err != nil {
				slog.Error("Application failed during bring up, initiating rollback", "error", err)
				o.cancelFunc()
				o.wg.Wait()
				return o.rollback()
			}
		case <-time.After(2 * time.Second):
		}
	}

	slog.Info("All applications started successfully")
	return nil
}

// BringDown stops all applications in the specified environment.
func (o *Orchestrator) BringDown(ctx context.Context) error {
	if !o.dryRun {
		if err := o.flagManager.Acquire(); err != nil {
			slog.Error("Failed to acquire flag", "error", err, "flagPath", o.flagManager.flagPath)
			return err
		}
		defer func() {
			if err := o.flagManager.Release(); err != nil {
				slog.Warn("Failed to release flag", "error", err, "flagPath", o.flagManager.flagPath)
			}
		}()
	} else {
		slog.Info("Dry-run mode: Skipping flag acquisition")
	}

	env := o.cfg.Environments[o.environment]

	for i := len(env.Applications) - 1; i >= 0; i-- {
		app := env.Applications[i]

		select {
		case <-ctx.Done():
			slog.Warn("Bring down operation canceled")
			return ctx.Err()
		default:
		}

		client, err := o.sshFactory.GetClient(app.Host, o.dryRun)
		if err != nil {
			slog.Error("Failed to get SSH client", "host", app.Host, "error", err)
			continue
		}

		// Stop the app
		slog.Info("Stopping app", "app", app.Name, "host", app.Host)
		if !o.dryRun {
			if err := client.RunCommand(app.StopCommand); err != nil {
				slog.Error("Failed to stop app", "app", app.Name, "host", app.Host, "error", err)
			} else {
				slog.Info("Stopped app", "app", app.Name, "host", app.Host)
			}
		} else {
			slog.Info("[Dry-run] Would execute stop command", "command", app.StopCommand, "app", app.Name, "host", app.Host)
		}

		// Check app status by running the check command
		if !o.dryRun {
			err = client.RunCommand(app.CheckCommand)
			if err == nil {
				// If check command succeeds rc 0, app is still running
				slog.Warn("App did not stop correctly", "app", app.Name, "host", app.Host)
			} else {
				// If check command fails rc != 0, app is not running
				slog.Info("App stopped successfully", "app", app.Name, "host", app.Host)
			}
		} else {
			slog.Info("[Dry-run] Assuming app stopped successfully", "app", app.Name, "host", app.Host)
		}

		o.mutex.Lock()
		o.appStates[app.Name] = false
		o.mutex.Unlock()
	}

	slog.Info("All applications stopped successfully")
	return nil
}

// monitorApps continuously monitors the running state of applications.
func (o *Orchestrator) monitorApps(ctx context.Context) {
	defer o.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Monitoring goroutine exiting due to context cancellation")
			return
		case <-ticker.C:
			o.mutex.Lock()
			for appName, started := range o.appStates {
				if !started {
					continue
				}

				var appConfig config.Application
				for _, app := range o.cfg.Environments[o.environment].Applications {
					if app.Name == appName {
						appConfig = app
						break
					}
				}

				client, err := o.sshFactory.GetClient(appConfig.Host, o.dryRun)
				if err != nil {
					slog.Error("Failed to get SSH client for monitoring", "host", appConfig.Host, "error", err)
					o.monitorChan <- fmt.Errorf("getting SSH client for host '%s': %w", appConfig.Host, err)
					o.mutex.Unlock()
					return
				}

				// Run the check command and evaluate exit status
				err = client.RunCommand(appConfig.CheckCommand)
				if err != nil {
					slog.Error("App check failed during monitoring", "app", appName, "host", appConfig.Host, "error", err)
					o.monitorChan <- fmt.Errorf("app '%s' on host '%s' failed during monitoring", appName, appConfig.Host)
					o.mutex.Unlock()
					return
				}
			}
			o.mutex.Unlock()
		}
	}
}

// rollback stops all started applications in reverse order.
func (o *Orchestrator) rollback() error {
	env := o.cfg.Environments[o.environment]
	slog.Info("Starting rollback process")

	for i := len(env.Applications) - 1; i >= 0; i-- {
		app := env.Applications[i]

		o.mutex.Lock()
		started := o.appStates[app.Name]
		o.mutex.Unlock()

		if !started {
			continue
		}

		client, err := o.sshFactory.GetClient(app.Host, o.dryRun)
		if err != nil {
			slog.Error("Failed to get SSH client for rollback", "host", app.Host, "error", err)
			continue
		}

		slog.Info("Stopping app during rollback", "app", app.Name, "host", app.Host)
		if err := client.RunCommand(app.StopCommand); err != nil {
			slog.Error("Failed to stop app during rollback", "app", app.Name, "host", app.Host, "error", err)
		} else {
			slog.Info("Stopped app during rollback", "app", app.Name, "host", app.Host)
		}

		o.mutex.Lock()
		o.appStates[app.Name] = false
		o.mutex.Unlock()
	}

	slog.Info("Rollback process completed")
	return fmt.Errorf("rollback completed due to failure")
}
