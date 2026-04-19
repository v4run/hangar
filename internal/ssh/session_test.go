package ssh

import (
	"testing"

	"github.com/v4run/hangar/internal/config"
)

func TestBuildSSHArgs(t *testing.T) {
	conn := &config.Connection{
		Name:         "test",
		Host:         "10.0.0.1",
		Port:         2222,
		User:         "deploy",
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
		Name: "test",
		Host: "10.0.0.1",
		Port: 22,
		User: "root",
	}

	args := BuildSSHArgs(conn, nil)
	expected := []string{"-p", "22", "root@10.0.0.1"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}
