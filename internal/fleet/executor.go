// internal/fleet/executor.go
package fleet

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/v4run/hangar/internal/config"
	sshauth "github.com/v4run/hangar/internal/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func hostKeyCallback() gossh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine home directory (%v), host key verification disabled\n", err)
		return gossh.InsecureIgnoreHostKey()
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load known_hosts (%v), host key verification disabled\n", err)
		return gossh.InsecureIgnoreHostKey()
	}
	return cb
}

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
		HostKeyCallback: hostKeyCallback(),
		Auth:            sshauth.BuildAuthMethods(&conn),
	}

	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)

	var client *gossh.Client
	var cleanup func()
	var err error

	if conn.JumpHost != "" {
		client, cleanup, err = dialViaJumpHost(conn, addr, sshConfig, cfg)
	} else {
		client, err = gossh.Dial("tcp", addr, sshConfig)
		cleanup = func() { client.Close() }
	}
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer cleanup()

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

func dialViaJumpHost(conn config.Connection, targetAddr string, targetConfig *gossh.ClientConfig, cfg *config.HangarConfig) (*gossh.Client, func(), error) {
	jump, err := cfg.FindByName(conn.JumpHost)
	if err != nil {
		return nil, nil, fmt.Errorf("jump host %q: %w", conn.JumpHost, err)
	}

	jumpConfig := &gossh.ClientConfig{
		User:            jump.User,
		HostKeyCallback: hostKeyCallback(),
		Auth:            sshauth.BuildAuthMethods(jump),
	}

	jumpAddr := fmt.Sprintf("%s:%d", jump.Host, jump.Port)
	jumpClient, err := gossh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("jump host dial: %w", err)
	}

	netConn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		return nil, nil, fmt.Errorf("jump host tunnel: %w", err)
	}

	ncc, chans, reqs, err := gossh.NewClientConn(netConn, targetAddr, targetConfig)
	if err != nil {
		netConn.Close()
		jumpClient.Close()
		return nil, nil, err
	}

	client := gossh.NewClient(ncc, chans, reqs)
	cleanup := func() {
		client.Close()
		netConn.Close()
		jumpClient.Close()
	}
	return client, cleanup, nil
}
