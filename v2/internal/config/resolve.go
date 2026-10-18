package config

import (
	"github.com/google/uuid"
)

// Scope names which level a field came from — useful for the detail
// pane's "— from prod/" annotation. The zero value, ScopeOwn, means
// the field lives on the connection itself.
type Scope int

const (
	ScopeOwn Scope = iota
	ScopeGroup
	ScopeGlobal
)

// Resolved is a connection after inheritance has been walked.
// FieldScope maps a field name (as it would appear in the UI, e.g.
// "User", "IdentityFile", "LocalForward") to the scope that
// contributed it. Fields the user never set anywhere are absent.
type Resolved struct {
	*Connection
	FieldScope  map[string]Scope
	FieldOrigin map[string]string // group name path like "prod/us-east"
	Groups      []*Group          // ancestor chain, leaf-first
}

// Resolve walks the group ancestry of conn and merges in defaults
// from (specific → general): connection → immediate group → parent
// → … → global. First non-zero value wins; slice/map fields are
// merged by appending/upserting, not overwritten.
func Resolve(cfg *Config, conn *Connection) *Resolved {
	byID := make(map[uuid.UUID]*Group, len(cfg.Groups))
	for i := range cfg.Groups {
		byID[cfg.Groups[i].ID] = &cfg.Groups[i]
	}

	// Build ancestor chain, leaf-first.
	var chain []*Group
	var names []string
	for cur := conn.Group; cur != uuid.Nil; {
		g, ok := byID[cur]
		if !ok {
			break
		}
		chain = append(chain, g)
		names = append([]string{g.Name}, names...)
		cur = g.Parent
	}

	out := *conn // shallow copy
	scope := map[string]Scope{}
	origin := map[string]string{}
	note := func(field string, from Scope, groupPath string) {
		scope[field] = from
		if groupPath != "" {
			origin[field] = groupPath
		}
	}

	// Own fields that are already set get ScopeOwn.
	if conn.User != "" {
		note("User", ScopeOwn, "")
	}
	if conn.IdentityFile != "" {
		note("IdentityFile", ScopeOwn, "")
	}
	if conn.JumpHost != uuid.Nil {
		note("JumpHost", ScopeOwn, "")
	}
	if conn.ShellInit != "" {
		note("ShellInit", ScopeOwn, "")
	}
	if len(conn.Tags) > 0 {
		note("Tags", ScopeOwn, "")
	}

	// Walk chain filling in zero fields.
	for i, g := range chain {
		path := joinNames(names, len(names)-i)
		if out.User == "" && g.User != "" {
			out.User = g.User
			note("User", ScopeGroup, path)
		}
		if out.IdentityFile == "" && g.IdentityFile != "" {
			out.IdentityFile = g.IdentityFile
			note("IdentityFile", ScopeGroup, path)
		}
		if out.JumpHost == uuid.Nil && g.JumpHost != uuid.Nil {
			out.JumpHost = g.JumpHost
			note("JumpHost", ScopeGroup, path)
		}
		if out.ShellInit == "" && g.ShellInit != "" {
			out.ShellInit = g.ShellInit
			note("ShellInit", ScopeGroup, path)
		}
		out.Tags = mergeStrings(out.Tags, g.Tags)
		out.Env = mergeMap(out.Env, g.Env)
		out.Scripts = mergeScripts(out.Scripts, g.Scripts)
		out.SSHOptions = mergeOptions(out.SSHOptions, g.SSHOptions)
	}

	// Global defaults.
	if cfg.ShellInit != "" && out.ShellInit == "" {
		out.ShellInit = cfg.ShellInit
		note("ShellInit", ScopeGlobal, "")
	}
	out.SSHOptions = mergeOptions(out.SSHOptions, cfg.SSHOptions)

	return &Resolved{
		Connection:  &out,
		FieldScope:  scope,
		FieldOrigin: origin,
		Groups:      chain,
	}
}

// GroupPath renders a group's full path (`prod/us-east/web`).
func GroupPath(cfg *Config, id uuid.UUID) string {
	byID := make(map[uuid.UUID]*Group, len(cfg.Groups))
	for i := range cfg.Groups {
		byID[cfg.Groups[i].ID] = &cfg.Groups[i]
	}
	var parts []string
	for cur := id; cur != uuid.Nil; {
		g, ok := byID[cur]
		if !ok {
			break
		}
		parts = append([]string{g.Name}, parts...)
		cur = g.Parent
	}
	return joinPath(parts)
}

func joinPath(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "/" + p
	}
	return out
}

// joinNames returns the first `n` segments of names joined with `/`.
func joinNames(names []string, n int) string {
	if n <= 0 || n > len(names) {
		n = len(names)
	}
	return joinPath(names[:n])
}

func mergeStrings(dst, src []string) []string {
	seen := map[string]bool{}
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range src {
		if !seen[s] {
			dst = append(dst, s)
			seen[s] = true
		}
	}
	return dst
}

func mergeMap(dst, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
	return dst
}

func mergeScripts(dst, src []Script) []Script {
	seen := map[string]bool{}
	for _, s := range dst {
		seen[s.Name] = true
	}
	for _, s := range src {
		if !seen[s.Name] {
			dst = append(dst, s)
		}
	}
	return dst
}

func mergeOptions(dst, src *SSHOptions) *SSHOptions {
	if src == nil {
		return dst
	}
	if dst == nil {
		dst = &SSHOptions{}
	}
	if dst.ForwardAgent == nil && src.ForwardAgent != nil {
		dst.ForwardAgent = src.ForwardAgent
	}
	if dst.Compression == nil && src.Compression != nil {
		dst.Compression = src.Compression
	}
	if dst.ServerAliveInterval == nil && src.ServerAliveInterval != nil {
		dst.ServerAliveInterval = src.ServerAliveInterval
	}
	if dst.ServerAliveCountMax == nil && src.ServerAliveCountMax != nil {
		dst.ServerAliveCountMax = src.ServerAliveCountMax
	}
	if dst.StrictHostKeyCheck == "" && src.StrictHostKeyCheck != "" {
		dst.StrictHostKeyCheck = src.StrictHostKeyCheck
	}
	if dst.RequestTTY == "" && src.RequestTTY != "" {
		dst.RequestTTY = src.RequestTTY
	}
	dst.LocalForward = append(dst.LocalForward, src.LocalForward...)
	dst.RemoteForward = append(dst.RemoteForward, src.RemoteForward...)
	if len(src.ExtraOptions) > 0 {
		if dst.ExtraOptions == nil {
			dst.ExtraOptions = make(map[string]string, len(src.ExtraOptions))
		}
		for k, v := range src.ExtraOptions {
			if _, exists := dst.ExtraOptions[k]; !exists {
				dst.ExtraOptions[k] = v
			}
		}
	}
	return dst
}
