package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/v4run/hangar/internal/config"
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
}

func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
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

	case tea.KeyMsg:
		if m.filtering {
			return m.handleFilterInput(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
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
		}
	}

	return m, nil
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
	statusBar := statusBarStyle.Render(" [c]onnect [d]isconnect [e]xec [s]ync [t]ag [/]find  [q]uit")

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, content, statusBar)
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
		if i == m.cursor {
			style = selectedStyle
			prefix = "> "
		}
		b.WriteString(style.Render(prefix + c.Name))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderMainPane() string {
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

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
