# Manual Group Reordering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users reorder sidebar groups manually via `J/K` on a group header, persisted to `connections.yaml`.

**Architecture:** Replace `HangarConfig.Groups map[string]bool` with an ordered `GroupList []string`. A custom `UnmarshalYAML` on `GroupList` accepts both legacy map and new sequence forms so existing configs continue to load. The sidebar render path iterates `cfg.Groups` directly instead of sorting alphabetically. `J/K` on a group header swaps adjacent entries in the slice; on a connection it preserves today's behavior.

**Tech Stack:** Go 1.x, `gopkg.in/yaml.v3`, Bubbletea, Lipgloss.

**Spec:** `docs/superpowers/specs/2026-05-09-manual-group-reorder-design.md`

---

## File Structure

**Files to modify:**
- `internal/config/types.go` — add `GroupList` type with `UnmarshalYAML`; change `HangarConfig.Groups` field type.
- `internal/config/config.go` — extend `Migrate()` to populate `Groups` from connection-referenced groups.
- `internal/tui/helpers.go` — simplify `sidebarItems()` to iterate `cfg.Groups`; add `groupIndex`, `swapGroupOrder`, `locateCursor` helpers.
- `internal/tui/update.go` — extend `J` and `K` cases to handle group headers.
- `internal/tui/handlers.go` — switch `handleAddGroupInput`, `handleEditGroupInput`, `handleDeleteGroupConfirm` from map ops to slice ops.
- `internal/tui/help.go` — update the help-line description for `J / K`.
- `README.md` — replace the "always sorted alphabetically" note with manual-reorder docs.

**Files to create:**
- `internal/config/types_test.go` — round-trip and back-compat tests for `GroupList`.

---

## Task 1: `GroupList` type and `UnmarshalYAML`

Defines the new ordered type for groups and a custom unmarshaler that accepts both the legacy map form and the new sequence form. Driven by tests.

**Files:**
- Create: `internal/config/types_test.go`
- Modify: `internal/config/types.go`

- [ ] **Step 1: Write the failing test (sequence form round-trip)**

Create `internal/config/types_test.go` with:

```go
package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGroupListUnmarshalSequence(t *testing.T) {
	const data = `
connections: []
groups:
  - prod
  - staging
  - dev
`
	var cfg HangarConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := GroupList{"prod", "staging", "dev"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len: got %d, want %d", len(cfg.Groups), len(want))
	}
	for i, g := range want {
		if cfg.Groups[i] != g {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], g)
		}
	}
}
```

- [ ] **Step 2: Run the test, expect a build failure**

Run: `go test ./internal/config/ -run TestGroupListUnmarshalSequence`
Expected: build error — `GroupList` undefined.

- [ ] **Step 3: Add `GroupList` type and change the field**

Edit `internal/config/types.go`. Change the `HangarConfig` struct (currently lines 52-57):

```go
type HangarConfig struct {
	Connections   []Connection `yaml:"connections"`
	SSHSync       SSHSync      `yaml:"ssh_sync"`
	GlobalScripts []Script     `yaml:"global_scripts,omitempty"`
	Groups        GroupList    `yaml:"groups,omitempty"`
}

// GroupList is an ordered list of group names. It accepts both legacy
// `map[string]bool` and the new `[]string` YAML forms during unmarshal.
type GroupList []string
```

- [ ] **Step 4: Run the test, expect it to pass**

