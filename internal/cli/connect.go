package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
	sshpkg "github.com/v4run/hangar/internal/ssh"
	"golang.org/x/term"
)

func newConnectCmd() *cobra.Command {
	var savePassword bool

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
			return sshpkg.Connect(conn, jumpHost)
		},
	}

	cmd.Flags().BoolVar(&savePassword, "save-password", false, "save password to keychain after connecting")

	return cmd
}

func promptPassword(connName string) (string, error) {
	// Check keychain first
	if pass, err := config.GetPassword(connName); err == nil && pass != "" {
		return pass, nil
	}

	fmt.Printf("Password for %s: ", connName)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}

func offerSavePassword(connName, password string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Save password to keychain? [y/N] ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "y" || answer == "yes" {
		if err := config.SetPassword(connName, password); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save password: %v\n", err)
		} else {
			fmt.Println("Password saved to keychain.")
		}
	}
}
