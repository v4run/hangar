package tui

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// enterVisual arms visual-range selection anchored on the current
// cursor. Subsequent j/k extend the range.
func (m *Model) enterVisual() {
	m.visualMode = true
	m.visualAnchor = m.cursor
}

func (m *Model) exitVisual() {
	m.visualMode = false
	m.visualAnchor = 0
}

// visualRange returns the [lo, hi] inclusive row indices currently
// highlighted by visual mode, or (0,0,false) if visual isn't active.
func (m Model) visualRange() (lo, hi int, ok bool) {
	if !m.visualMode {
		return 0, 0, false
	}
	lo, hi = m.visualAnchor, m.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi, true
}

// yankSelection copies the visual-range (or just the cursor row if
// visual isn't active) into the copy buffer. Only connections are
// clipped — groups and databases are ignored for now.
func (m *Model) yankSelection(cut bool) int {
	m.cutRows = map[uuid.UUID]bool{}
	m.copyRows = map[uuid.UUID]bool{}
	target := m.copyRows
	if cut {
		target = m.cutRows
	}
	var lo, hi int
	if ok := false; true {
		lo, hi, ok = m.visualRange()
		if !ok {
			lo, hi = m.cursor, m.cursor
		}
	}
	n := 0
	for i := lo; i <= hi; i++ {
		if i < 0 || i >= len(m.rows) {
			continue
		}
		if c := m.rows[i].conn; c != nil {
			target[c.ID] = true
			n++
		}
	}
	m.exitVisual()
	return n
}

// pasteSelection applies the clip buffer onto the group that
// contains the current cursor row (or root if there's no parent
// group). Cut rows are MOVED (Group reassigned). Copy rows are
// DUPLICATED with a fresh UUID and a `-copy` suffix if the new name
// collides with an existing peer.
func (m *Model) pasteSelection() int {
	target := m.cursorTargetGroup()
	cut := len(m.cutRows) > 0
	src := m.copyRows
	if cut {
		src = m.cutRows
	}
	n := 0
	if cut {
		for i := range m.cfg.Connections {
			if src[m.cfg.Connections[i].ID] {
				m.cfg.Connections[i].Group = target
				m.cfg.Connections[i].Order = 0 // force a renumber on next move
				n++
			}
		}
	} else {
		existing := map[string]bool{}
		for _, c := range m.cfg.Connections {
			if c.Group == target {
				existing[c.Name] = true
			}
		}
		for i := range m.cfg.Connections {
			if !src[m.cfg.Connections[i].ID] {
				continue
			}
			dup := m.cfg.Connections[i]
			dup.ID = uuid.New()
			dup.Group = target
			dup.Name = uniqueName(dup.Name, existing)
			existing[dup.Name] = true
			m.cfg.Connections = append(m.cfg.Connections, dup)
			n++
		}
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.setToast("save: "+err.Error(), toastErr)
		return 0
	}
	m.cutRows = map[uuid.UUID]bool{}
	m.copyRows = map[uuid.UUID]bool{}
	m.rebuildRows()
	return n
}

// cursorTargetGroup returns the group id the cursor would consider
// "home" for a paste: the group under the cursor, or the group that
// contains the connection/database under the cursor, or root
// (uuid.Nil).
func (m Model) cursorTargetGroup() uuid.UUID {
	r := m.selectedRow()
	switch {
	case r.group != nil:
		return r.group.ID
	case r.conn != nil:
		return r.conn.Group
	case r.db != nil:
		return r.db.Group
	}
	return uuid.Nil
}

func uniqueName(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-copy-%d", base, i-1)
		if i == 2 {
			candidate = base + "-copy"
		}
		if !taken[candidate] {
			return candidate
		}
	}
	return base + "-copy"
}

// rowIsClipped reports whether the row at the given index belongs to
// an active cut or copy mark (for render-time dim / marker styling).
func (m Model) rowIsClipped(i int) (cut, cop bool) {
	if i < 0 || i >= len(m.rows) {
		return false, false
	}
	c := m.rows[i].conn
	if c == nil {
		return false, false
	}
	return m.cutRows[c.ID], m.copyRows[c.ID]
}
