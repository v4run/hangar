package cli

import (
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
			gc, err := config.LoadGlobal(configDir())
			if err != nil {
				return err
			}
			conn, err := cfg.FindByName(args[0])
			if err != nil {
				return err
			}

			jumpHost := sshpkg.ResolveJumpHost(cfg, conn.JumpHost)

			// Merge SSH options
			var merged *config.SSHOptions
			useGlobal := conn.UseGlobalSettings == nil || *conn.UseGlobalSettings
			if useGlobal && gc.SSHOptions != nil {
				m := config.MergeSSHOptions(gc.SSHOptions, conn.SSHOptions)
				merged = &m
			} else if conn.SSHOptions != nil {
				merged = conn.SSHOptions
			}

			return sshpkg.Connect(conn, jumpHost, merged)
		},
	}

	return cmd
}
