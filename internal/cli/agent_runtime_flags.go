package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func parseAgentSpeedFlag(value string) (contract.Speed, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := speedpkg.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("cli: invalid agent speed: %w", err)
	}
	return contract.Speed(parsed), nil
}

func parseAgentACPFlags(optionValues, toggleValues []string) ([]contract.AgentACPOptionSelection, error) {
	options, err := parseAgentACPOptionFlags(optionValues)
	if err != nil {
		return nil, err
	}
	toggles, err := parseAgentACPToggleFlags(toggleValues)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(options)+len(toggles))
	for _, option := range options {
		seen[option.ID] = struct{}{}
	}
	for _, toggle := range toggles {
		if _, exists := seen[toggle.ID]; exists {
			return nil, fmt.Errorf("cli: duplicate ACP option ID %q across --acp-option and --acp-toggle", toggle.ID)
		}
		seen[toggle.ID] = struct{}{}
	}
	return append(options, toggles...), nil
}

func parseAgentACPOptionFlags(values []string) ([]contract.AgentACPOptionSelection, error) {
	if len(values) == 0 {
		return nil, nil
	}
	options := make([]contract.AgentACPOptionSelection, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		id, value, found := strings.Cut(strings.TrimSpace(raw), "=")
		id = strings.TrimSpace(id)
		value = strings.TrimSpace(value)
		if !found || id == "" || value == "" {
			return nil, fmt.Errorf(
				"cli: invalid --acp-option %q: expected id=value_id",
				raw,
			)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("cli: duplicate --acp-option ID %q", id)
		}
		seen[id] = struct{}{}
		options = append(options, contract.AgentACPOptionSelection{ID: id, ValueID: value})
	}
	return options, nil
}

func parseAgentACPToggleFlags(values []string) ([]contract.AgentACPOptionSelection, error) {
	if len(values) == 0 {
		return nil, nil
	}
	toggles := make([]contract.AgentACPOptionSelection, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		id, value, found := strings.Cut(strings.TrimSpace(raw), "=")
		id = strings.TrimSpace(id)
		value = strings.TrimSpace(value)
		if !found || id == "" || value == "" {
			return nil, fmt.Errorf("cli: invalid --acp-toggle %q: expected id=true|false", raw)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("cli: duplicate --acp-toggle ID %q", id)
		}
		seen[id] = struct{}{}
		if value != "true" && value != "false" {
			return nil, fmt.Errorf("cli: invalid --acp-toggle %q: expected id=true|false", raw)
		}
		parsed := value == "true"
		toggles = append(toggles, contract.AgentACPOptionSelection{ID: id, BoolValue: new(parsed)})
	}
	return toggles, nil
}

func cloneAgentACPOptions(options []contract.AgentACPOptionSelection) []contract.AgentACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]contract.AgentACPOptionSelection, len(options))
	for index, option := range options {
		cloned[index] = contract.AgentACPOptionSelection{ID: option.ID, ValueID: option.ValueID}
		if option.BoolValue != nil {
			cloned[index].BoolValue = new(*option.BoolValue)
		}
	}
	return cloned
}

func agentACPOptionsLabel(options []contract.AgentACPOptionSelection) string {
	parts := make([]string, 0, len(options))
	for _, option := range options {
		value := option.ValueID
		if option.BoolValue != nil {
			value = strconv.FormatBool(*option.BoolValue)
		}
		if id := strings.TrimSpace(option.ID); id != "" && value != "" {
			parts = append(parts, id+"="+value)
		}
	}
	return strings.Join(parts, ", ")
}
