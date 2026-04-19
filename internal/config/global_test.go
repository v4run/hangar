package config

import (
	"testing"
)

func TestLoadDefaultGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	gc, err := LoadGlobal(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc.PrefixKey != "ctrl+a" {
		t.Fatalf("expected default prefix ctrl+a, got %s", gc.PrefixKey)
	}
	if gc.SSHConfigPath != "~/.ssh/config" {
		t.Fatalf("expected default ssh config path, got %s", gc.SSHConfigPath)
	}
	if !gc.AutoSync {
		t.Fatal("expected auto_sync to default true")
	}
}

func TestSaveAndLoadGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	gc := &GlobalConfig{
		PrefixKey:     "ctrl+b",
		SSHConfigPath: "/custom/path",
		AutoSync:      false,
	}
	if err := SaveGlobal(dir, gc); err != nil {
		t.Fatalf("save error: %v", err)
	}
	loaded, err := LoadGlobal(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.PrefixKey != "ctrl+b" {
		t.Fatalf("expected ctrl+b, got %s", loaded.PrefixKey)
	}
	if loaded.AutoSync {
		t.Fatal("expected auto_sync false")
	}
}
