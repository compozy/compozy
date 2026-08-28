package runtimeoption

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Selection is one typed provider-advertised ACP runtime option.
// Exactly one of ValueID or BoolValue must be set.
type Selection struct {
	ID        string `json:"id"                   yaml:"id"                   toml:"id"`
	ValueID   string `json:"value_id,omitempty"   yaml:"value_id,omitempty"   toml:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty" yaml:"bool_value,omitempty" toml:"bool_value,omitempty"`
}

// Normalize validates and returns an ownership-safe canonical selection.
func (s Selection) Normalize() (Selection, error) {
	normalized := Selection{
		ID:      strings.TrimSpace(s.ID),
		ValueID: strings.TrimSpace(s.ValueID),
	}
	if normalized.ID == "" {
		return Selection{}, errors.New("ACP option ID is required")
	}
	if (normalized.ValueID == "") == (s.BoolValue == nil) {
		return Selection{}, errors.New("ACP option selection requires exactly one value_id or bool_value")
	}
	if s.BoolValue != nil {
		normalized.BoolValue = new(*s.BoolValue)
	}
	return normalized, nil
}

// Validate checks one selection against the typed runtime contract.
func (s Selection) Validate(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "acp_options"
	}
	if _, err := s.Normalize(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// UnmarshalYAML decodes one closed, typed ACP option selection.
func (s *Selection) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind != yaml.MappingNode {
		return errors.New("ACP option selection must be an object")
	}
	*s = Selection{}
	seen := make(map[string]struct{}, len(value.Content)/2)
	for index := 0; index < len(value.Content); index += 2 {
		keyNode := value.Content[index]
		valueNode := value.Content[index+1]
		key := strings.TrimSpace(keyNode.Value)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("ACP option selection.%s is duplicated", key)
		}
		seen[key] = struct{}{}
		switch key {
		case "id", "value_id":
			if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
				return fmt.Errorf("ACP option selection.%s must be a string", key)
			}
			if key == "id" {
				s.ID = valueNode.Value
			} else {
				s.ValueID = valueNode.Value
			}
		case "bool_value":
			if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!bool" {
				return errors.New("ACP option selection.bool_value must be a boolean")
			}
			var boolValue bool
			if err := valueNode.Decode(&boolValue); err != nil {
				return fmt.Errorf("ACP option selection.bool_value: %w", err)
			}
			s.BoolValue = new(boolValue)
		default:
			return fmt.Errorf("ACP option selection.%s is unknown", key)
		}
	}
	normalized, err := s.Normalize()
	if err != nil {
		return err
	}
	*s = normalized
	return nil
}

// NormalizeSelections validates, copies, deduplicates, and sorts selections.
func NormalizeSelections(path string, selections []Selection) ([]Selection, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = "acp_options"
	}
	normalized := make([]Selection, 0, len(selections))
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
	slices.SortFunc(normalized, func(left Selection, right Selection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return normalized, nil
}

// CloneSelections returns an ownership-safe copy with trimmed scalar values.
func CloneSelections(selections []Selection) []Selection {
	if len(selections) == 0 {
		return nil
	}
	cloned := make([]Selection, len(selections))
	for index, selection := range selections {
		cloned[index] = Selection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if selection.BoolValue != nil {
			cloned[index].BoolValue = new(*selection.BoolValue)
		}
	}
	return cloned
}

// CanonicalSelections clones and sorts selections by ID.
func CanonicalSelections(selections []Selection) []Selection {
	canonical := CloneSelections(selections)
	slices.SortFunc(canonical, func(left Selection, right Selection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return canonical
}

// MergeSelections overlays selections by ID and reports the applied overlay IDs.
func MergeSelections(base []Selection, overlay []Selection) ([]Selection, []string) {
	merged := make(map[string]Selection, len(base)+len(overlay))
	for _, selection := range CloneSelections(base) {
		if selection.ID != "" {
			merged[selection.ID] = selection
		}
	}
	applied := make([]string, 0, len(overlay))
	for _, selection := range CloneSelections(overlay) {
		if selection.ID == "" {
			continue
		}
		if _, exists := merged[selection.ID]; !exists || !slices.Contains(applied, selection.ID) {
			applied = append(applied, selection.ID)
		}
		merged[selection.ID] = selection
	}
	result := make([]Selection, 0, len(merged))
	for _, selection := range merged {
		result = append(result, selection)
	}
	slices.SortFunc(result, func(left Selection, right Selection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	slices.Sort(applied)
	return result, applied
}

// SemanticID canonicalizes an ACP option ID for reserved-setting conflict checks.
func SemanticID(id string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(id))
}
