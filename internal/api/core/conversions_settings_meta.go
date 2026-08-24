package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsUserSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsUserSectionResponseMetaPayload {
	return contract.SettingsUserSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsUserScopeKind(envelope.Scope),
		AvailableScopes: settingsUserScopeKindsPayload(envelope.AvailableScopes),
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

func settingsLayeredSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsLayeredSectionResponseMetaPayload {
	return contract.SettingsLayeredSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsLayeredScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		Profile:         strings.TrimSpace(envelope.ProfileName),
		AvailableScopes: settingsWorkspaceScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsUserWorkspaceSectionMetaPayload(
	envelope settingspkg.SectionEnvelope,
) contract.SettingsWorkspaceSectionResponseMetaPayload {
	return contract.SettingsWorkspaceSectionResponseMetaPayload{
		Section:         contract.SettingsSectionName(envelope.Section),
		Scope:           contract.SettingsWorkspaceScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		AvailableScopes: settingsUserWorkspaceScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsUserCollectionMetaPayload(
	envelope settingspkg.CollectionEnvelope,
) contract.SettingsUserCollectionResponseMetaPayload {
	return contract.SettingsUserCollectionResponseMetaPayload{
		Collection:      contract.SettingsCollectionName(envelope.Collection),
		Scope:           contract.SettingsUserScopeKind(envelope.Scope),
		AvailableScopes: settingsUserScopeKindsPayload(envelope.AvailableScopes),
	}
}

func settingsLayeredCollectionMetaPayload(
	envelope settingspkg.CollectionEnvelope,
) contract.SettingsLayeredCollectionResponseMetaPayload {
	return contract.SettingsLayeredCollectionResponseMetaPayload{
		Collection:      contract.SettingsCollectionName(envelope.Collection),
		Scope:           contract.SettingsLayeredScopeKind(envelope.Scope),
		WorkspaceID:     strings.TrimSpace(envelope.WorkspaceID),
		Profile:         strings.TrimSpace(envelope.ProfileName),
		AvailableScopes: settingsWorkspaceScopeKindsPayload(envelope.AvailableScopes),
	}
}
