# Bugs & Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix 8 bugs/features: UUID-based connection IDs, editable group/connection names, copy/paste keybinding changes, env vars, advanced SSH options, global settings, duplicate names, paste-into-fields, and JumpHost autocomplete.

**Architecture:** Add `uuid.UUID` as the internal connection identifier. Introduce a shared `SSHOptions` struct used by both `Connection` (per-connection overrides) and `GlobalConfig` (defaults). Merge options at SSH command build time. Refactor TUI to use UUID-keyed maps and add new form modes for group editing and global settings.

**Tech Stack:** Go, Bubbletea, lipgloss, github.com/google/uuid, gopkg.in/yaml.v3

---

### Task 1: Add UUID dependency and update data model types

**Files:**
- Modify: `internal/config/types.go`
- Modify: `go.mod` / `go.sum`

**Step 1: Add the uuid dependency**

Run: `cd /Users/varun/projects/personal/hangar && go get github.com/google/uuid`

**Step 2: Update types.go with UUID, SSHOptions, and updated structs**

Replace the entire contents of `internal/config/types.go` with:

```go
package config

import (
	"time"

	"github.com/google/uuid"
)

type Script struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

type SSHOptions struct {
	ForwardAgent        *bool             `yaml:"forward_agent,omitempty"`
	LocalForward        []string          `yaml:"local_forward,omitempty"`
	RemoteForward       []string          `yaml:"remote_forward,omitempty"`
	ServerAliveInterval *int              `yaml:"server_alive_interval,omitempty"`
	ServerAliveCountMax *int              `yaml:"server_alive_count_max,omitempty"`
	StrictHostKeyCheck  string            `yaml:"strict_host_key_checking,omitempty"`
	RequestTTY          string            `yaml:"request_tty,omitempty"`
	Compression         *bool             `yaml:"compression,omitempty"`
	EnvVars             map[string]string `yaml:"env_vars,omitempty"`
	ExtraOptions        map[string]string `yaml:"extra_options,omitempty"`
}

type Connection struct {
	ID                  uuid.UUID   `yaml:"id"`
	Name                string      `yaml:"name"`
	Host                string      `yaml:"host"`
	Port                int         `yaml:"port"`
	User                string      `yaml:"user"`
	IdentityFile        string      `yaml:"identity_file,omitempty"`
	Tags                []string    `yaml:"tags,omitempty"`
	JumpHost            string      `yaml:"jump_host,omitempty"`
	Group               string      `yaml:"group,omitempty"`
	SyncedFromSSHConfig bool        `yaml:"synced_from_ssh_config"`
	Scripts             []Script    `yaml:"scripts,omitempty"`
	Notes               string      `yaml:"notes,omitempty"`
	SSHOptions          *SSHOptions `yaml:"ssh_options,omitempty"`
	UseGlobalSettings   *bool       `yaml:"use_global_settings,omitempty"`
}

type SSHSync struct {
	LastSync          time.Time `yaml:"last_sync,omitempty"`
	LastSSHConfigHash string    `yaml:"last_ssh_config_hash,omitempty"`
}

type HangarConfig struct {
	Connections   []Connection    `yaml:"connections"`
	SSHSync       SSHSync         `yaml:"ssh_sync"`
	GlobalScripts []Script        `yaml:"global_scripts,omitempty"`
	Groups        map[string]bool `yaml:"groups,omitempty"`
}

type GlobalConfig struct {
	PrefixKey     string      `yaml:"prefix_key"`
	SSHConfigPath string      `yaml:"ssh_config_path"`
	AutoSync      bool        `yaml:"auto_sync"`
	SSHOptions    *SSHOptions `yaml:"ssh_options,omitempty"`
}
```

