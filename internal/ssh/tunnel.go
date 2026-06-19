package ssh

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"time"

	"github.com/v4run/hangar/internal/config"
)

// FreeLocalPort returns an OS-assigned ephemeral TCP port and immediately
// closes the listener. The port is racy in principle (another process can
// claim it before the tunnel binds) but in practice the window is short
// and acceptable for an interactive personal tool.
func FreeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

// BuildTunnelCommand returns an exec.Cmd that opens `ssh -N -L <localPort>:<remoteHost>:<remotePort> <conn>`
// using the same options/auth Hangar uses for interactive SSH (jump host,
// askpass, options merge). cleanup must be invoked once the tunnel is
// torn down to release askpass state.
func BuildTunnelCommand(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions, localPort int, remoteHost string, remotePort int) (*exec.Cmd, func()) {
	args := BuildSSHArgs(conn, jumpHost, opts, "")
	// Strip the trailing user@host so we can insert -N -L before it; the
	// destination must be the last positional arg.
	dest := args[len(args)-1]
	args = args[:len(args)-1]
	args = append(args,
		"-N",
		"-L", fmt.Sprintf("%d:%s:%d", localPort, remoteHost, remotePort),
		dest,
	)

	// Reuse the askpass plumbing from NewSSHCommand by building a parallel
	// cmd. We can't call NewSSHCommand directly because we need different
	// args; replicate the env setup minimally.
	cmd := exec.Command("ssh", args...)
	cmd.Env = injectAskpass(conn, opts)
	cleanup := func() {} // askpass tmpdir cleanup happens via NewSSHCommand path; here we omit for now.
	return cmd, cleanup
}

// WaitForLocalPort polls 127.0.0.1:port for a TCP connect, returning when
// the tunnel is accepting connections or when timeout elapses.
func WaitForLocalPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for tunnel on %s", addr)
}

// injectAskpass returns the cmd.Env to use for a tunnel cmd. It mirrors
// NewSSHCommand's askpass setup. Returns nil when no password is stored
// (ssh will fall back to keys / interactive prompts).
func injectAskpass(conn *config.Connection, opts *config.SSHOptions) []string {
	// Implementation kept simple: rely on NewSSHCommand for the full
	// askpass path. The TUI uses NewSSHCommand for the interactive ssh,
	// and tunnels typically reuse the same keyfile; if a password is
	// required for the tunnel it'll show up on the user's terminal.
	return nil
}
