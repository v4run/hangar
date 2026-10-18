package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestRenderEmptyConfig exercises the full render path against a
// freshly-installed config (no groups, no connections). This is the
// scenario that panicked in renderHeader on the first alpha release.
func TestRenderEmptyConfig(t *testing.T) {
	cfg := &config.Config{Schema: config.SchemaVersion}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	out := m2.(Model).View()
	if out == "" {
		t.Fatal("empty-config View returned empty string")
	}
	// Must at least draw the brand strip and the mode chip.
	if !strings.Contains(out, "hangar") {
		t.Fatalf("expected brand in output, got:\n%s", out)
	}
	if !strings.Contains(out, "BROWSE") {
		t.Fatalf("expected mode chip, got:\n%s", out)
	}
}

// TestRenderPopulatedConfig exercises the common happy path.
func TestRenderPopulatedConfig(t *testing.T) {
	g := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Groups: []config.Group{{ID: g, Name: "prod", User: "deploy"}},
		Connections: []config.Connection{
			{ID: uuid.New(), Name: "web-1", Host: "10.0.0.1", Group: g},
		},
	}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	out := m2.(Model).View()
	if !strings.Contains(out, "web-1") {
		t.Fatalf("expected connection name in output:\n%s", out)
	}
	if !strings.Contains(out, "deploy") {
		t.Fatalf("expected inherited user in details:\n%s", out)
	}
}

// TestRenderNarrowTerminal catches layout crashes at tight widths.
func TestRenderNarrowTerminal(t *testing.T) {
	cfg := &config.Config{Schema: config.SchemaVersion}
	m := New(cfg, t.TempDir())
	// 40 wide × 10 tall — small but legitimate.
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	_ = m2.(Model).View()
}

// TestRenderWideTerminal covers the opposite — proportional sidebar
// shouldn't overflow or crash on a huge terminal.
func TestRenderWideTerminal(t *testing.T) {
	cfg := &config.Config{Schema: config.SchemaVersion}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 240, Height: 60})
	_ = m2.(Model).View()
}
