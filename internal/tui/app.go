package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/v4run/hangar/internal/config"
	sshauth "github.com/v4run/hangar/internal/ssh"
)

type focus int

const (
	focusSidebar focus = iota
	focusScripts
)

type formMode int

const (
	formNone formMode = iota
	formAdd
	formEdit
	formDelete
	formTag
	formSync
	formAddScript
	formEditScript
	formDeleteScript
	formEditNotes
)

const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldUser
	fieldKey
	fieldJump
	fieldTags
	fieldPassword
	fieldCount
)

var fieldLabels = []string{"Name", "Host", "Port", "User", "Key", "Jump", "Tags", "Pass"}

type Model struct {
	cfg              *config.HangarConfig
	globalCfg        *config.GlobalConfig
	configDir        string
	width            int
	height           int
	focus            focus
	cursor           int
	sshConfigChanged bool
	filterText       string
	filtering        bool
	quitting bool
	form     formMode
	formFields       []string // field values: [name, host, port, user, key, jump, tags]
	formCursor       int      // which field is focused (0-6)
	formError        string   // validation error message
	formTarget       string              // connection name being edited/deleted/tagged
	tagInput         string              // input for tag mode
	syncEntries      []config.Connection // parsed SSH config entries for sync selection
	syncSelected     []bool             // selection state per entry
	syncCursor       int                // cursor position in sync list
	scriptCursor     int                // cursor in scripts list
	scriptName       string             // script name being added/edited
	scriptCommand    string             // script command being added/edited
	scriptField      int                // 0=name, 1=command
	scriptTarget     int                // index of script being edited
	notesInput       string             // notes text being edited
}

type sshExitMsg struct{ err error }

func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case sshExitMsg:
		// SSH session ended, back to TUI
		return m, nil

	case tea.KeyMsg:
		// Form input handling
		if m.form == formAdd || m.form == formEdit {
			return m.handleFormInput(msg)
		}
		if m.form == formDelete {
			return m.handleDeleteConfirm(msg)
		}
		if m.form == formTag {
			return m.handleTagInput(msg)
		}
		if m.form == formSync {
			return m.handleSyncInput(msg)
		}
		if m.form == formAddScript || m.form == formEditScript {
			return m.handleScriptFormInput(msg)
		}
		if m.form == formDeleteScript {
			return m.handleDeleteScriptConfirm(msg)
		}
		if m.form == formEditNotes {
			return m.handleNotesInput(msg)
		}

		// Filtering mode
		if m.filtering {
			return m.handleFilterInput(msg)
		}

		// Scripts pane focused
		if m.focus == focusScripts {
			return m.handleScriptsInput(msg)
		}

		// Normal sidebar mode
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "j", "down":
			conns := m.filteredConnections()
			if m.cursor < len(conns)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "l":
			// Move focus to scripts pane
			conns := m.filteredConnections()
			if len(conns) > 0 {
				m.focus = focusScripts
				m.scriptCursor = 0
			}
		case "/":
			m.filtering = true
			m.filterText = ""
		case "s", "S":
			gc, err := config.LoadGlobal(m.configDir)
			if err != nil {
				break
			}
			sshPath := sshauth.ExpandHome(gc.SSHConfigPath)
			entries, err := config.ParseSSHConfig(sshPath)
			if err != nil || len(entries) == 0 {
				break
			}
			m.syncEntries = entries
			m.syncSelected = make([]bool, len(entries))
			for i, e := range entries {
				_, err := m.cfg.FindByName(e.Name)
				if err != nil {
					m.syncSelected[i] = true
				}
			}
			m.syncCursor = 0
			m.form = formSync
		case "n":
			m.form = formAdd
			m.formFields = []string{"", "", "22", "", "", "", "", ""}
			m.formCursor = 0
			m.formError = ""
		case "e":
			conns := m.filteredConnections()
			if m.cursor < len(conns) {
				c := conns[m.cursor]
				m.form = formEdit
				m.formTarget = c.Name
				existingPass, _ := config.GetPassword(c.Name)
				m.formFields = []string{
					c.Name, c.Host, fmt.Sprintf("%d", c.Port), c.User,
					c.IdentityFile, c.JumpHost, strings.Join(c.Tags, ", "), existingPass,
				}
				m.formCursor = 1
				m.formError = ""
			}
		case "d":
			conns := m.filteredConnections()
			if m.cursor < len(conns) {
				m.form = formDelete
				m.formTarget = conns[m.cursor].Name
			}
		case "t":
			conns := m.filteredConnections()
			if m.cursor < len(conns) {
				m.form = formTag
				m.formTarget = conns[m.cursor].Name
				m.tagInput = ""
			}
		case "enter":
			conns := m.filteredConnections()
			if m.cursor < len(conns) {
				conn := conns[m.cursor]
				var jumpHost *config.Connection
				if conn.JumpHost != "" {
					if jh, err := m.cfg.FindByName(conn.JumpHost); err == nil {
						jumpHost = jh
					}
				}
				cmd, cleanup := sshauth.NewSSHCommand(&conn, jumpHost)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					cleanup()
					return sshExitMsg{err: err}
				})
			}
		}
	}

	return m, nil
}

