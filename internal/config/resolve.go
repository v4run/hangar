package config

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// resolveTimeout caps how long a $(...) command may run before it's killed.
const resolveTimeout = 10 * time.Second

// IsCommandValue reports whether s is the $(...) command-substitution form
// (matches bash-style command substitution but resolved by Hangar locally,
// not by the remote shell). Leading/trailing whitespace is tolerated.
func IsCommandValue(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "$(") && strings.HasSuffix(s, ")") && len(s) > 3
}

// ResolveValue returns s if it isn't a $(...) wrapper; otherwise it runs the
// inner command via `sh -c` with a 10s timeout and returns its trimmed stdout.
// On non-zero exit the error includes stderr so the user can debug.
func ResolveValue(s string) (string, error) {
	if !IsCommandValue(s) {
		return s, nil
	}
	inner := strings.TrimSpace(s)
	inner = strings.TrimSpace(inner[2 : len(inner)-1])
	if inner == "" {
		return "", fmt.Errorf("empty $(...) command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", inner)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(ee.Stderr))
			if stderr != "" {
				return "", fmt.Errorf("$(%s): %s", inner, stderr)
			}
		}
		return "", fmt.Errorf("$(%s): %w", inner, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// ResolveConnection returns a copy of conn with command-form fields resolved
// to their stdout. Host, User, IdentityFile, and Password are eligible. If any
// resolution fails, the partially-resolved copy is returned alongside the
// error so callers can surface a useful message.
func ResolveConnection(conn *Connection) (*Connection, error) {
	if conn == nil {
		return nil, nil
	}
	out := *conn
	var firstErr error
	resolve := func(field string, src *string) {
		if firstErr != nil {
			return
		}
		v, err := ResolveValue(*src)
		if err != nil {
			firstErr = fmt.Errorf("%s: %w", field, err)
			return
		}
		*src = v
	}
	resolve("host", &out.Host)
	resolve("user", &out.User)
	resolve("identity_file", &out.IdentityFile)
	resolve("password", &out.Password)
	return &out, firstErr
}

// ResolveDatabase mirrors ResolveConnection for Database profiles.
func ResolveDatabase(d *Database) (*Database, error) {
	if d == nil {
		return nil, nil
	}
	out := *d
	var firstErr error
	resolve := func(field string, src *string) {
		if firstErr != nil {
			return
		}
		v, err := ResolveValue(*src)
		if err != nil {
			firstErr = fmt.Errorf("%s: %w", field, err)
			return
		}
		*src = v
	}
	resolve("host", &out.Host)
	resolve("user", &out.User)
	resolve("db", &out.DBName)
	resolve("password", &out.Password)
	return &out, firstErr
}
