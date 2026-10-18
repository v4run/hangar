package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// leaderBinding is one row in the leader-menu overlay.
type leaderBinding struct {
	Key  string
	Desc string
	Run  func(*Model) tea.Cmd
}

// leaderBindings returns what the `space` overlay offers right now.
// Each row shows a single-key hint; invoking that key fires `Run`.
func (m *Model) leaderBindings() []leaderBinding {
	return []leaderBinding{
		{"c", "connect to selected", func(m *Model) tea.Cmd {
			if c := m.selectedConnection(); c != nil {
				return m.beginConnect(c)
			}
			m.setToast("no connection under cursor", toastErr)
			return nil
		}},
		{"a", "add connection", func(m *Model) tea.Cmd { return m.openAddConnectionPalette() }},
		{"e", "edit selected", func(m *Model) tea.Cmd {
			if c := m.selectedConnection(); c != nil {
				return m.openEditConnectionPalette(c)
			}
			if g := m.selectedGroup(); g != nil {
				return m.openEditGroupPalette(g)
			}
			return nil
		}},
		{"d", "delete selected", func(m *Model) tea.Cmd {
			if c := m.selectedConnection(); c != nil {
				return m.openDeleteConnectionPalette(c)
			}
			return nil
		}},
		{"g", "new group", func(m *Model) tea.Cmd { return m.openAddGroupPalette() }},
		{"p", "connection picker", func(m *Model) tea.Cmd { return m.cmdConn("") }},
		{"G", "group picker", func(m *Model) tea.Cmd { return m.cmdGroup("") }},
		{"D", "database picker", func(m *Model) tea.Cmd { return m.cmdDB("") }},
		{"i", "import ssh command", func(m *Model) tea.Cmd { return m.cmdImport("") }},
		{"o", "edit notes", func(m *Model) tea.Cmd { return m.cmdNotes("") }},
		{"s", "add script to selected", func(m *Model) tea.Cmd { return m.cmdAddScript("") }},
		{"r", "run a script on selected", func(m *Model) tea.Cmd { return m.openScriptRunner() }},
		{"I", "edit connection shell init", func(m *Model) tea.Cmd { return m.cmdShellInit("") }},
		{"S", "edit group shell init", func(m *Model) tea.Cmd { return m.cmdGroupShellInit("") }},
		{"X", "delete selected group", func(m *Model) tea.Cmd { return m.cmdDeleteGroup("") }},
		{"b", "add database", func(m *Model) tea.Cmd { return m.cmdAddDB("") }},
		{"?", "help", func(m *Model) tea.Cmd {
			m.overlay = OverlayHelp
			m.showAll = true
			return nil
		}},
	}
}

// openLeader paints the overlay. The user's next keystroke resolves
// to one of the registered bindings or dismisses the overlay.
func (m *Model) openLeader() {
	m.overlay = OverlayLeader
	m.leaderOffset = 0
}

// closeLeader dismisses the overlay without running anything.
func (m *Model) closeLeader() {
	m.overlay = OverlayNone
	m.leaderOffset = 0
}

// leaderMaxBody is the number of binding rows the overlay body can
// show inside the paddedBox at the current terminal height.
func (m Model) leaderMaxBody() int {
	// Reserve 4 lines (title + blank + blank + footer) and 2 for the
	// paddedBox border.
	max := m.height - 8
	if max < 4 {
		max = 4
	}
	return max
}

// leaderMaxOffset is the largest offset that still reveals new
// bindings — holding `down` past it is a no-op.
func (m Model) leaderMaxOffset() int {
	max := len(m.leaderBindings()) - m.leaderMaxBody()
	if max < 0 {
		return 0
	}
	return max
}

func (m *Model) handleLeaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Scroll keys. These intentionally use arrows + pgup/pgdn (not
	// j/k) so a future binding on j/k wouldn't be silently stolen.
	maxOff := m.leaderMaxOffset()
	switch msg.String() {
	case "esc":
		m.closeLeader()
		return m, nil
	case "down":
		if m.leaderOffset < maxOff {
			m.leaderOffset++
		}
		return m, nil
	case "up":
		if m.leaderOffset > 0 {
			m.leaderOffset--
		}
		return m, nil
	case "pgdown", "ctrl+d":
		m.leaderOffset += 10
		if m.leaderOffset > maxOff {
			m.leaderOffset = maxOff
		}
		return m, nil
	case "pgup", "ctrl+u":
		m.leaderOffset -= 10
		if m.leaderOffset < 0 {
			m.leaderOffset = 0
		}
		return m, nil
	}
	s := msg.String()
	for _, b := range m.leaderBindings() {
		if b.Key == s {
			m.closeLeader()
			return m, b.Run(m)
		}
	}
	// Unrecognised key — close overlay and let the main handler see
	// the next keystroke fresh.
	m.closeLeader()
	return m, nil
}
