// Package ssh builds ssh exec.Cmds against a resolved Connection.
// Password injection goes through SSH_ASKPASS; host-key confirmation
// happens inside the TUI (see cmd/hangar + knownhosts.go) BEFORE this
// command runs, so ssh never prompts.
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/v4run/hangar/v2/internal/config"
)

// BuildArgs returns the argv ssh should run with for a fully-resolved
// connection. shellInit, if non-empty, is injected into a bash -lc
// wrapper on the remote side (together with a user-supplied
// RemoteCommand when present).
func BuildArgs(cfg *config.Config, r *config.Resolved) []string {
	c := r.Connection
	args := []string{}
	// Safety rails: fail fast on a bad config instead of hanging the
	// TUI in a retry loop. Both are overridable by the user via
	// SSHOptions.ExtraOptions.
	if !optionSet(r, "ConnectTimeout") {
		args = append(args, "-o", "ConnectTimeout=10")
	}
	if !optionSet(r, "NumberOfPasswordPrompts") {
		args = append(args, "-o", "NumberOfPasswordPrompts=1")
	}
	if !optionSet(r, "ConnectionAttempts") {
		args = append(args, "-o", "ConnectionAttempts=1")
	}
	if r.SSHOptions != nil {
		o := r.SSHOptions
		if o.ForwardAgent != nil {
			args = append(args, "-o", "ForwardAgent="+boolYN(*o.ForwardAgent))
		}
		if o.Compression != nil {
			args = append(args, "-o", "Compression="+boolYN(*o.Compression))
		}
		if o.ServerAliveInterval != nil {
			args = append(args, "-o", fmt.Sprintf("ServerAliveInterval=%d", *o.ServerAliveInterval))
		}
		if o.ServerAliveCountMax != nil {
			args = append(args, "-o", fmt.Sprintf("ServerAliveCountMax=%d", *o.ServerAliveCountMax))
		}
		if o.StrictHostKeyCheck != "" {
			args = append(args, "-o", "StrictHostKeyChecking="+o.StrictHostKeyCheck)
		}
		if o.RequestTTY != "" {
			args = append(args, "-o", "RequestTTY="+o.RequestTTY)
		}
		for _, lf := range o.LocalForward {
			args = append(args, "-L", lf)
		}
		for _, rf := range o.RemoteForward {
			args = append(args, "-R", rf)
		}
		for k, v := range o.ExtraOptions {
			args = append(args, "-o", k+"="+v)
		}
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.Port != 0 && c.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", c.Port))
	}
	if c.JumpHost != uuid.Nil {
		if j := findConn(cfg, c.JumpHost); j != nil {
			args = append(args, "-J", formatDest(j))
		}
	}
	remote := strings.TrimSpace(c.RemoteCommand)
	if r.ShellInit != "" || remote != "" {
		// -t forces a pty so an interactive shell wrapper works.
		args = append(args, "-t", formatDest(c))
		args = append(args, buildRemoteWrapper(r.ShellInit, remote))
	} else {
		args = append(args, formatDest(c))
	}
	return args
}

// NewCommand builds the exec.Cmd that opens an interactive ssh
// session. The returned cleanup function removes any temporary
// askpass script; always defer it.
func NewCommand(cfg *config.Config, r *config.Resolved) (*exec.Cmd, func()) {
	args := BuildArgs(cfg, r)
	cmd := exec.Command("ssh", args...)
	if len(r.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range r.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	cleanup := func() {}
	if pw := resolvePassword(r.Connection); pw != "" {
		scriptDir, err := os.MkdirTemp("", "hangar-askpass-*")
		if err == nil {
			scriptPath := filepath.Join(scriptDir, "askpass.sh")
			body := fmt.Sprintf("#!/bin/sh\necho '%s'\n", escapeSingle(pw))
			if err := os.WriteFile(scriptPath, []byte(body), 0o700); err == nil {
				if cmd.Env == nil {
					cmd.Env = os.Environ()
				}
				cmd.Env = append(cmd.Env,
					"SSH_ASKPASS="+scriptPath,
					"SSH_ASKPASS_REQUIRE=force",
				)
				cleanup = func() { _ = os.RemoveAll(scriptDir) }
			} else {
				_ = os.RemoveAll(scriptDir)
			}
		}
	}
	return cmd, cleanup
}

func formatDest(c *config.Connection) string {
	if c.User != "" {
		return c.User + "@" + c.Host
	}
	return c.Host
}

func buildRemoteWrapper(shellInit, remote string) string {
	var body string
	switch {
	case shellInit != "" && remote != "":
		body = shellInit + "\n" + remote
	case shellInit != "":
		body = shellInit + "\nexec \"$SHELL\" -l"
	default:
		body = remote
	}
	return "bash -l -c '" + escapeSingle(body) + "'"
}

func escapeSingle(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// optionSet reports whether the resolved SSHOptions already carries
// `key` either as a dedicated field or in ExtraOptions — used to
// avoid stepping on a user-set value when we inject safety rails.
func optionSet(r *config.Resolved, key string) bool {
	if r == nil || r.SSHOptions == nil {
		return false
	}
	switch key {
	case "ConnectTimeout", "NumberOfPasswordPrompts", "ConnectionAttempts":
		_, ok := r.SSHOptions.ExtraOptions[key]
		return ok
	}
	return false
}

func boolYN(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func findConn(cfg *config.Config, id uuid.UUID) *config.Connection {
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == id {
			return &cfg.Connections[i]
		}
	}
	return nil
}
