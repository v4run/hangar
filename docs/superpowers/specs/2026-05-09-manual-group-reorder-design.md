# Manual Group Reordering — Design

## Problem

Groups in the sidebar are sorted alphabetically. Users want to control the order of groups manually so the sidebar reflects their own mental model (e.g., "prod" above "dev", or by region/team) instead of being driven by name.

Connection-level reordering with `J/K` already exists today and works within a group. This spec only changes the **group order** mechanism.

## Out of scope

These were explicitly excluded during brainstorming:

- Cross-group connection movement via `J/K` (use `x`/`y`/`p` as today).
- Reorderable ungrouped section — ungrouped connections remain pinned at the top.
- Replacing all sorting / a unified manual order across groups + connections + ungrouped.
- Drag-style visual reorder mode.

## User-facing behavior

- Cursor on a **group header** + `J` → group moves down one position; cursor follows it.
- Cursor on a **group header** + `K` → group moves up one position; cursor follows it.
- Cursor on a **connection** + `J/K` → existing behavior, swap with neighbor in `cfg.Connections` (unchanged).
- New manual order persists to `connections.yaml` immediately on swap.
- Ungrouped connections always render before any group (unchanged).

## Storage

Replace `Groups map[string]bool` with an ordered slice. The slice is the single source of truth for both "this group exists" (including empty groups) and "this is the render order".

```go
type HangarConfig struct {
    Connections   []Connection `yaml:"connections"`
    SSHSync       SSHSync      `yaml:"ssh_sync"`
    GlobalScripts []Script     `yaml:"global_scripts,omitempty"`
    Groups        GroupList    `yaml:"groups,omitempty"`
}

type GroupList []string
```

`GroupList` is a named type aliasing `[]string` so we can attach `UnmarshalYAML` without disturbing existing call sites that range/append over the field.

### Back-compat for existing configs

Existing user configs serialize `groups` as a map (`groups: {prod: true, dev: true}`). A custom `UnmarshalYAML` on `GroupList` accepts both forms:

