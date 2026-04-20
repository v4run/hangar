# Hangar TUI — Current UX Description

## Purpose

Hangar is a terminal-based SSH connection manager. Users organize SSH connections into groups, configure SSH options, and connect with a single keystroke. It runs in alternate screen mode (full terminal takeover).

## Technology

- Go with Bubbletea (TUI framework) and Lipgloss (styling)
- No mouse support — keyboard only
- Monospace terminal rendering
- Colors use ANSI 256 palette (color indices, not hex): magenta(13) for selection/cursor, cyan(14) for tags, gray(8) for dim/labels, red(1) for errors, yellow(3) for warnings, green(2) for success

## Layout

```
+---------------------------+-------------------------------------------+
| SIDEBAR (26 chars wide)   | MAIN PANE (remaining width)               |
| Fixed width               | Flexible width                            |
|                           |                                           |
| / filter text             | Connection detail OR form                 |
|                           |                                           |
|   ungrouped-conn-1        | **prod-server**  deploy@10.0.0.1:22       |
|   ungrouped-conn-2        | key ~/.ssh/id_ed25519                     |
|   ▾ production            | via bastion                               |
|     > prod-server         | pass ********                             |
|     prod-db               | staging, api                              |
|   ▾ staging               |                                           |
|     staging-1             | notes Some note text                      |
|   ▸ archived (3)          |                                           |
|                           | **Scripts**                               |
|                           |   > deploy   [global]                     |
|                           |     $ ./deploy.sh                         |
|                           |   health-check                            |
|                           |     $ curl localhost/health               |
+---------------------------+-------------------------------------------+
| n:new  e:edit  d:del  g:group  x:cut  y:copy  p:paste  G:settings   |
| enter:connect  s:sync  t:tag  l:scripts  /:find  q:quit             |
+----------------------------------------------------------------------+
```

- **Sidebar**: Left pane with border-right. Shows connections organized by groups.
- **Main Pane**: Right pane with left padding. Shows selected connection details or active form.
- **Status Bar**: Bottom line, gray text, shows context-sensitive keybindings.
- Content height = terminal height - 1 (for status bar).

## Sidebar Details

- **Filter bar** at top: When filtering active, shows `/ filtertext_` with cursor. When inactive but filter applied, shows dim `/ filtertext`.
- **Ungrouped connections** listed first (2-space indent).
- **Groups** shown with collapse arrow:
  - `▾ groupname` when expanded
  - `▸ groupname (N)` when collapsed (shows count)
- **Grouped connections** indented 4 spaces under their group.
- **Cursor**: `> ` prefix in magenta on the focused item. Selected item name rendered bold magenta.
- **Cut mark**: `~` suffix (dim) on connections marked for cut.
- **Copy mark**: `+` suffix (dim) on connections marked for copy.
- No scrolling indicator — content just overflows (items beyond terminal height are invisible).

## Main Pane — Connection Detail View

When a connection is selected (not on a group header), the main pane shows:

```
**connection-name**  user@host:port      (title bold + dim detail)
key ~/.ssh/id_ed25519                    (dim, only if set)
via bastion-name                         (dim, only if jump host set)
pass ********                            (dim, only if password stored)
staging, api                             (tags in cyan)

notes Some user note text                (dim label + normal text)

**Scripts**                              (bold header)
  > script-name   [global]              (cursor + badge if global)
    $ command-here                       (dim, indented)
  another-script
    $ other-command
```

When cursor is on a group header:
```
group: groupname

press space to expand/collapse
```

When no connections exist:
```
no connections

press n to add or s to sync from SSH config
```

## Forms

All forms render in the main pane (replacing connection detail). Forms have:
- A bold title at top
- Fields rendered vertically, one per line
- Active field shown with `> fieldname` in bold magenta + value + `_` cursor
- Inactive fields shown with `  fieldname` in gray (width 7) + value
- Error messages at bottom in red

### Add/Edit Connection Form

```
**New Connection** (or **Edit Connection**)

> name    myserver_
  host    10.0.0.1
  port    22
  user    deploy
  key     ~/.ssh/id_ed25519
  jump    bastion
  group   production
  tags    staging, api
  pass    ********

Advanced SSH Options
  fwdagent  [-]  yes   no
  compress  [-] [yes]  no
  localfwd  8080:localhost:80, 9090:localhost:90    (placeholder when empty)
  remotefwd 3000:localhost:3000                     (placeholder when empty)
  alive     60                                      (placeholder when empty)
  alivemax  3                                       (placeholder when empty)
  hostkey   [-]  yes   no   accept-new
  tty       [-]  yes   no   force   auto
  envs      KEY=value, ANOTHER=val                  (placeholder when empty)
  extra     TCPKeepAlive=yes, LogLevel=INFO         (placeholder when empty)
  useglobal [yes]  no
```

- Basic fields (name through pass): label width 7 chars, free text input
- Password field: shows `***` instead of actual value
- Advanced section: separated by a header line, label width 10 chars
- **Cycle fields** (ForwardAgent, Compression, HostKey, TTY, UseGlobal): pressing any key cycles through valid options. Active option shown in `[brackets]` bold magenta, others dim.
- **Free text fields** (LocalFwd, RemoteFwd, Alive, AliveMax, Envs, Extra): show dim placeholder text when empty, normal text when filled.
- **JumpHost autocomplete**: When typing in the Jump field, matching connections appear as suggestions below the field:
  ```
    > bastion (admin@10.0.0.1)
      prod-jump (root@jump.example.com)
  ctrl+n/p: navigate  enter: select
  ```

### Global Settings Form (G key)

