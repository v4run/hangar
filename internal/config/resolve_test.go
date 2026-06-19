package config

import (
	"strings"
	"testing"
)

func TestIsCommandValue(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"$(echo hi)", true},
		{"  $(echo hi)  ", true},
		{"$(echo hi", false},
		{"literal", false},
		{"$()", false},
		{"", false},
		{"$(", false},
	}
	for _, c := range cases {
		if got := IsCommandValue(c.s); got != c.want {
			t.Errorf("IsCommandValue(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestResolveValueLiteral(t *testing.T) {
	got, err := ResolveValue("literal")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "literal" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveValueCommand(t *testing.T) {
	got, err := ResolveValue("$(printf hi)")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveValueTrimsTrailingNewline(t *testing.T) {
	got, err := ResolveValue("$(echo hi)")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
}

func TestResolveValueErrorIncludesStderr(t *testing.T) {
	_, err := ResolveValue("$(sh -c 'echo nope 1>&2; exit 3')")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("error should include stderr (got: %v)", err)
	}
}

func TestResolveConnectionResolvesFields(t *testing.T) {
	conn := &Connection{
		Name:         "test",
		Host:         "$(printf host.example)",
		Port:         22,
		User:         "$(printf admin)",
		IdentityFile: "~/.ssh/id_rsa",
		Password:     "$(printf hunter2)",
	}
	out, err := ResolveConnection(conn)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.Host != "host.example" {
		t.Fatalf("host: %q", out.Host)
	}
	if out.User != "admin" {
		t.Fatalf("user: %q", out.User)
	}
	if out.Password != "hunter2" {
		t.Fatalf("password: %q", out.Password)
	}
	if out.IdentityFile != "~/.ssh/id_rsa" {
		t.Fatalf("identity_file should pass through: %q", out.IdentityFile)
	}
	// Original mustn't be mutated.
	if conn.Host == "host.example" {
		t.Fatal("ResolveConnection mutated the input")
	}
}

func TestResolveConnectionFailsLoudly(t *testing.T) {
	conn := &Connection{
		Name: "test",
		Host: "$(false)",
	}
	if _, err := ResolveConnection(conn); err == nil {
		t.Fatal("expected error from failing command")
	}
}
