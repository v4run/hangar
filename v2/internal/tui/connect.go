package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

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

// beginConnect walks the jump chain (jumps-first, then destination)
// and if ANY hop isn't already in ~/.ssh/known_hosts, opens the
// host-key palette for the first unknown hop. The confirmHostKey
// handler recurses after trust so a chain of unknown hops gets
// one palette each, in order. Only when every hop is trusted do we
// actually launch ssh — otherwise ssh's own prompt gets intercepted
// by our askpass helper (SSH_ASKPASS_REQUIRE=force) and the connect
// hangs or dies.
func (m *Model) beginConnect(c *config.Connection) tea.Cmd {
	m.state.Touch(c.ID)
	_ = config.SaveState(m.cfgDir, m.state)

	hop := firstUnknownHop(m.cfg, c)
	if hop != nil {
		return m.openHostKeyPaletteFor(c, hop)
	}
	resolved := config.Resolve(m.cfg, c)
	return m.launchSSH(resolved)
}

// firstUnknownHop walks dst.JumpHost → j.JumpHost → … → dst and
// returns the first connection in that chain whose Host isn't in
// known_hosts. Returns nil if every hop is trusted.
func firstUnknownHop(cfg *config.Config, dst *config.Connection) *config.Connection {
	// Build the hop list jumps-first.
	var chain []*config.Connection
	seen := map[uuid.UUID]bool{}
	var walk func(c *config.Connection)
	walk = func(c *config.Connection) {
		if c == nil || seen[c.ID] {
			return
		}
		seen[c.ID] = true
		if c.JumpHost != uuid.Nil {
			if j := findConnByID(cfg, c.JumpHost); j != nil {
				walk(j)
			}
		}
		chain = append(chain, c)
	}
	walk(dst)
	for _, c := range chain {
		if !sshauth.IsKnownHost(c.Host, portOr22(c.Port)) {
			return c
		}
	}
	return nil
}

// openHostKeyPaletteFor opens the first-time-trust modal for `hop`,
// stashing both the final destination (so we remember what we were
// trying to reach) and the hop whose key we're actually fetching.
func (m *Model) openHostKeyPaletteFor(dst, hop *config.Connection) tea.Cmd {
	m.pendingConn = dst
	m.pendingHop = hop
	title := "Unknown host"
	preamble := fmt.Sprintf("First-time connection to %s@%s:%d.", userOrDash(hop.User), hop.Host, portOr22(hop.Port))
	if hop.ID != dst.ID {
		preamble = fmt.Sprintf("First-time jump through %s@%s:%d (on the way to %s).",
			userOrDash(hop.User), hop.Host, portOr22(hop.Port), dst.Name)
	}
	m.palette = paletteState{
		kind:    PaletteHostKey,
		title:   title,
		message: preamble + "\nFetching host key…",
		target:  dst.ID,
	}
	m.pendingKeys = nil
	m.overlay = OverlayPalette
	m.mode = ModeConnect
	host, port := hop.Host, portOr22(hop.Port)
	m.spinning = true
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			keys, err := sshauth.ScanHost(host, port)
			return hostKeyScannedMsg{keys: keys, err: err}
		},
	)
}

func (m *Model) confirmHostKey() (tea.Model, tea.Cmd) {
	if err := sshauth.TrustHost(m.pendingKeys); err != nil {
		m.setToast("trust: "+err.Error(), toastErr)
		m.closePalette()
		m.pendingConn = nil
		m.pendingHop = nil
		m.pendingKeys = nil
		return *m, nil
	}
	dst := m.pendingConn
	m.closePalette()
	m.pendingConn = nil
	m.pendingHop = nil
	m.pendingKeys = nil
	if dst == nil {
		return *m, nil
	}
	// Re-run beginConnect — it finds the next untrusted hop (if
	// any) and either opens another palette or launches ssh.
	return *m, m.beginConnect(dst)
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
		return *m, nil
	}
	return *m, m.beginConnect(c)
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