func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.cursor = 0
	case "enter":
		m.filtering = false
		m.cursor = 0
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.cursor = 0
		}
	}
	return m, nil
}

func (m Model) filteredConnections() []config.Connection {
	if m.filterText == "" {
		return m.cfg.Connections
	}
	var filtered []config.Connection
	seen := make(map[string]bool)
	lower := strings.ToLower(m.filterText)
	for _, c := range m.cfg.Connections {
		if seen[c.Name] {
			continue
		}
		if strings.Contains(strings.ToLower(c.Name), lower) {
			filtered = append(filtered, c)
			seen[c.Name] = true
			continue
		}
		for _, t := range c.Tags {
			if strings.Contains(strings.ToLower(t), lower) {
				filtered = append(filtered, c)
				seen[c.Name] = true
				break
			}
		}
	}
	return filtered
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 {
		return "Loading..."
	}

	// Content area
	contentHeight := m.height - 1 // leave 1 line for status bar
	if contentHeight < 1 {
		contentHeight = 1
	}
	sidebarWidth := 26

	sidebar := m.renderSidebar()
	mainPane := m.renderMainPane()

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Width(sidebarWidth).Height(contentHeight).MaxHeight(contentHeight).Render(sidebar),
		mainPaneStyle.Width(m.width-sidebarWidth-3).Height(contentHeight).MaxHeight(contentHeight).Render(mainPane),
	)

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m Model) renderStatusBar() string {
	var bar string
	switch {
	case m.form == formAdd || m.form == formEdit:
		bar = " tab:next  shift+tab:prev  enter:save  esc:cancel"
	case m.form == formDelete || m.form == formDeleteScript:
		bar = " y:confirm  esc:cancel"
	case m.form == formTag:
		bar = " enter:save  esc:cancel  (prefix with - to remove)"
	case m.form == formSync:
		bar = " space:toggle  a:all  n:none  enter:import  esc:cancel"
	case m.form == formAddScript || m.form == formEditScript:
		bar = " tab:next  enter:save  esc:cancel"
	case m.form == formEditNotes:
		bar = " enter:save  esc:cancel"
	case m.focus == focusScripts:
		bar = " n:new  e:edit  d:del  enter:run  o:notes  h:back  q:quit"
	default:
		bar = " n:new  e:edit  d:del  enter:connect  s:sync  t:tag  l:scripts  /:find  q:quit"
	}
	return statusBarStyle.Render(bar)
}

