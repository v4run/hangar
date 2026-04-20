package tui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

// sectionDivider renders a labeled horizontal rule for form sections.
func sectionDivider(label string, width int) string {
	prefix := "── " + label + " "
	remaining := width - len(prefix)
	if remaining < 2 {
		remaining = 2
	}
	return dimStyle.Render(prefix + strings.Repeat("─", remaining))
}

// sidebarVisibleRows returns how many sidebar items fit in the viewport.
func (m Model) sidebarVisibleRows() int {
	// Reserve lines: filter row, blank line after filter, bottom indicator line, status bar
	rows := m.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

// adjustSidebarViewport ensures the cursor is visible in the sidebar viewport.
func (m *Model) adjustSidebarViewport() {
	visibleRows := m.sidebarVisibleRows()
	if m.cursor < m.sidebarOffset {
		m.sidebarOffset = m.cursor
	}
	if m.cursor >= m.sidebarOffset+visibleRows {
		m.sidebarOffset = m.cursor - visibleRows + 1
	}
	if m.sidebarOffset < 0 {
		m.sidebarOffset = 0
	}
}

func (m Model) filteredConnections() []config.Connection {
	if m.filterText == "" {
		return m.cfg.Connections
	}
	var filtered []config.Connection
	lower := strings.ToLower(m.filterText)
	for _, c := range m.cfg.Connections {
		if strings.Contains(strings.ToLower(c.Name), lower) {
			filtered = append(filtered, c)
			continue
		}
		for _, t := range c.Tags {
			if strings.Contains(strings.ToLower(t), lower) {
				filtered = append(filtered, c)
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
