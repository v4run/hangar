package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// v1Config is the subset of the v1 schema we need to migrate from.
// We parse into this intentionally-loose struct so unknown fields
// are ignored rather than failing the migration.
type v1Config struct {
	SSHOptions      *SSHOptions         `yaml:"ssh_options"`
	GlobalShellInit string              `yaml:"global_shell_init"`
	GlobalEnv       map[string]string   `yaml:"-"` // promoted from v1 config.yaml ssh_options.env_vars
	Groups          []string            `yaml:"groups"`
	GroupShellInit  map[string]string   `yaml:"group_shell_init"`
	Connections     []v1Connection      `yaml:"connections"`
	Databases       []v1Database        `yaml:"databases"`
}

type v1Connection struct {
	ID            uuid.UUID         `yaml:"id"`
	Name          string            `yaml:"name"`
	Host          string            `yaml:"host"`
	Port          int               `yaml:"port"`
	User          string            `yaml:"user"`
	IdentityFile  string            `yaml:"identity_file"`
	Tags          []string          `yaml:"tags"`
	JumpHost      string            `yaml:"jump_host"` // v1 stored a string (name or uuid-string)
	Group         string            `yaml:"group"`     // v1 stored the group name
	Scripts       []Script          `yaml:"scripts"`
	Notes         string            `yaml:"notes"`
	SSHOptions    *SSHOptions       `yaml:"ssh_options"`
	UseGlobalSettings  *bool        `yaml:"use_global_settings"`
	ShellInit     string            `yaml:"shell_init"`
	UseGlobalShellInit *bool        `yaml:"use_global_shell_init"`
	Password      string            `yaml:"password"`
	RemoteCommand string            `yaml:"remote_command"`
}

type v1Database struct {
	ID       uuid.UUID         `yaml:"id"`
	Name     string            `yaml:"name"`
	Engine   string            `yaml:"engine"`
	Host     string            `yaml:"host"`
	Port     int               `yaml:"port"`
	User     string            `yaml:"user"`
	DBName   string            `yaml:"dbname"`
	Tunnel   string            `yaml:"tunnel"` // v1 stored uuid-string
	Group    string            `yaml:"group"`
	Tags     []string          `yaml:"tags"`
	Notes    string            `yaml:"notes"`
	Password string            `yaml:"password"`
}

// v1Global parses the subset of v1's ~/.hangar/config.yaml we care
// about: global SSH options + the env_vars map that v1 nested under
// ssh_options. Everything else in that file is v1 chrome and
// ignored.
type v1Global struct {
	SSHOptions *v1SSHOptions `yaml:"ssh_options"`
}

// v1SSHOptions mirrors v2's SSHOptions but keeps v1's env_vars field
// so we can promote it onto cfg.Env (v2 moved env out of ssh_options
// onto Connection / Group / Config directly).
type v1SSHOptions struct {
	ForwardAgent        *bool             `yaml:"forward_agent,omitempty"`
	Compression         *bool             `yaml:"compression,omitempty"`
	LocalForward        []string          `yaml:"local_forward,omitempty"`
	RemoteForward       []string          `yaml:"remote_forward,omitempty"`
	ServerAliveInterval *int              `yaml:"server_alive_interval,omitempty"`
	ServerAliveCountMax *int              `yaml:"server_alive_count_max,omitempty"`
	StrictHostKeyCheck  string            `yaml:"strict_host_key_checking,omitempty"`
	RequestTTY          string            `yaml:"request_tty,omitempty"`
	EnvVars             map[string]string `yaml:"env_vars,omitempty"`
	ExtraOptions        map[string]string `yaml:"extra_options,omitempty"`
}

func (v *v1SSHOptions) toV2() *SSHOptions {
	if v == nil {
		return nil
	}
	return &SSHOptions{
		ForwardAgent:        v.ForwardAgent,
		Compression:         v.Compression,
		LocalForward:        append([]string(nil), v.LocalForward...),
		RemoteForward:       append([]string(nil), v.RemoteForward...),
		ServerAliveInterval: v.ServerAliveInterval,
		ServerAliveCountMax: v.ServerAliveCountMax,
		StrictHostKeyCheck:  v.StrictHostKeyCheck,
		RequestTTY:          v.RequestTTY,
		ExtraOptions:        v.ExtraOptions,
	}
}

// MigrateFromV1 looks for a v1 install (both ~/.hangar/connections.yaml
// — the connection/group/db data — and ~/.hangar/config.yaml — global
// SSH options). ok=false means no v1 data was found (fresh install).
func MigrateFromV1() (*Config, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false, err
	}
	v1 := &v1Config{}
	found := false

	// connections.yaml carries connections, groups, databases,
	// shell-init etc. Historically this file was called
	// `connections.yaml` in v1; older installs may have everything in
	// `config.yaml`.
	for _, name := range []string{"connections.yaml", "config.yaml"} {
		data, rErr := os.ReadFile(filepath.Join(home, ".hangar", name))
		if errors.Is(rErr, os.ErrNotExist) {
			continue
		}
		if rErr != nil {
			return nil, false, rErr
		}
		var parsed v1Config
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, false, fmt.Errorf("parse %s: %w", name, err)
		}
		if len(parsed.Connections) == 0 && len(parsed.Groups) == 0 && len(parsed.Databases) == 0 && parsed.GlobalShellInit == "" {
			continue
		}
		*v1 = parsed
		found = true
		break
	}

	// Separately, ~/.hangar/config.yaml holds the GlobalConfig
	// SSHOptions. Merge them in regardless of where the connections
	// came from.
	if data, err := os.ReadFile(filepath.Join(home, ".hangar", "config.yaml")); err == nil {
		var g v1Global
		if yaml.Unmarshal(data, &g) == nil && g.SSHOptions != nil {
			v1.SSHOptions = g.SSHOptions.toV2()
			if len(g.SSHOptions.EnvVars) > 0 {
				v1.GlobalEnv = g.SSHOptions.EnvVars
			}
			found = true
		}
	}

	if !found {
		return nil, false, nil
	}
	cfg := migrateV1(v1)
	return cfg, true, nil
}

