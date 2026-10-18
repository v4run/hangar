package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestLeaderOpensScriptPalette simulates: pick a connection in the
// sidebar, press space, press s. The palette must be the add-script
// palette.
func TestLeaderOpensScriptPalette(t *testing.T) {
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Connections: []config.Connection{
			{ID: uuid.New(), Name: "web", Host: "x"},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// cursor should already be on the only connection; verify.
	mm := m2.(Model)
	if mm.selectedConnection() == nil {
		t.Fatal("expected a connection under the default cursor")
	}
	// Press space (open leader).
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m3.(Model).overlay != OverlayLeader {
		t.Fatalf("expected OverlayLeader, got %v", m3.(Model).overlay)
	}
	// Press s.
	m4, _ := m3.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	mm = m4.(Model)
	if mm.overlay != OverlayPalette {
		t.Fatalf("expected OverlayPalette after leader s, got %v", mm.overlay)
	}
	if mm.palette.kind != PaletteScript {
		t.Fatalf("expected PaletteScript, got %v", mm.palette.kind)
	}
}

// TestLeaderOpensWhenGroupRowSelected guards against the "space is
// swallowed by group collapse" regression — on a group row, space
// must still open the leader so `space s` / `space o` work after
// moving the cursor onto a group.
func TestLeaderOpensWhenGroupRowSelected(t *testing.T) {
	gid := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{{ID: gid, Name: "prod"}},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	mm := m2.(Model)
	// The only row is the "prod" group — cursor should land on it.
	if mm.selectedGroup() == nil {
		t.Fatal("expected a group row under the default cursor")
	}
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m3.(Model).overlay != OverlayLeader {
		t.Fatalf("expected OverlayLeader after space on a group row, got %v", m3.(Model).overlay)
	}
}

// TestLeaderOpensNotesPalette mirrors the above for `space o`.
func TestLeaderOpensNotesPalette(t *testing.T) {
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Connections: []config.Connection{
			{ID: uuid.New(), Name: "web", Host: "x"},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m3.(Model).overlay != OverlayLeader {
		t.Fatalf("expected OverlayLeader, got %v", m3.(Model).overlay)
	}
	m4, _ := m3.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	mm := m4.(Model)
	if mm.overlay != OverlayPalette {
		t.Fatalf("expected OverlayPalette after leader o, got %v", mm.overlay)
	}
	if mm.palette.kind != PaletteNotes {
		t.Fatalf("expected PaletteNotes, got %v", mm.palette.kind)
	}
}
