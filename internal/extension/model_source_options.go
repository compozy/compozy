package extensionpkg

import (
	"fmt"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func modelSourceRuntimeOptions(
	rowIndex int,
	row extensioncontract.ModelSourceRow,
) ([]modelcatalog.ModelOptionDescriptor, []modelcatalog.ModelTransportBinding, error) {
	options, err := modelSourceOptionDescriptors(rowIndex, row.ConfigOptions)
	if err != nil {
		return nil, nil, err
	}
	bindings, err := modelSourceTransportBindings(rowIndex, row.TransportBindings, options)
	if err != nil {
		return nil, nil, err
	}
	return options, bindings, nil
}

func modelSourceOptionDescriptors(
	rowIndex int,
	values []extensioncontract.ModelSourceOptionDescriptor,
) ([]modelcatalog.ModelOptionDescriptor, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(values))
	options := make([]modelcatalog.ModelOptionDescriptor, 0, len(values))
	for optionIndex, value := range values {
		optionID := strings.TrimSpace(value.ID)
		if optionID == "" {
			return nil, modelSourceOptionError(rowIndex, optionIndex, "id is required")
		}
		if _, exists := seen[optionID]; exists {
			return nil, modelSourceOptionError(rowIndex, optionIndex, fmt.Sprintf("id %q is duplicated", optionID))
		}
		seen[optionID] = struct{}{}
		option, err := modelSourceOptionDescriptor(rowIndex, optionIndex, optionID, value)
		if err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, nil
}

func modelSourceOptionDescriptor(
	rowIndex int,
	optionIndex int,
	optionID string,
	value extensioncontract.ModelSourceOptionDescriptor,
) (modelcatalog.ModelOptionDescriptor, error) {
	option := modelcatalog.ModelOptionDescriptor{
		ID:             optionID,
		Label:          strings.TrimSpace(value.Label),
		Description:    strings.TrimSpace(value.Description),
		Category:       strings.TrimSpace(value.Category),
		CurrentValueID: strings.TrimSpace(value.CurrentValueID),
	}
	if value.CurrentBool != nil {
		option.CurrentBool = new(*value.CurrentBool)
	}
	switch value.Kind {
	case extensioncontract.ModelSourceOptionKindSelect:
		option.Kind = modelcatalog.ModelOptionKindSelect
		if option.CurrentBool != nil {
			return modelcatalog.ModelOptionDescriptor{}, modelSourceOptionError(
				rowIndex,
				optionIndex,
				"select option cannot set current_bool",
			)
		}
		values, err := modelSourceOptionValues(rowIndex, optionIndex, value.Values)
		if err != nil {
			return modelcatalog.ModelOptionDescriptor{}, err
		}
		option.Values = values
		if option.CurrentValueID != "" && !modelSourceOptionHasValue(option, option.CurrentValueID) {
			return modelcatalog.ModelOptionDescriptor{}, modelSourceOptionError(
				rowIndex,
				optionIndex,
				fmt.Sprintf("current_value_id %q is not advertised", option.CurrentValueID),
			)
		}
	case extensioncontract.ModelSourceOptionKindBoolean:
		option.Kind = modelcatalog.ModelOptionKindBoolean
		if option.CurrentValueID != "" {
			return modelcatalog.ModelOptionDescriptor{}, modelSourceOptionError(
				rowIndex,
				optionIndex,
				"boolean option cannot set current_value_id",
			)
		}
		if len(value.Values) != 0 {
			return modelcatalog.ModelOptionDescriptor{}, modelSourceOptionError(
				rowIndex,
				optionIndex,
				"boolean option cannot advertise select values",
			)
		}
	default:
		return modelcatalog.ModelOptionDescriptor{}, modelSourceOptionError(
			rowIndex,
			optionIndex,
			fmt.Sprintf("kind %q is not supported", value.Kind),
		)
	}
	return option, nil
}

func modelSourceOptionValues(
	rowIndex int,
	optionIndex int,
	values []extensioncontract.ModelSourceOptionValue,
) ([]modelcatalog.ModelOptionValue, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]modelcatalog.ModelOptionValue, 0, len(values))
	for valueIndex, value := range values {
		valueID := strings.TrimSpace(value.ValueID)
		if valueID == "" {
			return nil, modelSourceOptionValueError(rowIndex, optionIndex, valueIndex, "value_id is required")
		}
		if _, exists := seen[valueID]; exists {
			return nil, modelSourceOptionValueError(
				rowIndex,
				optionIndex,
				valueIndex,
				fmt.Sprintf("value_id %q is duplicated", valueID),
			)
		}
		if value.Order < 0 {
			return nil, modelSourceOptionValueError(rowIndex, optionIndex, valueIndex, "order must be non-negative")
		}
		seen[valueID] = struct{}{}
		normalized = append(normalized, modelcatalog.ModelOptionValue{
			ValueID:     valueID,
			Label:       strings.TrimSpace(value.Label),
			Description: strings.TrimSpace(value.Description),
			GroupID:     strings.TrimSpace(value.GroupID),
			GroupLabel:  strings.TrimSpace(value.GroupLabel),
			Order:       value.Order,
		})
	}
	return normalized, nil
}

