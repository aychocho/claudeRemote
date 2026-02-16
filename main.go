package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = "dev"

type config struct {
	user      string
	host      string
	port      string
	keyPath   string
	credsPath string
	apiKey    string
}

func main() {
	keyPath := flag.String("i", "", "SSH private key path")
	port := flag.String("p", "22", "SSH port")
	credsPath := flag.String("c", "", "Path to Claude credentials file")
	apiKey := flag.String("k", "", "ANTHROPIC_API_KEY (alternative to credentials file)")
	showVersion := flag.Bool("v", false, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claudeRemote [-i keypath] [-p port] [-c credspath] [-k apikey] user@host\n")
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

	target := flag.Arg(0)
	user, host, ok := parseTarget(target)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: invalid target %q, expected user@host\n", target)
		os.Exit(1)
	}

	if *credsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
			os.Exit(1)
		}
		*credsPath = home + "/.claude/.credentials.json"
	}

	cfg := config{
		user:      user,
		host:      host,
		port:      *port,
		keyPath:   *keyPath,
		credsPath: *credsPath,
		apiKey:    *apiKey,
	}

	addr := cfg.host + ":" + cfg.port
	fmt.Fprintf(os.Stderr, "Connecting to %s@%s:%s...\n", cfg.user, cfg.host, cfg.port)

	client, err := sshConnect(addr, cfg.user, cfg.keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Fprintf(os.Stderr, "Connected.\n")

	if err := provision(client, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Provisioning failed: %v\n", err)
		os.Exit(1)
	}

	var env map[string]string
	if cfg.apiKey != "" {
		env = map[string]string{"ANTHROPIC_API_KEY": cfg.apiKey}
	}

	if err := interactiveSession(client, "claude", env); err != nil {
		fmt.Fprintf(os.Stderr, "Session error: %v\n", err)
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
