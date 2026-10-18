package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
	sshauth "github.com/v4run/hangar/v2/internal/ssh"
)

type paletteState struct {
	kind    PaletteKind
	title   string
	message string
	fields  []paletteField
	focus   int
	target  uuid.UUID // for edit/delete
	err     string
}

type paletteField struct {
	label  string
	hint   string
	input  textinput.Model
	masked bool
}

func (m *Model) closePalette() {
	m.palette = paletteState{}
	m.paletteOpts = nil
	m.paletteScriptIdx = 0
	m.overlay = OverlayNone
	m.mode = ModeBrowse
}

func (m *Model) focusField(i int) tea.Cmd {
	if i < 0 || i >= len(m.palette.fields) {
		return nil
	}
	for j := range m.palette.fields {
		if j == i {
			m.palette.fields[j].input.Focus()
		} else {
			m.palette.fields[j].input.Blur()
		}
	}
	m.palette.focus = i
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
	switch msg.String() {
	case "esc":
		m.closePalette()
		return m, nil
	case "tab":
		return m, m.nextField()
	case "shift+tab":
		return m, m.prevField()
	case "ctrl+s", "enter":
		if m.palette.kind == PaletteDeleteConnection {
			// Enter alone shouldn't delete — require ctrl+s or y.
			if msg.String() == "enter" {
				m.closePalette()
				return m, nil
			}
		}
		return m.submitPalette()
	case "y", "Y":
		if m.palette.kind == PaletteDeleteConnection {
			return m.submitPalette()
		}
		if m.palette.kind == PaletteHostKey {
			return m.confirmHostKey()
		}
		if m.palette.kind == PaletteConfirmConnect {
			return m.confirmConnect()
		}
	case "n", "N":
		if m.palette.kind == PaletteDeleteConnection ||
			m.palette.kind == PaletteHostKey ||
			m.palette.kind == PaletteConfirmConnect {
			m.closePalette()
			return m, nil
		}
	}
	if len(m.palette.fields) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	m.palette.fields[m.palette.focus].input, cmd = m.palette.fields[m.palette.focus].input.Update(msg)
	return m, cmd
}

// --- connection form ---

const (
	connFieldName = iota
	connFieldHost
	connFieldPort
	connFieldUser
	connFieldKey
	connFieldGroup
	connFieldTags
	connFieldPassword
	connFieldRemoteCmd
	connFieldCount
)

var connFieldLabels = []string{
	"Name", "Host", "Port", "User", "Key", "Group", "Tags", "Pass", "RemoteCmd",
}
var connFieldHints = []string{
	"required, display name",
	"required, hostname or ip",
	"22",
	"login user (inherited from group if blank)",
	"~/.ssh/id_ed25519",
	"group name path, blank for ungrouped",
	"comma-separated",
	"stored in keychain; or $(cmd)",
	"run on remote after connect",
}

func (m *Model) openAddConnectionPalette() tea.Cmd {
	return m.openConnectionPalette(PaletteAddConnection, "New Connection", nil)
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
		fields[i] = paletteField{label: connFieldLabels[i], hint: connFieldHints[i], input: ti}
	}
	if c != nil {
		fields[connFieldName].input.SetValue(c.Name)
		fields[connFieldHost].input.SetValue(c.Host)
		if c.Port != 0 {
			fields[connFieldPort].input.SetValue(strconv.Itoa(c.Port))
		}
		fields[connFieldUser].input.SetValue(c.User)
		fields[connFieldKey].input.SetValue(c.IdentityFile)
		fields[connFieldGroup].input.SetValue(config.GroupPath(m.cfg, c.Group))
		fields[connFieldTags].input.SetValue(strings.Join(c.Tags, ", "))
		if c.Password != "" {
			fields[connFieldPassword].input.SetValue(c.Password)
		} else if pw, _ := sshauth.GetPassword(c.ID.String()); pw != "" {
			fields[connFieldPassword].input.SetValue(pw)
		}
		fields[connFieldRemoteCmd].input.SetValue(c.RemoteCommand)
	}
	for i := range fields {
		fields[i].input.CursorEnd()
	}
	m.palette = paletteState{kind: kind, title: title, fields: fields}
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

// --- group form (minimal — just a name for now) ---

func (m *Model) openAddGroupPalette() tea.Cmd {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "group name (slash for nesting: prod/us-east)"
	m.palette = paletteState{
		kind:   PaletteAddGroup,
		title:  "New Group",
		fields: []paletteField{{label: "Name", input: ti}},
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
}

func (m *Model) openEditGroupPalette(g *config.Group) tea.Cmd {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(g.Name)
	ti.CursorEnd()
	m.palette = paletteState{
		kind:   PaletteEditGroup,
		title:  "Edit group",
		fields: []paletteField{{label: "Name", input: ti}},
		target: g.ID,
	}
	m.overlay = OverlayPalette
	m.mode = ModeEdit
	return m.focusField(0)
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
	}
	return m, nil
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
		return m, nil
	}
	res, err := sshauth.ParseSSHCommand(raw)
	if err != nil {
		m.palette.err = err.Error()
		return m, nil
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
	return m, cmd
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
		return m, nil
	}
	m.closePalette()
	m.setToast("notes saved", toastOK)
	return m, nil
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
		return m, nil
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
		return m, nil
	}
	m.closePalette()
	m.setToast("script saved", toastOK)
	return m, nil
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
	"postgres | mysql | redis | sqlite",
	"required",
	"host or /path/to/file for sqlite",
	"",
	"",
	"",
	"group path, blank for ungrouped",
	"stored in keychain; or $(cmd)",
}

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
		fields[i] = paletteField{label: dbFieldLabels[i], hint: dbFieldHints[i], input: ti}
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
		return m, nil
	}
	switch engine {
	case "postgres", "mysql", "redis", "sqlite":
	default:
		m.palette.err = "engine must be postgres, mysql, redis, or sqlite"
		return m, nil
	}
	port := 0
	if ps := strings.TrimSpace(m.palette.fields[dbFieldPort].input.Value()); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil {
			m.palette.err = "port must be an integer"
			return m, nil
		}
		port = n
	}
	gpath := strings.TrimSpace(m.palette.fields[dbFieldGroup].input.Value())
	gID, err := m.resolveOrCreateGroupPath(gpath)
	if err != nil {
		m.palette.err = err.Error()
		return m, nil
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
		return m, nil
	}
	m.rebuildRows()
	m.closePalette()
	m.setToast("saved "+d.Name, toastOK)
	return m, nil
}

