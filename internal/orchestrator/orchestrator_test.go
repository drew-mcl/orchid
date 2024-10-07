// internal/orchestrator/orchestrator_test.go
package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"orchid/internal/config"
	"orchid/internal/ssh"
)

func TestOrchestrator_BringUp_FailureDuringMonitoring(t *testing.T) {
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

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Simulate app1 failing during monitoring after it starts
	go func() {
		time.Sleep(3 * time.Second)            // Wait for app1 to start and monitoring to begin
		mockClient1.SetAppState("app1", false) // Simulate app1 crash
	}()

	ctx := context.Background()
	err = o.BringUp(ctx)
	if err == nil {
		t.Fatalf("BringUp should have failed due to app1 failure during monitoring")
	}

	// Verify that rollback was performed (app1 should be stopped)
	if o.appStates["app1"] {
		t.Fatalf("App1 should have been rolled back and stopped")
	}

	// Verify that app2 was not started
	if o.appStates["app2"] {
		t.Fatalf("App2 should not have been started due to cancellation")
	}

	// Wait for monitoring goroutine to exit
	o.wg.Wait()
}

func TestOrchestrator_BringUp_Success(t *testing.T) {
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

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = o.BringUp(ctx)
	if err != nil {
		t.Fatalf("BringUp failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !o.appStates["app1"] {
		t.Fatalf("App1 should be marked as started")
	}

	cancel()
	o.wg.Wait()
}

func TestOrchestrator_BringUp_FailureAndRollback(t *testing.T) {
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

	// Simulate successful start and check for app1
	// For app2, starting fails
	mockClient2.SetCommandResponse("start_app2", errors.New("failed to start app2"))

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = o.BringUp(ctx)
	if err == nil {
		t.Fatalf("BringUp should have failed due to app2 start failure")
	}

	// Verify that rollback was performed (app1 should be stopped)
	if o.appStates["app1"] {
		t.Fatalf("App1 should have been rolled back and stopped")
	}

	o.wg.Wait()
}

func TestOrchestrator_BringDown_Success(t *testing.T) {
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

	mockClient.RunCommand("start_app1")

	// Mark the app as started in the orchestrator's state
	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
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
}

func TestOrchestrator_Rollback(t *testing.T) {
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

	// Start both apps to simulate them running
	mockClient1.RunCommand("start_app1")
	mockClient2.RunCommand("start_app2")

	// Mark both apps as started
	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
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
}

func TestOrchestrator_BringUp_ContextCancellation(t *testing.T) {
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

	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	// Run BringUp
	err = o.BringUp(ctx)
	if err == nil {
		t.Fatalf("BringUp should have failed due to context cancellation")
	}

	// Verify that the app was not started
	if o.appStates["app1"] {
		t.Fatalf("App1 should not have been started due to cancellation")
	}

	o.wg.Wait()
}

func TestOrchestrator_BringDown_ContextCancellation(t *testing.T) {
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

	// Start the app to simulate it running
	mockClient.RunCommand("start_app1")

	// Mark the app as started in the orchestrator's state
	o, err := NewOrchestrator(cfg, mockFactory, "test_env", nil, false)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	o.appStates["app1"] = true

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	// Run BringDown
	err = o.BringDown(ctx)
	if err == nil {
		t.Fatalf("BringDown should have failed due to context cancellation")
	}

	// Verify that the app state remains unchanged
	if !o.appStates["app1"] {
		t.Fatalf("App1 state should remain as started due to cancellation")
	}
}
