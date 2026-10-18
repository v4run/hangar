package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/v4run/hangar/v2/internal/config"
	sshauth "github.com/v4run/hangar/v2/internal/ssh"
)

// dbPickable adapts a *config.Database for the fuzzy picker.
type dbPickable struct {
	cfg *config.Config
	db  *config.Database
}

func (p dbPickable) Title() string { return p.db.Name }
func (p dbPickable) Subtitle() string {
	path := config.GroupPath(p.cfg, p.db.Group)
	if path != "" {
		return fmt.Sprintf("%s • %s", p.db.Engine, path)
	}
	return p.db.Engine
}
func (p dbPickable) Preview() string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %-11s %s\n", "Engine", p.db.Engine)
	fmt.Fprintf(&b, "  %-11s %s\n", "Host", p.db.Host)
	if p.db.Port != 0 {
		fmt.Fprintf(&b, "  %-11s %d\n", "Port", p.db.Port)
	}
	if p.db.User != "" {
		fmt.Fprintf(&b, "  %-11s %s\n", "User", p.db.User)
	}
	if p.db.DBName != "" {
		fmt.Fprintf(&b, "  %-11s %s\n", "Database", p.db.DBName)
	}
	return b.String()
}
func (p dbPickable) Keywords() []string {
	kw := []string{p.db.Name, p.db.Engine, p.db.Host, p.db.User, p.db.DBName}
	kw = append(kw, p.db.Tags...)
	return kw
}

// launchDatabase shells out to the engine-specific client. Only
// local launches are supported in alpha.1 (no SSH tunnel) — the
// tunnel path lands in alpha.2.
func (m *Model) launchDatabase(d *config.Database) tea.Cmd {
	password := resolveDBPassword(d)
	cmd, err := buildDBClient(d, d.Host, d.Port, password)
	if err != nil {
		m.setToast("db: "+err.Error(), toastErr)
		return m.clearToastCmd()
	}
	m.mode = ModeConnect
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sshExitMsg{err: err}
	})
}

func resolveDBPassword(d *config.Database) string {
	if d.Password != "" && sshauth.IsCommandValue(d.Password) {
		if v, err := sshauth.ResolveValue(d.Password); err == nil {
			return v
		}
	} else if d.Password != "" {
		return d.Password
	}
	if v, err := sshauth.GetPassword(d.ID.String()); err == nil && v != "" {
		return v
	}
	return ""
}

func buildDBClient(d *config.Database, host string, port int, password string) (*exec.Cmd, error) {
	switch d.Engine {
	case "postgres":
		path, err := exec.LookPath("psql")
		if err != nil {
			return nil, fmt.Errorf("psql not found")
		}
		args := []string{"-h", host}
		if port != 0 {
			args = append(args, "-p", fmt.Sprintf("%d", port))
		}
		if d.User != "" {
			args = append(args, "-U", d.User)
		}
		if d.DBName != "" {
			args = append(args, "-d", d.DBName)
		}
		c := exec.Command(path, args...)
		if password != "" {
			c.Env = append(c.Env, "PGPASSWORD="+password)
		}
		return c, nil
	case "mysql":
		path, err := exec.LookPath("mysql")
		if err != nil {
			return nil, fmt.Errorf("mysql not found")
		}
		args := []string{"-h", host}
		if port != 0 {
			args = append(args, "-P", fmt.Sprintf("%d", port))
		}
		if d.User != "" {
			args = append(args, "-u", d.User)
		}
		if d.DBName != "" {
			args = append(args, d.DBName)
		}
		if password != "" {
			args = append([]string{"-p" + password}, args...)
		}
		return exec.Command(path, args...), nil
	case "redis":
		path, err := exec.LookPath("redis-cli")
		if err != nil {
			return nil, fmt.Errorf("redis-cli not found")
		}
		args := []string{"-h", host}
		if port != 0 {
			args = append(args, "-p", fmt.Sprintf("%d", port))
		}
		if password != "" {
			args = append(args, "-a", password)
		}
		if d.DBName != "" {
			args = append(args, "-n", d.DBName)
		}
		return exec.Command(path, args...), nil
	case "sqlite":
		path, err := exec.LookPath("sqlite3")
		if err != nil {
			return nil, fmt.Errorf("sqlite3 not found")
		}
		return exec.Command(path, d.Host), nil
	}
	return nil, fmt.Errorf("unsupported engine %q", d.Engine)
}

// urlEncode is a tiny helper used when a client prefers a connection
// url (pgcli, etc.) — kept here so we don't drag net/url into every
// callsite. Currently unused in alpha.1.
var _ = url.PathEscape
