# Hangar — Terminal SSH Manager Design

## Overview

Hangar is a terminal SSH manager built in Go that provides both a TUI dashboard and CLI subcommands for managing SSH connections, live sessions, and fleet command execution.

## Architecture

**Hybrid SSH approach:**

- Interactive sessions: system `ssh` binary via PTY (full SSH compatibility)
- Fleet execution: `golang.org/x/crypto/ssh` (programmatic control, concurrent connections)

## Project Structure

```
hangar/
├── cmd/hangar/main.go
├── internal/
│   ├── config/       # Connection storage, SSH config sync
│   ├── ssh/          # SSH connection logic (system + Go SSH)
│   ├── tui/          # Bubbletea TUI components
│   ├── cli/          # Cobra CLI subcommands
│   └── fleet/        # Multi-server execution engine
├── go.mod
└── go.sum
```

**Key dependencies:**

- `cobra` — CLI framework
- `bubbletea` + `lipgloss` + `bubbles` — TUI framework
- `golang.org/x/crypto/ssh` — Go SSH for fleet exec
- `gopkg.in/yaml.v3` — config parsing
- `go-keyring` — system keychain access for password storage

## Config & Connection Storage

Connections stored in `~/.hangar/connections.yaml`:

```yaml
connections:
  - name: prod-api-1 # unique identifier
    host: 10.0.1.50
    port: 22
    user: deploy
    identity_file: ~/.ssh/id_ed25519
    tags: [production, api]
    jump_host: bastion-prod # references another connection by name
    synced_from_ssh_config: false

  - name: bastion-prod
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_ed25519
    tags: [production, bastion]
    synced_from_ssh_config: true

ssh_sync:
  last_sync: "2026-04-19T10:30:00Z"
  last_ssh_config_hash: "abc123..."
```

**SSH config sync:**

- On startup, hash `~/.ssh/config` and compare to `last_ssh_config_hash`
- If changed, show indicator in TUI ("SSH config changed — press `S` to sync")
- Sync imports new entries, updates changed ones, never deletes hangar-native entries
- Synced entries marked `synced_from_ssh_config: true`
- Also available via `hangar sync` CLI command

## CLI Interface

```
hangar                        # launches TUI
hangar connect <name>         # connect to a saved connection by name
hangar add <name>             # add a new connection interactively
hangar remove <name>          # remove a connection
hangar list                   # list all connections
hangar list --tag production  # filter by tag
hangar sync                   # sync from ~/.ssh/config
hangar exec <command>         # run command on servers
  --tag <tag>                 #   filter targets by tag
  --name <name1,name2>       #   filter targets by name
  --filter <server>           #   filter output to specific server
hangar tag <name> <tags...>   # add tags to a connection
hangar untag <name> <tags...> # remove tags from a connection
```

## TUI Layout

```
┌─────────────────────────────────────────────────────┐
│ hangar                          SSH config changed ⟳ │
├──────────────┬──────────────────────────────────────┤
│ Connections  │                                      │
│ ──────────── │                                      │
│ filter...    │         Active Session               │
│              │                                      │
│ > prod-api-1 │   $ whoami                           │
│   prod-api-2 │   deploy                             │
│   staging-1  │   $ _                                │
│   bastion    │                                      │
│              │                                      │
│ Sessions (2) │                                      │
│ ──────────── │                                      │
│   prod-api-1 │                                      │
│ > staging-1  │                                      │
│              │                                      │
├──────────────┴──────────────────────────────────────┤
│ [c]onnect [d]isconnect [e]xec [s]ync [t]ag [/]find │
└─────────────────────────────────────────────────────┘
```

- **Left sidebar:** connection list (top) with fuzzy filter + active sessions (bottom)
- **Right pane:** active session terminal output, or connection details when no session
- **Bottom bar:** keyboard shortcut hints
- **Top bar:** SSH config change indicator

**Session switching:** `Ctrl+a` prefix key (configurable) enters "hangar mode":

- `j/k` or arrows to navigate sidebar
- `Enter` to switch to selected session
- `c` to connect to selected server
- `d` to disconnect current session
- `Esc` to return to active session

## Fleet Execution

```
┌──────────────────────────────────────────────┐
│ hangar exec: uptime  (3 servers)             │
│──────────────────────────────────────────────│
│ █ prod-api-1   10:30:01 up 45 days, 2:13    │
│ █ prod-api-2   10:30:01 up 45 days, 2:12    │
│ █ bastion      10:30:02 up 120 days, 5:01   │
└──────────────────────────────────────────────┘
```

- Colored `█` left border per server (consistent color assignment)
- Output streams in real-time
- `--filter` shows only one server's output (no border needed)
- Errors shown inline (e.g., `█ prod-api-3  ERROR: connection refused`)
- Also available from TUI via `e` key — select targets, type command, output in right pane

## Authentication

**Auth methods tried in order:**

1. SSH agent (`SSH_AUTH_SOCK`)
2. Identity file from connection config
3. Password from system keychain
4. Password prompt (with option to save to keychain)

**Password storage:** System keychain via `go-keyring` (macOS Keychain / Linux secret-service), keyed by connection name (e.g., `hangar:prod-api-1`). No credentials in config files.

**Jump hosts / ProxyJump:**

- Configured per-connection via `jump_host` field, referencing another connection by name
- Supports chaining (jump host can itself have a jump host)
- Interactive sessions: translated to `-J` flag for system `ssh`
- Fleet exec: Go SSH dials the chain programmatically

## Global Configuration

`~/.hangar/config.yaml`:

```yaml
prefix_key: ctrl+a
ssh_config_path: ~/.ssh/config
auto_sync: true
```
