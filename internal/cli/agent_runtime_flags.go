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
		return "", fmt.Errorf("cli: invalid --speed: %w", err)
	}
	return parsed, nil
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
