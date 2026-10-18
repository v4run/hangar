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
	PaletteQuitConfirm
	PaletteGlobalSettings
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
	rows            []row
	collapsed       map[uuid.UUID]bool
	cursor          int  // index into `rows`
	offset          int  // top row visible in sidebar viewport
	recentCollapsed bool // Recent frecency section — collapsed or not

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
	paletteEditing    bool               // nav mode (j/k select) vs edit mode (keystrokes go to the focused field)
	paletteEditSnap   string             // field value when edit mode was entered — restored on esc
	spinner           spinner.Model      // tick-driven busy indicator for host-key scans etc.
	spinning          bool

	// Per-user mutable state (frecency, collapsed-groups); persists
	// in state.json.
	state *config.State

	// Mouse-capture toggle (M key) — off lets the terminal
	// drag-select text instead of hangar swallowing clicks.
	mouseEnabled bool

	// Visual mode + clip buffers (vim-style v / y / x / p).
	visualMode    bool
	visualAnchor  int                   // row index where `v` was pressed
	cutRows       map[uuid.UUID]bool    // marked for cut (move on paste)
	copyRows      map[uuid.UUID]bool    // marked for copy (duplicate on paste)

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

	st := config.LoadState(cfgDir)
	collapsed := make(map[uuid.UUID]bool, len(st.CollapsedGroups))
	for _, id := range st.CollapsedGroups {
		collapsed[id] = true
	}
	mouseEnabled := true
	if st.MouseDisabled != nil && *st.MouseDisabled {
		mouseEnabled = false
	}
	m := Model{
		cfg:             cfg,
		cfgDir:          cfgDir,
		collapsed:       collapsed,
		recentCollapsed: st.RecentCollapsed,
		keymap:          DefaultKeymap(),
		help:            help.New(),
		cmdInput:        ti,
		filter:          fi,
		spinner:         sp,
		state:           st,
		mouseEnabled:    mouseEnabled,
		cutRows:         map[uuid.UUID]bool{},
		copyRows:        map[uuid.UUID]bool{},
	}
	m.rebuildRows()
	return m
}

// persistMouse writes the current mouse-enabled flag to state.json.
func (m *Model) persistMouse() {
	disabled := !m.mouseEnabled
	m.state.MouseDisabled = &disabled
	_ = config.SaveState(m.cfgDir, m.state)
}

// persistCollapsed writes the current fold layout to state.json.
// Non-fatal on error — the in-memory state is still correct.
func (m *Model) persistCollapsed() {
	ids := make([]uuid.UUID, 0, len(m.collapsed))
	for id, c := range m.collapsed {
		if c {
			ids = append(ids, id)
		}
	}
	m.state.CollapsedGroups = ids
	m.state.RecentCollapsed = m.recentCollapsed
	_ = config.SaveState(m.cfgDir, m.state)
}

// Init satisfies tea.Model. If the persisted mouse state was
// disabled, emit tea.DisableMouse up front so the program starts
// in that mode (the Program was launched with tea.WithMouseCellMotion
// in main.go to keep the "default on" fast path).
func (m Model) Init() tea.Cmd {
	if !m.mouseEnabled {
		return tea.Batch(textinput.Blink, tea.DisableMouse)
	}
	return textinput.Blink
}
