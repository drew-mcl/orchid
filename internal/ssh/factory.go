package ssh

import (
	"sync"

	"log/slog"
)

// SSHFactory defines the interface for SSH connection factories.
type SSHFactory interface {
	GetClient(host string, dryRun bool) (Client, error)
	CloseAll()
}

// RealSSHFactory manages real SSH connections with pooling and reuse.
type RealSSHFactory struct {
	user           string
	keyPath        string
	knownHostsPath string
	port           int
	clients        map[string]Client
	mu             sync.Mutex
}

// NewRealSSHFactory creates a new RealSSHFactory.
func NewRealSSHFactory(user, keyPath, knownHostsPath string, port int) *RealSSHFactory {
	return &RealSSHFactory{
		user:           user,
		keyPath:        keyPath,
		knownHostsPath: knownHostsPath,
		port:           port,
		clients:        make(map[string]Client),
	}
}

// GetClient retrieves an existing SSH client or establishes a new one.
func (f *RealSSHFactory) GetClient(host string, dryRun bool) (Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if client, exists := f.clients[host]; exists && client.IsRunning() {
		return client, nil
	}

	client, err := NewSSHClient(f.user, f.keyPath, host, f.port, f.knownHostsPath, dryRun)
	if err != nil {
		return nil, err
	}

	f.clients[host] = client
	return client, nil
}

// CloseAll closes all SSH connections managed by the factory.
func (f *RealSSHFactory) CloseAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for host, client := range f.clients {
		if err := client.Close(); err != nil {
			slog.Warn("Failed to close SSH client", "host", host, "error", err)
		}
	}
}
