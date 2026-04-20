package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
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
	formAddGroup
	formDeleteGroup
	formAddScript
	formEditScript
	formDeleteScript
	formEditNotes
	formEditGroup
	formGlobalSettings
)

const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldUser
	fieldKey
	fieldJump
	fieldGroup
	fieldTags
	fieldPassword
	fieldCount
)

var fieldLabels = []string{"Name", "Host", "Port", "User", "Key", "Jump", "Group", "Tags", "Pass"}

const (
	fieldForwardAgent = fieldCount + iota
	fieldCompression
	fieldLocalForward
	fieldRemoteForward
	fieldServerAliveInterval
	fieldServerAliveCountMax
	fieldStrictHostKeyCheck
	fieldRequestTTY
	fieldEnvVars
	fieldExtraOptions
	fieldUseGlobalSettings
	fieldAdvancedCount
)

var advancedFieldLabels = []string{
	"FwdAgent", "Compress", "LocalFwd", "RemoteFwd",
	"Alive", "AliveMax", "HostKey", "TTY",
	"Envs", "Extra", "UseGlobal",
}

// sidebarItem represents a row in the sidebar — either a group header or a connection.
type sidebarItem struct {
	isGroup bool
	group   string             // group name (for headers)
	conn    *config.Connection // connection (for connection rows)
}

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
	collapsed        map[string]bool     // collapsed group state
	cutConnections   map[uuid.UUID]bool  // IDs of connections being moved (cut)
	copyConnections  map[uuid.UUID]bool  // IDs of connections being copied
	groupNameInput   string              // input for new group name
	quitting         bool
	form             formMode
	formFields       []string            // field values
	formCursor       int                 // which field is focused
	formError        string              // validation error message
	formTarget       uuid.UUID           // connection ID being edited/deleted/tagged
	formTargetGroup  string              // group name being edited/deleted
	tagInput         string              // input for tag mode
	syncEntries      []config.Connection // parsed SSH config entries for sync selection
	syncSelected     []bool              // selection state per entry
	syncCursor       int                 // cursor position in sync list
	scriptCursor     int                 // cursor in scripts list
	scriptName       string              // script name being added/edited
	scriptCommand    string              // script command being added/edited
	scriptField      int                 // 0=name, 1=command
	scriptTarget     int                 // index of script being edited
	notesInput       string              // notes text being edited
	jumpSuggestions  []config.Connection // autocomplete suggestions for JumpHost
	jumpSugCursor    int                 // cursor in jump suggestions
}

type sshExitMsg struct{ err error }

