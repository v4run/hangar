package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/v4run/hangar/internal/config"
	"github.com/v4run/hangar/internal/fleet"
)

type focus int

const (
	focusSidebar focus = iota
	focusSession
)

type Model struct {
	cfg              *config.HangarConfig
	globalCfg        *config.GlobalConfig
	configDir        string
	width            int
	height           int
	focus            focus
	cursor           int
	sshConfigChanged bool
	filterText       string
	filtering        bool
	quitting         bool
	sessions         []*Session
	activeSession    int  // index into sessions, -1 if none
	hangarMode       bool // true when prefix key (Ctrl+a) pressed
	execMode         bool     // true when in exec input mode
	execInput        string   // the command being typed
	execOutput       []string // output lines from fleet exec
	execRunning      bool     // true while exec is in progress
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
		activeSession:    -1,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize active session PTY if any
		if m.activeSession >= 0 && m.activeSession < len(m.sessions) {
			sidebarWidth := 25
			s := m.sessions[m.activeSession]
			cols := m.width - sidebarWidth - 3
			rows := m.height - 4
			if cols > 0 && rows > 0 {
				s.Resize(rows, cols)
			}
		}

	case tickMsg:
		// Re-render to show updated session output
		if m.hasActiveSessions() {
			return m, tickCmd()
		}

	case execResultMsg:
		m.execOutput = msg.lines
		m.execRunning = false
		return m, nil

	case tea.KeyMsg:
		// If in an active session and NOT in hangar mode, forward to PTY
		if m.focus == focusSession && !m.hangarMode && m.activeSession >= 0 {
			if msg.String() == "ctrl+a" {
				m.hangarMode = true
				return m, nil
			}
			// Forward keystroke to session
			s := m.sessions[m.activeSession]
			if s.IsActive() {
				data := keyToBytes(msg)
				if len(data) > 0 {
					s.Write(data)
				}
			}
			return m, nil
		}

		// If in hangar mode
		if m.hangarMode {
			switch msg.String() {
			case "esc":
				m.hangarMode = false
				m.focus = focusSidebar
			case "j", "down":
				if m.activeSession < len(m.sessions)-1 {
					m.activeSession++
				}
			case "k", "up":
				if m.activeSession > 0 {
					m.activeSession--
				}
			case "enter":
				m.hangarMode = false
				m.focus = focusSession
			case "d":
				// Disconnect current session
				if m.activeSession >= 0 && m.activeSession < len(m.sessions) {
					m.sessions[m.activeSession].Close()
					m.sessions = append(m.sessions[:m.activeSession], m.sessions[m.activeSession+1:]...)
					if len(m.sessions) == 0 {
						m.activeSession = -1
						m.hangarMode = false
						m.focus = focusSidebar
					} else {
						if m.activeSession >= len(m.sessions) {
							m.activeSession = len(m.sessions) - 1
						}
					}
				}
			}
			return m, nil
		}

		// Exec input mode
		if m.execMode && !m.execRunning {
			switch msg.String() {
			case "esc":
				m.execMode = false
				m.execInput = ""
				m.execOutput = nil
			case "enter":
				if m.execInput != "" {
					m.execRunning = true
					return m, m.startExec()
				}
			case "backspace":
				if len(m.execInput) > 0 {
					m.execInput = m.execInput[:len(m.execInput)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.execInput += msg.String()
				}
			}
			return m, nil
		}

		// Exec mode showing results — only esc to close
		if m.execMode && m.execRunning {
			return m, nil
		}
		if m.execMode && len(m.execOutput) > 0 {
			if msg.String() == "esc" {
				m.execMode = false
				m.execInput = ""
				m.execOutput = nil
			}
			return m, nil
		}

		// Filtering mode
		if m.filtering {
			return m.handleFilterInput(msg)
		}

		// Normal sidebar mode
		switch msg.String() {
		case "q", "ctrl+c":
			// Close all sessions before quitting
			for _, s := range m.sessions {
				s.Close()
			}
			m.quitting = true
			return m, tea.Quit
		case "j", "down":
			conns := m.filteredConnections()
			if m.cursor < len(conns)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "/":
			m.filtering = true
			m.filterText = ""
		case "s", "S":
			if m.sshConfigChanged {
				return m, m.doSync()
			}
		case "e":
			m.execMode = true
			m.execInput = ""
			m.execOutput = nil
		case "c":
			// Connect to highlighted connection
			conns := m.filteredConnections()
			if m.cursor < len(conns) {
				conn := conns[m.cursor]
				var jumpHost *config.Connection
				if conn.JumpHost != "" {
					jh, _ := m.cfg.FindByName(conn.JumpHost)
					jumpHost = jh
				}
				s, err := NewSession(&conn, jumpHost)
				if err == nil {
					m.sessions = append(m.sessions, s)
					m.activeSession = len(m.sessions) - 1
					m.focus = focusSession
					m.hangarMode = false
					// Resize to current dimensions
					sidebarWidth := 25
					cols := m.width - sidebarWidth - 3
					rows := m.height - 4
					if cols > 0 && rows > 0 {
						s.Resize(rows, cols)
					}
					return m, tickCmd()
				}
			}
		}
	}

	return m, nil
}

