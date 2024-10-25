// orchestrator/orchestrator.go
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"orchid/internal/config"
	"orchid/internal/ssh"
)

const (
	defaultHealthCheckTimeout  = 60 * time.Second
	defaultHealthCheckInterval = 2 * time.Second
	defaultOperationTimeout    = 5 * time.Minute
	startWaitDuration          = 5 * time.Second
)

type Options struct {
	Config              *config.Config
	Environment         string
	Force               bool
	DryRun              bool
	Logger              *slog.Logger
	HealthCheckTimeout  time.Duration
	HealthCheckInterval time.Duration
	OperationTimeout    time.Duration
	HandleDeps          bool
	StopDeps            bool
}

type Orchestrator struct {
	cfg        *config.Config
	env        string
	force      bool
	dryRun     bool
	logger     *slog.Logger
	sshManager *ssh.Manager
	options    Options
}

func New(opts Options) (*Orchestrator, error) {
	if opts.HealthCheckTimeout == 0 {
		opts.HealthCheckTimeout = defaultHealthCheckTimeout
	}
	if opts.HealthCheckInterval == 0 {
		opts.HealthCheckInterval = defaultHealthCheckInterval
	}
	if opts.OperationTimeout == 0 {
		opts.OperationTimeout = defaultOperationTimeout
	}

	sshManager := ssh.NewManager(opts.Logger)

	return &Orchestrator{
		cfg:        opts.Config,
		env:        opts.Environment,
		force:      opts.Force,
		dryRun:     opts.DryRun,
		logger:     opts.Logger,
		sshManager: sshManager,
		options:    opts,
	}, nil
}

func (o *Orchestrator) Up() error {
	env, ok := o.cfg.Environments[o.env]
	if !ok {
		return fmt.Errorf("environment %s not found", o.env)
	}

	o.logger.Info("starting orchestration UP",
		slog.String("environment", o.env),
		slog.Bool("force", o.force),
		slog.Bool("dry_run", o.dryRun),
		slog.Bool("handle_deps", o.options.HandleDeps),
	)

	ctx, cancel := context.WithTimeout(context.Background(), o.options.OperationTimeout)
	defer cancel()

	for i, step := range env.Sequence {
		stepLogger := o.logger.With(
			slog.String("step", step.Name),
			slog.Int("step_number", i+1),
			slog.String("type", step.Type),
		)

		var err error

		switch step.Type {
		case "dependency", "application":
			err = o.handleUp(ctx, step, env, stepLogger)
		case "command":
			err = o.handleCommand(ctx, step, env, stepLogger)
		default:
			err = fmt.Errorf("unknown step type: %s", step.Type)
		}

		if err != nil {
			stepLogger.Error("step failed", slog.String("error", err.Error()))
			return o.handleFailure(ctx, env, i)
		}

		if step.Type == "application" || (step.Type == "dependency" && o.options.HandleDeps) {
			stepLogger.Info("waiting before health check", slog.Duration("duration", startWaitDuration))
			if !o.dryRun {
				time.Sleep(startWaitDuration)
				stepLogger.Info("performing health check")

				if err := o.performHealthCheck(ctx, step, env, stepLogger); err != nil {
					stepLogger.Error("health check failed", slog.String("error", err.Error()))
					return o.handleFailure(ctx, env, i)
				}
			}
		}
	}

	o.logger.Info("orchestration UP completed successfully")
	return nil
}

func (o *Orchestrator) Down() error {
	env, ok := o.cfg.Environments[o.env]
	if !ok {
		return fmt.Errorf("environment %s not found", o.env)
	}

	o.logger.Info("starting orchestration DOWN",
		slog.String("environment", o.env),
		slog.Bool("force", o.force),
		slog.Bool("dry_run", o.dryRun),
		slog.Bool("stop_deps", o.options.StopDeps),
	)

	ctx, cancel := context.WithTimeout(context.Background(), o.options.OperationTimeout)
	defer cancel()

	// Stop services in reverse order
	for i := len(env.Sequence) - 1; i >= 0; i-- {
		step := env.Sequence[i]
		stepLogger := o.logger.With(
			slog.String("step", step.Name),
			slog.Int("step_number", i+1),
			slog.String("type", step.Type),
		)

		var err error

		switch step.Type {
		case "dependency", "application":
			// For dependencies, respect the StopDeps flag
			if step.Type == "dependency" && !o.options.StopDeps {
				stepLogger.Info("skipping dependency stop", slog.String("dependency", step.Name))
				continue
			}
			err = o.handleDown(ctx, step, env, stepLogger)
		case "command":
			stepLogger.Info("skipping command in down")
		default:
			err = fmt.Errorf("unknown step type: %s", step.Type)
		}

		if err != nil {
			stepLogger.Error("step failed", slog.String("error", err.Error()))
			// Continue stopping other services despite the error
		}
	}

	o.logger.Info("orchestration DOWN completed")
	return nil
}