func (m Model) renderSidebar() string {
	var b strings.Builder

	if m.filtering {
		b.WriteString(dimStyle.Render("/") + " " + normalStyle.Render(m.filterText) + cursorStyle.Render("_"))
	} else if m.filterText != "" {
		b.WriteString(dimStyle.Render("/ " + m.filterText))
	}
	b.WriteString("\n\n")

	conns := m.filteredConnections()
	if len(conns) == 0 {
		b.WriteString(dimStyle.Render("  no connections"))
		return b.String()
	}

	for i, c := range conns {
		if i == m.cursor {
			b.WriteString(cursorStyle.Render("> ") + selectedStyle.Render(c.Name))
		} else {
			b.WriteString("  " + normalStyle.Render(c.Name))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderMainPane() string {
	// Form modes
	switch m.form {
	case formAdd, formEdit:
		return m.renderForm()
	case formDelete:
		return m.renderDeleteConfirm()
	case formTag:
		return m.renderTagInput()
	case formSync:
		return m.renderSyncList()
	case formAddScript, formEditScript:
		return m.renderScriptForm()
	case formDeleteScript:
		return m.renderDeleteScriptConfirm()
	case formEditNotes:
		return m.renderNotesForm()
	}

	conns := m.filteredConnections()
	if len(conns) == 0 {
		return dimStyle.Render("no connections\n\npress n to add or s to sync from SSH config")
	}
	if m.cursor >= len(conns) {
		return ""
	}

	c := conns[m.cursor]
	var b strings.Builder

	// Connection info (compact)
	b.WriteString(titleStyle.Render(c.Name))
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("%s@%s:%d", c.User, c.Host, c.Port)))
	b.WriteString("\n")

	if c.IdentityFile != "" {
		b.WriteString(dimStyle.Render("key " + c.IdentityFile))
		b.WriteString("\n")
	}
	if c.JumpHost != "" {
		b.WriteString(dimStyle.Render("via " + c.JumpHost))
		b.WriteString("\n")
	}
	if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
		b.WriteString(dimStyle.Render("pass ********"))
		b.WriteString("\n")
	}

	if len(c.Tags) > 0 {
		for i, t := range c.Tags {
			if i > 0 {
				b.WriteString(dimStyle.Render(", "))
			}
			b.WriteString(tagStyle.Render(t))
		}
		b.WriteString("\n")
	}

	// Notes
	if c.Notes != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("notes "))
		b.WriteString(valueStyle.Render(c.Notes))
		b.WriteString("\n")
	}

	// Scripts section
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Scripts"))
	if m.focus == focusScripts {
		b.WriteString(dimStyle.Render("  (l to focus)"))
	}
	b.WriteString("\n\n")

	scripts := m.allScripts()
	if len(scripts) == 0 {
		b.WriteString(dimStyle.Render("  no scripts"))
	} else {
		for i, s := range scripts {
			isGlobal := m.isGlobalScript(i)
			prefix := "  "
			nameStyle := normalStyle
			if m.focus == focusScripts && i == m.scriptCursor {
				prefix = cursorStyle.Render("> ")
				nameStyle = selectedStyle
			}
			badge := ""
			if isGlobal {
				badge = dimStyle.Render(" [global]")
			}
			b.WriteString(prefix + nameStyle.Render(s.Name) + badge + "\n")
			cmdLine := dimStyle.Render("    $ " + s.Command)
			b.WriteString(cmdLine + "\n")
		}
	}

	return b.String()
}

func (m Model) renderScriptForm() string {
	var b strings.Builder
	if m.form == formAddScript {
		b.WriteString(titleStyle.Render("New Script"))
	} else {
		b.WriteString(titleStyle.Render("Edit Script"))
	}
	b.WriteString("\n\n")

	// Name field
	if m.scriptField == 0 {
		b.WriteString(activeFieldStyle.Render("> name") + " " + normalStyle.Render(m.scriptName) + cursorStyle.Render("_"))
	} else {
		b.WriteString("  " + labelStyle.Render("name") + " " + normalStyle.Render(m.scriptName))
	}
	b.WriteString("\n")

	// Command field
	if m.scriptField == 1 {
		b.WriteString(activeFieldStyle.Render("> cmd") + "  " + normalStyle.Render(m.scriptCommand) + cursorStyle.Render("_"))
	} else {
		b.WriteString("  " + labelStyle.Render("cmd") + "  " + normalStyle.Render(m.scriptCommand))
	}
	b.WriteString("\n")

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  " + m.formError))
	}

	return b.String()
}

func (m Model) renderDeleteScriptConfirm() string {
	scripts := m.allScripts()
	name := ""
	if m.scriptTarget < len(scripts) {
		name = scripts[m.scriptTarget].Name
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Script"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove ") + selectedStyle.Render(name) + normalStyle.Render("?"))
	return b.String()
}

func (m Model) renderNotesForm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Notes"))
	b.WriteString("\n\n")
	b.WriteString(activeFieldStyle.Render("> ") + normalStyle.Render(m.notesInput) + cursorStyle.Render("_"))
	return b.String()
}

