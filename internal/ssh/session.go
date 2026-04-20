package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

// ResolveJumpHost resolves a JumpHost value. If it's a valid UUID, looks up
// the connection by ID. Otherwise tries by name. Returns nil if unresolvable
// (caller should check if raw JumpHost value should be used as-is).
func ResolveJumpHost(cfg *config.HangarConfig, jumpHostVal string) *config.Connection {
	if jumpHostVal == "" {
		return nil
	}
	if id, err := uuid.Parse(jumpHostVal); err == nil {
		if jh, err := cfg.FindByID(id); err == nil {
			return jh
		}
	}
	if jh, err := cfg.FindByName(jumpHostVal); err == nil {
		return jh
	}
	return nil
}

func BuildSSHArgs(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions) []string {
	args := []string{"-p", strconv.Itoa(conn.Port)}

	if conn.IdentityFile != "" {
		args = append(args, "-i", conn.IdentityFile)
	}

	if jumpHost != nil {
		jumpStr := fmt.Sprintf("%s@%s:%d", jumpHost.User, jumpHost.Host, jumpHost.Port)
		args = append(args, "-J", jumpStr)
	} else if conn.JumpHost != "" {
		// Raw JumpHost value (not a resolvable UUID or name)
		if _, err := uuid.Parse(conn.JumpHost); err != nil {
			args = append(args, "-J", conn.JumpHost)
		}
	}

	// Apply SSH options
	if opts != nil {
		if opts.ForwardAgent != nil {
			args = append(args, "-o", fmt.Sprintf("ForwardAgent=%s", boolToYesNo(*opts.ForwardAgent)))
		}
		if opts.Compression != nil {
			args = append(args, "-o", fmt.Sprintf("Compression=%s", boolToYesNo(*opts.Compression)))
		}
		if opts.ServerAliveInterval != nil {
			args = append(args, "-o", fmt.Sprintf("ServerAliveInterval=%d", *opts.ServerAliveInterval))
		}
		if opts.ServerAliveCountMax != nil {
			args = append(args, "-o", fmt.Sprintf("ServerAliveCountMax=%d", *opts.ServerAliveCountMax))
		}
		if opts.StrictHostKeyCheck != "" {
			args = append(args, "-o", fmt.Sprintf("StrictHostKeyChecking=%s", opts.StrictHostKeyCheck))
		}
		if opts.RequestTTY != "" {
			args = append(args, "-o", fmt.Sprintf("RequestTTY=%s", opts.RequestTTY))
		}
		for _, lf := range opts.LocalForward {
			args = append(args, "-L", lf)
		}
		for _, rf := range opts.RemoteForward {
			args = append(args, "-R", rf)
		}
		for key := range opts.EnvVars {
			args = append(args, "-o", fmt.Sprintf("SendEnv=%s", key))
		}
		for key, val := range opts.ExtraOptions {
			args = append(args, "-o", fmt.Sprintf("%s=%s", key, val))
		}
	}

	args = append(args, fmt.Sprintf("%s@%s", conn.User, conn.Host))
	return args
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// NewSSHCommand creates an ssh exec.Cmd with askpass configured if a
// password is stored in the keychain for this connection. Returns the
// command and a cleanup function that must be called when done.
func NewSSHCommand(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions) (*exec.Cmd, func()) {
	args := BuildSSHArgs(conn, jumpHost, opts)
	cmd := exec.Command("ssh", args...)

	// Set environment variables from SSHOptions
	if opts != nil && len(opts.EnvVars) > 0 {
		cmd.Env = append(os.Environ())
		for key, val := range opts.EnvVars {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	password, err := config.GetPassword(conn.ID.String())
	if err != nil || password == "" {
		// Fallback: try legacy name-based lookup
		password, err = config.GetPassword(conn.Name)
		if err != nil || password == "" {
			return cmd, func() {}
		}
	}

	tmpDir, err := os.MkdirTemp("", "hangar-askpass-*")
	if err != nil {
		return cmd, func() {}
	}

	scriptPath := filepath.Join(tmpDir, "askpass.sh")
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", password)
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		os.RemoveAll(tmpDir)
		return cmd, func() {}
	}

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		"SSH_ASKPASS="+scriptPath,
		"SSH_ASKPASS_REQUIRE=force",
	)

	return cmd, func() { os.RemoveAll(tmpDir) }
}

func Connect(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions) error {
	cmd, cleanup := NewSSHCommand(conn, jumpHost, opts)
	defer cleanup()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
