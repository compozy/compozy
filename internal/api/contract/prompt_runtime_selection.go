package contract

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
)

// PromptRuntimeSelectionFromPayload converts an optional prompt runtime snapshot.
func PromptRuntimeSelectionFromPayload(
	payload *PromptRuntimeSelectionPayload,
) *session.RuntimeSelection {
	if payload == nil {
		return nil
	}
	return &session.RuntimeSelection{
		Provider:        strings.TrimSpace(payload.Provider),
		Model:           strings.TrimSpace(payload.Model),
		ReasoningEffort: strings.TrimSpace(string(payload.ReasoningEffort)),
		Speed:           payload.Speed,
		ACPOptions:      acpOptionSelectionsFromPayload(payload.ACPOptions),
	}
}

// PromptRuntimeSelectionPayloadFromSelection converts an optional runtime selection into its wire shape.
func PromptRuntimeSelectionPayloadFromSelection(
	selection *session.RuntimeSelection,
) *PromptRuntimeSelectionPayload {
	if selection == nil {
		return nil
	}
	return &PromptRuntimeSelectionPayload{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: ReasoningEffort(strings.TrimSpace(selection.ReasoningEffort)),
		Speed:           selection.Speed,
		ACPOptions:      acpOptionSelectionPayloadsFromACP(selection.ACPOptions),
	}
}

// ACPOptionSelectionsFromPayload converts public typed ACP options into the
// internal selection shape without exposing ACP wire fields.
func ACPOptionSelectionsFromPayload(
	options []AgentACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	return acpOptionSelectionsFromPayload(options)
}

func acpOptionSelectionsFromPayload(
	options []AgentACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]acp.SessionConfigOptionSelection, 0, len(options))
	for _, option := range options {
		selection := acp.SessionConfigOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			selection.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, selection)
	}
	return converted
}

// ACPOptionSelectionPayloadsFromACP converts internal typed ACP options into
// their provider-neutral public representation.
func ACPOptionSelectionPayloadsFromACP(
	options []acp.SessionConfigOptionSelection,
) []AgentACPOptionSelection {
	return acpOptionSelectionPayloadsFromACP(options)
}

func acpOptionSelectionPayloadsFromACP(
	options []acp.SessionConfigOptionSelection,
) []AgentACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]AgentACPOptionSelection, 0, len(options))
	for _, option := range options {
		selection := AgentACPOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			selection.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, selection)
	}
	return converted
}
