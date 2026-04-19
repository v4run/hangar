// internal/fleet/executor.go
package fleet

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/v4run/hangar/internal/config"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func ResolveTargets(cfg *config.HangarConfig, tag string, names []string) []config.Connection {
	seen := make(map[string]bool)
	var targets []config.Connection

	if tag != "" {
		for _, c := range cfg.FilterByTag(tag) {
			if !seen[c.Name] {
				targets = append(targets, c)
				seen[c.Name] = true
			}
		}
	}

	for _, name := range names {
		c, err := cfg.FindByName(name)
		if err == nil && !seen[c.Name] {
			targets = append(targets, *c)
			seen[c.Name] = true
		}
	}

	return targets
}

type Result struct {
	Server string
	Line   string
	Err    error
}

func Execute(targets []config.Connection, command string, output chan<- Result, cfg *config.HangarConfig) {
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(conn config.Connection) {
			defer wg.Done()
			if err := executeOnServer(conn, command, output, cfg); err != nil {
				output <- Result{Server: conn.Name, Err: err}
			}
		}(target)
	}

	wg.Wait()
	close(output)
}

func executeOnServer(conn config.Connection, command string, output chan<- Result, cfg *config.HangarConfig) error {
	sshConfig := &gossh.ClientConfig{
		User:            conn.User,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Auth:            buildAuthMethods(conn),
	}

	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)

	var client *gossh.Client
	var err error

	if conn.JumpHost != "" {
		client, err = dialViaJumpHost(conn, addr, sshConfig, cfg)
	} else {
		client, err = gossh.Dial("tcp", addr, sshConfig)
	}
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session failed: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}

	if err := session.Start(command); err != nil {
		return err
	}

	var streamWg sync.WaitGroup
	streamWg.Add(2)

	streamLines := func(r io.Reader) {
		defer streamWg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			output <- Result{Server: conn.Name, Line: scanner.Text()}
		}
	}

	go streamLines(stdout)
	go streamLines(stderr)
	streamWg.Wait()

	return session.Wait()
}

func buildAuthMethods(conn config.Connection) []gossh.AuthMethod {
	var methods []gossh.AuthMethod

	if agentAuth := sshAgentAuth(); agentAuth != nil {
		methods = append(methods, agentAuth)
	}

	if conn.IdentityFile != "" {
		if keyAuth := publicKeyAuth(conn.IdentityFile); keyAuth != nil {
			methods = append(methods, keyAuth)
		}
	}

	if pass, err := config.GetPassword(conn.Name); err == nil && pass != "" {
		methods = append(methods, gossh.Password(pass))
	}

	return methods
}

func sshAgentAuth() gossh.AuthMethod {
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

func publicKeyAuth(keyPath string) gossh.AuthMethod {
	path := expandHome(keyPath)
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

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func dialViaJumpHost(conn config.Connection, targetAddr string, targetConfig *gossh.ClientConfig, cfg *config.HangarConfig) (*gossh.Client, error) {
	jump, err := cfg.FindByName(conn.JumpHost)
	if err != nil {
		return nil, fmt.Errorf("jump host %q: %w", conn.JumpHost, err)
	}

	jumpConfig := &gossh.ClientConfig{
		User:            jump.User,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Auth:            buildAuthMethods(*jump),
	}

	jumpAddr := fmt.Sprintf("%s:%d", jump.Host, jump.Port)
	jumpClient, err := gossh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return nil, fmt.Errorf("jump host dial: %w", err)
	}

	netConn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("jump host tunnel: %w", err)
	}

	ncc, chans, reqs, err := gossh.NewClientConn(netConn, targetAddr, targetConfig)
	if err != nil {
		jumpClient.Close()
		return nil, err
	}

	return gossh.NewClient(ncc, chans, reqs), nil
}
