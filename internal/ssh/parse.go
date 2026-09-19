package ssh

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/v4run/hangar/internal/config"
)

// ParseSSHCommand parses a shell-style `ssh [opts] [user@]host [cmd...]`
// string into a Connection. Handles the flags Hangar knows how to store:
// -p / -i / -l / -J / -o / -L / -R / -A / -a / -C / -t / -T. Any trailing
// remote command is ignored. Unknown short flags are skipped; if we don't
// know whether they take an arg, we assume they do (safe default given
// ssh's flag surface).
func ParseSSHCommand(cmd string) (*config.Connection, error) {
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
			conn.JumpHost = tokens[i]
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
			// Known no-arg flags; nothing to store.
		case strings.HasPrefix(t, "-"):
			// Unknown flag — assume it takes an argument (F, b, B, c, e, m, w, S, Q, ...).
			// Better to silently skip than to misparse the destination.
			i++
		default:
			// First non-flag token is the destination.
			if strings.Contains(t, "@") {
				parts := strings.SplitN(t, "@", 2)
				conn.User = parts[0]
				conn.Host = parts[1]
			} else {
				conn.Host = t
			}
			// Anything after is a remote command — ignore.
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
	// Default the connection name from the host so the form is one Enter
	// away from saveable.
	conn.Name = conn.Host
	return conn, nil
}

// parseOption applies a single `-o KEY=VALUE` pair to opts, mapping the
// common OpenSSH keywords onto the SSHOptions struct fields. Unknown
// keys land in ExtraOptions verbatim so no user intent is lost.
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
	case "sendenv":
		if opts.EnvVars == nil {
			opts.EnvVars = make(map[string]string)
		}
		// We only have the name; value comes from the local env at connect.
		opts.EnvVars[val] = ""
	case "user":
		// Handled by -l normally; also honor -o User=name.
		if opts.ExtraOptions == nil {
			opts.ExtraOptions = make(map[string]string)
		}
		opts.ExtraOptions[key] = val
	case "port":
		// Handled by -p normally; ignore here (destination sets port).
	default:
		if opts.ExtraOptions == nil {
			opts.ExtraOptions = make(map[string]string)
		}
		opts.ExtraOptions[key] = val
	}
}

// tokenize splits a shell-style string into tokens, respecting single
// and double quotes and backslash escapes. Line continuations and inner
// newlines / tabs are treated as whitespace.
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
