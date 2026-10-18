package tui

import (
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// orderedChildren returns every direct child row (sub-group,
// connection, database) of `parent` sorted by (Order asc, name asc)
// so a connection with Order=5 can sit between two groups with Order
// 4 and 6. Ties fall back to name for stability.
func (m *Model) orderedChildren(
	parent uuid.UUID,
	kids map[uuid.UUID][]int,
	conns map[uuid.UUID][]int,
	dbs map[uuid.UUID][]int,
) []row {
	cfg := m.cfg
	var out []row
	for _, gi := range kids[parent] {
		out = append(out, row{kind: rowGroup, group: &cfg.Groups[gi]})
	}
	for _, ci := range conns[parent] {
		out = append(out, row{kind: rowConnection, conn: &cfg.Connections[ci]})
	}
	for _, di := range dbs[parent] {
		out = append(out, row{kind: rowDatabase, db: &cfg.Databases[di]})
	}
	sort.SliceStable(out, func(a, b int) bool {
		oa, ob := rowOrder(out[a]), rowOrder(out[b])
		if oa != ob {
			return oa < ob
		}
		return rowSortName(out[a]) < rowSortName(out[b])
	})
	return out
}

func rowOrder(r row) int {
	switch r.kind {
	case rowGroup:
		return r.group.Order
	case rowConnection:
		return r.conn.Order
	case rowDatabase:
		return r.db.Order
	}
	return 0
}

func rowSortName(r row) string {
	switch r.kind {
	case rowGroup:
		return r.group.Name
	case rowConnection:
		return r.conn.Name
	case rowDatabase:
		return r.db.Name
	}
	return ""
}

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

	// Index groups / connections / databases by parent group, in the
	// order they appear in the config slice (so J/K reorder is
	// observable).
	childrenOf := map[uuid.UUID][]int{}
	for i := range cfg.Groups {
		p := cfg.Groups[i].Parent
		childrenOf[p] = append(childrenOf[p], i)
	}
	connsByGroup := map[uuid.UUID][]int{}
	for i := range cfg.Connections {
		g := cfg.Connections[i].Group
		connsByGroup[g] = append(connsByGroup[g], i)
	}
	dbsByGroup := map[uuid.UUID][]int{}
	for i := range cfg.Databases {
		g := cfg.Databases[i].Group
		dbsByGroup[g] = append(dbsByGroup[g], i)
	}

	// Recent section at the very top (frecency, up to 5). Hidden when
	// a filter is active.
	if q == "" && m.state != nil {
		top := m.state.TopRecent(5)
		if len(top) > 0 {
			m.rows = append(m.rows, row{kind: rowSectionHeader, group: &config.Group{Name: "Recent"}})
			if !m.recentCollapsed {
				for _, id := range top {
					if c := findConnByID(cfg, id); c != nil {
						m.rows = append(m.rows, row{kind: rowConnection, depth: 1, conn: c})
						continue
					}
					if d := findDBByID(cfg, id); d != nil {
						m.rows = append(m.rows, row{kind: rowDatabase, depth: 1, db: d})
					}
				}
			}
		}
	}

	// Depth-first walk of the group tree. For each parent, children
	// (sub-groups + connections + databases) are interleaved by their
	// stored Order and emitted in that single sequence — so a
	// connection can sit between two groups at the same depth.
	var emit func(parent uuid.UUID, depth int)
	emit = func(parent uuid.UUID, depth int) {
		for _, child := range m.orderedChildren(parent, childrenOf, connsByGroup, dbsByGroup) {
			switch child.kind {
			case rowGroup:
				g := child.group
				if q != "" && !groupContainsMatch(cfg, g.ID, connMatches, dbMatches, childrenOf, connsByGroup, dbsByGroup) {
					continue
				}
				m.rows = append(m.rows, row{kind: rowGroup, depth: depth, group: g})
				if m.collapsed[g.ID] && q == "" {
					continue
				}
				emit(g.ID, depth+1)
			case rowConnection:
				if !connMatches(child.conn) {
					continue
				}
				m.rows = append(m.rows, row{kind: rowConnection, depth: depth, conn: child.conn})
			case rowDatabase:
				if !dbMatches(child.db) {
					continue
				}
				m.rows = append(m.rows, row{kind: rowDatabase, depth: depth, db: child.db})
			}
		}
	}
	emit(uuid.Nil, 0)

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

// groupByID looks up a group by its UUID.
func (m *Model) groupByID(id uuid.UUID) *config.Group {
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].ID == id {
			return &m.cfg.Groups[i]
		}
	}
	return nil
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