```
**Global SSH Settings**

> fwdagent  [-] [yes]  no
  compress  [-]  yes   no
  localfwd  8080:localhost:80, 9090:localhost:90
  remotefwd 3000:localhost:3000
  alive     60
  alivemax  3
  hostkey   [-]  yes   no   accept-new
  tty       [-]  yes   no   force   auto
  envs      KEY=value, ANOTHER=val
  extra     TCPKeepAlive=yes, LogLevel=INFO
```

Same as advanced section of connection form but without the UseGlobal field.

### Other Forms

- **Add Group**: Single field `> name groupname_`
- **Edit Group**: Single field pre-filled with current name
- **Delete Connection**: `Remove **name**?` + "this cannot be undone" + y/esc
- **Delete Group**: `Remove group **name**?` + "connections will be ungrouped, not deleted" + y/esc
- **Tag Input**: Shows existing tags in cyan, then `> add tagtext_` with hint "comma-separated, prefix with - to remove"
- **Script Form**: Two fields (name, command) with tab to switch
- **Notes Form**: Single field `> notetext_`
- **SSH Config Sync**: Checklist with `[x]`/`[ ]`, cursor, space to toggle, a/n for all/none, enter to import

## Keybindings

### Normal Mode (sidebar focused)

| Key | Action |
|-----|--------|
| j / down | Move cursor down |
| k / up | Move cursor up |
| space | Toggle group collapse |
| / | Enter filter mode |
| n | New connection form |
| e | Edit (connection name/settings, or group name if on group header) |
| d | Delete connection or group |
| t | Tag management |
| g | Create new group |
| x | Cut (mark for move) |
| y | Copy (mark for duplicate) |
| p | Paste (move cut items, duplicate copied items into target group) |
| G | Global SSH settings |
| l | Focus scripts pane |
| s / S | SSH config sync |
| enter | Connect to selected server |
| q / ctrl+c | Quit |

### Scripts Pane Focused

| Key | Action |
|-----|--------|
| j/k | Navigate scripts |
| n | New script |
| e | Edit script |
| d | Delete script |
| enter | Run script on connection |
| o | Edit notes |
| h / esc | Back to sidebar |

### Filter Mode

| Key | Action |
|-----|--------|
| any char | Append to filter |
| backspace | Delete last char |
| enter | Apply and exit filter mode |
| esc | Clear filter and exit |

### Form Mode

| Key | Action |
|-----|--------|
| tab / down | Next field |
| shift+tab / up | Previous field |
| enter | Save (or select JumpHost suggestion) |
| esc | Cancel |
| backspace | Delete char (or clear cycle field) |
| any char | Type in field (or cycle constrained field) |
| space | Cycle constrained field options |
| ctrl+n | Next JumpHost suggestion |
| ctrl+p | Previous JumpHost suggestion |

### Bracket Paste

Pasting text (ctrl+v in most terminals) delivers the full pasted string at once into the active form field.

## Styling

- **Selected/cursor items**: Color 13 (magenta/purple), bold
- **Dim/labels**: Color 8 (gray)
- **Tags**: Color 14 (cyan)
- **Errors**: Color 1 (red)
- **Warnings**: Color 3 (yellow)
- **Success/checkmarks**: Color 2 (green)
- **Sidebar border**: Right border using NormalBorder, color 8
- **Main pane**: PaddingLeft(2)
- **Status bar**: Full width, color 8

## Data Model (for context)

Each connection has: ID (UUID), Name, Host, Port, User, IdentityFile, Tags[], JumpHost, Group, Scripts[], Notes, SSHOptions (ForwardAgent, Compression, LocalForward[], RemoteForward[], ServerAliveInterval, ServerAliveCountMax, StrictHostKeyCheck, RequestTTY, EnvVars{}, ExtraOptions{}), UseGlobalSettings.

Global settings have the same SSHOptions fields as defaults for all connections.

## Known UX Limitations

1. No scrolling — long lists overflow beyond terminal height with no indicator
2. Sidebar is fixed 26 chars — long connection names get truncated
3. No search/filter in forms (e.g., can't filter the sync list)
4. No confirmation when pasting (could overwrite existing connections)
5. Status bar keybinding hints are dense and may not fit narrow terminals
6. No visual distinction between basic and advanced sections beyond the header text
7. Can't reorder connections or groups
8. No multi-select (cut/copy works one at a time)
9. No visual feedback after operations (e.g., "saved!", "connected!")
10. Empty state could be more welcoming/instructive

---

# Output Format Instructions for Design Feedback

Please provide your design improvement suggestions in the following structured format so they can be directly implemented by a developer using Claude Code:

## Required Output Format

For each design suggestion, provide:

```yaml
- id: short-kebab-case-id
  priority: high | medium | low
  category: layout | interaction | visual | feedback | accessibility
  summary: One sentence describing the change
  current: |
    Description of current behavior or appearance
  proposed: |
    Description of proposed behavior or appearance
  mockup: |
    ASCII art showing the proposed UI (use exact characters that would render in terminal)
    Use these conventions:
    - [bold] for bold text
    - (dim) for dim/gray text
    - {cyan} for colored text
    - <magenta> for selection/cursor color
    - Exact character widths matter — this is monospace
  implementation_hints:
    - Which component/area of the UI to modify
    - Any new state or data needed
    - Interaction behavior changes
  affects:
    - List of UI areas this change touches (sidebar, main-pane, status-bar, forms)
```

## Guidelines for Suggestions

- Keep changes implementable within a terminal TUI (no graphics, no mouse, monospace only)
- Respect the vim-style keyboard navigation philosophy
- Consider narrow terminals (80 chars minimum width)
- Don't suggest features outside the SSH connection management scope
- Be specific about spacing, alignment, and character choices
- If suggesting color changes, use ANSI 256 color indices (0-255)
- Prioritize: clarity > aesthetics > density
- Group related changes together
- Consider the flow: most users will add connections, organize into groups, then connect repeatedly
