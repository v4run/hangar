package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
	"github.com/v4run/hangar/v2/internal/tui/theme"
)

const (
	minSidebarWidth = 24
	maxSidebarWidth = 40
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	sidebarW := m.sidebarWidth()
	rightW := m.width - sidebarW - 2 // account for the gap

	contentH := m.height - 3 // header 1 + status 1 + margin
	if contentH < 3 {
		contentH = 3
	}

	header := m.renderHeader()
	sidebar := theme.Pane.
		Width(sidebarW - 2).
		Height(contentH).
		Render(m.renderSidebar(sidebarW-4, contentH-2))
	right := theme.Pane.
		Width(rightW - 2).
		Height(contentH).
		Render(m.renderDetail(rightW-4, contentH-2))

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	status := m.renderStatusBar()

	base := lipgloss.JoinVertical(lipgloss.Left, header, body, status)

	// Overlays paint on top.
	switch m.overlay {
	case OverlayCmdBar:
		return overlayLine(base, " "+m.cmdInput.View(), m.width, m.height)
	case OverlayLeader:
		return overlayCenter(base, m.renderLeader(), m.width, m.height)
	case OverlayPicker:
		return overlayCenter(base, m.renderPicker(), m.width, m.height)
	case OverlayPalette:
		return overlayCenter(base, m.renderPalette(), m.width, m.height)
	case OverlayHelp:
		return overlayCenter(base, m.renderHelp(), m.width, m.height)
	}
	return base
}

func (m Model) sidebarWidth() int {
	w := m.width / 4
	if w < minSidebarWidth {
		w = minSidebarWidth
	}
	if w > maxSidebarWidth {
		w = maxSidebarWidth
	}
	if w > m.width-20 {
		w = m.width - 20
	}
	return w
}

func (m Model) renderHeader() string {
	path := ""
	if len(m.rows) > 0 {
		r := m.rows[clampIndex(m.cursor, len(m.rows))]
		if r.conn != nil {
			path = config.GroupPath(m.cfg, r.conn.Group)
		} else if r.group != nil {
			path = config.GroupPath(m.cfg, r.group.ID)
		}
	}
	brand := theme.Title.Render(" ▞▚  hangar ")
	breadcrumb := ""
	if path != "" {
		breadcrumb = theme.Dim.Render(" ▸ ") + theme.Normal.Render(path)
	}
	right := theme.Dim.Render(fmt.Sprintf("%d conns · %d groups", len(m.cfg.Connections), len(m.cfg.Groups)))
	gap := strings.Repeat(" ", maxInt(1, m.width-lipgloss.Width(brand)-lipgloss.Width(breadcrumb)-lipgloss.Width(right)))
	return brand + breadcrumb + gap + right
}

// --- sidebar ---

