// internal/fleet/executor_test.go
package fleet

import (
	"testing"

	"github.com/v4run/hangar/internal/config"
)

func TestColorAssignment(t *testing.T) {
	servers := []string{"prod-1", "prod-2", "staging-1"}
	colors := AssignColors(servers)
	if len(colors) != 3 {
		t.Fatalf("expected 3 colors, got %d", len(colors))
	}
	seen := make(map[string]bool)
	for _, c := range colors {
		seen[c] = true
	}
	if len(seen) < 2 {
		t.Fatal("expected at least 2 distinct colors")
	}
}

func TestResolveTargets(t *testing.T) {
	cfg := &config.HangarConfig{
		Connections: []config.Connection{
			{Name: "prod-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"production"}},
			{Name: "prod-2", Host: "10.0.0.2", Port: 22, User: "root", Tags: []string{"production"}},
			{Name: "staging", Host: "10.0.0.3", Port: 22, User: "root", Tags: []string{"staging"}},
		},
	}

	// By tag
	targets := ResolveTargets(cfg, "production", nil)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	// By name
	targets = ResolveTargets(cfg, "", []string{"prod-1", "staging"})
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	// Both
	targets = ResolveTargets(cfg, "production", []string{"staging"})
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
}

func TestFormatLine(t *testing.T) {
	line := FormatLine("prod-1", "\033[31m", "some output", true)
	if line == "" {
		t.Fatal("expected non-empty formatted line")
	}

	// Without border
	line2 := FormatLine("prod-1", "\033[31m", "some output", false)
	if line2 != "some output" {
		t.Fatalf("expected raw output without border, got: %s", line2)
	}
}
