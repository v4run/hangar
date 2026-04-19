package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var tag string
	var names []string
	var filter string

	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Run a command across multiple servers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Fleet exec not yet implemented")
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filter targets by tag")
	cmd.Flags().StringSliceVar(&names, "name", nil, "filter targets by name")
	cmd.Flags().StringVar(&filter, "filter", "", "filter output to specific server")

	return cmd
}
