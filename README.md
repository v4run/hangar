# Hangar

A terminal SSH connection manager with a TUI dashboard. Built in Go.

## Features

- **TUI Dashboard** -- interactive terminal UI with connection browser and fuzzy filter
- **Connection Management** -- add, edit, remove, and tag SSH connections
- **Full-Screen SSH** -- connect to servers with full terminal support (colors, vim, htop)
- **SSH Config Sync** -- import connections from `~/.ssh/config` with automatic change detection
- **Tags** -- organize connections with tags, filter by tag in both CLI and TUI
- **Jump Host Support** -- ProxyJump chains for SSH connections
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

# Add with password (stored in system keychain)
hangar add staging --host staging.example.com --user admin --password secret

# Add a bastion / jump host
hangar add bastion --host bastion.example.com --user admin
hangar add internal-db --host 10.0.1.100 --user dbadmin --jump-host bastion

# Import from ~/.ssh/config
hangar sync

# List connections
hangar list
hangar list --tag production

# Connect to a server
hangar connect prod-api

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
│              │                                      │
├──────────────┴──────────────────────────────────────┤
│ [a]dd [enter]edit [x]del [c]onnect [t]ag [/]find   │
└─────────────────────────────────────────────────────┘
```

### Keybindings

| Key       | Action                                  |
| --------- | --------------------------------------- |
| `j` / `k` | Navigate connections                    |
| `/`       | Filter connections (by name or tag)     |
| `a`       | Add new connection                      |
| `Enter`   | Edit selected connection                |
| `x`       | Delete selected connection              |
| `t`       | Manage tags (prefix with `-` to remove) |
| `c`       | Connect (full-screen SSH, TUI resumes on exit) |
| `s`       | Sync from SSH config                    |
| `q`       | Quit                                    |

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
ssh_config_path: ~/.ssh/config
auto_sync: true # check for SSH config changes on startup
```

## Authentication

When connecting, hangar tries in order:

1. **SSH agent** (`SSH_AUTH_SOCK`)
2. **Identity file** specified in connection config
3. **System keychain** password (auto-provided via SSH_ASKPASS)
4. **Password prompt** (interactive fallback)

Passwords stored via `hangar add --password` or the TUI form are saved in the system keychain and automatically provided to SSH.

## License

MIT
