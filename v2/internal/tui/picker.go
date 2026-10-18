package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/sahilm/fuzzy"
)

// Pickable is anything that can show up in the fuzzy picker.
type Pickable interface {
	Title() string    // e.g. "web-1"
	Subtitle() string // e.g. "prod/us-east • deploy@10.0.4.12"
	Preview() string  // multi-line body shown in the right pane
	Keywords() []string
}

type pickerState struct {
	title    string
	items    []Pickable
	matches  []int // indexes into items for currently filtered rows
	cursor   int
	query    textinput.Model
	onSelect func(Pickable) tea.Cmd
}

// openPicker shows the fuzzy picker with items; onSelect fires when
// the user hits enter on a match.
func (m *Model) openPicker(title string, items []Pickable, onSelect func(Pickable) tea.Cmd) tea.Cmd {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = "type to filter…"
	ti.Focus()
	m.picker = pickerState{
		title:    title,
		items:    items,
		query:    ti,
		onSelect: onSelect,
	}
	m.pickerRefilter()
	m.overlay = OverlayPicker
	return textinput.Blink
}

func (m *Model) pickerRefilter() {
	q := strings.TrimSpace(m.picker.query.Value())
	m.picker.matches = m.picker.matches[:0]
	if q == "" {
		for i := range m.picker.items {
			m.picker.matches = append(m.picker.matches, i)
		}
	} else {
		src := make([]string, 0, len(m.picker.items))
		for _, it := range m.picker.items {
			src = append(src, strings.Join(it.Keywords(), " "))
		}
		for _, r := range fuzzy.Find(q, src) {
			m.picker.matches = append(m.picker.matches, r.Index)
		}
	}
	if m.picker.cursor >= len(m.picker.matches) {
		m.picker.cursor = len(m.picker.matches) - 1
	}
	if m.picker.cursor < 0 {
		m.picker.cursor = 0
	}
}

func (m *Model) pickerSelected() Pickable {
	if m.picker.cursor < 0 || m.picker.cursor >= len(m.picker.matches) {
		return nil
	}
	return m.picker.items[m.picker.matches[m.picker.cursor]]
}

func (m *Model) closePicker() {
	m.picker = pickerState{}
	m.overlay = OverlayNone
}

func (m *Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closePicker()
		return *m, nil
	case "enter":
		it := m.pickerSelected()
		onSel := m.picker.onSelect
		m.closePicker()
		if it != nil && onSel != nil {
			return *m, onSel(it)
		}
		return *m, nil
	case "up", "ctrl+p", "ctrl+k":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
		return *m, nil
	case "down", "ctrl+n", "ctrl+j":
		if m.picker.cursor < len(m.picker.matches)-1 {
			m.picker.cursor++
		}
		return *m, nil
	}
	var cmd tea.Cmd
	m.picker.query, cmd = m.picker.query.Update(msg)
	m.pickerRefilter()
	return *m, cmd
}