func (m Model) renderSidebar(w, h int) string {
	if h < 1 {
		return ""
	}
	var b strings.Builder
	// Filter row (always visible — either input or hint).
	switch {
	case m.filtering:
		b.WriteString(m.filter.View())
	case m.filter.Value() != "":
		b.WriteString(theme.Dim.Render("/ " + m.filter.Value()))
	default:
		b.WriteString(theme.Hint.Render("/ filter"))
	}
	b.WriteString("\n\n")
	listH := h - 2
	if listH < 1 {
		return b.String()
	}
	rows := m.rows
	// adjust viewport
	offset := m.offset
	if m.cursor < offset {
		offset = m.cursor
	}
	if m.cursor >= offset+listH {
		offset = m.cursor - listH + 1
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + listH
	if end > len(rows) {
		end = len(rows)
	}
	for i := offset; i < end; i++ {
		b.WriteString(m.renderRow(rows[i], i == m.cursor, w))
		b.WriteString("\n")
	}
	if len(rows) == 0 {
		b.WriteString(theme.Dim.Render("  (no matches)"))
	}
	return b.String()
}

func (m Model) renderRow(r row, selected bool, width int) string {
	indent := strings.Repeat("  ", r.depth)
	var icon, name string
	switch r.kind {
	case rowGroup:
		if m.collapsed[r.group.ID] {
			icon = "▸ "
		} else {
			icon = "▾ "
		}
		name = r.group.Name
	case rowConnection:
		icon = "• "
		name = r.conn.Name
	case rowDatabase:
		icon = "⬢ "
		name = r.db.Name
	}
	line := indent + icon + name
	if lipgloss.Width(line) > width {
		line = truncate(line, width)
	}
	if selected {
		return theme.Selected.Render(line)
	}
	switch r.kind {
	case rowGroup:
		return theme.Normal.Bold(true).Render(line)
	case rowDatabase:
		return theme.Normal.Render(line)
	}
	return theme.Normal.Render(line)
}

// --- detail pane ---

func (m Model) renderDetail(w, h int) string {
	tabs := m.renderTabs(w)
	body := ""
	switch m.tab {
	case TabDetails:
		body = m.renderDetailsTab(w)
	case TabScripts:
		body = m.renderScriptsTab(w)
	case TabForwards:
		body = m.renderForwardsTab(w)
	case TabNotes:
		body = m.renderNotesTab(w)
	}
	// vertical scroll
	lines := strings.Split(body, "\n")
	off := m.detailOffset
	if off > len(lines)-1 {
		off = len(lines) - 1
	}
	if off < 0 {
		off = 0
	}
	visibleHeight := h - 2
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	endL := off + visibleHeight
	if endL > len(lines) {
		endL = len(lines)
	}
	windowed := strings.Join(lines[off:endL], "\n")
	return tabs + "\n\n" + windowed
}

func (m Model) renderTabs(w int) string {
	var parts []string
	for t := Tab(0); t < Tab(tabCount); t++ {
		label := " " + t.String() + " "
		if t == m.tab {
			parts = append(parts, theme.Selected.Underline(true).Render(label))
		} else {
			parts = append(parts, theme.Dim.Render(label))
		}
	}
	return strings.Join(parts, theme.Dim.Render("│"))
}

func (m Model) renderDetailsTab(w int) string {
	c := m.selectedConnection()
	if c == nil {
		if g := m.selectedGroup(); g != nil {
			return m.renderGroupDetails(g)
		}
		return theme.Dim.Render("  (nothing selected)")
	}
	r := config.Resolve(m.cfg, c)
	line := func(label, val string, scope config.Scope, origin string) string {
		if val == "" {
			return ""
		}
		lab := theme.Label.Render(fmt.Sprintf("  %-10s", label))
		suffix := ""
		if scope == config.ScopeGroup && origin != "" {
			suffix = theme.Hint.Render("  — from " + origin + "/")
		} else if scope == config.ScopeGlobal {
			suffix = theme.Hint.Render("  — from global")
		}
		return lab + "  " + theme.Normal.Render(val) + suffix
	}
	var b strings.Builder
	b.WriteString(theme.Title.Render(c.Name))
	b.WriteString("\n")
	b.WriteString(theme.Dim.Render(fmt.Sprintf("  ssh %s", formatSSHHint(m.cfg, r))))
	b.WriteString("\n\n")
	write := func(s string) {
		if s != "" {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	write(line("Host", r.Host, config.ScopeOwn, ""))
	if r.Port != 0 && r.Port != 22 {
		write(line("Port", fmt.Sprintf("%d", r.Port), config.ScopeOwn, ""))
	}
	write(line("User", r.User, r.FieldScope["User"], r.FieldOrigin["User"]))
	write(line("Identity", r.IdentityFile, r.FieldScope["IdentityFile"], r.FieldOrigin["IdentityFile"]))
	if r.JumpHost != uuid.Nil {
		if jc := findConnByID(m.cfg, r.JumpHost); jc != nil {
			write(line("Jump", jc.Name, r.FieldScope["JumpHost"], r.FieldOrigin["JumpHost"]))
		}
	}
	write(line("Group", config.GroupPath(m.cfg, c.Group), config.ScopeOwn, ""))
	if len(r.Tags) > 0 {
		write(line("Tags", strings.Join(r.Tags, " "), config.ScopeOwn, ""))
	}
	if r.RemoteCommand != "" {
		write(line("RemoteCmd", r.RemoteCommand, config.ScopeOwn, ""))
	}
	return b.String()
}

func (m Model) renderGroupDetails(g *config.Group) string {
	var b strings.Builder
	b.WriteString(theme.Title.Render(g.Name))
	b.WriteString("\n")
	b.WriteString(theme.Dim.Render("  " + config.GroupPath(m.cfg, g.ID)))
	b.WriteString("\n\n")
	write := func(label, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(&b, "  %-10s  %s\n",
			theme.Label.Render(label),
			theme.Normal.Render(val))
	}
	write("User", g.User)
	write("Identity", g.IdentityFile)
	if len(g.Tags) > 0 {
		write("Tags", strings.Join(g.Tags, " "))
	}
	// Child counts.
	var kids, conns int
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].Parent == g.ID {
			kids++
		}
	}
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].Group == g.ID {
			conns++
		}
	}
	write("Subgroups", fmt.Sprintf("%d", kids))
	write("Conns", fmt.Sprintf("%d", conns))
	return b.String()
}

