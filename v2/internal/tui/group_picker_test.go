package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestGroupPickerJumpsCursor: open the Groups picker, type a few
// chars to narrow to one match, press enter — cursor must land on
// the picked group's header row.
func TestGroupPickerJumpsCursor(t *testing.T) {
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
	// Starting cursor is on alpha (row 0).
	if g := mm.selectedGroup(); g == nil || g.ID != a {
		t.Fatalf("expected cursor on alpha, got %+v", g)
	}
	// space → G opens the Groups picker.
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m4, _ := m3.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	mm = m4.(Model)
	if mm.overlay != OverlayPicker {
		t.Fatalf("expected OverlayPicker, got %v", mm.overlay)
	}
	// Type `b` to narrow to bravo.
	m5, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	mm = m5.(Model)
	// Enter — should jump the cursor to bravo.
	m6, _ := mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = m6.(Model)
	if mm.overlay != OverlayNone {
		t.Fatalf("expected picker to close, overlay=%v", mm.overlay)
	}
	if g := mm.selectedGroup(); g == nil || g.ID != b {
		t.Fatalf("expected cursor on bravo after picker select, got %+v (rows: %d)", g, len(mm.rows))
	}
}
