package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap collects every binding v2 cares about. Implementing
// help.KeyMap keeps the status footer + ? overlay in sync with real
// behaviour — a single source of truth for `space` leader, `?` grid
// and footer hints.
type KeyMap struct {
	// Movement
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Top     key.Binding
	Bottom  key.Binding
	PageUp  key.Binding
	PageDn  key.Binding

	// Primary actions
	Enter   key.Binding
	Tab     key.Binding
	ShiftTab key.Binding

	// Mode entries
	Filter  key.Binding // /
	Cmd     key.Binding // :
	Leader  key.Binding // space
	Help    key.Binding // ?

	// Row verbs (invoked from leader or directly)
	Connect     key.Binding
	Edit        key.Binding
	Add         key.Binding
	Delete      key.Binding
	Duplicate   key.Binding
	NewGroup    key.Binding
	ImportSSH   key.Binding
	ToggleExpand key.Binding // h / l / space on a group row

	// Reordering
	MoveUp   key.Binding
	MoveDown key.Binding

	// Global
	Quit key.Binding
	Esc  key.Binding
}

// ShortHelp returns footer-strip bindings (context-insensitive subset).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Edit, k.Add, k.Delete, k.Filter, k.Cmd, k.Leader, k.Help, k.Quit}
}

// FullHelp returns the `?` grid, grouped.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Top, k.Bottom, k.PageUp, k.PageDn},
		{k.Enter, k.Tab, k.ShiftTab, k.ToggleExpand},
		{k.Connect, k.Edit, k.Add, k.NewGroup, k.Duplicate, k.Delete, k.ImportSSH, k.MoveUp, k.MoveDown},
		{k.Filter, k.Cmd, k.Leader, k.Help, k.Quit},
	}
}

// DefaultKeymap returns the built-in bindings.
func DefaultKeymap() KeyMap {
	b := func(keys []string, help, desc string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
	}
	return KeyMap{
		Up:        b([]string{"k", "up"}, "k/↑", "up"),
		Down:      b([]string{"j", "down"}, "j/↓", "down"),
		Left:      b([]string{"h", "left"}, "h/←", "collapse / parent"),
		Right:     b([]string{"l", "right"}, "l/→", "expand / enter"),
		Top:       b([]string{"g"}, "gg", "top"),
		Bottom:    b([]string{"G"}, "G", "bottom"),
		PageUp:    b([]string{"pgup"}, "pgup", "scroll detail up"),
		PageDn:    b([]string{"pgdown"}, "pgdn", "scroll detail down"),
		Enter:     b([]string{"enter"}, "enter", "connect / confirm"),
		Tab:       b([]string{"tab"}, "tab", "next tab / field"),
		ShiftTab:  b([]string{"shift+tab"}, "⇧tab", "prev tab / field"),
		Filter:    b([]string{"/"}, "/", "filter"),
		Cmd:       b([]string{":"}, ":", "command"),
		Leader:    b([]string{" "}, "space", "leader"),
		Help:      b([]string{"?"}, "?", "help"),
		Connect:   b([]string{"c"}, "c", "connect"),
		Edit:      b([]string{"e"}, "e", "edit"),
		Add:       b([]string{"a"}, "a", "add"),
		Delete:    b([]string{"d"}, "d", "delete"),
		Duplicate: b([]string{"y"}, "y", "duplicate"),
		NewGroup:  b([]string{"g"}, "g", "new group"),
		ImportSSH: b([]string{"i"}, "i", "import ssh cmd"),
		ToggleExpand: b([]string{" "}, "space", "toggle expand"),
		MoveUp:   b([]string{"K"}, "K", "move up"),
		MoveDown: b([]string{"J"}, "J", "move down"),
		Quit:     b([]string{"q", "ctrl+c"}, "q", "quit"),
		Esc:      b([]string{"esc"}, "esc", "cancel"),
	}
}
