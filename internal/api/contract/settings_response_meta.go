package contract

type SettingsGlobalSectionResponseMetaPayload struct {
	Section         SettingsSectionName       `json:"section"`
	Scope           SettingsGlobalScopeKind   `json:"scope"`
	AvailableScopes []SettingsGlobalScopeKind `json:"available_scopes"`
}

type SettingsSkillsSectionResponseMetaPayload struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsAgentScopeKind   `json:"scope"`
	WorkspaceID     string                   `json:"workspace_id,omitempty"`
	AgentName       string                   `json:"agent_name,omitempty"`
	AvailableScopes []SettingsAgentScopeKind `json:"available_scopes"`
}

type SettingsGlobalWorkspaceSectionResponseMetaPayload struct {
	Section         SettingsSectionName          `json:"section"`
	Scope           SettingsWorkspaceScopeKind   `json:"scope"`
	WorkspaceID     string                       `json:"workspace_id,omitempty"`
	AvailableScopes []SettingsWorkspaceScopeKind `json:"available_scopes"`
}

type SettingsGlobalCollectionResponseMetaPayload struct {
	Collection      SettingsCollectionName    `json:"collection"`
	Scope           SettingsGlobalScopeKind   `json:"scope"`
	AvailableScopes []SettingsGlobalScopeKind `json:"available_scopes"`
}

type SettingsGlobalWorkspaceCollectionResponseMetaPayload struct {
	Collection      SettingsCollectionName       `json:"collection"`
	Scope           SettingsWorkspaceScopeKind   `json:"scope"`
	WorkspaceID     string                       `json:"workspace_id,omitempty"`
	AvailableScopes []SettingsWorkspaceScopeKind `json:"available_scopes"`
}
