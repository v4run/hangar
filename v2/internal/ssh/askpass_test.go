package ssh

import (
	"strings"
	"testing"
)

func TestBuildAskpassBranchesPerHost(t *testing.T) {
	entries := []passwordEntry{
		{Host: "bastion.prod", Password: "jumpSecret"},
		{Host: "web-1.prod", Password: "destSecret"},
	}
	script := buildAskpassScript(entries)
	// Must have a case per host.
	for _, want := range []string{"*bastion.prod*", "*web-1.prod*"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing match for %q:\n%s", want, script)
		}
	}
	// Fallback echoes the LAST entry (destination).
	if !strings.Contains(script, "destSecret") {
		t.Fatalf("script missing destination password fallback:\n%s", script)
	}
	// Passwords must be single-quoted to survive shell parsing.
	if !strings.Contains(script, "'jumpSecret'") || !strings.Contains(script, "'destSecret'") {
		t.Fatalf("passwords not quoted:\n%s", script)
	}
}

func TestBuildAskpassEscapesSingleQuote(t *testing.T) {
	entries := []passwordEntry{{Host: "h", Password: "a'b"}}
	script := buildAskpassScript(entries)
	// Our escape is '\'' (end single-quote, escaped single-quote,
	// reopen) — the generated script must contain the escape form.
	if !strings.Contains(script, `'a'\''b'`) {
		t.Fatalf("single-quote not escaped:\n%s", script)
	}
}
