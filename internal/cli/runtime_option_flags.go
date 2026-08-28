package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	runtimeACPOptionFlag = "acp-option"
	runtimeACPToggleFlag = "acp-toggle"
)

func bindACPOptionFlags(
	cmd *cobra.Command,
	optionValues *[]string,
	toggleValues *[]string,
	optionHelp string,
	toggleHelp string,
) {
	cmd.Flags().StringArrayVar(optionValues, runtimeACPOptionFlag, nil, optionHelp)
	cmd.Flags().StringArrayVar(toggleValues, runtimeACPToggleFlag, nil, toggleHelp)
}

func parseACPFlags(optionValues, toggleValues []string) ([]contract.AgentACPOptionSelection, error) {
	options, err := parseACPOptionFlags(optionValues)
	if err != nil {
		return nil, err
	}
	toggles, err := parseACPToggleFlags(toggleValues)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(options)+len(toggles))
	for _, option := range options {
		seen[option.ID] = struct{}{}
	}
	for _, toggle := range toggles {
		if _, exists := seen[toggle.ID]; exists {
			return nil, fmt.Errorf(
				"cli: duplicate ACP option ID %q across --%s and --%s",
				toggle.ID,
				runtimeACPOptionFlag,
				runtimeACPToggleFlag,
			)
		}
		seen[toggle.ID] = struct{}{}
	}
	return append(options, toggles...), nil
}

func parseACPOptionFlags(values []string) ([]contract.AgentACPOptionSelection, error) {
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
			return nil, fmt.Errorf("cli: invalid --%s %q: expected id=value_id", runtimeACPOptionFlag, raw)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("cli: duplicate --%s ID %q", runtimeACPOptionFlag, id)
		}
		seen[id] = struct{}{}
		options = append(options, contract.AgentACPOptionSelection{ID: id, ValueID: value})
	}
	return options, nil
}

func parseACPToggleFlags(values []string) ([]contract.AgentACPOptionSelection, error) {
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
			return nil, fmt.Errorf("cli: invalid --%s %q: expected id=true|false", runtimeACPToggleFlag, raw)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("cli: duplicate --%s ID %q", runtimeACPToggleFlag, id)
		}
		seen[id] = struct{}{}
		parsed, err := strconv.ParseBool(value)
		if err != nil || (value != "true" && value != "false") {
			return nil, fmt.Errorf("cli: invalid --%s %q: expected id=true|false", runtimeACPToggleFlag, raw)
		}
		toggles = append(toggles, contract.AgentACPOptionSelection{ID: id, BoolValue: new(parsed)})
	}
	return toggles, nil
}
