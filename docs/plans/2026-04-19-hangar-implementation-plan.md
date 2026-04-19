# Hangar SSH Manager Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a terminal SSH manager with TUI dashboard, CLI subcommands, live session management, and fleet command execution.

**Architecture:** Hybrid SSH — system `ssh` via PTY for interactive sessions, `golang.org/x/crypto/ssh` for fleet exec. Bubbletea TUI with sidebar + session pane. Cobra CLI for non-interactive commands. YAML config with SSH config sync.

**Tech Stack:** Go, cobra, bubbletea/lipgloss/bubbles, golang.org/x/crypto/ssh, gopkg.in/yaml.v3, go-keyring

---

### Task 1: Project Scaffolding & Go Module Init

**Files:**
- Create: `go.mod`
- Create: `cmd/hangar/main.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/varunvasan/hangar`
Expected: `go.mod` created

**Step 2: Create minimal main.go**

```go
// cmd/hangar/main.go
package main

import "fmt"

func main() {
	fmt.Println("hangar")
}
```

**Step 3: Verify it builds and runs**

Run: `go run cmd/hangar/main.go`
Expected: prints `hangar`

**Step 4: Commit**

```bash
git add go.mod cmd/hangar/main.go
git commit -m "feat: scaffold Go project with minimal main"
```

---

### Task 2: Config Data Model & YAML Persistence

**Files:**
- Create: `internal/config/types.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests for config load/save**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
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
	dir := t.TempDir()
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

	// Removing non-existent should fail
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — types and functions not defined

**Step 3: Implement types**

```go
// internal/config/types.go
package config

import "time"

type Connection struct {
	Name               string   `yaml:"name"`
	Host               string   `yaml:"host"`
	Port               int      `yaml:"port"`
	User               string   `yaml:"user"`
	IdentityFile       string   `yaml:"identity_file,omitempty"`
	Tags               []string `yaml:"tags,omitempty"`
	JumpHost           string   `yaml:"jump_host,omitempty"`
	SyncedFromSSHConfig bool    `yaml:"synced_from_ssh_config"`
}

type SSHSync struct {
	LastSync          time.Time `yaml:"last_sync,omitempty"`
	LastSSHConfigHash string    `yaml:"last_ssh_config_hash,omitempty"`
}

type HangarConfig struct {
	Connections []Connection `yaml:"connections"`
	SSHSync     SSHSync      `yaml:"ssh_sync"`
}

type GlobalConfig struct {
	PrefixKey     string `yaml:"prefix_key"`
	SSHConfigPath string `yaml:"ssh_config_path"`
	AutoSync      bool   `yaml:"auto_sync"`
}
```

**Step 4: Implement config load/save/operations**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

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
	return &cfg, nil
}

func Save(dir string, cfg *HangarConfig) error {
	seen := make(map[string]bool)
	for _, c := range cfg.Connections {
		if seen[c.Name] {
			return fmt.Errorf("duplicate connection name: %s", c.Name)
		}
		seen[c.Name] = true
	}

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
	for _, c := range cfg.Connections {
		if c.Name == conn.Name {
			return fmt.Errorf("connection %q already exists", conn.Name)
		}
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

func (cfg *HangarConfig) FindByName(name string) (*Connection, error) {
	for i := range cfg.Connections {
		if cfg.Connections[i].Name == name {
			return &cfg.Connections[i], nil
		}
	}
	return nil, fmt.Errorf("connection %q not found", name)
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
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add config data model with YAML persistence"
```

---

### Task 3: Tag Management

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing tests for tag/untag**

Add to `internal/config/config_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run "TestAddTags|TestRemoveTags"`
Expected: FAIL

**Step 3: Implement AddTags and RemoveTags**

Add to `internal/config/config.go`:

```go
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
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add tag management (add/remove tags)"
```

---

### Task 4: SSH Config Sync

**Files:**
- Create: `internal/config/sync.go`
- Create: `internal/config/sync_test.go`

**Step 1: Write failing tests for SSH config parsing and sync**

```go
// internal/config/sync_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

const testSSHConfig = `Host prod-server
    HostName 10.0.1.50
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_ed25519

Host staging
    HostName staging.example.com
    User admin

Host bastion
    HostName bastion.example.com
    User jump
    Port 22
