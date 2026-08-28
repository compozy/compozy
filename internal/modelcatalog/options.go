package modelcatalog

import (
	"fmt"
	"strings"

	"cmp"
	"slices"
)

// ModelOptionKind identifies the shape of a provider model option.
type ModelOptionKind string

const (
	// ModelOptionKindSelect identifies an option with one selected value.
	ModelOptionKindSelect ModelOptionKind = "select"
	// ModelOptionKindBoolean identifies an option with a true or false value.
	ModelOptionKindBoolean ModelOptionKind = "boolean"
)

// ModelOptionValue describes one value advertised by a selectable model option.
type ModelOptionValue struct {
	ValueID     string
	Label       string
	Description string
	GroupID     string
	GroupLabel  string
	Order       int
}

// ModelOptionDescriptor describes one provider model option and its current value.
type ModelOptionDescriptor struct {
	ID             string
	Label          string
	Description    string
	Category       string
	Kind           ModelOptionKind
	CurrentValueID string
	CurrentBool    *bool
	Values         []ModelOptionValue
}

// ModelOptionSelection captures one typed option value associated with a model binding.
type ModelOptionSelection struct {
	ID        string
	ValueID   string
	BoolValue *bool
}

// CloneModelOptionDescriptors returns an ownership-safe copy of model option descriptors.
func CloneModelOptionDescriptors(options []ModelOptionDescriptor) []ModelOptionDescriptor {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]ModelOptionDescriptor, len(options))
	for index, option := range options {
		cloned[index] = option
		cloned[index].CurrentBool = cloneModelRowPointer(option.CurrentBool)
		cloned[index].Values = slices.Clone(option.Values)
	}
	return cloned
}

// CloneModelOptionSelections returns an ownership-safe copy of model option selections.
func CloneModelOptionSelections(selections []ModelOptionSelection) []ModelOptionSelection {
	if len(selections) == 0 {
		return nil
	}
	cloned := make([]ModelOptionSelection, len(selections))
	for index, selection := range selections {
		cloned[index] = selection
		cloned[index].BoolValue = cloneModelRowPointer(selection.BoolValue)
	}
	return cloned
}

// ValidateModelOptionSelection enforces the typed selection invariant.
func ValidateModelOptionSelection(selection ModelOptionSelection) error {
	if strings.TrimSpace(selection.ID) == "" {
		return fmt.Errorf("model catalog: option id is required")
	}
	hasValueID := strings.TrimSpace(selection.ValueID) != ""
	hasBoolValue := selection.BoolValue != nil
	if hasValueID == hasBoolValue {
		return fmt.Errorf("model catalog: option %q must set exactly one of value_id or bool_value", selection.ID)
	}
	return nil
}

func sortModelOptionValues(values []ModelOptionValue) {
	slices.SortFunc(values, func(left ModelOptionValue, right ModelOptionValue) int {
		if order := cmp.Compare(left.Order, right.Order); order != 0 {
			return order
		}
		return cmp.Compare(left.ValueID, right.ValueID)
	})
}