func modelSourceTransportBindings(
	rowIndex int,
	values []extensioncontract.ModelSourceTransportBinding,
	options []modelcatalog.ModelOptionDescriptor,
) ([]modelcatalog.ModelTransportBinding, error) {
	if len(values) == 0 {
		return nil, nil
	}
	optionByID := make(map[string]modelcatalog.ModelOptionDescriptor, len(options))
	for _, option := range options {
		optionByID[option.ID] = option
	}
	seen := make(map[string]struct{}, len(values))
	bindings := make([]modelcatalog.ModelTransportBinding, 0, len(values))
	for bindingIndex, value := range values {
		transportID := strings.TrimSpace(value.TransportModelID)
		if transportID == "" {
			return nil, modelSourceBindingError(rowIndex, bindingIndex, "transport_model_id is required")
		}
		if _, exists := seen[transportID]; exists {
			return nil, modelSourceBindingError(
				rowIndex,
				bindingIndex,
				fmt.Sprintf("transport_model_id %q is duplicated", transportID),
			)
		}
		seen[transportID] = struct{}{}
		binding, err := modelSourceTransportBinding(rowIndex, bindingIndex, transportID, value, optionByID)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func modelSourceTransportBinding(
	rowIndex int,
	bindingIndex int,
	transportID string,
	value extensioncontract.ModelSourceTransportBinding,
	optionByID map[string]modelcatalog.ModelOptionDescriptor,
) (modelcatalog.ModelTransportBinding, error) {
	binding := modelcatalog.ModelTransportBinding{
		TransportModelID: transportID,
		Label:            strings.TrimSpace(value.Label),
		Fast:             cloneModelSourceBoolPointer(value.Fast),
		Thinking:         cloneModelSourceBoolPointer(value.Thinking),
	}
	if value.ReasoningEffort != nil {
		effort, err := parseModelSourceReasoningEffort(string(*value.ReasoningEffort))
		if err != nil {
			return modelcatalog.ModelTransportBinding{}, modelSourceBindingError(
				rowIndex,
				bindingIndex,
				err.Error(),
			)
		}
		binding.ReasoningEffort = &effort
	}
	selections, err := modelSourceBindingSelections(
		rowIndex,
		bindingIndex,
		value.OptionSelections,
		optionByID,
	)
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	binding.OptionSelections = selections
	return binding, nil
}

func modelSourceBindingSelections(
	rowIndex int,
	bindingIndex int,
	values []extensioncontract.ModelSourceOptionSelection,
	optionByID map[string]modelcatalog.ModelOptionDescriptor,
) ([]modelcatalog.ModelOptionSelection, error) {
	seen := make(map[string]struct{}, len(values))
	selections := make([]modelcatalog.ModelOptionSelection, 0, len(values))
	for selectionIndex, value := range values {
		selection := modelcatalog.ModelOptionSelection{
			ID:      strings.TrimSpace(value.ID),
			ValueID: strings.TrimSpace(value.ValueID),
		}
		if value.BoolValue != nil {
			selection.BoolValue = new(*value.BoolValue)
		}
		if err := modelcatalog.ValidateModelOptionSelection(selection); err != nil {
			return nil, modelSourceSelectionError(rowIndex, bindingIndex, selectionIndex, err.Error())
		}
		if _, exists := seen[selection.ID]; exists {
			return nil, modelSourceSelectionError(
				rowIndex,
				bindingIndex,
				selectionIndex,
				fmt.Sprintf("option id %q is duplicated", selection.ID),
			)
		}
		option, exists := optionByID[selection.ID]
		if !exists {
			return nil, modelSourceSelectionError(
				rowIndex,
				bindingIndex,
				selectionIndex,
				fmt.Sprintf("option id %q is not advertised", selection.ID),
			)
		}
		if err := validateModelSourceBindingSelection(option, selection); err != nil {
			return nil, modelSourceSelectionError(rowIndex, bindingIndex, selectionIndex, err.Error())
		}
		seen[selection.ID] = struct{}{}
		selections = append(selections, selection)
	}
	return selections, nil
}

func validateModelSourceBindingSelection(
	option modelcatalog.ModelOptionDescriptor,
	selection modelcatalog.ModelOptionSelection,
) error {
	switch option.Kind {
	case modelcatalog.ModelOptionKindSelect:
		if selection.BoolValue != nil {
			return fmt.Errorf("select option %q requires value_id", option.ID)
		}
		if !modelSourceOptionHasValue(option, selection.ValueID) {
			return fmt.Errorf("select option %q does not advertise value_id %q", option.ID, selection.ValueID)
		}
	case modelcatalog.ModelOptionKindBoolean:
		if selection.BoolValue == nil {
			return fmt.Errorf("boolean option %q requires bool_value", option.ID)
		}
	default:
		return fmt.Errorf("option %q has unsupported kind %q", option.ID, option.Kind)
	}
	return nil
}

func modelSourceOptionHasValue(option modelcatalog.ModelOptionDescriptor, valueID string) bool {
	for _, value := range option.Values {
		if value.ValueID == valueID {
			return true
		}
	}
	return false
}

func modelSourceOptionError(rowIndex int, optionIndex int, detail string) error {
	return fmt.Errorf("extension: model source row %d option %d %s", rowIndex, optionIndex, detail)
}

func modelSourceOptionValueError(rowIndex int, optionIndex int, valueIndex int, detail string) error {
	return fmt.Errorf(
		"extension: model source row %d option %d value %d %s",
		rowIndex,
		optionIndex,
		valueIndex,
		detail,
	)
}

func modelSourceBindingError(rowIndex int, bindingIndex int, detail string) error {
	return fmt.Errorf("extension: model source row %d binding %d %s", rowIndex, bindingIndex, detail)
}

func modelSourceSelectionError(rowIndex int, bindingIndex int, selectionIndex int, detail string) error {
	return fmt.Errorf(
		"extension: model source row %d binding %d selection %d %s",
		rowIndex,
		bindingIndex,
		selectionIndex,
		detail,
	)
}

func cloneModelSourceRuntimeFields(
	target *extensioncontract.ModelSourceRow,
	source extensioncontract.ModelSourceRow,
) {
	target.ConfigOptions = make([]extensioncontract.ModelSourceOptionDescriptor, len(source.ConfigOptions))
	for index, option := range source.ConfigOptions {
		target.ConfigOptions[index] = option
		target.ConfigOptions[index].CurrentBool = cloneModelSourceBoolPointer(option.CurrentBool)
		target.ConfigOptions[index].Values = append(
			[]extensioncontract.ModelSourceOptionValue(nil),
			option.Values...,
		)
	}
	target.TransportBindings = make([]extensioncontract.ModelSourceTransportBinding, len(source.TransportBindings))
	for index, binding := range source.TransportBindings {
		target.TransportBindings[index] = binding
		target.TransportBindings[index].Fast = cloneModelSourceBoolPointer(binding.Fast)
		target.TransportBindings[index].Thinking = cloneModelSourceBoolPointer(binding.Thinking)
		if binding.ReasoningEffort != nil {
			effort := *binding.ReasoningEffort
			target.TransportBindings[index].ReasoningEffort = &effort
		}
		target.TransportBindings[index].OptionSelections = make(
			[]extensioncontract.ModelSourceOptionSelection,
			len(binding.OptionSelections),
		)
		for selectionIndex, selection := range binding.OptionSelections {
			target.TransportBindings[index].OptionSelections[selectionIndex] = selection
			target.TransportBindings[index].OptionSelections[selectionIndex].BoolValue =
				cloneModelSourceBoolPointer(selection.BoolValue)
		}
	}
}
