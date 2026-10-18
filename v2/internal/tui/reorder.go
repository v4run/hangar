package tui

import (
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// moveSelectedBy moves the selected row up (-1) or down (+1) within
// its *combined* sibling list — groups, connections and databases
// share one position namespace per parent, so a connection can sit
// between two groups. Writes the resulting Order fields back to the
// slices and saves.
func moveSelectedBy_unused() {} // keep godoc happy — kept the old name for grep history

func (m *Model) moveSelectedBy(dir int) {
	if dir == 0 {
		return
	}
	r := m.selectedRow()
	if r.conn == nil && r.db == nil && r.group == nil {
		return
	}

	// Which parent are we reordering within?
	var parent uuid.UUID
	switch {
	case r.conn != nil:
		parent = r.conn.Group
	case r.db != nil:
		parent = r.db.Group
	case r.group != nil:
		parent = r.group.Parent
	}

	// Pull the combined sibling list in current rendered order.
	kids, conns, dbs := m.childLookups()
	siblings := m.orderedChildren(parent, kids, conns, dbs)

	// Find the moved row in that list by id.
	id := rowID(r)
	meAt := -1
	for i, s := range siblings {
		if rowID(s) == id {
			meAt = i
			break
		}
	}
	newPos := meAt + dir
	if meAt < 0 || newPos < 0 || newPos >= len(siblings) {
		return
	}
	// Swap positions in the slice.
	siblings[meAt], siblings[newPos] = siblings[newPos], siblings[meAt]

	// Renumber Order fields densely so adjacent moves stay stable.
	for i, s := range siblings {
		m.setRowOrder(s, i)
	}

	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.setToast("save: "+err.Error(), toastErr)
		return
	}
	m.rebuildRows()
	switch {
	case r.conn != nil:
		m.cursorToConnection(id)
	case r.db != nil:
		m.cursorToDatabase(id)
	case r.group != nil:
		m.cursorToGroup(id)
	}
}

// childLookups is a shared "index by parent" helper used by both the
// sidebar walk and reorder — kept out of orderedChildren so callers
// can share a single pass.
func (m *Model) childLookups() (kids, conns, dbs map[uuid.UUID][]int) {
	kids = map[uuid.UUID][]int{}
	for i := range m.cfg.Groups {
		kids[m.cfg.Groups[i].Parent] = append(kids[m.cfg.Groups[i].Parent], i)
	}
	conns = map[uuid.UUID][]int{}
	for i := range m.cfg.Connections {
		conns[m.cfg.Connections[i].Group] = append(conns[m.cfg.Connections[i].Group], i)
	}
	dbs = map[uuid.UUID][]int{}
	for i := range m.cfg.Databases {
		dbs[m.cfg.Databases[i].Group] = append(dbs[m.cfg.Databases[i].Group], i)
	}
	return
}

// setRowOrder writes `n` into the Order field of the row's backing
// struct in m.cfg (not into the row copy — row holds pointers).
func (m *Model) setRowOrder(r row, n int) {
	switch r.kind {
	case rowGroup:
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].ID == r.group.ID {
				m.cfg.Groups[i].Order = n
				return
			}
		}
	case rowConnection:
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].ID == r.conn.ID {
				m.cfg.Connections[i].Order = n
				return
			}
		}
	case rowDatabase:
		for i := range m.cfg.Databases {
			if m.cfg.Databases[i].ID == r.db.ID {
				m.cfg.Databases[i].Order = n
				return
			}
		}
	}
}

func rowID(r row) uuid.UUID {
	switch r.kind {
	case rowGroup:
		return r.group.ID
	case rowConnection:
		return r.conn.ID
	case rowDatabase:
		return r.db.ID
	}
	return uuid.Nil
}
