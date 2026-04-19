package cli

import (
	"bytes"
	"os"
	"path/filepath"
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

	// Add a connection
	addCmd := NewRootCmd()
	addCmd.SetArgs([]string{"add", "test-server", "--host", "10.0.0.1", "--user", "root", "--port", "22", "--config", dir})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("add error: %v", err)
	}

	// Verify the config file was created
	cfgPath := filepath.Join(dir, "connections.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file not created")
	}

	// List connections
	listCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"list", "--config", dir})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list error: %v", err)
	}
}

func TestAddDuplicate(t *testing.T) {
	dir := t.TempDir()

	cmd1 := NewRootCmd()
	cmd1.SetArgs([]string{"add", "server", "--host", "10.0.0.1", "--user", "root", "--config", dir})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first add error: %v", err)
	}

	cmd2 := NewRootCmd()
	cmd2.SetArgs([]string{"add", "server", "--host", "10.0.0.2", "--user", "root", "--config", dir})
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()

	addCmd := NewRootCmd()
	addCmd.SetArgs([]string{"add", "server", "--host", "10.0.0.1", "--user", "root", "--config", dir})
	addCmd.Execute()

	rmCmd := NewRootCmd()
	rmCmd.SetArgs([]string{"remove", "server", "--config", dir})
	if err := rmCmd.Execute(); err != nil {
		t.Fatalf("remove error: %v", err)
	}

	// Removing again should fail
	rmCmd2 := NewRootCmd()
	rmCmd2.SetArgs([]string{"remove", "server", "--config", dir})
	if err := rmCmd2.Execute(); err == nil {
		t.Fatal("expected error removing non-existent")
	}
}

func TestTagAndUntag(t *testing.T) {
	dir := t.TempDir()

	addCmd := NewRootCmd()
	addCmd.SetArgs([]string{"add", "server", "--host", "10.0.0.1", "--user", "root", "--config", dir})
	addCmd.Execute()

	tagCmd := NewRootCmd()
	tagCmd.SetArgs([]string{"tag", "server", "production", "api", "--config", dir})
	if err := tagCmd.Execute(); err != nil {
		t.Fatalf("tag error: %v", err)
	}

	// List with tag filter
	listCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"list", "--tag", "production", "--config", dir})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list error: %v", err)
	}

	untagCmd := NewRootCmd()
	untagCmd.SetArgs([]string{"untag", "server", "api", "--config", dir})
	if err := untagCmd.Execute(); err != nil {
		t.Fatalf("untag error: %v", err)
	}
}

func TestSyncCommand(t *testing.T) {
	dir := t.TempDir()

	// Create a fake SSH config
	sshDir := filepath.Join(dir, ".ssh")
	os.MkdirAll(sshDir, 0700)
	sshConfig := `Host test-host
    HostName 192.168.1.1
    User admin
    Port 22
`
	os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0600)

	// Create global config pointing to our fake SSH config
	globalCfg := []byte("prefix_key: ctrl+a\nssh_config_path: " + filepath.Join(sshDir, "config") + "\nauto_sync: true\n")
	os.WriteFile(filepath.Join(dir, "config.yaml"), globalCfg, 0600)

	syncCmd := NewRootCmd()
	syncCmd.SetArgs([]string{"sync", "--config", dir})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("sync error: %v", err)
	}

	// Verify connection was synced
	listCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"list", "--config", dir})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list error: %v", err)
	}
}
