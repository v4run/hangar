package config

import (
	"fmt"

	"github.com/google/uuid"
)

// Validate verifies invariants on the config: unique UUIDs, parent
// edges resolve, no cycles in the group tree, connection group refs
// are real, sibling group names unique within a parent.
func Validate(cfg *Config) error {
	groupByID := make(map[uuid.UUID]*Group, len(cfg.Groups))
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		if g.ID == uuid.Nil {
			return fmt.Errorf("group %q has no id", g.Name)
		}
		if _, dup := groupByID[g.ID]; dup {
			return fmt.Errorf("duplicate group id %s", g.ID)
		}
		groupByID[g.ID] = g
	}

	// Parent edges resolve + no cycles. Walking each node's chain
	// takes O(depth); cumulative O(N·depth) ≤ O(N²) which is fine for
	// configs this size.
	for _, g := range cfg.Groups {
		seen := map[uuid.UUID]bool{g.ID: true}
		cur := g.Parent
		for cur != uuid.Nil {
			if seen[cur] {
				return fmt.Errorf("group cycle involving %q", g.Name)
			}
			seen[cur] = true
			parent, ok := groupByID[cur]
			if !ok {
				return fmt.Errorf("group %q has unknown parent %s", g.Name, cur)
			}
			cur = parent.Parent
		}
	}

	// Sibling-name uniqueness.
	byParent := map[uuid.UUID]map[string]bool{}
	for _, g := range cfg.Groups {
		siblings, ok := byParent[g.Parent]
		if !ok {
			siblings = map[string]bool{}
			byParent[g.Parent] = siblings
		}
		if siblings[g.Name] {
			return fmt.Errorf("duplicate sibling group name %q under the same parent", g.Name)
		}
		siblings[g.Name] = true
	}

	// Connection group refs resolve.
	for _, c := range cfg.Connections {
		if c.ID == uuid.Nil {
			return fmt.Errorf("connection %q has no id", c.Name)
		}
		if c.Group != uuid.Nil {
			if _, ok := groupByID[c.Group]; !ok {
				return fmt.Errorf("connection %q points at unknown group %s", c.Name, c.Group)
			}
		}
	}
	for _, d := range cfg.Databases {
		if d.ID == uuid.Nil {
			return fmt.Errorf("database %q has no id", d.Name)
		}
		if d.Group != uuid.Nil {
			if _, ok := groupByID[d.Group]; !ok {
				return fmt.Errorf("database %q points at unknown group %s", d.Name, d.Group)
			}
		}
	}
	return nil
}

// GroupDepth returns the group's distance from the root; 0 for a
// root group. Returns -1 if the group isn't found.
func GroupDepth(cfg *Config, id uuid.UUID) int {
	groupByID := make(map[uuid.UUID]*Group, len(cfg.Groups))
	for i := range cfg.Groups {
		groupByID[cfg.Groups[i].ID] = &cfg.Groups[i]
	}
	g, ok := groupByID[id]
	if !ok {
		return -1
	}
	depth := 0
	for g.Parent != uuid.Nil {
		g = groupByID[g.Parent]
		if g == nil {
			return depth
		}
		depth++
	}
	return depth
}
