package cmdpalette

import (
	"fmt"
	"regexp"
	"strings"
)

var commandIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func ValidateDescriptor(descriptor Descriptor) error {
	descriptor = normalizeDescriptor(descriptor)
	if descriptor.ID == "" || !commandIDPattern.MatchString(string(descriptor.ID)) {
		return invalidDescriptor("id must be a lowercase dotted identifier")
	}
	if descriptor.Title == "" {
		return invalidDescriptor("%s: title is required", descriptor.ID)
	}
	if descriptor.Section == "" {
		return invalidDescriptor("%s: section is required", descriptor.ID)
	}
	if descriptor.Icon == "" {
		return invalidDescriptor("%s: icon is required", descriptor.ID)
	}
	if err := validateSource(descriptor); err != nil {
		return err
	}
	if err := validateAction(descriptor.ID, descriptor.Action); err != nil {
		return err
	}
	if err := validateArguments(descriptor.ID, descriptor.Arguments); err != nil {
		return err
	}
	if descriptor.Destructive {
		if descriptor.Confirmation == nil {
			return invalidDescriptor("%s: destructive commands require confirmation", descriptor.ID)
		}
		if strings.TrimSpace(descriptor.Confirmation.Title) == "" ||
			strings.TrimSpace(descriptor.Confirmation.Confirm) == "" {
			return invalidDescriptor("%s: confirmation title and confirm label are required", descriptor.ID)
		}
	}
	for index, predicate := range descriptor.When {
		if err := validatePredicate(predicate); err != nil {
			return invalidDescriptor("%s: when[%d]: %v", descriptor.ID, index, err)
		}
	}
	return nil
}

func normalizeDescriptor(descriptor Descriptor) Descriptor {
	descriptor.ID = CommandID(strings.TrimSpace(string(descriptor.ID)))
	descriptor.Title = strings.TrimSpace(descriptor.Title)
	descriptor.Section = strings.TrimSpace(descriptor.Section)
	descriptor.Icon = strings.TrimSpace(descriptor.Icon)
	descriptor.Source.Extension = strings.TrimSpace(descriptor.Source.Extension)
	descriptor.Action.Op = strings.TrimSpace(descriptor.Action.Op)
	descriptor.Action.Tool = strings.TrimSpace(descriptor.Action.Tool)
	descriptor.Action.View = strings.TrimSpace(descriptor.Action.View)
	descriptor.Action.App = strings.TrimSpace(descriptor.Action.App)
	descriptor.Action.URL = strings.TrimSpace(descriptor.Action.URL)
	for index := range descriptor.Arguments {
		descriptor.Arguments[index].Name = strings.TrimSpace(descriptor.Arguments[index].Name)
	}
	return descriptor
}

func validateSource(descriptor Descriptor) error {
	switch descriptor.Source.Kind {
	case SourceKindCore:
		if descriptor.Source.Extension != "" {
			return invalidDescriptor("%s: core source cannot name an extension", descriptor.ID)
		}
	case SourceKindExtension:
		if descriptor.Source.Extension == "" {
			return invalidDescriptor("%s: extension source name is required", descriptor.ID)
		}
		prefix := "ext." + descriptor.Source.Extension + "."
		if !strings.HasPrefix(string(descriptor.ID), prefix) {
			return invalidDescriptor("%s: extension command must use prefix %q", descriptor.ID, prefix)
		}
	default:
		return invalidDescriptor("%s: source kind must be core or extension", descriptor.ID)
	}
	return nil
}

func validateAction(id CommandID, action Action) error {
	values := map[ActionKind]string{
		ActionKindClientOp: action.Op,
		ActionKindTool:     action.Tool,
		ActionKindView:     action.View,
		ActionKindNavigate: action.App,
		ActionKindURL:      action.URL,
	}
	required, exists := values[action.Kind]
	if !exists {
		return invalidDescriptor("%s: unknown action kind %q", id, action.Kind)
	}
	if required == "" {
		return invalidDescriptor("%s: action %q requires its target", id, action.Kind)
	}
	for kind, value := range values {
		if kind != action.Kind && value != "" {
			return invalidDescriptor("%s: action %q cannot carry %q target", id, action.Kind, kind)
		}
	}
	return nil
}

func validateArguments(id CommandID, arguments []Argument) error {
	seen := make(map[string]struct{}, len(arguments))
	for index, argument := range arguments {
		if argument.Name == "" || !commandIDPattern.MatchString(argument.Name) {
			return invalidDescriptor("%s: arguments[%d] has invalid name", id, index)
		}
		if _, exists := seen[argument.Name]; exists {
			return invalidDescriptor("%s: duplicate argument %q", id, argument.Name)
		}
		seen[argument.Name] = struct{}{}
		switch argument.Type {
		case ArgumentTypeText, ArgumentTypePassword, ArgumentTypeCheckbox:
			if len(argument.Options) != 0 {
				return invalidDescriptor("%s: argument %q only allows options for dropdown", id, argument.Name)
			}
		case ArgumentTypeDropdown:
		default:
			return invalidDescriptor("%s: argument %q has unknown type %q", id, argument.Name, argument.Type)
		}
	}
	return nil
}

func invalidDescriptor(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDescriptor, fmt.Sprintf(format, arguments...))
}
