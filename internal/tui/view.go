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

	composed := lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
	return lipgloss.NewStyle().Padding(1, 1).Render(composed)
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
			hints = " " + cursorStyle.Render("-- INSERT --") + "  h/l:toggle  enter:confirm  esc:discard  ctrl+s:save"
		} else {
			hints = " j/k:navigate  enter:edit  ctrl+s:save  esc:cancel"
		}
	case m.form == formDelete || m.form == formDeleteScript || m.form == formDeleteGroup:
		hints = " y:confirm  esc:cancel"
	case m.form == formAddGroup || m.form == formEditGroup:
		hints = " enter:save  esc:cancel"
	case m.form == formGlobalSettings:
		if m.formEditing {
			hints = " " + cursorStyle.Render("-- INSERT --") + "  h/l:toggle  enter:confirm  esc:discard  ctrl+s:save"
		} else {
			hints = " j/k:navigate  enter:edit  i/I:shell-init  ctrl+s:save  esc:cancel"
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
	case m.form == formNewChooser:
		hints = " c:connection  d:database  esc:cancel"
	case m.form == formAddDatabase || m.form == formEditDatabase:
		if m.formEditing {
			hints = " " + cursorStyle.Render("-- INSERT --") + "  h/l:toggle  enter:confirm  esc:discard  ctrl+s:save"
		} else {
			hints = " j/k:navigate  enter:edit  ctrl+s:save  esc:cancel"
		}
	case m.form == formDeleteDatabase:
		hints = " y:confirm  esc:cancel"
	case m.form == formEditShellInit:
		if m.shellInitScope == shellInitScopeConnection {
			hints = " enter:newline  ctrl+s:save  ctrl+e:$EDITOR  ctrl+t:toggle-global  esc:cancel"
		} else {
			hints = " enter:newline  ctrl+s:save  ctrl+e:$EDITOR  esc:cancel"
		}
	case m.focus == focusScripts:
		if m.width >= 120 {
			hints = " n:new  e:edit  d:del  enter:run  o:notes  h:back  ?:help  q:quit"
		} else if m.width >= 80 {
			hints = " n:new  e:edit  d:del  enter:run  h:back  ?:help  q:quit"
		} else {
			hints = " ?:help  q:quit"
		}
	default:
		hints = m.sidebarHints()
	}
	return statusBarStyle.Render(brand + hints)
}

