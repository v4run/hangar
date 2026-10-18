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
	// And the whole thing must fit inside the terminal — otherwise
	// the shell will scroll the header off before the user ever
	// sees it. This is the regression that produced the "logo only
	// visible when I press a key" bug.
	if lines := strings.Count(out, "\n") + 1; lines > 30 {
		t.Fatalf("render overflowed terminal height: %d lines in a 30-tall view\n%s", lines, out)
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

// TestOverlayDoesNotLeakEscapes guards against the "escape codes
// visible around the modal" bug: when overlayOn dims the base it
// MUST strip ANSI first or slicing can split a `\x1b[…m` mid-
// sequence and emit `[38;5;…` as plain text. We open the leader
// overlay and look for the signature leaked prefix.
func TestOverlayDoesNotLeakEscapes(t *testing.T) {
	cfg := &config.Config{Schema: config.SchemaVersion}
	m := New(cfg, t.TempDir())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	out := m3.(Model).View()
	// A broken-escape leak looks like a literal `[38;` or `[0m` with
	// no ESC byte preceding it. Strip all legit escapes first; if
	// the remaining plain-text output still contains `[38;` or `[0m`
	// we leaked.
	// (We use a cheap substring check — the exact escape forms
	//  used by lipgloss both start with `[` after the ESC byte.)
	stripped := stripEscForTest(out)
	for _, needle := range []string{"[38;", "[48;", "[0m", "[1m"} {
		if strings.Contains(stripped, needle) {
			t.Fatalf("overlay leaked escape fragment %q into visible text:\n%s", needle, out)
		}
	}
}

// stripEscForTest removes CSI sequences. We do it inline (rather
// than depending on ansi.Strip) so the test fails even if the
// stripping library is the thing with the bug.
func stripEscForTest(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c >= 0x40 && c <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j - 1
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
