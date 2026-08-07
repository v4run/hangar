package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
	dbpkg "github.com/v4run/hangar/internal/db"
	sshauth "github.com/v4run/hangar/internal/ssh"
)

type shellInitEditorDoneMsg struct {
	content string
	err     error
}

// openShellInitInEditor suspends the TUI and opens $EDITOR on a tempfile
// seeded with the current shell-init buffer. On exit, the file is read and
// delivered as a shellInitEditorDoneMsg.
func (m *Model) openShellInitInEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	f, err := os.CreateTemp("", "hangar-shellinit-*.sh")
	if err != nil {
		return func() tea.Msg { return shellInitEditorDoneMsg{err: err} }
	}
	path := f.Name()
	_, _ = f.WriteString(m.shellInitInput)
	f.Close()

	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		data, readErr := os.ReadFile(path)
		os.Remove(path)
		if execErr != nil {
			return shellInitEditorDoneMsg{err: execErr}
		}
		if readErr != nil {
			return shellInitEditorDoneMsg{err: readErr}
		}
		return shellInitEditorDoneMsg{content: string(data)}
	})
}

func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.cursor = 0
		m.sidebarOffset = 0
	case "enter":
		m.filtering = false
		m.cursor = 0
		m.sidebarOffset = 0
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
		m.cursor = 0
		m.sidebarOffset = 0
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.cursor = 0
			m.sidebarOffset = 0
		}
	}
	return m, nil
}

func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.formEditing {
		return m.handleFormEditMode(msg)
	}
	return m.handleFormNavMode(msg)
}

// handleFormNavMode handles form input when navigating fields (not editing).
func (m Model) handleFormNavMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
		m.jumpSuggestions = nil
	case "j", "down", "tab":
		m.formCursor = (m.formCursor + 1) % fieldAdvancedCount
		m.jumpSuggestions = nil
		m.jumpSugCursor = 0
	case "k", "up", "shift+tab":
		m.formCursor = (m.formCursor - 1 + fieldAdvancedCount) % fieldAdvancedCount
		m.jumpSuggestions = nil
		m.jumpSugCursor = 0
	case "enter":
		// Enter edit mode for all fields
		m.formEditing = true
		m.formEditBuf = m.formFields[m.formCursor]
		if m.formCursor == fieldJump {
			// Convert UUID to display name for editing
			m.formFields[fieldJump] = m.jumpHostDisplay(m.formFields[fieldJump])
			m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
			m.jumpSugCursor = 0
		}
	case "ctrl+s":
		return m.saveForm()
	}
	return m, nil
}

// handleFormEditMode handles form input when editing a text field.
func (m Model) handleFormEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Discard changes, revert to snapshot
		m.formFields[m.formCursor] = m.formEditBuf
		m.formEditing = false
		m.jumpSuggestions = nil
	case "enter":
		// If on JumpHost with suggestions, select the highlighted one
		if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 && m.jumpSugCursor >= 0 && m.jumpSugCursor < len(m.jumpSuggestions) {
			selected := m.jumpSuggestions[m.jumpSugCursor]
			m.formFields[fieldJump] = selected.ID.String()
			m.jumpSuggestions = nil
			m.jumpSugCursor = 0
			m.formEditing = false
			return m, nil
		}
		// Confirm edit, return to nav mode
		if m.formCursor == fieldJump {
			// Resolve typed name back to UUID for storage
			m.formFields[fieldJump] = m.jumpHostResolve(m.formFields[fieldJump])
		}
		m.formEditing = false
		m.jumpSuggestions = nil
	case "ctrl+s":
		if m.formCursor == fieldJump {
			m.formFields[fieldJump] = m.jumpHostResolve(m.formFields[fieldJump])
		}
		m.formEditing = false
		return m.saveForm()
	case "ctrl+n":
		if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
			m.jumpSugCursor = (m.jumpSugCursor + 1) % len(m.jumpSuggestions)
			return m, nil
		}
	case "ctrl+p":
		if m.formCursor == fieldJump && len(m.jumpSuggestions) > 0 {
			m.jumpSugCursor = (m.jumpSugCursor - 1 + len(m.jumpSuggestions)) % len(m.jumpSuggestions)
			return m, nil
		}
	case "l":
		// Cycle forward on constrained fields
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == current && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
		// Fall through to default for text fields
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
		}
	case "h":
		// Cycle backward on constrained fields
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			prev := opts[len(opts)-1]
			for i, o := range opts {
				if o == current && i > 0 {
					prev = opts[i-1]
					break
				}
			}
			m.formFields[m.formCursor] = prev
			return m, nil
		}
		// Fall through to default for text fields
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
		}
	case "backspace":
		if _, ok := fieldCycleOptions[m.formCursor]; ok {
			return m, nil
		}
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
		}
	default:
		if _, ok := fieldCycleOptions[m.formCursor]; ok {
			return m, nil
		}
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
			if m.formCursor == fieldJump {
				m.jumpSuggestions = m.jumpHostSuggestions(m.formFields[fieldJump])
				m.jumpSugCursor = 0
			}
		}
	}
	return m, nil
}

func (m Model) handleDBFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.formEditing {
		return m.handleDBFormEditMode(msg)
	}
	return m.handleDBFormNavMode(msg)
}

