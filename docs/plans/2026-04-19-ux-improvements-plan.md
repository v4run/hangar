# UX Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 14 UX improvements to the hangar TUI: scroll indicators, toasts, paste confirmation, connect banner, responsive status bar, empty state, section dividers, sync filter, multi-select, name truncation, tag tokens, script results, reorder, and help overlay.

**Architecture:** First split the monolithic `internal/tui/app.go` (2025 lines) into logical files within the same package. Then implement features in dependency order: foundation systems (viewport, toasts) first, then visual improvements, then interaction changes. Each feature gets its own file where appropriate.

**Tech Stack:** Go, Bubbletea (tea.Model/tea.Cmd/tea.Msg), Lipgloss (styling), existing config/ssh packages.

---

### Task 0: Split app.go into logical files

**Files:**
- Modify: `internal/tui/app.go` → split into multiple files
- Create: `internal/tui/model.go` — Model struct, constants, field definitions, NewModel
- Create: `internal/tui/update.go` — Update method, top-level message routing
- Create: `internal/tui/handlers.go` — All handle* methods (form input, sidebar, scripts, sync, etc.)
- Create: `internal/tui/view.go` — View method, all render* methods
- Create: `internal/tui/helpers.go` — Helper methods (jumpHost*, parseSSHOptions*, populateSSHOptions*, sidebarItems, filteredConnections, selectedConnection, allScripts, isGlobalScript, renderCycleOptions, boolYesNo)
- Keep: `internal/tui/app.go` — Run function only
- Keep: `internal/tui/styles.go` — unchanged

All files stay in `package tui`. No behavior changes — pure refactor.

**Verification:** `go build ./...` and `go test ./...` must pass unchanged.

**Commit:** `refactor: split tui/app.go into model, update, handlers, view, helpers`

---

### Task 1: Sidebar viewport with scroll indicators

**Files:**
- Create: `internal/tui/viewport.go`
- Modify: `internal/tui/model.go` — add viewport state fields
- Modify: `internal/tui/view.go` — renderSidebar uses viewport
- Modify: `internal/tui/update.go` — cursor movement adjusts viewport offset

**What to build:**
- Add fields to Model: `sidebarOffset int` (first visible index)
- Compute `visibleRows = contentHeight - 3` (filter + blank + indicator)
- On cursor move: if `cursor < sidebarOffset` set `sidebarOffset = cursor`; if `cursor >= sidebarOffset + visibleRows` set `sidebarOffset = cursor - visibleRows + 1`
- In renderSidebar: only render items[sidebarOffset : sidebarOffset+visibleRows]
- Render scroll indicators:
  - If sidebarOffset > 0: show `▲` at top-right of sidebar
  - If items extend below: show `▼` at bottom-right and `[N more ↓]` dim at bottom
  - Scrollbar thumb: compute position as `sidebarOffset / totalItems * visibleRows`, render `█` for thumb rows, `░` for track in rightmost column (use color 8)

**Commit:** `feat: sidebar viewport with scroll indicators`

---

### Task 2: Operation toast system

**Files:**
- Create: `internal/tui/toast.go`
- Modify: `internal/tui/model.go` — add toast fields
- Modify: `internal/tui/view.go` — renderStatusBar shows toast
- Modify: `internal/tui/handlers.go` — emit toasts after operations

**What to build:**
- Toast struct: `type toast struct { text string; kind int; expireAt time.Time }`
- Kind constants: `toastOK = 2, toastErr = 1, toastWarn = 3, toastInfo = 13`
- Model fields: `activeToast *toast`
- `toastMsg` type for tea.Msg; `clearToastMsg` type
- `func showToast(text string, kind int) tea.Cmd` — returns tea.Tick(2500ms) that sends clearToastMsg
- Update handles clearToastMsg → sets activeToast = nil
- renderStatusBar: if activeToast != nil, render `{glyph} {text}` in kind color on left, keep `enter:connect /:find q:quit` on right; else normal hints
- Glyphs: ok="✓", err="✗", warn="⚠", info="▸"
- Emit toasts from: saveForm ("saved {name}"), handleDeleteConfirm ("deleted {name}"), paste ("pasted N into {group}"), handleSyncInput ("imported N hosts"), connect failure on return

**Commit:** `feat: transient toast messages for operation feedback`

---

### Task 3: Responsive status bar

**Files:**
- Modify: `internal/tui/view.go` — renderStatusBar
- Modify: `internal/tui/model.go` — add keybind priority data

**What to build:**
- Define keybind hints as a prioritized slice: `type hint struct { key, label string; priority int }`
- Three tiers based on m.width:
  - `>= 120`: all hints
  - `80-119`: n, e, d, enter, /, ?, q only
  - `< 80`: mode name + "?:help q:quit"