**Step 3: Verify it compiles (will have errors in other files — that's expected)**

Run: `cd /Users/varun/projects/personal/hangar && go vet ./internal/config/...`
Expected: Success (types.go is self-contained within the package)

**Step 4: Commit**

```bash
git add internal/config/types.go go.mod go.sum
git commit -m "feat: add UUID and SSHOptions to data model types"
```

---

### Task 2: Update config.go for UUID-based operations and migration

**Files:**
- Modify: `internal/config/config.go:1-139`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing tests for new UUID behavior**

Add these tests to the END of `internal/config/config_test.go`:

```go
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
	_, err = cfg.FindByID(uuid.New())
	if err == nil {
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
	err := cfg.RemoveByID(id1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	if cfg.Connections[0].Name != "server-2" {
		t.Fatalf("wrong connection remaining: %s", cfg.Connections[0].Name)
	}
}

func TestDuplicateNamesAllowed(t *testing.T) {
	dir := t.TempDir()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "server", Host: "10.0.0.1", Port: 22, User: "root"},
			{ID: uuid.New(), Name: "server", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	err := Save(dir, cfg)
	if err != nil {
		t.Fatalf("duplicate names should be allowed: %v", err)
	}
}

func TestAddAutoAssignsUUID(t *testing.T) {
	cfg := &HangarConfig{}
	conn := Connection{Name: "new-server", Host: "10.0.0.5", Port: 22, User: "deploy"}
	err := cfg.Add(conn)
	if err != nil {
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
	changed := cfg.Migrate()
	if !changed {
		t.Fatal("expected migration to report changes")
	}
	for _, c := range cfg.Connections {
		if c.ID == uuid.Nil {
			t.Fatalf("connection %s should have a UUID", c.Name)
		}
	}
	// Second call should be a no-op
	changed = cfg.Migrate()
	if changed {
		t.Fatal("second migration should be a no-op")
	}
}

func TestMigrateResolvesJumpHostNames(t *testing.T) {
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "bastion", Host: "10.0.0.1", Port: 22, User: "root"},
			{Name: "target", Host: "10.0.0.2", Port: 22, User: "root", JumpHost: "bastion"},
		},
	}
	bastionID := cfg.Connections[0].ID
	cfg.Migrate()
	target, _ := cfg.FindByName("target")
	if target.JumpHost != bastionID.String() {
		t.Fatalf("expected JumpHost to be UUID %s, got %s", bastionID, target.JumpHost)
	}
}
```

Also add the uuid import to the test file:

```go
import (
	"testing"

	"github.com/google/uuid"
)
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/config/ -run "TestFindByID|TestRemoveByID|TestDuplicateNamesAllowed|TestAddAutoAssignsUUID|TestMigrate" -v`
Expected: FAIL — FindByID, RemoveByID, Migrate methods don't exist

**Step 3: Implement UUID-based operations in config.go**

Replace `internal/config/config.go` with:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const connectionsFile = "connections.yaml"

func Load(dir string) (*HangarConfig, error) {
	path := filepath.Join(dir, connectionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &HangarConfig{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg HangarConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Migrate() {
		_ = Save(dir, &cfg)
	}
	return &cfg, nil
}

func Save(dir string, cfg *HangarConfig) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	path := filepath.Join(dir, connectionsFile)
	return os.WriteFile(path, data, 0600)
}

func (cfg *HangarConfig) Add(conn Connection) error {
	if conn.Name == "" {
		return fmt.Errorf("connection name is required")
	}
	if conn.Host == "" {
		return fmt.Errorf("host is required for connection %q", conn.Name)
	}
	if conn.User == "" {
		return fmt.Errorf("user is required for connection %q", conn.Name)
	}
	if conn.Port <= 0 {
		return fmt.Errorf("port must be positive for connection %q", conn.Name)
	}
	if conn.ID == uuid.Nil {
		conn.ID = uuid.New()
	}
	cfg.Connections = append(cfg.Connections, conn)
	return nil
}

func (cfg *HangarConfig) Remove(name string) error {
	for i, c := range cfg.Connections {
		if c.Name == name {
			cfg.Connections = append(cfg.Connections[:i], cfg.Connections[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("connection %q not found", name)
}

func (cfg *HangarConfig) RemoveByID(id uuid.UUID) error {
	for i, c := range cfg.Connections {
		if c.ID == id {
			cfg.Connections = append(cfg.Connections[:i], cfg.Connections[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("connection with ID %s not found", id)
}

func (cfg *HangarConfig) FindByName(name string) (*Connection, error) {
	for i := range cfg.Connections {
		if cfg.Connections[i].Name == name {
			return &cfg.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection %q not found", name)
}

func (cfg *HangarConfig) FindByID(id uuid.UUID) (*Connection, error) {
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == id {
			return &cfg.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection with ID %s not found", id)
}

func (cfg *HangarConfig) AddTags(name string, tags []string) error {
	c, err := cfg.FindByName(name)
	if err != nil {
		return err
	}
	existing := make(map[string]bool)
	for _, t := range c.Tags {
		existing[t] = true
	}
	for _, t := range tags {
		if !existing[t] {
			c.Tags = append(c.Tags, t)
			existing[t] = true
		}
	}
	return nil
}

func (cfg *HangarConfig) RemoveTags(name string, tags []string) error {
	c, err := cfg.FindByName(name)
	if err != nil {
		return err
	}
	remove := make(map[string]bool)
	for _, t := range tags {
		remove[t] = true
	}
	filtered := c.Tags[:0]
	for _, t := range c.Tags {
		if !remove[t] {
			filtered = append(filtered, t)
		}
	}
	c.Tags = filtered
	return nil
}

func (cfg *HangarConfig) FilterByTag(tag string) []Connection {
	var results []Connection
	for _, c := range cfg.Connections {
		for _, t := range c.Tags {
			if t == tag {
				results = append(results, c)
				break
			}
		}
	}
	return results
}

// Migrate assigns UUIDs to connections that don't have one and resolves
// name-based JumpHost references to UUIDs where possible. Returns true
// if any changes were made.
func (cfg *HangarConfig) Migrate() bool {
	changed := false
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == uuid.Nil {
			cfg.Connections[i].ID = uuid.New()
			changed = true
		}
	}
	// Resolve name-based JumpHost references to UUIDs
	for i := range cfg.Connections {
		jh := cfg.Connections[i].JumpHost
		if jh == "" {
			continue
		}
		// Already a UUID?
		if _, err := uuid.Parse(jh); err == nil {
			continue
		}
		// Try to resolve as a connection name
		if target, err := cfg.FindByName(jh); err == nil {
			cfg.Connections[i].JumpHost = target.ID.String()
			changed = true
		}
	}
	return changed
}
```

**Step 4: Update existing tests that check for duplicate name errors**

In `config_test.go`, update `TestConnectionNameUniqueness` — duplicate names are now ALLOWED:

```go
func TestConnectionNameUniqueness(t *testing.T) {
	dir := t.TempDir()
	cfg := &HangarConfig{
		Connections: []Connection{
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.1", Port: 22, User: "root"},
			{ID: uuid.New(), Name: "server-1", Host: "10.0.0.2", Port: 22, User: "root"},
		},
	}
	err := Save(dir, cfg)
	if err != nil {
		t.Fatalf("duplicate names should now be allowed: %v", err)
	}
}
```

Update `TestAddConnection` — remove the duplicate name check:

```go
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
	if cfg.Connections[0].ID == uuid.Nil {
		t.Fatal("expected UUID to be auto-assigned on Add")
	}

	// Adding same name should succeed (duplicates allowed)
	err = cfg.Add(conn)
	if err != nil {
		t.Fatalf("duplicate names should be allowed: %v", err)
	}
	if len(cfg.Connections) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(cfg.Connections))
	}
}
```

Update `TestSaveAndLoadConnection` to include UUID:

```go
func TestSaveAndLoadConnection(t *testing.T) {
	dir := t.TempDir()
	id := uuid.New()
	cfg := &HangarConfig{
		Connections: []Connection{
			{
				ID:   id,
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
	if c.ID != id {
		t.Fatalf("UUID mismatch: expected %s, got %s", id, c.ID)
	}
	if c.Name != "test-server" || c.Host != "192.168.1.1" || c.Port != 22 {
		t.Fatalf("connection mismatch: %+v", c)
	}
}
```

**Step 5: Run tests to verify they pass**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/config/ -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: UUID-based connection identity with migration"
```

---

### Task 3: Update keychain for UUID-based keys

**Files:**
- Modify: `internal/config/keychain.go:1-21`
- Modify: `internal/config/keychain_test.go`

**Step 1: Write failing test**

Replace `internal/config/keychain_test.go` with:

```go
package config

import (
	"testing"

	"github.com/google/uuid"
)

func TestKeychainKeyFormat(t *testing.T) {
	key := KeychainKey("prod-api-1")
	if key != "hangar:prod-api-1" {
		t.Fatalf("expected hangar:prod-api-1, got %s", key)
	}
}

func TestKeychainKeyFromUUID(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	key := KeychainKey(id.String())
	if key != "hangar:550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected hangar:<uuid>, got %s", key)
	}
}
```

**Step 2: Run test to verify it passes (KeychainKey already accepts string)**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/config/ -run TestKeychainKey -v`
Expected: PASS (no changes needed to keychain.go for now — the callers will change from passing `conn.Name` to passing `conn.ID.String()`)

**Step 3: Commit**

```bash
git add internal/config/keychain_test.go
git commit -m "test: verify keychain works with UUID-based keys"
```

---

### Task 4: SSHOptions merge logic and updated SSH command building

**Files:**
- Create: `internal/config/merge.go`
- Create: `internal/config/merge_test.go`
- Modify: `internal/ssh/session.go:1-69`
- Modify: `internal/ssh/session_test.go`

**Step 1: Write failing tests for MergeSSHOptions**

Create `internal/config/merge_test.go`:

```go
package config

import "testing"

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestMergeSSHOptionsNilGlobal(t *testing.T) {
	local := &SSHOptions{ForwardAgent: boolPtr(true)}
	result := MergeSSHOptions(nil, local)
	if result.ForwardAgent == nil || !*result.ForwardAgent {
		t.Fatal("expected ForwardAgent=true from local")
	}
}

func TestMergeSSHOptionsNilLocal(t *testing.T) {
	global := &SSHOptions{Compression: boolPtr(true)}
	result := MergeSSHOptions(global, nil)
	if result.Compression == nil || !*result.Compression {
		t.Fatal("expected Compression=true from global")
	}
}

func TestMergeSSHOptionsBothNil(t *testing.T) {
	result := MergeSSHOptions(nil, nil)
	if result.ForwardAgent != nil || result.Compression != nil {
		t.Fatal("expected all nil fields")
	}
}

func TestMergeSSHOptionsLocalOverridesGlobal(t *testing.T) {
	global := &SSHOptions{
		ForwardAgent:        boolPtr(true),
		Compression:         boolPtr(false),
		ServerAliveInterval: intPtr(30),
		StrictHostKeyCheck:  "yes",
		LocalForward:        []string{"8080:localhost:80"},
		EnvVars:             map[string]string{"ENV": "prod"},
		ExtraOptions:        map[string]string{"TCPKeepAlive": "yes"},
	}
	local := &SSHOptions{
		ForwardAgent:       boolPtr(false),
		Compression:        boolPtr(true),
		StrictHostKeyCheck: "no",
		LocalForward:       []string{"9090:localhost:90"},
		EnvVars:            map[string]string{"APP": "myapp"},
	}
	result := MergeSSHOptions(global, local)
	if *result.ForwardAgent != false {
		t.Fatal("local should override ForwardAgent")
	}
	if *result.Compression != true {
		t.Fatal("local should override Compression")
	}
	if *result.ServerAliveInterval != 30 {
		t.Fatal("global ServerAliveInterval should carry through")
	}
	if result.StrictHostKeyCheck != "no" {
		t.Fatal("local should override StrictHostKeyCheck")
	}
	// LocalForward: local replaces global entirely
	if len(result.LocalForward) != 1 || result.LocalForward[0] != "9090:localhost:90" {
		t.Fatalf("expected local forwards only, got %v", result.LocalForward)
	}
	// EnvVars: merged, local wins
	if result.EnvVars["ENV"] != "prod" {
		t.Fatal("global ENV should carry through")
	}
	if result.EnvVars["APP"] != "myapp" {
		t.Fatal("local APP should be present")
	}
	// ExtraOptions: global carries through
	if result.ExtraOptions["TCPKeepAlive"] != "yes" {
		t.Fatal("global ExtraOptions should carry through")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/config/ -run TestMergeSSHOptions -v`
Expected: FAIL — MergeSSHOptions doesn't exist

**Step 3: Implement MergeSSHOptions**

Create `internal/config/merge.go`:

```go
package config

// MergeSSHOptions merges global and local SSHOptions. Local values override
// global values. For maps (EnvVars, ExtraOptions), entries are merged with
// local keys winning. For slices (LocalForward, RemoteForward), local
// replaces global entirely if non-nil.
func MergeSSHOptions(global, local *SSHOptions) SSHOptions {
	if global == nil && local == nil {
		return SSHOptions{}
	}
	if global == nil {
		return *local
	}
	if local == nil {
		return *global
	}

	result := *global

	if local.ForwardAgent != nil {
		result.ForwardAgent = local.ForwardAgent
	}
	if local.Compression != nil {
		result.Compression = local.Compression
	}
	if local.ServerAliveInterval != nil {
		result.ServerAliveInterval = local.ServerAliveInterval
	}
	if local.ServerAliveCountMax != nil {
		result.ServerAliveCountMax = local.ServerAliveCountMax
	}
	if local.StrictHostKeyCheck != "" {
		result.StrictHostKeyCheck = local.StrictHostKeyCheck
	}
	if local.RequestTTY != "" {
		result.RequestTTY = local.RequestTTY
	}
	if local.LocalForward != nil {
		result.LocalForward = local.LocalForward
	}
	if local.RemoteForward != nil {
		result.RemoteForward = local.RemoteForward
	}

	// Merge maps: global as base, local overrides
	result.EnvVars = mergeMaps(global.EnvVars, local.EnvVars)
	result.ExtraOptions = mergeMaps(global.ExtraOptions, local.ExtraOptions)

	return result
}

func mergeMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := make(map[string]string)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
```

**Step 4: Run merge tests**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/config/ -run TestMergeSSHOptions -v`
Expected: ALL PASS

**Step 5: Write failing tests for updated BuildSSHArgs**

Add to `internal/ssh/session_test.go`:

```go
func TestBuildSSHArgsWithSSHOptions(t *testing.T) {
	conn := &config.Connection{
		ID:   uuid.New(),
		Name: "test",
		Host: "10.0.0.1",
		Port: 22,
		User: "root",
	}
	opts := config.SSHOptions{
		ForwardAgent:        boolPtr(true),
		Compression:         boolPtr(true),
		ServerAliveInterval: intPtr(30),
		StrictHostKeyCheck:  "accept-new",
		LocalForward:        []string{"8080:localhost:80", "9090:localhost:90"},
		RemoteForward:       []string{"3000:localhost:3000"},
		EnvVars:             map[string]string{"MY_VAR": "value"},
		ExtraOptions:        map[string]string{"TCPKeepAlive": "yes"},
	}

	args := BuildSSHArgs(conn, nil, &opts)

	argStr := fmt.Sprintf("%v", args)
	checks := map[string]bool{
		"-o ForwardAgent=yes":             false,
		"-o Compression=yes":              false,
		"-o ServerAliveInterval=30":       false,
		"-o StrictHostKeyChecking=accept-new": false,
		"-L 8080:localhost:80":            false,
		"-L 9090:localhost:90":            false,
		"-R 3000:localhost:3000":          false,
		"-o SendEnv=MY_VAR":              false,
		"-o TCPKeepAlive=yes":            false,
	}

	for i := 0; i < len(args)-1; i++ {
		pair := args[i] + " " + args[i+1]
		if _, ok := checks[pair]; ok {
			checks[pair] = true
		}
	}

	for check, found := range checks {
		if !found {
			t.Errorf("expected %q in args: %s", check, argStr)
		}
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }
```

Add `"fmt"` and `"github.com/google/uuid"` to session_test.go imports.

**Step 6: Run to verify failure**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/ssh/ -run TestBuildSSHArgsWithSSHOptions -v`
Expected: FAIL — BuildSSHArgs signature doesn't accept SSHOptions

**Step 7: Update BuildSSHArgs in session.go**

Replace `internal/ssh/session.go` with:

```go
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

// ResolveJumpHost resolves a JumpHost value. If it's a valid UUID, looks up
// the connection by ID. Otherwise returns nil (caller should use raw value).
func ResolveJumpHost(cfg *config.HangarConfig, jumpHostVal string) *config.Connection {
	if jumpHostVal == "" {
		return nil
	}
	if id, err := uuid.Parse(jumpHostVal); err == nil {
		if jh, err := cfg.FindByID(id); err == nil {
			return jh
		}
	}
	// Also try by name for backward compatibility
	if jh, err := cfg.FindByName(jumpHostVal); err == nil {
		return jh
	}
	return nil
}

func BuildSSHArgs(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions) []string {
	args := []string{"-p", strconv.Itoa(conn.Port)}

	if conn.IdentityFile != "" {
		args = append(args, "-i", conn.IdentityFile)
	}

	if jumpHost != nil {
		jumpStr := fmt.Sprintf("%s@%s:%d", jumpHost.User, jumpHost.Host, jumpHost.Port)
		args = append(args, "-J", jumpStr)
	} else if conn.JumpHost != "" {
		// Raw JumpHost value (not a UUID reference)
		if _, err := uuid.Parse(conn.JumpHost); err != nil {
			args = append(args, "-J", conn.JumpHost)
		}
	}

	// Apply SSH options
	if opts != nil {
		if opts.ForwardAgent != nil {
			args = append(args, "-o", fmt.Sprintf("ForwardAgent=%s", boolToYesNo(*opts.ForwardAgent)))
		}
		if opts.Compression != nil {
			args = append(args, "-o", fmt.Sprintf("Compression=%s", boolToYesNo(*opts.Compression)))
		}
		if opts.ServerAliveInterval != nil {
			args = append(args, "-o", fmt.Sprintf("ServerAliveInterval=%d", *opts.ServerAliveInterval))
		}
		if opts.ServerAliveCountMax != nil {
			args = append(args, "-o", fmt.Sprintf("ServerAliveCountMax=%d", *opts.ServerAliveCountMax))
		}
		if opts.StrictHostKeyCheck != "" {
			args = append(args, "-o", fmt.Sprintf("StrictHostKeyChecking=%s", opts.StrictHostKeyCheck))
		}
		if opts.RequestTTY != "" {
			args = append(args, "-o", fmt.Sprintf("RequestTTY=%s", opts.RequestTTY))
		}
		for _, lf := range opts.LocalForward {
			args = append(args, "-L", lf)
		}
		for _, rf := range opts.RemoteForward {
			args = append(args, "-R", rf)
		}
		for key := range opts.EnvVars {
			args = append(args, "-o", fmt.Sprintf("SendEnv=%s", key))
		}
		for key, val := range opts.ExtraOptions {
			args = append(args, "-o", fmt.Sprintf("%s=%s", key, val))
		}
	}

	args = append(args, fmt.Sprintf("%s@%s", conn.User, conn.Host))
	return args
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func NewSSHCommand(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions) (*exec.Cmd, func()) {
	args := BuildSSHArgs(conn, jumpHost, opts)
	cmd := exec.Command("ssh", args...)

	// Set environment variables from SSHOptions
	if opts != nil && len(opts.EnvVars) > 0 {
		cmd.Env = append(os.Environ())
		for key, val := range opts.EnvVars {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	password, err := config.GetPassword(conn.ID.String())
	if err != nil || password == "" {
		// Fallback: try legacy name-based lookup
		password, err = config.GetPassword(conn.Name)
		if err != nil || password == "" {
			return cmd, func() {}
		}
	}

	tmpDir, err := os.MkdirTemp("", "hangar-askpass-*")
	if err != nil {
		return cmd, func() {}
	}

	scriptPath := filepath.Join(tmpDir, "askpass.sh")
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", password)
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		os.RemoveAll(tmpDir)
		return cmd, func() {}
	}

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		"SSH_ASKPASS="+scriptPath,
		"SSH_ASKPASS_REQUIRE=force",
	)

	return cmd, func() { os.RemoveAll(tmpDir) }
}

func Connect(conn *config.Connection, jumpHost *config.Connection, opts *config.SSHOptions) error {
	cmd, cleanup := NewSSHCommand(conn, jumpHost, opts)
	defer cleanup()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

**Step 8: Update existing session tests for new signature**

In `internal/ssh/session_test.go`, update existing tests to pass `nil` as the third arg:

- `TestBuildSSHArgs`: `BuildSSHArgs(conn, nil, nil)`
- `TestBuildSSHArgsWithJumpHost`: `BuildSSHArgs(conn, jump, nil)`
- `TestBuildSSHArgsDefaultPort`: `BuildSSHArgs(conn, nil, nil)`

Also add uuid import and give connections IDs:
```go
conn := &config.Connection{
    ID:   uuid.New(),
    Name: "test",
    ...
}
```

**Step 9: Run all SSH tests**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/ssh/ -v`
Expected: ALL PASS

**Step 10: Commit**

```bash
git add internal/config/merge.go internal/config/merge_test.go internal/ssh/session.go internal/ssh/session_test.go
git commit -m "feat: SSHOptions merge logic and updated SSH command building"
```

---

### Task 5: Update sync.go for UUID assignment

**Files:**
- Modify: `internal/config/sync.go:36-109` (ParseSSHConfig — assign UUIDs to parsed entries)
- Modify: `internal/config/sync_test.go`

**Step 1: Update ParseSSHConfig to assign UUIDs**

In `internal/config/sync.go`, add `"github.com/google/uuid"` to imports.

In `ParseSSHConfig`, when creating a new Connection (line 80), add UUID:

```go
current = &Connection{
    ID:                  uuid.New(),
    Name:                val,
    SyncedFromSSHConfig: true,
}
```

**Step 2: Update SyncFromSSHConfig to handle UUIDs**

In `SyncFromSSHConfig`, when adding a new entry (line 119), the UUID comes from the parsed entry. When updating existing entries (lines 129-134), don't touch the ID.

No code changes needed for the update path — it already modifies fields on the existing pointer.

**Step 3: Update sync_test.go**

Add `"github.com/google/uuid"` to imports.

In `TestSyncFromSSHConfig`, add UUID to the manually-added connection and add a UUID check:

```go
cfg.Add(Connection{Name: "my-server", Host: "10.0.0.99", Port: 22, User: "me"})
```

After sync, verify synced entries have UUIDs:

```go
c, _ := cfg.FindByName("prod-server")
if c.ID == uuid.Nil {
    t.Fatal("synced entry should have a UUID")
}
```

**Step 4: Run sync tests**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/config/ -run TestSync -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/config/sync.go internal/config/sync_test.go
git commit -m "feat: assign UUIDs to synced SSH config entries"
```

---

### Task 6: Update CLI commands for UUID support

**Files:**
- Modify: `internal/cli/connect.go:1-42`
- Modify: `internal/cli/add.go:1-62`
- Modify: `internal/cli/remove.go:1-29`
- Modify: `internal/cli/list.go:1-47`
- Modify: `internal/cli/cli_test.go`

**Step 1: Update connect.go**

The connect command resolves JumpHost and passes merged SSHOptions:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/v4run/hangar/internal/config"
	sshpkg "github.com/v4run/hangar/internal/ssh"
)

func newConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect to a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			gc, err := config.LoadGlobal(configDir())
			if err != nil {
				return err
			}
			conn, err := cfg.FindByName(args[0])
			if err != nil {
				return err
			}

			jumpHost := sshpkg.ResolveJumpHost(cfg, conn.JumpHost)

			// Merge SSH options
			var merged *config.SSHOptions
			useGlobal := conn.UseGlobalSettings == nil || *conn.UseGlobalSettings
			if useGlobal && gc.SSHOptions != nil {
				m := config.MergeSSHOptions(gc.SSHOptions, conn.SSHOptions)
				merged = &m
			} else if conn.SSHOptions != nil {
				merged = conn.SSHOptions
			}

			return sshpkg.Connect(conn, jumpHost, merged)
		},
	}

	return cmd
}
```

**Step 2: Update add.go — no duplicate check needed**

No changes needed to add.go logic — `cfg.Add()` no longer checks duplicates. Just verify it still works.

**Step 3: Update remove.go — still works by name (first match)**

No changes needed. `cfg.Remove()` still removes by name.

**Step 4: Update list.go — show ID in verbose mode (optional)**

No changes needed for basic functionality.

**Step 5: Update cli_test.go**

Update `TestAddDuplicate` — duplicates should now succeed:

```go
func TestAddDuplicate(t *testing.T) {
	dir := t.TempDir()

	cmd1 := NewRootCmd()
	cmd1.SetArgs([]string{"add", "server", "--host", "10.0.0.1", "--user", "root", "--config", dir})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first add error: %v", err)
	}

	cmd2 := NewRootCmd()
	cmd2.SetArgs([]string{"add", "server", "--host", "10.0.0.2", "--user", "root", "--config", dir})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("duplicate names should be allowed: %v", err)
	}
}
```

**Step 6: Run CLI tests**

Run: `cd /Users/varun/projects/personal/hangar && go test ./internal/cli/ -v`
Expected: ALL PASS

**Step 7: Commit**

```bash
git add internal/cli/connect.go internal/cli/add.go internal/cli/remove.go internal/cli/list.go internal/cli/cli_test.go
git commit -m "feat: update CLI commands for UUID and SSHOptions support"
```

---

### Task 7: TUI — UUID-based internals and keybinding changes

This is the largest task. It modifies `internal/tui/app.go` extensively.

**Files:**
- Modify: `internal/tui/app.go:1-1337`

**Step 1: Update imports**

Add `"github.com/google/uuid"` to the imports block.

**Step 2: Update Model struct**

Replace these fields in the Model struct:

```go
cutConnections   map[string]bool // old: names
```

With:

```go
cutConnections   map[uuid.UUID]bool // UUIDs of connections being moved (cut)
copyConnections  map[uuid.UUID]bool // UUIDs of connections being copied (yank)
formTarget       uuid.UUID          // connection ID being edited/deleted/tagged
formTargetGroup  string             // group name being edited/deleted
```

Remove the existing `formTarget string` field.

**Step 3: Update NewModel**

```go
func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
		collapsed:        make(map[string]bool),
		cutConnections:   make(map[uuid.UUID]bool),
		copyConnections:  make(map[uuid.UUID]bool),
	}
}
```

**Step 4: Add new form modes**

Add to the formMode const block:

```go
formEditGroup
formGlobalSettings
```

**Step 5: Add SSHOptions form field constants**

Add after the existing field constants:

```go
// Advanced SSH options fields (in connection form, after fieldPassword)
const (
	fieldForwardAgent = fieldCount + iota
	fieldCompression
	fieldLocalForward
	fieldRemoteForward
	fieldServerAliveInterval
	fieldServerAliveCountMax
	fieldStrictHostKeyCheck
	fieldRequestTTY
	fieldEnvVars
	fieldExtraOptions
	fieldUseGlobalSettings
	fieldAdvancedCount
)

var advancedFieldLabels = []string{
	"FwdAgent", "Compress", "LocalFwd", "RemoteFwd",
	"Alive", "AliveMax", "HostKey", "TTY",
	"Envs", "Extra", "UseGlobal",
}
```

**Step 6: Update selectedConnection to use UUID**

```go
func (m Model) selectedConnection() *config.Connection {
	items := m.sidebarItems()
	if m.cursor >= len(items) || m.cursor < 0 {
		return nil
	}
	item := items[m.cursor]
	if item.isGroup {
		return nil
	}
	c, err := m.cfg.FindByID(item.conn.ID)
	if err != nil {
		return nil
	}
	return c
}
```

**Step 7: Update sidebar keybindings**

In the `Update` method's sidebar switch, make these changes:

Replace `case "e":` block (lines 226-238) with:

```go
case "e":
	items := m.sidebarItems()
	if m.cursor < len(items) && items[m.cursor].isGroup {
		// Edit group name
		m.form = formEditGroup
		m.formTargetGroup = items[m.cursor].group
		m.groupNameInput = items[m.cursor].group
		m.formError = ""
	} else {
		c := m.selectedConnection()
		if c != nil {
			m.form = formEdit
			m.formTarget = c.ID
			existingPass, _ := config.GetPassword(c.ID.String())
			if existingPass == "" {
				existingPass, _ = config.GetPassword(c.Name)
			}
			m.formFields = make([]string, fieldAdvancedCount)
			m.formFields[fieldName] = c.Name
			m.formFields[fieldHost] = c.Host
			m.formFields[fieldPort] = fmt.Sprintf("%d", c.Port)
			m.formFields[fieldUser] = c.User
			m.formFields[fieldKey] = c.IdentityFile
			m.formFields[fieldJump] = m.jumpHostDisplay(c.JumpHost)
			m.formFields[fieldGroup] = c.Group
			m.formFields[fieldTags] = strings.Join(c.Tags, ", ")
			m.formFields[fieldPassword] = existingPass
			m.populateSSHOptionsFields(c.SSHOptions, c.UseGlobalSettings)
			m.formCursor = 0
			m.formError = ""
		}
	}
```

Replace `case "d":` block (lines 239-251) with:

```go
case "d":
	items := m.sidebarItems()
	if m.cursor < len(items) && items[m.cursor].isGroup {
		m.form = formDeleteGroup
		m.formTargetGroup = items[m.cursor].group
	} else {
		c := m.selectedConnection()
		if c != nil {
			m.form = formDelete
			m.formTarget = c.ID
		}
	}
```

Replace `case "x":` block (lines 257-266) with:

```go
case "x":
	c := m.selectedConnection()
	if c != nil {
		// Remove from copy if present
		delete(m.copyConnections, c.ID)
		if m.cutConnections[c.ID] {
			delete(m.cutConnections, c.ID)
		} else {
			m.cutConnections[c.ID] = true
		}
	}
```

Replace `case "y":` block (lines 267-287) — change from paste to COPY:

```go
case "y":
	c := m.selectedConnection()
	if c != nil {
		// Remove from cut if present
		delete(m.cutConnections, c.ID)
		if m.copyConnections[c.ID] {
			delete(m.copyConnections, c.ID)
		} else {
			m.copyConnections[c.ID] = true
		}
	}
```

Add new `case "p":` for PASTE (after "y"):

```go
case "p":
	if len(m.cutConnections) > 0 || len(m.copyConnections) > 0 {
		items := m.sidebarItems()
		targetGroup := ""
		if m.cursor < len(items) {
			if items[m.cursor].isGroup {
				targetGroup = items[m.cursor].group
			} else if items[m.cursor].conn != nil {
				targetGroup = items[m.cursor].conn.Group
			}
		}
		// Move cut connections
		for id := range m.cutConnections {
			c, err := m.cfg.FindByID(id)
			if err == nil {
				c.Group = targetGroup
			}
		}
		// Duplicate copied connections
		for id := range m.copyConnections {
			c, err := m.cfg.FindByID(id)
			if err == nil {
				newConn := *c
				newConn.ID = uuid.New()
				newConn.Group = targetGroup
				m.cfg.Connections = append(m.cfg.Connections, newConn)
			}
		}
		config.Save(m.configDir, m.cfg)
		m.cutConnections = make(map[uuid.UUID]bool)
		m.copyConnections = make(map[uuid.UUID]bool)
	}
```

Add `case "G":` for global settings:

```go
case "G":
	m.form = formGlobalSettings
	m.formFields = make([]string, fieldAdvancedCount)
	m.populateSSHOptionsFields(m.globalCfg.SSHOptions, nil)
	m.formCursor = fieldForwardAgent
	m.formError = ""
```

Replace `case "n":` block (lines 211-225) with:

```go
case "n":
	m.form = formAdd
	currentGroup := ""
	items := m.sidebarItems()
	if m.cursor < len(items) {
		if items[m.cursor].isGroup {
			currentGroup = items[m.cursor].group
		} else if items[m.cursor].conn != nil {
			currentGroup = items[m.cursor].conn.Group
		}
	}
	m.formFields = make([]string, fieldAdvancedCount)
	m.formFields[fieldPort] = "22"
	m.formFields[fieldGroup] = currentGroup
	m.formCursor = 0
	m.formError = ""
```

Replace `case "t":` block (lines 288-294):

```go
case "t":
	c := m.selectedConnection()
	if c != nil {
		m.form = formTag
		m.formTarget = c.ID
		m.tagInput = ""
	}
```

Replace `case "enter":` block (lines 295-309):

```go
case "enter":
	c := m.selectedConnection()
	if c != nil {
		jumpHost := sshauth.ResolveJumpHost(m.cfg, c.JumpHost)
		var merged *config.SSHOptions
		useGlobal := c.UseGlobalSettings == nil || *c.UseGlobalSettings
		if useGlobal && m.globalCfg.SSHOptions != nil {
			mo := config.MergeSSHOptions(m.globalCfg.SSHOptions, c.SSHOptions)
			merged = &mo
		} else if c.SSHOptions != nil {
			merged = c.SSHOptions
		}
		cmd, cleanup := sshauth.NewSSHCommand(c, jumpHost, merged)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			cleanup()
			return sshExitMsg{err: err}
		})
	}
```

**Step 8: Update sidebar rendering for cut/copy marks**

In `renderSidebar()`, replace the cutMark logic (lines 535-538):

```go
cutMark := ""
if m.cutConnections[item.conn.ID] {
	cutMark = dimStyle.Render(" ~")
} else if m.copyConnections[item.conn.ID] {
	cutMark = dimStyle.Render(" +")
}
```

**Step 9: Update status bar**

In `renderStatusBar()`, replace the default sidebar bar (line 487):

```go
bar = " n:new  e:edit  d:del  g:group  x:cut  y:copy  p:paste  enter:connect  s:sync  t:tag  l:scripts  G:settings  /:find  q:quit"
```

**Step 10: Add form handlers for formEditGroup and formGlobalSettings**

In `Update()`, add handlers (after the existing form checks around lines 135-149):

```go
if m.form == formEditGroup {
	return m.handleEditGroupInput(msg)
}
if m.form == formGlobalSettings {
	return m.handleGlobalSettingsInput(msg)
}
```

**Step 11: Add helper methods**

Add these methods to the Model:

```go
// jumpHostDisplay converts a UUID JumpHost to a readable display string.
func (m Model) jumpHostDisplay(jumpHost string) string {
	if jumpHost == "" {
		return ""
	}
	if id, err := uuid.Parse(jumpHost); err == nil {
		if c, err := m.cfg.FindByID(id); err == nil {
			return c.Name
		}
	}
	return jumpHost
}

// jumpHostResolve converts a display name or raw value back to a UUID if possible.
func (m Model) jumpHostResolve(input string) string {
	if input == "" {
		return ""
	}
	// Already a UUID?
	if _, err := uuid.Parse(input); err == nil {
		return input
	}
	// Try to find by name
	if c, err := m.cfg.FindByName(input); err == nil {
		return c.ID.String()
	}
	// Return raw (user@host style)
	return input
}

// jumpHostSuggestions returns connections matching the partial input.
func (m Model) jumpHostSuggestions(partial string) []config.Connection {
	if partial == "" {
		return nil
	}
	lower := strings.ToLower(partial)
	var matches []config.Connection
	for _, c := range m.cfg.Connections {
		if strings.Contains(strings.ToLower(c.Name), lower) {
			matches = append(matches, c)
		}
	}
	if len(matches) > 5 {
		matches = matches[:5]
	}
	return matches
}

func (m *Model) populateSSHOptionsFields(opts *SSHOptions, useGlobal *bool) {
	if opts == nil {
		opts = &config.SSHOptions{}
	}
	if opts.ForwardAgent != nil && *opts.ForwardAgent {
		m.formFields[fieldForwardAgent] = "yes"
	}
	if opts.Compression != nil && *opts.Compression {
		m.formFields[fieldCompression] = "yes"
	}
	if len(opts.LocalForward) > 0 {
		m.formFields[fieldLocalForward] = strings.Join(opts.LocalForward, ", ")
	}
	if len(opts.RemoteForward) > 0 {
		m.formFields[fieldRemoteForward] = strings.Join(opts.RemoteForward, ", ")
	}
	if opts.ServerAliveInterval != nil {
		m.formFields[fieldServerAliveInterval] = fmt.Sprintf("%d", *opts.ServerAliveInterval)
	}
	if opts.ServerAliveCountMax != nil {
		m.formFields[fieldServerAliveCountMax] = fmt.Sprintf("%d", *opts.ServerAliveCountMax)
	}
	m.formFields[fieldStrictHostKeyCheck] = opts.StrictHostKeyCheck
	m.formFields[fieldRequestTTY] = opts.RequestTTY
	if len(opts.EnvVars) > 0 {
		var pairs []string
		for k, v := range opts.EnvVars {
			pairs = append(pairs, k+"="+v)
		}
		m.formFields[fieldEnvVars] = strings.Join(pairs, ", ")
	}
	if len(opts.ExtraOptions) > 0 {
		var pairs []string
		for k, v := range opts.ExtraOptions {
			pairs = append(pairs, k+"="+v)
		}
		m.formFields[fieldExtraOptions] = strings.Join(pairs, ", ")
	}
	if useGlobal != nil && !*useGlobal {
		m.formFields[fieldUseGlobalSettings] = "no"
	} else {
		m.formFields[fieldUseGlobalSettings] = "yes"
	}
}

func parseSSHOptionsFromFields(fields []string) (*config.SSHOptions, *bool) {
	opts := &config.SSHOptions{}
	hasAny := false

	if strings.ToLower(strings.TrimSpace(fields[fieldForwardAgent])) == "yes" {
		v := true
		opts.ForwardAgent = &v
		hasAny = true
	}
	if strings.ToLower(strings.TrimSpace(fields[fieldCompression])) == "yes" {
		v := true
		opts.Compression = &v
		hasAny = true
	}
	if lf := strings.TrimSpace(fields[fieldLocalForward]); lf != "" {
		for _, f := range strings.Split(lf, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				opts.LocalForward = append(opts.LocalForward, f)
			}
		}
		hasAny = true
	}
	if rf := strings.TrimSpace(fields[fieldRemoteForward]); rf != "" {
		for _, f := range strings.Split(rf, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				opts.RemoteForward = append(opts.RemoteForward, f)
			}
		}
		hasAny = true
	}
	if sai := strings.TrimSpace(fields[fieldServerAliveInterval]); sai != "" {
		if v, err := strconv.Atoi(sai); err == nil {
			opts.ServerAliveInterval = &v
			hasAny = true
		}
	}
	if sac := strings.TrimSpace(fields[fieldServerAliveCountMax]); sac != "" {
		if v, err := strconv.Atoi(sac); err == nil {
			opts.ServerAliveCountMax = &v
			hasAny = true
		}
	}
	if shk := strings.TrimSpace(fields[fieldStrictHostKeyCheck]); shk != "" {
		opts.StrictHostKeyCheck = shk
		hasAny = true
	}
	if tty := strings.TrimSpace(fields[fieldRequestTTY]); tty != "" {
		opts.RequestTTY = tty
		hasAny = true
	}
	if envs := strings.TrimSpace(fields[fieldEnvVars]); envs != "" {
		opts.EnvVars = make(map[string]string)
		for _, pair := range strings.Split(envs, ",") {
			pair = strings.TrimSpace(pair)
			if k, v, ok := strings.Cut(pair, "="); ok {
				opts.EnvVars[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		hasAny = true
	}
	if extra := strings.TrimSpace(fields[fieldExtraOptions]); extra != "" {
		opts.ExtraOptions = make(map[string]string)
		for _, pair := range strings.Split(extra, ",") {
			pair = strings.TrimSpace(pair)
			if k, v, ok := strings.Cut(pair, "="); ok {
				opts.ExtraOptions[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		hasAny = true
	}

	var useGlobal *bool
	if strings.ToLower(strings.TrimSpace(fields[fieldUseGlobalSettings])) == "no" {
		v := false
		useGlobal = &v
	}

	if !hasAny {
		return nil, useGlobal
	}
	return opts, useGlobal
}
```

Use `config.SSHOptions` (not `SSHOptions`) in the `populateSSHOptionsFields` signature — fix the type reference.

**Step 12: Add handleEditGroupInput**

```go
func (m Model) handleEditGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		newName := strings.TrimSpace(m.groupNameInput)
		if newName == "" {
			m.formError = "group name is required"
			return m, nil
		}
		oldName := m.formTargetGroup
		if newName != oldName {
			// Rename group in all connections
			for i := range m.cfg.Connections {
				if m.cfg.Connections[i].Group == oldName {
					m.cfg.Connections[i].Group = newName
				}
			}
			// Update Groups map
			if m.cfg.Groups != nil {
				delete(m.cfg.Groups, oldName)
				m.cfg.Groups[newName] = true
			}
			// Update collapsed state
			if wasCollapsed, ok := m.collapsed[oldName]; ok {
				delete(m.collapsed, oldName)
				m.collapsed[newName] = wasCollapsed
			}
			config.Save(m.configDir, m.cfg)
		}
		m.form = formNone
	case "backspace":
		if len(m.groupNameInput) > 0 {
			m.groupNameInput = m.groupNameInput[:len(m.groupNameInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.groupNameInput += msg.String()
		}
	}
	return m, nil
}
```

**Step 13: Add handleGlobalSettingsInput**

```go
func (m Model) handleGlobalSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.formCursor++
		if m.formCursor >= fieldAdvancedCount {
			m.formCursor = fieldForwardAgent
		}
		// Skip UseGlobalSettings in global settings form
		if m.formCursor == fieldUseGlobalSettings {
			m.formCursor = fieldForwardAgent
		}
	case "shift+tab", "up":
		m.formCursor--
		if m.formCursor < fieldForwardAgent {
			m.formCursor = fieldExtraOptions
		}
	case "enter":
		opts, _ := parseSSHOptionsFromFields(m.formFields)
		m.globalCfg.SSHOptions = opts
		config.SaveGlobal(m.configDir, m.globalCfg)
		m.form = formNone
	case "backspace":
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	}
	return m, nil
}
```

**Step 14: Update handleFormInput — allow name editing, extend fields**

Replace `handleFormInput` (lines 990-1015):

```go
func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.formCursor = (m.formCursor + 1) % fieldAdvancedCount
	case "shift+tab", "up":
		m.formCursor = (m.formCursor - 1 + fieldAdvancedCount) % fieldAdvancedCount
	case "enter":
		return m.saveForm()
	case "backspace":
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	}
	return m, nil
}
```

**Step 15: Update saveForm for UUID and SSHOptions**

Replace `saveForm` (lines 1018-1101):

```go
func (m Model) saveForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.formFields[fieldName])
	host := strings.TrimSpace(m.formFields[fieldHost])
	portStr := strings.TrimSpace(m.formFields[fieldPort])
	user := strings.TrimSpace(m.formFields[fieldUser])
	key := strings.TrimSpace(m.formFields[fieldKey])
	jumpInput := strings.TrimSpace(m.formFields[fieldJump])
	group := strings.TrimSpace(m.formFields[fieldGroup])
	tagsStr := strings.TrimSpace(m.formFields[fieldTags])
	password := m.formFields[fieldPassword]

	if name == "" {
		m.formError = "Name is required"
		return m, nil
	}
	if host == "" {
		m.formError = "Host is required"
		return m, nil
	}
	if user == "" {
		m.formError = "User is required"
		return m, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		m.formError = "Port must be a positive number"
		return m, nil
	}

	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	jump := m.jumpHostResolve(jumpInput)
	sshOpts, useGlobal := parseSSHOptionsFromFields(m.formFields)

	conn := config.Connection{
		Name:              name,
		Host:              host,
		Port:              port,
		User:              user,
		IdentityFile:      key,
		JumpHost:          jump,
		Group:             group,
		Tags:              tags,
		SSHOptions:        sshOpts,
		UseGlobalSettings: useGlobal,
	}

	if m.form == formAdd {
		if err := m.cfg.Add(conn); err != nil {
			m.formError = err.Error()
			return m, nil
		}
	} else if m.form == formEdit {
		existing, err := m.cfg.FindByID(m.formTarget)
		if err != nil {
			m.formError = "Connection not found"
			return m, nil
		}
		conn.ID = existing.ID
		conn.SyncedFromSSHConfig = existing.SyncedFromSSHConfig
		conn.Scripts = existing.Scripts
		conn.Notes = existing.Notes
		m.cfg.RemoveByID(m.formTarget)
		m.cfg.Connections = append(m.cfg.Connections, conn)
	}

	if err := config.Save(m.configDir, m.cfg); err != nil {
		m.formError = "Save failed: " + err.Error()
		return m, nil
	}

	// Use UUID for keychain
	connID := conn.ID.String()
	if password != "" {
		config.SetPassword(connID, password)
	} else {
		config.DeletePassword(connID)
	}

	m.form = formNone
	m.formError = ""
	return m, nil
}
```

**Step 16: Update handleDeleteConfirm to use UUID**

```go
func (m Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.cfg.RemoveByID(m.formTarget)
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.form = formNone
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}
```

**Step 17: Update handleDeleteGroupConfirm to use formTargetGroup**

```go
func (m Model) handleDeleteGroupConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].Group == m.formTargetGroup {
				m.cfg.Connections[i].Group = ""
			}
		}
		delete(m.collapsed, m.formTargetGroup)
		if m.cfg.Groups != nil {
			delete(m.cfg.Groups, m.formTargetGroup)
		}
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.form = formNone
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}
```

**Step 18: Update handleTagInput to use UUID**

In `handleTagInput`, replace `m.formTarget` string usages with finding by UUID:

```go
func (m Model) handleTagInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		if m.tagInput != "" {
			c, err := m.cfg.FindByID(m.formTarget)
			if err != nil {
				m.form = formNone
				return m, nil
			}
			var toAdd, toRemove []string
			for _, t := range strings.Split(m.tagInput, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				if strings.HasPrefix(t, "-") {
					toRemove = append(toRemove, t[1:])
				} else {
					toAdd = append(toAdd, t)
				}
			}
			if len(toAdd) > 0 {
				m.cfg.AddTags(c.Name, toAdd)
			}
			if len(toRemove) > 0 {
				m.cfg.RemoveTags(c.Name, toRemove)
			}
			config.Save(m.configDir, m.cfg)
		}
		m.form = formNone
	case "backspace":
		if len(m.tagInput) > 0 {
			m.tagInput = m.tagInput[:len(m.tagInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.tagInput += msg.String()
		}
	}
	return m, nil
}
```

**Step 19: Update renderForm to show advanced fields**

Replace `renderForm` (lines 1241-1272):

```go
func (m Model) renderForm() string {
	var b strings.Builder
	if m.form == formEdit {
		b.WriteString(titleStyle.Render("Edit Connection"))
	} else {
		b.WriteString(titleStyle.Render("New Connection"))
	}
	b.WriteString("\n\n")

	// Basic fields
	for i := 0; i < fieldCount; i++ {
		value := m.formFields[i]
		if i == fieldPassword && value != "" {
			value = strings.Repeat("*", len(value))
		}

		label := labelStyle.Render(strings.ToLower(fieldLabels[i]))
		if i == m.formCursor {
			b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(fieldLabels[i])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	// Advanced settings separator
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("  Advanced SSH Options"))
	b.WriteString("\n\n")

	advLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(10)
	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		if idx >= len(advancedFieldLabels) {
			break
		}
		label := advLabel.Render(strings.ToLower(advancedFieldLabels[idx]))
		if i == m.formCursor {
			b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}
```

**Step 20: Add renderEditGroup and renderGlobalSettings**

```go
func (m Model) renderEditGroup() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Group"))
	b.WriteString("\n\n")
	b.WriteString(activeFieldStyle.Render("> name") + " " + normalStyle.Render(m.groupNameInput) + cursorStyle.Render("_"))

	if m.formError != "" {
		b.WriteString("\n\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}

func (m Model) renderGlobalSettings() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Global SSH Settings"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  These defaults apply to all connections unless overridden."))
	b.WriteString("\n\n")

	advLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(10)
	for i := fieldForwardAgent; i < fieldAdvancedCount; i++ {
		if i == fieldUseGlobalSettings {
			continue // Not applicable in global settings
		}
		value := m.formFields[i]
		idx := i - fieldForwardAgent
		if idx >= len(advancedFieldLabels) {
			break
		}
		label := advLabel.Render(strings.ToLower(advancedFieldLabels[idx]))
		if i == m.formCursor {
			b.WriteString(activeFieldStyle.Render("> "+strings.ToLower(advancedFieldLabels[idx])) + " " + normalStyle.Render(value) + cursorStyle.Render("_"))
		} else {
			b.WriteString("  " + label + " " + normalStyle.Render(value))
		}
		b.WriteString("\n")
	}

	if m.formError != "" {
		b.WriteString("\n" + errorStyle.Render("  "+m.formError))
	}

	return b.String()
}
```

**Step 21: Update renderMainPane to include new form modes**

In `renderMainPane`, add cases in the switch (around lines 553-572):

```go
case formEditGroup:
	return m.renderEditGroup()
case formGlobalSettings:
	return m.renderGlobalSettings()
```

**Step 22: Update renderDeleteConfirm to display name from UUID**

```go
func (m Model) renderDeleteConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Connection"))
	b.WriteString("\n\n")
	name := m.formTarget.String()
	if c, err := m.cfg.FindByID(m.formTarget); err == nil {
		name = c.Name
	}
	b.WriteString(normalStyle.Render("Remove ") + selectedStyle.Render(name) + normalStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("this cannot be undone"))
	return b.String()
}
```

**Step 23: Update renderDeleteGroupConfirm to use formTargetGroup**

```go
func (m Model) renderDeleteGroupConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Group"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Remove group ") + selectedStyle.Render(m.formTargetGroup) + normalStyle.Render("?"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("connections will be ungrouped, not deleted"))
	return b.String()
}
```

**Step 24: Update renderTagInput to use UUID**

```go
func (m Model) renderTagInput() string {
	var b strings.Builder
	name := m.formTarget.String()
	if c, err := m.cfg.FindByID(m.formTarget); err == nil {
		name = c.Name
	}
	b.WriteString(titleStyle.Render("Tags: " + name))
	b.WriteString("\n\n")

	if c, err := m.cfg.FindByID(m.formTarget); err == nil && len(c.Tags) > 0 {
		for i, t := range c.Tags {
			if i > 0 {
				b.WriteString(dimStyle.Render(", "))
			}
			b.WriteString(tagStyle.Render(t))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(activeFieldStyle.Render("> add") + " " + normalStyle.Render(m.tagInput) + cursorStyle.Render("_"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("comma-separated, prefix with - to remove"))

	return b.String()
}
```

**Step 25: Update renderMainPane connection detail for password lookup by UUID**

In `renderMainPane`, replace the password check (line 602):

```go
if pass, err := config.GetPassword(c.ID.String()); err == nil && pass != "" {
	b.WriteString(dimStyle.Render("pass ********"))
	b.WriteString("\n")
} else if pass, err := config.GetPassword(c.Name); err == nil && pass != "" {
	b.WriteString(dimStyle.Render("pass ********"))
	b.WriteString("\n")
}
```

**Step 26: Update JumpHost display in connection detail**

Replace line 599 (`dimStyle.Render("via " + c.JumpHost)`):

```go
if c.JumpHost != "" {
	b.WriteString(dimStyle.Render("via " + m.jumpHostDisplay(c.JumpHost)))
	b.WriteString("\n")
}
```

**Step 27: Update handleScriptsInput for enter (run script) to use merged opts**

Replace lines 782-803 in `handleScriptsInput`:

```go
case "enter":
	conn := m.selectedConnection()
	if m.scriptCursor < len(scripts) && conn != nil {
		script := scripts[m.scriptCursor]
		jumpHost := sshauth.ResolveJumpHost(m.cfg, conn.JumpHost)
		var merged *config.SSHOptions
		useGlobal := conn.UseGlobalSettings == nil || *conn.UseGlobalSettings
		if useGlobal && m.globalCfg.SSHOptions != nil {
			mo := config.MergeSSHOptions(m.globalCfg.SSHOptions, conn.SSHOptions)
			merged = &mo
		} else if conn.SSHOptions != nil {
			merged = conn.SSHOptions
		}
		sshCmd, cleanup := sshauth.NewSSHCommand(conn, jumpHost, merged)
		escapedCmd := strings.ReplaceAll(script.Command, "'", "'\\''")
		remoteCmd := fmt.Sprintf("bash -l -c '%s; printf \"\\npress any key to continue...\"; read -n 1'", escapedCmd)
		userHost := sshCmd.Args[len(sshCmd.Args)-1]
		sshCmd.Args = append(sshCmd.Args[:len(sshCmd.Args)-1],
			"-t", userHost, remoteCmd)
		return m, tea.ExecProcess(sshCmd, func(err error) tea.Msg {
			cleanup()
			return sshExitMsg{err: err}
		})
	}
```

**Step 28: Update handleSyncInput — synced entries now have UUIDs from ParseSSHConfig**

In `handleSyncInput` (line 1187), when importing a new entry it already has a UUID from ParseSSHConfig. No changes needed for the import logic, but update the existing entry lookup. The existing code uses `FindByName` which still works.

**Step 29: Update handleAddGroupInput — remove "group already exists" check**

In `handleAddGroupInput`, remove lines 936-940 (the duplicate group check). Duplicate group names should be allowed since connections track group membership by name string, and users may want to recreate a group.

Actually, keep the check — duplicate group names would be confusing. But groups are just label strings, so two groups with the same name would be merged. Keep the existing behavior.

**Step 30: Build and verify compilation**

Run: `cd /Users/varun/projects/personal/hangar && go build ./...`
Expected: Success

**Step 31: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: UUID internals, copy/paste, edit group, global settings, advanced SSH options"
```

---

### Task 8: Bracket paste support for text fields

**Files:**
- Modify: `internal/tui/app.go` (Update function and form handlers)

**Step 1: Enable bracket paste in tea.NewProgram**

In the `Run` function (line 1333), add `tea.WithBracketedPaste()`:

```go
func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithBracketedPaste())
	_, err := p.Run()
	return err
}
```

**Step 2: Handle tea.PasteMsg in Update**

In the `Update` method, add a case for `tea.PasteMsg` BEFORE the `tea.KeyMsg` case:

```go
case tea.PasteMsg:
	pasted := string(msg)
	// Route pasted text to the active form field
	if m.form == formAdd || m.form == formEdit {
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += pasted
		}
	} else if m.form == formGlobalSettings {
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += pasted
		}
	} else if m.form == formAddGroup || m.form == formEditGroup {
		m.groupNameInput += pasted
	} else if m.form == formTag {
		m.tagInput += pasted
	} else if m.form == formEditNotes {
		m.notesInput += pasted
	} else if m.form == formAddScript || m.form == formEditScript {
		if m.scriptField == 0 {
			m.scriptName += pasted
		} else {
			m.scriptCommand += pasted
		}
	} else if m.filtering {
		m.filterText += pasted
		m.cursor = 0
	}
```

**Step 3: Build and verify**

Run: `cd /Users/varun/projects/personal/hangar && go build ./...`
Expected: Success

**Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: bracket paste support for all text input fields"
```

---

### Task 9: JumpHost inline autocomplete

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add autocomplete state to Model**

Add to the Model struct:

```go
jumpSuggestions []config.Connection // autocomplete suggestions for JumpHost field
jumpSugCursor  int                 // selected suggestion index (-1 = none)
```

**Step 2: Update handleFormInput for JumpHost autocomplete**

In `handleFormInput`, after appending a character (the `default:` case), update suggestions when on the JumpHost field:

```go
default:
	if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
		m.formFields[m.formCursor] += msg.String()
		if m.formCursor == fieldJump {
			m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
			m.jumpSugCursor = -1
		}
	}
```

Also handle backspace updating suggestions:

```go
case "backspace":
	if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
		m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		if m.formCursor == fieldJump {
			m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
			m.jumpSugCursor = -1
		}
	}
```

When on the JumpHost field and suggestions are showing, handle `ctrl+n`/`ctrl+p` to navigate and `enter` to accept a suggestion instead of submitting the form:

Add before the `case "enter":` in handleFormInput:

Check if on JumpHost field with active suggestions:

```go
case "enter":
	if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 && m.jumpSugCursor >= 0 {
		selected := m.jumpSuggestions[m.jumpSugCursor]
		m.formFields[fieldJump] = selected.Name
		m.jumpSuggestions = nil
		m.jumpSugCursor = -1
		return m, nil
	}
	return m.saveForm()
case "ctrl+n":
	if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
		m.jumpSugCursor = (m.jumpSugCursor + 1) % len(m.jumpSuggestions)
		return m, nil
	}
