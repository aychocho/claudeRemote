package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func interactiveSession(client *ssh.Client, command string, env map[string]string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// Set environment variables (may require AcceptEnv on the server)
	for k, v := range env {
		if err := session.Setenv(k, v); err != nil {
			// Setenv requires server-side AcceptEnv; fall back to export prefix
			command = fmt.Sprintf("export %s=%s; %s", k, shellQuote(v), command)
			break
		}
	}

	// Get terminal dimensions
	fd := int(os.Stdin.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24
	}

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}

	// Wire up I/O
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	// Put local terminal in raw mode
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("make raw: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Forward SIGWINCH
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			w, h, err := term.GetSize(fd)
			if err == nil {
				session.WindowChange(h, w)
			}
		}
	}()
	defer signal.Stop(sigCh)

	// Copy stdin in background
	go func() {
		io.Copy(stdin, os.Stdin)
		stdin.Close()
	}()

	// Start remote command
	if err := session.Start(command); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	return session.Wait()
}