func (m Model) handleDBFormNavMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "j", "down", "tab":
		m.formCursor = (m.formCursor + 1) % dbFieldCount
	case "k", "up", "shift+tab":
		m.formCursor = (m.formCursor - 1 + dbFieldCount) % dbFieldCount
	case "enter":
		m.formEditing = true
		m.formEditBuf = m.formFields[m.formCursor]
		if m.formCursor == dbFieldTunnel {
			m.formFields[dbFieldTunnel] = m.jumpHostDisplay(m.formFields[dbFieldTunnel])
		}
	case "ctrl+s":
		return m.saveDBForm()
	}
	return m, nil
}

func (m Model) handleDBFormEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.formFields[m.formCursor] = m.formEditBuf
		m.formEditing = false
	case "enter":
		if m.formCursor == dbFieldTunnel {
			m.formFields[dbFieldTunnel] = m.jumpHostResolve(m.formFields[dbFieldTunnel])
		}
		m.formEditing = false
	case "ctrl+s":
		if m.formCursor == dbFieldTunnel {
			m.formFields[dbFieldTunnel] = m.jumpHostResolve(m.formFields[dbFieldTunnel])
		}
		m.formEditing = false
		return m.saveDBForm()
	case "l":
		if opts, ok := dbFieldCycleOptions[m.formCursor]; ok {
			cur := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == cur && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	case "h":
		if opts, ok := dbFieldCycleOptions[m.formCursor]; ok {
			cur := m.formFields[m.formCursor]
			prev := opts[len(opts)-1]
			for i, o := range opts {
				if o == cur && i > 0 {
					prev = opts[i-1]
					break
				}
			}
			m.formFields[m.formCursor] = prev
			return m, nil
		}
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	case "backspace":
		if _, ok := dbFieldCycleOptions[m.formCursor]; ok {
			return m, nil
		}
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if _, ok := dbFieldCycleOptions[m.formCursor]; ok {
			return m, nil
		}
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	}
	return m, nil
}

func (m Model) saveDBForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.formFields[dbFieldName])
	if name == "" {
		m.formError = "name is required"
		return m, nil
	}
	engine := config.DBEngine(strings.TrimSpace(m.formFields[dbFieldEngine]))
	if engine == "" {
		m.formError = "engine is required"
		return m, nil
	}
	port := 0
	if v := strings.TrimSpace(m.formFields[dbFieldPort]); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			m.formError = "port must be a number"
			return m, nil
		}
		port = p
	}
	var tags []string
	if t := strings.TrimSpace(m.formFields[dbFieldTags]); t != "" {
		for _, p := range strings.Split(t, ",") {
			if p = strings.TrimSpace(p); p != "" {
				tags = append(tags, p)
			}
		}
	}
	client := config.PostgresClient(strings.TrimSpace(m.formFields[dbFieldClient]))
	if engine != config.EnginePostgres {
		client = ""
	}
	pwIn := m.formFields[dbFieldPassword]
	dbPasswordCmd := ""
	if config.IsCommandValue(pwIn) {
		dbPasswordCmd = pwIn
	}

	db := config.Database{
		Name:      name,
		Engine:    engine,
		Host:      strings.TrimSpace(m.formFields[dbFieldHost]),
		Port:      port,
		User:      strings.TrimSpace(m.formFields[dbFieldUser]),
		DBName:    strings.TrimSpace(m.formFields[dbFieldDBName]),
		SSHTunnel: strings.TrimSpace(m.formFields[dbFieldTunnel]),
		Client:    client,
		Group:     strings.TrimSpace(m.formFields[dbFieldGroup]),
		Tags:      tags,
		Notes:     strings.TrimSpace(m.formFields[dbFieldNotes]),
		Password:  dbPasswordCmd,
	}

	if m.form == formAddDatabase {
		if err := m.cfg.AddDatabase(db); err != nil {
			m.formError = err.Error()
			return m, nil
		}
		db.ID = m.cfg.Databases[len(m.cfg.Databases)-1].ID
	} else {
		existing, err := m.cfg.FindDatabaseByID(m.formTarget)
		if err != nil {
			m.formError = err.Error()
			return m, nil
		}
		db.ID = existing.ID
		if err := m.cfg.UpdateDatabaseByID(m.formTarget, db); err != nil {
			m.formError = err.Error()
			return m, nil
		}
	}

	if db.Group != "" && groupIndex(m.cfg.Groups, db.Group) < 0 {
		m.cfg.Groups = append(m.cfg.Groups, db.Group)
	}

	if pw := m.formFields[dbFieldPassword]; pw != "" && !config.IsCommandValue(pw) {
		_ = config.SetPassword(db.ID.String(), pw)
	} else {
		_ = config.DeletePassword(db.ID.String())
	}

	if err := config.Save(m.configDir, m.cfg); err != nil {
		m.formError = err.Error()
		return m, nil
	}
	m.form = formNone
	return m, nil
}

func (m Model) handleDeleteDatabaseConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.formTarget.String()
		if d, err := m.cfg.FindDatabaseByID(m.formTarget); err == nil {
			name = d.Name
		}
		_ = m.cfg.RemoveDatabaseByID(m.formTarget)
		_ = config.DeletePassword(m.formTarget.String())
		_ = config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.adjustSidebarViewport()
		m.form = formNone
		t, cmd := showToast("deleted database "+name, toastOK)
		m.activeToast = &t
		return m, cmd
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleScriptsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	scripts := m.allScripts()

	switch msg.String() {
	case "?":
		m.showHelp = !m.showHelp
	case "h", "esc":
		m.focus = focusSidebar
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if m.scriptCursor < len(scripts)-1 {
			m.scriptCursor++
		}
	case "k", "up":
		if m.scriptCursor > 0 {
			m.scriptCursor--
		}
	case "n":
		m.form = formAddScript
		m.scriptName = ""
		m.scriptCommand = ""
		m.scriptField = 0
		m.formError = ""
	case "e":
		if m.scriptCursor < len(scripts) {
			s := scripts[m.scriptCursor]
			m.form = formEditScript
			m.scriptTarget = m.scriptCursor
			m.scriptName = s.Name
			m.scriptCommand = s.Command
			m.scriptField = 0
			m.formError = ""
		}
	case "d":
		if m.scriptCursor < len(scripts) {
			m.form = formDeleteScript
			m.scriptTarget = m.scriptCursor
		}
	case "o":
		// Edit notes
		c := m.getSelectedConn()
		if c != nil {
			m.form = formEditNotes
			m.notesInput = c.Notes
		}
	case "enter":
		// Run the selected script
		conn := m.selectedConnection()
		if m.scriptCursor < len(scripts) && conn != nil {
			script := scripts[m.scriptCursor]
			m.runningScriptIdx = m.scriptCursor
			m.runningIsGlobal = m.isGlobalScript(m.scriptCursor)
			m.connectStart = time.Now()
			jumpHost := sshauth.ResolveJumpHost(m.cfg, conn.JumpHost)
			var opts *config.SSHOptions
			if conn.UseGlobalSettings == nil || *conn.UseGlobalSettings {
				mo := config.MergeSSHOptions(m.globalCfg.SSHOptions, conn.SSHOptions)
				opts = &mo
			} else {
				opts = conn.SSHOptions
			}
			sshCmd, cleanup := sshauth.NewSSHCommand(conn, jumpHost, opts, "")
			escapedCmd := strings.ReplaceAll(script.Command, "'", "'\\''")
			remoteCmd := fmt.Sprintf("bash -l -c '%s; printf \"\\npress any key to continue...\"; read -n 1'", escapedCmd)
			userHost := sshCmd.Args[len(sshCmd.Args)-1]
			sshCmd.Args = append(sshCmd.Args[:len(sshCmd.Args)-1],
				"-t", userHost, remoteCmd)
			return m, tea.ExecProcess(sshCmd, func(err error) tea.Msg {
				cleanup()
				return sshExitMsg{err: err}
			})
		}
	}
	return m, nil
}

func (m Model) handleScriptFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "tab", "down":
		m.scriptField = (m.scriptField + 1) % 2
	case "shift+tab", "up":
		m.scriptField = (m.scriptField + 1) % 2
	case "enter":
		name := strings.TrimSpace(m.scriptName)
		command := strings.TrimSpace(m.scriptCommand)
		if name == "" {
			m.formError = "name is required"
			return m, nil
		}
		if command == "" {
			m.formError = "command is required"
			return m, nil
		}
		script := config.Script{Name: name, Command: command}

		if m.form == formAddScript {
			c := m.getSelectedConn()
			if c != nil {
				c.Scripts = append(c.Scripts, script)
			}
		} else {
			// Edit
			if m.isGlobalScript(m.scriptTarget) {
				gi := m.scriptTarget
				c := m.getSelectedConn()
				if c != nil {
					gi -= len(c.Scripts)
				}
				if gi >= 0 && gi < len(m.cfg.GlobalScripts) {
					m.cfg.GlobalScripts[gi] = script
				}
			} else {
				c := m.getSelectedConn()
				if c != nil && m.scriptTarget < len(c.Scripts) {
					c.Scripts[m.scriptTarget] = script
				}
			}
		}
		config.Save(m.configDir, m.cfg)
		m.form = formNone
	case "backspace":
		if m.scriptField == 0 && len(m.scriptName) > 0 {
			m.scriptName = m.scriptName[:len(m.scriptName)-1]
		} else if m.scriptField == 1 && len(m.scriptCommand) > 0 {
			m.scriptCommand = m.scriptCommand[:len(m.scriptCommand)-1]
		}
	default:
		if len(msg.String()) == 1 {
			if m.scriptField == 0 {
				m.scriptName += msg.String()
			} else {
				m.scriptCommand += msg.String()
			}
		}
	}
	return m, nil
}

func (m Model) handleDeleteScriptConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.isGlobalScript(m.scriptTarget) {
			gi := m.scriptTarget
			c := m.getSelectedConn()
			if c != nil {
				gi -= len(c.Scripts)
			}
			if gi >= 0 && gi < len(m.cfg.GlobalScripts) {
				m.cfg.GlobalScripts = append(m.cfg.GlobalScripts[:gi], m.cfg.GlobalScripts[gi+1:]...)
			}
		} else {
			c := m.getSelectedConn()
			if c != nil && m.scriptTarget < len(c.Scripts) {
				c.Scripts = append(c.Scripts[:m.scriptTarget], c.Scripts[m.scriptTarget+1:]...)
			}
		}
		config.Save(m.configDir, m.cfg)
		scripts := m.allScripts()
		if m.scriptCursor >= len(scripts) && m.scriptCursor > 0 {
			m.scriptCursor--
		}
		m.form = formNone
		t, cmd := showToast("deleted script", toastOK)
		m.activeToast = &t
		return m, cmd
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleNotesInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		c := m.getSelectedConn()
		if c != nil {
			c.Notes = strings.TrimSpace(m.notesInput)
			config.Save(m.configDir, m.cfg)
		}
		m.form = formNone
	case "backspace":
		if len(m.notesInput) > 0 {
			m.notesInput = m.notesInput[:len(m.notesInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.notesInput += msg.String()
		}
	}
	return m, nil
}

