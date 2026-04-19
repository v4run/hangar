package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
	sshauth "github.com/v4run/hangar/internal/ssh"
	"github.com/v4run/hangar/internal/tui"
)

// cfgDir is set by the --config persistent flag before any command runs.
// It is safe because Cobra processes flags before invoking RunE.
var cfgDir string

func configDir() string {
	if cfgDir != "" {
		return cfgDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hangar")
}

func loadConfig() (*config.HangarConfig, error) {
	return config.Load(configDir())
}

func saveConfig(cfg *config.HangarConfig) error {
	return config.Save(configDir(), cfg)
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hangar",
		Short: "Terminal SSH manager",
		Long:  "Hangar is a terminal SSH connection manager with a TUI dashboard.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			gc, err := config.LoadGlobal(configDir())
			if err != nil {
				return err
			}

			// Check if SSH config changed
			sshChanged := false
			if gc.AutoSync {
				sshPath := sshauth.ExpandHome(gc.SSHConfigPath)
				changed, err := cfg.NeedsSync(sshPath)
				if err == nil {
					sshChanged = changed
				}
			}

			return tui.Run(cfg, gc, configDir(), sshChanged)
		},
	}

	root.PersistentFlags().StringVar(&cfgDir, "config", "", "config directory (default ~/.hangar)")

	root.AddCommand(newListCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newRemoveCmd())
	root.AddCommand(newConnectCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newTagCmd())
	root.AddCommand(newUntagCmd())

	return root
}