// handleUp manages the UP operation for both dependencies and applications
func (o *Orchestrator) handleUp(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	switch step.Type {
	case "application":
		return o.handleApplicationUp(ctx, step, env, logger)
	case "dependency":
		if o.options.HandleDeps {
			return o.handleDependencyUp(ctx, step, env, logger)
		} else {
			// HandleDeps is false: just verify dependencies are running
			return o.verifyDependencyRunning(ctx, step, env, logger)
		}
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// handleDown manages the DOWN operation for both dependencies and applications
func (o *Orchestrator) handleDown(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	switch step.Type {
	case "application":
		return o.handleApplicationDown(ctx, step, env, logger)
	case "dependency":
		if o.options.StopDeps {
			return o.handleDependencyDown(ctx, step, env, logger)
		} else {
			logger.Info("HandleDeps is false; skipping dependency stop")
			return nil
		}
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// handleApplicationUp manages the UP operation for applications
func (o *Orchestrator) handleApplicationUp(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	running, err := o.isServiceRunning(ctx, step, env, logger)
	if err != nil {
		return fmt.Errorf("failed to check application running state: %w", err)
	}

	if running {
		logger.Info("application is already running; skipping start")
		if err := o.stopService(ctx, step, env, logger); err != nil {
			return fmt.Errorf("failed to stop application: %w", err)
		}
		return nil
	}

	logger.Info("application is not running; starting", slog.String("service", step.Name))

	// Start the application
	if err := o.startService(ctx, step, env, logger); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	return nil
}

// handleDependencyUp manages the UP operation for dependencies when HandleDeps is true
func (o *Orchestrator) handleDependencyUp(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	running, err := o.isServiceRunning(ctx, step, env, logger)
	if err != nil {
		return fmt.Errorf("failed to check dependency running state: %w", err)
	}

	if running {
		logger.Info("dependency is already running; restarting", slog.String("service", step.Name))
		// Stop the dependency
		if err := o.stopService(ctx, step, env, logger); err != nil {
			return fmt.Errorf("failed to stop dependency: %w", err)
		}
	}

	// Start the dependency
	if err := o.startService(ctx, step, env, logger); err != nil {
		return fmt.Errorf("failed to start dependency: %w", err)
	}

	return nil
}

// verifyDependencyRunning checks if a dependency is running when HandleDeps is false
func (o *Orchestrator) verifyDependencyRunning(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	running, err := o.isServiceRunning(ctx, step, env, logger)
	if err != nil {
		return fmt.Errorf("failed to verify dependency running state: %w", err)
	}

	if !running {
		logger.Error("dependency is not running and HandleDeps is false", slog.String("service", step.Name))
		return fmt.Errorf("dependency %s is not running", step.Name)
	}

	logger.Info("dependency is running", slog.String("service", step.Name))
	return nil
}

// handleApplicationDown manages the DOWN operation for applications
func (o *Orchestrator) handleApplicationDown(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	// Stop the application
	if err := o.stopService(ctx, step, env, logger); err != nil {
		return fmt.Errorf("failed to stop application: %w", err)
	}

	return nil
}

// handleDependencyDown manages the DOWN operation for dependencies when StopDeps is true
func (o *Orchestrator) handleDependencyDown(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	// Stop the dependency
	if err := o.stopService(ctx, step, env, logger); err != nil {
		return fmt.Errorf("failed to stop dependency: %w", err)
	}

	return nil
}

func (o *Orchestrator) performHealthCheck(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	if o.dryRun {
		logger.Info("dry run - skipping health check")
		return nil
	}

	for _, hostName := range step.Hosts {
		host, ok := env.Hosts[hostName]
		if !ok {
			return fmt.Errorf("host %s not found in environment", hostName)
		}

		client, err := o.sshManager.GetClient(host, env.SSHDefaults)
		if err != nil {
			return fmt.Errorf("failed to get SSH client for host %s: %w", hostName, err)
		}

		output, err := client.Execute(ctx, step.Check)
		if err != nil {
			logger.Warn("health check failed",
				slog.String("host", hostName),
				slog.String("error", err.Error()),
				slog.String("output", output))
			return fmt.Errorf("health check command failed on host %s: %w", hostName, err)
		}

		logger.Info("health check passed", slog.String("host", hostName))
	}

	return nil
}

func (o *Orchestrator) handleFailure(ctx context.Context, env config.Environment, failedStepIndex int) error {
	o.logger.Info("initiating rollback due to failure")

	// Roll back services in reverse order up to the failed step
	for i := failedStepIndex - 1; i >= 0; i-- {
		step := env.Sequence[i]
		if step.Type != "command" {
			stepLogger := o.logger.With(
				slog.String("step", step.Name),
				slog.Int("step_number", i+1),
				slog.String("type", step.Type),
			)
			stepLogger.Info("rolling back service",
				slog.String("service", step.Name),
				slog.Int("step_number", i+1))

			if err := o.stopService(ctx, step, env, stepLogger); err != nil {
				stepLogger.Error("failed to stop service during rollback",
					slog.String("service", step.Name),
					slog.String("error", err.Error()))
				// Continue rolling back other services despite the error
			}
		}
	}

	return fmt.Errorf("orchestration failed at step %d", failedStepIndex+1)
}

func (o *Orchestrator) isServiceRunning(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) (bool, error) {
	if o.dryRun {
		logger.Info("dry run - setting service running check to true")
		return true, nil
	}

	for _, hostName := range step.Hosts {
		host, ok := env.Hosts[hostName]
		if !ok {
			return false, fmt.Errorf("host %s not found in environment", hostName)
		}

		client, err := o.sshManager.GetClient(host, env.SSHDefaults)
		if err != nil {
			return false, fmt.Errorf("failed to get SSH client for host %s: %w", hostName, err)
		}

		output, err := client.Execute(ctx, step.Check)
		if err != nil {
			logger.Debug("service check failed",
				slog.String("host", hostName),
				slog.String("error", err.Error()),
				slog.String("output", output))
			return false, nil
		}
	}

	return true, nil
}

func (o *Orchestrator) startService(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	if o.dryRun {
		logger.Info("dry run - would start service",
			slog.Any("hosts", step.Hosts),
			slog.String("start_command", step.Start))
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(step.Hosts))

	for _, hostName := range step.Hosts {
		host, ok := env.Hosts[hostName]
		if !ok {
			return fmt.Errorf("host %s not found in environment", hostName)
		}

		wg.Add(1)
		go func(h config.Host) {
			defer wg.Done()

			client, err := o.sshManager.GetClient(h, env.SSHDefaults)
			if err != nil {
				errCh <- fmt.Errorf("failed to get SSH client for host %s: %w", h.Hostname, err)
				return
			}

			output, err := client.Execute(ctx, step.Start)
			if err != nil {
				errCh <- fmt.Errorf("failed to start service on host %s: %w. Output: %s", h.Hostname, err, output)
				return
			}

			logger.Info("service start initiated",
				slog.String("host", h.Hostname),
				slog.String("service", step.Name))
		}(host)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to start service on some hosts: %v", errs)
	}

	return nil
}

func (o *Orchestrator) stopService(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	if o.dryRun {
		logger.Info("dry run - would stop service",
			slog.Any("hosts", step.Hosts),
			slog.String("stop_command", step.Stop))
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(step.Hosts))

	for _, hostName := range step.Hosts {
		host, ok := env.Hosts[hostName]
		if !ok {
			return fmt.Errorf("host %s not found in environment", hostName)
		}

		wg.Add(1)
		go func(h config.Host) {
			defer wg.Done()

			client, err := o.sshManager.GetClient(h, env.SSHDefaults)
			if err != nil {
				errCh <- fmt.Errorf("failed to get SSH client for host %s: %w", h.Hostname, err)
				return
			}

			output, err := client.Execute(ctx, step.Stop)
			if err != nil {
				errCh <- fmt.Errorf("failed to stop service on host %s: %w. Output: %s", h.Hostname, err, output)
				return
			}

			logger.Info("service stopped",
				slog.String("host", h.Hostname),
				slog.String("service", step.Name))
		}(host)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to stop service on some hosts: %v", errs)
	}

	return nil
}

func (o *Orchestrator) handleCommand(ctx context.Context, step config.Step, env config.Environment, logger *slog.Logger) error {
	if o.dryRun {
		logger.Info("dry run - would execute command",
			slog.Any("hosts", step.Hosts),
			slog.String("command", step.Run))
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(step.Hosts))

	for _, hostName := range step.Hosts {
		host, ok := env.Hosts[hostName]
		if !ok {
			return fmt.Errorf("host %s not found in environment", hostName)
		}

		wg.Add(1)
		go func(h config.Host) {
			defer wg.Done()

			client, err := o.sshManager.GetClient(h, env.SSHDefaults)
			if err != nil {
				errCh <- fmt.Errorf("failed to get SSH client for host %s: %w", h.Hostname, err)
				return
			}

			output, err := client.Execute(ctx, step.Run)
			if err != nil {
				errCh <- fmt.Errorf("failed to execute command on host %s: %w. Output: %s", h.Hostname, err, output)
				return
			}

			logger.Info("command executed",
				slog.String("host", h.Hostname),
				slog.String("command", step.Run))
		}(host)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to execute command on some hosts: %v", errs)
	}

	return nil
}