`

func TestParseSSHConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte(testSSHConfig), 0600)

	conns, err := ParseSSHConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(conns))
	}

	// Check prod-server
	if conns[0].Name != "prod-server" || conns[0].Host != "10.0.1.50" || conns[0].Port != 2222 {
		t.Fatalf("prod-server mismatch: %+v", conns[0])
	}
	if conns[0].User != "deploy" {
		t.Fatalf("expected user deploy, got %s", conns[0].User)
	}

	// Check staging defaults to port 22
	if conns[1].Port != 22 {
		t.Fatalf("expected default port 22, got %d", conns[1].Port)
	}
}

func TestHashSSHConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte(testSSHConfig), 0600)

	hash1, err := HashFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hash2, err := HashFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash1 != hash2 {
		t.Fatal("same file should produce same hash")
	}

	os.WriteFile(path, []byte(testSSHConfig+"\nHost new\n"), 0600)
	hash3, err := HashFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash1 == hash3 {
		t.Fatal("different content should produce different hash")
	}
}

func TestSyncFromSSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshPath := filepath.Join(dir, "ssh_config")
	os.WriteFile(sshPath, []byte(testSSHConfig), 0600)

	cfg := &HangarConfig{}
	// Add a hangar-native connection
	cfg.Add(Connection{Name: "my-server", Host: "10.0.0.99", Port: 22, User: "me"})

	added, updated, err := cfg.SyncFromSSHConfig(sshPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 3 {
		t.Fatalf("expected 3 added, got %d", added)
	}
	if updated != 0 {
		t.Fatalf("expected 0 updated, got %d", updated)
	}
	// Should have 4 total (1 native + 3 synced)
	if len(cfg.Connections) != 4 {
		t.Fatalf("expected 4 connections, got %d", len(cfg.Connections))
	}

	// Synced entries should be marked
	c, _ := cfg.FindByName("prod-server")
	if !c.SyncedFromSSHConfig {
		t.Fatal("synced entry should be marked")
	}

	// Native entry should not be marked
	my, _ := cfg.FindByName("my-server")
	if my.SyncedFromSSHConfig {
		t.Fatal("native entry should not be marked")
	}

	// Re-sync with a change should update
	updatedSSH := `Host prod-server
    HostName 10.0.1.99
    User newuser
    Port 2222

Host staging
    HostName staging.example.com
    User admin

Host bastion
    HostName bastion.example.com
    User jump
`
	os.WriteFile(sshPath, []byte(updatedSSH), 0600)
	added2, updated2, err := cfg.SyncFromSSHConfig(sshPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added2 != 0 {
		t.Fatalf("expected 0 added on re-sync, got %d", added2)
	}
	if updated2 != 1 {
		t.Fatalf("expected 1 updated, got %d", updated2)
	}
	c, _ = cfg.FindByName("prod-server")
	if c.Host != "10.0.1.99" || c.User != "newuser" {
		t.Fatalf("prod-server not updated: %+v", c)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run "TestParse|TestHash|TestSync"`
Expected: FAIL

**Step 3: Implement sync**

```go
// internal/config/sync.go
package config

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

func ParseSSHConfig(path string) ([]Connection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conns []Connection
	var current *Connection

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if strings.EqualFold(key, "Host") {
			if val == "*" {
				continue
			}
			if current != nil {
				if current.Port == 0 {
					current.Port = 22
				}
				conns = append(conns, *current)
			}
			current = &Connection{
				Name:                val,
				SyncedFromSSHConfig: true,
			}
		} else if current != nil {
			switch strings.ToLower(key) {
			case "hostname":
				current.Host = val
			case "user":
				current.User = val
			case "port":
				p, err := strconv.Atoi(val)
				if err == nil {
					current.Port = p
				}
			case "identityfile":
				current.IdentityFile = val
			case "proxyjump":
				current.JumpHost = val
			}
		}
	}
	if current != nil {
		if current.Port == 0 {
			current.Port = 22
		}
		conns = append(conns, *current)
	}
	return conns, scanner.Err()
}

func (cfg *HangarConfig) SyncFromSSHConfig(path string) (added, updated int, err error) {
	parsed, err := ParseSSHConfig(path)
	if err != nil {
		return 0, 0, err
	}

	for _, p := range parsed {
		existing, findErr := cfg.FindByName(p.Name)
		if findErr != nil {
			// New entry
			cfg.Connections = append(cfg.Connections, p)
			added++
		} else if existing.SyncedFromSSHConfig {
			// Update synced entry if changed
			changed := existing.Host != p.Host ||
				existing.Port != p.Port ||
				existing.User != p.User ||
				existing.IdentityFile != p.IdentityFile ||
				existing.JumpHost != p.JumpHost
			if changed {
				existing.Host = p.Host
				existing.Port = p.Port
				existing.User = p.User
				existing.IdentityFile = p.IdentityFile
				existing.JumpHost = p.JumpHost
				updated++
			}
		}
		// Skip if native entry with same name exists
	}

	hash, _ := HashFile(path)
	cfg.SSHSync.LastSync = time.Now()
	cfg.SSHSync.LastSSHConfigHash = hash

	return added, updated, nil
}

func (cfg *HangarConfig) NeedsSync(sshConfigPath string) (bool, error) {
	hash, err := HashFile(sshConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return hash != cfg.SSHSync.LastSSHConfigHash, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add SSH config parsing and sync"
```

---

### Task 5: Global Config

**Files:**
- Create: `internal/config/global.go`
- Create: `internal/config/global_test.go`

**Step 1: Write failing test**

```go
// internal/config/global_test.go
package config

import (
	"os"
	"path/filepath"
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v -run "TestLoadDefault|TestSaveAndLoadGlobal"`
Expected: FAIL

**Step 3: Implement global config**

