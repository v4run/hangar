package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
	sshauth "github.com/v4run/hangar/v2/internal/ssh"
)

type paletteState struct {
	kind        PaletteKind
	title       string
	message     string
	fields      []paletteField
	focus       int
	target      uuid.UUID // for edit/delete
	err         string
	tagFieldIdx int // index of the paletteField that renders chip-tags, -1 if none
}

type paletteField struct {
	label  string
	hint   string
	input  textinput.Model
	masked bool
	// choices, when non-empty, turns the field into a cycle picker:
	// typing is disabled, enter / l / →  cycles forward, h / ← back.
	choices []string
}

func (m *Model) closePalette() {
	m.palette = paletteState{}
	m.paletteOpts = nil
	m.paletteScriptIdx = 0
	m.paletteUseTA = false
	m.paletteUseTags = false
	m.paletteEditing = false
	m.paletteTextarea = textarea.Model{}
	m.overlay = OverlayNone
	m.mode = ModeBrowse
}

func (m *Model) focusField(i int) tea.Cmd {
	if i < 0 || i >= len(m.palette.fields) {
		return nil
	}
	m.palette.focus = i
	// In edit mode we actually hand focus to the field's input so
	// keystrokes land there. In nav mode no input is focused —
	// typing in nav mode is caught by the nav-mode key handler.
	for j := range m.palette.fields {
		if j == i && m.paletteEditing {
			m.palette.fields[j].input.Focus()
		} else {
			m.palette.fields[j].input.Blur()
		}
	}
	if m.paletteUseTags {
		if i == m.palette.tagFieldIdx && m.paletteEditing {
			m.paletteTags.Focus()
		} else {
			m.paletteTags.Blur()
		}
	}
	return textinput.Blink
}

func (m *Model) nextField() tea.Cmd {
	i := (m.palette.focus + 1) % len(m.palette.fields)
	return m.focusField(i)
}

func (m *Model) prevField() tea.Cmd {
	i := (m.palette.focus - 1 + len(m.palette.fields)) % len(m.palette.fields)
	return m.focusField(i)
}

func (m *Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Confirm-style palettes have no fields — handle them first.
	switch m.palette.kind {
	case PaletteDeleteConnection, PaletteDeleteDatabase,
		PaletteDeleteScript, PaletteDeleteForward:
		switch msg.String() {
		case "esc", "n", "N":
			m.closePalette()
			return *m, nil
		case "y", "Y", "ctrl+s":
			return m.submitPalette()
		}
		return *m, nil
	case PaletteQuitConfirm:
		switch msg.String() {
		case "esc", "n", "N":
			m.closePalette()
			return *m, nil
		case "y", "Y", "q":
			return *m, tea.Quit
		}
		return *m, nil
	case PaletteDeleteGroup:
		switch msg.String() {
		case "esc", "n", "N":
			m.closePalette()
			return *m, nil
		case "y", "ctrl+s":
			return m.deleteGroup(m.palette.target)
		case "Y":
			return m.deleteGroupCascade(m.palette.target)
		}
		return *m, nil
	case PaletteHostKey:
		switch msg.String() {
		case "esc", "n", "N":
			m.closePalette()
			m.pendingConn = nil
			m.pendingKeys = nil
			return *m, nil
		case "y", "Y", "enter":
			if len(m.pendingKeys) == 0 {
				return *m, nil // still scanning
			}
			return m.confirmHostKey()
		}
		return *m, nil
	case PaletteConfirmConnect:
		switch msg.String() {
		case "esc", "n", "N":
			m.closePalette()
			return *m, nil
		case "y", "Y", "enter":
			return m.confirmConnect()
		}
		return *m, nil
	}

	// ctrl+s saves from either nav or edit mode.
	if msg.String() == "ctrl+s" {
		return m.submitPalette()
	}

	// Multi-line textarea palette (shell-init). Textarea is always
	// "editing" because enter inserts a newline — esc closes the
	// palette directly.
	if m.paletteUseTA {
		if msg.String() == "esc" {
			m.closePalette()
			return *m, nil
		}
		var cmd tea.Cmd
		m.paletteTextarea, cmd = m.paletteTextarea.Update(msg)
		return *m, cmd
	}

	// Field grid palettes have two modes:
	//   nav  — j/k/tab/shift+tab select a field; enter enters edit;
	//          esc closes the palette; d/y actions still work for
	//          confirm-style palettes (handled above).
	//   edit — keystrokes go to the focused input/tagChips; enter
	//          commits the field (back to nav); esc also commits
	//          back to nav (not close).
	if len(m.palette.fields) == 0 {
		if msg.String() == "esc" {
			m.closePalette()
		}
		return *m, nil
	}

	if m.paletteEditing {
		cur := &m.palette.fields[m.palette.focus]
		switch msg.String() {
		case "enter":
			m.exitEditMode(true)
			return *m, nil
		case "esc":
			m.exitEditMode(false)
			return *m, nil
		}
		// Choice fields cycle on h / l / ← / → in edit mode; typing
		// is disabled.
		if len(cur.choices) > 0 {
			switch msg.String() {
			case "l", "right", " ":
				cycleChoiceField(cur, +1)
			case "h", "left":
				cycleChoiceField(cur, -1)
			}
			return *m, nil
		}
		// Tag-chip editor owns its field when active.
		if m.paletteUseTags && m.palette.focus == m.palette.tagFieldIdx {
			cmd, _ := m.paletteTags.Update(msg)
			return *m, cmd
		}
		var cmd tea.Cmd
		m.palette.fields[m.palette.focus].input, cmd = m.palette.fields[m.palette.focus].input.Update(msg)
		return *m, cmd
	}

	// Nav mode.
	switch msg.String() {
	case "esc":
		m.closePalette()
		return *m, nil
	case "j", "down", "tab":
		return *m, m.nextField()
	case "k", "up", "shift+tab":
		return *m, m.prevField()
	case "enter", "i":
		return *m, m.enterEditMode()
	case "g":
		m.palette.focus = 0
		return *m, m.focusField(0)
	case "G":
		return *m, m.focusField(len(m.palette.fields) - 1)
	}
	return *m, nil
}

// cycleChoiceField moves the field's current value to the next
// (dir=+1) or previous (-1) option in choices. If the current
// value isn't one of the choices, it lands on the first choice.
func cycleChoiceField(f *paletteField, dir int) {
	cur := f.input.Value()
	idx := -1
	for i, c := range f.choices {
		if c == cur {
			idx = i
			break
		}
	}
	n := len(f.choices)
	if idx < 0 {
		idx = 0
		dir = 0 // just land on first
	}
	idx = (idx + dir + n) % n
	f.input.SetValue(f.choices[idx])
}

// enterEditMode focuses the currently-selected field for text input
// and snapshots its value so esc can revert.
func (m *Model) enterEditMode() tea.Cmd {
	m.paletteEditing = true
	if m.palette.focus >= 0 && m.palette.focus < len(m.palette.fields) {
		m.paletteEditSnap = m.palette.fields[m.palette.focus].input.Value()
	}
	if m.paletteUseTags && m.palette.focus == m.palette.tagFieldIdx {
		return m.paletteTags.Focus()
	}
	if m.palette.focus >= 0 && m.palette.focus < len(m.palette.fields) {
		return m.palette.fields[m.palette.focus].input.Focus()
	}
	return nil
}

