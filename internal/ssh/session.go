package ssh

import (
	"fmt"
	"os"
	"os/exec"
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

func Connect(conn *config.Connection, jumpHost *config.Connection) error {
	args := BuildSSHArgs(conn, jumpHost)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
