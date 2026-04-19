// internal/ssh/auth.go
package ssh

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/v4run/hangar/internal/config"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func BuildAuthMethods(conn *config.Connection) []gossh.AuthMethod {
	var methods []gossh.AuthMethod

	if m := AgentAuth(); m != nil {
		methods = append(methods, m)
	}

	if conn.IdentityFile != "" {
		if m := PublicKeyAuth(conn.IdentityFile); m != nil {
			methods = append(methods, m)
		}
	}

	if pass, err := config.GetPassword(conn.Name); err == nil && pass != "" {
		methods = append(methods, gossh.Password(pass))
	}

	return methods
}

func AgentAuth() gossh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return gossh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

func PublicKeyAuth(keyPath string) gossh.AuthMethod {
	path := ExpandHome(keyPath)
	key, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	signer, err := gossh.ParsePrivateKey(key)
	if err != nil {
		return nil
	}
	return gossh.PublicKeys(signer)
}