// getSelectedConn returns a pointer to the currently selected connection.
func (m *Model) getSelectedConn() *config.Connection {
	conns := m.filteredConnections()
	if m.cursor >= len(conns) {
		return nil
	}
	// Find the actual connection in cfg by name
	c, err := m.cfg.FindByName(conns[m.cursor].Name)
	if err != nil {
		return nil
	}
	return c
}

// allScripts returns connection scripts + global scripts for the selected connection.
func (m Model) allScripts() []config.Script {
	c := m.getSelectedConn()
	if c == nil {
		return m.cfg.GlobalScripts
	}
	var all []config.Script
	all = append(all, c.Scripts...)
	all = append(all, m.cfg.GlobalScripts...)
	return all
}

// isGlobalScript returns whether the script at the given index is a global script.
func (m Model) isGlobalScript(idx int) bool {
	c := m.getSelectedConn()
	if c == nil {
		return true
	}
	return idx >= len(c.Scripts)
}

func (m Model) handleScriptsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	scripts := m.allScripts()

	switch msg.String() {
	case "h", "esc":
		m.focus = focusSidebar
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if m.scriptCursor < len(scripts)-1 {
			m.scriptCursor++
		}
	case "k", "up":
		if m.scriptCursor > 0 {
			m.scriptCursor--
		}
	case "n":
		m.form = formAddScript
		m.scriptName = ""
		m.scriptCommand = ""
		m.scriptField = 0
		m.formError = ""
	case "e":
		if m.scriptCursor < len(scripts) {
			s := scripts[m.scriptCursor]
			m.form = formEditScript
			m.scriptTarget = m.scriptCursor
			m.scriptName = s.Name
			m.scriptCommand = s.Command
			m.scriptField = 0
			m.formError = ""
		}
	case "d":
		if m.scriptCursor < len(scripts) {
			m.form = formDeleteScript
			m.scriptTarget = m.scriptCursor
		}
	case "o":
		// Edit notes
		c := m.getSelectedConn()
		if c != nil {
			m.form = formEditNotes
			m.notesInput = c.Notes
		}
	case "enter":
		// Run the selected script
		if m.scriptCursor < len(scripts) {
			conns := m.filteredConnections()
			if m.cursor < len(conns) {
				conn := conns[m.cursor]
				script := scripts[m.scriptCursor]
				var jumpHost *config.Connection
				if conn.JumpHost != "" {
					if jh, err := m.cfg.FindByName(conn.JumpHost); err == nil {
						jumpHost = jh
					}
				}
				cmd, cleanup := sshauth.NewSSHCommand(&conn, jumpHost)
				cmd.Args = append(cmd.Args, script.Command)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					cleanup()
					return sshExitMsg{err: err}
				})
			}
		}
	}
	return m, nil
}

func (m Model) handleScriptFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.scriptField = (m.scriptField + 1) % 2
	case "shift+tab", "up":
		m.scriptField = (m.scriptField + 1) % 2
	case "enter":
		name := strings.TrimSpace(m.scriptName)
		command := strings.TrimSpace(m.scriptCommand)
		if name == "" {
			m.formError = "name is required"
			return m, nil
		}
		if command == "" {
			m.formError = "command is required"
			return m, nil
		}
		script := config.Script{Name: name, Command: command}

		if m.form == formAddScript {
			c := m.getSelectedConn()
			if c != nil {
				c.Scripts = append(c.Scripts, script)
			}
		} else {
			// Edit
			if m.isGlobalScript(m.scriptTarget) {
				gi := m.scriptTarget
				c := m.getSelectedConn()
				if c != nil {
					gi -= len(c.Scripts)
				}
				if gi >= 0 && gi < len(m.cfg.GlobalScripts) {
					m.cfg.GlobalScripts[gi] = script
				}
			} else {
				c := m.getSelectedConn()
				if c != nil && m.scriptTarget < len(c.Scripts) {
					c.Scripts[m.scriptTarget] = script
				}
			}
		}
		config.Save(m.configDir, m.cfg)
		m.form = formNone
	case "backspace":
		if m.scriptField == 0 && len(m.scriptName) > 0 {
			m.scriptName = m.scriptName[:len(m.scriptName)-1]
		} else if m.scriptField == 1 && len(m.scriptCommand) > 0 {
			m.scriptCommand = m.scriptCommand[:len(m.scriptCommand)-1]
		}
	default:
		if len(msg.String()) == 1 {
			if m.scriptField == 0 {
				m.scriptName += msg.String()
			} else {
				m.scriptCommand += msg.String()
			}
		}
	}
	return m, nil
}

