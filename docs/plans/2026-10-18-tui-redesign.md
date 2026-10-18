# Hangar TUI redesign — unified plan

Date: 2026-10-18

## Context

Hangar's TUI grew organically: a 26-col left sidebar lists everything
(connections, databases, groups), the right pane shows the detail view
*or* the current form, and the status bar carries context-aware hints.
It works — but a session-long UX audit, a competitive survey, and a
look at best-in-class TUIs (k9s, gh dash, helix, zellij) converge on
the same structural issues:

- **Overloaded single letters.** `n`/`e`/`d`/`l` silently branch on
  cursor context; `l`/`h` mean different things in nav vs edit mode;
  `g`/`G` burn vim-grammar slots for unrelated features. The user
  can't predict what a key does without reading the help screen.
- **Forms destroy the detail view.** Opening the connection form
  replaces the panel you were just reading. There's no reference while
  editing. The nav-mode ↔ edit-mode toggle per field doubles form
  keystrokes.
- **Discoverability is help-only.** `v`, `M`, `o`, `J/K`, `i/I`, the
  paste-ssh-command path (`n` → `p`) are only in `?`. The status
  footer "reshapes by width" by *dropping bindings* rather than
  reflowing, so users can't rely on the keymap they see.
- **Fixed sidebar width.** 26 cols truncates names even on a 200-col
  terminal.
- **No cross-cutting search / command palette.** Every action is a
  memorised letter in a specific view.
- **Groups are folders, not inheritance scopes.** Termius' key idea —
  group-level defaults inherited by children — is the single biggest
  UX multiplier available to a connection manager, and prior art
  (lazyssh, sshm, sshs) all skip it.

## Implementation shape — ship as v2 (a true Go module v2)

v2 is a **separate installable version**, not a runtime flag on one
binary. Users install v2 and *get* v2. v1 continues to exist as a
tagged release for users who haven't upgraded; it is not a
code-path inside v2.

### Go's rule for major versions

Go enforces semantic import versioning: `v0` / `v1` share a module
path; `v2` and up **must** carry a `/v2` suffix in the module path
and import URL. See <https://go.dev/ref/mod#major-version-suffixes>.

### Repo layout: subdirectory module

Hangar v2 lives in a `v2/` subdirectory of this same repo, with its
own `go.mod` declaring `module github.com/v4run/hangar/v2`. v1 stays
at the repo root with its existing `go.mod`. Both can be worked on in
the same tree; neither imports the other.

```
hangar/
├── go.mod                    # module github.com/v4run/hangar  (v1)
├── cmd/hangar/               # v1 binary entrypoint
├── internal/
│   ├── config/               # v1's config package
│   ├── ssh/
│   ├── db/
│   └── tui/                  # v1 TUI (frozen)
└── v2/
    ├── go.mod                # module github.com/v4run/hangar/v2
    ├── cmd/hangar/           # v2 binary entrypoint (installs as `hangar`)
    └── internal/
        ├── config/           # v2's own copy — schema v2, nested groups
        ├── ssh/              # v2's own copy
        ├── db/               # v2's own copy
        └── tui/              # the full redesign (phases 1–4 land here)
```

Install:
```
go install github.com/v4run/hangar/v2/cmd/hangar@latest
```
Tag `v2.0.0` on the repo once v2 is release-ready; Go's module proxy
resolves the subdirectory layout automatically because the inner
`go.mod` carries the `/v2` suffix.

### Shared code: clean break, not shared import

The v1 → v2 migration is a schema and UX break, not a library share.
v2 **copies** `config`, `ssh`, `db` into `v2/internal/` and modifies
them freely (new types, new fields, new behavior). v2 has zero
dependency on v1. This costs us two copies during the overlap period;
in exchange, v2 is free to evolve the shared types for nested groups,
schema 2, and anything else without compatibility gymnastics. When v1
is retired, there's only one copy again.

### Binary name

v2's entrypoint is `v2/cmd/hangar/main.go` so `go install` produces a
binary named `hangar`. Installing v2 **overwrites** the user's v1
`hangar` on `$PATH`. "Install v2 = run v2." For users who want to
keep v1 around during the overlap, documented alternative:

```
# keep v1 around under a different name while trying v2
mv "$(which hangar)" "$(dirname "$(which hangar)")/hangar-v1"
go install github.com/v4run/hangar/v2/cmd/hangar@latest
```

