package config

import (
	"time"

	"github.com/google/uuid"
)

type Script struct {
	Name            string        `yaml:"name"`
	Command         string        `yaml:"command"`
	LastRunAt       *time.Time    `yaml:"last_run_at,omitempty"`
	LastRunDuration time.Duration `yaml:"last_run_duration,omitempty"`
	LastRunExit     int           `yaml:"last_run_exit,omitempty"`
}

type SSHOptions struct {
	ForwardAgent        *bool             `yaml:"forward_agent,omitempty"`
	LocalForward        []string          `yaml:"local_forward,omitempty"`
	RemoteForward       []string          `yaml:"remote_forward,omitempty"`
	ServerAliveInterval *int              `yaml:"server_alive_interval,omitempty"`
	ServerAliveCountMax *int              `yaml:"server_alive_count_max,omitempty"`
	StrictHostKeyCheck  string            `yaml:"strict_host_key_checking,omitempty"`
	RequestTTY          string            `yaml:"request_tty,omitempty"`
	Compression         *bool             `yaml:"compression,omitempty"`
	EnvVars             map[string]string `yaml:"env_vars,omitempty"`
	ExtraOptions        map[string]string `yaml:"extra_options,omitempty"`
}

type Connection struct {
	ID                  uuid.UUID   `yaml:"id"`
	Name                string      `yaml:"name"`
	Host                string      `yaml:"host"`
	Port                int         `yaml:"port"`
	User                string      `yaml:"user"`
	IdentityFile        string      `yaml:"identity_file,omitempty"`
	Tags                []string    `yaml:"tags,omitempty"`
	JumpHost            string      `yaml:"jump_host,omitempty"`
	Group               string      `yaml:"group,omitempty"`
	SyncedFromSSHConfig bool        `yaml:"synced_from_ssh_config"`
	Scripts             []Script    `yaml:"scripts,omitempty"`
	Notes               string      `yaml:"notes,omitempty"`
	SSHOptions          *SSHOptions `yaml:"ssh_options,omitempty"`
	UseGlobalSettings   *bool       `yaml:"use_global_settings,omitempty"`
}

type SSHSync struct {
	LastSync          time.Time `yaml:"last_sync,omitempty"`
	LastSSHConfigHash string    `yaml:"last_ssh_config_hash,omitempty"`
}

type HangarConfig struct {
	Connections   []Connection    `yaml:"connections"`
	SSHSync       SSHSync         `yaml:"ssh_sync"`
	GlobalScripts []Script        `yaml:"global_scripts,omitempty"`
	Groups        map[string]bool `yaml:"groups,omitempty"`
}

type GlobalConfig struct {
	PrefixKey     string      `yaml:"prefix_key"`
	SSHConfigPath string      `yaml:"ssh_config_path"`
	AutoSync      bool        `yaml:"auto_sync"`
	SSHOptions    *SSHOptions `yaml:"ssh_options,omitempty"`
}
