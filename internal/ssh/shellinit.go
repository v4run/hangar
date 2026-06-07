package ssh

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/v4run/hangar/internal/config"
)

// ComposeShellInit concatenates the global, group, and connection ShellInit
// snippets in that order, separated by comment dividers. Empty layers are
// skipped. Returns "" if no layer contributes content.
//
// Later layers override earlier ones in standard bash semantics (the last
// alias/function definition wins), so connection-level overrides group, and
// group overrides global.
func ComposeShellInit(cfg *config.HangarConfig, conn *config.Connection) string {
	var parts []string
	useGlobal := true
	if conn != nil && conn.UseGlobalShellInit != nil {
		useGlobal = *conn.UseGlobalShellInit
	}
	if useGlobal && cfg != nil && strings.TrimSpace(cfg.GlobalShellInit) != "" {
		parts = append(parts, "# --- hangar: global ---\n"+cfg.GlobalShellInit)
	}
	if cfg != nil && conn != nil && conn.Group != "" {
		if g, ok := cfg.GroupShellInit[conn.Group]; ok && strings.TrimSpace(g) != "" {
			parts = append(parts, fmt.Sprintf("# --- hangar: group %q ---\n%s", conn.Group, g))
		}
	}
	if conn != nil && strings.TrimSpace(conn.ShellInit) != "" {
		parts = append(parts, fmt.Sprintf("# --- hangar: connection %q ---\n%s", conn.Name, conn.ShellInit))
	}
	if len(parts) == 0 {
		return ""
	}
	// Preserve the remote user's normal interactive shell setup: bash with
	// --rcfile sources FILE *instead of* ~/.bashrc, and we're not a login
	// shell so ~/.bash_profile / ~/.profile aren't read either. Source
	// ~/.bashrc first so hangar's layers add to rather than replace the
	// user's existing env/PATH/aliases. Also print the motd files sshd
	// would normally show on login — when ssh is given a remote command
	// the session is classed as exec and the motd / last-login banner is
	// suppressed.
	preamble := "# --- hangar: source user rc ---\n" +
		"[ -f ~/.bashrc ] && . ~/.bashrc\n" +
		"[ -r /etc/motd ] && cat /etc/motd 2>/dev/null\n" +
		"[ -r /run/motd.dynamic ] && cat /run/motd.dynamic 2>/dev/null\n"
	return preamble + strings.Join(parts, "\n") + "\n"
}

// buildRemoteShellInitArg returns the single remote-command argument that,
// when passed as the trailing arg to ssh, starts an interactive bash with the
// given snippet sourced as its rcfile. The snippet is base64-encoded into the
// command string to avoid all quoting/escaping concerns.
//
// The remote host must have bash and base64 on PATH.
func buildRemoteShellInitArg(snippet string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(snippet))
	return fmt.Sprintf(`bash --rcfile <(printf %%s %q | base64 -d) -i`, encoded)
}
