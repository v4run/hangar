package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect to a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			conn, err := cfg.FindByName(args[0])
			if err != nil {
				return err
			}

			sshArgs := []string{
				"-p", strconv.Itoa(conn.Port),
			}
			if conn.IdentityFile != "" {
				sshArgs = append(sshArgs, "-i", conn.IdentityFile)
			}
			if conn.JumpHost != "" {
				jump, err := cfg.FindByName(conn.JumpHost)
				if err != nil {
					return fmt.Errorf("jump host %q: %w", conn.JumpHost, err)
				}
				jumpStr := fmt.Sprintf("%s@%s:%d", jump.User, jump.Host, jump.Port)
				sshArgs = append(sshArgs, "-J", jumpStr)
			}
			sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", conn.User, conn.Host))

			sshCmd := exec.Command("ssh", sshArgs...)
			sshCmd.Stdin = os.Stdin
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr
			return sshCmd.Run()
		},
	}
}
