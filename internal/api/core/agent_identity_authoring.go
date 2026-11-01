package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func applyDuplicateAgentIdentity(
	draft *compozyconfig.AgentDefinitionDraft,
	overrides *contract.DuplicateAgentOverrides,
) error {
	if overrides.DisplayName != nil {
		draft.DisplayName = *overrides.DisplayName
	}
	if overrides.Description != nil {
		draft.Description = *overrides.Description
	}
	if overrides.Kind != "" {
		draft.Kind = overrides.Kind
	}
	if overrides.Appearance != nil {
		if err := compozyconfig.ValidateAgentAppearance(overrides.Appearance); err != nil {
			return errors.Join(compozyconfig.ErrInvalidAgentDefinition, err)
		}
		draft.Appearance = compozyconfig.CloneAgentAppearance(overrides.Appearance)
	}

	return nil
}

func duplicateAgentDraft(
	source compozyconfig.AgentDef,
	req contract.DuplicateAgentRequest,
) (compozyconfig.AgentDefinitionDraft, error) {
	draft, err := authoredAgentDraftFromDef(source)
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, err
	}
	draft.Name = compozyconfig.NormalizeAgentName(req.Name)
	label := []rune(source.Label())
	draft.DisplayName = string(label[:min(len(label), 75)]) + " copy"
	if req.Kind != "" {
		draft.Kind = req.Kind
	}
	if req.Overrides == nil {
		return draft, nil
	}
	overrides := req.Overrides
	if err := applyDuplicateAgentIdentity(&draft, overrides); err != nil {
		return compozyconfig.AgentDefinitionDraft{}, err
	}
	if overrides.SessionEnv != nil {
		draft.SessionEnv = sessionEnvFromPayload(overrides.SessionEnv)
	}
	applyDuplicateAgentRuntime(&draft, overrides)
	if len(overrides.Tools) > 0 {
		draft.Tools = append([]string(nil), overrides.Tools...)
	}
	if len(overrides.Toolsets) > 0 {
		draft.Toolsets = append([]string(nil), overrides.Toolsets...)
	}
	if len(overrides.DenyTools) > 0 {
		draft.DenyTools = append([]string(nil), overrides.DenyTools...)
	}
	if overrides.Permissions != "" {
		draft.Permissions = string(overrides.Permissions)
	}
	if len(overrides.CategoryPath) > 0 {
		draft.CategoryPath = append([]string(nil), overrides.CategoryPath...)
	}
	if overrides.Skills != nil {
		draft.Skills.Disabled = append([]string(nil), overrides.Skills.Disabled...)
	}
	if overrides.Prompt != "" {
		draft.Prompt = overrides.Prompt
	}
	return draft, nil
}

func applyDuplicateAgentRuntime(draft *compozyconfig.AgentDefinitionDraft, overrides *contract.DuplicateAgentOverrides) {
	if overrides.Provider != "" {
		draft.Provider = overrides.Provider
	}
	if overrides.Command != "" {
		draft.Command = overrides.Command
	}
	if overrides.Model != "" {
		draft.Model = overrides.Model
	}
	if overrides.ReasoningEffort != "" {
		draft.ReasoningEffort = string(overrides.ReasoningEffort)
	}
	if overrides.ClearSpeed {
		draft.Speed = ""
	} else if overrides.Speed != "" {
		draft.Speed = overrides.Speed
	}
	if overrides.ClearACPOptions {
		draft.ACPOptions = nil
	} else if len(overrides.ACPOptions) > 0 {
		draft.ACPOptions = agentACPOptionsFromContract(overrides.ACPOptions)
	}
}
