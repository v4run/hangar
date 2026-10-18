package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/google/uuid"
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
		{"db", "list databases (fuzzy picker)", (*Model).cmdDB},
		{"group", "list groups (fuzzy picker)", (*Model).cmdGroup},
		{"add", "add a new connection", (*Model).cmdAdd},
		{"adddb", "add a new database", (*Model).cmdAddDB},
		{"edit", "edit the selected connection / db / group", (*Model).cmdEdit},
		{"delete", "delete the selected connection / db", (*Model).cmdDelete},
		{"newgroup", "create a group", (*Model).cmdNewGroup},
		{"notes", "edit notes on the selected connection", (*Model).cmdNotes},
		{"import", "paste an ssh command to prefill a new connection", (*Model).cmdImport},
		{"addscript", "add a script to the selected connection", (*Model).cmdAddScript},
		{"run", "run a script on the selected connection", (*Model).cmdRunScript},
		{"shellinit", "edit shell init (connection scope)", (*Model).cmdShellInit},
		{"groupinit", "edit shell init (selected group scope)", (*Model).cmdGroupShellInit},
		{"globalinit", "edit shell init (global scope)", (*Model).cmdGlobalShellInit},
		{"delgroup", "delete the selected group (re-parents its contents)", (*Model).cmdDeleteGroup},
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
func (m *Model) cmdAddDB(_ string) tea.Cmd { return m.openAddDatabasePalette() }
func (m *Model) cmdEdit(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openEditConnectionPalette(c)
	}
	if d := m.selectedDatabase(); d != nil {
		return m.openEditDatabasePalette(d)
	}
	if g := m.selectedGroup(); g != nil {
		return m.openEditGroupPalette(g)
	}
	m.setToast("nothing under cursor", toastErr)
	return nil
}
func (m *Model) cmdDelete(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openDeleteConnectionPalette(c)
	}
	if d := m.selectedDatabase(); d != nil {
		return m.openDeleteDatabasePalette(d)
	}
	m.setToast("no connection or db under cursor", toastErr)
	return nil
}
func (m *Model) cmdNewGroup(_ string) tea.Cmd { return m.openAddGroupPalette() }
func (m *Model) cmdNotes(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openNotesPalette(c)
	}
	m.setToast("no connection under cursor", toastErr)
	return nil
}
func (m *Model) cmdImport(_ string) tea.Cmd { return m.openImportSSHPalette() }
func (m *Model) cmdAddScript(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openAddScriptPalette(c)
	}
	m.setToast("no connection under cursor", toastErr)
	return nil
}
func (m *Model) cmdRunScript(_ string) tea.Cmd { return m.openScriptRunner() }

func (m *Model) cmdShellInit(_ string) tea.Cmd {
	if c := m.selectedConnection(); c != nil {
		return m.openShellInitPalette(shellInitScopeConnection, c.ID, c.ShellInit)
	}
	m.setToast("no connection under cursor", toastErr)
	return nil
}
func (m *Model) cmdGroupShellInit(_ string) tea.Cmd {
	g := m.selectedGroup()
	if g == nil {
		if c := m.selectedConnection(); c != nil && c.Group != uuid.Nil {
			g = m.groupByID(c.Group)
		}
	}
	if g == nil {
		m.setToast("no group under cursor", toastErr)
		return nil
	}
	return m.openShellInitPalette(shellInitScopeGroup, g.ID, g.ShellInit)
}
func (m *Model) cmdGlobalShellInit(_ string) tea.Cmd {
	return m.openShellInitPalette(shellInitScopeGlobal, uuid.Nil, m.cfg.ShellInit)
}
func (m *Model) cmdDeleteGroup(_ string) tea.Cmd {
	if g := m.selectedGroup(); g != nil {
		return m.openDeleteGroupPalette(g)
	}
	m.setToast("no group under cursor", toastErr)
	return nil
}
func (m *Model) cmdDB(_ string) tea.Cmd {
	items := make([]Pickable, 0, len(m.cfg.Databases))
	for i := range m.cfg.Databases {
		items = append(items, dbPickable{cfg: m.cfg, db: &m.cfg.Databases[i]})
	}
	return m.openPicker("Databases", items, func(p Pickable) tea.Cmd {
		if dp, ok := p.(dbPickable); ok {
			m.cursorToDatabase(dp.db.ID)
			return m.launchDatabase(dp.db)
		}
		return nil
	})
}
func (m *Model) cmdHelp(_ string) tea.Cmd {
	m.overlay = OverlayHelp
	m.showAll = true
	return nil
}
func (m *Model) cmdQuit(_ string) tea.Cmd { return tea.Quit }
