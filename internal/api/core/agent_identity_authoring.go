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
