package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

func sshConnect(addr, user, keyPath string) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	if am, err := agentAuthMethod(); err == nil {
		authMethods = append(authMethods, am)
	}

	// Add key file auth if provided
	if keyPath != "" {
		if am, err := loadKey(keyPath); err == nil {
			authMethods = append(authMethods, am)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: could not load key %s: %v\n", keyPath, err)
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no auth methods available (no SSH agent and no key provided)")
	}

	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("host key verification setup: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	return ssh.Dial("tcp", addr, config)
}

func hostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	knownHostsFile := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsFile); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s not found, accepting all host keys\n", knownHostsFile)
		return ssh.InsecureIgnoreHostKey(), nil
	}

	return knownhosts.New(knownHostsFile)
}

func agentAuthMethod() (ssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

func loadKey(path string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}
