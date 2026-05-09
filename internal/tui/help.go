package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("hangar — keybindings"))
	b.WriteString("    " + dimStyle.Render("? or esc to close"))
	b.WriteString("\n\n")

	sections := []struct {
		name  string
		binds [][2]string
	}{
		{"navigation", [][2]string{
			{"j / ↓", "move down"},
			{"k / ↑", "move up"},
			{"space", "toggle group collapse"},
			{"/", "filter connections"},
			{"l", "focus scripts pane"},
			{"h / esc", "back to sidebar"},
			{"enter", "connect to selected host"},
		}},
		{"editing", [][2]string{
			{"n", "new connection"},
			{"e", "edit connection / group"},
			{"d", "delete (with confirm)"},
			{"t", "tag management"},
			{"o", "edit notes (in scripts pane)"},
			{"g", "create new group"},
			{"G", "global SSH settings"},
		}},
		{"clipboard", [][2]string{
			{"x", "cut (mark for move)"},
			{"y", "copy (mark for duplicate)"},
			{"p", "paste into target group"},
			{"v", "visual-select mode"},
		}},
		{"scripts", [][2]string{
			{"l", "focus scripts pane"},
			{"n / e / d", "new / edit / delete script"},
			{"enter", "run script on connection"},
		}},
		{"other", [][2]string{
			{"s / S", "sync from ~/.ssh/config"},
			{"J / K", "reorder connection or group"},
			{"q / ctrl+c", "quit"},
		}},
	}

	width := m.width - 29 // main pane width
	if width < 40 {
		width = 40
	}

	for _, sec := range sections {
		b.WriteString(sectionDivider(sec.name, width) + "\n")
		for _, bind := range sec.binds {
			key := cursorStyle.Render(fmt.Sprintf("  %-14s", bind[0]))
			b.WriteString(key + normalStyle.Render(bind[1]) + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