func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
		collapsed:        make(map[string]bool),
		cutConnections:   make(map[uuid.UUID]bool),
		copyConnections:  make(map[uuid.UUID]bool),
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
		// Handle bracketed paste (bubbletea delivers pasted text as KeyMsg with Paste=true)
		if tea.Key(msg).Paste {
			pasted := string(tea.Key(msg).Runes)
			if m.form == formAdd || m.form == formEdit || m.form == formGlobalSettings {
				if m.formCursor < len(m.formFields) {
					m.formFields[m.formCursor] += pasted
				}
			} else if m.form == formTag {
				m.tagInput += pasted
			} else if m.form == formAddGroup || m.form == formEditGroup {
				m.groupNameInput += pasted
			} else if m.form == formEditNotes {
				m.notesInput += pasted
			} else if m.form == formAddScript || m.form == formEditScript {
				if m.scriptField == 0 {
					m.scriptName += pasted
				} else {
					m.scriptCommand += pasted
				}
			}
			return m, nil
		}
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
		if m.form == formAddGroup {
			return m.handleAddGroupInput(msg)
		}
		if m.form == formDeleteGroup {
			return m.handleDeleteGroupConfirm(msg)
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
		if m.form == formEditGroup {
			return m.handleEditGroupInput(msg)
		}
		if m.form == formGlobalSettings {
			return m.handleGlobalSettingsInput(msg)
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
			items := m.sidebarItems()
			if m.cursor < len(items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ":
			// Toggle group collapse
			items := m.sidebarItems()
			if m.cursor < len(items) && items[m.cursor].isGroup {
				g := items[m.cursor].group
				m.collapsed[g] = !m.collapsed[g]
			}
		case "l":
			// Move focus to scripts pane
			if m.selectedConnection() != nil {
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
			// Pre-fill group from current selection if on a group header or grouped connection
			currentGroup := ""
			items := m.sidebarItems()
			if m.cursor < len(items) {
				if items[m.cursor].isGroup {
					currentGroup = items[m.cursor].group
				} else if items[m.cursor].conn != nil {
					currentGroup = items[m.cursor].conn.Group
				}
			}
			m.formFields = make([]string, fieldAdvancedCount)
			m.formFields[fieldPort] = "22"
			m.formFields[fieldGroup] = currentGroup
			m.formFields[fieldUseGlobalSettings] = "yes"
			m.formCursor = 0
			m.formError = ""
		case "e":
			items := m.sidebarItems()
			if m.cursor < len(items) && items[m.cursor].isGroup {
				// Edit group name
				m.form = formEditGroup
				m.formTargetGroup = items[m.cursor].group
				m.groupNameInput = items[m.cursor].group
				m.formError = ""
			} else {
				c := m.selectedConnection()
				if c != nil {
					m.form = formEdit
					m.formTarget = c.ID
					existingPass, _ := config.GetPassword(c.ID.String())
					if existingPass == "" {
						existingPass, _ = config.GetPassword(c.Name)
					}
					jumpDisplay := m.jumpHostDisplay(c.JumpHost)
					m.formFields = make([]string, fieldAdvancedCount)
					m.formFields[fieldName] = c.Name
					m.formFields[fieldHost] = c.Host
					m.formFields[fieldPort] = fmt.Sprintf("%d", c.Port)
					m.formFields[fieldUser] = c.User
					m.formFields[fieldKey] = c.IdentityFile
					m.formFields[fieldJump] = jumpDisplay
					m.formFields[fieldGroup] = c.Group
					m.formFields[fieldTags] = strings.Join(c.Tags, ", ")
					m.formFields[fieldPassword] = existingPass
					m.populateSSHOptionsFields(c.SSHOptions, c.UseGlobalSettings)
					m.formCursor = 0
					m.formError = ""
				}
			}
		case "d":
			items := m.sidebarItems()
			if m.cursor < len(items) && items[m.cursor].isGroup {
				// Delete group
				m.form = formDeleteGroup
				m.formTargetGroup = items[m.cursor].group
			} else {
				c := m.selectedConnection()
				if c != nil {
					m.form = formDelete
					m.formTarget = c.ID
				}
			}
		case "g":
			// New group
			m.form = formAddGroup
			m.groupNameInput = ""
			m.formError = ""
		case "x":
			// Toggle cut on connection
			c := m.selectedConnection()
			if c != nil {
				if m.cutConnections[c.ID] {
					delete(m.cutConnections, c.ID)
				} else {
					m.cutConnections[c.ID] = true
					delete(m.copyConnections, c.ID)
				}
			}
		case "y":
			// Toggle copy on connection
			c := m.selectedConnection()
			if c != nil {
				if m.copyConnections[c.ID] {
					delete(m.copyConnections, c.ID)
				} else {
					m.copyConnections[c.ID] = true
					delete(m.cutConnections, c.ID)
				}
			}
		case "p":
			// Paste all cut/copied connections into group at cursor
			if len(m.cutConnections) > 0 || len(m.copyConnections) > 0 {
				items := m.sidebarItems()
				targetGroup := ""
				if m.cursor < len(items) {
					if items[m.cursor].isGroup {
						targetGroup = items[m.cursor].group
					} else if items[m.cursor].conn != nil {
						targetGroup = items[m.cursor].conn.Group
					}
				}
				// Handle cut items — move them
				for id := range m.cutConnections {
					c, err := m.cfg.FindByID(id)
					if err == nil {
						c.Group = targetGroup
					}
				}
				// Handle copy items — duplicate them
				for id := range m.copyConnections {
					c, err := m.cfg.FindByID(id)
					if err == nil {
						newConn := *c
						newConn.ID = uuid.New()
						newConn.Group = targetGroup
						m.cfg.Connections = append(m.cfg.Connections, newConn)
					}
				}
				config.Save(m.configDir, m.cfg)
				m.cutConnections = make(map[uuid.UUID]bool)
				m.copyConnections = make(map[uuid.UUID]bool)
			}
		case "G":
			// Open global settings form
			m.form = formGlobalSettings
			m.formFields = make([]string, fieldAdvancedCount)
			m.populateSSHOptionsFields(m.globalCfg.SSHOptions, nil)
			m.formCursor = fieldForwardAgent
			m.formError = ""
		case "t":
			c := m.selectedConnection()
			if c != nil {
				m.form = formTag
				m.formTarget = c.ID
				m.tagInput = ""
			}
		case "enter":
			c := m.selectedConnection()
			if c != nil {
				jumpHost := sshauth.ResolveJumpHost(m.cfg, c.JumpHost)
				var opts *config.SSHOptions
				if c.UseGlobalSettings == nil || *c.UseGlobalSettings {
					mo := config.MergeSSHOptions(m.globalCfg.SSHOptions, c.SSHOptions)
					opts = &mo
				} else {
					opts = c.SSHOptions
				}
				cmd, cleanup := sshauth.NewSSHCommand(c, jumpHost, opts)
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

// sidebarItems builds a flat list of groups and connections for the sidebar.
// Groups are sorted, ungrouped connections come first.
func (m Model) sidebarItems() []sidebarItem {
	conns := m.filteredConnections()

	// Collect groups in order of first appearance
	var groupOrder []string
	groupSeen := make(map[string]bool)
	var ungrouped []config.Connection

	for i := range conns {
		c := &conns[i]
		if c.Group == "" {
			ungrouped = append(ungrouped, *c)
		} else if !groupSeen[c.Group] {
			groupOrder = append(groupOrder, c.Group)
			groupSeen[c.Group] = true
		}
	}

	var items []sidebarItem

	// Ungrouped connections first
	for i := range ungrouped {
		items = append(items, sidebarItem{conn: &ungrouped[i]})
	}

	// Also include empty groups from cfg.Groups
	for g := range m.cfg.Groups {
		if !groupSeen[g] {
			groupOrder = append(groupOrder, g)
			groupSeen[g] = true
		}
	}

	sort.Strings(groupOrder)

	// Grouped connections
	for _, g := range groupOrder {
		items = append(items, sidebarItem{isGroup: true, group: g})
		if !m.collapsed[g] {
			for i := range conns {
				if conns[i].Group == g {
					c := conns[i]
					items = append(items, sidebarItem{conn: &c})
				}
			}
		}
	}

	return items
}

// selectedConnection returns the connection at the current cursor, or nil if cursor is on a group header.
func (m Model) selectedConnection() *config.Connection {
	items := m.sidebarItems()
	if m.cursor >= len(items) || m.cursor < 0 {
		return nil
	}
	item := items[m.cursor]
	if item.isGroup {
		return nil
	}
	c, err := m.cfg.FindByID(item.conn.ID)
	if err != nil {
		return nil
	}
	return c
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
	case m.form == formDelete || m.form == formDeleteScript || m.form == formDeleteGroup:
		bar = " y:confirm  esc:cancel"
	case m.form == formAddGroup || m.form == formEditGroup:
		bar = " enter:save  esc:cancel"
	case m.form == formGlobalSettings:
		bar = " tab:next  shift+tab:prev  enter:save  esc:cancel"
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
		bar = " n:new  e:edit  d:del  g:group  x:cut  y:copy  p:paste  G:settings  enter:connect  s:sync  t:tag  l:scripts  /:find  q:quit"
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
	items := m.sidebarItems()
	if len(items) == 0 {
		b.WriteString(dimStyle.Render("  no connections"))
		return b.String()
	}

	for i, item := range items {
		isCursor := m.focus == focusSidebar && i == m.cursor

		if item.isGroup {
			arrow := "▾"
			count := ""
			if m.collapsed[item.group] {
				arrow = "▸"
				n := 0
				for _, c := range conns {
					if c.Group == item.group {
						n++
					}
				}
				count = dimStyle.Render(fmt.Sprintf(" (%d)", n))
			}
			if isCursor {
				b.WriteString(cursorStyle.Render("> ") + selectedStyle.Render(arrow+" "+item.group) + count)
			} else {
				b.WriteString("  " + headerStyle.Render(arrow+" "+item.group) + count)
			}
		} else {
			indent := "  "
			if item.conn.Group != "" {
				indent = "    "
			}
			mark := ""
			if m.cutConnections[item.conn.ID] {
				mark = dimStyle.Render(" ~")
			} else if m.copyConnections[item.conn.ID] {
				mark = dimStyle.Render(" +")
			}
			if isCursor {
				b.WriteString(cursorStyle.Render("> ") + indent[2:] + selectedStyle.Render(item.conn.Name) + mark)
			} else {
				b.WriteString(indent + normalStyle.Render(item.conn.Name) + mark)
			}
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
	case formAddGroup:
		return m.renderAddGroup()
	case formDeleteGroup:
		return m.renderDeleteGroupConfirm()
	case formAddScript, formEditScript:
		return m.renderScriptForm()
	case formDeleteScript:
		return m.renderDeleteScriptConfirm()
	case formEditNotes:
		return m.renderNotesForm()
	case formEditGroup:
		return m.renderEditGroup()
	case formGlobalSettings:
		return m.renderGlobalSettings()
	}

	c := m.selectedConnection()
	if c == nil {
		// Cursor might be on a group header
		items := m.sidebarItems()
		if m.cursor < len(items) && items[m.cursor].isGroup {
			return dimStyle.Render("group: " + items[m.cursor].group + "\n\npress space to expand/collapse")
		}
		if len(m.filteredConnections()) == 0 {
			return dimStyle.Render("no connections\n\npress n to add or s to sync from SSH config")
		}
		return ""
	}

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
		b.WriteString(dimStyle.Render("via " + m.jumpHostDisplay(c.JumpHost)))
		b.WriteString("\n")
	}
	if pass, err := config.GetPassword(c.ID.String()); (err == nil && pass != "") {
		b.WriteString(dimStyle.Render("pass ********"))
		b.WriteString("\n")
	} else if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
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
		b.WriteString("\n" + errorStyle.Render("  "+m.formError))
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
	return m.selectedConnection()
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
		conn := m.selectedConnection()
		if m.scriptCursor < len(scripts) && conn != nil {
			script := scripts[m.scriptCursor]
			jumpHost := sshauth.ResolveJumpHost(m.cfg, conn.JumpHost)
			var opts *config.SSHOptions
			if conn.UseGlobalSettings == nil || *conn.UseGlobalSettings {
				mo := config.MergeSSHOptions(m.globalCfg.SSHOptions, conn.SSHOptions)
				opts = &mo
			} else {
				opts = conn.SSHOptions
			}
			sshCmd, cleanup := sshauth.NewSSHCommand(conn, jumpHost, opts)
			escapedCmd := strings.ReplaceAll(script.Command, "'", "'\\''")
			remoteCmd := fmt.Sprintf("bash -l -c '%s; printf \"\\npress any key to continue...\"; read -n 1'", escapedCmd)
			userHost := sshCmd.Args[len(sshCmd.Args)-1]
			sshCmd.Args = append(sshCmd.Args[:len(sshCmd.Args)-1],
				"-t", userHost, remoteCmd)
			return m, tea.ExecProcess(sshCmd, func(err error) tea.Msg {
				cleanup()
				return sshExitMsg{err: err}
			})
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

func (m Model) handleAddGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		name := strings.TrimSpace(m.groupNameInput)
		if name == "" {
			m.formError = "group name is required"
			return m, nil
		}
		// Check if group already exists
		for _, c := range m.cfg.Connections {
			if c.Group == name {
				m.formError = "group already exists"
				return m, nil
			}
		}
		// Create a placeholder — groups exist via connections having the group name.
		// Just expand it in collapsed map so it's visible.
		m.collapsed[name] = false
		// We need at least one connection to reference this group,
		// but groups can also just be tracked. Store empty groups separately.
		if m.cfg.Groups == nil {
			m.cfg.Groups = make(map[string]bool)
		}
		m.cfg.Groups[name] = true
		config.Save(m.configDir, m.cfg)
		m.form = formNone
	case "backspace":
		if len(m.groupNameInput) > 0 {
			m.groupNameInput = m.groupNameInput[:len(m.groupNameInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.groupNameInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleEditGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		newName := strings.TrimSpace(m.groupNameInput)
		if newName == "" {
			m.formError = "group name is required"
			return m, nil
		}
		oldName := m.formTargetGroup
		if newName == oldName {
			m.form = formNone
			return m, nil
		}
		// Rename group across all connections
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].Group == oldName {
				m.cfg.Connections[i].Group = newName
			}
		}
		// Update Groups map
		if m.cfg.Groups != nil {
			delete(m.cfg.Groups, oldName)
			m.cfg.Groups[newName] = true
		}
		// Update collapsed state
		if wasCollapsed, ok := m.collapsed[oldName]; ok {
			delete(m.collapsed, oldName)
			m.collapsed[newName] = wasCollapsed
		}
		config.Save(m.configDir, m.cfg)
		m.form = formNone
	case "backspace":
		if len(m.groupNameInput) > 0 {
			m.groupNameInput = m.groupNameInput[:len(m.groupNameInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.groupNameInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleGlobalSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.formCursor++
		if m.formCursor >= fieldAdvancedCount {
			m.formCursor = fieldForwardAgent
		}
	case "shift+tab", "up":
		m.formCursor--
		if m.formCursor < fieldForwardAgent {
			m.formCursor = fieldAdvancedCount - 1
		}
	case "enter":
		opts, _ := parseSSHOptionsFromFields(m.formFields)
		m.globalCfg.SSHOptions = opts
		config.SaveGlobal(m.configDir, m.globalCfg)
		m.form = formNone
	case "backspace":
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleDeleteGroupConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Ungroup all connections in this group
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].Group == m.formTargetGroup {
				m.cfg.Connections[i].Group = ""
			}
		}
		delete(m.collapsed, m.formTargetGroup)
		if m.cfg.Groups != nil {
			delete(m.cfg.Groups, m.formTargetGroup)
		}
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.form = formNone
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
		m.jumpSuggestions = nil
	case "ctrl+n":
		if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
			m.jumpSugCursor = (m.jumpSugCursor + 1) % len(m.jumpSuggestions)
			return m, nil
		}
	case "ctrl+p":
		if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
			m.jumpSugCursor = (m.jumpSugCursor - 1 + len(m.jumpSuggestions)) % len(m.jumpSuggestions)
			return m, nil
		}
	case "tab", "down":
		m.formCursor = (m.formCursor + 1) % fieldAdvancedCount
		m.jumpSuggestions = nil
		m.jumpSugCursor = 0
	case "shift+tab", "up":
		m.formCursor = (m.formCursor - 1 + fieldAdvancedCount) % fieldAdvancedCount
		m.jumpSuggestions = nil
		m.jumpSugCursor = 0
	case "enter":
		// If on JumpHost with suggestions, select the highlighted one
		if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 && m.jumpSugCursor >= 0 && m.jumpSugCursor < len(m.jumpSuggestions) {
			selected := m.jumpSuggestions[m.jumpSugCursor]
			m.formFields[fieldJump] = selected.Name
			m.jumpSuggestions = nil
			m.jumpSugCursor = 0
			return m, nil
		}
		return m.saveForm()
	case "backspace":
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
		}
	default:
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
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
	group := strings.TrimSpace(m.formFields[fieldGroup])
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

	// Resolve JumpHost display name to UUID for storage
	jumpResolved := m.jumpHostResolve(jump)

	// Parse SSH options from advanced fields
	sshOpts, useGlobal := parseSSHOptionsFromFields(m.formFields)

	conn := config.Connection{
		Name:              name,
		Host:              host,
		Port:              port,
		User:              user,
		IdentityFile:      key,
		JumpHost:          jumpResolved,
		Group:             group,
		Tags:              tags,
		SSHOptions:        sshOpts,
		UseGlobalSettings: useGlobal,
	}

	if m.form == formAdd {
		if err := m.cfg.Add(conn); err != nil {
			m.formError = err.Error()
			return m, nil
		}
	} else if m.form == formEdit {
		// Preserve existing fields from original connection
		existing, findErr := m.cfg.FindByID(m.formTarget)
		if findErr == nil {
			conn.ID = existing.ID
			conn.Scripts = existing.Scripts
			conn.Notes = existing.Notes
			conn.SyncedFromSSHConfig = existing.SyncedFromSSHConfig
		}
		m.cfg.RemoveByID(m.formTarget)
		m.cfg.Connections = append(m.cfg.Connections, conn)
	}

	// Save to disk
	if err := config.Save(m.configDir, m.cfg); err != nil {
		m.formError = "Save failed: " + err.Error()
		return m, nil
	}

	// Save or delete password in keychain
	connKey := conn.ID.String()
	if password != "" {
		config.SetPassword(connKey, password)
	} else {
		config.DeletePassword(connKey)
	}

	m.form = formNone
	m.formError = ""
	m.jumpSuggestions = nil
	return m, nil
}

func (m Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.cfg.RemoveByID(m.formTarget)
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
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
			// Look up the connection name from the UUID
			c, err := m.cfg.FindByID(m.formTarget)
			if err != nil {
				m.form = formNone
				return m, nil
			}
			connName := c.Name
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
				m.cfg.AddTags(connName, toAdd)
			}
			if len(toRemove) > 0 {
				m.cfg.RemoveTags(connName, toRemove)
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
		_ = imported
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
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	// JumpHost suggestions
	if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
		for i, s := range m.jumpSuggestions {
			if i > 5 {
				break
			}
			prefix := "    "
			nameStyle := dimStyle
			if i == m.jumpSugCursor {
				prefix = "  > "
				nameStyle = selectedStyle
			}
			b.WriteString(prefix + nameStyle.Render(s.Name) + dimStyle.Render(fmt.Sprintf(" (%s@%s)", s.User, s.Host)) + "\n")
		}
		b.WriteString(dimStyle.Render("  ctrl+n/p: navigate  enter: select") + "\n")
	}

	// Advanced settings section
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Advanced SSH Options"))
	b.WriteString("\n")

	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		label := labelStyle.Render(strings.ToLower(advancedFieldLabels[idx]))
		if i == m.formCursor {
			b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}

func (m Model) renderDeleteConfirm() string {
	// Look up name from UUID
	name := m.formTarget.String()
	if c, err := m.cfg.FindByID(m.formTarget); err == nil {
		name = c.Name
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Connection"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove ") + selectedStyle.Render(name) + normalStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("this cannot be undone"))
	return b.String()
}

func (m Model) renderAddGroup() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("New Group"))
	b.WriteString("\n\n")
	b.WriteString(activeFieldStyle.Render("> name") + " " + normalStyle.Render(m.groupNameInput) + cursorStyle.Render("_"))

	if m.formError != "" {
		b.WriteString("\n\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}

func (m Model) renderEditGroup() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Group"))
	b.WriteString("\n\n")
	b.WriteString(activeFieldStyle.Render("> name") + " " + normalStyle.Render(m.groupNameInput) + cursorStyle.Render("_"))

	if m.formError != "" {
		b.WriteString("\n\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}

func (m Model) renderGlobalSettings() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Global SSH Settings"))
	b.WriteString("\n\n")

	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		if i == fieldUseGlobalSettings {
			continue // skip UseGlobal in global settings
		}
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		label := labelStyle.Render(strings.ToLower(advancedFieldLabels[idx]))
		if i == m.formCursor {
			b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}

func (m Model) renderDeleteGroupConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Group"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove group ") + selectedStyle.Render(m.formTargetGroup) + normalStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("connections will be ungrouped, not deleted"))
	return b.String()
}

func (m Model) renderTagInput() string {
	// Look up name from UUID for display
	connName := m.formTarget.String()
	if c, err := m.cfg.FindByID(m.formTarget); err == nil {
		connName = c.Name
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Tags: " + connName))
	b.WriteString("\n\n")

	if c, err := m.cfg.FindByID(m.formTarget); err == nil && len(c.Tags) > 0 {
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

// jumpHostDisplay converts a JumpHost value (which may be a UUID string) to a display name.
func (m Model) jumpHostDisplay(jumpHost string) string {
	if jumpHost == "" {
		return ""
	}
	if id, err := uuid.Parse(jumpHost); err == nil {
		if c, err := m.cfg.FindByID(id); err == nil {
			return c.Name
		}
	}
	return jumpHost
}

// jumpHostResolve converts a display name to UUID string for storage.
func (m Model) jumpHostResolve(input string) string {
	if input == "" {
		return ""
	}
	// If it's already a UUID, keep it
	if _, err := uuid.Parse(input); err == nil {
		return input
	}
	// Try to find by name
	if c, err := m.cfg.FindByName(input); err == nil {
		return c.ID.String()
	}
	// Return as-is (could be a raw host string)
	return input
}

// jumpHostSuggestions returns connections whose names match the partial input.
func (m Model) jumpHostSuggestions(partial string) []config.Connection {
	if partial == "" {
		return nil
	}
	lower := strings.ToLower(partial)
	var results []config.Connection
	for _, c := range m.cfg.Connections {
		if strings.Contains(strings.ToLower(c.Name), lower) {
			results = append(results, c)
		}
	}
	return results
}

// populateSSHOptionsFields fills formFields with SSHOptions values.
func (m *Model) populateSSHOptionsFields(opts *config.SSHOptions, useGlobal *bool) {
	if opts != nil {
		if opts.ForwardAgent != nil {
			m.formFields[fieldForwardAgent] = boolToYesNo(*opts.ForwardAgent)
		}
		if opts.Compression != nil {
			m.formFields[fieldCompression] = boolToYesNo(*opts.Compression)
		}
		if len(opts.LocalForward) > 0 {
			m.formFields[fieldLocalForward] = strings.Join(opts.LocalForward, ", ")
		}
		if len(opts.RemoteForward) > 0 {
			m.formFields[fieldRemoteForward] = strings.Join(opts.RemoteForward, ", ")
		}
		if opts.ServerAliveInterval != nil {
			m.formFields[fieldServerAliveInterval] = strconv.Itoa(*opts.ServerAliveInterval)
		}
		if opts.ServerAliveCountMax != nil {
			m.formFields[fieldServerAliveCountMax] = strconv.Itoa(*opts.ServerAliveCountMax)
		}
		if opts.StrictHostKeyCheck != "" {
			m.formFields[fieldStrictHostKeyCheck] = opts.StrictHostKeyCheck
		}
		if opts.RequestTTY != "" {
			m.formFields[fieldRequestTTY] = opts.RequestTTY
		}
		if len(opts.EnvVars) > 0 {
			var pairs []string
			for k, v := range opts.EnvVars {
				pairs = append(pairs, k+"="+v)
			}
			m.formFields[fieldEnvVars] = strings.Join(pairs, ", ")
		}
		if len(opts.ExtraOptions) > 0 {
			var pairs []string
			for k, v := range opts.ExtraOptions {
				pairs = append(pairs, k+"="+v)
			}
			m.formFields[fieldExtraOptions] = strings.Join(pairs, ", ")
		}
	}
	if useGlobal != nil {
		m.formFields[fieldUseGlobalSettings] = boolToYesNo(*useGlobal)
	}
}

// parseSSHOptionsFromFields parses advanced form fields into SSHOptions.
func parseSSHOptionsFromFields(fields []string) (*config.SSHOptions, *bool) {
	opts := &config.SSHOptions{}
	hasAny := false

	if fa := strings.TrimSpace(fields[fieldForwardAgent]); fa != "" {
		val := yesNoBool(fa)
		opts.ForwardAgent = &val
		hasAny = true
	}
	if comp := strings.TrimSpace(fields[fieldCompression]); comp != "" {
		val := yesNoBool(comp)
		opts.Compression = &val
		hasAny = true
	}
	if lf := strings.TrimSpace(fields[fieldLocalForward]); lf != "" {
		for _, part := range strings.Split(lf, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				opts.LocalForward = append(opts.LocalForward, part)
			}
		}
		hasAny = true
	}
	if rf := strings.TrimSpace(fields[fieldRemoteForward]); rf != "" {
		for _, part := range strings.Split(rf, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				opts.RemoteForward = append(opts.RemoteForward, part)
			}
		}
		hasAny = true
	}
	if sai := strings.TrimSpace(fields[fieldServerAliveInterval]); sai != "" {
		if val, err := strconv.Atoi(sai); err == nil {
			opts.ServerAliveInterval = &val
			hasAny = true
		}
	}
	if sacm := strings.TrimSpace(fields[fieldServerAliveCountMax]); sacm != "" {
		if val, err := strconv.Atoi(sacm); err == nil {
			opts.ServerAliveCountMax = &val
			hasAny = true
		}
	}
	if shk := strings.TrimSpace(fields[fieldStrictHostKeyCheck]); shk != "" {
		opts.StrictHostKeyCheck = shk
		hasAny = true
	}
	if tty := strings.TrimSpace(fields[fieldRequestTTY]); tty != "" {
		opts.RequestTTY = tty
		hasAny = true
	}
	if envs := strings.TrimSpace(fields[fieldEnvVars]); envs != "" {
		opts.EnvVars = make(map[string]string)
		for _, pair := range strings.Split(envs, ",") {
			pair = strings.TrimSpace(pair)
			if parts := strings.SplitN(pair, "=", 2); len(parts) == 2 {
				opts.EnvVars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if len(opts.EnvVars) > 0 {
			hasAny = true
		} else {
			opts.EnvVars = nil
		}
	}
	if extra := strings.TrimSpace(fields[fieldExtraOptions]); extra != "" {
		opts.ExtraOptions = make(map[string]string)
		for _, pair := range strings.Split(extra, ",") {
			pair = strings.TrimSpace(pair)
			if parts := strings.SplitN(pair, "=", 2); len(parts) == 2 {
				opts.ExtraOptions[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if len(opts.ExtraOptions) > 0 {
			hasAny = true
		} else {
			opts.ExtraOptions = nil
		}
	}

	var useGlobal *bool
	if ug := strings.TrimSpace(fields[fieldUseGlobalSettings]); ug != "" {
		val := yesNoBool(ug)
		useGlobal = &val
	}

	if !hasAny {
		return nil, useGlobal
	}
	return opts, useGlobal
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func yesNoBool(s string) bool {
	return strings.ToLower(s) == "yes" || strings.ToLower(s) == "y" || strings.ToLower(s) == "true"
}

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
