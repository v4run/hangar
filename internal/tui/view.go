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
	if m.activeToast != nil {
		glyph := ""
		var glyphStyle lipgloss.Style
		switch m.activeToast.kind {
		case toastOK:
			glyph = "\u2713"
			glyphStyle = successStyle
		case toastErr:
			glyph = "\u2717"
			glyphStyle = errorStyle
		case toastWarn:
			glyph = "\u26a0"
			glyphStyle = warnStyle
		case toastInfo:
			glyph = "\u25b8"
			glyphStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
		}
		brand := dimStyle.Render(" hangar") + dimStyle.Render(" │")
		left := " " + glyphStyle.Render(glyph) + " " + m.activeToast.text
		right := "enter:connect  /:find  q:quit"
		gap := m.width - lipgloss.Width(brand) - lipgloss.Width(left) - len(right) - 1
		if gap < 1 {
			gap = 1
		}
		bar := brand + left + strings.Repeat(" ", gap) + dimStyle.Render(right)
		return statusBarStyle.Render(bar)
	}

	brand := dimStyle.Render(" hangar") + dimStyle.Render(" │")

	var hints string
	switch {
	case m.visualMode:
		hints = cursorStyle.Render(" -- VISUAL -- ") + dimStyle.Render("  j/k:extend  x:cut  y:copy  esc:cancel")
		return statusBarStyle.Render(brand + hints)
	case m.form == formAdd || m.form == formEdit:
		if m.formEditing {
			hints = " " + cursorStyle.Render("-- INSERT --") + "  enter:confirm  esc:discard  ctrl+s:save"
		} else {
			hints = " j/k:navigate  h/l:toggle  enter:edit  ctrl+s:save  esc:cancel"
		}
	case m.form == formDelete || m.form == formDeleteScript || m.form == formDeleteGroup:
		hints = " y:confirm  esc:cancel"
	case m.form == formAddGroup || m.form == formEditGroup:
		hints = " enter:save  esc:cancel"
	case m.form == formGlobalSettings:
		if m.formEditing {
			hints = " " + cursorStyle.Render("-- INSERT --") + "  enter:confirm  esc:discard  ctrl+s:save"
		} else {
			hints = " j/k:navigate  h/l:toggle  enter:edit  ctrl+s:save  esc:cancel"
		}
	case m.form == formTag:
		hints = " enter:save  esc:cancel  (prefix with - to remove)"
	case m.form == formPasteConfirm:
		hints = " r:rename  s:skip  esc:cancel"
	case m.form == formSync:
		hints = " space:toggle  a:all  n:none  /:filter  enter:import  esc:cancel"
	case m.form == formAddScript || m.form == formEditScript:
		hints = " tab:next  enter:save  esc:cancel"
	case m.form == formEditNotes:
		hints = " enter:save  esc:cancel"
	case m.focus == focusScripts:
		if m.width >= 120 {
			hints = " n:new  e:edit  d:del  enter:run  o:notes  h:back  ?:help  q:quit"
		} else if m.width >= 80 {
			hints = " n:new  e:edit  d:del  enter:run  h:back  ?:help  q:quit"
		} else {
			hints = " ?:help  q:quit"
		}
	default:
		if m.width >= 120 {
			hints = " n:new  e:edit  d:del  g:group  x:cut  y:copy  p:paste  G:settings  enter:connect  s:sync  t:tag  l:scripts  /:find  ?:help  q:quit"
		} else if m.width >= 80 {
			hints = " n:new  e:edit  d:del  enter:connect  /:find  ?:help  q:quit"
		} else {
			hints = " ?:help  q:quit"
		}
	}
	return statusBarStyle.Render(brand + hints)
}