// sidebarLayout describes the fixed geometry of the sidebar within the
// terminal so we can map click coordinates to item indices.
const (
	sidebarPaneX0     = 1 // outer padding col
	sidebarPaneWidth  = 26
	sidebarItemsY0Pad = 1 // outer padding row
	// Rows inside the sidebar before items start: title(1) + divider(1) +
	// filter(1) + up-indicator(1) = 4.
	sidebarItemsY0Inner = 4
)

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Only act while the sidebar is the primary view (no form / help open).
	if m.form != formNone || m.showHelp || m.connecting || m.visualMode {
		return m, nil
	}

	inSidebar := msg.X >= sidebarPaneX0 && msg.X < sidebarPaneX0+sidebarPaneWidth
	if !inSidebar {
		return m, nil
	}

	itemsY0 := sidebarItemsY0Pad + sidebarItemsY0Inner
	items := m.sidebarItems()

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.sidebarOffset > 0 {
			m.sidebarOffset--
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		visible := m.sidebarVisibleRows()
		if m.sidebarOffset+visible < len(items) {
			m.sidebarOffset++
		}
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		row := msg.Y - itemsY0
		if row < 0 {
			return m, nil
		}
		idx := row + m.sidebarOffset
		if idx < 0 || idx >= len(items) {
			return m, nil
		}
		m.focus = focusSidebar
		m.cursor = idx
		m.adjustSidebarViewport()
	}
	return m, nil
}

func (m Model) launchDatabase(d *config.Database) (tea.Cmd, error) {
	resolved, err := config.ResolveDatabase(d)
	if err != nil {
		return nil, err
	}
	d = resolved
	var tunnelCmd *exec.Cmd
	targetHost := d.Host
	targetPort := d.Port

	if d.SSHTunnel != "" && dbpkg.RequiresTunnelTarget(d.Engine) {
		var sshConn *config.Connection
		if id, err := uuid.Parse(d.SSHTunnel); err == nil {
			sshConn, _ = m.cfg.FindByID(id)
		}
		if sshConn == nil {
			sshConn, _ = m.cfg.FindByName(d.SSHTunnel)
		}
		if sshConn == nil {
			return nil, fmt.Errorf("ssh tunnel %q not found", d.SSHTunnel)
		}
		port, err := sshauth.FreeLocalPort()
		if err != nil {
			return nil, err
		}
		jump := sshauth.ResolveJumpHost(m.cfg, sshConn.JumpHost)
		var opts *config.SSHOptions
		if sshConn.UseGlobalSettings == nil || *sshConn.UseGlobalSettings {
			mo := config.MergeSSHOptions(m.globalCfg.SSHOptions, sshConn.SSHOptions)
			opts = &mo
		} else {
			opts = sshConn.SSHOptions
		}
		tunnelCmd, _ = sshauth.BuildTunnelCommand(sshConn, jump, opts, port, d.Host, d.Port)
		if err := tunnelCmd.Start(); err != nil {
			return nil, fmt.Errorf("opening tunnel: %w", err)
		}
		if err := sshauth.WaitForLocalPort(port, 6*time.Second); err != nil {
			_ = tunnelCmd.Process.Kill()
			return nil, err
		}
		targetHost = "127.0.0.1"
		targetPort = port
	}

	pw := d.Password
	if pw == "" {
		pw, _ = config.GetPassword(d.ID.String())
	}
	clientCmd, err := dbpkg.Build(d, targetHost, targetPort, pw)
	if err != nil {
		if tunnelCmd != nil {
			_ = tunnelCmd.Process.Kill()
		}
		return nil, err
	}
	c := exec.Command(clientCmd.Path, clientCmd.Args[1:]...)
	if len(clientCmd.Env) > 0 {
		c.Env = append(os.Environ(), clientCmd.Env...)
	}
	name := d.Name
	return tea.ExecProcess(c, func(execErr error) tea.Msg {
		if tunnelCmd != nil {
			_ = tunnelCmd.Process.Kill()
			_ = tunnelCmd.Wait()
		}
		return dbExitMsg{err: execErr, name: name}
	}), nil
}

func (m Model) handleNewChooser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "c", "C":
		m.beginAddConnection()
	case "d", "D":
		m.beginAddDatabase()
	}
	return m, nil
}

func (m *Model) beginAddConnection() {
	m.form = formAdd
	currentGroup := ""
	items := m.sidebarItems()
	if m.cursor < len(items) {
		if items[m.cursor].isGroup {
			currentGroup = items[m.cursor].group
		} else if items[m.cursor].conn != nil {
			currentGroup = items[m.cursor].conn.Group
		} else if items[m.cursor].db != nil {
			currentGroup = items[m.cursor].db.Group
		}
	}
	m.formFields = make([]string, fieldAdvancedCount)
	m.formFields[fieldPort] = "22"
	m.formFields[fieldGroup] = currentGroup
	m.formFields[fieldUseGlobalSettings] = "yes"
	m.formCursor = 0
	m.formError = ""
}

