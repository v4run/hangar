package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/v4run/hangar/v2/internal/config"
	sshauth "github.com/v4run/hangar/v2/internal/ssh"
)

// scriptPickable adapts a (connection, script) pair for the picker.
type scriptPickable struct {
	cfg  *config.Config
	conn *config.Connection
	s    config.Script
}

func (p scriptPickable) Title() string    { return p.s.Name }
func (p scriptPickable) Subtitle() string { return "$ " + p.s.Command }
func (p scriptPickable) Preview() string {
	return fmt.Sprintf("  %s\n\n  Runs via `ssh -t %s@%s 'bash -l -c <cmd>'`",
		p.s.Command, p.conn.User, p.conn.Host)
}
func (p scriptPickable) Keywords() []string {
	return []string{p.s.Name, p.s.Command}
}

// openScriptRunner opens a picker over the selected connection's
// resolved scripts (own + inherited from groups).
func (m *Model) openScriptRunner() tea.Cmd {
	c := m.selectedConnection()
	if c == nil {
		m.setToast("no connection under cursor", toastErr)
		return nil
	}
	r := config.Resolve(m.cfg, c)
	if len(r.Scripts) == 0 {
		m.setToast("no scripts on this connection", toastInfo)
		return nil
	}
	items := make([]Pickable, 0, len(r.Scripts))
	for _, s := range r.Scripts {
		items = append(items, scriptPickable{cfg: m.cfg, conn: c, s: s})
	}
	return m.openPicker("Run script", items, func(p Pickable) tea.Cmd {
		if sp, ok := p.(scriptPickable); ok {
			return m.runScriptOn(c, sp.s)
		}
		return nil
	})
}

// runScriptOn builds `ssh -t host 'bash -l -c "<cmd>; press any key"'`
// so the user sees the script output and gets to press a key before
// the TUI reclaims the screen.
func (m *Model) runScriptOn(c *config.Connection, s config.Script) tea.Cmd {
	r := config.Resolve(m.cfg, c)
	cmd, cleanup := sshauth.NewCommand(m.cfg, r)
	// Rewrite the tail of argv so we pass -t + the wrapped command,
	// mirroring how connect() does it when RemoteCommand is set.
	// Simplest: build fresh args by appending to the connection's
	// base args. We rely on NewCommand already emitting -t if the
	// resolved connection has RemoteCommand/ShellInit. For scripts
	// we override: strip anything after the user@host and append our
	// own.
	// Find the user@host arg (last positional before trailing cmd).
	destIdx := len(cmd.Args) - 1
	for i := len(cmd.Args) - 1; i > 0; i-- {
		arg := cmd.Args[i]
		if strings.Contains(arg, "@") || !strings.HasPrefix(arg, "-") {
			destIdx = i
			break
		}
	}
	escaped := strings.ReplaceAll(s.Command, "'", `'\''`)
	remote := fmt.Sprintf("bash -l -c '%s; printf \"\\npress any key to continue…\"; read -n 1'", escaped)
	cmd.Args = append(cmd.Args[:destIdx+1], "-t", cmd.Args[destIdx], remote)
	// We may have added -t twice if the base already had it; de-dup.
	cmd.Args = dedupTFlag(cmd.Args)
	m.mode = ModeConnect
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		cleanup()
		return sshExitMsg{err: err}
	})
}

func dedupTFlag(args []string) []string {
	seen := false
	out := args[:0]
	for _, a := range args {
		if a == "-t" {
			if seen {
				continue
			}
			seen = true
		}
		out = append(out, a)
	}
	return out
}