// exitEditMode blurs the active editor. commit=true keeps the
// in-field value (enter); commit=false restores the pre-edit
// snapshot (esc / ctrl+c).
func (m *Model) exitEditMode(commit bool) {
	if !commit && m.palette.focus >= 0 && m.palette.focus < len(m.palette.fields) {
		m.palette.fields[m.palette.focus].input.SetValue(m.paletteEditSnap)
		m.palette.fields[m.palette.focus].input.CursorEnd()
	}
	m.paletteEditing = false
	m.paletteEditSnap = ""
	for i := range m.palette.fields {
		m.palette.fields[i].input.Blur()
	}
	if m.paletteUseTags {
		m.paletteTags.Blur()
	}
}

// --- connection form ---

const (
	connFieldName = iota
	connFieldHost
	connFieldPort
	connFieldUser
	connFieldKey
	connFieldJump
	connFieldGroup
	connFieldTags
	connFieldPassword
	connFieldRemoteCmd
	connFieldEnv
	// --- Advanced SSH options (override the group / global chain) ---
	connFieldFwdAgent
	connFieldCompression
	connFieldLocalFwd
	connFieldRemoteFwd
	connFieldAlive
	connFieldAliveMax
	connFieldHostKey
	connFieldTTY
	connFieldExtra
	connFieldCount
)

var connFieldLabels = []string{
	"Name", "Host", "Port", "User", "Key", "Jump", "Group", "Tags", "Pass", "RemoteCmd", "Env",
	"FwdAgent", "Compress", "LocalFwd", "RemoteFwd", "Alive", "AliveMax", "HostKey", "TTY", "Extra",
}
var connFieldHints = []string{
	"required, display name",
	"required, hostname or ip",
	"22",
	"login user (inherited from group if blank)",
	"~/.ssh/id_ed25519",
	"name of a connection to use as jump host",
	"group name path, blank for ungrouped",
	"comma-separated",
	"stored in keychain; or $(cmd)",
	"run on remote after connect",
	"KEY=value, OTHER=val",
	"yes / no / blank",
	"yes / no / blank",
	"8080:localhost:80, 9090:localhost:90",
	"3000:localhost:3000",
	"ServerAliveInterval seconds",
	"ServerAliveCountMax",
	"yes / no / accept-new / blank",
	"yes / no / force / auto / blank",
	"KEY=value, OTHER=val",
}

func (m *Model) openAddConnectionPalette() tea.Cmd {
	cmd := m.openConnectionPalette(PaletteAddConnection, "New Connection", nil)
	// Prefill the Group field so a new connection drops into the
	// currently-selected group by default.
	if prefix := m.cursorGroupPathPrefix(); prefix != "" {
		// Strip the trailing slash — the Group field holds a path,
		// not a parent-of path.
		m.palette.fields[connFieldGroup].input.SetValue(strings.TrimRight(prefix, "/"))
		m.palette.fields[connFieldGroup].input.CursorEnd()
	}
	return cmd
}

func (m *Model) openEditConnectionPalette(c *config.Connection) tea.Cmd {
	return m.openConnectionPalette(PaletteEditConnection, "Edit "+c.Name, c)
}

