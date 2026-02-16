package terminal

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"claude-remote/internal/sshutil"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func InteractiveSession(client *ssh.Client, command string, env map[string]string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	for k, v := range env {
		if err := session.Setenv(k, v); err != nil {
			command = fmt.Sprintf("export %s=%s; %s", k, sshutil.ShellQuote(v), command)
			break
		}
	}

	fd := int(os.Stdin.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("make raw: %w", err)
	}
	defer term.Restore(fd, oldState)

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

	go func() {
		io.Copy(stdin, os.Stdin)
		stdin.Close()
	}()

	if err := session.Start(command); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	return session.Wait()
}
