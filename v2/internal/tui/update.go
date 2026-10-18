package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type toastClearMsg struct{ at time.Time }

func (m *Model) setToast(text string, kind toastKind) {
	m.toast = text
	m.toastKind = kind
	m.toastGone = time.Now().Add(3 * time.Second)
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
		if msg.err != nil {
			m.setToast("key scan: "+msg.err.Error(), toastErr)
			m.closePalette()
			m.pendingConn = nil
			return m, m.clearToastCmd()
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
		return m, tea.Batch(tea.EnableMouseCellMotion, m.clearToastCmd())
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help overlay captures all keys.
	if m.overlay == OverlayHelp {
		if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
			m.overlay = OverlayNone
			m.showAll = false
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
		// Space on a group row still toggles expand/collapse, as a
		// quality-of-life override.
		if g := m.selectedGroup(); g != nil {
			m.collapsed[g.ID] = !m.collapsed[g.ID]
			m.rebuildRows()
			return m, nil
		}
		m.openLeader()
		return m, nil
	case key.Matches(msg, m.keymap.Down):
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		return m, nil
	case key.Matches(msg, m.keymap.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case key.Matches(msg, m.keymap.Right):
		if g := m.selectedGroup(); g != nil && m.collapsed[g.ID] {
			m.collapsed[g.ID] = false
			m.rebuildRows()
			return m, nil
		}
	case key.Matches(msg, m.keymap.Left):
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
		return m, nil
	case key.Matches(msg, m.keymap.ShiftTab):
		m.tab = (m.tab + Tab(tabCount) - 1) % Tab(tabCount)
		return m, nil
	case key.Matches(msg, m.keymap.Enter):
		if c := m.selectedConnection(); c != nil {
			return m, m.beginConnect(c)
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
		if c := m.selectedConnection(); c != nil {
			return m, m.openEditConnectionPalette(c)
		}
		if g := m.selectedGroup(); g != nil {
			return m, m.openEditGroupPalette(g)
		}
	case key.Matches(msg, m.keymap.Add):
		return m, m.openAddConnectionPalette()
	case key.Matches(msg, m.keymap.NewGroup):
		return m, m.openAddGroupPalette()
	case key.Matches(msg, m.keymap.Delete):
		if c := m.selectedConnection(); c != nil {
			return m, m.openDeleteConnectionPalette(c)
		}
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