func (m *Model) openConnectionPalette(kind PaletteKind, title string, c *config.Connection) tea.Cmd {
	fields := make([]paletteField, connFieldCount)
	for i := range fields {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = connFieldHints[i]
		if i == connFieldPassword {
			ti.EchoMode = textinput.EchoPassword
		}
		pf := paletteField{label: connFieldLabels[i], hint: connFieldHints[i], input: ti}
		switch i {
		case connFieldFwdAgent, connFieldCompression:
			pf.choices = yesNoChoices
		case connFieldHostKey:
			pf.choices = hostKeyChoices
		case connFieldTTY:
			pf.choices = requestTTYChoices
		}
		fields[i] = pf
	}
	var initialTags []string
	if c != nil {
		fields[connFieldName].input.SetValue(c.Name)
		fields[connFieldHost].input.SetValue(c.Host)
		if c.Port != 0 {
			fields[connFieldPort].input.SetValue(strconv.Itoa(c.Port))
		}
		fields[connFieldUser].input.SetValue(c.User)
		fields[connFieldKey].input.SetValue(c.IdentityFile)
		fields[connFieldGroup].input.SetValue(config.GroupPath(m.cfg, c.Group))
		initialTags = append([]string(nil), c.Tags...)
		if c.Password != "" {
			fields[connFieldPassword].input.SetValue(c.Password)
		} else if pw, _ := sshauth.GetPassword(c.ID.String()); pw != "" {
			fields[connFieldPassword].input.SetValue(pw)
		}
		fields[connFieldRemoteCmd].input.SetValue(c.RemoteCommand)
		if c.JumpHost != uuid.Nil {
			if j := findConnByID(m.cfg, c.JumpHost); j != nil {
				fields[connFieldJump].input.SetValue(j.Name)
			}
		}
		if len(c.Env) > 0 {
			fields[connFieldEnv].input.SetValue(envMapToString(c.Env))
		}
		if o := c.SSHOptions; o != nil {
			if o.ForwardAgent != nil {
				fields[connFieldFwdAgent].input.SetValue(boolToYesNo(*o.ForwardAgent))
			}
			if o.Compression != nil {
				fields[connFieldCompression].input.SetValue(boolToYesNo(*o.Compression))
			}
			fields[connFieldLocalFwd].input.SetValue(strings.Join(o.LocalForward, ", "))
			fields[connFieldRemoteFwd].input.SetValue(strings.Join(o.RemoteForward, ", "))
			if o.ServerAliveInterval != nil {
				fields[connFieldAlive].input.SetValue(strconv.Itoa(*o.ServerAliveInterval))
			}
			if o.ServerAliveCountMax != nil {
				fields[connFieldAliveMax].input.SetValue(strconv.Itoa(*o.ServerAliveCountMax))
			}
			fields[connFieldHostKey].input.SetValue(o.StrictHostKeyCheck)
			fields[connFieldTTY].input.SetValue(o.RequestTTY)
			if len(o.ExtraOptions) > 0 {
				parts := make([]string, 0, len(o.ExtraOptions))
				for k, v := range o.ExtraOptions {
					parts = append(parts, k+"="+v)
				}
				fields[connFieldExtra].input.SetValue(strings.Join(parts, ", "))
			}
		}
	}
	for i := range fields {
		fields[i].input.CursorEnd()
	}
	m.palette = paletteState{kind: kind, title: title, fields: fields, tagFieldIdx: connFieldTags}
	m.paletteTags = newTagChips(initialTags)
	m.paletteUseTags = true
	if c != nil {
		m.palette.target = c.ID
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) openDeleteConnectionPalette(c *config.Connection) tea.Cmd {
	m.palette = paletteState{
		kind:    PaletteDeleteConnection,
		title:   "Delete connection",
		message: "Delete " + c.Name + "? (y/n)",
		target:  c.ID,
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return nil
}

// --- group form ---

const (
	groupFieldPath = iota
	groupFieldUser
	groupFieldKey
	groupFieldJump
	groupFieldFwdAgent
	groupFieldCompression
	groupFieldLocalFwd
	groupFieldRemoteFwd
	groupFieldTags
	groupFieldEnv
	groupFieldCount
)

var groupFieldLabels = []string{
	"Path", "User", "Key", "Jump", "FwdAgent", "Compress", "LocalFwd", "RemoteFwd", "Tags", "Env",
}
var groupFieldHints = []string{
	"slash-separated for nesting (prod/us-east)",
	"default user (inherited by children + connections)",
	"default identity file",
	"name of a connection to use as jump host",
	"yes / no / blank",
	"yes / no / blank",
	"8080:localhost:80, 9090:localhost:90",
	"3000:localhost:3000",
	"",
	"KEY=value, OTHER=val",
}

func (m *Model) openAddGroupPalette() tea.Cmd {
	cmd := m.openGroupPalette(PaletteAddGroup, "New Group", nil)
	// Prefill the Path field with the selected group's path (or the
	// group containing the selected connection) + "/" so adding a
	// nested group is just typing the leaf name.
	if prefix := m.cursorGroupPathPrefix(); prefix != "" {
		m.palette.fields[groupFieldPath].input.SetValue(prefix)
		m.palette.fields[groupFieldPath].input.CursorEnd()
	}
	return cmd
}

// cursorGroupPathPrefix returns the group path (ending in "/") of
// the currently-selected row, if any — so new-group / new-connection
// forms can prefill with "prod/us-east/". Returns "" when nothing
// useful is selected.
func (m *Model) cursorGroupPathPrefix() string {
	if g := m.selectedGroup(); g != nil {
		if path := config.GroupPath(m.cfg, g.ID); path != "" {
			return path + "/"
		}
	}
	if c := m.selectedConnection(); c != nil && c.Group != uuid.Nil {
		if path := config.GroupPath(m.cfg, c.Group); path != "" {
			return path + "/"
		}
	}
	if d := m.selectedDatabase(); d != nil && d.Group != uuid.Nil {
		if path := config.GroupPath(m.cfg, d.Group); path != "" {
			return path + "/"
		}
	}
	return ""
}

func (m *Model) openEditGroupPalette(g *config.Group) tea.Cmd {
	return m.openGroupPalette(PaletteEditGroup, "Edit "+g.Name, g)
}

func (m *Model) openGroupPalette(kind PaletteKind, title string, g *config.Group) tea.Cmd {
	fields := make([]paletteField, groupFieldCount)
	for i := range fields {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = groupFieldHints[i]
		pf := paletteField{label: groupFieldLabels[i], hint: groupFieldHints[i], input: ti}
		switch i {
		case groupFieldFwdAgent, groupFieldCompression:
			pf.choices = yesNoChoices
		}
		fields[i] = pf
	}
	var initialTags []string
	if g != nil {
		fields[groupFieldPath].input.SetValue(config.GroupPath(m.cfg, g.ID))
		fields[groupFieldUser].input.SetValue(g.User)
		fields[groupFieldKey].input.SetValue(g.IdentityFile)
		if g.JumpHost != uuid.Nil {
			if j := findConnByID(m.cfg, g.JumpHost); j != nil {
				fields[groupFieldJump].input.SetValue(j.Name)
			}
		}
		if g.SSHOptions != nil {
			if g.SSHOptions.ForwardAgent != nil {
				fields[groupFieldFwdAgent].input.SetValue(boolToYesNo(*g.SSHOptions.ForwardAgent))
			}
			if g.SSHOptions.Compression != nil {
				fields[groupFieldCompression].input.SetValue(boolToYesNo(*g.SSHOptions.Compression))
			}
			fields[groupFieldLocalFwd].input.SetValue(strings.Join(g.SSHOptions.LocalForward, ", "))
			fields[groupFieldRemoteFwd].input.SetValue(strings.Join(g.SSHOptions.RemoteForward, ", "))
		}
		initialTags = append([]string(nil), g.Tags...)
		if len(g.Env) > 0 {
			fields[groupFieldEnv].input.SetValue(envMapToString(g.Env))
		}
	}
	for i := range fields {
		fields[i].input.CursorEnd()
	}
	m.palette = paletteState{kind: kind, title: title, fields: fields, tagFieldIdx: groupFieldTags}
	m.paletteTags = newTagChips(initialTags)
	m.paletteUseTags = true
	if g != nil {
		m.palette.target = g.ID
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func parseYesNo(s string) *bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "true", "1":
		b := true
		return &b
	case "no", "n", "false", "0":
		b := false
		return &b
	}
	return nil
}

// --- submit ---

func (m *Model) submitPalette() (tea.Model, tea.Cmd) {
	switch m.palette.kind {
	case PaletteAddConnection:
		return m.saveConnection(uuid.Nil)
	case PaletteEditConnection:
		return m.saveConnection(m.palette.target)
	case PaletteDeleteConnection:
		return m.deleteConnection(m.palette.target)
	case PaletteAddGroup:
		return m.saveGroup(uuid.Nil)
	case PaletteEditGroup:
		return m.saveGroup(m.palette.target)
	case PaletteHostKey:
		return m.confirmHostKey()
	case PaletteConfirmConnect:
		return m.confirmConnect()
	case PaletteImportSSH:
		return m.submitImportSSH()
	case PaletteNotes:
		return m.submitNotes()
	case PaletteScript:
		return m.submitScript()
	case PaletteAddDatabase:
		return m.saveDatabase(uuid.Nil)
	case PaletteEditDatabase:
		return m.saveDatabase(m.palette.target)
	case PaletteDeleteDatabase:
		return m.deleteDatabase(m.palette.target)
	case PaletteDeleteGroup:
		// Handled directly in handlePaletteKey (two actions: reparent
		// vs cascade). submitPalette shouldn't be reached for this
		// palette, but if ctrl+s falls through we default to the
		// non-destructive re-parent.
		return m.deleteGroup(m.palette.target)
	case PaletteGlobalSettings:
		return m.saveGlobalSettings()
	case PaletteShellInit:
		return m.saveShellInit()
	case PaletteAddForward:
		return m.saveForward()
	case PaletteDeleteScript:
		return m.deleteScript()
	case PaletteDeleteForward:
		return m.deleteForward()
	}
	return *m, nil
}

func (m *Model) openDeleteScriptPalette(c *config.Connection, idx int) tea.Cmd {
	if idx < 0 || idx >= len(c.Scripts) {
		return nil
	}
	s := c.Scripts[idx]
	m.palette = paletteState{
		kind:    PaletteDeleteScript,
		title:   "Delete script",
		message: fmt.Sprintf("Delete %q (%s)? (y/n)", s.Name, s.Command),
		target:  c.ID,
	}
	m.paletteScriptIdx = idx
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return nil
}

func (m *Model) deleteScript() (tea.Model, tea.Cmd) {
	idx := m.paletteScriptIdx
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID != m.palette.target {
			continue
		}
		c := &m.cfg.Connections[i]
		if idx < 0 || idx >= len(c.Scripts) {
			m.closePalette()
			return *m, nil
		}
		name := c.Scripts[idx].Name
		c.Scripts = append(c.Scripts[:idx], c.Scripts[idx+1:]...)
		if err := config.Save(m.cfgDir, m.cfg); err != nil {
			m.palette.err = err.Error()
			return *m, nil
		}
		m.closePalette()
		if m.scriptCursor >= len(c.Scripts) && m.scriptCursor > 0 {
			m.scriptCursor--
		}
		m.setToast("deleted script "+name, toastOK)
		return *m, nil
	}
	m.closePalette()
	return *m, nil
}

func (m *Model) openDeleteForwardPalette(c *config.Connection, dir string, idx int) tea.Cmd {
	// idx is relative to the connection's OWN forward list; inherited
	// forwards aren't deletable here (they live on the parent group).
	var list []string
	if c.SSHOptions != nil {
		if dir == "L" {
			list = c.SSHOptions.LocalForward
		} else {
			list = c.SSHOptions.RemoteForward
		}
	}
	if idx < 0 || idx >= len(list) {
		m.setToast("that forward is inherited — edit the group instead", toastInfo)
		return nil
	}
	m.palette = paletteState{
		kind:    PaletteDeleteForward,
		title:   "Delete port forward",
		message: fmt.Sprintf("Delete %s %s? (y/n)", dir, list[idx]),
		target:  c.ID,
	}
	m.paletteForwardIdx = idx
	m.paletteForwardDir = dir
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return nil
}

func (m *Model) deleteForward() (tea.Model, tea.Cmd) {
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID != m.palette.target {
			continue
		}
		c := &m.cfg.Connections[i]
		if c.SSHOptions == nil {
			m.closePalette()
			return *m, nil
		}
		idx := m.paletteForwardIdx
		if m.paletteForwardDir == "L" {
			if idx >= 0 && idx < len(c.SSHOptions.LocalForward) {
				c.SSHOptions.LocalForward = append(c.SSHOptions.LocalForward[:idx], c.SSHOptions.LocalForward[idx+1:]...)
			}
		} else {
			if idx >= 0 && idx < len(c.SSHOptions.RemoteForward) {
				c.SSHOptions.RemoteForward = append(c.SSHOptions.RemoteForward[:idx], c.SSHOptions.RemoteForward[idx+1:]...)
			}
		}
		if err := config.Save(m.cfgDir, m.cfg); err != nil {
			m.palette.err = err.Error()
			return *m, nil
		}
		m.closePalette()
		m.setToast("forward removed", toastOK)
		return *m, nil
	}
	m.closePalette()
	return *m, nil
}