Overwrite is the sanctioned install story — zero flags, zero
confusion. A sidelined v1 is a documented one-liner, not a shipped
feature.

### Config file path

v2 writes to `~/.hangar/v2/config.yaml`, separate from v1's
`~/.hangar/config.yaml`. On first v2 run, if a v1 config exists and
no v2 config exists, v2 reads the v1 file, runs the Phase 4 migration
in memory, and writes the result to the v2 path. **v1's config is
never modified.** A user who rolls back to v1 simply uses the v1
binary again against the untouched v1 config — no `.v1.bak` restore
dance.

### Release workflow

- v1 goes into maintenance mode once v2's foundation (Phase 1) is in
  place; critical-fixes only.
- v2 phases 1–4 ship as their own PRs against `main`, each tagged
  under `v2.0.0-alpha.N` → `v2.0.0-beta.N` → `v2.0.0`.
- Once v2.0.0 is tagged, v1 gets a final `v1.x.y` tag and the v1 tree
  (`internal/tui/`, root `cmd/`) is deleted in a subsequent commit.
  The repo then has one module at `v2/` only; a root `go.mod` with
  no code is kept so historic `v1.x.y` tags still resolve.
- Issues and discussion stay in one repo (one of the main reasons to
  prefer subdirectory layout over a separate `hangar-v2` repo).

Research artifacts behind this plan:
- `/private/tmp/claude-501/.../a21118e648a96c933.output` — current-UX
  audit (not committed; summarised below).
- `/private/tmp/claude-501/.../a9528f8b8487e1ea9.output` — k9s / Charm
  / helix+zellij inspiration.
- `/private/tmp/claude-501/.../a9bcb793c9a8be06d.output` — SSH TUI
  prior-art survey (sshs, lazyssh, wishlist, Termius, Royal TSX, …).

Intended outcome: a Hangar that **teaches itself**, keeps context
visible while you edit, and has one keymap grammar you can predict.
Four phases, each shippable independently, each resolving a cluster of
audit findings.

## Design principles

1. **One mode-indicator, three entry points, zero overloaded letters.**
   - `/` filters the current list.
   - `:` pivots the view (`:conn`, `:db`, `:tags`, `:fwd`, `:recent`).
   - `space` opens a helix-style leader menu that shows every next key
     and what it does — the keymap teaches itself.
   - A coloured chip in the bottom-left names the current mode
     (`BROWSE` / `EDIT` / `CONNECT`).

2. **Context is always visible.** The right pane is a **tabbed detail
   pane** (`Details | Databases | Scripts | Forwards | Notes`), never
   replaced by a form. Transactional UI (add/edit/delete/import/
   host-key confirm) opens as a **centered floating palette** on top.

3. **One reusable fuzzy picker.** Query line + match list + live
   preview panel, used for connect, open-db, pick-script, jump-to-tag,
   recent hosts. Same keys, same shape, muscle memory transfers.

4. **Groups are an inheritance scope.** Defaults (identity, jump host,
   env, forwards, shell-init, scripts) live on a group; connections
   override. The `UseGlobalSettings` toggle disappears — inheritance
   is implicit by which scope holds the value.

5. **Charm aesthetics.** Muted palette, one accent, rounded borders,
   generous padding. All colours via `lipgloss.AdaptiveColor` — light
   terminals work. Status footer via `bubbles/help`.

## Target layout

```
┌───────────────────────────────────────────────────────────────────────┐
│ hangar  ▸  prod/web          ⌁ 2 sessions   ⇄ 3 fwds   •  deploy@…    │
├─────────────────────┬─────────────────────────────────────────────────┤
│ ◉ Recent            │ Details | Databases | Scripts | Forwards | Notes│
│   web-1  2m ago     │ ─────────────────────────────────────────────── │
│   db-prod  1h ago   │                                                 │
│                     │  Name       web-1                               │
│ ─ prod ───────────  │  Host       10.0.4.12   (via bastion.prod)      │
│   ▶ web-1  ●        │  User       deploy                              │
│     api-1           │  Identity   ~/.ssh/prod_rsa                     │
│     db-shadow  pg   │  Tags       web  http  critical                 │
│                     │                                                 │
│ ─ staging ────────  │  ssh deploy@10.0.4.12 -J bastion.prod           │
│   web-stg                                                             │
├─────────────────────┴─────────────────────────────────────────────────┤
│ BROWSE   c connect  e edit  d dup  t tag  / filter  : cmd   ? help   ⣾│
└───────────────────────────────────────────────────────────────────────┘
```

