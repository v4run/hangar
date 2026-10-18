package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestConnectionCanSitBetweenTwoGroups: three siblings under root
// — alpha (group), web (connection), bravo (group). Move the
// connection so it ends up between the two groups via J.
func TestConnectionCanSitBetweenTwoGroups(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	cid := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{
			{ID: a, Name: "alpha", Order: 0},
			{ID: b, Name: "bravo", Order: 2},
		},
		Connections: []config.Connection{
			{ID: cid, Name: "web", Host: "x", Order: 1},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	mm := m2.(Model)

	// Rendered order should be: alpha, web, bravo.
	wantOrder := []uuid.UUID{a, cid, b}
	gotOrder := make([]uuid.UUID, 0, 3)
	for _, r := range mm.rows {
		switch {
		case r.group != nil:
			gotOrder = append(gotOrder, r.group.ID)
		case r.conn != nil:
			gotOrder = append(gotOrder, r.conn.ID)
		}
	}
	if len(gotOrder) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(gotOrder), gotOrder)
	}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("row %d: want %v, got %v (full: %+v)", i, wantOrder[i], gotOrder[i], gotOrder)
		}
	}

	// Move the connection cursor onto `web` (row index 1), then J to
	// swap it past bravo so order becomes alpha, bravo, web.
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	mm = m3.(Model)
	if c := mm.selectedConnection(); c == nil || c.ID != cid {
		t.Fatalf("expected cursor on web, got %+v", c)
	}
	m4, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	mm = m4.(Model)
	// Check Order fields are now 0 / 1 / 2 for alpha / bravo / web.
	got := map[uuid.UUID]int{}
	for i := range mm.cfg.Groups {
		got[mm.cfg.Groups[i].ID] = mm.cfg.Groups[i].Order
	}
	for i := range mm.cfg.Connections {
		got[mm.cfg.Connections[i].ID] = mm.cfg.Connections[i].Order
	}
	if got[a] != 0 || got[b] != 1 || got[cid] != 2 {
		t.Fatalf("after J, orders should be alpha=0 bravo=1 web=2, got %v", got)
	}
}

// TestMoveGroupKeepsCursorOnMovedGroup: start with cursor on the
// first group, press J (move down one), the cursor must still be on
// the moved group — not on whatever group got shuffled into the old
// slot.
func TestMoveGroupKeepsCursorOnMovedGroup(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{
			{ID: a, Name: "alpha"},
			{ID: b, Name: "bravo"},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	mm := m2.(Model)
	// Cursor should start on alpha (row 0).
	if g := mm.selectedGroup(); g == nil || g.ID != a {
		t.Fatalf("expected cursor on alpha, got %+v", g)
	}
	// J moves it down one slot — now alpha should be at index 1,
	// bravo at index 0; cursor should still be on alpha.
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	mm = m3.(Model)
	if g := mm.selectedGroup(); g == nil || g.ID != a {
		t.Fatalf("expected cursor to follow alpha after move, got %+v", g)
	}
	// K moves it back up; cursor still on alpha.
	m4, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	mm = m4.(Model)
	if g := mm.selectedGroup(); g == nil || g.ID != a {
		t.Fatalf("expected cursor to follow alpha after move back, got %+v", g)
	}
}
