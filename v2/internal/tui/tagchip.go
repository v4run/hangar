package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/v4run/hangar/v2/internal/tui/theme"
)

// tagChips is a small inline editor for a list of string tags. Space
// or comma commit the current buffer as a chip; backspace on an empty
// buffer removes the last chip. The chips render as pill-style rounded
// badges with the accent color.
type tagChips struct {
	chips  []string
	buffer textinput.Model
}

func newTagChips(initial []string) tagChips {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "tag"
	ti.CharLimit = 32
	ti.Width = 20
	t := tagChips{chips: append([]string(nil), initial...), buffer: ti}
	return t
}

// Focus hands input focus to the buffer.
func (t *tagChips) Focus() tea.Cmd { return t.buffer.Focus() }

// Blur releases focus.
func (t *tagChips) Blur() { t.buffer.Blur() }

// Values returns the committed chips.
func (t *tagChips) Values() []string {
	// Flush any uncommitted buffer so the caller doesn't lose a half-
	// typed tag.
	out := append([]string(nil), t.chips...)
	if v := strings.TrimSpace(t.buffer.Value()); v != "" {
		out = append(out, v)
	}
	// de-dup
	seen := map[string]bool{}
	dedup := out[:0]
	for _, v := range out {
		if seen[v] {
			continue
		}
		seen[v] = true
		dedup = append(dedup, v)
	}
	return dedup
}

// Update feeds a key to the chip editor. Returns handled=true when
// the chip editor consumed the key.
func (t *tagChips) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case " ", ",":
		if v := strings.TrimSpace(t.buffer.Value()); v != "" {
			t.chips = append(t.chips, v)
			t.buffer.SetValue("")
		}
		return nil, true
	case "backspace":
		if t.buffer.Value() == "" && len(t.chips) > 0 {
			t.chips = t.chips[:len(t.chips)-1]
			return nil, true
		}
	}
	var cmd tea.Cmd
	t.buffer, cmd = t.buffer.Update(msg)
	return cmd, true
}

// View renders the chips followed by the live input buffer.
func (t *tagChips) View() string {
	chipStyle := lipgloss.NewStyle().
		Foreground(theme.Bg).
		Background(theme.AccentDim).
		Padding(0, 1)
	var parts []string
	for _, c := range t.chips {
		parts = append(parts, chipStyle.Render(c))
	}
	parts = append(parts, t.buffer.View())
	return strings.Join(parts, " ")
}