func (m Model) handleDeleteScriptConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.isGlobalScript(m.scriptTarget) {
			gi := m.scriptTarget
			c := m.getSelectedConn()
			if c != nil {
				gi -= len(c.Scripts)
			}
			if gi >= 0 && gi < len(m.cfg.GlobalScripts) {
				m.cfg.GlobalScripts = append(m.cfg.GlobalScripts[:gi], m.cfg.GlobalScripts[gi+1:]...)
			}
		} else {
			c := m.getSelectedConn()
			if c != nil && m.scriptTarget < len(c.Scripts) {
				c.Scripts = append(c.Scripts[:m.scriptTarget], c.Scripts[m.scriptTarget+1:]...)
			}
		}
		config.Save(m.configDir, m.cfg)
		scripts := m.allScripts()
		if m.scriptCursor >= len(scripts) && m.scriptCursor > 0 {
			m.scriptCursor--
		}
		m.form = formNone
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleNotesInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		c := m.getSelectedConn()
		if c != nil {
			c.Notes = strings.TrimSpace(m.notesInput)
			config.Save(m.configDir, m.cfg)
		}
		m.form = formNone
	case "backspace":
		if len(m.notesInput) > 0 {
			m.notesInput = m.notesInput[:len(m.notesInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.notesInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.formCursor = (m.formCursor + 1) % fieldCount
		if m.form == formEdit && m.formCursor == fieldName {
			m.formCursor = fieldHost
		}
	case "shift+tab", "up":
		m.formCursor = (m.formCursor - 1 + fieldCount) % fieldCount
		if m.form == formEdit && m.formCursor == fieldName {
			m.formCursor = fieldTags
		}
	case "enter":
		return m.saveForm()
	case "backspace":
		if len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.formFields[m.formCursor] += msg.String()
		}
	}
	return m, nil
}

func (m Model) saveForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.formFields[fieldName])
	host := strings.TrimSpace(m.formFields[fieldHost])
	portStr := strings.TrimSpace(m.formFields[fieldPort])
	user := strings.TrimSpace(m.formFields[fieldUser])
	key := strings.TrimSpace(m.formFields[fieldKey])
	jump := strings.TrimSpace(m.formFields[fieldJump])
	tagsStr := strings.TrimSpace(m.formFields[fieldTags])
	password := m.formFields[fieldPassword]

	// Validate
	if name == "" {
		m.formError = "Name is required"
		return m, nil
	}
	if host == "" {
		m.formError = "Host is required"
		return m, nil
	}
	if user == "" {
		m.formError = "User is required"
		return m, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		m.formError = "Port must be a positive number"
		return m, nil
	}

	// Parse tags
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	conn := config.Connection{
		Name:         name,
		Host:         host,
		Port:         port,
		User:         user,
		IdentityFile: key,
		JumpHost:     jump,
		Tags:         tags,
	}

	if m.form == formAdd {
		if err := m.cfg.Add(conn); err != nil {
			m.formError = err.Error()
			return m, nil
		}
	} else if m.form == formEdit {
		m.cfg.Remove(m.formTarget)
		conn.Name = m.formTarget // keep original name
		m.cfg.Connections = append(m.cfg.Connections, conn)
	}

	// Save to disk
	if err := config.Save(m.configDir, m.cfg); err != nil {
		m.formError = "Save failed: " + err.Error()
		return m, nil
	}

	// Save or delete password in keychain
	connName := name
	if m.form == formEdit {
		connName = m.formTarget
	}
	if password != "" {
		config.SetPassword(connName, password)
	} else {
		config.DeletePassword(connName)
	}

	m.form = formNone
	m.formError = ""
	return m, nil
}

