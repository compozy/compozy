package tools

import (
	"fmt"
	"slices"
	"strings"
)

// Toolset groups exact tools, patterns, and nested toolsets.
type Toolset struct {
	ID       ToolsetID   `json:"id"`
	Tools    []string    `json:"tools,omitempty"`
	Toolsets []ToolsetID `json:"toolsets,omitempty"`
}

// ToolsetView is a named toolset plus its current expansion diagnostics.
type ToolsetView struct {
	Toolset       Toolset      `json:"toolset"`
	ExpandedTools []ToolID     `json:"expanded_tools,omitempty"`
	ReasonCodes   []ReasonCode `json:"reason_codes,omitempty"`
}

// ToolsetCatalog expands named toolsets into concrete ToolID atoms.
type ToolsetCatalog struct {
	sets map[ToolsetID]Toolset
	ids  []ToolsetID
}

// NewToolsetCatalog validates and indexes toolsets by ID.
func NewToolsetCatalog(toolsets ...Toolset) (ToolsetCatalog, error) {
	sets := make(map[ToolsetID]Toolset, len(toolsets))
	ids := make([]ToolsetID, 0, len(toolsets))
	for i, toolset := range toolsets {
		if err := toolset.Validate(); err != nil {
			return ToolsetCatalog{}, wrapField(err, fmt.Sprintf("toolsets[%d]", i))
		}
		if _, ok := sets[toolset.ID]; ok {
			return ToolsetCatalog{}, NewValidationError(
				"toolsets",
				ReasonToolsetUnknown,
				fmt.Sprintf("duplicate toolset %q", toolset.ID),
			)
		}
		sets[toolset.ID] = cloneToolset(toolset)
		ids = append(ids, toolset.ID)
	}
	slices.Sort(ids)
	return ToolsetCatalog{sets: sets, ids: ids}, nil
}

// Validate ensures the toolset is syntactically expandable.
func (t Toolset) Validate() error {
	if err := t.ID.Validate(); err != nil {
		return err
	}
	for i, raw := range t.Tools {
		if _, err := ParseToolPattern(raw); err != nil {
			return wrapField(err, fmt.Sprintf("tools[%d]", i))
		}
	}
	for i, id := range t.Toolsets {
		if err := id.Validate(); err != nil {
			return wrapField(err, fmt.Sprintf("toolsets[%d]", i))
		}
	}
	return nil
}

// IDs returns the known toolset IDs in deterministic order.
func (c ToolsetCatalog) IDs() []ToolsetID {
	return append([]ToolsetID(nil), c.ids...)
}

// List returns all known toolsets in deterministic order.
func (c ToolsetCatalog) List() []Toolset {
	toolsets := make([]Toolset, 0, len(c.ids))
	for _, id := range c.ids {
		toolsets = append(toolsets, cloneToolset(c.sets[id]))
	}
	return toolsets
}

// Get returns one known toolset definition.
func (c ToolsetCatalog) Get(id ToolsetID) (Toolset, bool) {
	toolset, ok := c.sets[id]
	if !ok {
		return Toolset{}, false
	}
	return cloneToolset(toolset), true
}

// Contains reports whether any selected toolset grants the concrete tool ID.
func (c ToolsetCatalog) Contains(id ToolID, toolsetIDs []ToolsetID) (bool, error) {
	if err := id.Validate(); err != nil {
		return false, err
	}
	selected := append([]ToolsetID(nil), toolsetIDs...)
	slices.Sort(selected)
	for _, toolsetID := range selected {
		contains, err := c.contains(toolsetID, id, nil, make(map[ToolsetID]struct{}))
		if err != nil {
			return false, err
		}
		if contains {
			return true, nil
		}
	}
	return false, nil
}

// Expand resolves one toolset into concrete ToolID atoms.
func (c ToolsetCatalog) Expand(id ToolsetID, universe []ToolID) ([]ToolID, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	known := normalizeToolUniverse(universe)
	collector := make(map[ToolID]struct{})
	if err := c.expand(id, known, collector, nil, make(map[ToolsetID]struct{})); err != nil {
		return nil, err
	}
	return sortedToolIDsFromSet(collector), nil
}

// ExpandPatterns resolves patterns and toolsets into concrete ToolID atoms.
func (c ToolsetCatalog) ExpandPatterns(
	patterns []ToolPattern,
	toolsetIDs []ToolsetID,
	universe []ToolID,
) ([]ToolID, error) {
	known := normalizeToolUniverse(universe)
	collector := make(map[ToolID]struct{})
	for _, pattern := range patterns {
		if err := collectPatternMatches(pattern, known, collector); err != nil {
			return nil, err
		}
	}
	for _, id := range toolsetIDs {
		if err := c.expand(id, known, collector, nil, make(map[ToolsetID]struct{})); err != nil {
			return nil, err
		}
	}
	return sortedToolIDsFromSet(collector), nil
}

