package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

var textinputBlink = textinput.Blink

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
		m.rebuildRows()
		m.cursor = 0
		return m, nil
	case "enter":
		m.filtering = false
		m.filter.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.rebuildRows()
	m.cursor = 0
	return m, cmd
}

type toastClearMsg struct{ at time.Time }

func (m *Model) setToast(text string, kind toastKind) {
	// Toasts live on the status bar — one visible line. Collapse any
	// embedded newlines (the ssh-keyscan error path can emit several
	// `getaddrinfo … not known` repeats) and strip adjacent dupes
	// so the status bar never wraps.
	text = flattenToast(text)
	m.toast = text
	m.toastKind = kind
	m.toastGone = time.Now().Add(3 * time.Second)
}

func flattenToast(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	parts := strings.Split(s, "\n")
	// Keep only non-empty lines, trimmed.
	trimmed := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		trimmed = append(trimmed, p)
	}
	if len(trimmed) == 0 {
		return ""
	}
	// A toast is one glance on the status bar — show the first line
	// only. If the error was multi-line (ssh-keyscan's 4-copy
	// `getaddrinfo` barrage is the motivating example) append a
	// `(+N more)` tag so the user knows more detail was dropped.
	head := trimmed[0]
	if n := len(trimmed) - 1; n > 0 {
		head = head + " (+" + strconv.Itoa(n) + " more)"
	}
	// Hard cap so a huge error doesn't eat the hints strip. Keep 3
	// bytes for the ellipsis so the result fits exactly.
	const max = 120
	if len(head) > max {
		head = head[:max-3] + "..."
	}
	return head
}


func (m Model) clearToastCmd() tea.Cmd {
	target := m.toastGone
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return toastClearMsg{at: target}
	})
}

// Update is the one dispatcher. It delegates to overlay-specific
// handlers when an overlay is open, then falls through to sidebar
// navigation.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil
	case toastClearMsg:
		if !msg.at.IsZero() && msg.at.Equal(m.toastGone) {
			m.toast = ""
		}
		return m, nil
	case hostKeyScannedMsg:
		m.spinning = false
		if msg.err != nil {
			m.setToast("key scan: "+msg.err.Error(), toastErr)
			m.closePalette()
			m.pendingConn = nil
			return m, tea.Batch(tea.ClearScreen, m.clearToastCmd())
		}
		m.pendingKeys = msg.keys
		m.palette.message = ""
		return m, nil
	case sshExitMsg:
		m.mode = ModeBrowse
		if msg.err != nil {
			m.setToast("ssh: "+msg.err.Error(), toastErr)
		} else {
			m.setToast("disconnected", toastOK)
		}
		// tea.ClearScreen wipes anything ssh/db clients wrote to the
		// normal screen before alt-screen was re-entered — otherwise
		// the TUI can render one row down and scroll the header off.
		return m, tea.Batch(tea.ClearScreen, tea.EnableMouseCellMotion, m.clearToastCmd())
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case spinner.TickMsg:
		if m.spinning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	return m, nil
}

