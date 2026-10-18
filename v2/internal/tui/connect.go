package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/v4run/hangar/v2/internal/config"
	sshauth "github.com/v4run/hangar/v2/internal/ssh"
)

// hostKeyScannedMsg carries the result of a background ssh-keyscan.
type hostKeyScannedMsg struct {
	keys []sshauth.HostKey
	err  error
}

// sshExitMsg fires after the ssh child exits.
type sshExitMsg struct{ err error }

// beginConnect checks ~/.ssh/known_hosts; if the host isn't trusted,
// opens the host-key palette and triggers a background scan.
// Otherwise launches ssh immediately.
func (m *Model) beginConnect(c *config.Connection) tea.Cmd {
	resolved := config.Resolve(m.cfg, c)
	if !sshauth.IsKnownHost(resolved.Host, portOr22(resolved.Port)) {
		m.pendingConn = c
		m.palette = paletteState{
			kind:    PaletteHostKey,
			title:   "Unknown host",
			message: fmt.Sprintf("First-time connection to %s@%s:%d.\nFetching host key…", userOrDash(resolved.User), resolved.Host, portOr22(resolved.Port)),
			target:  c.ID,
		}
		m.pendingKeys = nil
		m.overlay = OverlayPalette
		m.mode = ModeConnect
		host, port := resolved.Host, portOr22(resolved.Port)
		return func() tea.Msg {
			keys, err := sshauth.ScanHost(host, port)
			return hostKeyScannedMsg{keys: keys, err: err}
		}
	}
	return m.launchSSH(resolved)
}

func (m *Model) confirmHostKey() (tea.Model, tea.Cmd) {
	if err := sshauth.TrustHost(m.pendingKeys); err != nil {
		m.setToast("trust: "+err.Error(), toastErr)
		m.closePalette()
		m.pendingConn = nil
		m.pendingKeys = nil
		return m, nil
	}
	c := m.pendingConn
	m.closePalette()
	m.pendingConn = nil
	m.pendingKeys = nil
	if c == nil {
		return m, nil
	}
	resolved := config.Resolve(m.cfg, c)
	return m, m.launchSSH(resolved)
}

func (m *Model) launchSSH(r *config.Resolved) tea.Cmd {
	cmd, cleanup := sshauth.NewCommand(m.cfg, r)
	m.mode = ModeConnect
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		cleanup()
		return sshExitMsg{err: err}
	})
}

// confirmConnect is used by the (optional) confirm-before-connect
// palette. Currently unused; placeholder for Phase 2's connect
// confirm hook if we add one.
func (m *Model) confirmConnect() (tea.Model, tea.Cmd) {
	c := m.pendingConn
	m.closePalette()
	m.pendingConn = nil
	if c == nil {
		return m, nil
	}
	return m, m.beginConnect(c)
}

func portOr22(p int) int {
	if p == 0 {
		return 22
	}
	return p
}

func userOrDash(u string) string {
	if u == "" {
		return "—"
	}
	return u
}