// sidebarHints renders a context-aware status bar for the sidebar (normal
// mode). The visible bindings depend on what's under the cursor — a group
// header, a connection, or nothing — and on terminal width.
func (m Model) sidebarHints() string {
	if m.width < 80 {
		return " ?:help  q:quit"
	}

	items := m.sidebarItems()
	var item sidebarItem
	hasItem := m.cursor < len(items)
	if hasItem {
		item = items[m.cursor]
	}

	hasClipboard := len(m.cutConnections) > 0 || len(m.copyConnections) > 0
	wide := m.width >= 120

	var parts []string
	add := func(p ...string) { parts = append(parts, p...) }

	switch {
	case !hasItem:
		add("n:new", "g:group", "G:settings", "s:sync", "/:find")
	case item.isGroup:
		add("space:fold", "e:rename", "d:del", "n:new", "g:group")
		if wide {
			add("i/I:init")
		}
		add("J/K:reorder")
		if hasClipboard {
			add("p:paste")
		}
		add("/:find")
	default:
		if item.db != nil {
			add("enter:open", "e:edit", "d:del", "t:tag", "/:find")
		} else {
			add("enter:connect", "e:edit", "d:del")
			if wide {
				add("t:tag", "i/I:init", "l:scripts", "o:notes")
			} else {
				add("t:tag", "l:scripts")
			}
			add("x:cut", "y:copy")
			if hasClipboard {
				add("p:paste")
			}
			add("J/K:move", "/:find")
		}
	}
	add("?:help", "q:quit")
	return " " + strings.Join(parts, "  ")
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
			// Count connections + databases in this group
			n := 0
			for _, c := range conns {
				if c.Group == item.group {
					n++
				}
			}
			for _, d := range m.cfg.Databases {
				if d.Group == item.group {
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
		} else if item.conn != nil {
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
		} else if item.db != nil {
			indent := "  "
			if item.db.Group != "" {
				indent = "    "
			}
			badge := " " + dimStyle.Render("["+engineBadge(item.db.Engine)+"]")
			badgeW := lipgloss.Width(badge)
			availWidth := sidebarW - len(indent) - badgeW
			displayName := item.db.Name
			if len(displayName) > availWidth {
				displayName = displayName[:availWidth-1] + "…"
			}
			// Right-align the badge: pad between the name and the badge so
			// the badge always sits flush against the sidebar edge.
			nameSeg := indent + displayName
			nameW := lipgloss.Width(nameSeg)
			pad := sidebarW - nameW - badgeW
			if pad < 1 {
				pad = 1
			}
			row := nameSeg + strings.Repeat(" ", pad) + badge
			if isCursor {
				b.WriteString(sidebarSelectedStyle.Render(row))
			} else {
				b.WriteString(row)
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
	case formEditShellInit:
		return m.renderShellInitForm()
	case formNewChooser:
		return m.renderNewChooser()
	case formAddDatabase, formEditDatabase:
		return m.renderDBForm()
	case formDeleteDatabase:
		return m.renderDeleteDatabaseConfirm()
	}

	items := m.sidebarItems()
	if m.cursor < len(items) && items[m.cursor].db != nil {
		return m.renderDatabaseDetail(items[m.cursor].db)
	}
	c := m.selectedConnection()
	if c == nil {
		if m.cursor < len(items) && items[m.cursor].isGroup {
			groupName := items[m.cursor].group
			n := 0
			for _, c := range m.cfg.Connections {
				if c.Group == groupName {
					n++
				}
			}
			for _, d := range m.cfg.Databases {
				if d.Group == groupName {
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
			gb.WriteString("\n")
			detailW := m.width - 31
			if detailW < 40 {
				detailW = 40
			}
			gb.WriteString(renderShellInitPreview(m, nil, groupName, detailW))
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

	b.WriteString(renderShellInitPreview(m, c, c.Group, detailW))

	return b.String()
}

// renderShellInitPreview renders a "shell init" section for the right pane,
// showing the layered snippets that would apply at connect time for the
// given context. If c is non-nil this is the connection-scope view (all
// three layers); if c is nil the cursor is on a group header (global +
// group layers). Returns "" if no layer contributes content.
func renderShellInitPreview(m Model, c *config.Connection, groupName string, width int) string {
	type layer struct {
		title   string
		content string
	}
	var layers []layer

	includeGlobal := true
	if c != nil && c.UseGlobalShellInit != nil {
		includeGlobal = *c.UseGlobalShellInit
	}
	if includeGlobal && strings.TrimSpace(m.cfg.GlobalShellInit) != "" {
		layers = append(layers, layer{"global", m.cfg.GlobalShellInit})
	} else if !includeGlobal && c != nil && strings.TrimSpace(m.cfg.GlobalShellInit) != "" {
		layers = append(layers, layer{"global (skipped)", ""})
	}
	if groupName != "" {
		if g, ok := m.cfg.GroupShellInit[groupName]; ok && strings.TrimSpace(g) != "" {
			layers = append(layers, layer{fmt.Sprintf("group %q", groupName), g})
		}
	}
	if c != nil && strings.TrimSpace(c.ShellInit) != "" {
		layers = append(layers, layer{"this connection", c.ShellInit})
	}
	if len(layers) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(sectionDivider("shell init", width))
	b.WriteString("\n\n")
	for i, l := range layers {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  " + dimStyle.Render(l.title) + "\n")
		if l.content == "" {
			continue
		}
		for _, line := range strings.Split(strings.TrimRight(l.content, "\n"), "\n") {
			b.WriteString("    " + normalStyle.Render(line) + "\n")
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
	b.WriteString(sectionDivider("basic", m.width-31) + "\n\n")

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
	b.WriteString(sectionDivider("advanced", m.width-31) + "\n\n")

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
	b.WriteString(sectionDivider("options", m.width-31) + "\n\n")

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

func (m Model) renderDatabaseDetail(d *config.Database) string {
	var b strings.Builder
	detailW := m.width - 31
	if detailW < 40 {
		detailW = 40
	}

	b.WriteString(titleStyle.Render(d.Name))
	b.WriteString("  " + dimStyle.Render("["+engineBadge(d.Engine)+"]"))
	b.WriteString("\n")

	// Connection line summary.
	var summary string
	switch d.Engine {
	case config.EngineSQLite:
		summary = "sqlite3 " + d.Host
	default:
		hostPort := fmt.Sprintf("%s:%d", d.Host, d.Port)
		who := d.User
		if d.DBName != "" {
			summary = fmt.Sprintf("%s@%s/%s", who, hostPort, d.DBName)
		} else if who != "" {
			summary = fmt.Sprintf("%s@%s", who, hostPort)
		} else {
			summary = hostPort
		}
	}
	b.WriteString(sshCmdStyle.Render(summary))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", detailW)))
	b.WriteString("\n")

	if d.SSHTunnel != "" {
		b.WriteString(labelStyle.Render("tunnel") + normalStyle.Render(m.jumpHostDisplay(d.SSHTunnel)))
		b.WriteString("\n")
	}
	if d.Engine == config.EnginePostgres {
		client := string(d.Client)
		if client == "" {
			client = "psql"
		}
		b.WriteString(labelStyle.Render("client") + normalStyle.Render(client))
		b.WriteString("\n")
	}
	if pw, err := config.GetPassword(d.ID.String()); err == nil && pw != "" {
		b.WriteString(labelStyle.Render("pass") + dimStyle.Render("********"))
		b.WriteString("\n")
	}
	if len(d.Tags) > 0 {
		b.WriteString(labelStyle.Render("tags"))
		for i, t := range d.Tags {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(tagStyle.Render("[" + t + "]"))
		}
		b.WriteString("\n")
	}
	if d.Notes != "" {
		b.WriteString(labelStyle.Render("notes") + valueStyle.Render(d.Notes))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderNewChooser() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("New"))
	b.WriteString("\n\n")
	b.WriteString("  " + cursorStyle.Render("c") + normalStyle.Render("   Connection — SSH bookmark"))
	b.WriteString("\n")
	b.WriteString("  " + cursorStyle.Render("d") + normalStyle.Render("   Database   — psql / mysql / redis / sqlite"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  esc to cancel"))
	return b.String()
}

func (m Model) renderDeleteDatabaseConfirm() string {
	name := m.formTarget.String()
	if d, err := m.cfg.FindDatabaseByID(m.formTarget); err == nil {
		name = d.Name
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Database"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove ") + selectedStyle.Render(name) + normalStyle.Render("?"))
	return b.String()
}

func (m Model) renderDBForm() string {
	var b strings.Builder
	if m.form == formEditDatabase {
		b.WriteString(titleStyle.Render("Edit Database"))
	} else {
		b.WriteString(titleStyle.Render("New Database"))
	}
	b.WriteString("\n\n")
	b.WriteString(sectionDivider("profile", m.width-31) + "\n\n")

	labelW := 8
	lblStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(labelW)
	for i := 0; i < dbFieldCount; i++ {
		value := m.formFields[i]
		if i == dbFieldPassword && value != "" {
			value = strings.Repeat("*", len(value))
		}
		if i == dbFieldTunnel && !(m.formEditing && i == m.formCursor) {
			value = m.jumpHostDisplay(value)
		}
		label := lblStyle.Render(strings.ToLower(dbFieldLabels[i]))
		dimmed := false
		// Dim postgres-only and tunnel fields when not relevant.
		engine := config.DBEngine(m.formFields[dbFieldEngine])
		if i == dbFieldClient && engine != config.EnginePostgres {
			dimmed = true
		}
		if engine == config.EngineSQLite && (i == dbFieldPort || i == dbFieldUser || i == dbFieldDBName || i == dbFieldTunnel || i == dbFieldPassword) {
			dimmed = true
		}

		if opts, ok := dbFieldCycleOptions[i]; ok {
			if i == m.formCursor {
				b.WriteString(activeFieldStyle.Render("> ") + label + " ")
				b.WriteString(renderCycleOptions(opts, value))
			} else {
				disp := value
				if disp == "" {
					disp = "-"
				}
				style := normalStyle
				if dimmed {
					style = dimStyle
				}
				b.WriteString("  " + label + " " + style.Render(disp))
			}
		} else {
			isActive := i == m.formCursor
			if isActive && m.formEditing {
				if value == "" {
					ph := dbFieldPlaceholders[i]
					b.WriteString(activeFieldStyle.Render("> ") + label + " " + dimStyle.Render(ph) + cursorStyle.Render("_"))
				} else {
					b.WriteString(activeFieldStyle.Render("> ") + label + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
				}
			} else if isActive {
				disp := value
				if disp == "" {
					disp = dbFieldPlaceholders[i]
					b.WriteString(activeFieldStyle.Render("> ") + label + " " + dimStyle.Render(disp))
				} else {
					b.WriteString(activeFieldStyle.Render("> ") + label + " " + selectedStyle.Render(disp))
				}
			} else {
				style := normalStyle
				if dimmed {
					style = dimStyle
				}
				if value == "" {
					if ph := dbFieldPlaceholders[i]; ph != "" {
						b.WriteString("  " + label + " " + dimStyle.Render(ph))
					} else {
						b.WriteString("  " + label + " " + dimStyle.Render("-"))
					}
				} else {
					b.WriteString("  " + label + " " + style.Render(value))
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

func (m Model) renderShellInitForm() string {
	var title string
	switch m.shellInitScope {
	case shellInitScopeGlobal:
		title = "Edit Shell Init — global"
	case shellInitScopeGroup:
		title = fmt.Sprintf("Edit Shell Init — group %q", m.shellInitGroup)
	case shellInitScopeConnection:
		name := "connection"
		inherit := "yes"
		if c, err := m.cfg.FindByID(m.formTarget); err == nil {
			name = c.Name
			if c.UseGlobalShellInit != nil && !*c.UseGlobalShellInit {
				inherit = "no"
			}
		}
		title = fmt.Sprintf("Edit Shell Init — %s   (inherit global: %s)", name, inherit)
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Shell snippet (aliases, functions, exports) sourced before the interactive remote shell."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Inheritance: global → group → connection (later overrides earlier)."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Note: ~/.bashrc and /etc/motd are sourced automatically; 'Last login' line is not shown."))
	b.WriteString("\n\n")
	// Render content line-by-line with a cursor at the end.
	for _, line := range strings.Split(m.shellInitInput, "\n") {
		b.WriteString("  ")
		b.WriteString(normalStyle.Render(line))
		b.WriteString("\n")
	}
	// Remove the trailing newline added by the loop and place cursor inline.
	out := b.String()
	if strings.HasSuffix(out, "\n") {
		out = out[:len(out)-1]
	}
	return out + cursorStyle.Render("_")
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
