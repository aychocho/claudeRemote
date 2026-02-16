# claudeRemote

SSH into a remote machine and launch [Claude Code](https://claude.ai/claude-code) with a single command. Handles installation and credential provisioning automatically.

## What it does

1. Connects to the remote host via SSH (agent or key-based auth)
2. Installs Claude Code if not already present
3. Uploads your local Claude credentials (or uses an API key)
4. Opens an interactive Claude Code session with full PTY support

## Install

### From .deb (Debian/Ubuntu)

Download the `.deb` and `checksums.txt` from the [releases page](../../releases/latest), then:

```bash
# verify the checksum
sha256sum -c checksums.txt

# install
sudo dpkg -i claudeRemote_*.deb
```

This places the binary at `/usr/local/bin/claudeRemote`.

To uninstall:

```bash
sudo dpkg -r claudeRemote
```

### Build from source

Requires Go 1.21+.

```bash
git clone <repo-url> && cd claude-remote

# option 1: build a local binary
make build        # produces ./claudeRemote

# option 2: install to $GOPATH/bin
make install
```

### Build the .deb yourself

Requires [nfpm](https://nfpm.goreleaser.com/install/).

```bash
make deb          # produces claudeRemote_<version>_<arch>.deb
make checksums    # produces checksums.txt
```

## Usage

```
claudeRemote [options] user@host
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-i` | SSH private key path | SSH agent |
| `-p` | SSH port | `22` |
| `-c` | Path to Claude credentials file | `~/.claude/.credentials.json` |
| `-k` | Anthropic API key (alternative to credentials) | — |
| `-v` | Print version and exit | — |

### Examples

```bash
# Connect using SSH agent
claudeRemote ubuntu@dev-server

# Connect with a specific key and port
claudeRemote -i ~/.ssh/id_ed25519 -p 2222 user@host

# Use an API key instead of credentials file
claudeRemote -k sk-ant-... user@host
```

## Authentication

**SSH:** Uses your local SSH agent by default. Pass `-i` for key-based auth.

**Claude:** By default, copies `~/.claude/.credentials.json` to the remote host. Alternatively, pass `-k` with an Anthropic API key.

**Host keys:** Verified against `~/.ssh/known_hosts` when available. Falls back to accepting all keys with a warning if the file is missing.

## Requirements

- Go 1.21+
- SSH access to the target machine
- Claude credentials or API key

## Acknowledgements

Getting around mandatory onboarding was heavily inspired (basically borrowed) from this blog post. https://www.xugj520.cn/en/archives/claude-code-login-bypass-guide.html