case "ctrl+p":
	if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
		if m.jumpSugCursor <= 0 {
			m.jumpSugCursor = len(m.jumpSuggestions) - 1
		} else {
			m.jumpSugCursor--
		}
		return m, nil
	}
```

Also clear suggestions when leaving the JumpHost field:

```go
case "tab", "down":
	m.formCursor = (m.formCursor + 1) % fieldAdvancedCount
	m.jumpSuggestions = nil
	m.jumpSugCursor = -1
case "shift+tab", "up":
	m.formCursor = (m.formCursor - 1 + fieldAdvancedCount) % fieldAdvancedCount
	m.jumpSuggestions = nil
	m.jumpSugCursor = -1
```

**Step 3: Render suggestions in renderForm**

In `renderForm`, after rendering the JumpHost field (when `i == fieldJump`), render suggestions:

```go
if i == fieldJump && len(m.jumpSuggestions) > 0 && i == m.formCursor {
	for si, s := range m.jumpSuggestions {
		prefix := "    "
		nameStyle := dimStyle
		if si == m.jumpSugCursor {
			prefix = "  > "
			nameStyle = selectedStyle
		}
		b.WriteString(prefix + nameStyle.Render(s.Name) + dimStyle.Render(fmt.Sprintf(" (%s@%s)", s.User, s.Host)) + "\n")
	}
}
```

**Step 4: Update status bar for form mode to mention ctrl+n/ctrl+p**

In `renderStatusBar`, update the form bar:

```go
case m.form == formAdd || m.form == formEdit:
	bar = " tab:next  shift+tab:prev  enter:save  esc:cancel  (jump: ctrl+n/p to pick)"