// keyToBytes converts a bubbletea key message to raw bytes for PTY input.
func keyToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		return []byte{127}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeyEscape:
		return []byte{27}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyUp:
		return []byte{27, '[', 'A'}
	case tea.KeyDown:
		return []byte{27, '[', 'B'}
	case tea.KeyRight:
		return []byte{27, '[', 'C'}
	case tea.KeyLeft:
		return []byte{27, '[', 'D'}
	case tea.KeyCtrlC:
		return []byte{3}
	case tea.KeyCtrlD:
		return []byte{4}
	case tea.KeyCtrlZ:
		return []byte{26}
	case tea.KeyCtrlL:
		return []byte{12}
	case tea.KeyDelete:
		return []byte{27, '[', '3', '~'}
	case tea.KeyHome:
		return []byte{27, '[', 'H'}
	case tea.KeyEnd:
		return []byte{27, '[', 'F'}
	case tea.KeyPgUp:
		return []byte{27, '[', '5', '~'}
	case tea.KeyPgDown:
		return []byte{27, '[', '6', '~'}
	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			return []byte(string(msg.Runes))
		}
	}
	return nil
}

func (m Model) hasActiveSessions() bool {
	for _, s := range m.sessions {
		if s.IsActive() {
			return true
		}
	}
	return false
}

func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.cursor = 0
	case "enter":
		m.filtering = false
		m.cursor = 0
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.cursor = 0
		}
	}
	return m, nil
}

func (m Model) filteredConnections() []config.Connection {
	if m.filterText == "" {
		return m.cfg.Connections
	}
	var filtered []config.Connection
	seen := make(map[string]bool)
	lower := strings.ToLower(m.filterText)
	for _, c := range m.cfg.Connections {
		if seen[c.Name] {
			continue
		}
		if strings.Contains(strings.ToLower(c.Name), lower) {
			filtered = append(filtered, c)
			seen[c.Name] = true
			continue
		}
		for _, t := range c.Tags {
			if strings.Contains(strings.ToLower(t), lower) {
				filtered = append(filtered, c)
				seen[c.Name] = true
				break
			}
		}
	}
	return filtered
}

func (m Model) doSync() tea.Cmd {
	return func() tea.Msg {
		return nil
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 {
		return "Loading..."
	}

	// Title bar
	titleLeft := titleStyle.Render(" hangar ")
	syncIndicator := ""
	if m.sshConfigChanged {
		syncIndicator = syncIndicatorStyle.Render(" SSH config changed (S to sync) ")
	}
	titlePadding := m.width - lipgloss.Width(titleLeft) - lipgloss.Width(syncIndicator)
	if titlePadding < 0 {
		titlePadding = 0
	}
	titleBar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleLeft,
		lipgloss.NewStyle().Width(titlePadding).Render(""),
		syncIndicator,
	)

	// Content area
	contentHeight := m.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}
	sidebarWidth := 25

	sidebar := m.renderSidebar()
	mainPane := m.renderMainPane()

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Width(sidebarWidth).Height(contentHeight).Render(sidebar),
		mainPaneStyle.Width(m.width-sidebarWidth-3).Height(contentHeight).Render(mainPane),
	)

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, content, statusBar)
}

func (m Model) renderStatusBar() string {
	if m.hangarMode {
		return statusBarStyle.Render(" HANGAR MODE: [j/k]navigate [enter]switch [d]disconnect [esc]back")
	}
	if m.execMode {
		if m.execRunning {
			return statusBarStyle.Render(" EXEC: running...")
		} else if len(m.execOutput) > 0 {
			return statusBarStyle.Render(" EXEC: complete  [esc]close")
		}
		return statusBarStyle.Render(" EXEC: type command, [enter]run [esc]cancel")
	}
	if m.focus == focusSession {
		return statusBarStyle.Render(" SESSION: Ctrl+a for hangar mode")
	}
	return statusBarStyle.Render(" [c]onnect [d]isconnect [e]xec [s]ync [t]ag [/]find  [q]uit")
}

