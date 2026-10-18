// Package tui is Hangar v2's redesigned terminal UI. One Model, one
// Update, one View — but split across files by concern (sidebar
// tree, detail tabs, picker, cmdbar, leader, palettes).
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/v4run/hangar/v2/internal/config"
	sshauth "github.com/v4run/hangar/v2/internal/ssh"
)

// paneFocus names which pane owns the keyboard arrow keys. The
// sidebar list owns them by default; `tab` moves focus into the
// right-pane detail, `h` / `esc` moves it back.
type paneFocus int

const (
	focusSidebar paneFocus = iota
	focusDetail
)

// Mode names what the next keystroke will do, surfaced as a coloured
// chip in the status bar.
type Mode int

const (
	ModeBrowse Mode = iota
	ModeEdit
	ModeConnect
)

func (m Mode) String() string {
	switch m {
	case ModeEdit:
		return "EDIT"
	case ModeConnect:
		return "CONNECT"
	default:
		return "BROWSE"
	}
}

// Overlay names which (if any) floating widget is currently painted
// over the main view. Only one overlay is active at a time.
type Overlay int

const (
	OverlayNone Overlay = iota
	OverlayCmdBar
	OverlayLeader
	OverlayPicker
	OverlayPalette
	OverlayHelp
)

// PaletteKind identifies which form a floating palette is rendering.
type PaletteKind int

const (
	PaletteNone PaletteKind = iota
	PaletteAddConnection
	PaletteEditConnection
	PaletteDeleteConnection
	PaletteAddGroup
	PaletteEditGroup
	PaletteHostKey
	PaletteConfirmConnect
	PaletteImportSSH
	PaletteNotes
	PaletteScript
	PaletteAddDatabase
	PaletteEditDatabase
	PaletteDeleteDatabase
	PaletteDeleteGroup
	PaletteShellInit
	PaletteAddForward
	PaletteDeleteScript
	PaletteDeleteForward
)

// Tab names a detail-pane tab.
type Tab int

const (
	TabDetails Tab = iota
	TabScripts
	TabForwards
	TabNotes
	tabCount
)

func (t Tab) String() string {
	return []string{"Details", "Scripts", "Forwards", "Notes"}[t]
}

// Model holds all v2 TUI state.
type Model struct {
	cfg       *config.Config
	cfgDir    string
	width     int
	height    int

	// Sidebar: flattened view of the group tree in render order,
	// with expand/collapse state keyed by group ID (`uuid.Nil` for
	// "ungrouped" section).
	rows      []row
	collapsed map[uuid.UUID]bool
	cursor    int  // index into `rows`
	offset    int  // top row visible in sidebar viewport

	// Current detail tab + per-tab item cursor + which pane owns
	// keyboard focus (sidebar list vs the active detail tab).
	tab           Tab
	paneFocus     paneFocus
	scriptCursor  int
	forwardCursor int

	// Mode indicator.
	mode Mode

	// Help footer (bubbles/help).
	help       help.Model
	keymap     KeyMap
	showAll      bool // ? toggles full help overlay
	helpOffset   int  // vertical scroll offset for the help overlay
	leaderOffset int  // vertical scroll offset for the leader overlay

	// Toast / status message.
	toast     string
	toastKind toastKind
	toastGone time.Time

	// Overlay (one at a time).
	overlay Overlay

	// Command bar state.
	cmdInput textinput.Model

	// Leader menu — just a flag; render reads from keymap.
	// (field reserved for future per-group leader menus)

	// Picker state.
	picker pickerState

	// Palette state.
	palette          paletteState
	paletteOpts        *config.SSHOptions // SSHOptions carried into add-conn from ssh-import
	paletteScriptIdx   int                // -1 = new script, else index into Scripts
	paletteForwardIdx  int                // index into the resolved forward list
	paletteForwardDir  string             // "L" or "R" — which list the index refers to
	shellInitScopeSel shellInitScope     // which scope the current ShellInit palette targets
	paletteTextarea   textarea.Model     // multi-line editor used by shell-init / notes palettes
	paletteTags       tagChips           // tag-chip editor for connection + group palettes
	paletteUseTA      bool               // true when the active palette owns the textarea
	paletteUseTags    bool               // true when the active palette owns tagChips
	spinner           spinner.Model      // tick-driven busy indicator for host-key scans etc.
	spinning          bool

	// Per-user mutable state (frecency); persists in state.json.
	state *config.State

	// Connection target of the current connect intent (host-key
	// confirm path).
	pendingConn *config.Connection
	pendingKeys []sshauth.HostKey

	// Right-pane vertical scroll offset for the detail content.
	detailOffset int

	// Sidebar filter. filter is the raw query; filtering == true means
	// the input is capturing keys. The rebuildRows call reads filter
	// each pass and hides non-matching rows.
	filtering bool
	filter    textinput.Model
}

// row is one sidebar line.
type row struct {
	kind    rowKind
	depth   int
	group   *config.Group
	conn    *config.Connection
	db      *config.Database
}

type rowKind int

const (
	rowGroup rowKind = iota
	rowConnection
	rowDatabase
	rowSectionHeader // "Recent", "Ungrouped" labels
)

type toastKind int

const (
	toastOK toastKind = iota
	toastErr
	toastInfo
)

// New returns a fresh Model backed by cfg.
func New(cfg *config.Config, cfgDir string) Model {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.Placeholder = "command"
	ti.CharLimit = 0

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.Placeholder = "filter"

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := Model{
		cfg:       cfg,
		cfgDir:    cfgDir,
		collapsed: make(map[uuid.UUID]bool),
		keymap:    DefaultKeymap(),
		help:      help.New(),
		cmdInput:  ti,
		filter:    fi,
		spinner:   sp,
		state:     config.LoadState(cfgDir),
	}
	m.rebuildRows()
	return m
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}