func (m Model) renderScriptsTab(w int) string {
	c := m.selectedConnection()
	if c == nil {
		return theme.Dim.Render("  (nothing selected)")
	}
	r := config.Resolve(m.cfg, c)
	if len(r.Scripts) == 0 {
		return theme.Dim.Render("  (no scripts — leader s to add)")
	}
	var b strings.Builder
	for i, s := range r.Scripts {
		marker := "•"
		if i < len(c.Scripts) {
			marker = theme.Cursor.Render("•")
		} else {
			marker = theme.Dim.Render("•")
		}
		fmt.Fprintf(&b, "  %s %s\n", marker, theme.Normal.Render(s.Name))
		fmt.Fprintf(&b, "      %s\n", theme.Dim.Render("$ "+s.Command))
	}
	b.WriteString("\n")
	b.WriteString(theme.Dim.Render("  leader s: new   leader e: edit selected (coming in alpha.2)"))
	return b.String()
}

func (m Model) renderForwardsTab(w int) string {
	c := m.selectedConnection()
	if c == nil {
		return theme.Dim.Render("  (nothing selected)")
	}
	r := config.Resolve(m.cfg, c)
	if r.SSHOptions == nil || (len(r.SSHOptions.LocalForward) == 0 && len(r.SSHOptions.RemoteForward) == 0) {
		return theme.Dim.Render("  (no port forwards configured)")
	}
	var b strings.Builder
	if len(r.SSHOptions.LocalForward) > 0 {
		b.WriteString(theme.Label.Render("  Local → remote"))
		b.WriteString("\n")
		for _, lf := range r.SSHOptions.LocalForward {
			fmt.Fprintf(&b, "    %s\n", theme.Normal.Render(lf))
		}
	}
	if len(r.SSHOptions.RemoteForward) > 0 {
		b.WriteString("\n")
		b.WriteString(theme.Label.Render("  Remote → local"))
		b.WriteString("\n")
		for _, rf := range r.SSHOptions.RemoteForward {
			fmt.Fprintf(&b, "    %s\n", theme.Normal.Render(rf))
		}
	}
	return b.String()
}

func (m Model) renderNotesTab(w int) string {
	c := m.selectedConnection()
	if c == nil {
		return theme.Dim.Render("  (nothing selected)")
	}
	if c.Notes == "" {
		return theme.Dim.Render("  (no notes)")
	}
	return "  " + strings.ReplaceAll(c.Notes, "\n", "\n  ")
}

// --- status bar ---

func (m Model) renderStatusBar() string {
	var chip string
	switch m.mode {
	case ModeEdit:
		chip = theme.ChipEdit.Render("EDIT")
	case ModeConnect:
		chip = theme.ChipConnect.Render("CONNECT")
	default:
		chip = theme.ChipBrowse.Render("BROWSE")
	}
	hints := m.help.View(m.keymap)
	right := ""
	if m.toast != "" {
		switch m.toastKind {
		case toastErr:
			right = theme.ErrorText.Render(m.toast)
		case toastOK:
			right = theme.OKText.Render(m.toast)
		default:
			right = theme.Dim.Render(m.toast)
		}
	}
	gap := strings.Repeat(" ", maxInt(1, m.width-lipgloss.Width(chip)-lipgloss.Width(hints)-lipgloss.Width(right)-2))
	return " " + chip + " " + hints + gap + right + " "
}

// --- overlays ---

func (m Model) renderLeader() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Leader"))
	b.WriteString("\n\n")
	for _, lb := range m.leaderBindings() {
		fmt.Fprintf(&b, "  %s  %s\n",
			theme.Cursor.Render(lb.Key),
			theme.Normal.Render(lb.Desc))
	}
	b.WriteString("\n")
	b.WriteString(theme.Dim.Render("  esc  cancel"))
	return paddedBox(b.String(), 36)
}

func (m Model) renderPicker() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render(m.picker.title))
	b.WriteString("\n")
	b.WriteString(m.picker.query.View())
	b.WriteString("\n\n")

	width := minInt(80, m.width-10)
	listW := width/2 - 2
	previewW := width - listW - 3

	// List
	var listB strings.Builder
	visible := minInt(10, len(m.picker.matches))
	start := 0
	if m.picker.cursor >= visible {
		start = m.picker.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.picker.matches) {
		end = len(m.picker.matches)
	}
	for i := start; i < end; i++ {
		it := m.picker.items[m.picker.matches[i]]
		line := it.Title()
		if sub := it.Subtitle(); sub != "" {
			line += theme.Dim.Render("  " + sub)
		}
		line = truncate(line, listW)
		if i == m.picker.cursor {
			line = theme.Selected.Render("› " + line)
		} else {
			line = "  " + theme.Normal.Render(line)
		}
		listB.WriteString(line)
		listB.WriteString("\n")
	}
	// Preview
	var prev string
	if sel := m.pickerSelected(); sel != nil {
		prev = sel.Preview()
	}
	previewBox := theme.Pane.Width(previewW).Height(visible).Render(prev)
	listBox := lipgloss.NewStyle().Width(listW).Height(visible).Render(listB.String())
	row := lipgloss.JoinHorizontal(lipgloss.Top, listBox, previewBox)
	b.WriteString(row)
	return paddedBox(b.String(), width)
}