```go
// internal/config/global.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const globalConfigFile = "config.yaml"

func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		PrefixKey:     "ctrl+a",
		SSHConfigPath: "~/.ssh/config",
		AutoSync:      true,
	}
}

func LoadGlobal(dir string) (*GlobalConfig, error) {
	path := filepath.Join(dir, globalConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultGlobalConfig(), nil
		}
		return nil, fmt.Errorf("reading global config: %w", err)
	}
	gc := DefaultGlobalConfig()
	if err := yaml.Unmarshal(data, gc); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	return gc, nil
}

func SaveGlobal(dir string, gc *GlobalConfig) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(gc)
	if err != nil {
		return fmt.Errorf("marshaling global config: %w", err)
	}
	path := filepath.Join(dir, globalConfigFile)
	return os.WriteFile(path, data, 0600)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add global config with defaults"
```

---

### Task 6: Keychain Password Storage

**Files:**
- Create: `internal/config/keychain.go`
- Create: `internal/config/keychain_test.go`

**Step 1: Write failing tests**

```go
// internal/config/keychain_test.go
package config

import (
	"testing"
)

func TestKeychainKeyFormat(t *testing.T) {
	key := KeychainKey("prod-api-1")
	if key != "hangar:prod-api-1" {
		t.Fatalf("expected hangar:prod-api-1, got %s", key)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v -run TestKeychainKey`
Expected: FAIL

**Step 3: Implement keychain wrapper**

```go
// internal/config/keychain.go
package config

import "github.com/zalando/go-keyring"

const keychainService = "hangar"

func KeychainKey(connName string) string {
	return "hangar:" + connName
}

func GetPassword(connName string) (string, error) {
	return keyring.Get(keychainService, connName)
}

func SetPassword(connName, password string) error {
	return keyring.Set(keychainService, connName, password)
}

func DeletePassword(connName string) error {
	return keyring.Delete(keychainService, connName)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v -run TestKeychainKey`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add keychain password storage wrapper"
```

---

### Task 7: CLI Framework with Cobra

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/list.go`
- Create: `internal/cli/add.go`
- Create: `internal/cli/remove.go`
- Create: `internal/cli/connect.go`
- Create: `internal/cli/sync.go`
- Create: `internal/cli/tag.go`
- Create: `internal/cli/exec.go`
- Modify: `cmd/hangar/main.go`

**Step 1: Implement root command**

```go
// internal/cli/root.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/varunvasan/hangar/internal/config"
)

var cfgDir string

func configDir() string {
	if cfgDir != "" {
		return cfgDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hangar")
}

func loadConfig() (*config.HangarConfig, error) {
	return config.Load(configDir())
}

func saveConfig(cfg *config.HangarConfig) error {
	return config.Save(configDir(), cfg)
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hangar",
		Short: "Terminal SSH manager",
		Long:  "Hangar is a terminal SSH manager with TUI dashboard, session management, and fleet execution.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TUI launch will go here
			fmt.Println("TUI not yet implemented")
			return nil
		},
	}

	root.PersistentFlags().StringVar(&cfgDir, "config", "", "config directory (default ~/.hangar)")

	root.AddCommand(newListCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newRemoveCmd())
	root.AddCommand(newConnectCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newTagCmd())
	root.AddCommand(newUntagCmd())
	root.AddCommand(newExecCmd())

	return root
}
```

**Step 2: Implement list command**

```go
// internal/cli/list.go
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			conns := cfg.Connections
			if tag != "" {
				conns = cfg.FilterByTag(tag)
			}

			if len(conns) == 0 {
				fmt.Println("No connections found.")
				return nil
			}

			for _, c := range conns {
				tags := ""
				if len(c.Tags) > 0 {
					tags = " [" + strings.Join(c.Tags, ", ") + "]"
				}
				fmt.Printf("  %s — %s@%s:%d%s\n", c.Name, c.User, c.Host, c.Port, tags)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	return cmd
}
```

**Step 3: Implement add command**

```go
// internal/cli/add.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/varunvasan/hangar/internal/config"
)

func newAddCmd() *cobra.Command {
	var host, user, identityFile, jumpHost string
	var port int
	var tags []string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			conn := config.Connection{
				Name:         args[0],
				Host:         host,
				Port:         port,
				User:         user,
				IdentityFile: identityFile,
				JumpHost:     jumpHost,
				Tags:         tags,
			}

			if err := cfg.Add(conn); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Added connection %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "hostname or IP (required)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&user, "user", "", "username (required)")
	cmd.Flags().StringVar(&identityFile, "identity-file", "", "path to SSH key")
	cmd.Flags().StringVar(&jumpHost, "jump-host", "", "jump host connection name")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tags for this connection")
	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("user")

	return cmd
}
```

**Step 4: Implement remove command**

```go
// internal/cli/remove.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.Remove(args[0]); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Removed connection %q\n", args[0])
			return nil
		},
	}
}
```

**Step 5: Implement connect command (stub for now)**