```

**Step 5: Build and verify**

Run: `cd /Users/varun/projects/personal/hangar && go build ./...`
Expected: Success

**Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: inline JumpHost autocomplete with ctrl+n/p navigation"
```

---

### Task 10: Run all tests and fix any issues

**Step 1: Run full test suite**

Run: `cd /Users/varun/projects/personal/hangar && go test ./... -v`
Expected: ALL PASS

If any tests fail, diagnose and fix. Common issues:
- Import cycle with uuid
- Test assertions that assumed name uniqueness
- Signature mismatches from updated functions

**Step 2: Run go vet**

Run: `cd /Users/varun/projects/personal/hangar && go vet ./...`
Expected: Clean

**Step 3: Build**

Run: `cd /Users/varun/projects/personal/hangar && go build ./cmd/hangar/`
Expected: Success

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve test and build issues"
```

---

### Task 11: Manual verification

**Step 1: Launch the TUI**

Run: `cd /Users/varun/projects/personal/hangar && go run ./cmd/hangar/`

**Verify each feature:**

1. **Edit group name** — Navigate to a group header, press `e`, type new name, press enter
2. **Edit connection name** — Navigate to a connection, press `e`, change the name field, press enter
3. **Copy (y) and paste (p)** — Navigate to a connection, press `y` (should show `+`), navigate to a group, press `p` (should duplicate)
4. **Cut (x) and paste (p)** — Navigate to a connection, press `x` (should show `~`), navigate to a group, press `p` (should move)
5. **Duplicate names** — Press `n`, add a connection with the same name as an existing one — should succeed
6. **Global settings (G)** — Press `G`, fill in ForwardAgent=yes, press enter
7. **Advanced settings** — Press `n` or `e`, tab past basic fields to see advanced SSH options
8. **Paste into fields** — Copy text from clipboard, paste into a form field
9. **JumpHost autocomplete** — In add/edit form, type in the Jump field — matching connections should appear, use ctrl+n/p to navigate, enter to select

**Step 2: Commit final state**

```bash
git add -A
git commit -m "feat: complete bugs and features implementation"
```
