package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const stateFile = "state.yaml"

// UIState holds TUI state that should survive restarts but is neither
// user-edited settings (config.yaml) nor synced data (connections.yaml).
type UIState struct {
	// CollapsedGroups lists sidebar groups the user has collapsed. Groups
	// not listed are expanded, so a missing file means "all expanded".
	CollapsedGroups []string `yaml:"collapsed_groups,omitempty"`
}

func LoadState(dir string) (*UIState, error) {
	path := filepath.Join(dir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UIState{}, nil
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}
	var st UIState
	if err := yaml.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &st, nil
}

func SaveState(dir string, st *UIState) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	path := filepath.Join(dir, stateFile)
	return os.WriteFile(path, data, 0600)
}
