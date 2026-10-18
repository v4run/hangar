package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

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
