package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestVisualYankMoveWithPaste: select two connections with v + j,
// cut them, move cursor onto a different group, paste → both
// connections should end up in that group.
func TestVisualYankMoveWithPaste(t *testing.T) {
	g1 := uuid.New()
	g2 := uuid.New()
	c1 := uuid.New()
	c2 := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{
			{ID: g1, Name: "alpha"},
			{ID: g2, Name: "bravo"},
		},
		Connections: []config.Connection{
			{ID: c1, Name: "web1", Host: "x", Group: g1},
			{ID: c2, Name: "web2", Host: "x", Group: g1},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// Expand alpha so children are visible.
	mm := m2.(Model)
	// Walk to first connection under alpha (alpha is row 0; collapsed
	// defaults to false so children render).
	for i, r := range mm.rows {
		if r.conn != nil && r.conn.ID == c1 {
			mm.cursor = i
			break
		}
	}
	// v (enter visual), j (extend to c2), x (cut).
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	mm = m3.(Model)
	m4, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	mm = m4.(Model)
	m5, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	mm = m5.(Model)
	if mm.visualMode {
		t.Fatal("expected visual to exit after x")
	}
	if len(mm.cutRows) != 2 {
		t.Fatalf("expected 2 cut rows, got %d", len(mm.cutRows))
	}
	// Move cursor onto bravo group.
	for i, r := range mm.rows {
		if r.group != nil && r.group.ID == g2 {
			mm.cursor = i
			break
		}
	}
	m6, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	mm = m6.(Model)
	// Both connections should now point at g2.
	for _, c := range mm.cfg.Connections {
		if c.Group != g2 {
			t.Fatalf("connection %q still in %v, expected %v", c.Name, c.Group, g2)
		}
	}
	if len(mm.cutRows) != 0 {
		t.Fatalf("cut buffer should clear after paste, got %d", len(mm.cutRows))
	}
}