// handleMouse supports click-to-select in the sidebar, click-on-tab
// in the detail header, click-on-row in the fuzzy picker, and wheel
// scrolling (sidebar if the pointer is over it, otherwise the detail
// pane or the picker when it's active).
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Picker overlay — click a visible match to select, wheel scrolls.
	if m.overlay == OverlayPicker {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.picker.cursor > 0 {
				m.picker.cursor--
			}
		case tea.MouseButtonWheelDown:
			if m.picker.cursor < len(m.picker.matches)-1 {
				m.picker.cursor++
			}
		}
		return m, nil
	}
	if m.overlay != OverlayNone {
		return m, nil
	}
	sidebarW := m.sidebarWidth()
	inSidebar := msg.X < sidebarW
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inSidebar {
			if m.cursor > 0 {
				m.cursor--
			}
		} else if m.detailOffset > 0 {
			m.detailOffset--
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if inSidebar {
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		} else {
			m.detailOffset++
		}
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		if inSidebar {
			// Sidebar rows render under: header(1) + pane top border(1) +
			// filter row(1) + blank(1) = y>=4. Rows then flow with the
			// offset from render.
			row := msg.Y - 4 + m.offset
			if row < 0 || row >= len(m.rows) {
				return m, nil
			}
			m.cursor = row
			return m, nil
		}
		// Detail-pane tab bar: y == 2 (header + pane top border).
		// Each tab is " Tab " padded; approximate hit zones using the
		// tabCount.
		if msg.Y == 2 {
			// Rough: distribute tabs evenly inside the right pane.
			rightX := msg.X - sidebarW
			per := (m.width - sidebarW) / int(tabCount)
			if per < 1 {
				per = 1
			}
			idx := rightX / per
			if idx < 0 {
				idx = 0
			}
			if idx >= int(tabCount) {
				idx = int(tabCount) - 1
			}
			m.tab = Tab(idx)
		}
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay captures all keys.
	if m.overlay == OverlayHelp {
		maxOff := m.helpMaxOffset()
		switch msg.String() {
		case "?", "esc", "q":
			m.overlay = OverlayNone
			m.showAll = false
			m.helpOffset = 0
		case "j", "down":
			if m.helpOffset < maxOff {
				m.helpOffset++
			}
		case "k", "up":
			if m.helpOffset > 0 {
				m.helpOffset--
			}
		case "pgdown", " ", "ctrl+d":
			m.helpOffset += 10
			if m.helpOffset > maxOff {
				m.helpOffset = maxOff
			}
		case "pgup", "b", "ctrl+u":
			m.helpOffset -= 10
			if m.helpOffset < 0 {
				m.helpOffset = 0
			}
		case "g":
			m.helpOffset = 0
		case "G":
			m.helpOffset = maxOff
		}
		return m, nil
	}
	// Overlay-specific handlers.
	switch m.overlay {
	case OverlayCmdBar:
		return m.handleCmdBarKey(msg)
	case OverlayLeader:
		return m.handleLeaderKey(msg)
	case OverlayPicker:
		return m.handlePickerKey(msg)
	case OverlayPalette:
		return m.handlePaletteKey(msg)
	}

	// Filter mode: captures sidebar typing.
	if m.filtering {
		return m.handleFilterKey(msg)
	}
	if key.Matches(msg, m.keymap.Filter) {
		m.filtering = true
		m.filter.Focus()
		return m, textinputBlink
	}

	// Browse-mode keys.
	switch {
	case key.Matches(msg, m.keymap.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keymap.Help):
		m.overlay = OverlayHelp
		m.showAll = true
		return m, nil
	case key.Matches(msg, m.keymap.Cmd):
		return m, m.openCmdBar()
	case key.Matches(msg, m.keymap.Leader):
		// Space is the leader key everywhere — do NOT short-circuit
		// on a group row. Collapse / expand lives on enter, h, l and
		// the arrows; the leader needs an unambiguous entry point so
		// `space s` / `space o` work regardless of what's selected.
		m.openLeader()
		return m, nil
	case key.Matches(msg, m.keymap.Down):
		if m.paneFocus == focusDetail {
			m.moveDetailCursor(+1)
			return m, nil
		}
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.scriptCursor, m.forwardCursor = 0, 0
		}
		return m, nil
	case key.Matches(msg, m.keymap.Up):
		if m.paneFocus == focusDetail {
			m.moveDetailCursor(-1)
			return m, nil
		}
		if m.cursor > 0 {
			m.cursor--
			m.scriptCursor, m.forwardCursor = 0, 0
		}
		return m, nil
	case key.Matches(msg, m.keymap.Right):
		if g := m.selectedGroup(); g != nil {
			if m.collapsed[g.ID] {
				m.collapsed[g.ID] = false
				m.rebuildRows()
			}
			return m, nil
		}
		// Connection / database row — focus the detail pane so the
		// user can tab through its tabs and act on items.
		if c := m.selectedConnection(); c != nil {
			m.paneFocus = focusDetail
			return m, nil
		}
		if d := m.selectedDatabase(); d != nil {
			m.paneFocus = focusDetail
			_ = d
			return m, nil
		}
	case key.Matches(msg, m.keymap.Left):
		if m.paneFocus == focusDetail {
			m.paneFocus = focusSidebar
			return m, nil
		}
		if g := m.selectedGroup(); g != nil && !m.collapsed[g.ID] {
			m.collapsed[g.ID] = true
			m.rebuildRows()
			return m, nil
		}
	case key.Matches(msg, m.keymap.Top):
		m.cursor = 0
		return m, nil
	case key.Matches(msg, m.keymap.Bottom):
		m.cursor = len(m.rows) - 1
		return m, nil
	case key.Matches(msg, m.keymap.Tab):
		m.tab = (m.tab + 1) % Tab(tabCount)
		m.paneFocus = focusDetail
		m.scriptCursor = 0
		m.forwardCursor = 0
		return m, nil
	case key.Matches(msg, m.keymap.ShiftTab):
		m.tab = (m.tab + Tab(tabCount) - 1) % Tab(tabCount)
		m.paneFocus = focusDetail
		m.scriptCursor = 0
		m.forwardCursor = 0
		return m, nil
	case msg.String() == "esc":
		if m.paneFocus == focusDetail {
			m.paneFocus = focusSidebar
			return m, nil
		}
	case key.Matches(msg, m.keymap.Enter):
		if r := m.selectedRow(); r.kind == rowSectionHeader && r.group != nil && r.group.Name == "Recent" {
			m.recentCollapsed = !m.recentCollapsed
			m.rebuildRows()
			return m, nil
		}
		if m.paneFocus == focusDetail && m.tab == TabScripts {
			if c := m.selectedConnection(); c != nil {
				r := config.Resolve(m.cfg, c)
				if m.scriptCursor >= 0 && m.scriptCursor < len(r.Scripts) {
					return m, m.runScriptOn(c, r.Scripts[m.scriptCursor])
				}
			}
			return m, nil
		}
		if c := m.selectedConnection(); c != nil {
			return m, m.beginConnect(c)
		}
		if d := m.selectedDatabase(); d != nil {
			return m, m.launchDatabase(d)
		}
		if g := m.selectedGroup(); g != nil {
			m.collapsed[g.ID] = !m.collapsed[g.ID]
			m.rebuildRows()
			return m, nil
		}
	case key.Matches(msg, m.keymap.Connect):
		if c := m.selectedConnection(); c != nil {
			return m, m.beginConnect(c)
		}
	case key.Matches(msg, m.keymap.Edit):
		if m.paneFocus == focusDetail {
			switch m.tab {
			case TabScripts:
				if c := m.selectedConnection(); c != nil && m.scriptCursor < len(c.Scripts) {
					return m, m.openEditScriptPalette(c, m.scriptCursor)
				}
				return m, nil
			case TabNotes:
				if c := m.selectedConnection(); c != nil {
					return m, m.openNotesPalette(c)
				}
				return m, nil
			}
		}
		if c := m.selectedConnection(); c != nil {
			return m, m.openEditConnectionPalette(c)
		}
		if g := m.selectedGroup(); g != nil {
			return m, m.openEditGroupPalette(g)
		}
	case key.Matches(msg, m.keymap.Add):
		if m.paneFocus == focusDetail {
			switch m.tab {
			case TabScripts:
				if c := m.selectedConnection(); c != nil {
					return m, m.openAddScriptPalette(c)
				}
				return m, nil
			case TabForwards:
				if c := m.selectedConnection(); c != nil {
					return m, m.openAddForwardPalette(c)
				}
				return m, nil
			}
		}
		return m, m.openAddConnectionPalette()
	case key.Matches(msg, m.keymap.NewGroup):
		return m, m.openAddGroupPalette()
	case key.Matches(msg, m.keymap.Delete):
		if m.paneFocus == focusDetail {
			switch m.tab {
			case TabScripts:
				if c := m.selectedConnection(); c != nil && m.scriptCursor < len(c.Scripts) {
					return m, m.openDeleteScriptPalette(c, m.scriptCursor)
				}
				return m, nil
			case TabForwards:
				return m, m.deleteFocusedForward()
			}
		}
		if c := m.selectedConnection(); c != nil {
			return m, m.openDeleteConnectionPalette(c)
		}
		if d := m.selectedDatabase(); d != nil {
			return m, m.openDeleteDatabasePalette(d)
		}
		if g := m.selectedGroup(); g != nil {
			return m, m.openDeleteGroupPalette(g)
		}
	case key.Matches(msg, m.keymap.ToggleOpen):
		if r := m.selectedRow(); r.kind == rowSectionHeader && r.group != nil && r.group.Name == "Recent" {
			m.recentCollapsed = !m.recentCollapsed
			m.rebuildRows()
			return m, nil
		}
		if g := m.selectedGroup(); g != nil {
			m.collapsed[g.ID] = !m.collapsed[g.ID]
			m.rebuildRows()
			return m, nil
		}
		m.setToast("that row isn't a group", toastInfo)
		return m, nil
	case key.Matches(msg, m.keymap.MoveUp):
		m.moveSelectedBy(-1)
		return m, nil
	case key.Matches(msg, m.keymap.MoveDown):
		m.moveSelectedBy(+1)
		return m, nil
	case key.Matches(msg, m.keymap.PageUp):
		if m.detailOffset > 0 {
			m.detailOffset -= 5
			if m.detailOffset < 0 {
				m.detailOffset = 0
			}
		}
		return m, nil
	case key.Matches(msg, m.keymap.PageDn):
		m.detailOffset += 5
		return m, nil
	}
	return m, nil
}

