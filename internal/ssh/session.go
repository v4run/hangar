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

// SetupAskpass creates a temporary SSH_ASKPASS script that provides the
// stored password to ssh. Returns a cleanup function and the env vars
// to set on the command. Returns nil env if no password is stored.
func SetupAskpass(connName string) (env []string, cleanup func()) {
	password, err := config.GetPassword(connName)
	if err != nil || password == "" {
		return nil, func() {}
	}

	tmpDir, err := os.MkdirTemp("", "hangar-askpass-*")
	if err != nil {
		return nil, func() {}
	}

	scriptPath := filepath.Join(tmpDir, "askpass.sh")
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", password)
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		os.RemoveAll(tmpDir)
		return nil, func() {}
	}

	env = []string{
		"SSH_ASKPASS=" + scriptPath,
		"SSH_ASKPASS_REQUIRE=force",
	}

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return env, cleanup
}

// PrepareCommand creates an ssh exec.Cmd with askpass configured if a
// password is stored for the connection.
func PrepareCommand(conn *config.Connection, jumpHost *config.Connection) (cmd *exec.Cmd, cleanup func()) {
	args := BuildSSHArgs(conn, jumpHost)
	cmd = exec.Command("ssh", args...)

	askpassEnv, askpassCleanup := SetupAskpass(conn.Name)
	if askpassEnv != nil {
		cmd.Env = append(os.Environ(), askpassEnv...)
	}

	return cmd, askpassCleanup
}

func Connect(conn *config.Connection, jumpHost *config.Connection) error {
	cmd, cleanup := PrepareCommand(conn, jumpHost)
	defer cleanup()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
