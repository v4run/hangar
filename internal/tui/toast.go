package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	toastOK   = 2  // green
	toastErr  = 1  // red
	toastWarn = 3  // yellow
	toastInfo = 13 // magenta
)

type toast struct {
	text string
	kind int
}

type clearToastMsg struct{}

func showToast(text string, kind int) (toast, tea.Cmd) {
	t := toast{text: text, kind: kind}
	cmd := tea.Tick(2500*time.Millisecond, func(time.Time) tea.Msg {
		return clearToastMsg{}
	})
	return t, cmd
}
