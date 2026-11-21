package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/spf13/pflag"
)

func newProjectedCommandFlagSet(descriptor contract.ExtensionCommandPayload) *pflag.FlagSet {
	flags := pflag.NewFlagSet(descriptor.Verb, pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for _, flag := range descriptor.Flags {
		usage := commandFlagHelp(flag)
		if flag.Repeatable {
			flags.StringArray(flag.Name, nil, usage)
			if flag.Type == toolspkg.CommandFlagBoolean {
				flags.Lookup(flag.Name).NoOptDefVal = toolBoolTrue
			}
			continue
		}
		if flag.Type == toolspkg.CommandFlagBoolean {
			flags.Bool(flag.Name, false, usage)
			continue
		}
		flags.String(flag.Name, "", usage)
	}
	return flags
}

func buildCommandInput(
	descriptor contract.ExtensionCommandPayload,
	flags *pflag.FlagSet,
	rawInput string,
) (json.RawMessage, error) {
	if flags == nil {
		return nil, fmt.Errorf("cli: projected flag set is required")
	}
	changed := make(map[string]struct{})
	flags.Visit(func(flag *pflag.Flag) { changed[flag.Name] = struct{}{} })
	if rawInput != "" {
		if len(changed) > 0 {
			return nil, fmt.Errorf("cli: --input and projected command flags are mutually exclusive")
		}
		return parseToolInputJSON("input", rawInput)
	}

	input := make(map[string]any, len(changed))
	for _, descriptorFlag := range descriptor.Flags {
		_, present := changed[descriptorFlag.Name]
		if !present {
			if descriptorFlag.Required {
				return nil, fmt.Errorf(
					"cli: required flag --%s for input field %q is missing",
					descriptorFlag.Name,
					descriptorFlag.Field,
				)
			}
			continue
		}
		value, err := commandFlagValue(flags, descriptorFlag)
		if err != nil {
			return nil, fmt.Errorf(
				"cli: convert flag --%s for input field %q: %w",
				descriptorFlag.Name,
				descriptorFlag.Field,
				err,
			)
		}
		input[descriptorFlag.Field] = value
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("cli: encode command input: %w", err)
	}
	canonical, err := toolspkg.CanonicalJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("cli: canonicalize command input: %w", err)
	}
	return canonical, nil
}

func commandFlagValue(flags *pflag.FlagSet, flag toolspkg.CommandFlag) (any, error) {
	if flag.Repeatable {
		values, err := flags.GetStringArray(flag.Name)
		if err != nil {
			return nil, err
		}
		converted := make([]any, 0, len(values))
		for _, value := range values {
			item, err := convertCommandFlagScalar(value, flag.Type)
			if err != nil {
				return nil, err
			}
			converted = append(converted, item)
		}
		return converted, nil
	}
	if flag.Type == toolspkg.CommandFlagBoolean {
		value, err := flags.GetBool(flag.Name)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	value, err := flags.GetString(flag.Name)
	if err != nil {
		return nil, err
	}
	return convertCommandFlagScalar(value, flag.Type)
}

func convertCommandFlagScalar(value string, flagType toolspkg.CommandFlagType) (any, error) {
	switch flagType {
	case toolspkg.CommandFlagString:
		return value, nil
	case toolspkg.CommandFlagBoolean:
		return parseCommandBoolean(value)
	case toolspkg.CommandFlagInteger:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", value)
		}
		return parsed, nil
	case toolspkg.CommandFlagNumber:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number, got %q", value)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported projected type %q", flagType)
	}
}

func parseCommandBoolean(value string) (bool, error) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("expected boolean, got %q", value)
	}
	return parsed, nil
}

func commandFlagHelp(flag toolspkg.CommandFlag) string {
	parts := []string{string(flag.Type), "field=" + flag.Field}
	if flag.Repeatable {
		parts = append(parts, "repeatable")
	}
	if flag.Required {
		parts = append(parts, "required")
	}
	if flag.Nullable {
		parts = append(parts, "nullable")
	}
	if len(flag.Enum) > 0 {
		parts = append(parts, "choices="+strings.Join(flag.Enum, "|"))
	}
	if len(flag.Default) > 0 {
		parts = append(parts, "default="+string(flag.Default))
	}
	if flag.Minimum != nil {
		parts = append(parts, "minimum="+strconv.FormatFloat(*flag.Minimum, 'g', -1, 64))
	}
	if flag.Maximum != nil {
		parts = append(parts, "maximum="+strconv.FormatFloat(*flag.Maximum, 'g', -1, 64))
	}
	return strings.Join(parts, ", ")
}
