package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/v4run/hangar/internal/config"
	sshpkg "github.com/v4run/hangar/internal/ssh"
)

// Session represents an active SSH connection running inside a PTY.
type Session struct {
	Name         string
	conn         *config.Connection
	cmd          *exec.Cmd
	ptmx         *os.File
	output       []byte
	mu           sync.Mutex
	active       bool
	password     string
	passwordSent bool
}

// NewSession starts an SSH session in a PTY for the given connection.
func NewSession(conn *config.Connection, jumpHost *config.Connection) (*Session, error) {
	args := sshpkg.BuildSSHArgs(conn, jumpHost)
	cmd := exec.Command("ssh", args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("starting pty: %w", err)
	}

	s := &Session{
		Name:     conn.Name,
		conn:     conn,
		cmd:      cmd,
		ptmx:     ptmx,
		active:   true,
		password: sshpkg.GetStoredPassword(conn.Name),
	}

	go s.readOutput()

	return s, nil
}

func (s *Session) readOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.mu.Lock()
			chunk := buf[:n]

			// Detect clear screen sequences and reset buffer
			if bytes.Contains(chunk, []byte("\x1b[2J")) ||
				bytes.Contains(chunk, []byte("\x1b[H\x1b[2J")) ||
				bytes.Contains(chunk, []byte("\x1bc")) {
				s.output = nil
			}

			s.output = append(s.output, chunk...)
			// Keep only last 64KB of output
			if len(s.output) > 65536 {
				s.output = s.output[len(s.output)-65536:]
			}

			// Auto-type password when prompt is detected
			if s.password != "" && !s.passwordSent {
				if detectPasswordPrompt(s.output) {
					s.ptmx.Write([]byte(s.password + "\r"))
					s.passwordSent = true
				}
			}

			s.mu.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				_ = err
			}
			s.mu.Lock()
			s.active = false
			s.mu.Unlock()
			return
		}
	}
}

// detectPasswordPrompt checks if the output ends with a password prompt.
func detectPasswordPrompt(output []byte) bool {
	// Look at the last 100 bytes for the prompt
	tail := output
	if len(tail) > 100 {
		tail = tail[len(tail)-100:]
	}
	lower := bytes.ToLower(tail)
	trimmed := bytes.TrimRight(lower, " ")
	return bytes.HasSuffix(trimmed, []byte("password:")) ||
		bytes.HasSuffix(trimmed, []byte("password: ")) ||
		strings.HasSuffix(string(trimmed), "'s password:")
}

// Write sends data to the session's PTY.
func (s *Session) Write(data []byte) (int, error) {
	return s.ptmx.Write(data)
}

// Resize changes the PTY window size.
func (s *Session) Resize(rows, cols int) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Close terminates the session and cleans up resources.
func (s *Session) Close() {
	s.ptmx.Close()
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait() //nolint:errcheck
	}
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
}

// Output returns a copy of the session's buffered output.
func (s *Session) Output() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]byte, len(s.output))
	copy(out, s.output)
	return out
}

// IsActive returns whether the session is still running.
func (s *Session) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}
