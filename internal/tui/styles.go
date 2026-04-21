package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Sidebar
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("8"))

	// Main pane
	mainPaneStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	statusKeyStyle = lipgloss.NewStyle().
			Bold(true)

	// List items
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13"))

	normalStyle = lipgloss.NewStyle()

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// Section header
	headerStyle = lipgloss.NewStyle().
			Bold(true)

	// Group header in sidebar — dim, uppercase feel
	groupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Bold(true)

	// Detail pane
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Width(7)

	valueStyle = lipgloss.NewStyle()

	titleStyle = lipgloss.NewStyle().
			Bold(true)

	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	// Sidebar selected row background highlight
	sidebarSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Bold(true).
				Foreground(lipgloss.Color("13"))

	// SSH command style — dim monospace feel
	sshCmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// Detail label — fixed width for alignment
	detailLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Width(7)

	// Form
	activeFieldStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("13")).
				Bold(true)

	// Indicators
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
)
