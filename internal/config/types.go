package config

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
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
	Connections   []Connection `yaml:"connections"`
	SSHSync       SSHSync      `yaml:"ssh_sync"`
	GlobalScripts []Script     `yaml:"global_scripts,omitempty"`
	Groups        GroupList    `yaml:"groups,omitempty"`

	// groupsFromLegacyMap is set by UnmarshalYAML when the on-disk groups
	// field was written in the old map[string]bool form. Migrate() uses this
	// to trigger a Save even when there are no structural changes.
	groupsFromLegacyMap bool
}

// UnmarshalYAML deserializes HangarConfig and records whether the groups field
// was in the legacy map form so that Migrate() can trigger a format upgrade.
func (cfg *HangarConfig) UnmarshalYAML(node *yaml.Node) error {
	// Use a type alias to avoid infinite recursion.
	type hangarConfigAlias HangarConfig
	var alias hangarConfigAlias
	if err := node.Decode(&alias); err != nil {
		return err
	}
	*cfg = HangarConfig(alias)

	// Walk the mapping to detect whether the groups field used the legacy
	// map[string]bool form.
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		if keyNode.Value == "groups" && valueNode.Kind == yaml.MappingNode {
			cfg.groupsFromLegacyMap = true
			break
		}
	}
	return nil
}

// GroupList is an ordered list of group names. It accepts both legacy
// `map[string]bool` and the new `[]string` YAML forms during unmarshal.
type GroupList []string

// UnmarshalYAML accepts both the new sequence form (`[a, b, c]`) and the
// legacy mapping form (`{a: true, b: true}`). Mapping form is sorted
// alphabetically for deterministic ordering on first migration.
func (g *GroupList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var s []string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*g = s
	case yaml.MappingNode:
		var m map[string]bool
		if err := node.Decode(&m); err != nil {
			return err
		}
		names := make([]string, 0, len(m))
		for k := range m {
			names = append(names, k)
		}
		sort.Strings(names)
		*g = names
	default:
		return fmt.Errorf("groups: unexpected YAML node kind %d", node.Kind)
	}
	return nil
}

type GlobalConfig struct {
	PrefixKey     string      `yaml:"prefix_key"`
	SSHConfigPath string      `yaml:"ssh_config_path"`
	AutoSync      bool        `yaml:"auto_sync"`
	SSHOptions    *SSHOptions `yaml:"ssh_options,omitempty"`
}