Run: `go test ./internal/config/ -run TestGroupListUnmarshalSequence`
Expected: PASS. (The default sequence decode path works without a custom unmarshaler — that's intentional; the next step exercises the legacy form.)

- [ ] **Step 5: Write the failing test (legacy map form)**

Append to `internal/config/types_test.go`:

```go
import "sort"

func TestGroupListUnmarshalLegacyMap(t *testing.T) {
	const data = `
connections: []
groups:
  prod: true
  staging: true
  dev: true
`
	var cfg HangarConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Groups) != 3 {
		t.Fatalf("len: got %d, want 3", len(cfg.Groups))
	}
	got := []string(cfg.Groups)
	sort.Strings(got) // we expect deterministic alphabetical order
	want := []string{"dev", "prod", "staging"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
```

If the file already has imports, merge `"sort"` into the existing import block instead of adding a second one.

- [ ] **Step 6: Run the test, expect it to fail with a YAML error**

Run: `go test ./internal/config/ -run TestGroupListUnmarshalLegacyMap`
Expected: FAIL — YAML decoder reports a type mismatch (cannot unmarshal mapping into `GroupList`/`[]string`).

- [ ] **Step 7: Add the custom unmarshaler**

Edit `internal/config/types.go`. Add at the bottom of the file:

```go
// UnmarshalYAML accepts both the new sequence form (`[a, b, c]`) and the
// legacy mapping form (`{a: true, b: true}`). Mapping form is sorted
// alphabetically for deterministic ordering on first migration.
func (g *GroupList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var s []string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*g = s
	case yaml.MappingNode:
		var m map[string]bool
		if err := node.Decode(&m); err != nil {
			return err
		}
		names := make([]string, 0, len(m))
		for k := range m {
			names = append(names, k)
		}
		sort.Strings(names)
		*g = names
	default:
		return fmt.Errorf("groups: unexpected YAML node kind %d", node.Kind)
	}
	return nil
}
```

Add the imports `"fmt"`, `"sort"`, and `"gopkg.in/yaml.v3"` to the existing import block in `types.go` (the file currently imports `"time"` and `"github.com/google/uuid"`).

- [ ] **Step 8: Run both tests, expect them to pass**

Run: `go test ./internal/config/ -run TestGroupListUnmarshal`
Expected: PASS for both `TestGroupListUnmarshalSequence` and `TestGroupListUnmarshalLegacyMap`.

- [ ] **Step 9: Write the failing test (invalid node kind)**

Append to `internal/config/types_test.go`:

```go
func TestGroupListUnmarshalInvalidKind(t *testing.T) {
	const data = `
connections: []
groups: "not-a-list"
`
	var cfg HangarConfig
	err := yaml.Unmarshal([]byte(data), &cfg)
	if err == nil {
		t.Fatalf("expected error for scalar groups value, got nil")
	}
}
```

- [ ] **Step 10: Run, expect pass (the default path already errors)**

Run: `go test ./internal/config/ -run TestGroupListUnmarshalInvalidKind`
Expected: PASS — `default` branch returns an error for scalar input.

- [ ] **Step 11: Run the full config package test suite**

Run: `go test ./internal/config/`
Expected: All tests PASS. (No existing test in `config_test.go` should break — `Groups` was already `omitempty` and the test fixtures don't populate it.)

- [ ] **Step 12: Commit**

```bash
git add internal/config/types.go internal/config/types_test.go
git commit -m "feat(config): add GroupList type with legacy-map unmarshal"
```

---

## Task 2: Make TUI consumers compile against `GroupList`

Switching the field from a map to a slice breaks every call site that does `for k := range m.cfg.Groups`, `m.cfg.Groups[name] = true`, or `delete(...)`. Update them in one task to keep the tree green.

**Files:**
- Modify: `internal/tui/helpers.go`
- Modify: `internal/tui/handlers.go`

- [ ] **Step 1: Verify the build is broken in the expected places**

Run: `go build ./...`
Expected: compile errors in `internal/tui/helpers.go` (around line 99) and `internal/tui/handlers.go` (around lines 410-411, 450-452, 484-485).

- [ ] **Step 2: Add `groupIndex` helper**

Edit `internal/tui/helpers.go`. Add this function near the bottom of the file (after `formatScriptDuration`, before `filteredSyncEntries`):

```go
// groupIndex returns the position of name in groups, or -1 if absent.
func groupIndex(groups []string, name string) int {
	for i, g := range groups {
		if g == name {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 3: Update `sidebarItems()` to iterate `cfg.Groups`**

Edit `internal/tui/helpers.go`. Replace the `sidebarItems()` body (currently lines 73-122). The new version drops the alphabetical sort and the empty-groups merge loop:

```go
// sidebarItems builds a flat list of groups and connections for the sidebar.
// Ungrouped connections come first, then groups in cfg.Groups order.
func (m Model) sidebarItems() []sidebarItem {
	conns := m.filteredConnections()

	var ungrouped []config.Connection
	for i := range conns {
		if conns[i].Group == "" {
			ungrouped = append(ungrouped, conns[i])
		}
	}

	var items []sidebarItem
	for i := range ungrouped {
		items = append(items, sidebarItem{conn: &ungrouped[i]})
	}

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

	return items
}
```

Remove the `"sort"` import from `helpers.go`. `sort.Strings(groupOrder)` was the only use in this file; the build will fail with "imported and not used" otherwise.

- [ ] **Step 4: Update `handleAddGroupInput` (around line 388 in handlers.go)**

Edit `internal/tui/handlers.go`. Replace the `case "enter":` block inside `handleAddGroupInput` (currently lines 392-415):

```go
	case "enter":
		name := strings.TrimSpace(m.groupNameInput)
		if name == "" {
			m.formError = "group name is required"
			return m, nil
		}
		if groupIndex(m.cfg.Groups, name) >= 0 {
			m.formError = "group already exists"
			return m, nil
		}
		m.collapsed[name] = false
		m.cfg.Groups = append(m.cfg.Groups, name)
		config.Save(m.configDir, m.cfg)
		m.form = formNone
```

The connection-iteration collision check is removed: `cfg.Groups` is authoritative now (catches both populated and empty groups).

- [ ] **Step 5: Update `handleEditGroupInput` (around line 428 in handlers.go)**

Edit `internal/tui/handlers.go`. Replace the body inside `case "enter":` after the early-return for unchanged name (currently lines 443-459):

```go
		// Rename group across all connections
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].Group == oldName {
				m.cfg.Connections[i].Group = newName
			}
		}
		// Update Groups slice. If newName already exists, drop the old entry
		// (connections merge into the existing group's position).
		if oldIdx := groupIndex(m.cfg.Groups, oldName); oldIdx >= 0 {
			if groupIndex(m.cfg.Groups, newName) < 0 {
				m.cfg.Groups[oldIdx] = newName
			} else {
				m.cfg.Groups = append(m.cfg.Groups[:oldIdx], m.cfg.Groups[oldIdx+1:]...)
			}
		}
		// Update collapsed state
		if wasCollapsed, ok := m.collapsed[oldName]; ok {
			delete(m.collapsed, oldName)
			m.collapsed[newName] = wasCollapsed
		}
		config.Save(m.configDir, m.cfg)
		m.form = formNone
