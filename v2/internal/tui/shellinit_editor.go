package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// shellInitEditorDoneMsg carries the result of a $EDITOR shell-init
// session back to the Update loop.
type shellInitEditorDoneMsg struct {
	scope shellInitScope
	id    uuid.UUID
	text  string
	err   error
}

// openShellInitInEditor shells out to $EDITOR with the current
// shell-init text in a tempfile. On exit the file is read back and
// delivered as a shellInitEditorDoneMsg.
func (m *Model) openShellInitInEditor(scope shellInitScope, id uuid.UUID, initial string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	dir, err := os.MkdirTemp("", "hangar-shellinit-*")
	if err != nil {
		m.setToast("tmp: "+err.Error(), toastErr)
		return m.clearToastCmd()
	}
	path := filepath.Join(dir, "shell-init.sh")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		m.setToast("write: "+err.Error(), toastErr)
		return m.clearToastCmd()
	}
	cmd := exec.Command(editor, path)
	m.mode = ModeEdit
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		defer os.RemoveAll(dir)
		if execErr != nil {
			return shellInitEditorDoneMsg{scope: scope, id: id, err: execErr}
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return shellInitEditorDoneMsg{scope: scope, id: id, err: readErr}
		}
		return shellInitEditorDoneMsg{scope: scope, id: id, text: string(data)}
	})
}

// applyShellInitEditorResult writes the edited text to the right
// scope on the config.
func (m *Model) applyShellInitEditorResult(msg shellInitEditorDoneMsg) {
	switch msg.scope {
	case shellInitScopeGlobal:
		m.cfg.ShellInit = msg.text
	case shellInitScopeGroup:
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].ID == msg.id {
				m.cfg.Groups[i].ShellInit = msg.text
				break
			}
		}
	default: // shellInitScopeConnection
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].ID == msg.id {
				m.cfg.Connections[i].ShellInit = msg.text
				break
			}
		}
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.setToast("save: "+err.Error(), toastErr)
		return
	}
	m.setToast("shell init saved", toastOK)
}

// rowShellInitScope figures out which scope + id the shell-init
// shortcuts should operate on, based on the sidebar cursor row.
func (m *Model) rowShellInitScope() (shellInitScope, uuid.UUID, string, bool) {
	if c := m.selectedConnection(); c != nil {
		return shellInitScopeConnection, c.ID, c.ShellInit, true
	}
	if g := m.selectedGroup(); g != nil {
		return shellInitScopeGroup, g.ID, g.ShellInit, true
	}
	return 0, uuid.Nil, "", false
}

var _ = fmt.Sprintf // reserved for future diagnostic logging