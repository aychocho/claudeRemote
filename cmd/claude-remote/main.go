package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"claude-remote/internal/provision"
	"claude-remote/internal/sshutil"
	"claude-remote/internal/terminal"
)

var version = "dev"

func main() {
	keyPath := flag.String("i", "", "SSH private key path")
	port := flag.String("p", "22", "SSH port")
	claudeDir := flag.String("c", "", "Path to local .claude config directory (e.g. ~/.claude)")
	apiKey := flag.String("k", "", "ANTHROPIC_API_KEY (or set $ANTHROPIC_API_KEY)")
	showVersion := flag.Bool("v", false, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claudeRemote [-i keypath] [-p port] [-c claudedir] [-k apikey] user@host\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("claudeRemote %s\n", version)
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	portNum, err := strconv.Atoi(*port)
	if err != nil || portNum < 1 || portNum > 65535 {
		fmt.Fprintf(os.Stderr, "Error: invalid port %q\n", *port)
		os.Exit(1)
	}

	target := flag.Arg(0)
	user, host, ok := parseTarget(target)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: invalid target %q, expected user@host\n", target)
		os.Exit(1)
	}

	// Pick up API key from environment if not passed via flag.
	if *apiKey == "" {
		*apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if *claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
			os.Exit(1)
		}
		*claudeDir = home + "/.claude"
	}

	addr := host + ":" + *port
	fmt.Fprintf(os.Stderr, "Connecting to %s@%s:%s...\n", user, host, *port)

	client, err := sshutil.Connect(addr, user, *keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Fprintf(os.Stderr, "Connected.\n")

	claudePath, err := provision.Run(client, *claudeDir, *apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Provisioning failed: %v\n", err)
		os.Exit(1)
	}

	var env map[string]string
	if *apiKey != "" {
		env = map[string]string{"ANTHROPIC_API_KEY": *apiKey}
	}

	sessionErr := terminal.InteractiveSession(client, sshutil.ShellQuote(claudePath), env)

	fmt.Fprintf(os.Stderr, "Cleaning up remote ~/.claude...\n")
	if _, err := sshutil.RunCmd(client, "rm -rf \"$HOME/.claude\""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove remote ~/.claude: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Remote ~/.claude removed.\n")
	}

	if sessionErr != nil {
		fmt.Fprintf(os.Stderr, "Session error: %v\n", sessionErr)
		os.Exit(1)
	}
}

func parseTarget(target string) (user, host string, ok bool) {
	parts := strings.SplitN(target, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
