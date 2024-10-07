// internal/ssh/client.go
package ssh

import (
	"fmt"
	"os"
	"time"

	"log/slog"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client defines the interface for SSH clients.
type Client interface {
	RunCommand(cmd string) error
	Close() error
	IsRunning() bool
}

// SSHClient is the real SSH client implementing the Client interface.
type SSHClient struct {
	client  *ssh.Client
	running bool
	dryRun  bool
}

// NewSSHClient establishes a new SSH connection or initializes a dry-run client.
func NewSSHClient(user, keyPath, host string, port int, knownHostsPath string, dryRun bool) (*SSHClient, error) {
	if dryRun {
		// In dry-run mode, return a client without establishing a connection
		return &SSHClient{running: true, dryRun: true}, nil
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		slog.Error("Unable to read SSH key", "error", err)
		return nil, fmt.Errorf("reading SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		slog.Error("Unable to parse SSH key", "error", err)
		return nil, fmt.Errorf("parsing SSH key: %w", err)
	}

	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		slog.Warn("Failed to load known_hosts, falling back to InsecureIgnoreHostKey", "error", err)
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		slog.Error("Failed to dial SSH", "address", address, "error", err)
		return nil, fmt.Errorf("dialing SSH: %w", err)
	}

	return &SSHClient{client: client, running: true, dryRun: false}, nil
}

// RunCommand executes a command on the remote host.
func (s *SSHClient) RunCommand(cmd string) error {
	if s.dryRun {
		slog.Info("[Dry-run] Would execute SSH command", "command", cmd)
		return nil
	}

	session, err := s.client.NewSession()
	if err != nil {
		slog.Error("Failed to create SSH session", "error", err)
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	if err := session.Run(cmd); err != nil {
		slog.Error("Failed to run command", "command", cmd, "error", err)
		return fmt.Errorf("running command '%s': %w", cmd, err)
	}

	return nil
}

// Close closes the SSH connection.
func (s *SSHClient) Close() error {
	if s.dryRun {
		slog.Info("[Dry-run] Would close SSH client")
		return nil
	}

	if err := s.client.Close(); err != nil {
		slog.Error("Failed to close SSH client", "error", err)
		return fmt.Errorf("closing SSH client: %w", err)
	}
	s.running = false
	return nil
}

// IsRunning checks if the SSH client is still running.
func (s *SSHClient) IsRunning() bool {
	return s.running
}
