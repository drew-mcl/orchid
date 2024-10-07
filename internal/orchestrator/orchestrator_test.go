package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orchid/internal/config"
	"orchid/internal/ssh"
)

// Helper function to set environment variables for tests
func setTestEnv(t *testing.T, pipelineID, commitRef, projectName, environment string) {
	t.Helper()
	os.Setenv("CI_PIPELINE_ID", pipelineID)
	os.Setenv("CI_COMMIT_REF_NAME", commitRef)
	os.Setenv("CI_PROJECT_NAME", projectName)
	os.Setenv("CI_ENVIRONMENT_NAME", environment)
}

func TestOrchestrator_BringUp_FailureDuringMonitoring(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")

	// Set environment variables for FlagManager
	setTestEnv(t, "12345", "main", "orchid_project", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
					{
						Name:          "app2",
						Host:          "host2",
						StartCommand:  "start_app2",
						StopCommand:   "stop_app2",
						CheckCommand:  "check_app2",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient1 := ssh.NewMockSSHClient()
	mockClient2 := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient1)
	mockFactory.AddMockClient("host2", mockClient2)

	// Initialize FlagManager
	flagManager := NewFlagManager(flagPath, "test_env")

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Simulate app1 failing during monitoring
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for 1 second, within BringUp's 2-second monitoring window
		time.Sleep(1 * time.Second)
		mockClient1.SetCommandResponse("check_app1", errors.New("app not running"))
	}()

	ctx := context.Background()
	err = o.BringUp(ctx)
	wg.Wait() // Ensure the simulated failure has occurred

	if err == nil {
		t.Fatalf("BringUp should have failed due to app1 failure during monitoring")
	}

	// Verify that rollback was performed (app1 and app2 should be stopped)
	if o.appStates["app1"] {
		t.Fatalf("App1 should have been rolled back and stopped")
	}
	if o.appStates["app2"] {
		t.Fatalf("App2 should have been rolled back and stopped")
	}

	// Verify that the flag was released after rollback
	if _, err := os.Stat(o.flagManager.flagPath); !os.IsNotExist(err) {
		t.Fatalf("Flag file should be removed after rollback")
	}
}

func TestOrchestrator_BringUp_Success(t *testing.T) {
	// Setup temporary directory for the flag file
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")
	flagManager := NewFlagManager(flagPath, "test_env")

	// Set environment variables for FlagManager
	setTestEnv(t, "67890", "develop", "orchid_project2", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient)

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Start BringUp in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := o.BringUp(context.Background())
		if err != nil {
			t.Errorf("BringUp failed: %v", err)
		}
	}()

	// Wait for BringUp to complete
	wg.Wait()

	// Verify that the app was started
	if !o.appStates["app1"] {
		t.Fatalf("App1 should be marked as started")
	}

	// Verify that the flag is acquired (flag file exists)
	if _, err := os.Stat(o.flagManager.flagPath); os.IsNotExist(err) {
		t.Fatalf("Flag file should exist after BringUp")
	}

	// Clean up by bringing down
	err = o.BringDown(context.Background())
	if err != nil {
		t.Fatalf("BringDown failed: %v", err)
	}

	// Verify that the flag was released
	if _, err := os.Stat(o.flagManager.flagPath); !os.IsNotExist(err) {
		t.Fatalf("Flag file should be removed after BringDown")
	}
}

func TestOrchestrator_BringUp_FailureAndRollback(t *testing.T) {
	// Setup temporary directory for the flag file
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")
	flagManager := NewFlagManager(flagPath, "test_env")

	// Set environment variables for FlagManager
	setTestEnv(t, "54321", "feature", "orchid_project3", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
					{
						Name:          "app2",
						Host:          "host2",
						StartCommand:  "start_app2",
						StopCommand:   "stop_app2",
						CheckCommand:  "check_app2",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient1 := ssh.NewMockSSHClient()
	mockClient2 := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient1)
	mockFactory.AddMockClient("host2", mockClient2)

	// Simulate app2 failing to start
	mockClient2.SetCommandResponse("start_app2", errors.New("failed to start app2"))

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	ctx := context.Background()
	err = o.BringUp(ctx)
	if err == nil {
		t.Fatalf("BringUp should have failed due to app2 start failure")
	}

	// Verify that rollback was performed (app1 and app2 should be stopped)
	if o.appStates["app1"] {
		t.Fatalf("App1 should have been rolled back and stopped")
	}
	if o.appStates["app2"] {
		t.Fatalf("App2 should have been rolled back and stopped")
	}

	// Verify that the flag was released after rollback
	if _, err := os.Stat(o.flagManager.flagPath); !os.IsNotExist(err) {
		t.Fatalf("Flag file should be removed after rollback")
	}
}

func TestOrchestrator_BringDown_Success(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")

	// Set environment variables for FlagManager
	setTestEnv(t, "11223", "staging", "orchid_project4", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient)

	// Initialize FlagManager and acquire the flag
	flagManager := NewFlagManager(flagPath, "test_env")
	err := flagManager.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire flag: %v", err)
	}

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	o.appStates["app1"] = true

	ctx := context.Background()
	err = o.BringDown(ctx)
	if err != nil {
		t.Fatalf("BringDown failed: %v", err)
	}

	// Verify that the app was stopped
	if o.appStates["app1"] {
		t.Fatalf("App1 should be marked as stopped")
	}

	// Verify that the flag was released
	if _, err := os.Stat(flagPath); !os.IsNotExist(err) {
		t.Fatalf("Flag file should be removed after BringDown")
	}
}