func (m Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.cfg.Remove(m.formTarget)
		config.Save(m.configDir, m.cfg)
		if m.cursor >= len(m.cfg.Connections) && m.cursor > 0 {
			m.cursor--
		}
		m.form = formNone
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleTagInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		if m.tagInput != "" {
			var toAdd, toRemove []string
			for _, t := range strings.Split(m.tagInput, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				if strings.HasPrefix(t, "-") {
					toRemove = append(toRemove, t[1:])
				} else {
					toAdd = append(toAdd, t)
				}
			}
			if len(toAdd) > 0 {
				m.cfg.AddTags(m.formTarget, toAdd)
			}
			if len(toRemove) > 0 {
				m.cfg.RemoveTags(m.formTarget, toRemove)
			}
			config.Save(m.configDir, m.cfg)
		}
		m.form = formNone
	case "backspace":
		if len(m.tagInput) > 0 {
			m.tagInput = m.tagInput[:len(m.tagInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.tagInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleSyncInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "j", "down":
		if m.syncCursor < len(m.syncEntries)-1 {
			m.syncCursor++
		}
	case "k", "up":
		if m.syncCursor > 0 {
			m.syncCursor--
		}
	case " ":
		if m.syncCursor < len(m.syncSelected) {
			m.syncSelected[m.syncCursor] = !m.syncSelected[m.syncCursor]
		}
	case "a":
		for i := range m.syncSelected {
			m.syncSelected[i] = true
		}
	case "n":
		for i := range m.syncSelected {
			m.syncSelected[i] = false
		}
	case "enter":
		imported := 0
		for i, entry := range m.syncEntries {
			if !m.syncSelected[i] {
				continue
			}
			existing, err := m.cfg.FindByName(entry.Name)
			if err != nil {
				// New entry — add it
				m.cfg.Connections = append(m.cfg.Connections, entry)
				imported++
			} else if existing.SyncedFromSSHConfig {
				// Update existing synced entry
				existing.Host = entry.Host
				existing.Port = entry.Port
				existing.User = entry.User
				existing.IdentityFile = entry.IdentityFile
				existing.JumpHost = entry.JumpHost
				imported++
			}
		}
		config.Save(m.configDir, m.cfg)
		m.sshConfigChanged = false
		m.form = formNone
	}
	return m, nil
}

func (m Model) renderSyncList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Import from SSH Config"))
	b.WriteString("\n\n")

	for i, entry := range m.syncEntries {
		check := "[ ]"
		if m.syncSelected[i] {
			check = successStyle.Render("[x]")
		}

		_, err := m.cfg.FindByName(entry.Name)
		alreadyImported := err == nil

		name := entry.Name
		detail := dimStyle.Render(fmt.Sprintf("  %s@%s:%d", entry.User, entry.Host, entry.Port))
		badge := ""
		if alreadyImported {
			badge = dimStyle.Render(" (imported)")
		}

		if i == m.syncCursor {
			b.WriteString(cursorStyle.Render("> ") + check + " " + selectedStyle.Render(name) + detail + badge)
		} else {
			b.WriteString("  " + check + " " + normalStyle.Render(name) + detail + badge)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderForm() string {
	var b strings.Builder
	if m.form == formEdit {
		b.WriteString(titleStyle.Render("Edit Connection"))
	} else {
		b.WriteString(titleStyle.Render("New Connection"))
	}
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		value := m.formFields[i]
		if i == fieldPassword && value != "" {
			value = strings.Repeat("*", len(value))
		}

		label := labelStyle.Render(strings.ToLower(fieldLabels[i]))
		if i == m.formCursor {
			b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(fieldLabels[i])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
		} else if m.form == formEdit && i == fieldName {
			b.WriteString("  " + label + " " + dimStyle.Render(value+" (readonly)"))
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  " + m.formError))
	}

	return b.String()
}

func (m Model) renderDeleteConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Connection"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove ") + selectedStyle.Render(m.formTarget) + normalStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("this cannot be undone"))
	return b.String()
}

func (m Model) renderTagInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Tags: " + m.formTarget))
	b.WriteString("\n\n")

	c, err := m.cfg.FindByName(m.formTarget)
	if err == nil && len(c.Tags) > 0 {
		for i, t := range c.Tags {
			if i > 0 {
				b.WriteString(dimStyle.Render(", "))
			}
			b.WriteString(tagStyle.Render(t))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(activeFieldStyle.Render("> add") + " " + normalStyle.Render(m.tagInput) + cursorStyle.Render("_"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("comma-separated, prefix with - to remove"))

	return b.String()
}

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
