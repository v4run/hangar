package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStateMissingFile(t *testing.T) {
	st, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(st.CollapsedGroups) != 0 {
		t.Fatalf("expected no collapsed groups, got %v", st.CollapsedGroups)
	}
}

func TestSaveLoadStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := SaveState(dir, &UIState{CollapsedGroups: []string{"prod", "staging"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state.yaml")); err != nil {
		t.Fatalf("state.yaml not written: %v", err)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.CollapsedGroups) != 2 || st.CollapsedGroups[0] != "prod" || st.CollapsedGroups[1] != "staging" {
		t.Fatalf("unexpected collapsed groups: %v", st.CollapsedGroups)
	}
}
