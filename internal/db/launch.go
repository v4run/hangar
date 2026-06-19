// Package db builds the local CLI commands used to attach to a database
// profile. The commands are pure data — execution and tunnel orchestration
// live in the TUI / SSH packages.
package db

import (
	"fmt"
	"net/url"
	"os/exec"

	"github.com/v4run/hangar/internal/config"
)

// ClientCommand is what a caller wires into exec.Cmd: argv + env.
type ClientCommand struct {
	Path string   // resolved binary on PATH
	Args []string // argv[0] = Path
	Env  []string // additional KEY=VALUE entries (merged with os.Environ by caller)
}

// Build returns the command to launch the client for the given database
// profile. host/port override the profile's own values — that's how the
// tunneled path injects 127.0.0.1:<localport>. Pass empty host to use the
// profile values. password is consumed (via env or URL) so the caller
// doesn't have to wire it.
func Build(d *config.Database, host string, port int, password string) (ClientCommand, error) {
	if host == "" {
		host = d.Host
	}
	if port == 0 {
		port = d.Port
	}
	switch d.Engine {
	case config.EnginePostgres:
		return buildPostgres(d, host, port, password)
	case config.EngineMySQL:
		return buildMySQL(d, host, port, password)
	case config.EngineRedis:
		return buildRedis(d, host, port, password)
	case config.EngineSQLite:
		return buildSQLite(d)
	default:
		return ClientCommand{}, fmt.Errorf("unsupported engine %q", d.Engine)
	}
}

func buildPostgres(d *config.Database, host string, port int, password string) (ClientCommand, error) {
	if d.Client == config.ClientPGCLI {
		path, err := exec.LookPath("pgcli")
		if err != nil {
			return ClientCommand{}, fmt.Errorf("pgcli not found on PATH: %w", err)
		}
		u := &url.URL{
			Scheme: "postgresql",
			Host:   fmt.Sprintf("%s:%d", host, port),
			Path:   "/" + d.DBName,
		}
		if d.User != "" {
			if password != "" {
				u.User = url.UserPassword(d.User, password)
			} else {
				u.User = url.User(d.User)
			}
		}
		return ClientCommand{Path: path, Args: []string{path, u.String()}}, nil
	}
	path, err := exec.LookPath("psql")
	if err != nil {
		return ClientCommand{}, fmt.Errorf("psql not found on PATH: %w", err)
	}
	args := []string{path, "-h", host, "-p", fmt.Sprintf("%d", port)}
	if d.User != "" {
		args = append(args, "-U", d.User)
	}
	if d.DBName != "" {
		args = append(args, "-d", d.DBName)
	}
	cmd := ClientCommand{Path: path, Args: args}
	if password != "" {
		cmd.Env = []string{"PGPASSWORD=" + password}
	}
	return cmd, nil
}

func buildMySQL(d *config.Database, host string, port int, password string) (ClientCommand, error) {
	path, err := exec.LookPath("mysql")
	if err != nil {
		return ClientCommand{}, fmt.Errorf("mysql not found on PATH: %w", err)
	}
	args := []string{path, "-h", host, "-P", fmt.Sprintf("%d", port)}
	if d.User != "" {
		args = append(args, "-u", d.User)
	}
	if d.DBName != "" {
		args = append(args, d.DBName)
	}
	cmd := ClientCommand{Path: path, Args: args}
	if password != "" {
		cmd.Env = []string{"MYSQL_PWD=" + password}
	}
	return cmd, nil
}

func buildRedis(d *config.Database, host string, port int, password string) (ClientCommand, error) {
	path, err := exec.LookPath("redis-cli")
	if err != nil {
		return ClientCommand{}, fmt.Errorf("redis-cli not found on PATH: %w", err)
	}
	args := []string{path, "-h", host, "-p", fmt.Sprintf("%d", port)}
	if d.User != "" {
		args = append(args, "--user", d.User)
	}
	cmd := ClientCommand{Path: path, Args: args}
	if password != "" {
		cmd.Env = []string{"REDISCLI_AUTH=" + password}
	}
	return cmd, nil
}

func buildSQLite(d *config.Database) (ClientCommand, error) {
	path, err := exec.LookPath("sqlite3")
	if err != nil {
		return ClientCommand{}, fmt.Errorf("sqlite3 not found on PATH: %w", err)
	}
	return ClientCommand{Path: path, Args: []string{path, d.Host}}, nil
}

// RequiresTunnelTarget reports whether the engine connects over TCP and
// therefore can be tunneled via ssh -L. SQLite is local-file only.
func RequiresTunnelTarget(engine config.DBEngine) bool {
	return engine != config.EngineSQLite
}