func (m *Model) beginAddDatabase() {
	m.form = formAddDatabase
	currentGroup := ""
	items := m.sidebarItems()
	if m.cursor < len(items) {
		if items[m.cursor].isGroup {
			currentGroup = items[m.cursor].group
		} else if items[m.cursor].conn != nil {
			currentGroup = items[m.cursor].conn.Group
		} else if items[m.cursor].db != nil {
			currentGroup = items[m.cursor].db.Group
		}
	}
	m.formFields = make([]string, dbFieldCount)
	m.formFields[dbFieldEngine] = "postgres"
	m.formFields[dbFieldPort] = "5432"
	m.formFields[dbFieldClient] = "psql"
	m.formFields[dbFieldGroup] = currentGroup
	m.formCursor = 0
	m.formEditing = false
	m.formError = ""
}

func (m Model) handleShellInitInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
		return m, nil
	case "ctrl+s":
		m.persistShellInit()
		m.form = formNone
		return m, nil
	case "ctrl+e":
		return m, m.openShellInitInEditor()
	case "ctrl+t":
		if m.shellInitScope == shellInitScopeConnection {
			if c, err := m.cfg.FindByID(m.formTarget); err == nil {
				cur := true
				if c.UseGlobalShellInit != nil {
					cur = *c.UseGlobalShellInit
				}
				flipped := !cur
				c.UseGlobalShellInit = &flipped
				config.Save(m.configDir, m.cfg)
			}
		}
		return m, nil
	case "enter":
		m.shellInitInput += "\n"
		return m, nil
	case "backspace":
		if len(m.shellInitInput) > 0 {
			m.shellInitInput = m.shellInitInput[:len(m.shellInitInput)-1]
		}
		return m, nil
	case "tab":
		m.shellInitInput += "\t"
		return m, nil
	}
	s := msg.String()
	if len(s) == 1 {
		m.shellInitInput += s
	} else if len(msg.Runes) > 0 {
		m.shellInitInput += string(msg.Runes)
	}
	return m, nil
}

// persistShellInit writes m.shellInitInput to the scope's target and saves.
func (m *Model) persistShellInit() {
	switch m.shellInitScope {
	case shellInitScopeGlobal:
		m.cfg.GlobalShellInit = m.shellInitInput
	case shellInitScopeGroup:
		if m.cfg.GroupShellInit == nil {
			m.cfg.GroupShellInit = make(map[string]string)
		}
		if strings.TrimSpace(m.shellInitInput) == "" {
			delete(m.cfg.GroupShellInit, m.shellInitGroup)
		} else {
			m.cfg.GroupShellInit[m.shellInitGroup] = m.shellInitInput
		}
	case shellInitScopeConnection:
		if c, err := m.cfg.FindByID(m.formTarget); err == nil {
			c.ShellInit = m.shellInitInput
		}
	}
	config.Save(m.configDir, m.cfg)
}

