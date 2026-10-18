package ssh

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HostKey is one key line returned by ssh-keyscan plus a human
// fingerprint.
type HostKey struct {
	Line        string // raw known_hosts line (hashed via ssh-keyscan -H)
	KeyType     string // e.g. "ssh-ed25519"
	Fingerprint string // SHA256:... (matches `ssh-keygen -lf`)
}

// IsKnownHost reports whether host:port already has an entry in the
// user's known_hosts file (via `ssh-keygen -F <target>`).
func IsKnownHost(host string, port int) bool {
	target := host
	if port != 0 && port != 22 {
		target = fmt.Sprintf("[%s]:%d", host, port)
	}
	cmd := exec.Command("ssh-keygen", "-F", target)
	return cmd.Run() == nil
}

// ScanHost runs `ssh-keyscan -H` against host:port and returns each
// key line with its SHA256 fingerprint.
func ScanHost(host string, port int) ([]HostKey, error) {
	args := []string{"-H", "-T", "5"}
	if port != 0 && port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", port))
	}
	args = append(args, host)
	cmd := exec.Command("ssh-keyscan", args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh-keyscan: %w (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	var keys []HostKey
	for _, raw := range strings.Split(out.String(), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		keyType, b64 := fields[1], fields[2]
		keyBytes, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(keyBytes)
		fp := "SHA256:" + strings.TrimRight(base64.StdEncoding.EncodeToString(sum[:]), "=")
		keys = append(keys, HostKey{Line: line, KeyType: keyType, Fingerprint: fp})
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys returned (host unreachable?)")
	}
	return keys, nil
}

// TrustHost appends the given key lines to the user's known_hosts
// file.
func TrustHost(keys []HostKey) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, k := range keys {
		if _, err := fmt.Fprintln(f, k.Line); err != nil {
			return err
		}
	}
	return nil
}