```go
// internal/cli/connect.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect to a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			conn, err := cfg.FindByName(args[0])
			if err != nil {
				return err
			}

			sshArgs := []string{
				"-p", strconv.Itoa(conn.Port),
			}
			if conn.IdentityFile != "" {
				sshArgs = append(sshArgs, "-i", conn.IdentityFile)
			}
			if conn.JumpHost != "" {
				jump, err := cfg.FindByName(conn.JumpHost)
				if err != nil {
					return fmt.Errorf("jump host %q: %w", conn.JumpHost, err)
				}
				jumpStr := fmt.Sprintf("%s@%s:%d", jump.User, jump.Host, jump.Port)
				sshArgs = append(sshArgs, "-J", jumpStr)
			}
			sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", conn.User, conn.Host))

			sshCmd := exec.Command("ssh", sshArgs...)
			sshCmd.Stdin = os.Stdin
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr
			return sshCmd.Run()
		},
	}
}
```

**Step 6: Implement sync command**

```go
// internal/cli/sync.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/varunvasan/hangar/internal/config"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync connections from ~/.ssh/config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			gc, err := config.LoadGlobal(configDir())
			if err != nil {
				return err
			}

			sshPath := gc.SSHConfigPath
			if sshPath == "~/.ssh/config" {
				home, _ := os.UserHomeDir()
				sshPath = filepath.Join(home, ".ssh", "config")
			}

			added, updated, err := cfg.SyncFromSSHConfig(sshPath)
			if err != nil {
				return err
			}

			if err := saveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Synced from %s: %d added, %d updated\n", sshPath, added, updated)
			return nil
		},
	}
}
```

**Step 7: Implement tag/untag commands**

```go
// internal/cli/tag.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <name> <tags...>",
		Short: "Add tags to a connection",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.AddTags(args[0], args[1:]); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Tagged %q with %v\n", args[0], args[1:])
			return nil
		},
	}
}

func newUntagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untag <name> <tags...>",
		Short: "Remove tags from a connection",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.RemoveTags(args[0], args[1:]); err != nil {
				return err
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Untagged %q: removed %v\n", args[0], args[1:])
			return nil
		},
	}
}
```

**Step 8: Implement exec command (stub — fleet engine comes later)**

```go
// internal/cli/exec.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var tag string
	var names []string
	var filter string

	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Run a command across multiple servers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fleet engine integration comes in Task 9
			fmt.Println("Fleet exec not yet implemented")
			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filter targets by tag")
	cmd.Flags().StringSliceVar(&names, "name", nil, "filter targets by name")
	cmd.Flags().StringVar(&filter, "filter", "", "filter output to specific server")

	return cmd
}
```

**Step 9: Update main.go**

```go
// cmd/hangar/main.go
package main

import (
	"fmt"
	"os"

	"github.com/varunvasan/hangar/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 10: Verify it builds**

Run: `go mod tidy && go build ./cmd/hangar/`
Expected: builds without error

**Step 11: Verify CLI works**

Run: `./hangar --help`
Expected: shows help with all subcommands listed

**Step 12: Commit**

```bash
git add cmd/ internal/cli/ go.mod go.sum
git commit -m "feat: add CLI framework with all subcommands"
```

---

### Task 8: System SSH Interactive Sessions

**Files:**
- Create: `internal/ssh/session.go`
- Create: `internal/ssh/session_test.go`

**Step 1: Write failing test for SSH args builder**

```go
// internal/ssh/session_test.go
package ssh

import (
	"testing"

	"github.com/varunvasan/hangar/internal/config"
)

func TestBuildSSHArgs(t *testing.T) {
	conn := &config.Connection{
		Name: "test",
		Host: "10.0.0.1",
		Port: 2222,
		User: "deploy",
		IdentityFile: "~/.ssh/id_ed25519",
	}

	args := BuildSSHArgs(conn, nil)
	expected := []string{"-p", "2222", "-i", "~/.ssh/id_ed25519", "deploy@10.0.0.1"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Fatalf("arg %d: expected %s, got %s", i, expected[i], a)
		}
	}
}

