package sshutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

func RunCmd(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, "%s", stderr.String())
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func RunCmdStream(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdout = os.Stderr
	session.Stderr = os.Stderr

	return session.Run(cmd)
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func UploadFile(client *ssh.Client, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdin = io.NopCloser(bytes.NewReader(data))
	session.Stderr = os.Stderr

	quoted := ShellQuote(remotePath)
	cmd := fmt.Sprintf("mkdir -p \"$(dirname %s)\" && cat > %s && chmod 600 %s", quoted, quoted, quoted)
	return session.Run(cmd)
}
