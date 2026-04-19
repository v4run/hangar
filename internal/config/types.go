package config

import "time"

type Connection struct {
	Name                string   `yaml:"name"`
	Host                string   `yaml:"host"`
	Port                int      `yaml:"port"`
	User                string   `yaml:"user"`
	IdentityFile        string   `yaml:"identity_file,omitempty"`
	Tags                []string `yaml:"tags,omitempty"`
	JumpHost            string   `yaml:"jump_host,omitempty"`
	SyncedFromSSHConfig bool     `yaml:"synced_from_ssh_config"`
}

type SSHSync struct {
	LastSync          time.Time `yaml:"last_sync,omitempty"`
	LastSSHConfigHash string    `yaml:"last_ssh_config_hash,omitempty"`
}

type HangarConfig struct {
	Connections []Connection `yaml:"connections"`
	SSHSync     SSHSync      `yaml:"ssh_sync"`
}

type GlobalConfig struct {
	PrefixKey     string `yaml:"prefix_key"`
	SSHConfigPath string `yaml:"ssh_config_path"`
	AutoSync      bool   `yaml:"auto_sync"`
}
