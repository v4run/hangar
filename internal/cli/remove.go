package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.Remove(args[0]); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Removed connection %q\n", args[0])
			return nil
		},
	}
}