Context strip (top), proportional-width sidebar with a `Recent`
section plus groups, tabbed detail (right), mode chip + action strip +
spinner slot (bottom). On width shrink the action strip truncates
with an ellipsis rather than dropping bindings.

## Keymap grammar

| Key | Meaning | Where |
| --- | --- | --- |
| `/` | Filter current list | Any list |
| `:` | Command bar (`:conn`, `:db`, `:tags`, `:fwd`, `:recent`, `:help`) | Global |
| `space` | Leader-menu overlay showing every next key | Global |
| `enter` | Primary action on selection | Any list |
| `tab` / `shift-tab` | Switch detail-pane tab | Browse |
| `gg` / `G` | Top / bottom | Any list |
| `?` | Grouped help grid, includes user bindings | Global |
| `esc` | Close palette / exit mode | Any |

Palette form grammar: `tab`/`shift-tab` move between fields; the
focused field *is* the cursor target — no nav↔edit toggle. `enter`
confirms, `esc` cancels, `ctrl+s` saves. Cycle fields (yes/no) still
accept `h`/`l` but only when focused.

---

## Phase 1 — Theme foundation + help footer + proportional sidebar

**Goal.** Make the aesthetic shift (muted palette, one accent, adaptive
color, rounded borders, breathable spacing) without changing any
behaviour. Set up `bubbles/help` for the status footer so Phase 2 can
drop into it. Resolves audit findings 7 (sidebar width), 4 (status
bar reshape), and part of 5 (discoverability).

**Critical files.**
- `internal/tui/styles.go` *(new)* — consolidate every `lipgloss.Style`
  currently sprinkled across `view.go`. Define semantic roles:
  `accent`, `muted`, `subtle`, `danger`, `labelFg`, `valueFg`,
  `selectedBg`, `borderDim`, each as `lipgloss.AdaptiveColor{Light,
  Dark}`. Replace raw `lipgloss.Color("...")` call sites.
- `internal/tui/view.go` — grep for `lipgloss.NewStyle()` and
  hardcoded colors; redirect to the new semantic styles. Add 1-cell
  inner padding to the sidebar block (`sidebarStyle`). Switch sidebar
  width from the fixed `26` constant to `max(24, m.width * 0.25)`
  clamped to `[24, 40]`.
- `internal/tui/model.go` — hold a `help.Model` (from
  `github.com/charmbracelet/bubbles/help`). On every `WindowSizeMsg`
  update its width.
- `internal/tui/keys.go` *(new)* — define a `keyMap` struct
  implementing `help.KeyMap`. Group bindings for `help.Model`'s full
  view. Replace the hand-rolled status-bar hint string.
- `internal/tui/help.go` — fold existing help content into the new
  `keyMap`; keep the overlay as the "full" view, status bar as short.

**Not touching.** Any handler logic. Any form layout. Any config.

**Verify.**
- `go build ./... && go test ./...` clean.
- Visual walkthrough: light-terminal test (`COLORFGBG=15;0`),
  dark-terminal test, 80-col terminal, 200-col terminal. Sidebar
  width scales; names fit on 200-col. Help footer never truncates
  bindings — only ellipses the last entries.

---

## Phase 2 — Picker + command bar + leader menu

**Goal.** Replace cursor-context-branching letters (`n`/`e`/`d`/`l`)
with explicit entry points. Highest UX leverage; no config changes.
Resolves audit findings 3, 5, 6 and much of 9.

**Critical files.**
- `internal/tui/picker/picker.go` *(new package)* — the one reusable
  fuzzy picker. Backed by `github.com/charmbracelet/bubbles/list` and
  `github.com/sahilm/fuzzy`. API:
  ```go
  type Item interface {
      Title() string
      Subtitle() string          // for the match row
      Preview() string           // right-side preview
      Keywords() []string        // host, user, tag, group — all searched
  }
  func New(items []Item, title string, onSelect func(Item) tea.Cmd) Model
  ```
  Centered modal, query at top, list on left, preview on right.
- `internal/tui/cmdbar.go` *(new)* — the `:` command bar.
  Registry of commands with a one-line description; `:` opens a thin
  input strip above the status bar; typing filters. Initial command
  set: `conn`, `db`, `tags`, `fwd`, `recent`, `help`, `settings`,
  `import`, `sync`.
