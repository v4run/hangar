package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/v4run/hangar/internal/config"
)

func BuildSSHArgs(conn *config.Connection, jumpHost *config.Connection) []string {
	args := []string{"-p", strconv.Itoa(conn.Port)}

	if conn.IdentityFile != "" {
		args = append(args, "-i", conn.IdentityFile)
	}

	if jumpHost != nil {
		jumpStr := fmt.Sprintf("%s@%s:%d", jumpHost.User, jumpHost.Host, jumpHost.Port)
		args = append(args, "-J", jumpStr)
	}

	args = append(args, fmt.Sprintf("%s@%s", conn.User, conn.Host))
	return args
}

// NewSSHCommand creates an ssh exec.Cmd with askpass configured if a
// password is stored in the keychain for this connection. Returns the
// command and a cleanup function that must be called when done.
func NewSSHCommand(conn *config.Connection, jumpHost *config.Connection) (*exec.Cmd, func()) {
	args := BuildSSHArgs(conn, jumpHost)
	cmd := exec.Command("ssh", args...)

	password, err := config.GetPassword(conn.Name)
	if err != nil || password == "" {
		return cmd, func() {}
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

	cmd.Env = append(os.Environ(),
		"SSH_ASKPASS="+scriptPath,
		"SSH_ASKPASS_REQUIRE=force",
	)

	return cmd, func() { os.RemoveAll(tmpDir) }
}

func Connect(conn *config.Connection, jumpHost *config.Connection) error {
	cmd, cleanup := NewSSHCommand(conn, jumpHost)
	defer cleanup()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
