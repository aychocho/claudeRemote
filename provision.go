package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

func runCmd(client *ssh.Client, cmd string) (string, error) {
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

func runCmdStream(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdout = os.Stderr
	session.Stderr = os.Stderr

	return session.Run(cmd)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func uploadFile(client *ssh.Client, localPath, remotePath string) error {
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

	quoted := shellQuote(remotePath)
	cmd := fmt.Sprintf("mkdir -p \"$(dirname %s)\" && cat > %s && chmod 600 %s", quoted, quoted, quoted)
	return session.Run(cmd)
}

func provision(client *ssh.Client, cfg config) error {
	// Check if claude is installed
	fmt.Fprintf(os.Stderr, "Checking for Claude Code...\n")
	_, err := runCmd(client, "command -v claude")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Claude Code not found, installing...\n")
		if err := runCmdStream(client, "curl -fsSL https://cli.claude.ai/install.sh | sh"); err != nil {
			return fmt.Errorf("install claude: %w", err)
		}
		// Verify installation
		if _, err := runCmd(client, "command -v claude || ~/.local/bin/claude --version"); err != nil {
			return fmt.Errorf("claude not found after install: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Claude Code installed.\n")
	} else {
		fmt.Fprintf(os.Stderr, "Claude Code already installed.\n")
	}

	// Copy auth
	if cfg.apiKey != "" {
		fmt.Fprintf(os.Stderr, "Using API key for authentication.\n")
	} else {
		fmt.Fprintf(os.Stderr, "Uploading credentials...\n")
		if err := uploadFile(client, cfg.credsPath, "~/.claude/.credentials.json"); err != nil {
			return fmt.Errorf("upload credentials: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Credentials uploaded.\n")
	}

	return nil
}