// --- port forward add ---

func (m *Model) openAddForwardPalette(c *config.Connection) tea.Cmd {
	dir := textinput.New()
	dir.Prompt = ""
	dir.Placeholder = "L or R (local or remote)"
	spec := textinput.New()
	spec.Prompt = ""
	spec.Placeholder = "8080:localhost:80"
	m.palette = paletteState{
		kind:  PaletteAddForward,
		title: "Add port forward — " + c.Name,
		fields: []paletteField{
			{label: "Direction", input: dir},
			{label: "Spec", input: spec},
		},
		target: c.ID,
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) saveForward() (tea.Model, tea.Cmd) {
	dir := strings.ToUpper(strings.TrimSpace(m.palette.fields[0].input.Value()))
	spec := strings.TrimSpace(m.palette.fields[1].input.Value())
	if dir != "L" && dir != "R" {
		m.palette.err = "direction must be L or R"
		return *m, nil
	}
	if spec == "" {
		m.palette.err = "spec is required"
		return *m, nil
	}
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID != m.palette.target {
			continue
		}
		c := &m.cfg.Connections[i]
		if c.SSHOptions == nil {
			c.SSHOptions = &config.SSHOptions{}
		}
		if dir == "L" {
			c.SSHOptions.LocalForward = append(c.SSHOptions.LocalForward, spec)
		} else {
			c.SSHOptions.RemoteForward = append(c.SSHOptions.RemoteForward, spec)
		}
		break
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.closePalette()
	m.setToast("forward added", toastOK)
	return *m, nil
}

// --- global settings ---

const (
	gsFieldFwdAgent = iota
	gsFieldCompression
	gsFieldLocalFwd
	gsFieldRemoteFwd
	gsFieldAlive
	gsFieldAliveMax
	gsFieldHostKey
	gsFieldTTY
	gsFieldExtra
	gsFieldEnv
	gsFieldCount
)

var gsFieldLabels = []string{
	"FwdAgent", "Compress", "LocalFwd", "RemoteFwd",
	"Alive", "AliveMax", "HostKey", "TTY", "Extra", "Env",
}
var gsFieldHints = []string{
	"yes / no / blank",
	"yes / no / blank",
	"8080:localhost:80, 9090:localhost:90",
	"3000:localhost:3000",
	"ServerAliveInterval seconds",
	"ServerAliveCountMax",
	"yes / no / accept-new / blank",
	"yes / no / force / auto / blank",
	"KEY=value, OTHER=val",
	"KEY=value, OTHER=val",
}

func (m *Model) openGlobalSettingsPalette() tea.Cmd {
	fields := make([]paletteField, gsFieldCount)
	for i := range fields {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = gsFieldHints[i]
		pf := paletteField{label: gsFieldLabels[i], hint: gsFieldHints[i], input: ti}
		switch i {
		case gsFieldFwdAgent, gsFieldCompression:
			pf.choices = yesNoChoices
		case gsFieldHostKey:
			pf.choices = hostKeyChoices
		case gsFieldTTY:
			pf.choices = requestTTYChoices
		}
		fields[i] = pf
	}
	if o := m.cfg.SSHOptions; o != nil {
		if o.ForwardAgent != nil {
			fields[gsFieldFwdAgent].input.SetValue(boolToYesNo(*o.ForwardAgent))
		}
		if o.Compression != nil {
			fields[gsFieldCompression].input.SetValue(boolToYesNo(*o.Compression))
		}
		fields[gsFieldLocalFwd].input.SetValue(strings.Join(o.LocalForward, ", "))
		fields[gsFieldRemoteFwd].input.SetValue(strings.Join(o.RemoteForward, ", "))
		if o.ServerAliveInterval != nil {
			fields[gsFieldAlive].input.SetValue(strconv.Itoa(*o.ServerAliveInterval))
		}
		if o.ServerAliveCountMax != nil {
			fields[gsFieldAliveMax].input.SetValue(strconv.Itoa(*o.ServerAliveCountMax))
		}
		fields[gsFieldHostKey].input.SetValue(o.StrictHostKeyCheck)
		fields[gsFieldTTY].input.SetValue(o.RequestTTY)
		if len(o.ExtraOptions) > 0 {
			parts := make([]string, 0, len(o.ExtraOptions))
			for k, v := range o.ExtraOptions {
				parts = append(parts, k+"="+v)
			}
			fields[gsFieldExtra].input.SetValue(strings.Join(parts, ", "))
		}
	}
	if len(m.cfg.Env) > 0 {
		fields[gsFieldEnv].input.SetValue(envMapToString(m.cfg.Env))
	}
	for i := range fields {
		fields[i].input.CursorEnd()
	}
	m.palette = paletteState{kind: PaletteGlobalSettings, title: "Global SSH settings", fields: fields}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) saveGlobalSettings() (tea.Model, tea.Cmd) {
	opts := &config.SSHOptions{}
	touched := false

	if v := parseYesNo(m.palette.fields[gsFieldFwdAgent].input.Value()); v != nil {
		opts.ForwardAgent = v
		touched = true
	}
	if v := parseYesNo(m.palette.fields[gsFieldCompression].input.Value()); v != nil {
		opts.Compression = v
		touched = true
	}
	if lf := splitAndTrim(m.palette.fields[gsFieldLocalFwd].input.Value(), ","); len(lf) > 0 {
		opts.LocalForward = lf
		touched = true
	}
	if rf := splitAndTrim(m.palette.fields[gsFieldRemoteFwd].input.Value(), ","); len(rf) > 0 {
		opts.RemoteForward = rf
		touched = true
	}
	if s := strings.TrimSpace(m.palette.fields[gsFieldAlive].input.Value()); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			opts.ServerAliveInterval = &n
			touched = true
		} else {
			m.palette.err = "Alive must be an integer"
			return *m, nil
		}
	}
	if s := strings.TrimSpace(m.palette.fields[gsFieldAliveMax].input.Value()); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			opts.ServerAliveCountMax = &n
			touched = true
		} else {
			m.palette.err = "AliveMax must be an integer"
			return *m, nil
		}
	}
	if hk := strings.TrimSpace(m.palette.fields[gsFieldHostKey].input.Value()); hk != "" {
		opts.StrictHostKeyCheck = hk
		touched = true
	}
	if tty := strings.TrimSpace(m.palette.fields[gsFieldTTY].input.Value()); tty != "" {
		opts.RequestTTY = tty
		touched = true
	}
	if ex := strings.TrimSpace(m.palette.fields[gsFieldExtra].input.Value()); ex != "" {
		opts.ExtraOptions = map[string]string{}
		for _, pair := range splitAndTrim(ex, ",") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			opts.ExtraOptions[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
		if len(opts.ExtraOptions) > 0 {
			touched = true
		}
	}
	if touched {
		m.cfg.SSHOptions = opts
	} else {
		m.cfg.SSHOptions = nil
	}
	m.cfg.Env = parseEnvMap(m.palette.fields[gsFieldEnv].input.Value())
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.closePalette()
	m.setToast("global settings saved", toastOK)
	return *m, nil
}

