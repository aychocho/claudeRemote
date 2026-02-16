package sshutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// UploadDir tars a local directory and extracts it at remoteDir on the remote host.
// excludes are passed as --exclude flags to tar.
func UploadDir(client *ssh.Client, localDir, remoteDir string, excludes []string) error {
	remoteHome, err := RunCmd(client, "echo $HOME")
	if err != nil {
		return fmt.Errorf("resolve remote HOME: %w", err)
	}
	if strings.HasPrefix(remoteDir, "$HOME") {
		remoteDir = remoteHome + remoteDir[len("$HOME"):]
	} else if strings.HasPrefix(remoteDir, "~") {
		remoteDir = remoteHome + remoteDir[len("~"):]
	}

	// Build local tar command.
	tarArgs := []string{"-cf", "-", "-C", localDir}
	for _, ex := range excludes {
		tarArgs = append(tarArgs, "--exclude", ex)
	}
	tarArgs = append(tarArgs, ".")
	tarCmd := exec.Command("tar", tarArgs...)

	tarOut, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("tar stdout pipe: %w", err)
	}
	tarCmd.Stderr = os.Stderr

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("start local tar: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		tarCmd.Process.Kill()
		tarCmd.Wait()
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdin = tarOut
	session.Stderr = os.Stderr

	quoted := ShellQuote(remoteDir)
	remoteCmd := fmt.Sprintf("mkdir -p %s && tar -xf - -C %s", quoted, quoted)
	if err := session.Run(remoteCmd); err != nil {
		tarCmd.Process.Kill()
		tarCmd.Wait()
		return fmt.Errorf("remote extract: %w", err)
	}

	return tarCmd.Wait()
}

func UploadFile(client *ssh.Client, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	// Resolve $HOME or ~ on the remote side before quoting, since
	// ShellQuote uses single quotes which prevent variable expansion.
	if strings.HasPrefix(remotePath, "$HOME") {
		resolved, err := RunCmd(client, "echo $HOME")
		if err != nil {
			return fmt.Errorf("resolve remote HOME: %w", err)
		}
		remotePath = resolved + remotePath[len("$HOME"):]
	} else if strings.HasPrefix(remotePath, "~") {
		resolved, err := RunCmd(client, "echo $HOME")
		if err != nil {
			return fmt.Errorf("resolve remote HOME: %w", err)
		}
		remotePath = resolved + remotePath[len("~"):]
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
