package ssh

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

func TestBuildSSHArgs(t *testing.T) {
	conn := &config.Connection{
		ID:           uuid.New(),
		Name:         "test",
		Host:         "10.0.0.1",
		Port:         2222,
		User:         "deploy",
		IdentityFile: "~/.ssh/id_ed25519",
	}

	args := BuildSSHArgs(conn, nil, nil)
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
		ID:   uuid.New(),
		Name: "target",
		Host: "10.0.1.50",
		Port: 22,
		User: "deploy",
	}
	jump := &config.Connection{
		ID:   uuid.New(),
		Name: "bastion",
		Host: "bastion.example.com",
		Port: 22,
		User: "admin",
	}

	args := BuildSSHArgs(conn, jump, nil)
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

func TestBuildSSHArgsDefaultPort(t *testing.T) {
	conn := &config.Connection{
		ID:   uuid.New(),
		Name: "test",
		Host: "10.0.0.1",
		Port: 22,
		User: "root",
	}

	args := BuildSSHArgs(conn, nil, nil)
	expected := []string{"-p", "22", "root@10.0.0.1"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestBuildSSHArgsWithSSHOptions(t *testing.T) {
	conn := &config.Connection{
		ID:   uuid.New(),
		Name: "test",
		Host: "10.0.0.1",
		Port: 22,
		User: "root",
	}
	opts := &config.SSHOptions{
		ForwardAgent:        boolPtr(true),
		Compression:         boolPtr(true),
		ServerAliveInterval: intPtr(30),
		StrictHostKeyCheck:  "accept-new",
		LocalForward:        []string{"8080:localhost:80", "9090:localhost:90"},
		RemoteForward:       []string{"3000:localhost:3000"},
		EnvVars:             map[string]string{"MY_VAR": "value"},
		ExtraOptions:        map[string]string{"TCPKeepAlive": "yes"},
	}

	args := BuildSSHArgs(conn, nil, opts)
	argStr := fmt.Sprintf("%v", args)

	checks := map[string]bool{
		"-o ForwardAgent=yes":                 false,
		"-o Compression=yes":                  false,
		"-o ServerAliveInterval=30":           false,
		"-o StrictHostKeyChecking=accept-new": false,
		"-L 8080:localhost:80":                false,
		"-L 9090:localhost:90":                false,
		"-R 3000:localhost:3000":              false,
		"-o SendEnv=MY_VAR":                   false,
		"-o TCPKeepAlive=yes":                 false,
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

func TestBuildSSHArgsRawJumpHost(t *testing.T) {
	conn := &config.Connection{
		ID:       uuid.New(),
		Name:     "test",
		Host:     "10.0.0.1",
		Port:     22,
		User:     "root",
		JumpHost: "admin@bastion.example.com:22",
	}

	args := BuildSSHArgs(conn, nil, nil)
	found := false
	for i, a := range args {
		if a == "-J" && i+1 < len(args) {
			if args[i+1] != "admin@bastion.example.com:22" {
				t.Fatalf("wrong raw jump host: %s", args[i+1])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected -J flag for raw JumpHost in args: %v", args)
	}
}

func TestResolveJumpHost(t *testing.T) {
	bastionID := uuid.New()
	cfg := &config.HangarConfig{
		Connections: []config.Connection{
			{ID: bastionID, Name: "bastion", Host: "10.0.0.1", Port: 22, User: "jump"},
			{ID: uuid.New(), Name: "target", Host: "10.0.0.2", Port: 22, User: "deploy"},
		},
	}

	// Resolve by UUID
	result := ResolveJumpHost(cfg, bastionID.String())
	if result == nil || result.Name != "bastion" {
		t.Fatal("expected to resolve bastion by UUID")
	}

	// Resolve by name
	result = ResolveJumpHost(cfg, "bastion")
	if result == nil || result.Name != "bastion" {
		t.Fatal("expected to resolve bastion by name")
	}

	// Unresolvable returns nil
	result = ResolveJumpHost(cfg, "admin@other.host:22")
	if result != nil {
		t.Fatal("expected nil for unresolvable raw value")
	}

	// Empty returns nil
	result = ResolveJumpHost(cfg, "")
	if result != nil {
		t.Fatal("expected nil for empty")
	}
}
