package config

import (
	"testing"

	"github.com/google/uuid"
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
	id := uuid.New()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: id, Name: "test-server", Host: "192.168.1.1", Port: 22, User: "admin", Tags: []string{"staging"}},
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
	if c.ID != id {
		t.Fatalf("UUID mismatch: expected %s, got %s", id, c.ID)
	}
	if c.Name != "test-server" || c.Host != "192.168.1.1" || c.Port != 22 {
		t.Fatalf("connection mismatch: %+v", c)
	}
}

func TestDuplicateNamesAllowed(t *testing.T) {
	dir := t.TempDir()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("duplicate names should be allowed: %v", err)
	}
}

func TestAddConnection(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	conn := Connection{Name: "new-server", Host: "10.0.0.5", Port: 22, User: "deploy"}
	if err := cfg.Add(conn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	if cfg.Connections[0].ID == uuid.Nil {
		t.Fatal("expected UUID to be auto-assigned on Add")
	}
	if err := cfg.Add(conn); err != nil {
		t.Fatalf("duplicate names should be allowed: %v", err)
	}
	if len(cfg.Connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(cfg.Connections))
	}
}

func TestRemoveConnection(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{ID: uuid.New(), Name: "server-2", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	if err := cfg.Remove("server-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	if cfg.Connections[0].Name != "server-2" {
		t.Fatalf("wrong connection remaining: %s", cfg.Connections[0].Name)
	}
	if err := cfg.Remove("server-99"); err == nil {
		t.Fatal("expected error for non-existent connection")
	}
}

func TestFindByName(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "prod-1", Host: "10.0.0.1", Port: 22, User: "root"},
		},
	}
	c, err := cfg.FindByName("prod-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Host != "10.0.0.1" {
		t.Fatalf("wrong host: %s", c.Host)
	}
	if _, err = cfg.FindByName("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent name")
	}
}

func TestFindByID(t *testing.T) {
	id := uuid.New()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: id, Name: "prod-1", Host: "10.0.0.1", Port: 22, User: "root"},
		},
	}
	c, err := cfg.FindByID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Host != "10.0.0.1" {
		t.Fatalf("wrong host: %s", c.Host)
	}
	if _, err = cfg.FindByID(uuid.New()); err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestRemoveByID(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: id1, Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{ID: id2, Name: "server-2", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	if err := cfg.RemoveByID(id1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
}

func TestAddAutoAssignsUUID(t *testing.T) {
	cfg := &HangarConfig{}
	conn := Connection{Name: "new-server", Host: "10.0.0.5", Port: 22, User: "deploy"}
	if err := cfg.Add(conn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Connections[0].ID == uuid.Nil {
		t.Fatal("expected UUID to be auto-assigned")
	}
}

func TestMigrateAssignsUUIDs(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{Name: "server-2", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	if !cfg.Migrate() {
		t.Fatal("expected migration to report changes")
	}
	for _, c := range cfg.Connections {
		if c.ID == uuid.Nil {
			t.Fatalf("connection %s should have a UUID", c.Name)
		}
	}
	if cfg.Migrate() {
		t.Fatal("second migration should be a no-op")
	}
}

func TestMigrateResolvesJumpHostNames(t *testing.T) {
	bastionID := uuid.New()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: bastionID, Name: "bastion", Host: "10.0.0.1", Port: 22, User: "root"},
			{Name: "target", Host: "10.0.0.2", Port: 22, User: "root", JumpHost: "bastion"},
		},
	}
	cfg.Migrate()
	target, _ := cfg.FindByName("target")
	if target.JumpHost != bastionID.String() {
		t.Fatalf("expected JumpHost to be UUID %s, got %s", bastionID, target.JumpHost)
	}
}

func TestFilterByTag(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "prod-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"production", "api"}},
			{ID: uuid.New(), Name: "staging-1", Host: "10.0.0.2", Port: 22, User: "root", Tags: []string{"staging", "api"}},
			{ID: uuid.New(), Name: "prod-2", Host: "10.0.0.3", Port: 22, User: "root", Tags: []string{"production", "db"}},
		},
	}
	if results := cfg.FilterByTag("production"); len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results := cfg.FilterByTag("nonexistent"); len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestAddTags(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"staging"}},
		},
	}
	if err := cfg.AddTags("server-1", []string{"api", "web"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, _ := cfg.FindByName("server-1")
	if len(c.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(c.Tags))
	}
}

func TestRemoveTags(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root", Tags: []string{"staging", "api", "web"}},
		},
	}
	if err := cfg.RemoveTags("server-1", []string{"api", "web"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, _ := cfg.FindByName("server-1")
	if len(c.Tags) != 1 || c.Tags[0] != "staging" {
		t.Fatalf("expected [staging], got %v", c.Tags)
	}
}

func TestAddConnectionValidation(t *testing.T) {
	cfg := &HangarConfig{}
	if err := cfg.Add(Connection{Name: "", Host: "10.0.0.1", Port: 22, User: "root"}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := cfg.Add(Connection{Name: "test", Host: "", Port: 22, User: "root"}); err == nil {
		t.Fatal("expected error for empty host")
	}
	if err := cfg.Add(Connection{Name: "test", Host: "10.0.0.1", Port: 22, User: ""}); err == nil {
		t.Fatal("expected error for empty user")
	}
	if err := cfg.Add(Connection{Name: "test", Host: "10.0.0.1", Port: 0, User: "root"}); err == nil {
		t.Fatal("expected error for zero port")
	}
}

func TestMigrateBackfillsGroups(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "a", Host: "h", Port: 22, User: "u", Group: "prod"},
			{ID: uuid.New(), Name: "b", Host: "h", Port: 22, User: "u", Group: "dev"},
			{ID: uuid.New(), Name: "c", Host: "h", Port: 22, User: "u", Group: "prod"},
		},
	}
	if !cfg.Migrate() {
		t.Fatal("expected Migrate to report changes")
	}
	want := []string{"dev", "prod"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len: got %d, want %d", len(cfg.Groups), len(want))
	}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}

func TestMigrateAppendsOrphanGroup(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "a", Host: "h", Port: 22, User: "u", Group: "newgroup"},
		},
		Groups: GroupList{"existing"},
	}
	if !cfg.Migrate() {
		t.Fatal("expected Migrate to report changes")
	}
	want := []string{"existing", "newgroup"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len: got %d, want %d (Groups=%v)", len(cfg.Groups), len(want), cfg.Groups)
	}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}

func TestMigrateIdempotentOnHealthyConfig(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "a", Host: "h", Port: 22, User: "u", Group: "prod"},
		},
		Groups: GroupList{"prod", "dev"},
	}
	if cfg.Migrate() {
		t.Fatal("expected Migrate to be a no-op on healthy config")
	}
	want := []string{"prod", "dev"}
	if len(cfg.Groups) != len(want) {
		t.Fatalf("len changed: got %d, want %d", len(cfg.Groups), len(want))
	}
	for i := range want {
		if cfg.Groups[i] != want[i] {
			t.Fatalf("Groups[%d]: got %q, want %q", i, cfg.Groups[i], want[i])
		}
	}
}

func TestMigrateNoOpOnEmptyConfig(t *testing.T) {
	cfg := &HangarConfig{}
	if cfg.Migrate() {
		t.Fatal("expected Migrate to be a no-op on empty config")
	}
	if len(cfg.Groups) != 0 {
		t.Fatalf("expected empty Groups, got %d entries", len(cfg.Groups))
	}
}