```

- [ ] **Step 6: Update `handleDeleteGroupConfirm` (around line 473 in handlers.go)**

Edit `internal/tui/handlers.go`. Replace lines 484-486 (the `if m.cfg.Groups != nil { delete(...) }` block) with:

```go
		if idx := groupIndex(m.cfg.Groups, m.formTargetGroup); idx >= 0 {
			m.cfg.Groups = append(m.cfg.Groups[:idx], m.cfg.Groups[idx+1:]...)
		}
```

- [ ] **Step 7: Build the project**

Run: `go build ./...`
Expected: success, no compile errors.

- [ ] **Step 8: Run the full test suite**

Run: `go test ./...`
Expected: all PASS. The TUI has no tests; the config tests from Task 1 still pass.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/helpers.go internal/tui/handlers.go
git commit -m "feat(tui): switch group storage to ordered slice"
```

---

## Task 3: Migrate existing configs

Add migration logic so configs that already exist on disk get a populated `Groups` slice on first load. Two cases:
1. `Groups` is empty but connections reference groups → backfill from connections, alphabetical.
2. A connection references a group missing from `Groups` → append it.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test (backfill from connections)**

Edit `internal/config/config_test.go`. Append:

```go
func TestMigrateBackfillsGroups(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "a", Host: "h", Port: 22, User: "u", Group: "prod"},
			{ID: uuid.New(), Name: "b", Host: "h", Port: 22, User: "u", Group: "dev"},
			{ID: uuid.New(), Name: "c", Host: "h", Port: 22, User: "u", Group: "prod"},
		},
	}
	if !cfg.Migrate() {
		t.Fatal("expected Migrate to report changes")
	}
	want := []string{"dev", "prod"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len: got %d, want %d", len(cfg.Groups), len(want))
	}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run the test, expect FAIL**

Run: `go test ./internal/config/ -run TestMigrateBackfillsGroups`
Expected: FAIL — `Groups` stays empty after `Migrate()`.

- [ ] **Step 3: Extend `Migrate()` to backfill Groups**

Edit `internal/config/config.go`. Append to the body of `Migrate()` (currently ends at line 174 with `return changed`), inserting before `return changed`:

```go
	// Backfill Groups slice from connection-referenced groups.
	have := make(map[string]bool, len(cfg.Groups))
	for _, g := range cfg.Groups {
		have[g] = true
	}
	var missing []string
	for _, c := range cfg.Connections {
		if c.Group != "" && !have[c.Group] {
			have[c.Group] = true
			missing = append(missing, c.Group)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		cfg.Groups = append(cfg.Groups, missing...)
		changed = true
	}
```

Add `"sort"` to the imports in `config.go` if not already present.

- [ ] **Step 4: Run the test, expect PASS**

Run: `go test ./internal/config/ -run TestMigrateBackfillsGroups`
Expected: PASS.

- [ ] **Step 5: Write the failing test (orphan group reference)**

Append to `internal/config/config_test.go`:

```go
func TestMigrateAppendsOrphanGroup(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "a", Host: "h", Port: 22, User: "u", Group: "newgroup"},
		},
		Groups: GroupList{"existing"},
	}
	if !cfg.Migrate() {
		t.Fatal("expected Migrate to report changes")
	}
	want := []string{"existing", "newgroup"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len: got %d, want %d (Groups=%v)", len(cfg.Groups), len(want), cfg.Groups)
	}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}
