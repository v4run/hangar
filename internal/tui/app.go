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

	// Title bar
	titleLeft := titleStyle.Render(" hangar ")
	syncIndicator := ""
	if m.sshConfigChanged {
		syncIndicator = syncIndicatorStyle.Render(" SSH config changed (S to sync) ")
	}
	titlePadding := m.width - lipgloss.Width(titleLeft) - lipgloss.Width(syncIndicator)
	if titlePadding < 0 {
		titlePadding = 0
	}
	titleBar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleLeft,
		lipgloss.NewStyle().Width(titlePadding).Render(""),
		syncIndicator,
	)

	// Content area
	contentHeight := m.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}
	sidebarWidth := 25

	sidebar := m.renderSidebar()
	mainPane := m.renderMainPane()

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Width(sidebarWidth).Height(contentHeight).MaxHeight(contentHeight).Render(sidebar),
		mainPaneStyle.Width(m.width-sidebarWidth-3).Height(contentHeight).MaxHeight(contentHeight).Render(mainPane),
	)

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, content, statusBar)
}

func (m Model) renderStatusBar() string {
	if m.form == formAdd || m.form == formEdit {
		return statusBarStyle.Render(" FORM: [Tab]next [Shift+Tab]prev [Enter]save [Esc]cancel")
	}
	if m.form == formDelete {
		return statusBarStyle.Render(" DELETE: [y]confirm [n/Esc]cancel")
	}
	if m.form == formTag {
		return statusBarStyle.Render(" TAG: [Enter]save [Esc]cancel  prefix with - to remove")
	}
	if m.form == formSync {
		return statusBarStyle.Render(" SYNC: [space]toggle [a]ll [n]one [Enter]import [Esc]cancel")
	}
	return statusBarStyle.Render(" [n]ew [e]dit [d]el [enter]connect [s]ync [t]ag [/]find [q]uit")
}

func (m Model) renderSidebar() string {
	var b strings.Builder

	b.WriteString(sectionHeaderStyle.Render("Connections"))
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(fmt.Sprintf("> %s_\n", m.filterText))
	} else if m.filterText != "" {
		b.WriteString(fmt.Sprintf("> %s\n", m.filterText))
	} else {
		b.WriteString("  /filter...\n")
	}

	conns := m.filteredConnections()
	for i, c := range conns {
		style := normalStyle
		prefix := "  "
		if m.focus == focusSidebar && i == m.cursor {
			style = selectedStyle
			prefix = "> "
		}
		b.WriteString(style.Render(prefix + c.Name))
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
		return "No connections. Use 'hangar add' or 'hangar sync' to get started."
	}
	if m.cursor >= len(conns) {
		return ""
	}

	c := conns[m.cursor]
	var b strings.Builder
	b.WriteString(selectedStyle.Render(c.Name))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Host: %s\n", c.Host))
	b.WriteString(fmt.Sprintf("  Port: %d\n", c.Port))
	b.WriteString(fmt.Sprintf("  User: %s\n", c.User))
	if c.IdentityFile != "" {
		b.WriteString(fmt.Sprintf("  Key:  %s\n", c.IdentityFile))
	}
	if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
		b.WriteString("  Pass: ****\n")
	}
	if c.JumpHost != "" {
		b.WriteString(fmt.Sprintf("  Jump: %s\n", c.JumpHost))
	}
	if len(c.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(c.Tags, ", ")))
	}
	if c.SyncedFromSSHConfig {
		b.WriteString("  Source: synced from SSH config\n")
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
	b.WriteString(selectedStyle.Render("Import from SSH Config"))
	b.WriteString("\n\n")

	for i, entry := range m.syncEntries {
		check := "[ ]"
		if m.syncSelected[i] {
			check = "[x]"
		}

		// Show if already imported
		_, err := m.cfg.FindByName(entry.Name)
		alreadyImported := err == nil

		prefix := "  "
		style := normalStyle
		if i == m.syncCursor {
			prefix = "> "
			style = selectedStyle
		}

		label := fmt.Sprintf("%s %s  %s@%s:%d", check, entry.Name, entry.User, entry.Host, entry.Port)
		if alreadyImported {
			label += " (imported)"
		}
		b.WriteString(style.Render(prefix + label))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  [space] toggle  [a]ll  [n]one  [enter] import  [esc] cancel"))

	return b.String()
}

func (m Model) renderForm() string {
	var b strings.Builder
	title := "Add Connection"
	if m.form == formEdit {
		title = "Edit Connection"
	}
	b.WriteString(selectedStyle.Render(title))
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		label := fmt.Sprintf("  %s: ", fieldLabels[i])
		value := m.formFields[i]

		// Mask password field
		displayValue := value
		if i == fieldPassword && value != "" {
			displayValue = strings.Repeat("*", len(value))
		}

		if i == m.formCursor {
			b.WriteString(selectedStyle.Render(label))
			b.WriteString(displayValue + "_")
		} else {
			if m.form == formEdit && i == fieldName {
				b.WriteString(normalStyle.Render(label + displayValue + " (read-only)"))
			} else {
				b.WriteString(normalStyle.Render(label + displayValue))
			}
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  Error: " + m.formError))
	}

	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  [Tab] next field  [Shift+Tab] prev  [Enter] save  [Esc] cancel"))

	return b.String()
}

func (m Model) renderDeleteConfirm() string {
	var b strings.Builder
	b.WriteString(selectedStyle.Render("Delete Connection"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Remove %q?\n\n", m.formTarget))
	b.WriteString(normalStyle.Render("  [y] yes  [n/Esc] cancel"))
	return b.String()
}

func (m Model) renderTagInput() string {
	var b strings.Builder
	b.WriteString(selectedStyle.Render("Manage Tags"))
	b.WriteString("\n\n")

	// Show current tags
	c, err := m.cfg.FindByName(m.formTarget)
	if err == nil {
		if len(c.Tags) > 0 {
			b.WriteString(fmt.Sprintf("  Current: %s\n\n", strings.Join(c.Tags, ", ")))
		} else {
			b.WriteString("  Current: (none)\n\n")
		}
	}

	b.WriteString(fmt.Sprintf("  Tags: %s_\n", m.tagInput))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  Comma-separated. Prefix with - to remove (e.g. -api)"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  [Enter] save  [Esc] cancel"))

	return b.String()
}

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
