# Hangar: Bugs & Features Design

## Summary

This design addresses 8 bugs/features: UUID-based connection IDs, editable group and connection names, copy/paste keybinding changes, environment variables, advanced SSH options, global settings, duplicate name support, and paste-into-fields support.

## Data Model Changes

### UUID-based Connection Identity

Connections get a `uuid.UUID` ID field as the primary identifier. Names become display-only and non-unique. All internal lookups (cut/paste buffers, keychain storage, JumpHost references) use UUIDs.

```go
type Connection struct {
    ID                  uuid.UUID         `yaml:"id"`
    Name                string            `yaml:"name"`
    Host                string            `yaml:"host"`
    Port                int               `yaml:"port"`
    User                string            `yaml:"user"`
    IdentityFile        string            `yaml:"identity_file,omitempty"`
    Tags                []string          `yaml:"tags,omitempty"`
    JumpHost            string            `yaml:"jump_host,omitempty"`     // UUID string OR raw "user@host:port"
    Group               string            `yaml:"group,omitempty"`
    SyncedFromSSHConfig bool              `yaml:"synced_from_ssh_config"`
    Scripts             []Script          `yaml:"scripts,omitempty"`
    Notes               string            `yaml:"notes,omitempty"`
    SSHOptions          *SSHOptions       `yaml:"ssh_options,omitempty"`
    UseGlobalSettings   *bool             `yaml:"use_global_settings,omitempty"` // nil/true = inherit, false = don't
}
```

### SSHOptions Struct (shared between Connection and GlobalConfig)

```go
type SSHOptions struct {
    ForwardAgent         *bool             `yaml:"forward_agent,omitempty"`
    LocalForward         []string          `yaml:"local_forward,omitempty"`
    RemoteForward        []string          `yaml:"remote_forward,omitempty"`
    ServerAliveInterval  *int              `yaml:"server_alive_interval,omitempty"`
    ServerAliveCountMax  *int              `yaml:"server_alive_count_max,omitempty"`
    StrictHostKeyCheck   string            `yaml:"strict_host_key_checking,omitempty"`
    RequestTTY           string            `yaml:"request_tty,omitempty"`
    Compression          *bool             `yaml:"compression,omitempty"`
    EnvVars              map[string]string `yaml:"env_vars,omitempty"`
    ExtraOptions         map[string]string `yaml:"extra_options,omitempty"`
}
```

### Updated GlobalConfig

```go
type GlobalConfig struct {
    PrefixKey     string      `yaml:"prefix_key"`
    SSHConfigPath string      `yaml:"ssh_config_path"`
    AutoSync      bool        `yaml:"auto_sync"`
    SSHOptions    *SSHOptions `yaml:"ssh_options,omitempty"`
}
```

## Keybinding Changes

| Key | Old Action | New Action |
|-----|-----------|------------|
| `y` | Paste cut connections | Copy (yank) selected connection |
| `p` | _(none)_ | Paste cut or copied connections into target group |
| `x` | Cut connection | Cut connection _(unchanged)_ |
| `e` | Edit connection only | Edit connection or group name (context-dependent) |
| `G` | _(none)_ | Open global settings form |

### Copy vs Cut

- `x` marks connections for **cut** (move). Shown with `~` in sidebar.
- `y` marks connections for **copy** (duplicate). Shown with `+` in sidebar.
- `p` pastes: cut items are moved, copied items are duplicated with new UUIDs.
- Clears both buffers after paste.

### Editing Group Names

When cursor is on a group header and `e` is pressed, opens a single-field form pre-filled with the group name. On submit, renames the group across all connections and the `cfg.Groups` map.

### Editing Connection Names

Name field is now editable in the edit form (was read-only). Safe because UUIDs are the real identifiers.

## Advanced SSH Settings

### Per-Connection

The connection add/edit form gains an "Advanced Settings" section below existing fields with fields for all SSHOptions plus `UseGlobalSettings` toggle. Empty fields inherit from global.

### Global Settings (`G` keybinding)

Opens a full-screen form with the same SSHOptions fields. Values act as defaults for all connections unless overridden per-connection.

## JumpHost Picker

The JumpHost field supports inline autocomplete. As the user types, matching connections appear as suggestions below the field. Selecting one inserts the connection's UUID. Raw values (e.g., `user@bastion:22`) are also accepted.

## SSH Command Building

When building SSH args, merge options:
1. Start with global SSHOptions
2. If `UseGlobalSettings != false`, overlay per-connection SSHOptions (non-nil values override)
3. Convert to SSH flags: `-o ForwardAgent=yes`, `-L` for LocalForward, `-R` for RemoteForward, `-o KEY=VALUE` for ExtraOptions

JumpHost resolution: if parseable as UUID, resolve to connection and build `-J user@host:port`. Otherwise use as raw value.

## Paste into Text Fields

Handle Bubbletea's bracket paste (`tea.PasteMsg`) to support pasting multi-character strings into form text fields.

## Migration

On config load:
1. Connections without `id` get `uuid.New()` auto-assigned
2. JumpHost values matching existing connection names are replaced with UUIDs
3. Keychain entries are migrated from name-based to UUID-based keys
4. Save immediately after migration

Core lookup changes from `FindByName` to `FindByID`. `FindByName` remains for CLI and returns first match.
