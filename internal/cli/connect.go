package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
	sshpkg "github.com/v4run/hangar/internal/ssh"
)

func newConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
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

			// Resolve jump host
			var jumpHost *config.Connection
			if conn.JumpHost != "" {
				jh, err := cfg.FindByName(conn.JumpHost)
				if err != nil {
					return fmt.Errorf("jump host %q: %w", conn.JumpHost, err)
				}
				jumpHost = jh
			}

			// Use the ssh package to build args and connect
			return sshpkg.Connect(conn, jumpHost, nil)
		},
	}

	return cmd
}
