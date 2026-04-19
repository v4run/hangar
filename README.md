# Hangar

A terminal SSH manager with a TUI dashboard, live session management, and fleet command execution. Built in Go.

## Features

- **TUI Dashboard** -- interactive terminal UI with connection browser, fuzzy filter, and live SSH sessions
- **Connection Management** -- save, organize, and tag SSH connections in `~/.hangar/connections.yaml`
- **Live Sessions** -- connect to servers from the TUI, switch between multiple active sessions with `Ctrl+a`
- **Fleet Execution** -- run commands across multiple servers concurrently with colored streaming output
- **SSH Config Sync** -- import connections from `~/.ssh/config` with automatic change detection
- **Tags** -- organize connections with tags, filter by tag in both CLI and TUI
- **Jump Host Support** -- ProxyJump chains for both interactive sessions and fleet execution
- **Keychain Integration** -- store passwords in the system keychain (macOS Keychain / Linux secret-service)

## Install

```bash
go install github.com/v4run/hangar/cmd/hangar@latest
```

Or build from source:

```bash
git clone https://github.com/v4run/hangar.git
cd hangar
go build -o hangar ./cmd/hangar/
```

## Quick Start

```bash
# Add a connection
hangar add prod-api --host 10.0.1.50 --user deploy --tag production,api

# Add a bastion / jump host
hangar add bastion --host bastion.example.com --user admin --tag production
hangar add internal-db --host 10.0.1.100 --user dbadmin --jump-host bastion

# Import from ~/.ssh/config
hangar sync

# List connections
hangar list
hangar list --tag production

# Connect to a server
hangar connect prod-api

# Run a command across servers
hangar exec --tag production uptime
hangar exec --name prod-api,internal-db "df -h"
hangar exec --tag production --filter prod-api "cat /etc/hostname"

# Launch the TUI
hangar
```

## CLI Reference

```
hangar                        Launch TUI dashboard
hangar connect <name>         Connect to a saved connection
hangar add <name>             Add a new connection
hangar remove <name>          Remove a connection
hangar list                   List all connections
hangar list --tag <tag>       Filter connections by tag
hangar sync                   Sync from ~/.ssh/config
hangar exec <command>         Run command across servers
  --tag <tag>                   filter targets by tag
  --name <n1,n2>                filter targets by name
  --filter <server>             show output from one server only
hangar tag <name> <tags...>   Add tags to a connection
hangar untag <name> <tags...> Remove tags from a connection
```

## TUI

Launch with `hangar` (no arguments).

```
┌─────────────────────────────────────────────────────┐
│ hangar                          SSH config changed   │
├──────────────┬──────────────────────────────────────┤
│ Connections  │                                      │
│ ──────────── │                                      │
│  /filter...  │       Connection Details             │
│              │                                      │
│ > prod-api   │  Host: 10.0.1.50                     │
│   bastion    │  Port: 22                            │
│   staging    │  User: deploy                        │
│              │  Tags: production, api               │
│ Sessions (1) │                                      │
│ ──────────── │                                      │
│ > prod-api   │                                      │
│              │                                      │
├──────────────┴──────────────────────────────────────┤
│ [a]dd [enter]edit [x]del [c]onnect [e]xec [t]ag    │
└─────────────────────────────────────────────────────┘
```

### Keybindings

**Sidebar mode:**

| Key       | Action                                  |
| --------- | --------------------------------------- |
| `j` / `k` | Navigate connections                    |
| `/`       | Filter connections (by name or tag)     |
| `a`       | Add new connection                      |
| `Enter`   | Edit selected connection                |
| `x`       | Delete selected connection              |
| `t`       | Manage tags (prefix with `-` to remove) |
| `c`       | Connect to selected server              |
| `e`       | Execute command across servers          |
| `s`       | Sync from SSH config                    |
| `q`       | Quit                                    |

**Session mode** (inside an active SSH session):

| Key      | Action                                  |
| -------- | --------------------------------------- |
| `Ctrl+a` | Enter hangar mode (escape from session) |

**Hangar mode** (after `Ctrl+a`):

| Key       | Action                     |
| --------- | -------------------------- |
| `j` / `k` | Navigate sessions          |
| `Enter`   | Switch to selected session |
| `d`       | Disconnect current session |
| `Esc`     | Return to active session   |

## Fleet Execution

Run commands across multiple servers with colored streaming output. Each server gets a colored left border for easy identification.

```bash
$ hangar exec --tag production uptime
hangar exec: uptime  (3 servers)
█ prod-api-1     10:30:01 up 45 days, 2:13
█ prod-api-2     10:30:01 up 45 days, 2:12
█ bastion         10:30:02 up 120 days, 5:01
```

Use `--filter` to show output from a single server (no border):

```bash
$ hangar exec --tag production --filter prod-api-1 uptime
 10:30:01 up 45 days, 2:13
```

## Configuration

### Connections

Stored in `~/.hangar/connections.yaml`:

```yaml
connections:
  - name: prod-api
    host: 10.0.1.50
    port: 22
    user: deploy
    identity_file: ~/.ssh/id_ed25519
    tags: [production, api]
    jump_host: bastion
    synced_from_ssh_config: false
```

### Global Config

Stored in `~/.hangar/config.yaml`:

```yaml
prefix_key: ctrl+a # TUI escape key (configurable)
ssh_config_path: ~/.ssh/config
auto_sync: true # check for SSH config changes on startup
```

## Authentication

Auth methods are tried in order:

1. **SSH agent** (`SSH_AUTH_SOCK`)
2. **Identity file** specified in connection config
3. **System keychain** password (macOS Keychain / Linux secret-service)
4. **Password prompt** (interactive, for system SSH connections)

## Architecture

Hangar uses a hybrid SSH approach:

- **Interactive sessions**: system `ssh` binary via PTY -- full compatibility with SSH features, agent forwarding, escape sequences
- **Fleet execution**: `golang.org/x/crypto/ssh` -- programmatic control for concurrent connections and structured output

## License

MIT