- Render hints left-to-right, stop when remaining width < next hint length
- Always include `q:quit` as rightmost item

**Commit:** `feat: responsive status bar with width-based hint tiers`

---

### Task 4: Section divider in forms

**Files:**
- Modify: `internal/tui/view.go` — renderForm, renderGlobalSettings
- Modify: `internal/tui/helpers.go` — add sectionDivider helper

**What to build:**
- `func sectionDivider(label string, width int) string` — returns `"── {label} ─────..."` in color 8, padded with `─` to fill width
- Use in renderForm between basic fields and advanced section
- Use in renderGlobalSettings at top

**Commit:** `feat: visual section dividers in connection forms`

---

### Task 5: Connection name truncation with tooltip

**Files:**
- Modify: `internal/tui/view.go` — renderSidebar

**What to build:**
- Compute `availWidth = 26 - indent - 2` (cursor glyph width)
- If `len(name) > availWidth`: render `name[:availWidth-1] + "…"`
- For the cursor row only: render a second dim line underneath showing `user@host:port` (from the connection)
- Account for this extra line in viewport height calculation (visibleRows decreases by 1 when cursor is on a truncated row)

**Commit:** `feat: truncate long names with ellipsis and tooltip on focused row`

---

### Task 6: Empty state redesign

**Files:**
- Modify: `internal/tui/view.go` — renderMainPane empty states

**What to build:**
- When no connections and no filter: render a centered welcome panel with:
  - `▞▚ hangar` title + dim "ssh bookmarks, organised" tagline
  - Box (Lipgloss border) with:
    - "no connections yet"
    - `n  add your first connection`
    - `s  import from ~/.ssh/config`
    - `g  create a group first`
    - dim tip line
- When filter has no matches: render variant "no matches for {filterText}" with esc:clear option
- Use `lipgloss.Place()` to center in main pane if terminal is tall enough (>= 14 rows)
- Fallback to simple text for very small terminals

**Commit:** `feat: welcoming empty state panel with onboarding hints`

---

### Task 7: Group header badges

**Files:**
- Modify: `internal/tui/view.go` — renderSidebar group header rendering

**What to build:**
- On expanded groups: show `▾ groupname` + right-aligned dim count `(N)` at the end of available width
- On collapsed groups: keep existing `▸ groupname (N)` behavior
- On empty groups (expanded): show `▾ groupname` + dim `(empty)` right-aligned
- Pad with spaces to align the count to the right edge of sidebar
- No online/ping feature (skip that part — too complex for now)

**Commit:** `feat: connection count badges on group headers`

---

### Task 8: Help overlay

**Files:**
- Create: `internal/tui/help.go`
- Modify: `internal/tui/model.go` — add showHelp bool
- Modify: `internal/tui/update.go` — handle "?" key, route to help
- Modify: `internal/tui/view.go` — render help when active

**What to build:**
- Model field: `showHelp bool`
- In normal/scripts mode, "?" toggles showHelp
- When showHelp=true, renderMainPane returns a static help view with all keybindings grouped by category (Navigation, Editing, Clipboard, Scripts, Other)
- Use sectionDivider for category headers
- "?" or "esc" closes help
- Status bar shows `? or esc to close` when help is open

**Commit:** `feat: help overlay with all keybindings grouped by category`

---

### Task 9: Paste collision confirmation

**Files:**
- Modify: `internal/tui/model.go` — add pasteConfirm state
- Modify: `internal/tui/handlers.go` — paste handler checks collisions
- Modify: `internal/tui/view.go` — renderPasteConfirm
- Modify: `internal/tui/update.go` — route to paste confirm handler

**What to build:**
- New form mode: `formPasteConfirm`
- Model fields: `pasteCollisions []string` (names that conflict), `pasteTargetGroup string`, `pasteItems []uuid.UUID`, `pasteIsCut bool`
- On "p" key: before executing paste, scan target group for name collisions
  - If no collisions: paste immediately + toast
  - If collisions: set formPasteConfirm, store collision data
- Render paste confirm: show list of items, highlight conflicts, offer r/s/esc choices
  - `r`: rename duplicates (append "-copy", "-copy-2" until unique)
  - `s`: skip conflicting items
  - `esc`: cancel
- After resolution: execute paste + toast

**Commit:** `feat: paste collision detection with rename/skip options`

---

### Task 10: Connect banner

**Files:**
- Modify: `internal/tui/model.go` — add connecting state
- Modify: `internal/tui/update.go` — pre-connect flow
- Modify: `internal/tui/view.go` — renderConnecting

**What to build:**
- Model fields: `connecting bool`, `connectTarget *config.Connection`, `connectStart time.Time`
- On "enter" to connect: set connecting=true, render banner, then use tea.Tick(300ms) to delay before exec
- Render connecting view: show target info (host, user, port, identity, jump, forwards)
- After SSH returns: compute duration, emit toast with session info ("disconnected — 12m 04s, exit 0")
- sshExitMsg now carries exit code; format toast based on exit code (0=ok, else=err)

