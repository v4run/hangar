// Package config holds Hangar v2's on-disk schema (schema: 2) and the
// resolver that flattens a connection against its group ancestry +
// global defaults into the effective Connection that ssh.Session uses.
package config

import "github.com/google/uuid"

// SchemaVersion is the integer v2 writes into every config file.
const SchemaVersion = 2

// Config is the top-level file shape.
type Config struct {
	Schema       int               `yaml:"schema"`
	SSHOptions   *SSHOptions       `yaml:"ssh_options,omitempty"`
	ShellInit    string            `yaml:"shell_init,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"`
	Groups       []Group           `yaml:"groups,omitempty"`
	Connections  []Connection      `yaml:"connections,omitempty"`
	Databases    []Database        `yaml:"databases,omitempty"`
}

// Group is an inheritance scope. Groups form a tree via Parent (zero
// uuid = root). Defaults set here cascade to every descendant group
// and connection unless the child overrides.
type Group struct {
	ID         uuid.UUID   `yaml:"id"`
	Name       string      `yaml:"name"`
	Parent     uuid.UUID   `yaml:"parent,omitempty"`
	Order      int         `yaml:"order,omitempty"` // sort key within the parent — mixes with Connection/Database.Order
	User       string      `yaml:"user,omitempty"`
	IdentityFile string    `yaml:"identity_file,omitempty"`
	JumpHost   uuid.UUID   `yaml:"jump_host,omitempty"`
	Tags       []string    `yaml:"tags,omitempty"`
	SSHOptions *SSHOptions `yaml:"ssh_options,omitempty"`
	ShellInit  string      `yaml:"shell_init,omitempty"`
	Scripts    []Script    `yaml:"scripts,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
}

// Connection is a single host.
type Connection struct {
	ID            uuid.UUID   `yaml:"id"`
	Name          string      `yaml:"name"`
	Host          string      `yaml:"host"`
	Port          int         `yaml:"port,omitempty"`
	User          string      `yaml:"user,omitempty"`
	IdentityFile  string      `yaml:"identity_file,omitempty"`
	Group         uuid.UUID   `yaml:"group,omitempty"`
	Order         int         `yaml:"order,omitempty"` // sort key within Group — mixes with Group/Database.Order
	JumpHost      uuid.UUID   `yaml:"jump_host,omitempty"`
	Tags          []string    `yaml:"tags,omitempty"`
	SSHOptions    *SSHOptions `yaml:"ssh_options,omitempty"`
	ShellInit     string      `yaml:"shell_init,omitempty"`
	Scripts       []Script    `yaml:"scripts,omitempty"`
	Env           map[string]string `yaml:"env,omitempty"`
	Notes         string      `yaml:"notes,omitempty"`
	Password      string      `yaml:"password,omitempty"` // $(cmd) form; literals go to keychain
	RemoteCommand string      `yaml:"remote_command,omitempty"`
}

// Script attached to a connection or inherited from a group.
type Script struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

// SSHOptions holds the subset of ssh_config keys Hangar understands.
// All fields are pointer-ish so "not set" is distinguishable from
// "explicitly set to zero" and inheritance works.
type SSHOptions struct {
	ForwardAgent        *bool             `yaml:"forward_agent,omitempty"`
	Compression         *bool             `yaml:"compression,omitempty"`
	LocalForward        []string          `yaml:"local_forward,omitempty"`
	RemoteForward       []string          `yaml:"remote_forward,omitempty"`
	ServerAliveInterval *int              `yaml:"server_alive_interval,omitempty"`
	ServerAliveCountMax *int              `yaml:"server_alive_count_max,omitempty"`
	StrictHostKeyCheck  string            `yaml:"strict_host_key_checking,omitempty"`
	RequestTTY          string            `yaml:"request_tty,omitempty"`
	ExtraOptions        map[string]string `yaml:"extra_options,omitempty"`
}

// Database is the DB engine entry. Kept minimal in v2.0.0-alpha.1 and
// expanded in later phases.
type Database struct {
	ID       uuid.UUID         `yaml:"id"`
	Name     string            `yaml:"name"`
	Engine   string            `yaml:"engine"` // postgres | mysql | redis | sqlite
	Host     string            `yaml:"host,omitempty"`
	Port     int               `yaml:"port,omitempty"`
	User     string            `yaml:"user,omitempty"`
	DBName   string            `yaml:"dbname,omitempty"`
	TunnelVia uuid.UUID        `yaml:"tunnel_via,omitempty"` // connection to tunnel through
	Group    uuid.UUID         `yaml:"group,omitempty"`
	Order    int               `yaml:"order,omitempty"`
	Tags     []string          `yaml:"tags,omitempty"`
	Notes    string            `yaml:"notes,omitempty"`
	Password string            `yaml:"password,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
}