func (m *Model) deleteDatabase(id uuid.UUID) (tea.Model, tea.Cmd) {
	for i := range m.cfg.Databases {
		if m.cfg.Databases[i].ID == id {
			name := m.cfg.Databases[i].Name
			m.cfg.Databases = append(m.cfg.Databases[:i], m.cfg.Databases[i+1:]...)
			_ = sshauth.DeletePassword(id.String())
			if err := config.Save(m.cfgDir, m.cfg); err != nil {
				m.palette.err = err.Error()
				return m, nil
			}
			m.rebuildRows()
			m.closePalette()
			m.setToast("deleted "+name, toastOK)
			return m, nil
		}
	}
	m.closePalette()
	return m, nil
}

func (m *Model) saveConnection(id uuid.UUID) (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.palette.fields[connFieldName].input.Value())
	host := strings.TrimSpace(m.palette.fields[connFieldHost].input.Value())
	if name == "" || host == "" {
		m.palette.err = "name and host are required"
		return m, nil
	}
	port := 0
	if ps := strings.TrimSpace(m.palette.fields[connFieldPort].input.Value()); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil {
			m.palette.err = "port must be an integer"
			return m, nil
		}
		port = n
	}
	gpath := strings.TrimSpace(m.palette.fields[connFieldGroup].input.Value())
	gID, err := m.resolveOrCreateGroupPath(gpath)
	if err != nil {
		m.palette.err = err.Error()
		return m, nil
	}

	tags := splitAndTrim(m.palette.fields[connFieldTags].input.Value(), ",")
	password := strings.TrimSpace(m.palette.fields[connFieldPassword].input.Value())
	rc := strings.TrimSpace(m.palette.fields[connFieldRemoteCmd].input.Value())

	conn := config.Connection{
		Name:          name,
		Host:          host,
		Port:          port,
		User:          strings.TrimSpace(m.palette.fields[connFieldUser].input.Value()),
		IdentityFile:  strings.TrimSpace(m.palette.fields[connFieldKey].input.Value()),
		Group:         gID,
		Tags:          tags,
		RemoteCommand: rc,
		SSHOptions:    m.paletteOpts,
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
				// Preserve notes / scripts / shell init that we don't
				// expose in this minimal form.
				existing := m.cfg.Connections[i]
				conn.Notes = existing.Notes
				conn.Scripts = existing.Scripts
				conn.ShellInit = existing.ShellInit
				conn.SSHOptions = existing.SSHOptions
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
		return m, nil
	}
	m.rebuildRows()
	m.cursorToConnection(conn.ID)
	m.closePalette()
	m.setToast("saved "+conn.Name, toastOK)
	return m, nil
}

func (m *Model) deleteConnection(id uuid.UUID) (tea.Model, tea.Cmd) {
	for i := range m.cfg.Connections {
		if m.cfg.Connections[i].ID == id {
			name := m.cfg.Connections[i].Name
			m.cfg.Connections = append(m.cfg.Connections[:i], m.cfg.Connections[i+1:]...)
			_ = sshauth.DeletePassword(id.String())
			if err := config.Save(m.cfgDir, m.cfg); err != nil {
				m.palette.err = err.Error()
				return m, nil
			}
			m.rebuildRows()
			m.closePalette()
			m.setToast("deleted "+name, toastOK)
			return m, nil
		}
	}
	m.closePalette()
	return m, nil
}

func (m *Model) saveGroup(id uuid.UUID) (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.palette.fields[0].input.Value())
	if name == "" {
		m.palette.err = "name is required"
		return m, nil
	}
	gID, err := m.resolveOrCreateGroupPath(name)
	if err != nil {
		m.palette.err = err.Error()
		return m, nil
	}
	// Rename an existing group when editing.
	if id != uuid.Nil && id != gID {
		for i := range m.cfg.Groups {
			if m.cfg.Groups[i].ID == id {
				// Transfer connections to the new group; drop the old
				// one.
				for ci := range m.cfg.Connections {
					if m.cfg.Connections[ci].Group == id {
						m.cfg.Connections[ci].Group = gID
					}
				}
				m.cfg.Groups = append(m.cfg.Groups[:i], m.cfg.Groups[i+1:]...)
				break
			}
		}
	}
	if err := config.Save(m.cfgDir, m.cfg); err != nil {
		m.palette.err = err.Error()
		return m, nil
	}
	m.rebuildRows()
	m.cursorToGroup(gID)
	m.closePalette()
	m.setToast("saved group "+name, toastOK)
	return m, nil
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
