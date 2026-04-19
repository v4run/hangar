package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primary    = lipgloss.Color("#7C3AED") // violet
	accent     = lipgloss.Color("#06B6D4") // cyan
	subtle     = lipgloss.Color("#64748B") // slate
	text       = lipgloss.Color("#E2E8F0") // light slate
	dimText    = lipgloss.Color("#94A3B8") // dim slate
	success    = lipgloss.Color("#22C55E") // green
	danger     = lipgloss.Color("#EF4444") // red
	warning    = lipgloss.Color("#F59E0B") // amber
	surface    = lipgloss.Color("#1E293B") // dark slate
	cursorBg   = lipgloss.Color("#334155") // slightly lighter

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primary).
			Padding(0, 1)

	titleBarStyle = lipgloss.NewStyle().
			Background(surface)

	// Sidebar
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1)

	// Main pane
	mainPaneStyle = lipgloss.NewStyle().
			Padding(0, 2)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(dimText).
			Background(surface).
			Padding(0, 1)

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	// List items
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(cursorBg).
			Bold(true).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(text).
			Padding(0, 1)

	// Section headers
	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(primary).
				Bold(true).
				MarginBottom(1)

	// Connection details
	detailTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				MarginBottom(1)

	detailLabelStyle = lipgloss.NewStyle().
				Foreground(dimText).
				Width(8)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(text)

	tagStyle = lipgloss.NewStyle().
			Foreground(primary).
			Background(lipgloss.Color("#2E1065")).
			Padding(0, 1).
			MarginRight(1)

	// Indicators
	syncIndicatorStyle = lipgloss.NewStyle().
				Foreground(warning).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(danger).
			Bold(true)

	successLabelStyle = lipgloss.NewStyle().
				Foreground(success)

	// Form
	formTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			MarginBottom(1)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(dimText).
			Width(7).
			Align(lipgloss.Right).
			MarginRight(1)

	formActiveLabel = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true).
			Width(7).
			Align(lipgloss.Right).
			MarginRight(1)

	formInputStyle = lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#334155"))

	formActiveInput = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(accent)

	formReadonlyStyle = lipgloss.NewStyle().
				Foreground(dimText).
				Italic(true)

	// Filter
	filterPromptStyle = lipgloss.NewStyle().
				Foreground(accent)

	filterInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))

	// Sync list
	checkboxOn = lipgloss.NewStyle().
			Foreground(success).
			Bold(true)

	checkboxOff = lipgloss.NewStyle().
			Foreground(dimText)

	importedBadge = lipgloss.NewStyle().
			Foreground(dimText).
			Italic(true)

	// Empty state
	emptyStyle = lipgloss.NewStyle().
			Foreground(dimText).
			Italic(true).
			Padding(2, 0)

	// Divider
	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#334155"))
)