func (m Model) handleAddGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		name := strings.TrimSpace(m.groupNameInput)
		if name == "" {
			m.formError = "group name is required"
			return m, nil
		}
		if groupIndex(m.cfg.Groups, name) >= 0 {
			m.formError = "group already exists"
			return m, nil
		}
		m.collapsed[name] = false
		m.cfg.Groups = append(m.cfg.Groups, name)
		config.Save(m.configDir, m.cfg)
		m.form = formNone
	case "backspace":
		if len(m.groupNameInput) > 0 {
			m.groupNameInput = m.groupNameInput[:len(m.groupNameInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.groupNameInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleEditGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		newName := strings.TrimSpace(m.groupNameInput)
		if newName == "" {
			m.formError = "group name is required"
			return m, nil
		}
		oldName := m.formTargetGroup
		if newName == oldName {
			m.form = formNone
			return m, nil
		}
		// Rename group across all connections
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].Group == oldName {
				m.cfg.Connections[i].Group = newName
			}
		}
		// Capture whether newName already exists BEFORE mutating the slice.
		oldIdx := groupIndex(m.cfg.Groups, oldName)
		newNameExists := groupIndex(m.cfg.Groups, newName) >= 0
		// Update Groups slice. If newName already exists, drop the old entry
		// (connections merge into the existing group's position).
		if oldIdx >= 0 {
			if !newNameExists {
				m.cfg.Groups[oldIdx] = newName
			} else {
				m.cfg.Groups = append(m.cfg.Groups[:oldIdx], m.cfg.Groups[oldIdx+1:]...)
			}
		}
		// Update collapsed state. Only transfer to newName when newName had no prior state.
		if wasCollapsed, ok := m.collapsed[oldName]; ok {
			delete(m.collapsed, oldName)
			if !newNameExists {
				m.collapsed[newName] = wasCollapsed
			}
		}
		// Migrate per-group shell-init snippet (only when newName had none).
		if old, ok := m.cfg.GroupShellInit[oldName]; ok {
			delete(m.cfg.GroupShellInit, oldName)
			if !newNameExists {
				if m.cfg.GroupShellInit == nil {
					m.cfg.GroupShellInit = make(map[string]string)
				}
				m.cfg.GroupShellInit[newName] = old
			}
		}
		config.Save(m.configDir, m.cfg)
		m.form = formNone
	case "backspace":
		if len(m.groupNameInput) > 0 {
			m.groupNameInput = m.groupNameInput[:len(m.groupNameInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.groupNameInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleDeleteGroupConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.formTargetGroup
		// Ungroup all connections in this group
		for i := range m.cfg.Connections {
			if m.cfg.Connections[i].Group == m.formTargetGroup {
				m.cfg.Connections[i].Group = ""
			}
		}
		delete(m.collapsed, m.formTargetGroup)
		delete(m.cfg.GroupShellInit, m.formTargetGroup)
		if idx := groupIndex(m.cfg.Groups, m.formTargetGroup); idx >= 0 {
			m.cfg.Groups = append(m.cfg.Groups[:idx], m.cfg.Groups[idx+1:]...)
		}
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.adjustSidebarViewport()
		m.form = formNone
		t, cmd := showToast("deleted group "+name, toastOK)
		m.activeToast = &t
		return m, cmd
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.formTarget.String()
		if c, err := m.cfg.FindByID(m.formTarget); err == nil {
			name = c.Name
		}
		m.cfg.RemoveByID(m.formTarget)
		config.Save(m.configDir, m.cfg)
		items := m.sidebarItems()
		if m.cursor >= len(items) && m.cursor > 0 {
			m.cursor--
		}
		m.adjustSidebarViewport()
		m.form = formNone
		t, cmd := showToast("deleted "+name, toastOK)
		m.activeToast = &t
		return m, cmd
	case "n", "N", "esc":
		m.form = formNone
	}
	return m, nil
}

func (m Model) handleTagInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "enter":
		// Save: set the connection's tags to tagTokens
		c, err := m.cfg.FindByID(m.formTarget)
		if err != nil {
			m.form = formNone
			return m, nil
		}
		// Commit any remaining buffer
		if buf := strings.TrimSpace(m.tagBuffer); buf != "" {
			m.tagTokens = append(m.tagTokens, buf)
			m.tagBuffer = ""
		}
		c.Tags = m.tagTokens
		config.Save(m.configDir, m.cfg)
		t, cmd := showToast("tags updated", toastOK)
		m.activeToast = &t
		m.form = formNone
		return m, cmd
	case " ", ",":
		// Commit buffer as a token
		buf := strings.TrimSpace(m.tagBuffer)
		if buf != "" {
			// Check for duplicate
			exists := false
			for _, t := range m.tagTokens {
				if t == buf {
					exists = true
					break
				}
			}
			if !exists {
				m.tagTokens = append(m.tagTokens, buf)
			}
			m.tagBuffer = ""
		}
	case "backspace":
		if len(m.tagBuffer) > 0 {
			m.tagBuffer = m.tagBuffer[:len(m.tagBuffer)-1]
		} else if len(m.tagTokens) > 0 {
			// Pop last token
			m.tagTokens = m.tagTokens[:len(m.tagTokens)-1]
		}
	case "tab":
		// Autocomplete from existing tags across all connections
		if m.tagBuffer != "" {
			suggestion := m.suggestTag(m.tagBuffer)
			if suggestion != "" {
				m.tagBuffer = suggestion
			}
		}
	default:
		if len(msg.String()) == 1 {
			m.tagBuffer += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleSyncInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Sync filter mode
	if m.syncFiltering {
		switch msg.String() {
		case "esc":
			m.syncFiltering = false
			m.syncFilterText = ""
		case "enter":
			m.syncFiltering = false
		case "backspace":
			if len(m.syncFilterText) > 0 {
				m.syncFilterText = m.syncFilterText[:len(m.syncFilterText)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.syncFilterText += msg.String()
			}
		}
		// Adjust cursor to stay within filtered entries
		filtered := m.filteredSyncEntries()
		if len(filtered) > 0 {
			found := false
			for _, idx := range filtered {
				if idx == m.syncCursor {
					found = true
					break
				}
			}
			if !found {
				m.syncCursor = filtered[0]
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.form = formNone
		m.syncFilterText = ""
		m.syncFiltering = false
	case "/":
		m.syncFiltering = true
		m.syncFilterText = ""
	case "j", "down":
		filtered := m.filteredSyncEntries()
		if len(filtered) == 0 {
			break
		}
		// Find current position in filtered list and move to next
		for i, idx := range filtered {
			if idx == m.syncCursor {
				if i+1 < len(filtered) {
					m.syncCursor = filtered[i+1]
				}
				break
			}
		}
	case "k", "up":
		filtered := m.filteredSyncEntries()
		if len(filtered) == 0 {
			break
		}
		for i, idx := range filtered {
			if idx == m.syncCursor {
				if i-1 >= 0 {
					m.syncCursor = filtered[i-1]
				}
				break
			}
		}
	case " ":
		if m.syncCursor < len(m.syncSelected) {
			m.syncSelected[m.syncCursor] = !m.syncSelected[m.syncCursor]
		}
	case "a":
		// Select all visible (filtered) entries
		filtered := m.filteredSyncEntries()
		for _, idx := range filtered {
			m.syncSelected[idx] = true
		}
	case "n":
		// Deselect all visible (filtered) entries
		filtered := m.filteredSyncEntries()
		for _, idx := range filtered {
			m.syncSelected[idx] = false
		}
	case "enter":
		imported := 0
		for i, entry := range m.syncEntries {
			if !m.syncSelected[i] {
				continue
			}
			existing, err := m.cfg.FindByName(entry.Name)
			if err != nil {
				// New entry — add it
				m.cfg.Connections = append(m.cfg.Connections, entry)
				imported++
			} else if existing.SyncedFromSSHConfig {
				// Update existing synced entry
				existing.Host = entry.Host
				existing.Port = entry.Port
				existing.User = entry.User
				existing.IdentityFile = entry.IdentityFile
				existing.JumpHost = entry.JumpHost
				imported++
			}
		}
		config.Save(m.configDir, m.cfg)
		m.sshConfigChanged = false
		m.syncFilterText = ""
		m.syncFiltering = false
		m.form = formNone
		t, cmd := showToast(fmt.Sprintf("imported %d hosts", imported), toastOK)
		m.activeToast = &t
		return m, cmd
	}
	return m, nil
}

func (m Model) handlePasteConfirmInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Rename: append "-copy" (or "-copy-2", etc.) to conflicting names
		m.executePaste(true)
		t, cmd := showToast(fmt.Sprintf("pasted %d items (renamed)", len(m.pasteItems)), toastOK)
		m.activeToast = &t
		m.form = formNone
		return m, cmd
	case "s":
		// Skip: paste only non-conflicting items
		m.executePasteSkipCollisions()
		t, cmd := showToast("pasted (skipped conflicts)", toastOK)
		m.activeToast = &t
		m.form = formNone
		return m, cmd
	case "esc":
		m.form = formNone
	}
	return m, nil
}

func (m *Model) executePaste(rename bool) {
	for _, id := range m.pasteItems {
		c, err := m.cfg.FindByID(id)
		if err != nil {
			continue
		}
		if m.pasteIsCut {
			c.Group = m.pasteTargetGroup
		} else {
			newConn := *c
			newConn.ID = uuid.New()
			newConn.Group = m.pasteTargetGroup
			if rename && m.isNameCollision(newConn.Name) {
				newConn.Name = m.uniqueName(newConn.Name)
			}
			m.cfg.Connections = append(m.cfg.Connections, newConn)
		}
	}
	config.Save(m.configDir, m.cfg)
	m.cutConnections = make(map[uuid.UUID]bool)
	m.copyConnections = make(map[uuid.UUID]bool)
}

func (m *Model) executePasteSkipCollisions() {
	collisionSet := make(map[string]bool)
	for _, name := range m.pasteCollisions {
		collisionSet[name] = true
	}
	for _, id := range m.pasteItems {
		c, err := m.cfg.FindByID(id)
		if err != nil {
			continue
		}
		if collisionSet[c.Name] {
			continue // skip
		}
		if m.pasteIsCut {
			c.Group = m.pasteTargetGroup
		} else {
			newConn := *c
			newConn.ID = uuid.New()
			newConn.Group = m.pasteTargetGroup
			m.cfg.Connections = append(m.cfg.Connections, newConn)
		}
	}
	config.Save(m.configDir, m.cfg)
	m.cutConnections = make(map[uuid.UUID]bool)
	m.copyConnections = make(map[uuid.UUID]bool)
}

func (m Model) isNameCollision(name string) bool {
	for _, c := range m.cfg.Connections {
		if c.Name == name && c.Group == m.pasteTargetGroup {
			return true
		}
	}
	return false
}

func (m Model) uniqueName(name string) string {
	candidate := name + "-copy"
	n := 2
	for m.isNameCollision(candidate) {
		candidate = fmt.Sprintf("%s-copy-%d", name, n)
		n++
	}
	return candidate
}

func (m Model) handleGlobalSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.formEditing {
		return m.handleGlobalSettingsEditMode(msg)
	}
	return m.handleGlobalSettingsNavMode(msg)
}

func (m Model) handleGlobalSettingsNavMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.form = formNone
	case "j", "down", "tab":
		m.formCursor++
		if m.formCursor == fieldUseGlobalSettings {
			m.formCursor++
		}
		if m.formCursor >= fieldAdvancedCount {
			m.formCursor = fieldForwardAgent
		}
	case "k", "up", "shift+tab":
		m.formCursor--
		if m.formCursor == fieldUseGlobalSettings {
			m.formCursor--
		}
		if m.formCursor < fieldForwardAgent {
			m.formCursor = fieldExtraOptions
		}
	case "enter":
		m.formEditing = true
		m.formEditBuf = m.formFields[m.formCursor]
	case "i", "I":
		m.shellInitScope = shellInitScopeGlobal
		m.shellInitInput = m.cfg.GlobalShellInit
		m.form = formEditShellInit
		if msg.String() == "I" {
			return m, m.openShellInitInEditor()
		}
		return m, nil
	case "ctrl+s":
		opts, _ := parseSSHOptionsFromFields(m.formFields)
		m.globalCfg.SSHOptions = opts
		config.SaveGlobal(m.configDir, m.globalCfg)
		t, cmd := showToast("global settings saved", toastOK)
		m.activeToast = &t
		m.form = formNone
		return m, cmd
	}
	return m, nil
}

