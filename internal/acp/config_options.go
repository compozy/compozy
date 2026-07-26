package acp

import (
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

const (
	booleanTrueText = "true"
)

// SessionConfigOptionKind identifies the ACP config option shape AGH exposes.
type SessionConfigOptionKind string

const (
	// SessionConfigOptionKindSelect is a single-value ACP config selector.
	SessionConfigOptionKindSelect SessionConfigOptionKind = "select"
	// SessionConfigOptionKindBoolean is an ACP boolean config toggle.
	SessionConfigOptionKindBoolean SessionConfigOptionKind = "boolean"
)

// SessionConfigOption captures one active ACP session config option.
type SessionConfigOption struct {
	ID          string
	Label       string
	Description string
	Category    string
	Kind        SessionConfigOptionKind
	Current     string
	Values      []SessionConfigOptionValue
}

// SessionConfigOptionValue captures one selectable value for an ACP config option.
type SessionConfigOptionValue struct {
	Value       string
	Label       string
	Description string
}

// CloneSessionConfigOptions returns a deep copy of session config options.
func CloneSessionConfigOptions(options []SessionConfigOption) []SessionConfigOption {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]SessionConfigOption, 0, len(options))
	for _, option := range options {
		copyOption := option
		copyOption.Values = append([]SessionConfigOptionValue(nil), option.Values...)
		cloned = append(cloned, copyOption)
	}
	return cloned
}

func sessionConfigOptionsFromSDK(options []acpsdk.SessionConfigOption) []SessionConfigOption {
	if len(options) == 0 {
		return nil
	}
	converted := make([]SessionConfigOption, 0, len(options))
	for _, option := range options {
		if convertedOption, ok := sessionConfigOptionFromSDK(option); ok {
			converted = append(converted, convertedOption)
		}
	}
	return converted
}

func sessionConfigOptionFromSDK(option acpsdk.SessionConfigOption) (SessionConfigOption, bool) {
	switch {
	case option.Select != nil:
		selectOption := option.Select
		id := strings.TrimSpace(string(selectOption.Id))
		if id == "" {
			return SessionConfigOption{}, false
		}
		return SessionConfigOption{
			ID:          id,
			Label:       strings.TrimSpace(selectOption.Name),
			Description: trimStringPointer(selectOption.Description),
			Category:    trimConfigOptionCategory(selectOption.Category),
			Kind:        SessionConfigOptionKindSelect,
			Current:     strings.TrimSpace(string(selectOption.CurrentValue)),
			Values:      sessionConfigValuesFromSDK(selectOption.Options),
		}, true
	case option.Boolean != nil:
		booleanOption := option.Boolean
		id := strings.TrimSpace(string(booleanOption.Id))
		if id == "" {
			return SessionConfigOption{}, false
		}
		current := "false"
		if booleanOption.CurrentValue {
			current = booleanTrueText
		}
		return SessionConfigOption{
			ID:          id,
			Label:       strings.TrimSpace(booleanOption.Name),
			Description: trimStringPointer(booleanOption.Description),
			Category:    trimConfigOptionCategory(booleanOption.Category),
			Kind:        SessionConfigOptionKindBoolean,
			Current:     current,
		}, true
	default:
		return SessionConfigOption{}, false
	}
}

func trimConfigOptionCategory(value *acpsdk.SessionConfigOptionCategory) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(string(*value))
}

func sessionConfigValuesFromSDK(options acpsdk.SessionConfigSelectOptions) []SessionConfigOptionValue {
	values := make([]SessionConfigOptionValue, 0)
	if options.Ungrouped != nil {
		for _, value := range *options.Ungrouped {
			values = append(values, sessionConfigValueFromSDK(value))
		}
	}
	if options.Grouped != nil {
		for _, group := range *options.Grouped {
			for _, value := range group.Options {
				values = append(values, sessionConfigValueFromSDK(value))
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func sessionConfigValueFromSDK(value acpsdk.SessionConfigSelectOption) SessionConfigOptionValue {
	return SessionConfigOptionValue{
		Value:       strings.TrimSpace(string(value.Value)),
		Label:       strings.TrimSpace(value.Name),
		Description: trimStringPointer(value.Description),
	}
}

func trimStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func findModelConfigOption(options []SessionConfigOption) (SessionConfigOption, bool) {
	if option, ok := findSelectConfigOption(options, sessionConfigModelKey); ok {
		return option, true
	}
	return findSelectConfigOptionByCategory(options, string(acpsdk.SessionConfigOptionCategoryModel))
}

func findReasoningConfigOption(options []SessionConfigOption) (SessionConfigOption, bool) {
	if option, ok := findSelectConfigOption(options, "reasoning_effort", "effort"); ok {
		return option, true
	}
	return findSelectConfigOptionByCategory(options, string(acpsdk.SessionConfigOptionCategoryThoughtLevel))
}

func findSelectConfigOptionByCategory(options []SessionConfigOption, category string) (SessionConfigOption, bool) {
	for _, option := range options {
		if option.Kind == SessionConfigOptionKindSelect && strings.TrimSpace(option.Category) == category {
			return option, true
		}
	}
	return SessionConfigOption{}, false
}

func findSelectConfigOption(options []SessionConfigOption, candidateIDs ...string) (SessionConfigOption, bool) {
	for _, candidateID := range candidateIDs {
		for _, option := range options {
			if option.Kind != SessionConfigOptionKindSelect {
				continue
			}
			if strings.TrimSpace(option.ID) == candidateID {
				return option, true
			}
		}
	}
	return SessionConfigOption{}, false
}

func configOptionAllowsValue(option SessionConfigOption, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || option.Kind != SessionConfigOptionKindSelect {
		return false
	}
	for _, candidate := range option.Values {
		if strings.TrimSpace(candidate.Value) == value {
			return true
		}
	}
	return false
}
