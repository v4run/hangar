package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const connectionsFile = "connections.yaml"

func Load(dir string) (*HangarConfig, error) {
	path := filepath.Join(dir, connectionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &HangarConfig{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg HangarConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Migrate() {
		_ = Save(dir, &cfg)
	}
	return &cfg, nil
}

func Save(dir string, cfg *HangarConfig) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	path := filepath.Join(dir, connectionsFile)
	return os.WriteFile(path, data, 0600)
}

func (cfg *HangarConfig) Add(conn Connection) error {
	if conn.Name == "" {
		return fmt.Errorf("connection name is required")
	}
	if conn.Host == "" {
		return fmt.Errorf("host is required for connection %q", conn.Name)
	}
	if conn.User == "" {
		return fmt.Errorf("user is required for connection %q", conn.Name)
	}
	if conn.Port <= 0 {
		return fmt.Errorf("port must be positive for connection %q", conn.Name)
	}
	if conn.ID == uuid.Nil {
		conn.ID = uuid.New()
	}
	cfg.Connections = append(cfg.Connections, conn)
	return nil
}

func (cfg *HangarConfig) Remove(name string) error {
	for i, c := range cfg.Connections {
		if c.Name == name {
			cfg.Connections = append(cfg.Connections[:i], cfg.Connections[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("connection %q not found", name)
}

func (cfg *HangarConfig) RemoveByID(id uuid.UUID) error {
	for i, c := range cfg.Connections {
		if c.ID == id {
			cfg.Connections = append(cfg.Connections[:i], cfg.Connections[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("connection with ID %s not found", id)
}

func (cfg *HangarConfig) FindByName(name string) (*Connection, error) {
	for i := range cfg.Connections {
		if cfg.Connections[i].Name == name {
			return &cfg.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection %q not found", name)
}

func (cfg *HangarConfig) FindByID(id uuid.UUID) (*Connection, error) {
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == id {
			return &cfg.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection with ID %s not found", id)
}

func (cfg *HangarConfig) AddTags(name string, tags []string) error {
	c, err := cfg.FindByName(name)
	if err != nil {
		return err
	}
	existing := make(map[string]bool)
	for _, t := range c.Tags {
		existing[t] = true
	}
	for _, t := range tags {
		if !existing[t] {
			c.Tags = append(c.Tags, t)
			existing[t] = true
		}
	}
	return nil
}

func (cfg *HangarConfig) RemoveTags(name string, tags []string) error {
	c, err := cfg.FindByName(name)
	if err != nil {
		return err
	}
	remove := make(map[string]bool)
	for _, t := range tags {
		remove[t] = true
	}
	filtered := c.Tags[:0]
	for _, t := range c.Tags {
		if !remove[t] {
			filtered = append(filtered, t)
		}
	}
	c.Tags = filtered
	return nil
}

func (cfg *HangarConfig) FilterByTag(tag string) []Connection {
	var results []Connection
	for _, c := range cfg.Connections {
		for _, t := range c.Tags {
			if t == tag {
				results = append(results, c)
				break
			}
		}
	}
	return results
}

func (cfg *HangarConfig) Migrate() bool {
	changed := false
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == uuid.Nil {
			cfg.Connections[i].ID = uuid.New()
			changed = true
		}
	}
	for i := range cfg.Connections {
		jh := cfg.Connections[i].JumpHost
		if jh == "" {
			continue
		}
		if _, err := uuid.Parse(jh); err == nil {
			continue
		}
		if target, err := cfg.FindByName(jh); err == nil {
			cfg.Connections[i].JumpHost = target.ID.String()
			changed = true
		}
	}
	// Backfill Groups slice from connection-referenced groups.
	have := make(map[string]bool, len(cfg.Groups)+len(cfg.Connections))
	for _, g := range cfg.Groups {
		have[g] = true
	}
	var missing []string
	for _, c := range cfg.Connections {
		if c.Group != "" && !have[c.Group] {
			have[c.Group] = true
			missing = append(missing, c.Group)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		cfg.Groups = append(cfg.Groups, missing...)
		changed = true
	}
	return changed
}
