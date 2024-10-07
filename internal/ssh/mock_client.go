package ssh

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// MockSSHClient is a mock implementation of the Client interface for testing.
type MockSSHClient struct {
	commandsRun map[string]error
	mu          sync.Mutex
	running     bool
	appStates   map[string]bool // Map of app names to whether they are started
}

// NewMockSSHClient creates a new MockSSHClient instance.
func NewMockSSHClient() *MockSSHClient {
	return &MockSSHClient{
		commandsRun: make(map[string]error),
		running:     true,
		appStates:   make(map[string]bool),
	}
}

// RunCommand simulates running a command on the remote host.
func (m *MockSSHClient) RunCommand(cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if a specific error is set for this command
	if err, exists := m.commandsRun[cmd]; exists {
		return err
	}

	// Extract the command and app name
	parts := strings.SplitN(cmd, "_", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid command format")
	}
	cmdName, appName := parts[0], parts[1]

	switch cmdName {
	case "start":
		m.appStates[appName] = true
		return nil // Assume start command always succeeds unless overridden
	case "stop":
		m.appStates[appName] = false
		return nil // Assume stop command always succeeds
	case "check":
		if m.appStates[appName] {
			return nil // App is running
		} else {
			return errors.New("app not running")
		}
	default:
		// For other commands, return success
		return nil
	}
}

// Close simulates closing the SSH connection.
func (m *MockSSHClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return errors.New("client already closed")
	}
	m.running = false
	return nil
}

// IsRunning returns whether the mock SSH client is running.
func (m *MockSSHClient) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// SetCommandResponse sets the response for a given command.
func (m *MockSSHClient) SetCommandResponse(cmd string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandsRun[cmd] = err
}

// SetAppState sets the running state of an app.
func (m *MockSSHClient) SetAppState(appName string, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appStates[appName] = running
}

// GetAppState returns the running state of an app.
func (m *MockSSHClient) GetAppState(appName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.appStates[appName]
}
