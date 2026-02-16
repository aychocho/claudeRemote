package provision

import (
	"fmt"
	"os"

	"claude-remote/internal/sshutil"

	"golang.org/x/crypto/ssh"
)

func Run(client *ssh.Client, credsPath, apiKey string) error {
	fmt.Fprintf(os.Stderr, "Checking for Claude Code...\n")
	_, err := sshutil.RunCmd(client, "command -v claude")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Claude Code not found, installing...\n")
		if err := sshutil.RunCmdStream(client, "curl -fsSL https://cli.claude.ai/install.sh | sh"); err != nil {
			return fmt.Errorf("install claude: %w", err)
		}
		if _, err := sshutil.RunCmd(client, "command -v claude || ~/.local/bin/claude --version"); err != nil {
			return fmt.Errorf("claude not found after install: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Claude Code installed.\n")
	} else {
		fmt.Fprintf(os.Stderr, "Claude Code already installed.\n")
	}

	if apiKey != "" {
		fmt.Fprintf(os.Stderr, "Using API key for authentication.\n")
	} else {
		fmt.Fprintf(os.Stderr, "Uploading credentials...\n")
		if err := sshutil.UploadFile(client, credsPath, "~/.claude/.credentials.json"); err != nil {
			return fmt.Errorf("upload credentials: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Credentials uploaded.\n")
	}

	return nil
}
