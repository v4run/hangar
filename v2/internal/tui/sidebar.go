package tui

import (
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// groupContainsMatch returns true if g (or a descendant) has a
// connection / database that matches the filter predicates. Used so
// filter mode never shows an empty group header.
func groupContainsMatch(
	cfg *config.Config,
	g uuid.UUID,
	connOK func(*config.Connection) bool,
	dbOK func(*config.Database) bool,
	kids map[uuid.UUID][]int,
	conns map[uuid.UUID][]int,
	dbs map[uuid.UUID][]int,
) bool {
	for _, ci := range conns[g] {
		if connOK(&cfg.Connections[ci]) {
			return true
		}
	}
	for _, di := range dbs[g] {
		if dbOK(&cfg.Databases[di]) {
			return true
		}
	}
	for _, ki := range kids[g] {
		if groupContainsMatch(cfg, cfg.Groups[ki].ID, connOK, dbOK, kids, conns, dbs) {
			return true
		}
	}
	return false
}

// rebuildRows flattens the group tree + connections into the ordered
// row list the sidebar renders. Collapsed groups hide their children.
// If the filter is non-empty, any row whose Keywords miss the query
// is skipped; its ancestor group chain stays visible so matches
// render in context.
func (m *Model) rebuildRows() {
	m.rows = m.rows[:0]
	cfg := m.cfg
	if cfg == nil {
		return
	}
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	connMatches := func(c *config.Connection) bool {
		if q == "" {
			return true
		}
		if strings.Contains(strings.ToLower(c.Name), q) ||
			strings.Contains(strings.ToLower(c.Host), q) ||
			strings.Contains(strings.ToLower(c.User), q) {
			return true
		}
		for _, t := range c.Tags {
			if strings.Contains(strings.ToLower(t), q) {
				return true
			}
		}
		return strings.Contains(strings.ToLower(config.GroupPath(cfg, c.Group)), q)
	}
	dbMatches := func(d *config.Database) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strings.ToLower(d.Name+" "+d.Host+" "+d.Engine), q)
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
		c := &cfg.Connections[i]
		if !connMatches(c) {
			continue
		}
		m.rows = append(m.rows, row{kind: rowConnection, conn: c})
	}
	for _, i := range dbsByGroup[uuid.Nil] {
		d := &cfg.Databases[i]
		if !dbMatches(d) {
			continue
		}
		m.rows = append(m.rows, row{kind: rowDatabase, db: d})
	}

	// Depth-first walk of the group tree. When filtering, a group row
	// is only emitted if it or a descendant matches.
	var walk func(parent uuid.UUID, depth int)
	walk = func(parent uuid.UUID, depth int) {
		for _, gi := range childrenOf[parent] {
			g := &cfg.Groups[gi]
			// When filtering, skip the group entirely unless it
			// contains at least one matching row.
			if q != "" && !groupContainsMatch(cfg, g.ID, connMatches, dbMatches, childrenOf, connsByGroup, dbsByGroup) {
				continue
			}
			m.rows = append(m.rows, row{kind: rowGroup, depth: depth, group: g})
			// Collapsed groups still hide children UNLESS the user is
			// filtering: filter mode auto-expands matches so hits are
			// visible without pressing right-arrow on every row.
			if m.collapsed[g.ID] && q == "" {
				continue
			}
			walk(g.ID, depth+1)
			for _, ci := range connsByGroup[g.ID] {
				c := &cfg.Connections[ci]
				if !connMatches(c) {
					continue
				}
				m.rows = append(m.rows, row{kind: rowConnection, depth: depth + 1, conn: c})
			}
			for _, di := range dbsByGroup[g.ID] {
				d := &cfg.Databases[di]
				if !dbMatches(d) {
					continue
				}
				m.rows = append(m.rows, row{kind: rowDatabase, depth: depth + 1, db: d})
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

// selectedDatabase returns the database under the cursor, or nil.
func (m *Model) selectedDatabase() *config.Database {
	return m.selectedRow().db
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
