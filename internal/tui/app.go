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
	title := titleStyle.Render("HANGAR")
	syncIndicator := ""
	if m.sshConfigChanged {
		syncIndicator = syncIndicatorStyle.Render(" * SSH config changed ")
	}
	titlePad := m.width - lipgloss.Width(title) - lipgloss.Width(syncIndicator)
	if titlePad < 0 {
		titlePad = 0
	}
	titleBar := titleBarStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			title,
			lipgloss.NewStyle().Width(titlePad).Background(surface).Render(""),
			syncIndicator,
		),
	)

	// Content area
	contentHeight := m.height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}
	sidebarWidth := 28

	sidebar := m.renderSidebar()
	mainPane := m.renderMainPane()

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Width(sidebarWidth).Height(contentHeight).MaxHeight(contentHeight).Render(sidebar),
		mainPaneStyle.Width(m.width-sidebarWidth-4).Height(contentHeight).MaxHeight(contentHeight).Render(mainPane),
	)

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, content, statusBar)
}

func styledKey(key, label string) string {
	return statusKeyStyle.Render(key) + statusBarStyle.Render(" "+label+"  ")
}

func (m Model) renderStatusBar() string {
	var bar string
	switch {
	case m.form == formAdd || m.form == formEdit:
		bar = styledKey("Tab", "next") + styledKey("S-Tab", "prev") + styledKey("Enter", "save") + styledKey("Esc", "cancel")
	case m.form == formDelete:
		bar = styledKey("y", "confirm") + styledKey("Esc", "cancel")
	case m.form == formTag:
		bar = styledKey("Enter", "save") + styledKey("Esc", "cancel") + statusBarStyle.Render("  prefix with - to remove")
	case m.form == formSync:
		bar = styledKey("Space", "toggle") + styledKey("a", "all") + styledKey("n", "none") + styledKey("Enter", "import") + styledKey("Esc", "cancel")
	default:
		bar = styledKey("n", "new") + styledKey("e", "edit") + styledKey("d", "del") + styledKey("Enter", "connect") + styledKey("s", "sync") + styledKey("t", "tag") + styledKey("/", "find") + styledKey("q", "quit")
	}
	return statusBarStyle.Width(m.width).Render(bar)
}

func (m Model) renderSidebar() string {
	var b strings.Builder

	b.WriteString(sectionHeaderStyle.Render("Connections"))
	b.WriteString("\n")

	// Filter bar
	if m.filtering {
		b.WriteString(filterPromptStyle.Render(" / ") + filterInputStyle.Render(m.filterText+"_"))
	} else if m.filterText != "" {
		b.WriteString(filterPromptStyle.Render(" / ") + filterInputStyle.Render(m.filterText))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(dimText).Render("  / filter..."))
	}
	b.WriteString("\n\n")

	conns := m.filteredConnections()
	if len(conns) == 0 {
		b.WriteString(emptyStyle.Render("  No connections"))
		return b.String()
	}

	for i, c := range conns {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" " + c.Name))
		} else {
			b.WriteString(normalStyle.Render(c.Name))
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
		return emptyStyle.Render("No connections\n\nPress n to add or s to sync from SSH config")
	}
	if m.cursor >= len(conns) {
		return ""
	}

	c := conns[m.cursor]
	var b strings.Builder

	b.WriteString(detailTitleStyle.Render(c.Name))
	b.WriteString("\n\n")

	row := func(label, value string) {
		b.WriteString(detailLabelStyle.Render(label) + detailValueStyle.Render(value) + "\n")
	}

	row("Host", c.Host)
	row("Port", fmt.Sprintf("%d", c.Port))
	row("User", c.User)
	if c.IdentityFile != "" {
		row("Key", c.IdentityFile)
	}
	if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
		row("Pass", "********")
	}
	if c.JumpHost != "" {
		row("Jump", c.JumpHost)
	}

	if len(c.Tags) > 0 {
		b.WriteString("\n")
		for _, t := range c.Tags {
			b.WriteString(tagStyle.Render(t))
		}
		b.WriteString("\n")
	}

	if c.SyncedFromSSHConfig {
		b.WriteString("\n")
		b.WriteString(importedBadge.Render("synced from SSH config"))
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
	b.WriteString(formTitleStyle.Render("Import from SSH Config"))
	b.WriteString("\n\n")

	for i, entry := range m.syncEntries {
		var check string
		if m.syncSelected[i] {
			check = checkboxOn.Render("[x]")
		} else {
			check = checkboxOff.Render("[ ]")
		}

		_, err := m.cfg.FindByName(entry.Name)
		alreadyImported := err == nil

		name := detailValueStyle.Render(entry.Name)
		detail := lipgloss.NewStyle().Foreground(dimText).Render(
			fmt.Sprintf("  %s@%s:%d", entry.User, entry.Host, entry.Port))
		badge := ""
		if alreadyImported {
			badge = importedBadge.Render(" (imported)")
		}

		line := check + " " + name + detail + badge
		if i == m.syncCursor {
			b.WriteString(lipgloss.NewStyle().Background(cursorBg).Render(" " + line + " "))
		} else {
			b.WriteString(" " + line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderForm() string {
	var b strings.Builder
	title := "New Connection"
	if m.form == formEdit {
		title = "Edit Connection"
	}
	b.WriteString(formTitleStyle.Render(title))
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		value := m.formFields[i]

		// Mask password field
		displayValue := value
		if i == fieldPassword && value != "" {
			displayValue = strings.Repeat("*", len(value))
		}

		if i == m.formCursor {
			label := formActiveLabel.Render(fieldLabels[i])
			input := formActiveInput.Width(30).Render(displayValue + "_")
			b.WriteString(label + input)
		} else if m.form == formEdit && i == fieldName {
			label := formLabelStyle.Render(fieldLabels[i])
			input := formReadonlyStyle.Render(displayValue)
			b.WriteString(label + input)
		} else {
			label := formLabelStyle.Render(fieldLabels[i])
			input := formInputStyle.Width(30).Render(displayValue)
			b.WriteString(label + input)
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + m.formError))
	}

	return b.String()
}

func (m Model) renderDeleteConfirm() string {
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("Delete Connection"))
	b.WriteString("\n\n")
	b.WriteString(detailValueStyle.Render("Remove "))
	b.WriteString(detailTitleStyle.Render(m.formTarget))
	b.WriteString(detailValueStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(dimText).Render("This cannot be undone."))
	return b.String()
}

func (m Model) renderTagInput() string {
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("Manage Tags"))
	b.WriteString("\n\n")

	c, err := m.cfg.FindByName(m.formTarget)
	if err == nil {
		if len(c.Tags) > 0 {
			for _, t := range c.Tags {
				b.WriteString(tagStyle.Render(t))
			}
			b.WriteString("\n")
		} else {
			b.WriteString(emptyStyle.Render("No tags"))
		}
	}

	b.WriteString("\n")
	b.WriteString(formActiveLabel.Render("Tags"))
	b.WriteString(formActiveInput.Width(30).Render(m.tagInput + "_"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(dimText).Render("Comma-separated. Prefix with - to remove."))

	return b.String()
}

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
