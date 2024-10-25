package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log/slog"
	"sync"

	"orchid/internal/config"

	"golang.org/x/crypto/ssh"
)

type Manager struct {
	logger  *slog.Logger
	clients map[string]*Client
	mu      sync.RWMutex
}

type Client struct {
	client *ssh.Client
	logger *slog.Logger
}

func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger:  logger,
		clients: make(map[string]*Client),
	}
}

func (m *Manager) GetClient(host config.Host, defaults config.SSHDefaults) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use host.Hostname as the key in the clients map
	clientKey := host.Hostname
	if client, ok := m.clients[clientKey]; ok {
		return client, nil
	}

	// Determine SSH user and key
	user := host.SSHUser
	if user == "" {
		user = defaults.User
	}

	keyPath := host.SSHKey
	if keyPath == "" {
		keyPath = defaults.Key
	}

	// Read private key file
	keyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key '%s': %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key '%s': %w", keyPath, err)
	}

	// Set default timeout if not specified
	timeout := defaults.Timeout
	if timeout == 0 {
		timeout = 30 // Default timeout of 30 seconds
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use proper host key verification
		Timeout:         timeout,
	}

	clientConn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host.Hostname), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH on host %s: %w", host.Hostname, err)
	}

	sshClient := &Client{
		client: clientConn,
		logger: m.logger.With(slog.String("host", host.Hostname)),
	}

	m.clients[clientKey] = sshClient
	return sshClient, nil
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		if err := client.client.Close(); err != nil {
			m.logger.Error("failed to close SSH connection",
				slog.String("error", err.Error()))
		}
	}
	m.clients = make(map[string]*Client)
}

func (c *Client) Execute(ctx context.Context, cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Handle context cancellation
	done := make(chan error, 1)
	var outputBuf bytes.Buffer

	session.Stdout = &outputBuf
	session.Stderr = &outputBuf

	go func() {
		err := session.Run(cmd)
		done <- err
	}()

	select {
	case <-ctx.Done():
		if err := session.Signal(ssh.SIGINT); err != nil {
			c.logger.Warn("failed to send interrupt signal to remote process", slog.String("error", err.Error()))
		}
		return "", ctx.Err()
	case err := <-done:
		output := outputBuf.String()
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				// Non-zero exit status
				return output, fmt.Errorf("command exited with status %d: %w", exitErr.ExitStatus(), err)
			}
			return output, fmt.Errorf("failed to run command: %w", err)
		}
		return output, nil
	}
}
