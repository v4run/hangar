package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const globalConfigFile = "config.yaml"

func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		PrefixKey:     "ctrl+a",
		SSHConfigPath: "~/.ssh/config",
		AutoSync:      true,
	}
}

func LoadGlobal(dir string) (*GlobalConfig, error) {
	path := filepath.Join(dir, globalConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultGlobalConfig(), nil
		}
		return nil, fmt.Errorf("reading global config: %w", err)
	}
	gc := DefaultGlobalConfig()
	if err := yaml.Unmarshal(data, gc); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	return gc, nil
}

func SaveGlobal(dir string, gc *GlobalConfig) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(gc)
	if err != nil {
		return fmt.Errorf("marshaling global config: %w", err)
	}
	path := filepath.Join(dir, globalConfigFile)
	return os.WriteFile(path, data, 0600)
}
