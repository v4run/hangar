package tui

import (
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// moveSelectedBy swaps the selected connection or database with the
// previous (dir=-1) or next (+1) sibling within the same group. For
// a selected group row it swaps with the previous / next sibling
// group under the same parent. Connections and groups are reordered
// *within their peers only* so inheritance isn't accidentally
// restructured. Saves the config after a swap.
func (m *Model) moveSelectedBy(dir int) {
	if dir == 0 {
		return
	}
	r := m.selectedRow()
	// Capture IDs BEFORE the swap — the row's *config.Group /
	// *config.Connection / *config.Database pointers index into the
	// backing slice, and after a swap they point at whatever was
	// moved INTO that slot (not the item we moved).
	var kind rowKind
	var id uuid.UUID
	switch {
	case r.conn != nil:
		kind, id = rowConnection, r.conn.ID
		m.swapConn(r.conn, dir)
	case r.db != nil:
		kind, id = rowDatabase, r.db.ID
		m.swapDB(r.db, dir)
	case r.group != nil:
		kind, id = rowGroup, r.group.ID
		m.swapGroup(r.group, dir)
	default:
		return
	}
	_ = config.Save(m.cfgDir, m.cfg)
	m.rebuildRows()
	switch kind {
	case rowConnection:
		m.cursorToConnection(id)
	case rowDatabase:
		m.cursorToDatabase(id)
	case rowGroup:
		m.cursorToGroup(id)
	}
}

func (m *Model) swapConn(c *config.Connection, dir int) {
	// Find peers (same group) and the index of c among them.
	var peers []int
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].Group == c.Group {
			peers = append(peers, i)
		}
	}
	meAt := -1
	for pi, idx := range peers {
		if m.cfg.Connections[idx].ID == c.ID {
			meAt = pi
			break
		}
	}
	if meAt < 0 {
		return
	}
	newPos := meAt + dir
	if newPos < 0 || newPos >= len(peers) {
		return
	}
	a, b := peers[meAt], peers[newPos]
	m.cfg.Connections[a], m.cfg.Connections[b] = m.cfg.Connections[b], m.cfg.Connections[a]
}

func (m *Model) swapDB(d *config.Database, dir int) {
	var peers []int
	for i := range m.cfg.Databases {
		if m.cfg.Databases[i].Group == d.Group {
			peers = append(peers, i)
		}
	}
	meAt := -1
	for pi, idx := range peers {
		if m.cfg.Databases[idx].ID == d.ID {
			meAt = pi
			break
		}
	}
	if meAt < 0 {
		return
	}
	newPos := meAt + dir
	if newPos < 0 || newPos >= len(peers) {
		return
	}
	a, b := peers[meAt], peers[newPos]
	m.cfg.Databases[a], m.cfg.Databases[b] = m.cfg.Databases[b], m.cfg.Databases[a]
}

func (m *Model) swapGroup(g *config.Group, dir int) {
	var peers []int
	parent := g.Parent
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].Parent == parent {
			peers = append(peers, i)
		}
	}
	meAt := -1
	for pi, idx := range peers {
		if m.cfg.Groups[idx].ID == g.ID {
			meAt = pi
			break
		}
	}
	if meAt < 0 {
		return
	}
	newPos := meAt + dir
	if newPos < 0 || newPos >= len(peers) {
		return
	}
	a, b := peers[meAt], peers[newPos]
	m.cfg.Groups[a], m.cfg.Groups[b] = m.cfg.Groups[b], m.cfg.Groups[a]
}

// used indirectly to shut the compiler up about an unused symbol
// when reorder.go is the only file importing uuid.
var _ = uuid.Nil
