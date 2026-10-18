package ssh

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/v4run/hangar/v2/internal/config"
	"github.com/zalando/go-keyring"
)

const keyringService = "hangar"

// SetPassword stores a literal password in the OS keychain, keyed by
// the connection's UUID (as a string).
func SetPassword(connID, password string) error {
	return keyring.Set(keyringService, connID, password)
}

// GetPassword retrieves the keychain-stored password for connID, or
// "" if none is set. Returns a non-nil error on actual keyring
// failure (not just "no entry").
func GetPassword(connID string) (string, error) {
	v, err := keyring.Get(keyringService, connID)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

// DeletePassword removes the keychain entry for connID.
func DeletePassword(connID string) error {
	err := keyring.Delete(keyringService, connID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// IsCommandValue reports whether s looks like a `$(…)` command
// substitution (so it should be resolved at connect time, not
// treated as a literal).
func IsCommandValue(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "$(") && strings.HasSuffix(s, ")")
}

// ResolveValue runs the `$(…)` form through sh -c and returns its
// stdout (trimmed). Non-command strings pass through unchanged.
func ResolveValue(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !IsCommandValue(s) {
		return s, nil
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "$("), ")")
	out, err := exec.Command("sh", "-c", inner).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// resolvePassword returns the plaintext password to inject for a
// connection: $(cmd) form resolved, then keychain fallback.
func resolvePassword(c *config.Connection) string {
	if c.Password != "" {
		if IsCommandValue(c.Password) {
			if v, err := ResolveValue(c.Password); err == nil && v != "" {
				return v
			}
		} else {
			return c.Password
		}
	}
	if v, err := GetPassword(c.ID.String()); err == nil && v != "" {
		return v
	}
	return ""
}
