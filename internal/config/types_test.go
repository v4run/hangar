package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGroupListUnmarshalSequence(t *testing.T) {
	const data = `
connections: []
groups:
  - prod
  - staging
  - dev
`
	var cfg HangarConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := GroupList{"prod", "staging", "dev"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len: got %d, want %d", len(cfg.Groups), len(want))
	}
	for i, g := range want {
		if cfg.Groups[i] != g {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], g)
		}
	}
}

func TestGroupListUnmarshalLegacyMap(t *testing.T) {
	const data = `
connections: []
groups:
  prod: true
  staging: true
  dev: true
`
	var cfg HangarConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Groups) != 3 {
		t.Fatalf("len: got %d, want 3", len(cfg.Groups))
	}
	want := []string{"dev", "prod", "staging"}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}

func TestGroupListUnmarshalInvalidKind(t *testing.T) {
	const data = `
connections: []
groups: "not-a-list"
`
	var cfg HangarConfig
	err := yaml.Unmarshal([]byte(data), &cfg)
	if err == nil {
		t.Fatalf("expected error for scalar groups value, got nil")
	}
}
