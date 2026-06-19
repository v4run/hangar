package cli

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
	dbpkg "github.com/v4run/hangar/internal/db"
	sshpkg "github.com/v4run/hangar/internal/ssh"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db <name>",
		Short: "Open a saved database profile",
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
			d, err := cfg.FindDatabaseByName(args[0])
			if err != nil {
				return err
			}

			targetHost, targetPort := d.Host, d.Port
			var tunnelCmd *exec.Cmd
			if d.SSHTunnel != "" && dbpkg.RequiresTunnelTarget(d.Engine) {
				sshConn, err := resolveSSHTunnel(cfg, d.SSHTunnel)
				if err != nil {
					return err
				}
				port, err := sshpkg.FreeLocalPort()
				if err != nil {
					return err
				}
				jump := sshpkg.ResolveJumpHost(cfg, sshConn.JumpHost)
				var opts *config.SSHOptions
				if sshConn.UseGlobalSettings == nil || *sshConn.UseGlobalSettings {
					mo := config.MergeSSHOptions(gc.SSHOptions, sshConn.SSHOptions)
					opts = &mo
				} else {
					opts = sshConn.SSHOptions
				}
				tunnelCmd, _ = sshpkg.BuildTunnelCommand(sshConn, jump, opts, port, d.Host, d.Port)
				if err := tunnelCmd.Start(); err != nil {
					return fmt.Errorf("opening tunnel: %w", err)
				}
				defer func() {
					_ = tunnelCmd.Process.Kill()
					_ = tunnelCmd.Wait()
				}()
				if err := sshpkg.WaitForLocalPort(port, 6*time.Second); err != nil {
					return err
				}
				targetHost = "127.0.0.1"
				targetPort = port
			}

			pw, _ := config.GetPassword(d.ID.String())
			cc, err := dbpkg.Build(d, targetHost, targetPort, pw)
			if err != nil {
				return err
			}
			c := exec.Command(cc.Path, cc.Args[1:]...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if len(cc.Env) > 0 {
				c.Env = append(os.Environ(), cc.Env...)
			}
			return c.Run()
		},
	}
	return cmd
}

func resolveSSHTunnel(cfg *config.HangarConfig, ref string) (*config.Connection, error) {
	if c := sshpkg.ResolveJumpHost(cfg, ref); c != nil {
		return c, nil
	}
	return nil, fmt.Errorf("ssh tunnel %q not found", ref)
}