```

- [ ] **Step 6: Run the test, expect PASS**

Run: `go test ./internal/config/ -run TestMigrateAppendsOrphanGroup`
Expected: PASS — the same code path from Step 3 already covers this case (the `have` map seeds from existing entries, missing names append at the end).

- [ ] **Step 7: Write the idempotency test**

Append to `internal/config/config_test.go`:

```go
func TestMigrateIdempotentOnHealthyConfig(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "a", Host: "h", Port: 22, User: "u", Group: "prod"},
		},
		Groups: GroupList{"prod", "dev"},
	}
	if cfg.Migrate() {
		t.Fatal("expected Migrate to be a no-op on healthy config")
	}
	want := []string{"prod", "dev"}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}
```

- [ ] **Step 8: Run, expect PASS**

Run: `go test ./internal/config/ -run TestMigrateIdempotentOnHealthyConfig`
Expected: PASS — no missing names, no UUID/JumpHost changes, so `Migrate` returns false.

- [ ] **Step 9: Run the full config suite**

Run: `go test ./internal/config/`
Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): migrate existing configs to populate Groups slice"
```

---

## Task 4: J/K reorder on group headers

Add the helpers and wire `J`/`K` to swap group order when the cursor is on a group header. Keep cursor tracking the same logical item across the reflow.

**Files:**
- Modify: `internal/tui/helpers.go`
- Modify: `internal/tui/update.go`

- [ ] **Step 1: Add `swapGroupOrder` and `locateCursor` helpers**

Edit `internal/tui/helpers.go`. Add after the existing `swapSidebarItems` function (currently ends around line 465):

```go
// swapGroupOrder swaps the named group with its neighbor by `dir` (-1 or +1)
// in cfg.Groups. Persists on success.
func (m *Model) swapGroupOrder(name string, dir int) {
	idx := groupIndex(m.cfg.Groups, name)
	if idx < 0 {
		return
	}
	target := idx + dir
	if target < 0 || target >= len(m.cfg.Groups) {
		return
	}
	m.cfg.Groups[idx], m.cfg.Groups[target] = m.cfg.Groups[target], m.cfg.Groups[idx]
	config.Save(m.configDir, m.cfg)
}

// locateCursor returns the index of the given sidebar item in the (rebuilt)
// flat list. Used to keep the cursor anchored on the same logical item after
// a reorder reflows the sidebar. Returns clamped fallback if the item is
// somehow gone.
func (m Model) locateCursor(item sidebarItem) int {
	items := m.sidebarItems()
	for i, it := range items {
		if item.isGroup {
			if it.isGroup && it.group == item.group {
				return i
			}
			continue
		}
		if !it.isGroup && it.conn != nil && item.conn != nil && it.conn.ID == item.conn.ID {
			return i
		}
	}
	if len(items) == 0 {
		return 0
	}
	if m.cursor >= len(items) {
		return len(items) - 1
	}
	return m.cursor
}
```

