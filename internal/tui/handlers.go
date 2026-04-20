package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/v4run/hangar/internal/config"
	sshauth "github.com/v4run/hangar/internal/ssh"
)

func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.cursor = 0
		m.sidebarOffset = 0
	case "enter":
		m.filtering = false
		m.cursor = 0
		m.sidebarOffset = 0
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
		m.cursor = 0
		m.sidebarOffset = 0
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.cursor = 0
			m.sidebarOffset = 0
		}
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
	case " ":
		// For cycle fields, space cycles through options
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == current && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
		// Otherwise treat space as a character
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += " "
		}
	case "backspace":
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			// For cycle fields, backspace clears the value
			if _, ok := fieldCycleOptions[m.formCursor]; ok {
				m.formFields[m.formCursor] = ""
				return m, nil
			}
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
		}
	default:
		// For cycle fields, any key cycles through options
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == current && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
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
		t, cmd := showToast("deleted script", toastOK)
		m.activeToast = &t
		return m, cmd
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

func (m Model) handleDeleteGroupConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.formTargetGroup
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
		m.adjustSidebarViewport()
		m.form = formNone
		t, cmd := showToast("deleted group "+name, toastOK)
		m.activeToast = &t
		return m, cmd
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.formTarget.String()
		if c, err := m.cfg.FindByID(m.formTarget); err == nil {
			name = c.Name
		}
		m.cfg.RemoveByID(m.formTarget)
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.adjustSidebarViewport()
		m.form = formNone
		t, cmd := showToast("deleted "+name, toastOK)
		m.activeToast = &t
		return m, cmd
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
			c, err := m.cfg.FindByID(m.formTarget)
			if err != nil {
				m.form = formNone
				return m, nil
			}
			for _, t := range strings.Split(m.tagInput, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				if strings.HasPrefix(t, "-") {
					// Remove tag
					tag := t[1:]
					filtered := c.Tags[:0]
					for _, existing := range c.Tags {
						if existing != tag {
							filtered = append(filtered, existing)
						}
					}
					c.Tags = filtered
				} else {
					// Add tag if not present
					found := false
					for _, existing := range c.Tags {
						if existing == t {
							found = true
							break
						}
					}
					if !found {
						c.Tags = append(c.Tags, t)
					}
				}
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
		t, cmd := showToast(fmt.Sprintf("imported %d hosts", imported), toastOK)
		m.activeToast = &t
		return m, cmd
	}
	return m, nil
}

func (m Model) handleGlobalSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.formCursor++
		if m.formCursor == fieldUseGlobalSettings {
			m.formCursor++
		}
		if m.formCursor >= fieldAdvancedCount {
			m.formCursor = fieldForwardAgent
		}
	case "shift+tab", "up":
		m.formCursor--
		if m.formCursor == fieldUseGlobalSettings {
			m.formCursor--
		}
		if m.formCursor < fieldForwardAgent {
			m.formCursor = fieldExtraOptions
		}
	case "enter":
		opts, _ := parseSSHOptionsFromFields(m.formFields)
		m.globalCfg.SSHOptions = opts
		config.SaveGlobal(m.configDir, m.globalCfg)
		m.form = formNone
	case " ":
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == current && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += " "
		}
	case "backspace":
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			if _, ok := fieldCycleOptions[m.formCursor]; ok {
				m.formFields[m.formCursor] = ""
				return m, nil
			}
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == current && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
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
	t, cmd := showToast("saved "+name, toastOK)
	m.activeToast = &t
	return m, cmd
}
