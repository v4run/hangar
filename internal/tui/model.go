package tui

import (
	"time"

	"github.com/google/uuid"
	"github.com/v4run/hangar/internal/config"
)

type focus int

const (
	focusSidebar focus = iota
	focusScripts
)

type formMode int

const (
	formNone formMode = iota
	formAdd
	formEdit
	formDelete
	formTag
	formSync
	formAddGroup
	formDeleteGroup
	formAddScript
	formEditScript
	formDeleteScript
	formEditNotes
	formEditGroup
	formGlobalSettings
	formPasteConfirm
)

const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldUser
	fieldKey
	fieldJump
	fieldGroup
	fieldTags
	fieldPassword
	fieldCount
)

var fieldLabels = []string{"Name", "Host", "Port", "User", "Key", "Jump", "Group", "Tags", "Pass"}

const (
	fieldForwardAgent = fieldCount + iota
	fieldCompression
	fieldLocalForward
	fieldRemoteForward
	fieldServerAliveInterval
	fieldServerAliveCountMax
	fieldStrictHostKeyCheck
	fieldRequestTTY
	fieldEnvVars
	fieldExtraOptions
	fieldUseGlobalSettings
	fieldAdvancedCount
)

var advancedFieldLabels = []string{
	"FwdAgent", "Compress", "LocalFwd", "RemoteFwd",
	"Alive", "AliveMax", "HostKey", "TTY",
	"Envs", "Extra", "UseGlobal",
}

// fieldCycleOptions defines the valid values for constrained fields.
// Space cycles forward through the options.
var fieldCycleOptions = map[int][]string{
	fieldForwardAgent:       {"", "yes", "no"},
	fieldCompression:        {"", "yes", "no"},
	fieldStrictHostKeyCheck: {"", "yes", "no", "accept-new"},
	fieldRequestTTY:         {"", "yes", "no", "force", "auto"},
	fieldUseGlobalSettings:  {"yes", "no"},
}

// fieldPlaceholders shows example values for free text advanced fields.
var fieldPlaceholders = map[int]string{
	fieldLocalForward:        "8080:localhost:80, 9090:localhost:90",
	fieldRemoteForward:       "3000:localhost:3000",
	fieldServerAliveInterval: "60",
	fieldServerAliveCountMax: "3",
	fieldEnvVars:             "KEY=value, ANOTHER=val",
	fieldExtraOptions:        "TCPKeepAlive=yes, LogLevel=INFO",
}

// sidebarItem represents a row in the sidebar — either a group header or a connection.
type sidebarItem struct {
	isGroup bool
	group   string             // group name (for headers)
	conn    *config.Connection // connection (for connection rows)
}

type Model struct {
	cfg              *config.HangarConfig
	globalCfg        *config.GlobalConfig
	configDir        string
	width            int
	height           int
	focus            focus
	cursor           int
	sidebarOffset    int // index of first visible item in sidebar viewport
	sshConfigChanged bool
	filterText       string
	filtering        bool
	collapsed        map[string]bool     // collapsed group state
	cutConnections   map[uuid.UUID]bool  // IDs of connections being moved (cut)
	copyConnections  map[uuid.UUID]bool  // IDs of connections being copied
	groupNameInput   string              // input for new group name
	quitting         bool
	form             formMode
	formFields       []string            // field values
	formCursor       int                 // which field is focused
	formError        string              // validation error message
	formTarget       uuid.UUID           // connection ID being edited/deleted/tagged
	formTargetGroup  string              // group name being edited/deleted
	tagInput         string              // input for tag mode
	syncEntries      []config.Connection // parsed SSH config entries for sync selection
	syncSelected     []bool              // selection state per entry
	syncCursor       int                 // cursor position in sync list
	scriptCursor     int                 // cursor in scripts list
	scriptName       string              // script name being added/edited
	scriptCommand    string              // script command being added/edited
	scriptField      int                 // 0=name, 1=command
	scriptTarget     int                 // index of script being edited
	notesInput       string              // notes text being edited
	jumpSuggestions  []config.Connection // autocomplete suggestions for JumpHost
	jumpSugCursor    int                 // cursor in jump suggestions
	activeToast      *toast              // transient status message
	showHelp         bool                // help overlay visible
	pasteCollisions  []string            // names that conflict in target group
	pasteTargetGroup string              // where items are being pasted
	pasteItems       []uuid.UUID         // items to paste
	pasteIsCut       bool                // true = cut (move), false = copy (duplicate)
	connecting       bool                // connect banner visible
	connectTarget    *config.Connection  // connection being connected to
	connectStart     time.Time           // when connection started
	syncFilterText   string              // filter text in sync list
	syncFiltering    bool                // whether sync filter is active
	visualMode       bool                // visual selection mode active
	visualAnchor     int                 // cursor position when visual mode started
	tagTokens        []string            // committed tags in the editor
	tagBuffer        string              // current typing buffer
	runningScriptIdx int                 // index of script being run (-1 if none)
	runningIsGlobal  bool                // whether the running script is global
}

type sshExitMsg struct{ err error }

type connectReadyMsg struct{}

func NewModel(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) Model {
	return Model{
		cfg:              cfg,
		globalCfg:        globalCfg,
		configDir:        configDir,
		focus:            focusSidebar,
		sshConfigChanged: sshChanged,
		collapsed:        make(map[string]bool),
		cutConnections:   make(map[uuid.UUID]bool),
		copyConnections:  make(map[uuid.UUID]bool),
		runningScriptIdx: -1,
	}
}
