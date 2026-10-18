package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
)

// State is Hangar v2's per-user mutable state: last-used /
// use-count per connection and database, used to drive the Recent
// sidebar section + frecency sort in the picker.
//
// Lives at ~/.hangar/v2/state.json. If missing or unreadable, Hangar
// starts from an empty state — never fatal.
type State struct {
	// Usage[id] records the last-touched time and total count for
	// a connection or database.
	Usage map[uuid.UUID]Usage `json:"usage,omitempty"`
	// CollapsedGroups is the set of group IDs the user has
	// collapsed in the sidebar — persists across runs so the fold
	// layout survives restart.
	CollapsedGroups []uuid.UUID `json:"collapsed_groups,omitempty"`
	// RecentCollapsed persists the frecency section's open/closed
	// state, same spirit.
	RecentCollapsed bool `json:"recent_collapsed,omitempty"`
}

// Usage is one row of the Usage map.
type Usage struct {
	Count    int       `json:"count"`
	LastUsed time.Time `json:"last_used"`
}

// LoadState reads state.json from dir. Returns an empty State on
// missing / malformed files so startup never fails.
func LoadState(dir string) *State {
	path := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// swallow — state is non-critical
		}
		return &State{Usage: map[uuid.UUID]Usage{}}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{Usage: map[uuid.UUID]Usage{}}
	}
	if s.Usage == nil {
		s.Usage = map[uuid.UUID]Usage{}
	}
	return &s
}

// SaveState writes state atomically. Non-fatal on error.
func SaveState(dir string, s *State) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "state.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Touch bumps the usage counter + timestamp for id. Caller saves
// state when convenient.
func (s *State) Touch(id uuid.UUID) {
	u := s.Usage[id]
	u.Count++
	u.LastUsed = time.Now()
	s.Usage[id] = u
}

// TopRecent returns up to n ids ordered by frecency (recent + often).
// Score = count * decay(now - last_used). Decay is a soft exponential
// over 7 days so a thing used last week counts for roughly half as
// much as something used today.
func (s *State) TopRecent(n int) []uuid.UUID {
	type scored struct {
		id    uuid.UUID
		score float64
	}
	ranked := make([]scored, 0, len(s.Usage))
	now := time.Now()
	for id, u := range s.Usage {
		age := now.Sub(u.LastUsed).Hours() / 24.0
		decay := 1.0 / (1.0 + age/7.0)
		ranked = append(ranked, scored{id: id, score: float64(u.Count) * decay})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if n > len(ranked) {
		n = len(ranked)
	}
	out := make([]uuid.UUID, n)
	for i := 0; i < n; i++ {
		out[i] = ranked[i].id
	}
	return out
}
