package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
	sshauth "github.com/v4run/hangar/internal/ssh"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync connections from ~/.ssh/config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			gc, err := config.LoadGlobal(configDir())
			if err != nil {
				return err
			}

			sshPath := sshauth.ExpandHome(gc.SSHConfigPath)

			added, updated, err := cfg.SyncFromSSHConfig(sshPath)
			if err != nil {
				return err
			}

			if err := saveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Synced from %s: %d added, %d updated\n", sshPath, added, updated)
			return nil
		},
	}
}
