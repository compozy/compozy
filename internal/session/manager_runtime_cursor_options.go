package session

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func cursorModelOptionSelections(
	model modelcatalog.Model,
	requested []acp.SessionConfigOptionSelection,
) ([]modelcatalog.ModelOptionSelection, error) {
	selected := make(map[string]modelcatalog.ModelOptionSelection, len(model.ConfigOptions))
	descriptors := make(map[string]modelcatalog.ModelOptionDescriptor, len(model.ConfigOptions))
	for _, descriptor := range model.ConfigOptions {
		id := strings.TrimSpace(descriptor.ID)
		if id == "" {
			continue
		}
		descriptors[id] = descriptor
		switch {
		case descriptor.Kind == modelcatalog.ModelOptionKindSelect && strings.TrimSpace(descriptor.CurrentValueID) != "":
			selected[id] = modelcatalog.ModelOptionSelection{
				ID:      id,
				ValueID: strings.TrimSpace(descriptor.CurrentValueID),
			}
		case descriptor.Kind == modelcatalog.ModelOptionKindBoolean && descriptor.CurrentBool != nil:
			selected[id] = modelcatalog.ModelOptionSelection{ID: id, BoolValue: new(*descriptor.CurrentBool)}
		}
	}
	for _, option := range requested {
		id := strings.TrimSpace(option.ID)
		if isDedicatedCursorOptionID(id) {
			return nil, fmt.Errorf("session: Cursor ACP option %q duplicates a dedicated runtime setting", id)
		}
		descriptor, ok := descriptors[id]
		if !ok {
			return nil, fmt.Errorf("session: Cursor ACP option %q is not advertised for model %q", id, model.ModelID)
		}
		candidate := modelcatalog.ModelOptionSelection{ID: id, ValueID: strings.TrimSpace(option.ValueID)}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		if err := validateCursorModelOptionSelection(descriptor, candidate); err != nil {
			return nil, err
		}
		selected[id] = candidate
	}
	result := make([]modelcatalog.ModelOptionSelection, 0, len(selected))
	for _, option := range selected {
		result = append(result, option)
	}
	slices.SortFunc(result, func(left, right modelcatalog.ModelOptionSelection) int {
		return strings.Compare(left.ID, right.ID)
	})
	return result, nil
}

func validateCursorModelOptionSelection(
	descriptor modelcatalog.ModelOptionDescriptor,
	selection modelcatalog.ModelOptionSelection,
) error {
	if err := modelcatalog.ValidateModelOptionSelection(selection); err != nil {
		return err
	}
	switch descriptor.Kind {
	case modelcatalog.ModelOptionKindBoolean:
		if selection.BoolValue == nil {
			return fmt.Errorf("session: Cursor ACP option %q requires a boolean value", descriptor.ID)
		}
	case modelcatalog.ModelOptionKindSelect:
		for _, value := range descriptor.Values {
			if strings.TrimSpace(value.ValueID) == selection.ValueID {
				return nil
			}
		}
		return fmt.Errorf(
			"session: Cursor ACP option %q does not allow value %q",
			descriptor.ID,
			selection.ValueID,
		)
	default:
		return fmt.Errorf("session: Cursor ACP option %q has unsupported kind %q", descriptor.ID, descriptor.Kind)
	}
	return nil
}

func isDedicatedCursorOptionID(id string) bool {
	switch strings.TrimSpace(id) {
	case "model", "reasoning_effort", "effort", "fast", "speed":
		return true
	default:
		return false
	}
}

func cursorBindingOptionSelectionsMatch(
	binding []modelcatalog.ModelOptionSelection,
	wanted []modelcatalog.ModelOptionSelection,
) bool {
	if len(binding) != len(wanted) {
		return false
	}
	for _, expected := range wanted {
		matched := false
		for _, candidate := range binding {
			if candidate.ID != expected.ID || candidate.ValueID != expected.ValueID ||
				(candidate.BoolValue == nil) != (expected.BoolValue == nil) {
				continue
			}
			if candidate.BoolValue == nil || *candidate.BoolValue == *expected.BoolValue {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func cursorBooleanOptionSelection(
	selections []modelcatalog.ModelOptionSelection,
	id string,
) (bool, bool) {
	for _, selection := range selections {
		if selection.ID == id && selection.BoolValue != nil {
			return *selection.BoolValue, true
		}
	}
	return false, false
}
