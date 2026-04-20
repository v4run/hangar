package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/v4run/hangar/internal/config"
)

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
	b.WriteString("\n")

	conns := m.filteredConnections()
	items := m.sidebarItems()
	if len(items) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  no connections"))
		return b.String()
	}

	visibleRows := m.sidebarVisibleRows()

	// Up indicator
	if m.sidebarOffset > 0 {
		b.WriteString(dimStyle.Render(" ▲"))
	}
	b.WriteString("\n")

	// Determine visible slice
	start := m.sidebarOffset
	end := start + visibleRows
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
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

	// Down indicator
	if end < len(items) {
		remaining := len(items) - end
		b.WriteString(dimStyle.Render(fmt.Sprintf(" [%d more ↓]", remaining)))
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

	advLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(10)
	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		label := advLabelStyle.Render(strings.ToLower(advancedFieldLabels[idx]))
		if opts, ok := fieldCycleOptions[i]; ok {
			// Render as cycle selector
			if i == m.formCursor {
				b.WriteString(activeFieldStyle.Render("> " + strings.ToLower(advancedFieldLabels[idx])) + " ")
				b.WriteString(renderCycleOptions(opts, value))
			} else {
				display := value
				if display == "" {
					display = "-"
				}
				b.WriteString("  " + label + " " + normalStyle.Render(display))
			}
		} else {
			// Free text field
			if i == m.formCursor {
				if value == "" {
					ph := fieldPlaceholders[i]
					b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + dimStyle.Render(ph) + cursorStyle.Render("_"))
				} else {
					b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
				}
			} else {
				if value == "" {
					b.WriteString("  " + label + " " + dimStyle.Render(fieldPlaceholders[i]))
				} else {
					b.WriteString("  " + label + " " + normalStyle.Render(value))
				}
			}
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

func (m Model) renderDeleteGroupConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Group"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove group ") + selectedStyle.Render(m.formTargetGroup) + normalStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("connections will be ungrouped, not deleted"))
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

	advLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(10)
	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		if i == fieldUseGlobalSettings {
			continue // skip UseGlobal in global settings
		}
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		label := advLabelStyle.Render(strings.ToLower(advancedFieldLabels[idx]))
		if opts, ok := fieldCycleOptions[i]; ok {
			if i == m.formCursor {
				b.WriteString(activeFieldStyle.Render("> " + strings.ToLower(advancedFieldLabels[idx])) + " ")
				b.WriteString(renderCycleOptions(opts, value))
			} else {
				display := value
				if display == "" {
					display = "-"
				}
				b.WriteString("  " + label + " " + normalStyle.Render(display))
			}
		} else {
			if i == m.formCursor {
				if value == "" {
					ph := fieldPlaceholders[i]
					b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + dimStyle.Render(ph) + cursorStyle.Render("_"))
				} else {
					b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
				}
			} else {
				if value == "" {
					b.WriteString("  " + label + " " + dimStyle.Render(fieldPlaceholders[i]))
				} else {
					b.WriteString("  " + label + " " + normalStyle.Render(value))
				}
			}
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  "+m.formError))
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

// renderCycleOptions renders cycle field options with the active one highlighted.
func renderCycleOptions(opts []string, current string) string {
	var parts []string
	for _, o := range opts {
		display := o
		if display == "" {
			display = "-"
		}
		if o == current {
			parts = append(parts, selectedStyle.Render("["+display+"]"))
		} else {
			parts = append(parts, dimStyle.Render(" "+display+" "))
		}
	}
	return strings.Join(parts, " ")
}