// openQuitConfirm presents a y/n palette instead of exiting
// immediately — bound to q. ctrl+c still quits without confirming
// (power-user escape hatch).
func (m *Model) openQuitConfirm() tea.Cmd {
	m.palette = paletteState{
		kind:    PaletteQuitConfirm,
		title:   "Quit Hangar?",
		message: "Any unsaved form inputs will be discarded.",
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return nil
}

func (m *Model) openDeleteGroupPalette(g *config.Group) tea.Cmd {
	// Count the whole subtree so the prompt is specific.
	descGroups, descConns, descDBs := m.groupDescendantCounts(g.ID)
	// Direct children (what re-parenting would surface at this
	// level — informational).
	directKids := 0
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].Parent == g.ID {
			directKids++
		}
	}
	directConns := 0
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].Group == g.ID {
			directConns++
		}
	}
	msg := fmt.Sprintf(
		"Delete group %q.\n\n"+
			"  y        re-parent %d direct subgroup(s) + %d direct connection(s) to its parent.\n"+
			"  Y        cascade: delete this group AND everything beneath it\n"+
			"             (%d subgroup(s), %d connection(s), %d database(s)).\n"+
			"  n / esc  cancel",
		g.Name, directKids, directConns, descGroups, descConns, descDBs,
	)
	m.palette = paletteState{
		kind:    PaletteDeleteGroup,
		title:   "Delete group",
		message: msg,
		target:  g.ID,
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return nil
}

// groupDescendantCounts returns (groups, connections, databases)
// strictly below root — root itself is NOT counted.
func (m *Model) groupDescendantCounts(root uuid.UUID) (int, int, int) {
	kids := map[uuid.UUID][]uuid.UUID{}
	for _, g := range m.cfg.Groups {
		kids[g.Parent] = append(kids[g.Parent], g.ID)
	}
	// Collect every descendant group id.
	var walk func(id uuid.UUID) []uuid.UUID
	walk = func(id uuid.UUID) []uuid.UUID {
		out := []uuid.UUID{id}
		for _, c := range kids[id] {
			out = append(out, walk(c)...)
		}
		return out
	}
	all := walk(root) // includes root
	member := map[uuid.UUID]bool{}
	for _, id := range all {
		member[id] = true
	}
	var conns, dbs int
	for i := range m.cfg.Connections {
		if member[m.cfg.Connections[i].Group] {
			conns++
		}
	}
	for i := range m.cfg.Databases {
		if member[m.cfg.Databases[i].Group] {
			dbs++
		}
	}
	return len(all) - 1, conns, dbs
}

// deleteGroupCascade removes the group plus every descendant group,
// connection and database. Keychain passwords for every removed row
// are cleared. Non-reversible — the delete palette makes the user
// hit a distinct `D` to invoke it (not `y`) specifically to avoid a
// mis-fire.
func (m *Model) deleteGroupCascade(id uuid.UUID) (tea.Model, tea.Cmd) {
	kids := map[uuid.UUID][]uuid.UUID{}
	for _, g := range m.cfg.Groups {
		kids[g.Parent] = append(kids[g.Parent], g.ID)
	}
	var walk func(x uuid.UUID) []uuid.UUID
	walk = func(x uuid.UUID) []uuid.UUID {
		out := []uuid.UUID{x}
		for _, c := range kids[x] {
			out = append(out, walk(c)...)
		}
		return out
	}
	all := walk(id)
	member := map[uuid.UUID]bool{}
	for _, g := range all {
		member[g] = true
	}
	// Prune connections + clear their keychain entries.
	remainingConns := m.cfg.Connections[:0]
	connDeleted := 0
	for _, c := range m.cfg.Connections {
		if member[c.Group] {
			_ = sshauth.DeletePassword(c.ID.String())
			connDeleted++
			continue
		}
		remainingConns = append(remainingConns, c)
	}
	m.cfg.Connections = remainingConns
	// Prune databases.
	remainingDBs := m.cfg.Databases[:0]
	dbDeleted := 0
	for _, d := range m.cfg.Databases {
		if member[d.Group] {
			_ = sshauth.DeletePassword(d.ID.String())
			dbDeleted++
			continue
		}
		remainingDBs = append(remainingDBs, d)
	}
	m.cfg.Databases = remainingDBs
	// Prune groups.
	remainingGroups := m.cfg.Groups[:0]
	for _, g := range m.cfg.Groups {
		if member[g.ID] {
			continue
		}
		remainingGroups = append(remainingGroups, g)
	}
	m.cfg.Groups = remainingGroups
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.rebuildRows()
	m.closePalette()
	m.setToast(fmt.Sprintf("deleted %d group(s), %d connection(s), %d database(s)", len(all), connDeleted, dbDeleted), toastOK)
	return *m, nil
}

func (m *Model) deleteGroup(id uuid.UUID) (tea.Model, tea.Cmd) {
	var parent uuid.UUID
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].ID == id {
			parent = m.cfg.Groups[i].Parent
			break
		}
	}
	// Re-parent children + connections + databases to this group's
	// parent (preserves the user's work — nothing is orphaned).
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].Parent == id {
			m.cfg.Groups[i].Parent = parent
		}
	}
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].Group == id {
			m.cfg.Connections[i].Group = parent
		}
	}
	for i := range m.cfg.Databases {
		if m.cfg.Databases[i].Group == id {
			m.cfg.Databases[i].Group = parent
		}
	}
	// Remove the group row itself.
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].ID == id {
			name := m.cfg.Groups[i].Name
			m.cfg.Groups = append(m.cfg.Groups[:i], m.cfg.Groups[i+1:]...)
			if err := config.Save(m.cfgDir, m.cfg); err != nil {
				m.palette.err = err.Error()
				return *m, nil
			}
			m.rebuildRows()
			m.closePalette()
			m.setToast("deleted group "+name, toastOK)
			return *m, nil
		}
	}
	m.closePalette()
	return *m, nil
}

// --- shell init multi-line editor ---

func (m *Model) openShellInitPalette(scope shellInitScope, target uuid.UUID, initial string) tea.Cmd {
	ta := textarea.New()
	ta.Placeholder = "bash lines — enter adds a newline, ctrl+s saves, esc cancels"
	ta.CharLimit = 0
	ta.SetWidth(70)
	ta.SetHeight(10)
	ta.SetValue(initial)
	m.palette = paletteState{
		kind:   PaletteShellInit,
		title:  shellInitTitle(scope),
		target: target,
	}
	m.paletteTextarea = ta
	m.paletteUseTA = true
	m.shellInitScopeSel = scope
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.paletteTextarea.Focus()
}

type shellInitScope int

const (
	shellInitScopeConnection shellInitScope = iota
	shellInitScopeGroup
	shellInitScopeGlobal
)

func shellInitTitle(s shellInitScope) string {
	switch s {
	case shellInitScopeGroup:
		return "Group shell init"
	case shellInitScopeGlobal:
		return "Global shell init"
	}
	return "Connection shell init"
}

