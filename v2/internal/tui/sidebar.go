package tui

import (
	"sort"

	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// rebuildRows flattens the group tree + connections into the ordered
// row list the sidebar renders. Collapsed groups hide their children.
func (m *Model) rebuildRows() {
	m.rows = m.rows[:0]
	cfg := m.cfg
	if cfg == nil {
		return
	}

	// Index groups by parent (slice of indices into cfg.Groups).
	childrenOf := map[uuid.UUID][]int{}
	for i := range cfg.Groups {
		p := cfg.Groups[i].Parent
		childrenOf[p] = append(childrenOf[p], i)
	}
	for _, idxs := range childrenOf {
		sort.SliceStable(idxs, func(a, b int) bool {
			return cfg.Groups[idxs[a]].Name < cfg.Groups[idxs[b]].Name
		})
	}

	// Connections by group.
	connsByGroup := map[uuid.UUID][]int{}
	for i := range cfg.Connections {
		g := cfg.Connections[i].Group
		connsByGroup[g] = append(connsByGroup[g], i)
	}
	for _, idxs := range connsByGroup {
		sort.SliceStable(idxs, func(a, b int) bool {
			return cfg.Connections[idxs[a]].Name < cfg.Connections[idxs[b]].Name
		})
	}

	// Databases by group (deferred display: show them under a group's
	// connections for now).
	dbsByGroup := map[uuid.UUID][]int{}
	for i := range cfg.Databases {
		g := cfg.Databases[i].Group
		dbsByGroup[g] = append(dbsByGroup[g], i)
	}
	for _, idxs := range dbsByGroup {
		sort.SliceStable(idxs, func(a, b int) bool {
			return cfg.Databases[idxs[a]].Name < cfg.Databases[idxs[b]].Name
		})
	}

	// Ungrouped items first (no header if none exist).
	for _, i := range connsByGroup[uuid.Nil] {
		m.rows = append(m.rows, row{kind: rowConnection, conn: &cfg.Connections[i]})
	}
	for _, i := range dbsByGroup[uuid.Nil] {
		m.rows = append(m.rows, row{kind: rowDatabase, db: &cfg.Databases[i]})
	}

	// Depth-first walk of the group tree.
	var walk func(parent uuid.UUID, depth int)
	walk = func(parent uuid.UUID, depth int) {
		for _, gi := range childrenOf[parent] {
			g := &cfg.Groups[gi]
			m.rows = append(m.rows, row{kind: rowGroup, depth: depth, group: g})
			if m.collapsed[g.ID] {
				continue
			}
			// Child groups (recursive).
			walk(g.ID, depth+1)
			// Connections directly under this group.
			for _, ci := range connsByGroup[g.ID] {
				m.rows = append(m.rows, row{
					kind:  rowConnection,
					depth: depth + 1,
					conn:  &cfg.Connections[ci],
				})
			}
			// Databases.
			for _, di := range dbsByGroup[g.ID] {
				m.rows = append(m.rows, row{
					kind:  rowDatabase,
					depth: depth + 1,
					db:    &cfg.Databases[di],
				})
			}
		}
	}
	walk(uuid.Nil, 0)

	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// selectedRow returns the row under the cursor, or an empty row.
func (m *Model) selectedRow() row {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}
	}
	return m.rows[m.cursor]
}

// selectedConnection returns the connection under the cursor, or nil.
func (m *Model) selectedConnection() *config.Connection {
	return m.selectedRow().conn
}

// selectedGroup returns the group under the cursor, or nil.
func (m *Model) selectedGroup() *config.Group {
	return m.selectedRow().group
}

// adjustSidebarViewport keeps m.cursor within the visible window.
func (m *Model) adjustSidebarViewport(visible int) {
	if visible < 1 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
