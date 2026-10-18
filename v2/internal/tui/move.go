package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// moveTarget is a pickable row in the move-to picker: every group,
// plus a synthetic "(root)" entry so users can move things out of
// any group entirely.
type moveTarget struct {
	cfg *config.Config
	id  uuid.UUID // uuid.Nil = root
	g   *config.Group
}

func (t moveTarget) Title() string {
	if t.id == uuid.Nil {
		return "(root)"
	}
	return t.g.Name
}

func (t moveTarget) Subtitle() string {
	if t.id == uuid.Nil {
		return "no parent group"
	}
	return config.GroupPath(t.cfg, t.id)
}

func (t moveTarget) Preview() string {
	if t.id == uuid.Nil {
		return "  Move to the root — item becomes ungrouped."
	}
	return "  Move under " + config.GroupPath(t.cfg, t.id) + "."
}

func (t moveTarget) Keywords() []string {
	if t.id == uuid.Nil {
		return []string{"root"}
	}
	return []string{t.g.Name, config.GroupPath(t.cfg, t.id)}
}

// openMovePicker opens a fuzzy picker over every group (plus a root
// option) and, on selection, reparents the currently-selected row.
func (m *Model) openMovePicker() tea.Cmd {
	r := m.selectedRow()
	if r.conn == nil && r.db == nil && r.group == nil {
		m.setToast("nothing selected to move", toastInfo)
		return nil
	}
	// Build the target list. For a group we exclude itself + any
	// descendants so cycle creation is impossible at the UI level.
	forbidden := map[uuid.UUID]bool{}
	if r.group != nil {
		for _, id := range m.groupSubtree(r.group.ID) {
			forbidden[id] = true
		}
	}
	items := []Pickable{moveTarget{cfg: m.cfg, id: uuid.Nil}}
	for i := range m.cfg.Groups {
		g := &m.cfg.Groups[i]
		if forbidden[g.ID] {
			continue
		}
		items = append(items, moveTarget{cfg: m.cfg, id: g.ID, g: g})
	}
	title := "Move " + rowSummary(r) + " to…"
	return m.openPicker(title, items, func(p Pickable) tea.Cmd {
		mt, ok := p.(moveTarget)
		if !ok {
			return nil
		}
		return m.moveSelectedRow(r, mt.id)
	})
}

// moveSelectedRow updates the Group / Parent of the row to `newParent`
// (uuid.Nil = root), saves, and leaves the cursor on the moved row.
func (m *Model) moveSelectedRow(r row, newParent uuid.UUID) tea.Cmd {
	switch {
	case r.conn != nil:
		id := r.conn.ID
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].ID == id {
				m.cfg.Connections[i].Group = newParent
				break
			}
		}
		m.saveAndRecenter(func() { m.cursorToConnection(id) }, "moved "+r.conn.Name)
	case r.db != nil:
		id := r.db.ID
		for i := range m.cfg.Databases {
			if m.cfg.Databases[i].ID == id {
				m.cfg.Databases[i].Group = newParent
				break
			}
		}
		m.saveAndRecenter(func() { m.cursorToDatabase(id) }, "moved "+r.db.Name)
	case r.group != nil:
		id := r.group.ID
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].ID == id {
				m.cfg.Groups[i].Parent = newParent
				break
			}
		}
		m.saveAndRecenter(func() { m.cursorToGroup(id) }, "moved "+r.group.Name)
	}
	return nil
}

func (m *Model) saveAndRecenter(after func(), toastText string) {
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.setToast("save: "+err.Error(), toastErr)
		return
	}
	m.rebuildRows()
	if after != nil {
		after()
	}
	m.setToast(toastText, toastOK)
}

// groupSubtree returns root + every descendant group id. Used by
// move to exclude illegal destinations.
func (m *Model) groupSubtree(root uuid.UUID) []uuid.UUID {
	kids := map[uuid.UUID][]uuid.UUID{}
	for _, g := range m.cfg.Groups {
		kids[g.Parent] = append(kids[g.Parent], g.ID)
	}
	var walk func(id uuid.UUID) []uuid.UUID
	walk = func(id uuid.UUID) []uuid.UUID {
		out := []uuid.UUID{id}
		for _, c := range kids[id] {
			out = append(out, walk(c)...)
		}
		return out
	}
	return walk(root)
}

func rowSummary(r row) string {
	switch {
	case r.conn != nil:
		return "connection " + r.conn.Name
	case r.db != nil:
		return "database " + r.db.Name
	case r.group != nil:
		return "group " + r.group.Name
	}
	return "row"
}
