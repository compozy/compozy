package dsl

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// ACPOptionSelection identifies one provider-advertised ACP option value.
// Exactly one of ValueID or BoolValue must be set.
type ACPOptionSelection struct {
	ID        string `json:"id"                   yaml:"id"                   toml:"id"`
	ValueID   string `json:"value_id,omitempty"   yaml:"value_id,omitempty"   toml:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty" yaml:"bool_value,omitempty" toml:"bool_value,omitempty"`
}

// Normalize trims one ACP option selection and copies its pointer value.
func (s ACPOptionSelection) Normalize() (ACPOptionSelection, error) {
	normalized := ACPOptionSelection{
		ID:      strings.TrimSpace(s.ID),
		ValueID: strings.TrimSpace(s.ValueID),
	}
	if normalized.ID == "" {
		return ACPOptionSelection{}, errors.New("acp option id is required")
	}
	if (normalized.ValueID == "") == (s.BoolValue == nil) {
		return ACPOptionSelection{}, errors.New(
			"acp option selection requires exactly one value_id or bool_value",
		)
	}
	if s.BoolValue != nil {
		normalized.BoolValue = new(*s.BoolValue)
	}
	return normalized, nil
}

// Validate checks one ACP option selection against the typed runtime contract.
func (s ACPOptionSelection) Validate(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "acp_options"
	}
	if _, err := s.Normalize(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// UnmarshalYAML decodes one strict ACP option selection.
func (s *ACPOptionSelection) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind != yaml.MappingNode {
		return errors.New("acp option selection must be an object")
	}
	*s = ACPOptionSelection{}
	seen := make(map[string]struct{}, len(value.Content)/2)
	for index := 0; index < len(value.Content); index += 2 {
		keyNode := value.Content[index]
		valueNode := value.Content[index+1]
		key := strings.TrimSpace(keyNode.Value)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("acp option selection.%s is duplicated", key)
		}
		seen[key] = struct{}{}
		switch key {
		case "id", "value_id":
			if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
				return fmt.Errorf("acp option selection.%s must be a string", key)
			}
			if key == "id" {
				s.ID = valueNode.Value
			} else {
				s.ValueID = valueNode.Value
			}
		case "bool_value":
			if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!bool" {
				return errors.New("acp option selection.bool_value must be a boolean")
			}
			var boolValue bool
			if err := valueNode.Decode(&boolValue); err != nil {
				return fmt.Errorf("acp option selection.bool_value: %w", err)
			}
			s.BoolValue = new(boolValue)
		default:
			return fmt.Errorf("acp option selection.%s is unknown", key)
		}
	}
	normalized, err := s.Normalize()
	if err != nil {
		return err
	}
	*s = normalized
	return nil
}

// NormalizeACPOptionSelections validates, copies, and sorts ACP selections by ID.
func NormalizeACPOptionSelections(selections []ACPOptionSelection) ([]ACPOptionSelection, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	normalized := make([]ACPOptionSelection, 0, len(selections))
	seen := make(map[string]struct{}, len(selections))
	for index, selection := range selections {
		candidate, err := selection.Normalize()
		if err != nil {
			return nil, fmt.Errorf("acp_options[%d]: %w", index, err)
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf("acp_options[%d]: option %q is selected more than once", index, candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(normalized, func(left ACPOptionSelection, right ACPOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return normalized, nil
}

// CloneACPOptionSelections returns an ownership-safe copy of ACP selections.
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