func TestBuildSSHArgsWithJumpHost(t *testing.T) {
	conn := &config.Connection{
		Name: "target",
		Host: "10.0.1.50",
		Port: 22,
		User: "deploy",
	}
	jump := &config.Connection{
		Name: "bastion",
		Host: "bastion.example.com",
		Port: 22,
		User: "admin",
	}

	args := BuildSSHArgs(conn, jump)
	// Should contain -J flag
	found := false
	for i, a := range args {
		if a == "-J" && i+1 < len(args) {
			if args[i+1] != "admin@bastion.example.com:22" {
				t.Fatalf("wrong jump host: %s", args[i+1])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected -J flag in args: %v", args)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/ssh/ -v`
Expected: FAIL

**Step 3: Implement SSH session**

```go
// internal/ssh/session.go
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/varunvasan/hangar/internal/config"
)

func BuildSSHArgs(conn *config.Connection, jumpHost *config.Connection) []string {
	args := []string{"-p", strconv.Itoa(conn.Port)}

	if conn.IdentityFile != "" {
		args = append(args, "-i", conn.IdentityFile)
	}

	if jumpHost != nil {
		jumpStr := fmt.Sprintf("%s@%s:%d", jumpHost.User, jumpHost.Host, jumpHost.Port)
		args = append(args, "-J", jumpStr)
	}

	args = append(args, fmt.Sprintf("%s@%s", conn.User, conn.Host))
	return args
}

func Connect(conn *config.Connection, jumpHost *config.Connection) error {
	args := BuildSSHArgs(conn, jumpHost)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/ssh/ -v`
Expected: all PASS

**Step 5: Update connect CLI command to use ssh package**

Modify `internal/cli/connect.go` to call `ssh.Connect()` instead of inline implementation.

**Step 6: Commit**

```bash
git add internal/ssh/ internal/cli/connect.go
git commit -m "feat: add SSH session builder with jump host support"
```

---

### Task 9: Fleet Execution Engine

**Files:**
- Create: `internal/fleet/executor.go`
- Create: `internal/fleet/executor_test.go`
- Create: `internal/fleet/output.go`

**Step 1: Write failing tests for fleet executor**

```go
// internal/fleet/executor_test.go
package fleet

import (
	"testing"

	"github.com/varunvasan/hangar/internal/config"
)

func TestColorAssignment(t *testing.T) {
	servers := []string{"prod-1", "prod-2", "staging-1"}
	colors := AssignColors(servers)
	if len(colors) != 3 {
		t.Fatalf("expected 3 colors, got %d", len(colors))
	}
	// Each server should get a different color
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/fleet/ -v`
Expected: FAIL

**Step 3: Implement output formatter**

```go
// internal/fleet/output.go
package fleet

import "fmt"

var colorPalette = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
	"\033[91m", // bright red
	"\033[92m", // bright green
	"\033[93m", // bright yellow
	"\033[94m", // bright blue
}

const colorReset = "\033[0m"

func AssignColors(servers []string) map[string]string {
	colors := make(map[string]string)
	for i, s := range servers {
		colors[s] = colorPalette[i%len(colorPalette)]
	}
	return colors
}

func FormatLine(serverName, color, line string, showBorder bool) string {
	if !showBorder {
		return line
	}
	return fmt.Sprintf("%s\u2588%s %-15s %s", color, colorReset, serverName, line)
}
```

**Step 4: Implement fleet executor**

```go
// internal/fleet/executor.go
package fleet

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/varunvasan/hangar/internal/config"
	gossh "golang.org/x/crypto/ssh"
)

func ResolveTargets(cfg *config.HangarConfig, tag string, names []string) []config.Connection {
	seen := make(map[string]bool)
	var targets []config.Connection

	if tag != "" {
		for _, c := range cfg.FilterByTag(tag) {
			if !seen[c.Name] {
				targets = append(targets, c)
				seen[c.Name] = true
			}
		}
	}

	for _, name := range names {
		c, err := cfg.FindByName(name)
		if err == nil && !seen[c.Name] {
			targets = append(targets, *c)
			seen[c.Name] = true
		}
	}

	return targets
}

type Result struct {
	Server string
	Line   string
	Err    error
}

func Execute(targets []config.Connection, command string, output chan<- Result, cfg *config.HangarConfig) {
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(conn config.Connection) {
			defer wg.Done()
			if err := executeOnServer(conn, command, output, cfg); err != nil {
				output <- Result{Server: conn.Name, Err: err}
			}
		}(target)
	}

	wg.Wait()
	close(output)
}

func executeOnServer(conn config.Connection, command string, output chan<- Result, cfg *config.HangarConfig) error {
	sshConfig := &gossh.ClientConfig{
		User:            conn.User,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Auth:            buildAuthMethods(conn),
	}

	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)

	var client *gossh.Client
	var err error

	if conn.JumpHost != "" {
		client, err = dialViaJumpHost(conn, addr, sshConfig, cfg)
	} else {
		client, err = gossh.Dial("tcp", addr, sshConfig)
	}
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session failed: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}

	if err := session.Start(command); err != nil {
		return err
	}

	// Stream stdout and stderr
	var streamWg sync.WaitGroup
	streamWg.Add(2)

	streamLines := func(r io.Reader) {
		defer streamWg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			output <- Result{Server: conn.Name, Line: scanner.Text()}
		}
	}

	go streamLines(stdout)
	go streamLines(stderr)
	streamWg.Wait()

	return session.Wait()
}

func buildAuthMethods(conn config.Connection) []gossh.AuthMethod {
	var methods []gossh.AuthMethod

	// SSH agent
	if agentAuth := sshAgentAuth(); agentAuth != nil {
		methods = append(methods, agentAuth)
	}

	// Identity file
	if conn.IdentityFile != "" {
		if keyAuth := publicKeyAuth(conn.IdentityFile); keyAuth != nil {
			methods = append(methods, keyAuth)
		}
	}

	// Keychain password
	if pass, err := config.GetPassword(conn.Name); err == nil {
		methods = append(methods, gossh.Password(pass))
	}

	return methods
}

func sshAgentAuth() gossh.AuthMethod {
	// Will be implemented to connect to SSH_AUTH_SOCK
	return nil
}

func publicKeyAuth(keyPath string) gossh.AuthMethod {
	keyPath = expandHome(keyPath)
	// Read key file and parse
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := fmt.Println() // placeholder
		_ = home
	}
	return path
}

func dialViaJumpHost(conn config.Connection, targetAddr string, targetConfig *gossh.ClientConfig, cfg *config.HangarConfig) (*gossh.Client, error) {
	jump, err := cfg.FindByName(conn.JumpHost)
	if err != nil {
		return nil, fmt.Errorf("jump host %q: %w", conn.JumpHost, err)
	}

	jumpConfig := &gossh.ClientConfig{
		User:            jump.User,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Auth:            buildAuthMethods(*jump),
	}

	jumpAddr := fmt.Sprintf("%s:%d", jump.Host, jump.Port)
	jumpClient, err := gossh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return nil, fmt.Errorf("jump host dial: %w", err)
	}

	netConn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("jump host tunnel: %w", err)
	}

	ncc, chans, reqs, err := gossh.NewClientConn(netConn, targetAddr, targetConfig)
	if err != nil {
		jumpClient.Close()
		return nil, err
	}

	return gossh.NewClient(ncc, chans, reqs), nil
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/fleet/ -v`
Expected: PASS (the unit tests for color assignment and target resolution)

**Step 6: Commit**

```bash
git add internal/fleet/
git commit -m "feat: add fleet execution engine with streaming output"
```

---

### Task 10: Wire Fleet Exec into CLI

**Files:**
- Modify: `internal/cli/exec.go`

**Step 1: Update exec command to use fleet engine**

```go
// internal/cli/exec.go
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/varunvasan/hangar/internal/fleet"
)

func newExecCmd() *cobra.Command {
	var tag string
	var names []string
	var filter string

	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Run a command across multiple servers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			targets := fleet.ResolveTargets(cfg, tag, names)
			if len(targets) == 0 {
				return fmt.Errorf("no targets matched")
			}

			command := strings.Join(args, " ")
			serverNames := make([]string, len(targets))
			for i, t := range targets {
				serverNames[i] = t.Name
			}
			colors := fleet.AssignColors(serverNames)

			fmt.Printf("hangar exec: %s  (%d servers)\n", command, len(targets))

			output := make(chan fleet.Result, 100)
			go fleet.Execute(targets, command, output, cfg)

			for result := range output {
				if filter != "" && result.Server != filter {
					continue
				}

				showBorder := filter == ""
				if result.Err != nil {
					color := colors[result.Server]
					fmt.Println(fleet.FormatLine(result.Server, color, fmt.Sprintf("ERROR: %v", result.Err), showBorder))
				} else {
					color := colors[result.Server]
					fmt.Println(fleet.FormatLine(result.Server, color, result.Line, showBorder))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filter targets by tag")
	cmd.Flags().StringSliceVar(&names, "name", nil, "filter targets by name")
	cmd.Flags().StringVar(&filter, "filter", "", "filter output to specific server")

	return cmd
}
```

**Step 2: Verify it builds**

Run: `go build ./cmd/hangar/`
Expected: builds without error

**Step 3: Commit**

```bash
git add internal/cli/exec.go
git commit -m "feat: wire fleet exec into CLI with colored output"
```

---

### Task 11: Auth — SSH Agent & Key File Support

**Files:**
- Create: `internal/ssh/auth.go`
- Create: `internal/ssh/auth_test.go`
- Modify: `internal/fleet/executor.go` (replace stubs)

**Step 1: Write failing tests**

```go
// internal/ssh/auth_test.go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ssh/ -v -run TestExpand`
Expected: FAIL

**Step 3: Implement auth helpers**

```go
// internal/ssh/auth.go
package ssh

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/varunvasan/hangar/internal/config"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func BuildAuthMethods(conn *config.Connection) []gossh.AuthMethod {
	var methods []gossh.AuthMethod

	if m := AgentAuth(); m != nil {
		methods = append(methods, m)
	}

	if conn.IdentityFile != "" {
		if m := PublicKeyAuth(conn.IdentityFile); m != nil {
			methods = append(methods, m)
		}
	}

	if pass, err := config.GetPassword(conn.Name); err == nil && pass != "" {
		methods = append(methods, gossh.Password(pass))
	}

	return methods
}

func AgentAuth() gossh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return gossh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

func PublicKeyAuth(keyPath string) gossh.AuthMethod {
	path := ExpandHome(keyPath)
	key, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	signer, err := gossh.ParsePrivateKey(key)
	if err != nil {
		return nil
	}
	return gossh.PublicKeys(signer)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/ssh/ -v`
Expected: all PASS

**Step 5: Update fleet executor to use ssh.BuildAuthMethods and ssh.ExpandHome**

Replace the stub `buildAuthMethods`, `sshAgentAuth`, `publicKeyAuth`, and `expandHome` in `internal/fleet/executor.go` with calls to the `ssh` package.

**Step 6: Commit**

```bash
git add internal/ssh/auth.go internal/ssh/auth_test.go internal/fleet/executor.go
git commit -m "feat: add SSH agent and key file authentication"
```

---

### Task 12: TUI — Base App Shell

**Files:**
- Create: `internal/tui/app.go`
- Create: `internal/tui/styles.go`
- Modify: `internal/cli/root.go`

**Step 1: Implement styles**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 1)

	sidebarStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("240")).
		Width(25)

	mainPaneStyle = lipgloss.NewStyle().
		Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	normalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	syncIndicatorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	sectionHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Bold(true)
)
```

**Step 2: Implement base app model**

```go
// internal/tui/app.go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/varunvasan/hangar/internal/config"
)

