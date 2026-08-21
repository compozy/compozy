package settings

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func diffPersonaSettings(current compozyconfig.DefaultsConfig, desired compozyconfig.DefaultsConfig) []string {
	changed := make([]string, 0, 3)
	if current.Agent != desired.Agent {
		changed = append(changed, "defaults.agent")
	}
	if current.Provider != desired.Provider {
		changed = append(changed, "defaults.provider")
	}
	if current.Sandbox != desired.Sandbox {
		changed = append(changed, "defaults.sandbox")
	}
	return changed
}

func applyPersonaSettings(editor *compozyconfig.OverlayEditor, desired compozyconfig.DefaultsConfig) error {
	return applyValueUpdates(editor, []struct {
		path  []string
		value any
	}{
		{path: []string{sectionsDefaultsKey, "agent"}, value: desired.Agent},
		{path: []string{sectionsDefaultsKey, sectionsProviderKey}, value: desired.Provider},
		{path: []string{sectionsDefaultsKey, "sandbox"}, value: desired.Sandbox},
	})
}

func (s *service) updatePersonaSection(ctx context.Context, req SectionUpdateRequest) (MutationResult, error) {
	if req.Persona == nil {
		return MutationResult{}, validationError(errors.New("settings: persona section payload is required"))
	}
	desired := *req.Persona
	if err := desired.Validate(); err != nil {
		return MutationResult{}, validationError(err)
	}
	loaded, err := s.loadScopedSectionUpdateForProfile(
		ctx,
		req.Section,
		req.Scope,
		req.WorkspaceID,
		req.ProfileName,
		ScopeUser,
		ScopeProfile,
		ScopeWorkspace,
	)
	if err != nil {
		return MutationResult{}, err
	}
	changed := diffPersonaSettings(loaded.config.Defaults, desired)
	return s.updateScopedConfigSectionForProfile(
		req.Section,
		changed,
		loaded.target,
		loaded.scope,
		loaded.workspaceID,
		loaded.profileName,
		loaded.workspaceRoot,
		func(editor *compozyconfig.OverlayEditor) error { return applyPersonaSettings(editor, desired) },
	)
}
