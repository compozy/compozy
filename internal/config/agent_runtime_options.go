package config

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// ACPOptionSelection identifies one provider-advertised ACP option default.
// Exactly one of ValueID or BoolValue must be set.
type ACPOptionSelection struct {
	ID        string `json:"id"                   yaml:"id"                   toml:"id"`
	ValueID   string `json:"value_id,omitempty"   yaml:"value_id,omitempty"   toml:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty" yaml:"bool_value,omitempty" toml:"bool_value,omitempty"`
}

// Normalize trims one ACP option selection and copies its pointer value.
func (s ACPOptionSelection) Normalize() (ACPOptionSelection, error) {
	normalized := ACPOptionSelection{
		ID:        strings.TrimSpace(s.ID),
		ValueID:   strings.TrimSpace(s.ValueID),
		BoolValue: s.BoolValue,
	}
	if normalized.ID == "" {
		return ACPOptionSelection{}, errors.New("config: ACP option ID is required")
	}
	if (normalized.ValueID == "") == (normalized.BoolValue == nil) {
		return ACPOptionSelection{}, errors.New(
			"config: ACP option selection requires exactly one value_id or bool_value",
		)
	}
	if normalized.BoolValue != nil {
		normalized.BoolValue = new(*normalized.BoolValue)
	}
	return normalized, nil
}

// Validate checks one ACP option selection against the typed config contract.
func (s ACPOptionSelection) Validate(path string) error {
	if strings.TrimSpace(path) == "" {
		path = "agent.acp_options"
	}
	if _, err := s.Normalize(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// NormalizeACPOptionSelections validates, copies, and sorts ACP option defaults by ID.
func NormalizeACPOptionSelections(selections []ACPOptionSelection) ([]ACPOptionSelection, error) {
	return normalizeACPOptionSelectionsAt("agent.acp_options", selections)
}

func normalizeACPOptionSelectionsAt(path string, selections []ACPOptionSelection) ([]ACPOptionSelection, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	normalized := make([]ACPOptionSelection, 0, len(selections))
	seen := make(map[string]struct{}, len(selections))
	for index, selection := range selections {
		candidate, err := selection.Normalize()
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", path, index, err)
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf("%s[%d]: ACP option %q is selected more than once", path, index, candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(normalized, func(left ACPOptionSelection, right ACPOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return normalized, nil
}

func validateACPOptionSelections(path string, selections []ACPOptionSelection) error {
	_, err := normalizeACPOptionSelectionsAt(path, selections)
	return err
}

// CloneACPOptionSelections returns an independent copy of ACP option defaults.
func CloneACPOptionSelections(selections []ACPOptionSelection) []ACPOptionSelection {
	if len(selections) == 0 {
		return nil
	}
	cloned := make([]ACPOptionSelection, len(selections))
	for index, selection := range selections {
		cloned[index] = ACPOptionSelection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if selection.BoolValue != nil {
			cloned[index].BoolValue = new(*selection.BoolValue)
		}
	}
	return cloned
}

func canonicalACPOptionSelections(selections []ACPOptionSelection) []ACPOptionSelection {
	canonical := CloneACPOptionSelections(selections)
	slices.SortFunc(canonical, func(left ACPOptionSelection, right ACPOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return canonical
}

func validateAgentSpeed(value speedpkg.Speed, path string) error {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return nil
	}
	if _, err := speedpkg.Parse(trimmed); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func normalizeAgentSpeed(value speedpkg.Speed) speedpkg.Speed {
	return speedpkg.Speed(strings.TrimSpace(string(value)))
}