func (c ToolsetCatalog) expand(
	id ToolsetID,
	universe []ToolID,
	collector map[ToolID]struct{},
	path []ToolsetID,
	visiting map[ToolsetID]struct{},
) error {
	toolset, ok := c.sets[id]
	if !ok {
		return NewValidationError("toolset", ReasonToolsetUnknown, fmt.Sprintf("unknown toolset %q", id))
	}
	if _, ok := visiting[id]; ok {
		cycle := append(append([]ToolsetID(nil), path...), id)
		return NewValidationError("toolset", ReasonToolsetCycle, "toolset cycle: "+formatToolsetPath(cycle))
	}

	visiting[id] = struct{}{}
	defer delete(visiting, id)

	for _, raw := range toolset.Tools {
		pattern, err := ParseToolPattern(raw)
		if err != nil {
			return err
		}
		if err := collectPatternMatches(pattern, universe, collector); err != nil {
			return err
		}
	}
	nextPath := append(append([]ToolsetID(nil), path...), id)
	nested := append([]ToolsetID(nil), toolset.Toolsets...)
	slices.Sort(nested)
	for _, nestedID := range nested {
		if err := c.expand(nestedID, universe, collector, nextPath, visiting); err != nil {
			return err
		}
	}
	return nil
}

func (c ToolsetCatalog) contains(
	toolsetID ToolsetID,
	toolID ToolID,
	path []ToolsetID,
	visiting map[ToolsetID]struct{},
) (bool, error) {
	toolset, ok := c.sets[toolsetID]
	if !ok {
		return false, NewValidationError(
			"toolset",
			ReasonToolsetUnknown,
			fmt.Sprintf("unknown toolset %q", toolsetID),
		)
	}
	if _, ok := visiting[toolsetID]; ok {
		cycle := append(append([]ToolsetID(nil), path...), toolsetID)
		return false, NewValidationError(
			"toolset",
			ReasonToolsetCycle,
			"toolset cycle: "+formatToolsetPath(cycle),
		)
	}

	visiting[toolsetID] = struct{}{}
	defer delete(visiting, toolsetID)

	matched := false
	for _, raw := range toolset.Tools {
		pattern, err := ParseToolPattern(raw)
		if err != nil {
			return false, err
		}
		matched = matched || pattern.Match(toolID)
	}
	nextPath := append(append([]ToolsetID(nil), path...), toolsetID)
	nested := append([]ToolsetID(nil), toolset.Toolsets...)
	slices.Sort(nested)
	for _, nestedID := range nested {
		contains, err := c.contains(nestedID, toolID, nextPath, visiting)
		if err != nil {
			return false, err
		}
		matched = matched || contains
	}
	return matched, nil
}

func collectPatternMatches(pattern ToolPattern, universe []ToolID, collector map[ToolID]struct{}) error {
	matched := false
	for _, id := range universe {
		if pattern.Match(id) {
			collector[id] = struct{}{}
			matched = true
		}
	}
	if matched {
		return nil
	}
	if id, ok := pattern.exactID(); ok {
		return NewValidationError("tool", ReasonToolUnknown, fmt.Sprintf("unknown tool %q", id))
	}
	return NewValidationError("tool_pattern", ReasonToolUnknown, fmt.Sprintf("pattern %q matched no tools", pattern))
}

func normalizeToolUniverse(ids []ToolID) []ToolID {
	normalized := make([]ToolID, 0, len(ids))
	seen := make(map[ToolID]struct{}, len(ids))
	for _, id := range ids {
		if err := id.Validate(); err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	slices.Sort(normalized)
	return normalized
}

func sortedToolIDsFromSet(set map[ToolID]struct{}) []ToolID {
	ids := make([]ToolID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func cloneToolset(src Toolset) Toolset {
	return Toolset{
		ID:       src.ID,
		Tools:    append([]string(nil), src.Tools...),
		Toolsets: append([]ToolsetID(nil), src.Toolsets...),
	}
}

func formatToolsetPath(path []ToolsetID) string {
	parts := make([]string, 0, len(path))
	for _, id := range path {
		parts = append(parts, id.String())
	}
	return strings.Join(parts, " -> ")
}
