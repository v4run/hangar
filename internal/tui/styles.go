package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Sidebar
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("238"))

	// Main pane
	mainPaneStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)

	// List items
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Section header
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)

	// Detail pane
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(7)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Bold(true)

	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	// Form
	activeFieldStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	// Indicators
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("78"))
)
