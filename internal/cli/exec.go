package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/fleet"
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
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			targets := fleet.ResolveTargets(cfg, tag, names)
			if len(targets) == 0 {
				return fmt.Errorf("no targets matched")
			}

			command := strings.Join(args, " ")
			serverNames := make([]string, len(targets))
			for i, t := range targets {
				serverNames[i] = t.Name
			}
			colors := fleet.AssignColors(serverNames)
			nameWidth := fleet.MaxNameWidth(serverNames)

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "hangar exec: %s  (%d servers)\n", command, len(targets))

			output := make(chan fleet.Result, 100)
			go fleet.Execute(targets, command, output, cfg)

			for result := range output {
				if filter != "" && result.Server != filter {
					continue
				}

				showBorder := filter == ""
				if result.Err != nil {
					color := colors[result.Server]
					fmt.Fprintln(out, fleet.FormatLine(result.Server, color, fmt.Sprintf("ERROR: %v", result.Err), showBorder, nameWidth))
				} else {
					color := colors[result.Server]
					fmt.Fprintln(out, fleet.FormatLine(result.Server, color, result.Line, showBorder, nameWidth))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filter targets by tag")
	cmd.Flags().StringSliceVar(&names, "name", nil, "filter targets by name")
	cmd.Flags().StringVar(&filter, "filter", "", "filter output to specific server")

	return cmd
}
