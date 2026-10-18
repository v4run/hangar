package config

import (
	"testing"

	"github.com/google/uuid"
)

func TestResolveFlat(t *testing.T) {
	g := uuid.New()
	cfg := &Config{
		Schema: SchemaVersion,
		Groups: []Group{
			{ID: g, Name: "prod", User: "deploy", IdentityFile: "~/.ssh/prod"},
		},
		Connections: []Connection{
			{ID: uuid.New(), Name: "web", Host: "web-1", Group: g},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	r := Resolve(cfg, &cfg.Connections[0])
	if r.User != "deploy" {
		t.Fatalf("user: %q", r.User)
	}
	if r.FieldScope["User"] != ScopeGroup {
		t.Fatalf("expected ScopeGroup for User, got %v", r.FieldScope["User"])
	}
	if r.FieldOrigin["User"] != "prod" {
		t.Fatalf("origin: %q", r.FieldOrigin["User"])
	}
}

func TestResolveNested(t *testing.T) {
	prod := uuid.New()
	east := uuid.New()
	web := uuid.New()
	cfg := &Config{
		Schema: SchemaVersion,
		Groups: []Group{
			{ID: prod, Name: "prod", User: "deploy"},
			{ID: east, Name: "us-east", Parent: prod, SSHOptions: &SSHOptions{LocalForward: []string{"5432:localhost:5432"}}},
			{ID: web, Name: "web", Parent: east, IdentityFile: "~/.ssh/web"},
		},
		Connections: []Connection{
			{ID: uuid.New(), Name: "web-1", Host: "10.0.0.1", Group: web},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	r := Resolve(cfg, &cfg.Connections[0])
	if r.User != "deploy" {
		t.Fatalf("user: %q", r.User)
	}
	if r.FieldOrigin["User"] != "prod" {
		t.Fatalf("origin User: %q", r.FieldOrigin["User"])
	}
	if r.IdentityFile != "~/.ssh/web" {
		t.Fatalf("key: %q", r.IdentityFile)
	}
	if r.FieldOrigin["IdentityFile"] != "prod/us-east/web" {
		t.Fatalf("origin key: %q", r.FieldOrigin["IdentityFile"])
	}
	if r.SSHOptions == nil || len(r.SSHOptions.LocalForward) != 1 || r.SSHOptions.LocalForward[0] != "5432:localhost:5432" {
		t.Fatalf("local forward: %+v", r.SSHOptions)
	}
}

func TestResolveConnOverridesGroup(t *testing.T) {
	g := uuid.New()
	cfg := &Config{
		Schema: SchemaVersion,
		Groups: []Group{{ID: g, Name: "prod", User: "deploy"}},
		Connections: []Connection{
			{ID: uuid.New(), Name: "web", Host: "x", Group: g, User: "root"},
		},
	}
	r := Resolve(cfg, &cfg.Connections[0])
	if r.User != "root" {
		t.Fatalf("user: %q", r.User)
	}
	if r.FieldScope["User"] != ScopeOwn {
		t.Fatalf("scope: %v", r.FieldScope["User"])
	}
}

func TestValidateDetectsCycle(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	cfg := &Config{
		Schema: SchemaVersion,
		Groups: []Group{
			{ID: a, Name: "a", Parent: b},
			{ID: b, Name: "b", Parent: a},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateDetectsOrphan(t *testing.T) {
	cfg := &Config{
		Schema: SchemaVersion,
		Groups: []Group{
			{ID: uuid.New(), Name: "a", Parent: uuid.New()},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected orphan error")
	}
}

func TestValidateDuplicateSiblingNames(t *testing.T) {
	p := uuid.New()
	cfg := &Config{
		Schema: SchemaVersion,
		Groups: []Group{
			{ID: p, Name: "p"},
			{ID: uuid.New(), Name: "k", Parent: p},
			{ID: uuid.New(), Name: "k", Parent: p},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected sibling duplicate")
	}
}

func TestMigrateV1(t *testing.T) {
	v1 := &v1Config{
		Groups:          []string{"prod", "staging"},
		GroupShellInit:  map[string]string{"prod": "alias l='ls -la'"},
		GlobalShellInit: "echo welcome",
		Connections: []v1Connection{
			{ID: uuid.New(), Name: "web-1", Host: "10.0.0.1", Group: "prod", User: "deploy"},
		},
	}
	cfg := migrateV1(v1)
	if cfg.Schema != SchemaVersion {
		t.Fatalf("schema: %d", cfg.Schema)
	}
	if len(cfg.Groups) != 2 {
		t.Fatalf("groups: %+v", cfg.Groups)
	}
	// find prod
	var prod *Group
	for i := range cfg.Groups {
		if cfg.Groups[i].Name == "prod" {
			prod = &cfg.Groups[i]
		}
	}
	if prod == nil || prod.ID == uuid.Nil {
		t.Fatalf("prod missing or missing id")
	}
	if prod.ShellInit != "alias l='ls -la'" {
		t.Fatalf("group shell init: %q", prod.ShellInit)
	}
	if cfg.Connections[0].Group != prod.ID {
		t.Fatalf("connection group not rewired to UUID: %v vs %v", cfg.Connections[0].Group, prod.ID)
	}
	if cfg.ShellInit != "echo welcome" {
		t.Fatalf("global init: %q", cfg.ShellInit)
	}
}
