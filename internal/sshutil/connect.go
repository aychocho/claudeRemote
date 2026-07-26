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

	// Collect every public key (the explicit -i key first, then any agent keys)
	// into a SINGLE publickey auth method. The x/crypto/ssh client dedups auth
	// methods by name, so registering two separate "publickey" methods means only
	// the first is ever attempted — an empty/unhelpful agent would then shadow the
	// -i key and auth would fail even though the key is authorized.
	var signers []ssh.Signer

	if keyPath != "" {
		if s, err := loadKeySigner(keyPath); err == nil {
			signers = append(signers, s)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: could not load key %s: %v\n", keyPath, err)
		}
	}

	if agentSigs, err := agentSigners(); err == nil {
		signers = append(signers, agentSigs...)
	}

	if len(signers) > 0 {
		authMethods = append(authMethods, ssh.PublicKeys(signers...))
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

	// Create the file if it doesn't exist yet.
	if _, err := os.Stat(knownHostsFile); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(knownHostsFile), 0700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(knownHostsFile, nil, 0600); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	cb, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, err
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		// If the error is anything other than "key is unknown", reject.
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) || len(keyErr.Want) > 0 {
			// len(Want) > 0 means the host exists but the key changed (MITM warning).
			return err
		}

		// Prompt user to accept the unknown host key.
		fmt.Fprintf(os.Stderr, "The authenticity of host '%s' can't be established.\n", hostname)
		fmt.Fprintf(os.Stderr, "%s key fingerprint is %s.\n",
			key.Type(), ssh.FingerprintSHA256(key))
		fmt.Fprintf(os.Stderr, "Are you sure you want to continue connecting (yes/no)? ")

		var answer string
		if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		if answer != "yes" {
			return fmt.Errorf("host key verification rejected by user")
		}

		// Append the key to known_hosts.
		line := knownhosts.Line([]string{knownhosts.Normalize(remote.String())}, key)
		f, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to open known_hosts: %w", err)
		}
		defer f.Close()
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("failed to write known_hosts: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Warning: Permanently added '%s' to the list of known hosts.\n",
			knownhosts.Normalize(remote.String()))
		return nil
	}, nil
}

func agentSigners() ([]ssh.Signer, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to SSH agent: %w", err)
	}

	// The connection is intentionally left open: the returned signers sign via
	// the agent during authentication and need it alive. It is reclaimed when the
	// process exits (this is a short-lived CLI).
	agentClient := agent.NewClient(conn)
	signers, err := agentClient.Signers()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH agent not responding: %w", err)
	}
	return signers, nil
}

func loadKeySigner(path string) (ssh.Signer, error) {
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

	return signer, nil
}