func (m Model) renderSidebar() string {
	sidebarW := 24 // usable width inside sidebar (26 minus border)
	var b strings.Builder

	// Sidebar header
	b.WriteString(titleStyle.Render(" ▞▚  hangar"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(" " + strings.Repeat("─", sidebarW-1)))
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(dimStyle.Render(" /") + " " + normalStyle.Render(m.filterText) + cursorStyle.Render("_"))
	} else if m.filterText != "" {
		b.WriteString(dimStyle.Render(" / " + m.filterText))
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
	renderRows := visibleRows
	start := m.sidebarOffset
	end := start + renderRows
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
		isCursor := m.focus == focusSidebar && i == m.cursor

		if item.isGroup {
			arrow := "▾"
			// Count connections in this group
			n := 0
			for _, c := range conns {
				if c.Group == item.group {
					n++
				}
			}
			if m.collapsed[item.group] {
				arrow = "▸"
			}
			groupName := strings.ToUpper(item.group)
			countStr := fmt.Sprintf("%d", n)
			if n == 0 && !m.collapsed[item.group] {
				countStr = "·"
			}

			// Right-align the count
			nameWidth := lipgloss.Width(arrow) + 1 + lipgloss.Width(groupName)
			padWidth := sidebarW - 2 - nameWidth - lipgloss.Width(countStr) // 2 for leading spaces
			if padWidth < 1 {
				padWidth = 1
			}
			pad := strings.Repeat(" ", padWidth)

			if isCursor {
				// Full-width background highlight for selected group
				row := " " + arrow + " " + groupName + pad + countStr
				// Pad to full sidebar width
				rowW := lipgloss.Width(row)
				if rowW < sidebarW {
					row += strings.Repeat(" ", sidebarW-rowW)
				}
				b.WriteString(sidebarSelectedStyle.Render(row))
			} else {
				b.WriteString(" " + groupStyle.Render(arrow+" "+groupName) + pad + dimStyle.Render(countStr))
			}
		} else {
			indent := "  "
			if item.conn.Group != "" {
				indent = "    "
			}
			mark := ""
			if m.isInVisualRange(i) {
				mark = dimStyle.Render(" ·")
			} else if m.cutConnections[item.conn.ID] {
				mark = dimStyle.Render(" ~")
			} else if m.copyConnections[item.conn.ID] {
				mark = dimStyle.Render(" +")
			}
			// Compute available width for name truncation
			availWidth := sidebarW - len(indent)
			if len(mark) > 0 {
				availWidth -= 2
			}
			displayName := item.conn.Name
			if len(displayName) > availWidth {
				displayName = displayName[:availWidth-1] + "…"
			}
			if isCursor {
				// Full-width background highlight for selected connection
				row := indent + displayName
				markW := lipgloss.Width(mark)
				rowW := lipgloss.Width(row)
				if rowW+markW < sidebarW {
					row += strings.Repeat(" ", sidebarW-rowW-markW)
				}
				b.WriteString(sidebarSelectedStyle.Render(row) + mark)
			} else {
				b.WriteString(indent + normalStyle.Render(displayName) + mark)
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
	if m.connecting {
		return m.renderConnecting()
	}

	if m.showHelp {
		return m.renderHelp()
	}

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
	case formPasteConfirm:
		return m.renderPasteConfirm()
	}

	c := m.selectedConnection()
	if c == nil {
		// Cursor might be on a group header
		items := m.sidebarItems()
		if m.cursor < len(items) && items[m.cursor].isGroup {
			groupName := items[m.cursor].group
			n := 0
			for _, c := range m.cfg.Connections {
				if c.Group == groupName {
					n++
				}
			}
			var gb strings.Builder
			gb.WriteString(titleStyle.Render(strings.ToUpper(groupName)))
			gb.WriteString("\n")
			gb.WriteString(dimStyle.Render(fmt.Sprintf("%d connections", n)))
			gb.WriteString("\n\n")
			gb.WriteString(dimStyle.Render("space") + normalStyle.Render("  toggle collapse"))
			gb.WriteString("\n")
			gb.WriteString(dimStyle.Render("e") + normalStyle.Render("      rename group"))
			gb.WriteString("\n")
			gb.WriteString(dimStyle.Render("d") + normalStyle.Render("      delete group"))
			return gb.String()
		}
		if len(m.filteredConnections()) == 0 {
			if m.filterText != "" {
				return m.renderFilterEmpty()
			}
			return m.renderEmptyState()
		}
		return ""
	}

	var b strings.Builder
	detailW := m.width - 31 // sidebar(26) + border(1) + mainPane paddingLeft(2) + margin(2)
	if detailW < 40 {
		detailW = 40
	}

	// Connection name
	b.WriteString(titleStyle.Render(c.Name))
	b.WriteString("\n")

	// SSH command line
	sshCmd := fmt.Sprintf("ssh %s@%s", c.User, c.Host)
	if c.Port != 22 {
		sshCmd += fmt.Sprintf(" -p %d", c.Port)
	}
	b.WriteString(sshCmdStyle.Render(sshCmd))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", detailW)))
	b.WriteString("\n")

	if c.IdentityFile != "" {
		b.WriteString(labelStyle.Render("key") + normalStyle.Render(c.IdentityFile))
		b.WriteString("\n")
	}
	if c.JumpHost != "" {
		b.WriteString(labelStyle.Render("jump") + normalStyle.Render(m.jumpHostDisplay(c.JumpHost)))
		b.WriteString("\n")
	}
	if pass, err := config.GetPassword(c.ID.String()); (err == nil && pass != "") {
		b.WriteString(labelStyle.Render("pass") + dimStyle.Render("********"))
		b.WriteString("\n")
	} else if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
		b.WriteString(labelStyle.Render("pass") + dimStyle.Render("********"))
		b.WriteString("\n")
	}

	if len(c.Tags) > 0 {
		b.WriteString(labelStyle.Render("tags"))
		for i, t := range c.Tags {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(tagStyle.Render("["+t+"]"))
		}
		b.WriteString("\n")
	}

	if c.Notes != "" {
		b.WriteString(labelStyle.Render("notes") + valueStyle.Render(c.Notes))
		b.WriteString("\n")
	}

	// Scripts section
	b.WriteString("\n")
	b.WriteString(sectionDivider("scripts", detailW))
	if m.focus == focusScripts {
		b.WriteString("  " + dimStyle.Render("(l to focus)"))
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
			if s.LastRunAt != nil {
				exitStyle := successStyle
				if s.LastRunExit != 0 {
					exitStyle = errorStyle
				}
				b.WriteString("    " + dimStyle.Render("last: ") + exitStyle.Render(fmt.Sprintf("exit %d", s.LastRunExit)))
				b.WriteString(dimStyle.Render(fmt.Sprintf(" \u00b7 %s \u00b7 %s", relativeTime(*s.LastRunAt), formatScriptDuration(s.LastRunDuration))))
				b.WriteString("\n")
			} else {
				b.WriteString("    " + dimStyle.Render("never run") + "\n")
			}
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
		// Show display name for jump host when not editing
		if i == fieldJump && !(m.formEditing && i == m.formCursor) {
			value = m.jumpHostDisplay(value)
		}

		label := labelStyle.Render(strings.ToLower(fieldLabels[i]))
		if i == m.formCursor {
			if m.formEditing {
				b.WriteString(activeFieldStyle.Render("> ") + label + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
			} else {
				b.WriteString(activeFieldStyle.Render("> ") + label + " " + selectedStyle.Render(value))
			}
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
	b.WriteString(sectionDivider("advanced", m.width-29) + "\n\n")

	advLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(10)
	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		label := advLabelStyle.Render(strings.ToLower(advancedFieldLabels[idx]))
		if opts, ok := fieldCycleOptions[i]; ok {
			// Render as cycle selector
			if i == m.formCursor {
				b.WriteString(activeFieldStyle.Render("> ") + label + " ")
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
				if m.formEditing {
					if value == "" {
						ph := fieldPlaceholders[i]
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + dimStyle.Render(ph) + cursorStyle.Render("_"))
					} else {
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
					}
				} else {
					if value == "" {
						ph := fieldPlaceholders[i]
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + dimStyle.Render(ph))
					} else {
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + selectedStyle.Render(value))
					}
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
	b.WriteString(sectionDivider("options", m.width-29) + "\n\n")

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
				b.WriteString(activeFieldStyle.Render("> ") + label + " ")
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
				if m.formEditing {
					if value == "" {
						ph := fieldPlaceholders[i]
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + dimStyle.Render(ph) + cursorStyle.Render("_"))
					} else {
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
					}
				} else {
					if value == "" {
						ph := fieldPlaceholders[i]
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + dimStyle.Render(ph))
					} else {
						b.WriteString(activeFieldStyle.Render("> ") + label + " " + selectedStyle.Render(value))
					}
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
		b.WriteString(activeFieldStyle.Render("> ") + labelStyle.Render("name") + " " + normalStyle.Render(m.scriptName) + cursorStyle.Render("_"))
	} else {
		b.WriteString("  " + labelStyle.Render("name") + " " + normalStyle.Render(m.scriptName))
	}
	b.WriteString("\n")

	// Command field
	if m.scriptField == 1 {
		b.WriteString(activeFieldStyle.Render("> ") + labelStyle.Render("cmd") + " " + normalStyle.Render(m.scriptCommand) + cursorStyle.Render("_"))
	} else {
		b.WriteString("  " + labelStyle.Render("cmd") + " " + normalStyle.Render(m.scriptCommand))
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
	var b strings.Builder
	name := m.formTarget.String()
	if c, err := m.cfg.FindByID(m.formTarget); err == nil {
		name = c.Name
	}
	b.WriteString(titleStyle.Render("Tags: " + name))
	b.WriteString("\n\n")

	// Render tokens as chips
	b.WriteString("  ")
	for _, t := range m.tagTokens {
		b.WriteString(tagStyle.Render("["+t+"]") + " ")
	}
	// Buffer with cursor
	b.WriteString(normalStyle.Render(m.tagBuffer) + cursorStyle.Render("_"))
	b.WriteString("\n\n")

	// Show existing tags as suggestions
	existing := m.allExistingTags()
	if len(existing) > 0 {
		b.WriteString(dimStyle.Render("  existing: "))
		shown := 0
		for _, t := range existing {
			// Skip tags already in tokens
			skip := false
			for _, tok := range m.tagTokens {
				if tok == t {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			if shown > 0 {
				b.WriteString(dimStyle.Render("  "))
			}
			b.WriteString(dimStyle.Render(t))
			shown++
			if shown >= 10 {
				break
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  space/,:add  backspace:remove  tab:complete  enter:save"))
	return b.String()
}

func (m Model) renderSyncList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Import from SSH Config"))
	b.WriteString("\n\n")

	// Filter bar
	if m.syncFiltering {
		b.WriteString(dimStyle.Render("/") + " " + normalStyle.Render(m.syncFilterText) + cursorStyle.Render("_"))
	} else if m.syncFilterText != "" {
		b.WriteString(dimStyle.Render("/ " + m.syncFilterText))
	}
	filtered := m.filteredSyncEntries()
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("%d / %d shown", len(filtered), len(m.syncEntries))))
	b.WriteString("\n\n")

	// Build a set of visible indices
	filteredSet := make(map[int]bool)
	for _, idx := range filtered {
		filteredSet[idx] = true
	}

	for i, entry := range m.syncEntries {
		if !filteredSet[i] {
			continue
		}

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

func (m Model) renderPasteConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Paste"))
	b.WriteString(" into " + tagStyle.Render(m.pasteTargetGroup))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d items, %d name collisions:", len(m.pasteItems), len(m.pasteCollisions))))
	b.WriteString("\n\n")
	for _, name := range m.pasteCollisions {
		b.WriteString("  " + warnStyle.Render("\u26a0 "+name) + dimStyle.Render(" (conflicts)") + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + cursorStyle.Render("r") + "  rename duplicates (append -copy)\n")
	b.WriteString("  " + cursorStyle.Render("s") + "  skip conflicting items\n")
	b.WriteString("  " + cursorStyle.Render("esc") + "  cancel paste\n")
	return b.String()
}

func (m Model) renderConnecting() string {
	c := m.connectTarget
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  \u259e\u259a  hangar") + "\n\n")
	b.WriteString("    connecting to    " + normalStyle.Render(fmt.Sprintf("%s@%s:%d", c.User, c.Host, c.Port)) + "\n")
	if c.IdentityFile != "" {
		b.WriteString("    identity         " + dimStyle.Render(c.IdentityFile) + "\n")
	}
	if c.JumpHost != "" {
		b.WriteString("    via              " + dimStyle.Render(m.jumpHostDisplay(c.JumpHost)) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("    " + dimStyle.Render("\u2819 establishing connection...") + "\n")
	return b.String()
}

func (m Model) renderEmptyState() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("  ▞▚  hangar") + "  " + dimStyle.Render("ssh bookmarks, organised"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  no connections yet"))
	b.WriteString("\n\n")
	b.WriteString("  " + cursorStyle.Render("n") + "  add your first connection\n")
	b.WriteString("  " + cursorStyle.Render("s") + "  import from ~/.ssh/config\n")
	b.WriteString("  " + cursorStyle.Render("g") + "  create a group first\n")
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  tip: tag hosts with t, attach scripts with l"))
	return b.String()
}

func (m Model) renderFilterEmpty() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  no matches for ") + tagStyle.Render(m.filterText))
	b.WriteString("\n\n")
	b.WriteString("  " + cursorStyle.Render("esc") + "  clear filter\n")
	b.WriteString("  " + cursorStyle.Render("n") + "  add a new connection\n")
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
