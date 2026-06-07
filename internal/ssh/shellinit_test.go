package ssh

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

func TestComposeShellInitLayering(t *testing.T) {
	cfg := &config.HangarConfig{
		GlobalShellInit: "alias g=git",
		GroupShellInit:  map[string]string{"prod": "alias deploy='echo prod'"},
	}
	conn := &config.Connection{Name: "web", Group: "prod", ShellInit: "foo() { echo hi; }"}
	out := ComposeShellInit(cfg, conn)

	// All three markers present, in order.
	idxGlobal := strings.Index(out, "# --- hangar: global ---")
	idxGroup := strings.Index(out, `# --- hangar: group "prod" ---`)
	idxConn := strings.Index(out, `# --- hangar: connection "web" ---`)
	if idxGlobal < 0 || idxGroup < 0 || idxConn < 0 {
		t.Fatalf("missing markers in:\n%s", out)
	}
	if !(idxGlobal < idxGroup && idxGroup < idxConn) {
		t.Fatalf("wrong order in:\n%s", out)
	}

	// Snippets are present.
	for _, want := range []string{"alias g=git", "alias deploy='echo prod'", "foo() { echo hi; }"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestComposeShellInitEmpty(t *testing.T) {
	cfg := &config.HangarConfig{}
	conn := &config.Connection{Name: "x"}
	if out := ComposeShellInit(cfg, conn); out != "" {
		t.Fatalf("expected empty, got %q", out)
	}
}

func TestComposeShellInitSkipsEmptyLayers(t *testing.T) {
	cfg := &config.HangarConfig{GroupShellInit: map[string]string{"prod": "   "}}
	conn := &config.Connection{Name: "x", Group: "prod", ShellInit: "alias x=1"}
	out := ComposeShellInit(cfg, conn)
	if strings.Contains(out, "global") || strings.Contains(out, "group") {
		t.Fatalf("expected only connection layer, got:\n%s", out)
	}
	if !strings.Contains(out, "alias x=1") {
		t.Fatalf("missing connection snippet in:\n%s", out)
	}
}

func TestBuildSSHArgsWithShellInit(t *testing.T) {
	conn := &config.Connection{
		ID: uuid.New(), Name: "web", Host: "h.example", Port: 22, User: "u",
	}
	snippet := "alias ll='ls -la'\nfoo() { echo hi; }\n"
	args := BuildSSHArgs(conn, nil, nil, snippet)

	// -t must be present.
	foundT := false
	for _, a := range args {
		if a == "-t" {
			foundT = true
			break
		}
	}
	if !foundT {
		t.Fatalf("expected -t flag in: %v", args)
	}

	// Trailing arg is the remote bash command with base64-decoded snippet.
	last := args[len(args)-1]
	re := regexp.MustCompile(`^bash --rcfile <\(printf %s "([A-Za-z0-9+/=]+)" \| base64 -d\) -i$`)
	matches := re.FindStringSubmatch(last)
	if matches == nil {
		t.Fatalf("unexpected trailing arg shape: %q", last)
	}
	decoded, err := base64.StdEncoding.DecodeString(matches[1])
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != snippet {
		t.Fatalf("decoded mismatch:\n got: %q\nwant: %q", decoded, snippet)
	}
}

func TestBuildSSHArgsWithoutShellInitUnchanged(t *testing.T) {
	conn := &config.Connection{
		ID: uuid.New(), Name: "web", Host: "h.example", Port: 22, User: "u",
	}
	args := BuildSSHArgs(conn, nil, nil, "")
	for _, a := range args {
		if a == "-t" {
			t.Fatalf("did not expect -t when shellInit is empty: %v", args)
		}
		if strings.HasPrefix(a, "bash --rcfile") {
			t.Fatalf("did not expect remote command when shellInit is empty: %v", args)
		}
	}
}
