package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <name> <tags...>",
		Short: "Add tags to a connection",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.AddTags(args[0], args[1:]); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Tagged %q with %v\n", args[0], args[1:])
			return nil
		},
	}
}

func newUntagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untag <name> <tags...>",
		Short: "Remove tags from a connection",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.RemoveTags(args[0], args[1:]); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Untagged %q: removed %v\n", args[0], args[1:])
			return nil
		},
	}
}
