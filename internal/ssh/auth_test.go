package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandHome("~/test/path")
	expected := filepath.Join(home, "test/path")
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	result2 := ExpandHome("/absolute/path")
	if result2 != "/absolute/path" {
		t.Fatalf("expected /absolute/path, got %s", result2)
	}
}