**Commit:** `feat: pre-connect banner and post-disconnect session summary`

---

### Task 11: Filter in sync list

**Files:**
- Modify: `internal/tui/handlers.go` — handleSyncInput
- Modify: `internal/tui/model.go` — add syncFilterText
- Modify: `internal/tui/view.go` — renderSyncList

**What to build:**
- Model fields: `syncFilterText string`, `syncFiltering bool`
- In sync form: "/" enters filter mode (same as sidebar)
- Filter matches case-insensitive on entry Name, Host, User
- Render shows filter bar at top with count: `/ filtertext_    7 / 84 shown`
- "a" selects all VISIBLE entries, "n" deselects all visible
- Selection state preserved across filter changes (keyed by index in original list)

**Commit:** `feat: filter support in SSH config sync list`

---

### Task 12: Multi-select visual mode

**Files:**
- Create: `internal/tui/visual.go`
- Modify: `internal/tui/model.go` — add visual mode state
- Modify: `internal/tui/update.go` — route visual mode input
- Modify: `internal/tui/view.go` — render visual selection marks

**What to build:**
- Model fields: `visualMode bool`, `visualAnchor int` (starting cursor position)
- "v" in sidebar: enter visual mode, set anchor = cursor
- j/k in visual mode: extend selection (range = min(anchor,cursor) to max(anchor,cursor))
- "x" in visual mode: cut all items in range, exit visual mode
- "y" in visual mode: copy all items in range, exit visual mode
- "esc": exit visual mode without action
- Render: items in range get `·` prefix (dim) before their name
- Status bar: `-- VISUAL --  j/k:extend  x:cut  y:copy  esc:cancel`
- Only connections (not group headers) get selected; skip headers in range

**Commit:** `feat: visual-select mode for bulk cut/copy operations`

---

### Task 13: Tag token editor

**Files:**
- Modify: `internal/tui/model.go` — add tag editor state
- Modify: `internal/tui/handlers.go` — handleTagInput rewrite
- Modify: `internal/tui/view.go` — renderTagInput rewrite

**What to build:**
- Model fields: `tagTokens []string` (committed tags), `tagBuffer string` (typing)
- On entering tag form: populate tagTokens from connection's existing tags
- Render: show tokens as `[tag1] [tag2] buffer_` in cyan
- Space or comma: commit buffer to tagTokens (if non-empty, not duplicate)
- Backspace on empty buffer: pop last token
- "-" prefix: token marked for removal (render in red)
- Below tokens: show existing tags across all connections as dim autocomplete suggestions
- Tab: complete first matching suggestion
- Enter: save (apply additions and removals to connection)

**Commit:** `feat: tag token editor with chips and autocomplete`

---

### Task 14: Script result persistence

**Files:**
- Modify: `internal/config/types.go` — extend Script struct
- Modify: `internal/tui/model.go` — capture script results
- Modify: `internal/tui/handlers.go` — update script after run
- Modify: `internal/tui/view.go` — render last run info

**What to build:**
- Extend Script struct: `LastRunAt *time.Time`, `LastRunDuration time.Duration`, `LastRunExit int`
- After a script runs (on sshExitMsg with script context): update the script's LastRun fields, save config
- In renderMainPane scripts section: below each script's `$ command` line, if LastRunAt != nil:
  - Show: `last run: {exit N} · {relative time} · {duration}s`
  - Color exit 0 → green, non-zero → red
  - Relative time: "just now", "2m ago", "1h ago", "yesterday", date
- If never run: show dim `never run`

**Commit:** `feat: persist and display script execution results`

---

### Task 15: Reorder with J/K

**Files:**
- Modify: `internal/tui/handlers.go` — handle J/K keys
- Modify: `internal/tui/model.go` — add SortOrder field if needed

**What to build:**
- In normal sidebar mode: "J" (shift+j) swaps current item with the one below; "K" (shift+k) swaps with above
- For connections: swap positions in cfg.Connections slice
- For groups: swap order in the groups list (reorder groupOrder)
- Save after each swap
- If cursor is on a group header: swap entire group (header + its connections)
- Cursor follows the moved item

**Commit:** `feat: reorder connections and groups with J/K`

---

### Execution Order

Tasks should be executed in this order due to dependencies:
1. Task 0 (split files — everything depends on clean structure)
2. Task 1 (viewport — sidebar features depend on it)
3. Task 2 (toasts — many features emit toasts)
4. Task 3 (status bar — feeds into help overlay)
5. Tasks 4-7 (visual improvements — independent, can be parallel)
6. Task 8 (help overlay)
7. Tasks 9-15 (interaction features — mostly independent)
