package tui

import (
	"fmt"
	"strings"

	"github.com/v4run/hangar/v2/internal/config"
)

// connPickable adapts a *config.Connection for the fuzzy picker.
type connPickable struct {
	cfg  *config.Config
	conn *config.Connection
}

func (c connPickable) Title() string { return c.conn.Name }

func (c connPickable) Subtitle() string {
	path := config.GroupPath(c.cfg, c.conn.Group)
	user := c.conn.User
	if user == "" {
		user = "—"
	}
	host := c.conn.Host
	if path != "" {
		return fmt.Sprintf("%s • %s@%s", path, user, host)
	}
	return fmt.Sprintf("%s@%s", user, host)
}

func (c connPickable) Preview() string {
	r := config.Resolve(c.cfg, c.conn)
	var b strings.Builder
	line := func(label, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(&b, "  %-11s %s\n", label, val)
	}
	line("Name", c.conn.Name)
	line("Host", c.conn.Host)
	if c.conn.Port != 0 {
		line("Port", fmt.Sprintf("%d", c.conn.Port))
	}
	line("User", r.User)
	line("Identity", r.IdentityFile)
	line("Group", config.GroupPath(c.cfg, c.conn.Group))
	if len(r.Tags) > 0 {
		line("Tags", strings.Join(r.Tags, " "))
	}
	if c.conn.Notes != "" {
		b.WriteString("\n  ")
		b.WriteString(c.conn.Notes)
	}
	return b.String()
}

func (c connPickable) Keywords() []string {
	kw := []string{c.conn.Name, c.conn.Host, c.conn.User}
	kw = append(kw, c.conn.Tags...)
	if p := config.GroupPath(c.cfg, c.conn.Group); p != "" {
		kw = append(kw, strings.Split(p, "/")...)
		kw = append(kw, p)
	}
	return kw
}

// groupPickable adapts a *config.Group.
type groupPickable struct {
	cfg *config.Config
	g   *config.Group
}

func (g groupPickable) Title() string    { return g.g.Name }
func (g groupPickable) Subtitle() string { return config.GroupPath(g.cfg, g.g.ID) }
func (g groupPickable) Preview() string {
	var b strings.Builder
	path := config.GroupPath(g.cfg, g.g.ID)
	fmt.Fprintf(&b, "  %-11s %s\n", "Path", path)
	if g.g.User != "" {
		fmt.Fprintf(&b, "  %-11s %s\n", "Default user", g.g.User)
	}
	if g.g.IdentityFile != "" {
		fmt.Fprintf(&b, "  %-11s %s\n", "Default key", g.g.IdentityFile)
	}
	var count int
	for i := range g.cfg.Connections {
		if g.cfg.Connections[i].Group == g.g.ID {
			count++
		}
	}
	fmt.Fprintf(&b, "  %-11s %d\n", "Connections", count)
	return b.String()
}
func (g groupPickable) Keywords() []string {
	kw := []string{g.g.Name, config.GroupPath(g.cfg, g.g.ID)}
	kw = append(kw, g.g.Tags...)
	return kw
}
