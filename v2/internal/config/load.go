package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultDir returns the directory v2 reads/writes under
// `$HOME/.hangar/v2`.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hangar", "v2"), nil
}

// Load reads config.yaml from dir. If the file is missing a Config
// with schema 2 is returned (the fresh-install case). If a v1 config
// exists at the sibling path and no v2 file is present, Load auto-
// runs the migration in memory and writes the result to dir.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		var c Config
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if c.Schema == 0 {
			c.Schema = SchemaVersion
		}
		if c.Schema != SchemaVersion {
			return nil, fmt.Errorf("%s: schema %d not supported by this binary (expected %d)", path, c.Schema, SchemaVersion)
		}
		if err := Validate(&c); err != nil {
			return nil, fmt.Errorf("validate %s: %w", path, err)
		}
		return &c, nil
	case errors.Is(err, os.ErrNotExist):
		// Fresh install OR first v2 run against an existing v1 config.
		if migrated, ok, mErr := MigrateFromV1(); mErr != nil {
			return nil, fmt.Errorf("migrate v1 config: %w", mErr)
		} else if ok {
			if err := Save(dir, migrated); err != nil {
				return nil, fmt.Errorf("save migrated config: %w", err)
			}
			return migrated, nil
		}
		return &Config{Schema: SchemaVersion}, nil
	default:
		return nil, err
	}
}

// Save writes the config atomically to dir/config.yaml.
func Save(dir string, cfg *Config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if cfg.Schema == 0 {
		cfg.Schema = SchemaVersion
	}
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
