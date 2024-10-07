package ssh

import (
	"fmt"
	"sync"
)

// MockSSHFactory is a mock implementation of the SSHFactory interface for testing.
type MockSSHFactory struct {
	clients map[string]Client
	mu      sync.Mutex
}

// NewMockSSHFactory creates a new MockSSHFactory instance.
func NewMockSSHFactory() *MockSSHFactory {
	return &MockSSHFactory{
		clients: make(map[string]Client),
	}
}

// GetClient retrieves a mock SSH client for the specified host.
func (f *MockSSHFactory) GetClient(host string, dryRun bool) (Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if client, exists := f.clients[host]; exists {
		return client, nil
	}
	return nil, fmt.Errorf("no mock client for host '%s'", host)
}

// AddMockClient adds a mock SSH client for a specific host.
func (f *MockSSHFactory) AddMockClient(host string, client Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients[host] = client
}

// CloseAll is a no-op for the mock factory.
func (f *MockSSHFactory) CloseAll() {
	// No-op for mocks
}
