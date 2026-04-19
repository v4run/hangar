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
)

type formMode int

const (
	formNone formMode = iota
	formAdd
	formEdit
	formDelete
	formTag
	formSync
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
		// Form input handling (add/edit/delete/tag)
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

		// Filtering mode
		if m.filtering {
			return m.handleFilterInput(msg)
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
			// Mark entries already imported
			m.syncEntries = entries
			m.syncSelected = make([]bool, len(entries))
			for i, e := range entries {
				_, err := m.cfg.FindByName(e.Name)
				if err != nil {
					// Not yet imported — pre-select new entries
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
				m.formCursor = 1 // skip name field for edit
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
			// Connect to highlighted connection — full screen SSH
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
	case m.form == formDelete:
		bar = " y:confirm  esc:cancel"
	case m.form == formTag:
		bar = " enter:save  esc:cancel  (prefix with - to remove)"
	case m.form == formSync:
		bar = " space:toggle  a:all  n:none  enter:import  esc:cancel"
	default:
		bar = " n:new  e:edit  d:del  enter:connect  s:sync  t:tag  /:find  q:quit"
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
	// Form mode (add/edit)
	if m.form == formAdd || m.form == formEdit {
		return m.renderForm()
	}

	// Delete confirmation
	if m.form == formDelete {
		return m.renderDeleteConfirm()
	}

	// Tag input
	if m.form == formTag {
		return m.renderTagInput()
	}

	// Sync selection
	if m.form == formSync {
		return m.renderSyncList()
	}

	// Show connection details
	conns := m.filteredConnections()
	if len(conns) == 0 {
		return dimStyle.Render("no connections\n\npress n to add or s to sync from SSH config")
	}
	if m.cursor >= len(conns) {
		return ""
	}

	c := conns[m.cursor]
	var b strings.Builder

	b.WriteString(titleStyle.Render(c.Name))
	b.WriteString("\n\n")

	row := func(label, value string) {
		b.WriteString(labelStyle.Render(label) + " " + valueStyle.Render(value) + "\n")
	}

	row("host", c.Host)
	row("port", fmt.Sprintf("%d", c.Port))
	row("user", c.User)
	if c.IdentityFile != "" {
		row("key", c.IdentityFile)
	}
	if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
		row("pass", "********")
	}
	if c.JumpHost != "" {
		row("jump", c.JumpHost)
	}

	if len(c.Tags) > 0 {
		b.WriteString("\n")
		for i, t := range c.Tags {
			if i > 0 {
				b.WriteString(dimStyle.Render(", "))
			}
			b.WriteString(tagStyle.Render(t))
		}
		b.WriteString("\n")
	}

	if c.SyncedFromSSHConfig {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("synced from ssh config"))
	}

	return b.String()
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
