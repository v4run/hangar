package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
	sshauth "github.com/v4run/hangar/internal/ssh"
)

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
