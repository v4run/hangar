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

// MigrateFromV1 looks for ~/.hangar/config.yaml and, if present,
// returns an equivalent schema-2 Config. ok=false means no v1
// config was found (fresh install).
func MigrateFromV1() (*Config, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false, err
	}
	path := filepath.Join(home, ".hangar", "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var v1 v1Config
	if err := yaml.Unmarshal(data, &v1); err != nil {
		return nil, false, fmt.Errorf("parse v1 config: %w", err)
	}
	cfg := migrateV1(&v1)
	return cfg, true, nil
}

func migrateV1(v1 *v1Config) *Config {
	out := &Config{
		Schema:     SchemaVersion,
		SSHOptions: v1.SSHOptions,
		ShellInit:  v1.GlobalShellInit,
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