// cursorToConnection moves the sidebar cursor onto the row with the
// given connection ID, if present.
func (m *Model) cursorToConnection(id uuid.UUID) {
	for i, r := range m.rows {
		if r.conn != nil && r.conn.ID == id {
			m.cursor = i
			return
		}
	}
}

// moveDetailCursor advances the per-tab cursor in the right pane
// (Scripts or Forwards). On Details / Notes it's a no-op.
func (m *Model) moveDetailCursor(delta int) {
	c := m.selectedConnection()
	if c == nil {
		return
	}
	switch m.tab {
	case TabScripts:
		r := config.Resolve(m.cfg, c)
		n := len(r.Scripts)
		if n == 0 {
			return
		}
		m.scriptCursor = clampInt(m.scriptCursor+delta, 0, n-1)
	case TabForwards:
		r := config.Resolve(m.cfg, c)
		n := 0
		if r.SSHOptions != nil {
			n = len(r.SSHOptions.LocalForward) + len(r.SSHOptions.RemoteForward)
		}
		if n == 0 {
			return
		}
		m.forwardCursor = clampInt(m.forwardCursor+delta, 0, n-1)
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// deleteFocusedForward figures out which direction + index the
// forwardCursor points at (across the connection's OWN local +
// remote lists; inherited forwards aren't deletable here) and
// opens the delete palette.
func (m *Model) deleteFocusedForward() tea.Cmd {
	c := m.selectedConnection()
	if c == nil || c.SSHOptions == nil {
		return nil
	}
	locals := c.SSHOptions.LocalForward
	remotes := c.SSHOptions.RemoteForward
	idx := m.forwardCursor
	if idx < len(locals) {
		return m.openDeleteForwardPalette(c, "L", idx)
	}
	ri := idx - len(locals)
	if ri < len(remotes) {
		return m.openDeleteForwardPalette(c, "R", ri)
	}
	m.setToast("that forward is inherited — edit the group instead", toastInfo)
	return nil
}

// cursorToDatabase moves the sidebar cursor onto the given db row.
func (m *Model) cursorToDatabase(id uuid.UUID) {
	for i, r := range m.rows {
		if r.db != nil && r.db.ID == id {
			m.cursor = i
			return
		}
	}
}

// cursorToGroup moves the sidebar cursor onto the given group row.
// Expands any collapsed ancestors so the group is visible.
func (m *Model) cursorToGroup(id uuid.UUID) {
	// Expand ancestors.
	byID := make(map[uuid.UUID]*m2GroupRef, len(m.cfg.Groups))
	for i := range m.cfg.Groups {
		byID[m.cfg.Groups[i].ID] = &m2GroupRef{p: m.cfg.Groups[i].Parent}
	}
	for cur := id; cur != uuid.Nil; {
		g, ok := byID[cur]
		if !ok {
			break
		}
		delete(m.collapsed, cur)
		cur = g.p
	}
	m.rebuildRows()
	for i, r := range m.rows {
		if r.group != nil && r.group.ID == id {
			m.cursor = i
			return
		}
	}
}

type m2GroupRef struct{ p uuid.UUID }