- [ ] **Step 2: Update the `J` case in update.go**

Edit `internal/tui/update.go`. Replace the existing `case "J":` block (currently lines 432-439):

```go
		case "J":
			items := m.sidebarItems()
			if m.cursor >= len(items) {
				break
			}
			item := items[m.cursor]
			if item.isGroup {
				m.swapGroupOrder(item.group, +1)
				m.cursor = m.locateCursor(item)
			} else if m.cursor < len(items)-1 {
				m.swapSidebarItems(m.cursor, m.cursor+1)
				m.cursor++
			}
			m.adjustSidebarViewport()
```

Connection-swap branch keeps the original `m.cursor++` so that when the swap is a no-op at a group boundary, the cursor still advances onto the next group's header (matching today's behavior). The group-header branch uses `locateCursor` because the flat-list index of the moved group depends on neighboring group sizes.

- [ ] **Step 3: Update the `K` case in update.go**

Replace `case "K":` (currently lines 440-446):

```go
		case "K":
			if m.cursor == 0 {
				break
			}
			items := m.sidebarItems()
			if m.cursor >= len(items) {
				break
			}
			item := items[m.cursor]
			if item.isGroup {
				m.swapGroupOrder(item.group, -1)
				m.cursor = m.locateCursor(item)
			} else {
				m.swapSidebarItems(m.cursor, m.cursor-1)
				m.cursor--
			}
			m.adjustSidebarViewport()
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 6: Manual smoke test**

Run: `make run` (which builds and launches `./hangar`).

Verify:
- Two or more groups exist in the config.
- Cursor on a group header. Press `J` → group moves down with all its connections; cursor stays on that group's header in its new position.
- Press `K` → group moves back up; cursor follows.
- At the bottom group, `J` is a no-op.
- At the top group (first non-ungrouped), `K` is a no-op.
- On a connection inside a group, `J/K` still swaps with the adjacent connection in the same group (existing behavior preserved).
- Quit, restart — the new group order persists.

If any of these fail, fix before proceeding.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/helpers.go internal/tui/update.go
git commit -m "feat(tui): J/K on a group header reorders groups manually"
```

---

## Task 5: Update help text and README

**Files:**
- Modify: `internal/tui/help.go`
- Modify: `README.md`

- [ ] **Step 1: Update the help text**

Edit `internal/tui/help.go`. Find the line:

```go
				{"J / K", "reorder items"},
```

Replace with:

```go
				{"J / K", "reorder connection or group"},
```

- [ ] **Step 2: Update README**

Edit `README.md`. Find line 127:

```
Connections can be organized into collapsible groups. Groups are always sorted alphabetically. Collapsed groups show the connection count.
```

Replace with:

```
Connections can be organized into collapsible groups. Use `J`/`K` on a group header to reorder groups manually; on a connection it swaps with the neighbor in the same group. Collapsed groups show the connection count.
```

- [ ] **Step 3: Build to ensure nothing else broke**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Run the full test suite once more**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/help.go README.md
git commit -m "docs: describe manual group reordering in help and README"
```

---

## Done

Five commits land in order:
1. `feat(config): add GroupList type with legacy-map unmarshal`
2. `feat(tui): switch group storage to ordered slice`
3. `feat(config): migrate existing configs to populate Groups slice`
4. `feat(tui): J/K on a group header reorders groups manually`
5. `docs: describe manual group reordering in help and README`

After all commits, sanity-check by running `hangar` against an existing config that has groups. Confirm groups load, can be reordered, and the new order persists across restarts.
