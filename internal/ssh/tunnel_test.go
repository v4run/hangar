package ssh

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

func TestFreeLocalPortRange(t *testing.T) {
	p, err := FreeLocalPort()
	if err != nil {
		t.Fatalf("FreeLocalPort: %v", err)
	}
	if p < 1024 || p > 65535 {
		t.Fatalf("out of ephemeral range: %d", p)
	}
}

func TestBuildTunnelCommandShape(t *testing.T) {
	conn := &config.Connection{
		ID: uuid.New(), Name: "bastion", Host: "10.0.0.1", Port: 22, User: "deploy",
	}
	cmd, _ := BuildTunnelCommand(conn, nil, nil, 6543, "db.internal", 5432)
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "-N") {
		t.Fatalf("missing -N: %s", joined)
	}
	if !strings.Contains(joined, "-L 6543:db.internal:5432") {
		t.Fatalf("missing -L mapping: %s", joined)
	}
	if !strings.HasSuffix(joined, "deploy@10.0.0.1") {
		t.Fatalf("destination not last: %s", joined)
	}
}
