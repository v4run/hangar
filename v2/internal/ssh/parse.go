package ssh

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/v4run/hangar/v2/internal/config"
)

// ImportResult is what ParseSSHCommand produces: a prefilled
// Connection and, when the ssh command referenced one, the raw
// jump-host string (user@host form). Callers resolve the jump host
// into a UUID (or offer to create a connection for it) themselves —
// at parse time we don't have the full config to look it up.
type ImportResult struct {
	Connection *config.Connection
	JumpHost   string // "" when the command had no -J
}

// ParseSSHCommand parses a shell-style `ssh [opts] [user@]host [cmd...]`
// into a connection. See docs/plans/… for the supported flag surface.
func ParseSSHCommand(cmd string) (*ImportResult, error) {
	tokens, err := tokenize(cmd)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	if tokens[0] != "ssh" {
		return nil, fmt.Errorf("not an ssh command (starts with %q)", tokens[0])
	}
	tokens = tokens[1:]

	conn := &config.Connection{Port: 22}
	opts := &config.SSHOptions{}
	usedOpts := false
	jumpHost := ""

	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case t == "-p":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-p requires a port")
			}
			p, err := strconv.Atoi(tokens[i])
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", tokens[i])
			}
			conn.Port = p
		case strings.HasPrefix(t, "-p") && len(t) > 2:
			p, err := strconv.Atoi(t[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", t[2:])
			}
			conn.Port = p
		case t == "-i":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-i requires a path")
			}
			conn.IdentityFile = tokens[i]
		case t == "-l":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-l requires a user")
			}
			conn.User = tokens[i]
		case t == "-J":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-J requires a jump-host")
			}
			jumpHost = tokens[i]
		case t == "-o":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-o requires a KEY=VALUE")
			}
			parseOption(tokens[i], opts)
			usedOpts = true
		case t == "-L":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-L requires a spec")
			}
			opts.LocalForward = append(opts.LocalForward, tokens[i])
			usedOpts = true
		case t == "-R":
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("-R requires a spec")
			}
			opts.RemoteForward = append(opts.RemoteForward, tokens[i])
			usedOpts = true
		case t == "-A":
			b := true
			opts.ForwardAgent = &b
			usedOpts = true
		case t == "-a":
			b := false
			opts.ForwardAgent = &b
			usedOpts = true
		case t == "-C":
			b := true
			opts.Compression = &b
			usedOpts = true
		case t == "-t":
			opts.RequestTTY = "yes"
			usedOpts = true
		case t == "-T":
			opts.RequestTTY = "no"
			usedOpts = true
		case t == "-N", t == "-n", t == "-q", t == "-v", t == "-vv", t == "-vvv",
			t == "-x", t == "-X", t == "-Y", t == "-4", t == "-6", t == "-g", t == "-k":
			// no-arg flags; nothing to store
		case strings.HasPrefix(t, "-"):
			// unknown flag — assume it takes an arg so we don't misparse
			i++
		default:
			if strings.Contains(t, "@") {
				parts := strings.SplitN(t, "@", 2)
				conn.User = parts[0]
				conn.Host = parts[1]
			} else {
				conn.Host = t
			}
			i = len(tokens)
			continue
		}
		i++
	}

	if conn.Host == "" {
		return nil, fmt.Errorf("no host in command")
	}
	if usedOpts {
		conn.SSHOptions = opts
	}
	conn.Name = conn.Host
	return &ImportResult{Connection: conn, JumpHost: jumpHost}, nil
}

func parseOption(s string, opts *config.SSHOptions) {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return
	}
	key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
	switch strings.ToLower(key) {
	case "forwardagent":
		b := strings.EqualFold(val, "yes")
		opts.ForwardAgent = &b
	case "compression":
		b := strings.EqualFold(val, "yes")
		opts.Compression = &b
	case "serveraliveinterval":
		if n, err := strconv.Atoi(val); err == nil {
			opts.ServerAliveInterval = &n
		}
	case "serveralivecountmax":
		if n, err := strconv.Atoi(val); err == nil {
			opts.ServerAliveCountMax = &n
		}
	case "stricthostkeychecking":
		opts.StrictHostKeyCheck = val
	case "requesttty":
		opts.RequestTTY = val
	case "port":
		// handled by -p; ignore
	default:
		if opts.ExtraOptions == nil {
			opts.ExtraOptions = make(map[string]string)
		}
		opts.ExtraOptions[key] = val
	}
}

func tokenize(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	escape := false
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		if escape {
			cur.WriteRune(r)
			escape = false
			continue
		}
		switch {
		case r == '\\' && !inSingle:
			escape = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !inSingle && !inDouble:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return tokens, nil
}