type focus int

const (
	focusSidebar focus = iota
	focusSession
)

type Model struct {
	cfg           *config.HangarConfig
	globalCfg     *config.GlobalConfig
	configDir     string
	width         int
	height        int
	focus         focus
	cursor        int
	sshConfigChanged bool
	filterText    string
	filtering     bool
	quitting      bool
}

func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.filtering {
			return m.handleFilterInput(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.filteredConnections())-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "/":
			m.filtering = true
			m.filterText = ""
		case "s", "S":
			if m.sshConfigChanged {
				return m, m.syncSSHConfig()
			}
		}
	}

	return m, nil
}

func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.cursor = 0
	case "enter":
		m.filtering = false
		m.cursor = 0
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.cursor = 0
		}
	}
	return m, nil
}

func (m Model) filteredConnections() []config.Connection {
	if m.filterText == "" {
		return m.cfg.Connections
	}
	var filtered []config.Connection
	for _, c := range m.cfg.Connections {
		if strings.Contains(strings.ToLower(c.Name), strings.ToLower(m.filterText)) {
			filtered = append(filtered, c)
		}
		// Also match by tag
		for _, t := range c.Tags {
			if strings.Contains(strings.ToLower(t), strings.ToLower(m.filterText)) {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered
}

func (m Model) syncSSHConfig() tea.Cmd {
	return func() tea.Msg {
		// Sync will be implemented as a tea.Msg
		return nil
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Title bar
	titleLeft := titleStyle.Render(" hangar ")
	syncIndicator := ""
	if m.sshConfigChanged {
		syncIndicator = syncIndicatorStyle.Render(" SSH config changed ⟳ ")
	}
	titleBar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleLeft,
		lipgloss.NewStyle().Width(m.width-lipgloss.Width(titleLeft)-lipgloss.Width(syncIndicator)).Render(""),
		syncIndicator,
	)

	// Sidebar
	sidebar := m.renderSidebar()

	// Main pane
	mainPane := m.renderMainPane()

	// Content area
	contentHeight := m.height - 3 // title + status bar
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Height(contentHeight).Render(sidebar),
		mainPaneStyle.Height(contentHeight).Width(m.width-27).Render(mainPane),
	)

	// Status bar
	statusBar := statusBarStyle.Render(" [c]onnect [d]isconnect [e]xec [s]ync [t]ag [/]find  [q]uit")

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, content, statusBar)
}

func (m Model) renderSidebar() string {
	var b strings.Builder

	b.WriteString(sectionHeaderStyle.Render("Connections"))
	b.WriteString("\n")

	// Filter
	if m.filtering {
		b.WriteString(fmt.Sprintf("🔍 %s▌\n", m.filterText))
	} else if m.filterText != "" {
		b.WriteString(fmt.Sprintf("🔍 %s\n", m.filterText))
	} else {
		b.WriteString("  /filter...\n")
	}

	conns := m.filteredConnections()
	for i, c := range conns {
		style := normalStyle
		prefix := "  "
		if i == m.cursor {
			style = selectedStyle
			prefix = "▸ "
		}
		b.WriteString(style.Render(prefix + c.Name))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderMainPane() string {
	conns := m.filteredConnections()
	if len(conns) == 0 {
		return "No connections. Use 'hangar add' or 'hangar sync' to get started."
	}
	if m.cursor >= len(conns) {
		return ""
	}

	c := conns[m.cursor]
	var b strings.Builder
	b.WriteString(selectedStyle.Render(c.Name))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Host: %s\n", c.Host))
	b.WriteString(fmt.Sprintf("  Port: %d\n", c.Port))
	b.WriteString(fmt.Sprintf("  User: %s\n", c.User))
	if c.IdentityFile != "" {
		b.WriteString(fmt.Sprintf("  Key:  %s\n", c.IdentityFile))
	}
	if c.JumpHost != "" {
		b.WriteString(fmt.Sprintf("  Jump: %s\n", c.JumpHost))
	}
	if len(c.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(c.Tags, ", ")))
	}
	if c.SyncedFromSSHConfig {
		b.WriteString("  Source: synced from SSH config\n")
	}

	return b.String()
}

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

**Step 3: Wire TUI into root command**

Update `internal/cli/root.go` `RunE` to call `tui.Run()` instead of printing placeholder.

**Step 4: Verify it builds and runs**

Run: `go build ./cmd/hangar/ && ./hangar`
Expected: TUI launches with sidebar and empty main pane

**Step 5: Commit**

```bash
git add internal/tui/ internal/cli/root.go
git commit -m "feat: add TUI base shell with sidebar and connection details"
```

---

### Task 13: TUI — Live SSH Sessions

**Files:**
- Create: `internal/tui/session.go`
- Modify: `internal/tui/app.go`

**Step 1: Implement session manager**

```go
// internal/tui/session.go
package tui

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"fmt"

	"github.com/creack/pty"
	"github.com/varunvasan/hangar/internal/config"
	sshpkg "github.com/varunvasan/hangar/internal/ssh"
)

type Session struct {
	Name    string
	conn    *config.Connection
	cmd     *exec.Cmd
	ptmx    *os.File
	output  []byte
	active  bool
}

func NewSession(conn *config.Connection, jumpHost *config.Connection) (*Session, error) {
	args := sshpkg.BuildSSHArgs(conn, jumpHost)
	cmd := exec.Command("ssh", args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("starting pty: %w", err)
	}

	s := &Session{
		Name:   conn.Name,
		conn:   conn,
		cmd:    cmd,
		ptmx:   ptmx,
		active: true,
	}

	// Start reading output in background
	go s.readOutput()

	return s, nil
}

func (s *Session) readOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.output = append(s.output, buf[:n]...)
		}
		if err != nil {
			if err != io.EOF {
				// Session ended
			}
			s.active = false
			return
		}
	}
}

func (s *Session) Write(data []byte) (int, error) {
	return s.ptmx.Write(data)
}

func (s *Session) Resize(rows, cols int) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (s *Session) Close() {
	s.ptmx.Close()
	s.cmd.Process.Kill()
	s.cmd.Wait()
	s.active = false
}

func (s *Session) Output() []byte {
	return s.output
}
```

**Step 2: Add session management to app model**

Update `internal/tui/app.go` to add:
- `sessions []*Session` field
- `activeSession int` field
- Session list in sidebar (below connections)
- `c` key to connect (spawn new session)
- `d` key to disconnect (close session)
- Session terminal output in right pane
- `Ctrl+a` prefix key handling for session mode

**Step 3: Verify it builds**

Run: `go build ./cmd/hangar/`
Expected: builds without error

**Step 4: Commit**

```bash
git add internal/tui/
git commit -m "feat: add live SSH session management in TUI"
```

---

### Task 14: TUI — Fleet Exec View

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add exec mode to TUI**

Add to `internal/tui/app.go`:
- `e` key opens exec input (text input for command)
- Target selection via tag filter or multi-select
- Results stream into right pane with colored left borders
- Reuse `fleet.Execute()` and `fleet.FormatLine()`

**Step 2: Verify it builds and runs**

Run: `go build ./cmd/hangar/ && ./hangar`
Expected: pressing `e` opens command input

**Step 3: Commit**

```bash
git add internal/tui/
git commit -m "feat: add fleet exec view in TUI"
```

---

### Task 15: TUI — SSH Config Sync Integration

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Implement sync message and handler**

Add to `internal/tui/app.go`:
- `syncMsg` tea.Msg type with results
- `S` key triggers sync, shows results in a brief notification
- After sync, refresh connection list
- Update `sshConfigChanged` to false after sync

**Step 2: Verify sync works in TUI**

Run: `go build ./cmd/hangar/ && ./hangar`
Expected: sync indicator shows, pressing `S` syncs and refreshes list

**Step 3: Commit**

```bash
git add internal/tui/
git commit -m "feat: add SSH config sync to TUI"
```

---

### Task 16: Password Prompt Integration

**Files:**
- Modify: `internal/tui/session.go`
- Modify: `internal/cli/connect.go`

**Step 1: Add password prompt to CLI connect**

When SSH connection fails with auth error, prompt for password and offer to save to keychain.

**Step 2: Handle password in TUI sessions**

Detect password prompt from PTY output (looks for "password:" pattern), show a password input overlay.

**Step 3: Commit**

```bash
git add internal/tui/ internal/cli/
git commit -m "feat: add password prompt with keychain save option"
```

---

### Task 17: End-to-End Testing & Polish

**Files:**
- Create: `internal/cli/cli_test.go`

**Step 1: Write CLI integration tests**

```go
// internal/cli/cli_test.go
package cli

import (
	"bytes"
	"testing"
)

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "--config", dir})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddAndList(t *testing.T) {
	dir := t.TempDir()
	root := NewRootCmd()

	// Add
	root.SetArgs([]string{"add", "test-server", "--host", "10.0.0.1", "--user", "root", "--config", dir})
	if err := root.Execute(); err != nil {
		t.Fatalf("add error: %v", err)
	}

	// List
	root2 := NewRootCmd()
	buf := new(bytes.Buffer)
	root2.SetOut(buf)
	root2.SetArgs([]string{"list", "--config", dir})
	if err := root2.Execute(); err != nil {
		t.Fatalf("list error: %v", err)
	}
}
```

**Step 2: Run all tests**

Run: `go test ./... -v`
Expected: all PASS

**Step 3: Build final binary**

Run: `go build -o hangar ./cmd/hangar/`
Expected: binary built

**Step 4: Commit**

```bash
git add internal/cli/cli_test.go
git commit -m "feat: add CLI integration tests"
```

---

Plan complete and saved to `docs/plans/2026-04-19-hangar-implementation-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open new session with executing-plans, batch execution with checkpoints

Which approach?