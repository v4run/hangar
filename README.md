# Hangar

A terminal SSH connection manager with a TUI dashboard. Built in Go.

## Features

- **TUI Dashboard** -- interactive terminal UI with connection browser and fuzzy filter
- **Connection Management** -- add, edit, remove, and tag SSH connections
- **Groups** -- organize connections into collapsible groups, cut/paste connections between groups
- **Scripts** -- per-connection and global scripts, run remotely with one keypress
- **Notes** -- attach notes to connections
- **Full-Screen SSH** -- connect to servers with full terminal support (colors, vim, htop)
- **Selective SSH Config Sync** -- import selected connections from `~/.ssh/config`
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
┌──────────────┬──────────────────────────────────────┐
│ / filter...  │ prod-api  deploy@10.0.1.50:22        │
│              │ key ~/.ssh/id_ed25519                │
│ > prod-api   │ via bastion                          │
│   bastion    │ production, api                      │
│              │                                      │
│ ▾ staging    │ notes maintenance window: Sun 2-4am  │
│     web-1    │                                      │
│     web-2    │ Scripts                              │
│ ▸ dev (3)    │ > tail logs            $ tail -f ... │
│              │   disk usage  [global] $ df -h       │
├──────────────┴──────────────────────────────────────┤
│ n:new e:edit d:del g:group x:cut y:paste enter:...  │
└─────────────────────────────────────────────────────┘
```

### Keybindings — Connection List (left pane)

| Key       | Action                                         |
| --------- | ---------------------------------------------- |
| `j` / `k` | Navigate connections and groups                |
| `/`       | Filter connections (by name or tag)            |
| `n`       | New connection                                 |
| `e`       | Edit selected connection                       |
| `d`       | Delete connection or group                     |
| `t`       | Manage tags (prefix with `-` to remove)        |
| `g`       | Create new group                               |
| `x`       | Cut connection (for moving between groups)     |
| `y`       | Paste connection into group at cursor          |
| `space`   | Expand/collapse group                          |
| `l`       | Focus scripts pane (right side)                |
| `s`       | Sync from SSH config (selective import)        |
| `Enter`   | Connect (full-screen SSH, TUI resumes on exit) |
| `q`       | Quit                                           |

### Keybindings — Scripts Pane (right pane)

| Key       | Action                  |
| --------- | ----------------------- |
| `j` / `k` | Navigate scripts        |
| `n`       | New script              |
| `e`       | Edit script             |
| `d`       | Delete script           |
| `o`       | Edit notes              |
| `Enter`   | Run script on server    |
| `h`       | Back to connection list |

## Groups

Connections can be organized into collapsible groups. Use `J`/`K` on a group header to reorder groups manually; on a connection it swaps with the neighbor in the same group. Collapsed groups show the connection count.

- Create a group with `g`, assign connections via the group field in the add/edit form
- Move connections between groups with `x` (cut) and `y` (paste)
- Delete a group with `d` on the group header — connections are ungrouped, not deleted
- New connections pre-fill the group from your current cursor position

## Scripts

Attach scripts to connections or define global scripts shared across all servers.

- **Per-connection scripts** -- saved in the connection config, run on that server
- **Global scripts** -- shared across all connections, shown with `[global]` badge
- Scripts run via `ssh -t server "bash -l -c 'command'"` for proper TTY and locale support
- Output is shown full-screen with a "press any key" prompt before returning to TUI

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
    group: production
    notes: "maintenance window: Sundays 2-4am"
    scripts:
      - name: tail logs
        command: tail -f /var/log/app.log
      - name: restart
        command: systemctl restart app

global_scripts:
  - name: disk usage
    command: df -h
  - name: uptime
    command: uptime
```

### Global Config

Stored in `~/.hangar/config.yaml`:

```yaml
ssh_config_path: ~/.ssh/config
auto_sync: true
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
