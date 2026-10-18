package tui

import (
	"testing"

	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
)

// TestConnectionEditPersistsAllFields guards against the "edit
// silently wipes advanced settings" bug: the connection save path
// used to overwrite the newly-built conn.SSHOptions with the
// existing one, so any edit to FwdAgent / Compress / Alive / Extra
// / etc. was lost between save and reload.
func TestConnectionEditPersistsAllFields(t *testing.T) {
	id := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Connections: []config.Connection{
			{ID: id, Name: "web-1", Host: "x"},
		},
	}
	m := New(cfg, t.TempDir())
	// Walk straight into the edit palette for the connection.
	m.openEditConnectionPalette(&m.cfg.Connections[0])
	// Set Extra to a value.
	m.palette.fields[connFieldExtra].input.SetValue("TCPKeepAlive=yes")
	// Also something elsewhere to show we're not clobbering Name.
	m.palette.fields[connFieldName].input.SetValue("web-1")
	_, _ = m.saveConnection(id)
	// Reload roundtrip isn't necessary — saveConnection writes
	// in-place to m.cfg before Save. Check the in-memory struct.
	stored := m.cfg.Connections[0]
	if stored.SSHOptions == nil {
		t.Fatal("expected SSHOptions to be set after save, got nil")
	}
	v, ok := stored.SSHOptions.ExtraOptions["TCPKeepAlive"]
	if !ok || v != "yes" {
		t.Fatalf("expected Extra TCPKeepAlive=yes persisted, got %+v", stored.SSHOptions.ExtraOptions)
	}
}

// TestDatabaseEditPreservesUnexposedFields guards against the same
// shape on database edit — tags / notes / env / tunnel aren't on
// the DB palette, so they must survive an edit.
func TestDatabaseEditPreservesUnexposedFields(t *testing.T) {
	id := uuid.New()
	tunnel := uuid.New()
	cfg := &config.Config{
		Schema: config.SchemaVersion,
		Databases: []config.Database{
			{
				ID:        id,
				Name:      "cache",
				Engine:    "redis",
				Host:      "x",
				Tags:      []string{"prod", "shared"},
				Notes:     "edge-only",
				TunnelVia: tunnel,
				Env:       map[string]string{"TZ": "UTC"},
			},
		},
	}
	m := New(cfg, t.TempDir())
	m.openEditDatabasePalette(&m.cfg.Databases[0])
	// Change nothing, just save.
	_, _ = m.saveDatabase(id)
	stored := m.cfg.Databases[0]
	if len(stored.Tags) != 2 {
		t.Fatalf("tags lost: %+v", stored.Tags)
	}
	if stored.Notes != "edge-only" {
		t.Fatalf("notes lost: %q", stored.Notes)
	}
	if stored.TunnelVia != tunnel {
		t.Fatalf("tunnel lost: %v", stored.TunnelVia)
	}
	if stored.Env["TZ"] != "UTC" {
		t.Fatalf("env lost: %+v", stored.Env)
	}
}
