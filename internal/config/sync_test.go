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

	if conns[0].Name != "prod-server" || conns[0].Host != "10.0.1.50" || conns[0].Port != 2222 {
		t.Fatalf("prod-server mismatch: %+v", conns[0])
	}
	if conns[0].User != "deploy" {
		t.Fatalf("expected user deploy, got %s", conns[0].User)
	}

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

func TestParseSSHConfigWithEquals(t *testing.T) {
	dir := t.TempDir()
	sshConfig := `Host equaltest
    HostName=10.0.0.50
    User=admin
    Port=2222
`
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte(sshConfig), 0600)

	conns, err := ParseSSHConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].Host != "10.0.0.50" || conns[0].User != "admin" || conns[0].Port != 2222 {
		t.Fatalf("wrong values: %+v", conns[0])
	}
}

func TestParseSSHConfigWithMatch(t *testing.T) {
	dir := t.TempDir()
	sshConfig := `Host normalhost
    HostName 10.0.0.1
    User admin

Match host *.internal
    ProxyJump bastion

Host aftermatch
    HostName 10.0.0.2
    User deploy
`
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte(sshConfig), 0600)

	conns, err := ParseSSHConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections (skipping Match), got %d", len(conns))
	}
	if conns[0].Name != "normalhost" {
		t.Fatalf("expected normalhost, got %s", conns[0].Name)
	}
	if conns[1].Name != "aftermatch" {
		t.Fatalf("expected aftermatch, got %s", conns[1].Name)
	}
}

func TestSyncFromSSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshPath := filepath.Join(dir, "ssh_config")
	os.WriteFile(sshPath, []byte(testSSHConfig), 0600)

	cfg := &HangarConfig{}
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
	if len(cfg.Connections) != 4 {
		t.Fatalf("expected 4 connections, got %d", len(cfg.Connections))
	}

	c, _ := cfg.FindByName("prod-server")
	if !c.SyncedFromSSHConfig {
		t.Fatal("synced entry should be marked")
	}

	my, _ := cfg.FindByName("my-server")
	if my.SyncedFromSSHConfig {
		t.Fatal("native entry should not be marked")
	}

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
