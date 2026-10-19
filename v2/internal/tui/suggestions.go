package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/v4run/hangar/v2/internal/config"
)

// connectionNameSuggestions returns every connection name usable as
// a jump host, alpha-sorted. `exclude` (optional) is skipped so a
// connection can't suggest itself as its own jump.
func connectionNameSuggestions(cfg *config.Config, exclude *config.Connection) []string {
	out := make([]string, 0, len(cfg.Connections))
	for i := range cfg.Connections {
		c := &cfg.Connections[i]
		if exclude != nil && c.ID == exclude.ID {
			continue
		}
		if c.Name == "" {
			continue
		}
		out = append(out, c.Name)
	}
	sort.Strings(out)
	return out
}

// identityFileSuggestions returns candidate private-key paths from
// ~/.ssh. Filters obvious non-keys (known_hosts, config, *.pub) and
// returns `~/.ssh/<file>` form so the user sees a path they can
// paste or tab-complete.
func identityFileSuggestions() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".ssh")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".pub") {
			continue
		}
		switch name {
		case "known_hosts", "known_hosts.old", "config", "authorized_keys", "environment", "rc":
			continue
		}
		out = append(out, "~/.ssh/"+name)
	}
	sort.Strings(out)
	return out
}