func (m *Model) saveShellInit() (tea.Model, tea.Cmd) {
	val := m.paletteTextarea.Value()
	switch m.shellInitScopeSel {
	case shellInitScopeGlobal:
		m.cfg.ShellInit = val
	case shellInitScopeGroup:
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].ID == m.palette.target {
				m.cfg.Groups[i].ShellInit = val
				break
			}
		}
	default:
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].ID == m.palette.target {
				m.cfg.Connections[i].ShellInit = val
				break
			}
		}
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.closePalette()
	m.setToast("shell init saved", toastOK)
	return *m, nil
}

// --- SSH command import ---

func (m *Model) openImportSSHPalette() tea.Cmd {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "paste a full ssh command"
	m.palette = paletteState{
		kind:   PaletteImportSSH,
		title:  "Import SSH command",
		fields: []paletteField{{label: "Cmd", input: ti}},
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) submitImportSSH() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.palette.fields[0].input.Value())
	if raw == "" {
		m.palette.err = "paste an ssh command first"
		return *m, nil
	}
	res, err := sshauth.ParseSSHCommand(raw)
	if err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	// Switch to the add-connection palette prefilled from the parse.
	m.closePalette()
	cmd := m.openAddConnectionPalette()
	// Prefill form fields from the parsed Connection.
	c := res.Connection
	m.palette.fields[connFieldName].input.SetValue(c.Name)
	m.palette.fields[connFieldHost].input.SetValue(c.Host)
	if c.Port != 0 && c.Port != 22 {
		m.palette.fields[connFieldPort].input.SetValue(strconv.Itoa(c.Port))
	}
	m.palette.fields[connFieldUser].input.SetValue(c.User)
	m.palette.fields[connFieldKey].input.SetValue(c.IdentityFile)
	// The parser can't resolve a jump host to a UUID; stash the raw
	// label as a palette message so the user knows to wire it after.
	if res.JumpHost != "" {
		m.palette.message = "parsed -J " + res.JumpHost + " — set the Jump target manually after save"
	}
	// Pin the SSHOptions on the newly-opened palette so saveConnection
	// can carry them forward.
	m.paletteOpts = c.SSHOptions
	for i := range m.palette.fields {
		m.palette.fields[i].input.CursorEnd()
	}
	return *m, cmd
}

// --- notes ---

func (m *Model) openNotesPalette(c *config.Connection) tea.Cmd {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "notes"
	ti.SetValue(c.Notes)
	ti.CursorEnd()
	m.palette = paletteState{
		kind:   PaletteNotes,
		title:  "Notes — " + c.Name,
		fields: []paletteField{{label: "Notes", input: ti}},
		target: c.ID,
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) submitNotes() (tea.Model, tea.Cmd) {
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID == m.palette.target {
			m.cfg.Connections[i].Notes = strings.TrimSpace(m.palette.fields[0].input.Value())
			break
		}
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.closePalette()
	m.setToast("notes saved", toastOK)
	return *m, nil
}

// --- scripts ---

type scriptPaletteContext struct {
	connID uuid.UUID
	index  int // -1 for new
}

func (m *Model) openAddScriptPalette(c *config.Connection) tea.Cmd {
	return m.openScriptPalette(c, -1)
}

func (m *Model) openEditScriptPalette(c *config.Connection, idx int) tea.Cmd {
	return m.openScriptPalette(c, idx)
}

func (m *Model) openScriptPalette(c *config.Connection, idx int) tea.Cmd {
	nameTi := textinput.New()
	nameTi.Prompt = ""
	nameTi.Placeholder = "label"
	cmdTi := textinput.New()
	cmdTi.Prompt = ""
	cmdTi.Placeholder = "command to run on remote"
	if idx >= 0 && idx < len(c.Scripts) {
		nameTi.SetValue(c.Scripts[idx].Name)
		cmdTi.SetValue(c.Scripts[idx].Command)
	}
	nameTi.CursorEnd()
	cmdTi.CursorEnd()
	m.palette = paletteState{
		kind:  PaletteScript,
		title: "Script — " + c.Name,
		fields: []paletteField{
			{label: "Name", input: nameTi},
			{label: "Cmd", input: cmdTi},
		},
		target: c.ID,
	}
	m.paletteScriptIdx = idx
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) submitScript() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.palette.fields[0].input.Value())
	cmd := strings.TrimSpace(m.palette.fields[1].input.Value())
	if name == "" || cmd == "" {
		m.palette.err = "name and command are required"
		return *m, nil
	}
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID != m.palette.target {
			continue
		}
		c := &m.cfg.Connections[i]
		if m.paletteScriptIdx < 0 {
			c.Scripts = append(c.Scripts, config.Script{Name: name, Command: cmd})
		} else if m.paletteScriptIdx < len(c.Scripts) {
			c.Scripts[m.paletteScriptIdx] = config.Script{Name: name, Command: cmd}
		}
		break
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.closePalette()
	m.setToast("script saved", toastOK)
	return *m, nil
}

// --- databases ---

const (
	dbFieldEngine = iota
	dbFieldName
	dbFieldHost
	dbFieldPort
	dbFieldUser
	dbFieldDBName
	dbFieldGroup
	dbFieldPassword
	dbFieldCount
)

var dbFieldLabels = []string{"Engine", "Name", "Host", "Port", "User", "DBName", "Group", "Pass"}
var dbFieldHints = []string{
	"engine (h/l to cycle)",
	"required",
	"host or /path/to/file for sqlite",
	"",
	"",
	"",
	"group path, blank for ungrouped",
	"stored in keychain; or $(cmd)",
}

var dbEngineChoices = []string{"postgres", "mysql", "redis", "sqlite"}

// Choice sets for the SSH advanced fields. Blank is explicitly a
// choice — it means "not set at this scope, inherit from the chain".
var (
	yesNoChoices       = []string{"", "yes", "no"}
	hostKeyChoices     = []string{"", "yes", "no", "accept-new"}
	requestTTYChoices  = []string{"", "yes", "no", "force", "auto"}
)

