package config

import (
	"testing"
)

func TestLoadEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(cfg.Connections))
	}
}

func TestSaveAndLoadConnection(t *testing.T) {
	dir := t.TempDir()
	cfg := &HangarConfig{
		Connections: []Connection{
			{
				Name: "test-server",
				Host: "192.168.1.1",
				Port: 22,
				User: "admin",
				Tags: []string{"staging"},
			},
		},
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(loaded.Connections))
	}
	c := loaded.Connections[0]
	if c.Name != "test-server" || c.Host != "192.168.1.1" || c.Port != 22 {
		t.Fatalf("connection mismatch: %+v", c)
	}
	if len(c.Tags) != 1 || c.Tags[0] != "staging" {
		t.Fatalf("tags mismatch: %v", c.Tags)
	}
}

func TestConnectionNameUniqueness(t *testing.T) {
	dir := t.TempDir()
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{Name: "server-1", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	err := Save(dir, cfg)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
}

func TestAddConnection(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	conn := Connection{Name: "new-server", Host: "10.0.0.5", Port: 22, User: "deploy"}
	err := cfg.Add(conn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}

	// Adding duplicate should fail
	err = cfg.Add(conn)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestRemoveConnection(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{Name: "server-2", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	err := cfg.Remove("server-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	if cfg.Connections[0].Name != "server-2" {
		t.Fatalf("wrong connection remaining: %s", cfg.Connections[0].Name)
	}

	err = cfg.Remove("server-99")
	if err == nil {
		t.Fatal("expected error for non-existent connection")
	}
}

func TestFilterByTag(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "prod-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"production", "api"}},
			{Name: "staging-1", Host: "10.0.0.2", Port: 22, User: "root", Tags: []string{"staging", "api"}},
			{Name: "prod-2", Host: "10.0.0.3", Port: 22, User: "root", Tags: []string{"production", "db"}},
		},
	}
	results := cfg.FilterByTag("production")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	results = cfg.FilterByTag("api")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	results = cfg.FilterByTag("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFindByName(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "prod-1", Host: "10.0.0.1", Port: 22, User: "root"},
		},
	}
	c, err := cfg.FindByName("prod-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Host != "10.0.0.1" {
		t.Fatalf("wrong host: %s", c.Host)
	}
	_, err = cfg.FindByName("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent name")
	}
}

func TestAddTags(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"staging"}},
		},
	}
	err := cfg.AddTags("server-1", []string{"api", "web"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, _ := cfg.FindByName("server-1")
	if len(c.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(c.Tags), c.Tags)
	}

	// Adding existing tag should not duplicate
	err = cfg.AddTags("server-1", []string{"staging"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, _ = cfg.FindByName("server-1")
	if len(c.Tags) != 3 {
		t.Fatalf("expected 3 tags after duplicate add, got %d", len(c.Tags))
	}
}

func TestAddConnectionValidation(t *testing.T) {
	cfg := &HangarConfig{}

	// Empty name
	err := cfg.Add(Connection{Name: "", Host: "10.0.0.1", Port: 22, User: "root"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	// Empty host
	err = cfg.Add(Connection{Name: "test", Host: "", Port: 22, User: "root"})
	if err == nil {
		t.Fatal("expected error for empty host")
	}

	// Empty user
	err = cfg.Add(Connection{Name: "test", Host: "10.0.0.1", Port: 22, User: ""})
	if err == nil {
		t.Fatal("expected error for empty user")
	}

	// Invalid port
	err = cfg.Add(Connection{Name: "test", Host: "10.0.0.1", Port: 0, User: "root"})
	if err == nil {
		t.Fatal("expected error for zero port")
	}

	// Negative port
	err = cfg.Add(Connection{Name: "test", Host: "10.0.0.1", Port: -1, User: "root"})
	if err == nil {
		t.Fatal("expected error for negative port")
	}
}

func TestRemoveTags(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"staging", "api", "web"}},
		},
	}
	err := cfg.RemoveTags("server-1", []string{"api", "web"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, _ := cfg.FindByName("server-1")
	if len(c.Tags) != 1 || c.Tags[0] != "staging" {
		t.Fatalf("expected [staging], got %v", c.Tags)
	}
}
