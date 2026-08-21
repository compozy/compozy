package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsGlobalSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsGlobalSectionResponseMetaPayload {
	return contract.SettingsGlobalSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsGlobalScopeKind(envelope.Scope),
		AvailableScopes: settingsGlobalScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsSkillsSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsSkillsSectionResponseMetaPayload {
	return contract.SettingsSkillsSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsAgentScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		AgentName:       strings.TrimSpace(envelope.AgentName),
		AvailableScopes: settingsAgentScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsGlobalWorkspaceSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsGlobalWorkspaceSectionResponseMetaPayload {
	return contract.SettingsGlobalWorkspaceSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsWorkspaceScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		AvailableScopes: settingsWorkspaceScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsGlobalCollectionMetaPayload(
	envelope settingspkg.CollectionEnvelope,
) contract.SettingsGlobalCollectionResponseMetaPayload {
	return contract.SettingsGlobalCollectionResponseMetaPayload{
		Collection:      contract.SettingsCollectionName(envelope.Collection),
		Scope:           contract.SettingsGlobalScopeKind(envelope.Scope),
		AvailableScopes: settingsGlobalScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsGlobalWorkspaceCollectionMetaPayload(
	envelope settingspkg.CollectionEnvelope,
) contract.SettingsGlobalWorkspaceCollectionResponseMetaPayload {
	return contract.SettingsGlobalWorkspaceCollectionResponseMetaPayload{
		Collection:      contract.SettingsCollectionName(envelope.Collection),
		Scope:           contract.SettingsWorkspaceScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		AvailableScopes: settingsWorkspaceScopeKindsPayload(envelope.AvailableScopes),
	}
}
