package sshutil

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

func Connect(addr, user, keyPath string) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	if am, err := agentAuthMethod(); err == nil {
		authMethods = append(authMethods, am)
	}

	if keyPath != "" {
		if am, err := loadKey(keyPath); err == nil {
			authMethods = append(authMethods, am)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: could not load key %s: %v\n", keyPath, err)
		}
	}

	authMethods = append(authMethods, ssh.KeyboardInteractive(
		func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i, q := range questions {
				fmt.Fprintf(os.Stderr, "%s", q)
				if echos[i] {
					var line string
					if _, err := fmt.Fscanln(os.Stdin, &line); err != nil {
						return nil, err
					}
					answers[i] = line
				} else {
					pw, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Fprintln(os.Stderr)
					if err != nil {
						return nil, err
					}
					answers[i] = string(pw)
				}
			}
			return answers, nil
		},
	))

	authMethods = append(authMethods, ssh.PasswordCallback(func() (string, error) {
		fmt.Fprintf(os.Stderr, "Password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(pw), nil
	}))

	hostKeyCb, err := hostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("host key verification setup: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCb,
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
		return nil, fmt.Errorf("%s not found — run 'ssh-keyscan <host> >> %s' to add the host key first", knownHostsFile, knownHostsFile)
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
	// Verify the agent is alive; a stale socket will connect but fail on use.
	if _, err := agentClient.List(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH agent not responding: %w", err)
	}
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

func loadKey(path string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		var missingErr *ssh.PassphraseMissingError
		if errors.As(err, &missingErr) {
			fmt.Fprintf(os.Stderr, "Enter passphrase for key %s: ", path)
			passphrase, readErr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if readErr != nil {
				return nil, fmt.Errorf("read passphrase: %w", readErr)
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(data, passphrase)
		}
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
	}

	return ssh.PublicKeys(signer), nil
}
