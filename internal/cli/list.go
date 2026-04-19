package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			conns := cfg.Connections
			if tag != "" {
				conns = cfg.FilterByTag(tag)
			}

			if len(conns) == 0 {
				fmt.Println("No connections found.")
				return nil
			}

			for _, c := range conns {
				tags := ""
				if len(c.Tags) > 0 {
					tags = " [" + strings.Join(c.Tags, ", ") + "]"
				}
				fmt.Printf("  %s — %s@%s:%d%s\n", c.Name, c.User, c.Host, c.Port, tags)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	return cmd
}