func (m Model) renderSidebar() string {
	var b strings.Builder

	b.WriteString(sectionHeaderStyle.Render("Connections"))
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(fmt.Sprintf("> %s_\n", m.filterText))
	} else if m.filterText != "" {
		b.WriteString(fmt.Sprintf("> %s\n", m.filterText))
	} else {
		b.WriteString("  /filter...\n")
	}

	conns := m.filteredConnections()
	for i, c := range conns {
		style := normalStyle
		prefix := "  "
		if m.focus == focusSidebar && i == m.cursor {
			style = selectedStyle
			prefix = "> "
		}
		b.WriteString(style.Render(prefix + c.Name))
		b.WriteString("\n")
	}

	// Sessions section
	if len(m.sessions) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionHeaderStyle.Render(fmt.Sprintf("Sessions (%d)", len(m.sessions))))
		b.WriteString("\n")
		for i, s := range m.sessions {
			style := normalStyle
			prefix := "  "
			if (m.focus == focusSession || m.hangarMode) && i == m.activeSession {
				style = selectedStyle
				prefix = "> "
			}
			status := ""
			if !s.IsActive() {
				status = " (closed)"
			}
			b.WriteString(style.Render(prefix + s.Name + status))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderMainPane() string {
	// Show exec mode
	if m.execMode {
		var b strings.Builder
		if m.execRunning {
			b.WriteString("Running: " + m.execInput + "...\n\n")
		} else if len(m.execOutput) > 0 {
			b.WriteString("Exec: " + m.execInput + "\n\n")
			for _, line := range m.execOutput {
				b.WriteString(line + "\n")
			}
		} else {
			b.WriteString("Exec> " + m.execInput + "_\n\n")
			b.WriteString("Press Enter to run, Esc to cancel\n")
		}
		return b.String()
	}

	// Show session output when viewing a session
	if m.focus == focusSession && m.activeSession >= 0 && m.activeSession < len(m.sessions) {
		s := m.sessions[m.activeSession]
		output := string(s.Output())
		lines := strings.Split(output, "\n")
		maxLines := m.height - 4
		if maxLines < 1 {
			maxLines = 1
		}
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		return strings.Join(lines, "\n")
	}

	// Show session output in hangar mode too
	if m.hangarMode && m.activeSession >= 0 && m.activeSession < len(m.sessions) {
		s := m.sessions[m.activeSession]
		output := string(s.Output())
		lines := strings.Split(output, "\n")
		maxLines := m.height - 4
		if maxLines < 1 {
			maxLines = 1
		}
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		return strings.Join(lines, "\n")
	}

	// Show connection details
	conns := m.filteredConnections()
	if len(conns) == 0 {
		return "No connections. Use 'hangar add' or 'hangar sync' to get started."
	}
	if m.cursor >= len(conns) {
		return ""
	}

	c := conns[m.cursor]
	var b strings.Builder
	b.WriteString(selectedStyle.Render(c.Name))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Host: %s\n", c.Host))
	b.WriteString(fmt.Sprintf("  Port: %d\n", c.Port))
	b.WriteString(fmt.Sprintf("  User: %s\n", c.User))
	if c.IdentityFile != "" {
		b.WriteString(fmt.Sprintf("  Key:  %s\n", c.IdentityFile))
	}
	if c.JumpHost != "" {
		b.WriteString(fmt.Sprintf("  Jump: %s\n", c.JumpHost))
	}
	if len(c.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(c.Tags, ", ")))
	}
	if c.SyncedFromSSHConfig {
		b.WriteString("  Source: synced from SSH config\n")
	}

	return b.String()
}

type execResultMsg struct {
	lines []string
}

func (m Model) startExec() tea.Cmd {
	return func() tea.Msg {
		targets := m.cfg.Connections
		if len(targets) == 0 {
			return execResultMsg{lines: []string{"No connections configured."}}
		}

		output := make(chan fleet.Result, 100)
		go fleet.Execute(targets, m.execInput, output, m.cfg)

		var lines []string
		serverNames := make([]string, len(targets))
		for i, t := range targets {
			serverNames[i] = t.Name
		}
		colors := fleet.AssignColors(serverNames)

		for result := range output {
			if result.Err != nil {
				lines = append(lines, fleet.FormatLine(result.Server, colors[result.Server],
					fmt.Sprintf("ERROR: %v", result.Err), true))
			} else {
				lines = append(lines, fleet.FormatLine(result.Server, colors[result.Server],
					result.Line, true))
			}
		}

		return execResultMsg{lines: lines}
	}
}

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