func (m *Model) openAddDatabasePalette() tea.Cmd {
	return m.openDatabasePalette(PaletteAddDatabase, "New Database", nil)
}
func (m *Model) openEditDatabasePalette(d *config.Database) tea.Cmd {
	return m.openDatabasePalette(PaletteEditDatabase, "Edit "+d.Name, d)
}
func (m *Model) openDatabasePalette(kind PaletteKind, title string, d *config.Database) tea.Cmd {
	fields := make([]paletteField, dbFieldCount)
	for i := range fields {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = dbFieldHints[i]
		if i == dbFieldPassword {
			ti.EchoMode = textinput.EchoPassword
		}
		if i == dbFieldEngine {
			ti.SetValue(dbEngineChoices[0])
		}
		pf := paletteField{label: dbFieldLabels[i], hint: dbFieldHints[i], input: ti}
		if i == dbFieldEngine {
			pf.choices = dbEngineChoices
		}
		fields[i] = pf
	}
	if d != nil {
		fields[dbFieldEngine].input.SetValue(d.Engine)
		fields[dbFieldName].input.SetValue(d.Name)
		fields[dbFieldHost].input.SetValue(d.Host)
		if d.Port != 0 {
			fields[dbFieldPort].input.SetValue(strconv.Itoa(d.Port))
		}
		fields[dbFieldUser].input.SetValue(d.User)
		fields[dbFieldDBName].input.SetValue(d.DBName)
		fields[dbFieldGroup].input.SetValue(config.GroupPath(m.cfg, d.Group))
		if d.Password != "" {
			fields[dbFieldPassword].input.SetValue(d.Password)
		} else if pw, _ := sshauth.GetPassword(d.ID.String()); pw != "" {
			fields[dbFieldPassword].input.SetValue(pw)
		}
	}
	for i := range fields {
		fields[i].input.CursorEnd()
	}
	m.palette = paletteState{kind: kind, title: title, fields: fields}
	if d != nil {
		m.palette.target = d.ID
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) openDeleteDatabasePalette(d *config.Database) tea.Cmd {
	m.palette = paletteState{
		kind:    PaletteDeleteDatabase,
		title:   "Delete database",
		message: "Delete " + d.Name + "? (y/n)",
		target:  d.ID,
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return nil
}

func (m *Model) saveDatabase(id uuid.UUID) (tea.Model, tea.Cmd) {
	engine := strings.ToLower(strings.TrimSpace(m.palette.fields[dbFieldEngine].input.Value()))
	name := strings.TrimSpace(m.palette.fields[dbFieldName].input.Value())
	if name == "" {
		m.palette.err = "name is required"
		return *m, nil
	}
	switch engine {
	case "postgres", "mysql", "redis", "sqlite":
	default:
		m.palette.err = "engine must be postgres, mysql, redis, or sqlite"
		return *m, nil
	}
	port := 0
	if ps := strings.TrimSpace(m.palette.fields[dbFieldPort].input.Value()); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil {
			m.palette.err = "port must be an integer"
			return *m, nil
		}
		port = n
	}
	gpath := strings.TrimSpace(m.palette.fields[dbFieldGroup].input.Value())
	gID, err := m.resolveOrCreateGroupPath(gpath)
	if err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	password := strings.TrimSpace(m.palette.fields[dbFieldPassword].input.Value())
	d := config.Database{
		Name:   name,
		Engine: engine,
		Host:   strings.TrimSpace(m.palette.fields[dbFieldHost].input.Value()),
		Port:   port,
		User:   strings.TrimSpace(m.palette.fields[dbFieldUser].input.Value()),
		DBName: strings.TrimSpace(m.palette.fields[dbFieldDBName].input.Value()),
		Group:  gID,
	}
	if sshauth.IsCommandValue(password) {
		d.Password = password
	}
	if id == uuid.Nil {
		d.ID = uuid.New()
		m.cfg.Databases = append(m.cfg.Databases, d)
	} else {
		d.ID = id
		for i := range m.cfg.Databases {
			if m.cfg.Databases[i].ID == id {
				// Preserve fields the DB palette doesn't own —
				// otherwise edit would silently clear them.
				existing := m.cfg.Databases[i]
				d.Tags = existing.Tags
				d.Notes = existing.Notes
				d.TunnelVia = existing.TunnelVia
				d.Env = existing.Env
				d.Order = existing.Order
				m.cfg.Databases[i] = d
				break
			}
		}
	}
	if password != "" && !sshauth.IsCommandValue(password) {
		_ = sshauth.SetPassword(d.ID.String(), password)
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.rebuildRows()
	m.closePalette()
	m.setToast("saved "+d.Name, toastOK)
	return *m, nil
}

func (m *Model) deleteDatabase(id uuid.UUID) (tea.Model, tea.Cmd) {
	for i := range m.cfg.Databases {
		if m.cfg.Databases[i].ID == id {
			name := m.cfg.Databases[i].Name
			m.cfg.Databases = append(m.cfg.Databases[:i], m.cfg.Databases[i+1:]...)
			_ = sshauth.DeletePassword(id.String())
			if err := config.Save(m.cfgDir, m.cfg); err != nil {
				m.palette.err = err.Error()
				return *m, nil
			}
			m.rebuildRows()
			m.closePalette()
			m.setToast("deleted "+name, toastOK)
			return *m, nil
		}
	}
	m.closePalette()
	return *m, nil
}

func (m *Model) saveConnection(id uuid.UUID) (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.palette.fields[connFieldName].input.Value())
	host := strings.TrimSpace(m.palette.fields[connFieldHost].input.Value())
	if name == "" || host == "" {
		m.palette.err = "name and host are required"
		return *m, nil
	}
	port := 0
	if ps := strings.TrimSpace(m.palette.fields[connFieldPort].input.Value()); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil {
			m.palette.err = "port must be an integer"
			return *m, nil
		}
		port = n
	}
	gpath := strings.TrimSpace(m.palette.fields[connFieldGroup].input.Value())
	gID, err := m.resolveOrCreateGroupPath(gpath)
	if err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}

	tags := m.paletteTags.Values()
	password := strings.TrimSpace(m.palette.fields[connFieldPassword].input.Value())
	rc := strings.TrimSpace(m.palette.fields[connFieldRemoteCmd].input.Value())

	// Resolve the Jump field (which stores a connection NAME) into a
	// UUID.
	var jumpID uuid.UUID
	if jn := strings.TrimSpace(m.palette.fields[connFieldJump].input.Value()); jn != "" {
		if j := m.connectionByName(jn); j != nil {
			jumpID = j.ID
		} else {
			m.palette.err = "unknown jump-host connection: " + jn
			return *m, nil
		}
	}

	// SSH options from the advanced fields. nil = "nothing set at
	// connection scope, cascade from group / global".
	opts := &config.SSHOptions{}
	touched := false
	if v := parseYesNo(m.palette.fields[connFieldFwdAgent].input.Value()); v != nil {
		opts.ForwardAgent = v
		touched = true
	}
	if v := parseYesNo(m.palette.fields[connFieldCompression].input.Value()); v != nil {
		opts.Compression = v
		touched = true
	}
	if lf := splitAndTrim(m.palette.fields[connFieldLocalFwd].input.Value(), ","); len(lf) > 0 {
		opts.LocalForward = lf
		touched = true
	}
	if rf := splitAndTrim(m.palette.fields[connFieldRemoteFwd].input.Value(), ","); len(rf) > 0 {
		opts.RemoteForward = rf
		touched = true
	}
	if s := strings.TrimSpace(m.palette.fields[connFieldAlive].input.Value()); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			opts.ServerAliveInterval = &n
			touched = true
		} else {
			m.palette.err = "Alive must be an integer"
			return *m, nil
		}
	}
	if s := strings.TrimSpace(m.palette.fields[connFieldAliveMax].input.Value()); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			opts.ServerAliveCountMax = &n
			touched = true
		} else {
			m.palette.err = "AliveMax must be an integer"
			return *m, nil
		}
	}
	if hk := strings.TrimSpace(m.palette.fields[connFieldHostKey].input.Value()); hk != "" {
		opts.StrictHostKeyCheck = hk
		touched = true
	}
	if tty := strings.TrimSpace(m.palette.fields[connFieldTTY].input.Value()); tty != "" {
		opts.RequestTTY = tty
		touched = true
	}
	if ex := strings.TrimSpace(m.palette.fields[connFieldExtra].input.Value()); ex != "" {
		opts.ExtraOptions = map[string]string{}
		for _, pair := range splitAndTrim(ex, ",") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			opts.ExtraOptions[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
		if len(opts.ExtraOptions) > 0 {
			touched = true
		}
	}
	// If the user came in via ssh-import, m.paletteOpts carried the
	// parsed options forward; merge them so an import that doesn't
	// touch the advanced fields still persists.
	if !touched && m.paletteOpts != nil {
		opts = m.paletteOpts
		touched = true
	}
	var finalOpts *config.SSHOptions
	if touched {
		finalOpts = opts
	}

	conn := config.Connection{
		Name:          name,
		Host:          host,
		Port:          port,
		User:          strings.TrimSpace(m.palette.fields[connFieldUser].input.Value()),
		IdentityFile:  strings.TrimSpace(m.palette.fields[connFieldKey].input.Value()),
		JumpHost:      jumpID,
		Group:         gID,
		Tags:          tags,
		RemoteCommand: rc,
		SSHOptions:    finalOpts,
		Env:           parseEnvMap(m.palette.fields[connFieldEnv].input.Value()),
	}
	if sshauth.IsCommandValue(password) {
		conn.Password = password
	}

	if id == uuid.Nil {
		conn.ID = uuid.New()
		m.cfg.Connections = append(m.cfg.Connections, conn)
	} else {
		conn.ID = id
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].ID == id {
				// Preserve fields the connection palette doesn't own.
				// SSHOptions IS now in this form (used to be
				// preserved from `existing` — that was eating every
				// edit to the advanced fields).
				existing := m.cfg.Connections[i]
				conn.Notes = existing.Notes       // notes palette
				conn.Scripts = existing.Scripts   // scripts tab
				conn.ShellInit = existing.ShellInit // shellinit palette (i / I)
				conn.Order = existing.Order       // sibling sort position
				m.cfg.Connections[i] = conn
				break
			}
		}
	}
	// Write a literal password to keychain; $(cmd) stays on the struct.
	if password != "" && !sshauth.IsCommandValue(password) {
		_ = sshauth.SetPassword(conn.ID.String(), password)
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = "save: " + err.Error()
		return *m, nil
	}
	m.rebuildRows()
	m.cursorToConnection(conn.ID)
	m.closePalette()
	m.setToast("saved "+conn.Name, toastOK)
	return *m, nil
}

