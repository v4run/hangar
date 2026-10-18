package tui

import (
	"github.com/charmbracelet/bubbles/key"
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
}

// closeLeader dismisses the overlay without running anything.
func (m *Model) closeLeader() {
	m.overlay = OverlayNone
}

func (m *Model) handleLeaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keymap.Esc) {
		m.closeLeader()
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