- `internal/tui/leader.go` *(new)* — helix-style `space` leader. An
  overlay showing every next key and its action; selecting opens the
  bound action. Reads from the same `keyMap` as `?`, so bindings only
  need to be declared once.
- `internal/tui/update.go` — rewrite the sidebar key dispatch. New
  flow: `/` → filter mode (unchanged); `:` → cmd bar; `space` →
  leader. Remove `n`/`e`/`d`/`l`/`t` as top-level letters; keep as
  bindings inside the leader menu with the same letters. Palette
  returns an action, action routes.
- `internal/tui/handlers.go` — delete the chooser form
  (`formNewChooser`) and the add-ssh-import form path through it:
  both reachable from `:import` and the leader menu now.
- `internal/tui/help.go` — regenerate help from the new `keyMap`
  entries + the command bar registry so `?` and `:help` stay in sync.

**Design decisions.**
- Picker *subtitle* shows the group path (`prod/web/web-1`), so sort
  order can be alpha but search matches group+host simultaneously
  (prior-art finding 3).
- `:conn` sets the sidebar scope to connections; `:db` to databases;
  `:recent` to the frecency section. These are **sidebar modes**,
  not pop-ups.
- Picker preview renders the same tabbed detail content for a
  Connection row, so the user is pre-reading what they'd see after
  selecting.

**Not touching.** Form field layout (still right-pane for now — Phase
3 moves them into palettes). Config schema.

**Verify.**
- `go build ./... && go test ./...` clean.
- Manual: `space` shows every binding; selecting `c` triggers connect
  flow; `:conn` jumps to connections view; `:db` to databases. Picker
  on 1k synthetic hosts stays smooth (test with a seeded config).
- Help overlay (`?`) and command bar (`:help`) show the same keymap.

---

## Phase 3 — Tabbed detail + floating palettes

**Goal.** Stop replacing the detail view with forms. Palette forms;
tabbed right pane. Resolves audit findings 1, 2, 8.

**Critical files.**
- `internal/tui/detail.go` *(new, split from `view.go`)* — the right
  pane's tab controller. Tabs: `Details`, `Databases`, `Scripts`,
  `Forwards`, `Notes`. `tab` / `shift-tab` cycle; mouse click on a
  tab works too (click already added this session). Each tab is its
  own `render*` method.
- `internal/tui/palette.go` *(new)* — a generic floating-palette
  component. Centered, soft border, dim backdrop (overlay-style —
  render the detail pane, then paint the palette on top of it). Carry
  a title bar, a body, and a footer hint strip.
- `internal/tui/forms/connection.go` *(new, split from
  `handlers.go` + `view.go`)* — the add/edit-connection palette.
  Fields rendered in **tab-navigable groups** (`basic`, `auth`,
  `advanced`, `scripts`), not a flat list. Delete the nav↔edit
  toggle: `tab` moves focus, the focused `textinput.Model` *is* the
  cursor target. Cycle fields render as inline chip rows with `h`/`l`
  (or click) to cycle.
- `internal/tui/forms/{database,group,script,notes,import}.go` —
  mirror the connection palette for each form currently in
  `handlers.go`.
- `internal/tui/update.go` — route each existing `formXxx` state into
  its new palette instead of the right-pane renderer. Keep the
  `formNone`/`formXxx` enum (less churn) but wire renders through
  `palette.Model`.

**Design decisions.**
- Palette width = `min(80, m.width - 8)`. Palette height = fit content,
  cap at `m.height - 4`. Scrollable inside if content exceeds.
- Backdrop: render detail pane with 40% opacity by dimming its style's
  foreground; the palette on top gets full-brightness styles so the
  focus pop is obvious.
- Mode chip switches to `EDIT` while a palette is open; `BROWSE` when
  closed.
- Host-key confirm, delete confirm, chooser → all small palettes.

**Not touching.** Config schema. Shell init editor still has the
textarea gap (separate follow-up, called out below).

**Verify.**
- `go build ./... && go test ./...` clean.
- Manual: open add-connection palette; the detail of the previously
  selected row is still visible under the dim. Resize the terminal
  while a palette is open — palette and detail both reflow. `esc`
  closes the palette; mode chip returns to `BROWSE`.
- `tab`/`shift-tab` cycles form fields with the cursor landing in the
  textinput directly (no `enter` to enter edit mode).

---

