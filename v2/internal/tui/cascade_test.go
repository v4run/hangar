package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestCascadeDeleteRemovesWholeSubtree builds a prod → us-east → web
// tree with one connection and one database in `web`, hits delete on
// `prod`, and presses D for cascade. Everything should be gone.
func TestCascadeDeleteRemovesWholeSubtree(t *testing.T) {
	prod := uuid.New()
	east := uuid.New()
	web := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{
			{ID: prod, Name: "prod"},
			{ID: east, Name: "us-east", Parent: prod},
			{ID: web, Name: "web", Parent: east},
		},
		Connections: []config.Connection{
			{ID: uuid.New(), Name: "api-1", Host: "x", Group: web},
		},
		Databases: []config.Database{
			{ID: uuid.New(), Name: "cache", Engine: "redis", Group: web},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// Cursor should be on prod (first row).
	mm := m2.(Model)
	if g := mm.selectedGroup(); g == nil || g.ID != prod {
		t.Fatalf("expected cursor on prod, got %+v", g)
	}
	// Press d to open delete-group palette.
	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m3.(Model).palette.kind != PaletteDeleteGroup {
		t.Fatalf("expected PaletteDeleteGroup, got %v", m3.(Model).palette.kind)
	}
	// Press shift+Y for cascade.
	m4, _ := m3.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
	mm = m4.(Model)
	if len(mm.cfg.Groups) != 0 {
		t.Fatalf("expected all groups removed, got %+v", mm.cfg.Groups)
	}
	if len(mm.cfg.Connections) != 0 {
		t.Fatalf("expected all connections removed, got %+v", mm.cfg.Connections)
	}
	if len(mm.cfg.Databases) != 0 {
		t.Fatalf("expected all databases removed, got %+v", mm.cfg.Databases)
	}
}

// TestReparentDeletePreservesSubtree exercises the default `y`
// path — the children survive, reparented to the deleted group's
// parent.
func TestReparentDeletePreservesSubtree(t *testing.T) {
	prod := uuid.New()
	east := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{
			{ID: prod, Name: "prod"},
			{ID: east, Name: "us-east", Parent: prod},
		},
		Connections: []config.Connection{
			{ID: uuid.New(), Name: "api-1", Host: "x", Group: east},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// Move cursor onto `us-east` — one down from prod.
	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	mm := m3.(Model)
	if g := mm.selectedGroup(); g == nil || g.ID != east {
		t.Fatalf("expected cursor on us-east, got %+v", g)
	}
	m4, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	// y re-parents.
	m5, _ := m4.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	mm = m5.(Model)
	if len(mm.cfg.Groups) != 1 || mm.cfg.Groups[0].ID != prod {
		t.Fatalf("expected only prod left, got %+v", mm.cfg.Groups)
	}
	if len(mm.cfg.Connections) != 1 || mm.cfg.Connections[0].Group != prod {
		t.Fatalf("expected api-1 reparented to prod, got %+v", mm.cfg.Connections)
	}
}
