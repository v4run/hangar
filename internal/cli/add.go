package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
)

func newAddCmd() *cobra.Command {
	var host, user, identityFile, jumpHost, password string
	var port int
	var tags []string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			conn := config.Connection{
				Name:         args[0],
				Host:         host,
				Port:         port,
				User:         user,
				IdentityFile: identityFile,
				JumpHost:     jumpHost,
				Tags:         tags,
			}

			if err := cfg.Add(conn); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}

			if password != "" {
				config.SetPassword(args[0], password)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added connection %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "hostname or IP (required)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&user, "user", "", "username (required)")
	cmd.Flags().StringVar(&identityFile, "identity-file", "", "path to SSH key")
	cmd.Flags().StringVar(&jumpHost, "jump-host", "", "jump host connection name")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tags for this connection")
	cmd.Flags().StringVar(&password, "password", "", "password (stored in system keychain)")
	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("user")

	return cmd
}
