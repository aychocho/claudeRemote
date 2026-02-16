package provision

import (
	"fmt"
	"os"

	"claude-remote/internal/sshutil"

	"golang.org/x/crypto/ssh"
)

// findClaude returns the absolute path to the claude binary on the remote host.
func findClaude(client *ssh.Client) (string, error) {
	paths := []string{
		"command -v claude",
		"echo $HOME/.claude/bin/claude",
		"echo $HOME/.local/bin/claude",
	}
	for _, cmd := range paths {
		out, err := sshutil.RunCmd(client, cmd)
		if err != nil || out == "" {
			continue
		}
		// Verify the resolved path actually exists and is executable.
		if _, err := sshutil.RunCmd(client, out+" --version 2>/dev/null"); err == nil {
			return out, nil
		}
	}
	return "", fmt.Errorf("claude binary not found")
}

// Run installs Claude Code if needed, uploads credentials, and returns the
// resolved path to the claude binary.
// Directories/files that are large, machine-specific, or unnecessary on the remote.
var claudeDirExcludes = []string{
	"debug",
	"cache",
	"paste-cache",
	"file-history",
	"history.jsonl",
	"session-env",
	"shell-snapshots",
	"stats-cache.json",
	"telemetry",
	"todos",
	"plans",
	"tasks",
	"teams",
	"projects",
}

func Run(client *ssh.Client, claudeDir, apiKey string) (string, error) {
	fmt.Fprintf(os.Stderr, "Checking for Claude Code...\n")
	claudePath, err := findClaude(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Claude Code not found, installing...\n")
		if err := sshutil.RunCmdStream(client, "curl -fsSL https://claude.ai/install.sh | sh"); err != nil {
			return "", fmt.Errorf("install claude: %w", err)
		}
		claudePath, err = findClaude(client)
		if err != nil {
			return "", fmt.Errorf("claude not found after install: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Claude Code installed.\n")
	} else {
		fmt.Fprintf(os.Stderr, "Claude Code already installed.\n")
	}

	if apiKey != "" {
		fmt.Fprintf(os.Stderr, "Using API key for authentication.\n")
	} else {
		fmt.Fprintf(os.Stderr, "Uploading ~/.claude to remote host...\n")
		if err := sshutil.UploadDir(client, claudeDir, "$HOME/.claude", claudeDirExcludes); err != nil {
			return "", fmt.Errorf("upload .claude directory: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Credentials synced.\n")
	}

	// Mark onboarding as complete so Claude Code doesn't show the first-run wizard.
	// Use a python/node one-liner to merge into existing file, or create it.
	_, _ = sshutil.RunCmd(client, `
		f="$HOME/.claude.json";
		if [ -f "$f" ]; then
			tmp=$(mktemp) && python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
d['hasCompletedOnboarding']=True
json.dump(d,open(sys.argv[1],'w'),indent=2)
" "$f" 2>/dev/null || true
		else
			echo '{"hasCompletedOnboarding":true}' > "$f"
		fi`)

	return claudePath, nil
}
