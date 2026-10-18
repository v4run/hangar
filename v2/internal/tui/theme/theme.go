// Package theme centralises every lipgloss style v2 uses. All colors
// go through lipgloss.AdaptiveColor so light and dark terminals both
// render legibly. v2 uses a muted gray palette with a single accent
// (cyan on dark, teal on light) for focus / selection.
package theme

import "github.com/charmbracelet/lipgloss"

// Role-colored tokens. Reference these from render code — never a raw
// lipgloss.Color.
var (
	Accent    = lipgloss.AdaptiveColor{Light: "#007a84", Dark: "#5eead4"}
	AccentDim = lipgloss.AdaptiveColor{Light: "#99c9cd", Dark: "#2d6b70"}
	Fg        = lipgloss.AdaptiveColor{Light: "#1f2328", Dark: "#e6edf3"}
	Muted     = lipgloss.AdaptiveColor{Light: "#59636e", Dark: "#8b949e"}
	Subtle    = lipgloss.AdaptiveColor{Light: "#8c959f", Dark: "#656d76"}
	Border    = lipgloss.AdaptiveColor{Light: "#d1d9e0", Dark: "#30363d"}
	BorderHot = lipgloss.AdaptiveColor{Light: "#007a84", Dark: "#5eead4"}
	Bg        = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#0d1117"}
	BgAlt     = lipgloss.AdaptiveColor{Light: "#f6f8fa", Dark: "#161b22"}
	Danger    = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"}
	Success   = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"}
	Warn      = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"}
)

// Reusable styles.
var (
	// Base text at normal weight.
	Normal = lipgloss.NewStyle().Foreground(Fg)

	// Dim / secondary text.
	Dim = lipgloss.NewStyle().Foreground(Muted)

	// Extra-dim hint text.
	Hint = lipgloss.NewStyle().Foreground(Subtle)

	// Field labels in the detail view.
	Label = lipgloss.NewStyle().Foreground(Muted)

	// Section titles at the top of a pane.
	Title = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	// Caret for text input.
	Cursor = lipgloss.NewStyle().Foreground(Accent).Bold(true)

	// Selected sidebar row.
	Selected = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// Row currently in focus (hovered but not activated).
	Focus = lipgloss.NewStyle().Foreground(Fg).Bold(true)

	// Error toast.
	ErrorText = lipgloss.NewStyle().Foreground(Danger)

	// Success toast.
	OKText = lipgloss.NewStyle().Foreground(Success)

	// Pane border — rounded, dim by default.
	Pane = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(0, 1)

	// Pane border when the pane holds focus.
	PaneFocused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderHot).
			Padding(0, 1)

	// Mode chip (BROWSE / EDIT / CONNECT) in the status bar.
	ChipBrowse = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Accent).
			Padding(0, 1).
			Bold(true)
	ChipEdit = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Warn).
			Padding(0, 1).
			Bold(true)
	ChipConnect = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Success).
			Padding(0, 1).
			Bold(true)
)