func migrateV1(v1 *v1Config) *Config {
	out := &Config{
		Schema:     SchemaVersion,
		SSHOptions: v1.SSHOptions,
		ShellInit:  v1.GlobalShellInit,
		Env:        v1.GlobalEnv,
	}

	// 1. Promote each v1 group name to a Group{} with a fresh UUID.
	nameToID := make(map[string]uuid.UUID, len(v1.Groups))
	for _, name := range v1.Groups {
		id := uuid.New()
		nameToID[name] = id
		g := Group{ID: id, Name: name}
		if si, ok := v1.GroupShellInit[name]; ok {
			g.ShellInit = si
		}
		out.Groups = append(out.Groups, g)
	}
	// Any shell-init entries referring to groups not in Groups[] still
	// get promoted (be forgiving).
	for name, si := range v1.GroupShellInit {
		if _, ok := nameToID[name]; ok {
			continue
		}
		id := uuid.New()
		nameToID[name] = id
		out.Groups = append(out.Groups, Group{ID: id, Name: name, ShellInit: si})
	}

	connNameToID := make(map[string]uuid.UUID, len(v1.Connections))
	for _, c := range v1.Connections {
		connNameToID[c.Name] = c.ID
	}

	// 2. Translate connections.
	for _, c := range v1.Connections {
		nc := Connection{
			ID:            c.ID,
			Name:          c.Name,
			Host:          c.Host,
			Port:          c.Port,
			User:          c.User,
			IdentityFile:  c.IdentityFile,
			Tags:          c.Tags,
			Scripts:       c.Scripts,
			Notes:         c.Notes,
			ShellInit:     c.ShellInit,
			Password:      c.Password,
			RemoteCommand: c.RemoteCommand,
		}
		if gid, ok := nameToID[c.Group]; ok {
			nc.Group = gid
		}
		// JumpHost in v1 was a UUID string or a connection name.
		if c.JumpHost != "" {
			if parsed, err := uuid.Parse(c.JumpHost); err == nil {
				nc.JumpHost = parsed
			} else if other, ok := connNameToID[c.JumpHost]; ok {
				nc.JumpHost = other
			}
		}
		// SSH options: if the connection used global settings, we
		// collapse the effective merge onto the connection so behavior
		// is bit-identical post-migration.
		if c.UseGlobalSettings != nil && !*c.UseGlobalSettings {
			nc.SSHOptions = c.SSHOptions
		} else {
			nc.SSHOptions = mergeOptions(cloneOptions(c.SSHOptions), v1.SSHOptions)
		}
		// Shell-init: ditto; if the user opted out, we snapshot the
		// effective text onto the connection.
		if c.UseGlobalShellInit != nil && !*c.UseGlobalShellInit {
			// own string already set above; leave it
		} else if nc.ShellInit == "" && v1.GlobalShellInit != "" {
			// Group shell init will cascade naturally through Resolve,
			// so only promote the global snippet if nothing else exists.
			nc.ShellInit = ""
		}
		out.Connections = append(out.Connections, nc)
	}

	// 3. Translate databases.
	for _, d := range v1.Databases {
		nd := Database{
			ID:       d.ID,
			Name:     d.Name,
			Engine:   d.Engine,
			Host:     d.Host,
			Port:     d.Port,
			User:     d.User,
			DBName:   d.DBName,
			Tags:     d.Tags,
			Notes:    d.Notes,
			Password: d.Password,
		}
		if gid, ok := nameToID[d.Group]; ok {
			nd.Group = gid
		}
		if d.Tunnel != "" {
			if parsed, err := uuid.Parse(d.Tunnel); err == nil {
				nd.TunnelVia = parsed
			} else if cid, ok := connNameToID[d.Tunnel]; ok {
				nd.TunnelVia = cid
			}
		}
		out.Databases = append(out.Databases, nd)
	}
	return out
}

func cloneOptions(o *SSHOptions) *SSHOptions {
	if o == nil {
		return nil
	}
	c := *o
	if o.LocalForward != nil {
		c.LocalForward = append([]string(nil), o.LocalForward...)
	}
	if o.RemoteForward != nil {
		c.RemoteForward = append([]string(nil), o.RemoteForward...)
	}
	if o.ExtraOptions != nil {
		c.ExtraOptions = make(map[string]string, len(o.ExtraOptions))
		for k, v := range o.ExtraOptions {
			c.ExtraOptions[k] = v
		}
	}
	return &c
}