## Phase 4 — Multi-level group inheritance

**Goal.** Group is an inheritance scope **and groups themselves
nest**: a group has an optional parent group, forming a tree.
Defaults flow root → leaf → connection. Deletes `UseGlobalSettings`,
`UseGlobalShellInit`, and the per-connection advanced fields that are
really defaults. The "gap in the market" move; also the one with
real config-schema implications.

### Hierarchy model

- **Shape: tree, not DAG.** A group has **at most one parent**. No
  multiple inheritance. This keeps resolution O(depth) and keeps the
  mental model trivial ("where did this value come from?" has one
  answer).
- **Soft depth cap.** No hard limit, but we validate ≤ 6 levels on
  save and warn beyond that. Beyond 4 the sidebar's indentation
  stops rendering extra levels of indent and uses breadcrumb
  rendering (`prod › us-east › web › api`) in the Details tab
  instead.
- **Parent reference by UUID, display by name.** Groups get IDs like
  connections do, so renaming `prod` doesn't rewrite every child's
  `parent:` field. Yaml stores UUIDs; the TUI never shows them.
- **Uniqueness.** Group *name* is unique within its parent (siblings
  can't collide). Full paths are globally unique by construction.
  Two different `web` groups under different parents are fine
  (`prod/web`, `staging/web`).
- **Resolution order.** `connection → parent group → grandparent →
  … → root group → global`. First non-empty value wins (same rule
  the current `MergeSSHOptions` uses, just applied up a chain).
- **Cycle protection.** On every save (and on config load), walk the
  parent chain and refuse to persist if a cycle is detected or if a
  parent UUID doesn't resolve. The TUI prevents the obvious cases
  (can't set a group's parent to itself or a descendant) in the
  group-edit palette.

### Config shape (schema v2)

```yaml
schema: 2

ssh_options: { ... }          # global defaults (unchanged in spirit)
global_shell_init: ...

groups:
  - id: 01HQX...
    name: prod
    parent: ""                # root
    user: deploy
    identity_file: ~/.ssh/prod_rsa
    ssh_options: { forward_agent: true }

  - id: 01HQY...
    name: us-east
    parent: 01HQX...
    ssh_options:
      local_forward: ["5432:localhost:5432"]

  - id: 01HQZ...
    name: web
    parent: 01HQY...

connections:
  - id: ...
    name: web-1
    host: 10.0.4.12
    group: 01HQZ...           # points at a leaf group's UUID
```

The config stays a flat `groups` list — nesting is implicit via
`parent` edges. Reading it as a tree is a one-liner (`O(N)` build);
writing it keeps the file diff-friendly (no deep YAML nesting that
reorders on save).

### UI rendering

- **Sidebar.** Each group row is indented by `depth × 2` cells, with
  a chevron (`▸`/`▾`) toggleable per group. Collapsed state persists
  per group. On a 40-col-wide sidebar with 4+ levels of indent, we
  switch to a compact `prod › us-east › web/` label on the row and
  stop indenting (prevents infinite rightward drift).
- **Fuzzy picker.** Match score accumulates across every path
  segment: `pr web 1` hits `prod/web/web-1`. Subtitle shows the full
  path; the match row shows just the leaf name.
- **Details tab.** Each inherited row annotates its source:
  `User   deploy   — from prod/`. Overridden rows show their
  parent's value dimmed next to them: `IdentityFile   ~/.ssh/dev   
  (parent: ~/.ssh/prod_rsa)`. This makes "where did this come from"
  readable at a glance (Termius does this; most SSH TUIs don't).
- **Breadcrumb in context strip.** The top strip's `hangar ▸ prod ▸
  us-east ▸ web ▸ web-1` shows depth live.
- **Group-edit palette.** Fields are the same as a connection's
  advanced block. The `Parent` field uses the fuzzy picker scoped to
  "all groups except self + descendants." Changing parent re-roots
  the subtree; children's effective values recompute on save.

### Migration from v1

v1 has:
- `cfg.Groups []string` (flat, name-only).
- `cfg.GroupShellInit map[string]string`.
- Per-connection `UseGlobalSettings`, `UseGlobalShellInit`.

Migration (`internal/config/migrate.go`):
1. Promote each `cfg.Groups[i]` string to `Group{ID: uuid.New(),
   Name: s, Parent: ""}`. Everyone starts as a root group — no
   automatic nesting from v1 data (there's no signal for it in v1).
2. Move each `GroupShellInit[name]` into the matching group's
   `ShellInit` field.
3. For every connection with `UseGlobalSettings == false`, copy the
   current effective `MergeSSHOptions(global, conn)` result onto the
   connection as explicit overrides. Then clear the toggle. Result:
   bit-identical ssh invocations after migration.
4. For every connection with `UseGlobalShellInit == false`, same
   deal with shell init.
5. Rewrite `connection.Group` from the group *name* to the group
   *UUID*.
6. Set `schema: 2`. Back up the original to `config.yaml.v1.bak`.

v1 refuses to start on a `schema: 2` file ("hangar v1 can't read a v2
config; run `hangar --ui v2`") so the user doesn't accidentally
clobber the nested structure.

### Critical files (updates to the Phase 4 section above)

- `internal/config/types.go` — `Group` now has `ID uuid.UUID`,
  `Parent uuid.UUID` (nullable via `uuid.Nil`). `Connection.Group`
  becomes `uuid.UUID`.
- `internal/config/resolve.go` *(new)* — the up-the-chain resolver
  described above. Walks `conn.Group → group.Parent → …` collecting
  non-empty fields; adds global defaults last. Returns the flattened
  `*Connection` the ssh code already expects, plus a parallel
  `map[field]scopePath` for the Details tab's "— from X" annotation.
- `internal/config/validate.go` *(new)* — cycle check, orphan check,
  depth warning. Runs on every `Save`.
- `internal/config/migrate.go` — the v1→v2 migration.
- `internal/tui/v2/sidebar.go` — tree render + collapse state per
  group ID + overflow breadcrumb for deep paths.
- `internal/tui/v2/forms/group.go` — group palette with the fuzzy
  Parent picker.

### Deferred within Phase 4

- **Moving a connection between groups.** A new "move to…" action in
  the leader menu (`space m`) that opens the group picker. Not
  strictly required at Phase 4 landing but a natural follow-up.
- **Group-level scripts.** Already in the type sketch; needs its own
  little resolver pass so a connection's effective script list is
  parent-scripts + own-scripts. One extra section in the Scripts tab
  ("inherited from `prod/`") handles the UI.

**Not touching.** The actual ssh invocation (`internal/ssh/session.go`
etc.) stays unchanged — it just gets the already-resolved connection
back from `ResolveEffective`.

**Verify.**
- `go test ./internal/config/...` — new tests covering: round-trip
  v1→v2 migrate, resolve with no parents / one parent / 3-deep
  chain, parent-override wins, no-parent-no-group falls back to
  global, cycle detection refuses to save, orphan-parent refuses to
  save, deep-chain warning triggers at depth > 6.
- Manual: load an existing v1 config, confirm `.v1.bak` written,
  confirm `go run ./cmd/hangar --ui v2` connects identically to all
  existing connections (diff generated effective ssh args against
  pre-migration). Create a 3-deep chain (`prod → us-east → web`),
  set `user: deploy` on `prod`, `local_forward` on `us-east`, verify
  a `web/web-1` connection inherits both and the Details tab shows
  "— from prod/" and "— from prod/us-east/" against the right rows.

---

## Deferred (explicitly out of scope)

- **Shell-init multi-line editor and script form input** are still
  append-only (audit finding 10). Migrate to `bubbles/textarea`
  separately; the picker + palette work in Phase 2/3 is the
  prerequisite.
- **Pin / favourites / frecency `Recent` section.** The layout shows
  it, but the ranking data (last-used, use-count) isn't tracked yet.
  Add after Phase 4; needs a small state file.
- **Server-mode directory (wishlist's trick).** Interesting but it's
  a separate program shape — park it.
- **`on_connect` scripts (assh's trick).** Could layer on top of the
  existing Scripts primitive once group inheritance lands.

## Rollout order and risk

| Phase | Risk | User-visible win |
| --- | --- | --- |
| 1 — theme + help footer + proportional sidebar | low | polish, correct status bar, no name truncation |
| 2 — picker + `:` + `space` leader | medium | **biggest UX leap**; removes the overloaded-letter cliff |
| 3 — tabbed detail + floating palettes | medium | detail stays visible while editing; small-terminal fixed |
| 4 — group inheritance | **high** (schema migration) | the "gap in the market" move; forms shrink dramatically |

Phase 1 and 2 together resolve ~60% of audit findings. Each phase
leaves `main` shippable.