func (m Model) handleGlobalSettingsEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.formFields[m.formCursor] = m.formEditBuf
		m.formEditing = false
	case "enter":
		m.formEditing = false
	case "ctrl+s":
		m.formEditing = false
		opts, _ := parseSSHOptionsFromFields(m.formFields)
		m.globalCfg.SSHOptions = opts
		config.SaveGlobal(m.configDir, m.globalCfg)
		t, cmd := showToast("global settings saved", toastOK)
		m.activeToast = &t
		m.form = formNone
		return m, cmd
	case "l":
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			next := opts[0]
			for i, o := range opts {
				if o == current && i+1 < len(opts) {
					next = opts[i+1]
					break
				}
			}
			m.formFields[m.formCursor] = next
			return m, nil
		}
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	case "h":
		if opts, ok := fieldCycleOptions[m.formCursor]; ok {
			current := m.formFields[m.formCursor]
			prev := opts[len(opts)-1]
			for i, o := range opts {
				if o == current && i > 0 {
					prev = opts[i-1]
					break
				}
			}
			m.formFields[m.formCursor] = prev
			return m, nil
		}
		if m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	case "backspace":
		if _, ok := fieldCycleOptions[m.formCursor]; ok {
			return m, nil
		}
		if m.formCursor < len(m.formFields) && len(m.formFields[m.formCursor]) > 0 {
			m.formFields[m.formCursor] = m.formFields[m.formCursor][:len(m.formFields[m.formCursor])-1]
		}
	default:
		if _, ok := fieldCycleOptions[m.formCursor]; ok {
			return m, nil
		}
		if len(msg.String()) == 1 && m.formCursor < len(m.formFields) {
			m.formFields[m.formCursor] += msg.String()
		}
	}
	return m, nil
}