- `yaml.SequenceNode` → decode straight into `[]string`.
- `yaml.MappingNode` → decode into `map[string]bool`, take the keys, sort alphabetically (deterministic first migration that matches today's display order), assign.
- Other node kinds → return an error.

The first `Save` after load rewrites the file in slice form, so the map representation is encountered at most once per existing config.

### Migration

Extend `HangarConfig.Migrate()` to handle two cases:

1. `Groups` is empty but `Connections` reference group names → append all distinct names sorted alphabetically. Preserves orphan groups that exist only via connection references.
2. A connection references a group not currently in `Groups` → append it. Catches user edits to the YAML that introduce groups outside the TUI.

Both cases set `changed = true` so the migration writes back.

## Render path

In `internal/tui/helpers.go::sidebarItems()`:

- Keep the ungrouped-first block as-is.
- Replace the existing "collect groups in order of first appearance / sort alphabetically / merge empty groups" block with a single iteration over `m.cfg.Groups`:

```go
for _, g := range m.cfg.Groups {
    items = append(items, sidebarItem{isGroup: true, group: g})
    if !m.collapsed[g] {
        for i := range conns {
            if conns[i].Group == g {
                c := conns[i]
                items = append(items, sidebarItem{conn: &c})
            }
        }
    }
}
```

Drop the `groupSeen` map, the `sort.Strings(groupOrder)` call, and the empty-group merge loop — `cfg.Groups` is authoritative.

## Reorder handlers

In `internal/tui/update.go`, expand the `J` and `K` cases (currently in the normal-sidebar `switch msg.String()` block):

```go
case "J":
    items := m.sidebarItems()
    if m.cursor < len(items) {
        item := items[m.cursor]
        if item.isGroup {
            m.swapGroupOrder(item.group, +1)
        } else if m.cursor < len(items)-1 {
            m.swapSidebarItems(m.cursor, m.cursor+1)
            m.cursor++
        }
        m.cursor = m.locateCursor(item)
        m.adjustSidebarViewport()
    }
case "K":
    items := m.sidebarItems()
    if m.cursor < len(items) && m.cursor > 0 {
        item := items[m.cursor]
        if item.isGroup {
            m.swapGroupOrder(item.group, -1)
        } else {
            m.swapSidebarItems(m.cursor, m.cursor-1)
            m.cursor--
        }
        m.cursor = m.locateCursor(item)
        m.adjustSidebarViewport()
    }
```

Two new helpers in `helpers.go`:

- `func (m *Model) swapGroupOrder(name string, dir int)`:
  - Find `name` in `m.cfg.Groups`, swap with neighbor at `idx+dir` if in range.
  - On success, `config.Save(m.configDir, m.cfg)`.
- `func (m Model) locateCursor(item sidebarItem) int`:
  - Re-derive flat list, find the index of `item` (by group name for headers, by connection ID for connections), return it. Clamps to last index if not found.

This keeps the cursor visually anchored to the same logical thing the user just moved.

## Group CRUD changes

In `internal/tui/handlers.go`, three handlers currently mutate `m.cfg.Groups` as a map. They become slice ops via a new helper `groupIndex(groups, name) int` (returns `-1` if absent):

**`handleAddGroupInput`**
- Collision check today only iterates `cfg.Connections`, so empty-but-registered groups slip through. Replace with `groupIndex(m.cfg.Groups, name) >= 0` — fixes the gap in passing since the slice now tracks empty groups as first-class.
- On accept: `m.cfg.Groups = append(m.cfg.Groups, name)` (new group lands at the bottom of the order).

**`handleEditGroupInput`**
- Today's map model implicitly merges if `newName` already exists (set semantics + connections re-grouping). The slice must replicate that:
  - If `groupIndex(m.cfg.Groups, newName) < 0` → replace `oldName` at its index with `newName` (preserves order).
  - Else → remove `oldName`'s entry; connections previously in `oldName` get rewritten to `newName` and merge into the existing group's position. Same outcome the map produced.
- `m.collapsed` rekeying (existing logic at handlers.go:454-458) stays as-is — covers both branches.

**`handleDeleteGroupConfirm`**
- Find index, splice it out: `m.cfg.Groups = append(m.cfg.Groups[:i], m.cfg.Groups[i+1:]...)`.

The `nil`-check on `m.cfg.Groups` becomes a `len()==0` check; nil slices append fine in Go.

## Help and docs

- `internal/tui/help.go`: change `{"J / K", "reorder items"}` → `{"J / K", "reorder connection or group"}`.
- `README.md`:
  - Remove "Groups are always sorted alphabetically." (line 127).
  - Add a sentence: "Use `J`/`K` on a group header to reorder groups; on a connection it swaps with its neighbor in the same group."

## Tests

- `config/types_test.go` (new file or extend existing):
  - `UnmarshalYAML` over a sequence form: round-trips.
  - `UnmarshalYAML` over a legacy mapping form: returns alphabetically-ordered slice.
  - `UnmarshalYAML` over an unexpected node kind: returns error.
- `config/config_test.go`:
  - `Migrate()` populates `Groups` from connection-referenced groups when slice is empty.
  - `Migrate()` appends a group referenced by a connection but missing from `Groups`.
  - `Migrate()` does not reorder existing `Groups` content (idempotent on a healthy config).
- TUI: existing TUI test scaffolding will be checked during planning. If absent, `swapGroupOrder` is small enough that a unit test on the model directly is sufficient.

## Risks / things to verify during implementation

- Persisting on every swap means many writes if user holds `J`. The TUI already does this for connection swaps and for many other ops; matching that pattern is fine.
- `m.collapsed[group]` keying survives reorders unchanged (only group names change in collapsed map, and only on rename).
- Sidebar viewport (`adjustSidebarViewport`) needs the post-reflow cursor before being called; the handler order above ensures that.
- Rename merge: when renaming into an existing group, the merged group keeps its original position. Confirm this matches user expectation — the alternative (move into `oldName`'s position) is also defensible but feels more surprising.
