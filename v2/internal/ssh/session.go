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
	"sort"
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
		// Sort ExtraOptions keys so successive renders / executions
		// produce the same argv order (map iteration is random in
		// Go). The sort is cheap and makes the detail pane's SSH
		// command stable, which otherwise shuffles each refresh.
		keys := make([]string, 0, len(o.ExtraOptions))
		for k := range o.ExtraOptions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, "-o", k+"="+o.ExtraOptions[k])
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
		envKeys := make([]string, 0, len(r.Env))
		for k := range r.Env {
			envKeys = append(envKeys, k)
		}
		sort.Strings(envKeys)
		for _, k := range envKeys {
			cmd.Env = append(cmd.Env, k+"="+r.Env[k])
		}
	}
	cleanup := func() {}
	// Collect password entries for the destination AND every jump
	// host in the chain, so ssh asking for either gets the right
	// one. The askpass script that ssh invokes receives the prompt
	// text as $1 — we grep for the hostname in that text.
	entries := collectPasswordEntries(cfg, r.Connection)
	if len(entries) > 0 {
		scriptDir, err := os.MkdirTemp("", "hangar-askpass-*")
		if err == nil {
			scriptPath := filepath.Join(scriptDir, "askpass.sh")
			body := buildAskpassScript(entries)
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

// passwordEntry pairs a host (as it appears in ssh's prompt) with
// the password to answer with.
type passwordEntry struct {
	Host     string
	Password string
}

// collectPasswordEntries walks the destination + jump chain and
// returns a password per hop that has one. Destination comes last so
// its password is the fallback when the prompt doesn't match any
// known host.
func collectPasswordEntries(cfg *config.Config, dst *config.Connection) []passwordEntry {
	seen := map[uuid.UUID]bool{}
	var out []passwordEntry
	// Walk jumps first.
	for cur := dst.JumpHost; cur != uuid.Nil && !seen[cur]; {
		seen[cur] = true
		j := findConn(cfg, cur)
		if j == nil {
			break
		}
		if pw := resolvePassword(j); pw != "" {
			out = append(out, passwordEntry{Host: j.Host, Password: pw})
		}
		cur = j.JumpHost
	}
	if pw := resolvePassword(dst); pw != "" {
		out = append(out, passwordEntry{Host: dst.Host, Password: pw})
	}
	return out
}

// buildAskpassScript writes a /bin/sh script that reads the prompt
// text from $1 (ssh calls askpass with the full prompt as argv[1]),
// grep-matches it against each entry's host, and echoes the first
// matching password. The last entry acts as fallback so an
// unmatchable prompt still answers with the destination password.
func buildAskpassScript(entries []passwordEntry) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("case \"$1\" in\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  *%s*) printf '%%s\\n' '%s' ; exit 0 ;;\n",
			escapeSingle(e.Host), escapeSingle(e.Password))
	}
	// Fallback — the LAST entry (which is the destination; see
	// collectPasswordEntries) answers anything we couldn't match.
	fmt.Fprintf(&b, "  *) printf '%%s\\n' '%s' ;;\n",
		escapeSingle(entries[len(entries)-1].Password))
	b.WriteString("esac\n")
	return b.String()
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
