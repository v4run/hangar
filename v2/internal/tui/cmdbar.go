package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
)

// Command is one registered `:` command.
type Command struct {
	Name string
	Help string
	Run  func(*Model, string) tea.Cmd
}

// commands returns the registered command list for a model.
func (m *Model) commands() []Command {
	return []Command{
		{"conn", "list connections (fuzzy picker)", (*Model).cmdConn},
		{"group", "list groups (fuzzy picker)", (*Model).cmdGroup},
		{"add", "add a new connection", (*Model).cmdAdd},
		{"edit", "edit the selected connection", (*Model).cmdEdit},
		{"delete", "delete the selected connection", (*Model).cmdDelete},
		{"newgroup", "create a group", (*Model).cmdNewGroup},
		{"help", "show help overlay", (*Model).cmdHelp},
		{"quit", "quit", (*Model).cmdQuit},
	}
}

func (m *Model) openCmdBar() tea.Cmd {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.Placeholder = "command (try `conn`)"
	ti.Focus()
	m.cmdInput = ti
	m.overlay = OverlayCmdBar
	return textinput.Blink
}

func (m *Model) closeCmdBar() {
	m.cmdInput.Blur()
	m.cmdInput.SetValue("")
	m.overlay = OverlayNone
}

func (m *Model) handleCmdBarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeCmdBar()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.cmdInput.Value())
		m.closeCmdBar()
		return m, m.runCommand(name)
	case "tab":
		// Auto-complete to the first matching command.
		q := strings.TrimSpace(m.cmdInput.Value())
		for _, c := range m.commands() {
			if strings.HasPrefix(c.Name, q) {
				m.cmdInput.SetValue(c.Name)
				m.cmdInput.CursorEnd()
				break
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	return m, cmd
}

func (m *Model) runCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}
	name := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = strings.Join(parts[1:], " ")
	}
	for _, c := range m.commands() {
		if c.Name == name {
			return c.Run(m, rest)
		}
	}
	m.setToast("unknown command: "+name, toastErr)
	return nil
}

// --- command implementations ---

func (m *Model) cmdConn(_ string) tea.Cmd {
	items := make([]Pickable, 0, len(m.cfg.Connections))
	for i := range m.cfg.Connections {
		items = append(items, connPickable{cfg: m.cfg, conn: &m.cfg.Connections[i]})
	}
	return m.openPicker("Connections", items, func(p Pickable) tea.Cmd {
		if cp, ok := p.(connPickable); ok {
			m.cursorToConnection(cp.conn.ID)
			return m.beginConnect(cp.conn)
		}
		return nil
	})
}

func (m *Model) cmdGroup(_ string) tea.Cmd {
	items := make([]Pickable, 0, len(m.cfg.Groups))
	for i := range m.cfg.Groups {
		items = append(items, groupPickable{cfg: m.cfg, g: &m.cfg.Groups[i]})
	}
	return m.openPicker("Groups", items, func(p Pickable) tea.Cmd {
		if gp, ok := p.(groupPickable); ok {
			m.cursorToGroup(gp.g.ID)
		}
		return nil
	})
}

func (m *Model) cmdAdd(_ string) tea.Cmd { return m.openAddConnectionPalette() }
func (m *Model) cmdEdit(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openEditConnectionPalette(c)
	}
	m.setToast("no connection under cursor", toastErr)
	return nil
}
func (m *Model) cmdDelete(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openDeleteConnectionPalette(c)
	}
	m.setToast("no connection under cursor", toastErr)
	return nil
}
func (m *Model) cmdNewGroup(_ string) tea.Cmd { return m.openAddGroupPalette() }
func (m *Model) cmdHelp(_ string) tea.Cmd {
	m.overlay = OverlayHelp
	m.showAll = true
	return nil
}
func (m *Model) cmdQuit(_ string) tea.Cmd { return tea.Quit }