func (m *Model) deleteConnection(id uuid.UUID) (tea.Model, tea.Cmd) {
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID == id {
			name := m.cfg.Connections[i].Name
			m.cfg.Connections = append(m.cfg.Connections[:i], m.cfg.Connections[i+1:]...)
			_ = sshauth.DeletePassword(id.String())
			if err := config.Save(m.cfgDir, m.cfg); err != nil {
				m.palette.err = err.Error()
				return *m, nil
			}
			m.rebuildRows()
			m.closePalette()
			m.setToast("deleted "+name, toastOK)
			return *m, nil
		}
	}
	m.closePalette()
	return *m, nil
}

func (m *Model) saveGroup(id uuid.UUID) (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(m.palette.fields[groupFieldPath].input.Value())
	if path == "" {
		m.palette.err = "path is required"
		return *m, nil
	}
	gID, err := m.resolveOrCreateGroupPath(path)
	if err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	// If editing an existing group whose path has moved, transfer its
	// contents (child groups, connections, databases) to the new UUID
	// and drop the original row.
	if id != uuid.Nil && id != gID {
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].Parent == id {
				m.cfg.Groups[i].Parent = gID
			}
		}
		for ci := range m.cfg.Connections {
			if m.cfg.Connections[ci].Group == id {
				m.cfg.Connections[ci].Group = gID
			}
		}
		for di := range m.cfg.Databases {
			if m.cfg.Databases[di].Group == id {
				m.cfg.Databases[di].Group = gID
			}
		}
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].ID == id {
				m.cfg.Groups = append(m.cfg.Groups[:i], m.cfg.Groups[i+1:]...)
				break
			}
		}
	}
	// Fill in the inheritable fields on the (possibly new) group.
	for i := range m.cfg.Groups {
		if m.cfg.Groups[i].ID != gID {
			continue
		}
		g := &m.cfg.Groups[i]
		g.User = strings.TrimSpace(m.palette.fields[groupFieldUser].input.Value())
		g.IdentityFile = strings.TrimSpace(m.palette.fields[groupFieldKey].input.Value())
		// JumpHost: resolve the typed connection name to a UUID.
		jumpName := strings.TrimSpace(m.palette.fields[groupFieldJump].input.Value())
		if jumpName == "" {
			g.JumpHost = uuid.Nil
		} else if j := m.connectionByName(jumpName); j != nil {
			g.JumpHost = j.ID
		} else {
			m.palette.err = "unknown jump-host connection: " + jumpName
			return *m, nil
		}
		opts := &config.SSHOptions{}
		touched := false
		if fa := parseYesNo(m.palette.fields[groupFieldFwdAgent].input.Value()); fa != nil {
			opts.ForwardAgent = fa
			touched = true
		}
		if cmp := parseYesNo(m.palette.fields[groupFieldCompression].input.Value()); cmp != nil {
			opts.Compression = cmp
			touched = true
		}
		if lf := splitAndTrim(m.palette.fields[groupFieldLocalFwd].input.Value(), ","); len(lf) > 0 {
			opts.LocalForward = lf
			touched = true
		}
		if rf := splitAndTrim(m.palette.fields[groupFieldRemoteFwd].input.Value(), ","); len(rf) > 0 {
			opts.RemoteForward = rf
			touched = true
		}
		// Preserve the SSH option fields the group palette doesn't
		// expose (Alive, AliveMax, HostKey, TTY, Extra) — otherwise
		// an edit to the exposed fields would silently drop them.
		if g.SSHOptions != nil {
			if opts.ServerAliveInterval == nil {
				opts.ServerAliveInterval = g.SSHOptions.ServerAliveInterval
			}
			if opts.ServerAliveCountMax == nil {
				opts.ServerAliveCountMax = g.SSHOptions.ServerAliveCountMax
			}
			if opts.StrictHostKeyCheck == "" {
				opts.StrictHostKeyCheck = g.SSHOptions.StrictHostKeyCheck
			}
			if opts.RequestTTY == "" {
				opts.RequestTTY = g.SSHOptions.RequestTTY
			}
			if len(opts.ExtraOptions) == 0 {
				opts.ExtraOptions = g.SSHOptions.ExtraOptions
			}
			if opts.ServerAliveInterval != nil || opts.ServerAliveCountMax != nil ||
				opts.StrictHostKeyCheck != "" || opts.RequestTTY != "" ||
				len(opts.ExtraOptions) > 0 {
				touched = true
			}
		}
		if touched {
			g.SSHOptions = opts
		} else {
			g.SSHOptions = nil
		}
		g.Tags = m.paletteTags.Values()
		g.Env = parseEnvMap(m.palette.fields[groupFieldEnv].input.Value())
		break
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return *m, nil
	}
	m.rebuildRows()
	m.cursorToGroup(gID)
	m.closePalette()
	m.setToast("saved group "+path, toastOK)
	return *m, nil
}

// connectionByName looks up a connection by its display name.
func (m *Model) connectionByName(name string) *config.Connection {
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].Name == name {
			return &m.cfg.Connections[i]
		}
	}
	return nil
}

// resolveOrCreateGroupPath returns the UUID of the group at the given
// slash-separated path, creating intermediates as needed. An empty
// path returns uuid.Nil (ungrouped).
func (m *Model) resolveOrCreateGroupPath(path string) (uuid.UUID, error) {
	path = strings.Trim(path, "/ ")
	if path == "" {
		return uuid.Nil, nil
	}
	parts := strings.Split(path, "/")
	parent := uuid.Nil
	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			return uuid.Nil, nil
		}
		found := uuid.Nil
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].Parent == parent && m.cfg.Groups[i].Name == name {
				found = m.cfg.Groups[i].ID
				break
			}
		}
		if found == uuid.Nil {
			g := config.Group{ID: uuid.New(), Name: name, Parent: parent}
			m.cfg.Groups = append(m.cfg.Groups, g)
			found = g.ID
		}
		parent = found
	}
	return parent, nil
}

// envMapToString serialises an Env map as a stable "KEY=value, …"
// string so the palette input shows the existing entries in a
// deterministic order.
func envMapToString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Alpha sort — stable, matches detail-tab display order.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ", ")
}

// parseEnvMap takes "KEY=value, OTHER=val" and returns the map, or
// nil for empty input.
func parseEnvMap(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