func (m Model) renderPalette() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render(m.palette.title))
	b.WriteString("\n\n")
	if m.palette.message != "" {
		b.WriteString(theme.Normal.Render(m.palette.message))
		b.WriteString("\n\n")
	}
	switch m.palette.kind {
	case PaletteHostKey:
		if len(m.pendingKeys) == 0 {
			b.WriteString(theme.Dim.Render("  Fetching host key…"))
			b.WriteString("\n\n")
		} else {
			b.WriteString(theme.Label.Render("  Fingerprints:"))
			b.WriteString("\n")
			for _, k := range m.pendingKeys {
				fmt.Fprintf(&b, "    %s  %s\n",
					theme.Dim.Render(k.KeyType),
					theme.Normal.Render(k.Fingerprint))
			}
			b.WriteString("\n")
			b.WriteString(theme.Normal.Render("  Trust this host and continue?"))
			b.WriteString("\n\n")
			b.WriteString(theme.Dim.Render("  y: trust and connect   n: cancel"))
		}
	case PaletteDeleteConnection:
		b.WriteString(theme.Dim.Render("  y: delete   n/esc: cancel"))
	default:
		for i, f := range m.palette.fields {
			lab := theme.Label.Render(fmt.Sprintf("  %-9s ", f.label))
			var val string
			if i == m.palette.focus {
				val = f.input.View()
			} else {
				val = theme.Dim.Render(f.input.Value())
				if f.input.Value() == "" {
					val = theme.Hint.Render(f.hint)
				}
			}
			b.WriteString(lab + val + "\n")
		}
		b.WriteString("\n")
		b.WriteString(theme.Dim.Render("  tab: next   shift+tab: prev   enter / ctrl+s: save   esc: cancel"))
	}
	if m.palette.err != "" {
		b.WriteString("\n\n")
		b.WriteString(theme.ErrorText.Render("  " + m.palette.err))
	}
	return paddedBox(b.String(), minInt(80, m.width-10))
}

func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(theme.Title.Render("Hangar v2 — help"))
	b.WriteString("\n\n")
	for _, group := range m.keymap.FullHelp() {
		for _, k := range group {
			fmt.Fprintf(&b, "  %-14s  %s\n", theme.Cursor.Render(k.Help().Key), theme.Normal.Render(k.Help().Desc))
		}
		b.WriteString("\n")
	}
	b.WriteString(theme.Dim.Render("  esc / ? to close"))
	return paddedBox(b.String(), minInt(72, m.width-10))
}

// --- helpers ---

func formatSSHHint(cfg *config.Config, r *config.Resolved) string {
	parts := []string{}
	if r.Port != 0 && r.Port != 22 {
		parts = append(parts, fmt.Sprintf("-p %d", r.Port))
	}
	if r.IdentityFile != "" {
		parts = append(parts, "-i "+r.IdentityFile)
	}
	if r.JumpHost != uuid.Nil {
		if j := findConnByID(cfg, r.JumpHost); j != nil {
			parts = append(parts, "-J "+formatConnDest(j))
		}
	}
	parts = append(parts, formatConnDest(r.Connection))
	return strings.Join(parts, " ")
}

func formatConnDest(c *config.Connection) string {
	if c.User != "" {
		return c.User + "@" + c.Host
	}
	return c.Host
}

func findConnByID(cfg *config.Config, id uuid.UUID) *config.Connection {
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == id {
			return &cfg.Connections[i]
		}
	}
	return nil
}

func paddedBox(s string, width int) string {
	return theme.Pane.Width(width).Render(s)
}

func overlayCenter(base, overlay string, width, height int) string {
	// Lipgloss doesn't have a native overlay; render the base and
	// then draw the overlay centered by composing the full-screen
	// output. Simplest: place overlay centered inside a canvas of
	// base-dimensions, letting rows pass through where overlay is
	// empty. We fake it by printing the overlay in a Place block
	// and letting the terminal overwrite — v2 adds a proper
	// overlay compositor in alpha.2.
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "))
}

func overlayLine(base, line string, width, height int) string {
	// Draw the base, then a thin bar near the bottom.
	// Place the cmdbar one row above the status bar.
	rows := strings.Split(base, "\n")
	if len(rows) >= 2 {
		rows[len(rows)-2] = truncate(line+strings.Repeat(" ", width), width)
	}
	return strings.Join(rows, "\n")
}

func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	// Simple: cut on runes until width fits.
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > w-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func clampIndex(i, n int) int {
	if n == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