func TestOrchestrator_Rollback(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")

	// Set environment variables for FlagManager
	setTestEnv(t, "33445", "production", "orchid_project5", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
					{
						Name:          "app2",
						Host:          "host2",
						StartCommand:  "start_app2",
						StopCommand:   "stop_app2",
						CheckCommand:  "check_app2",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient1 := ssh.NewMockSSHClient()
	mockClient2 := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient1)
	mockFactory.AddMockClient("host2", mockClient2)

	// Initialize FlagManager and acquire the flag
	flagManager := NewFlagManager(flagPath, "test_env")
	err := flagManager.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire flag: %v", err)
	}

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	o.appStates["app1"] = true
	o.appStates["app2"] = true

	err = o.rollback()
	if err == nil {
		t.Fatalf("Rollback should have returned an error indicating failure")
	}

	// Verify that both apps were stopped
	if o.appStates["app1"] {
		t.Fatalf("App1 should have been stopped during rollback")
	}
	if o.appStates["app2"] {
		t.Fatalf("App2 should have been stopped during rollback")
	}

	// Verify that the flag was released after rollback
	if _, err := os.Stat(o.flagManager.flagPath); !os.IsNotExist(err) {
		t.Fatalf("Flag file should be removed after rollback")
	}
}

func TestOrchestrator_BringUp_ContextCancellation(t *testing.T) {
	// Setup temporary directory for the flag file
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")
	flagManager := NewFlagManager(flagPath, "test_env")

	// Set environment variables for FlagManager
	setTestEnv(t, "55667", "qa", "orchid_project6", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient)

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Start BringUp with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = o.BringUp(ctx)
	if err == nil {
		t.Fatalf("BringUp should have failed due to context cancellation")
	}

	// Verify that the app was not started
	if o.appStates["app1"] {
		t.Fatalf("App1 should not have been started due to cancellation")
	}

	// Verify that the flag was not acquired
	if _, err := os.Stat(o.flagManager.flagPath); !os.IsNotExist(err) {
		t.Fatalf("Flag file should not exist after failed BringUp due to cancellation")
	}
}

func TestOrchestrator_BringDown_ContextCancellation(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_env.flag")

	// Set environment variables for FlagManager
	setTestEnv(t, "77889", "dev", "orchid_project7", "test_env")
	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	cfg := &config.Config{
		Environments: map[string]config.Environment{
			"test_env": {
				Applications: []config.Application{
					{
						Name:          "app1",
						Host:          "host1",
						StartCommand:  "start_app1",
						StopCommand:   "stop_app1",
						CheckCommand:  "check_app1",
						CheckInterval: 1,
					},
				},
			},
		},
	}

	mockFactory := ssh.NewMockSSHFactory()
	mockClient := ssh.NewMockSSHClient()
	mockFactory.AddMockClient("host1", mockClient)

	// Initialize FlagManager and acquire the flag
	flagManager := NewFlagManager(flagPath, "test_env")
	err := flagManager.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire flag: %v", err)
	}

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", flagManager, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	o.appStates["app1"] = true

	// Start BringDown with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = o.BringDown(ctx)
	if err == nil {
		t.Fatalf("BringDown should have failed due to context cancellation")
	}

	// Verify that the app state remains unchanged
	if !o.appStates["app1"] {
		t.Fatalf("App1 state should remain as started due to cancellation")
	}

	// Verify that the flag is still acquired (since BringDown was canceled before release)
	if _, err := os.Stat(o.flagManager.flagPath); os.IsNotExist(err) {
		t.Fatalf("Flag file should still exist after canceled BringDown")
	}
}