func (m Model) saveForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.formFields[fieldName])
	host := strings.TrimSpace(m.formFields[fieldHost])
	portStr := strings.TrimSpace(m.formFields[fieldPort])
	user := strings.TrimSpace(m.formFields[fieldUser])
	key := strings.TrimSpace(m.formFields[fieldKey])
	jump := strings.TrimSpace(m.formFields[fieldJump])
	group := strings.TrimSpace(m.formFields[fieldGroup])
	tagsStr := strings.TrimSpace(m.formFields[fieldTags])
	password := m.formFields[fieldPassword]

	// Validate
	if name == "" {
		m.formError = "Name is required"
		return m, nil
	}
	if host == "" {
		m.formError = "Host is required"
		return m, nil
	}
	if user == "" {
		m.formError = "User is required"
		return m, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		m.formError = "Port must be a positive number"
		return m, nil
	}

	// Parse tags
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	// Resolve JumpHost display name to UUID for storage
	jumpResolved := m.jumpHostResolve(jump)

	// Parse SSH options from advanced fields
	sshOpts, useGlobal := parseSSHOptionsFromFields(m.formFields)

	// If the password input is a $(...) command, store it on the connection
	// (resolved at connect time) and clear the keychain. Literal passwords
	// continue to go to the keychain as before.
	passwordCmd := ""
	if config.IsCommandValue(password) {
		passwordCmd = password
	}

	conn := config.Connection{
		Name:              name,
		Host:              host,
		Port:              port,
		User:              user,
		IdentityFile:      key,
		JumpHost:          jumpResolved,
		Group:             group,
		Tags:              tags,
		SSHOptions:        sshOpts,
		UseGlobalSettings: useGlobal,
		Password:          passwordCmd,
	}

	if m.form == formAdd {
		if err := m.cfg.Add(conn); err != nil {
			m.formError = err.Error()
			return m, nil
		}
	} else if m.form == formEdit {
		existing, findErr := m.cfg.FindByID(m.formTarget)
		if findErr != nil {
			m.formError = findErr.Error()
			return m, nil
		}
		// Preserve fields not represented in the form.
		conn.ID = existing.ID
		conn.Scripts = existing.Scripts
		conn.Notes = existing.Notes
		conn.SyncedFromSSHConfig = existing.SyncedFromSSHConfig
		conn.ShellInit = existing.ShellInit
		conn.UseGlobalShellInit = existing.UseGlobalShellInit
		// Update in place so the connection keeps its position in its group.
		if err := m.cfg.UpdateByID(m.formTarget, conn); err != nil {
			m.formError = err.Error()
			return m, nil
		}
	}

	// Auto-register the group if it is new (typed as free text in the form).
	if conn.Group != "" && groupIndex(m.cfg.Groups, conn.Group) < 0 {
		m.cfg.Groups = append(m.cfg.Groups, conn.Group)
	}

	// Save to disk
	if err := config.Save(m.configDir, m.cfg); err != nil {
		m.formError = "Save failed: " + err.Error()
		return m, nil
	}

	// Save or delete password in keychain. $(...) commands are stored on
	// the Connection itself (above) and must not also live in the keychain.
	connKey := conn.ID.String()
	if password != "" && !config.IsCommandValue(password) {
		config.SetPassword(connKey, password)
	} else {
		config.DeletePassword(connKey)
	}

	m.form = formNone
	m.formError = ""
	m.jumpSuggestions = nil
	t, cmd := showToast("saved "+name, toastOK)
	m.activeToast = &t
	return m, cmd
}
