package contract

type SettingsUserSectionResponseMetaPayload struct {
	Section         SettingsSectionName     `json:"section"`
	Scope           SettingsUserScopeKind   `json:"scope"`
	AvailableScopes []SettingsUserScopeKind `json:"available_scopes"`
}

type SettingsSkillsSectionResponseMetaPayload struct {
	Section         SettingsSectionName      `json:"section"`
	Scope           SettingsAgentScopeKind   `json:"scope"`
	WorkspaceID     string                   `json:"workspace_id,omitempty"`
	AgentName       string                   `json:"agent_name,omitempty"`
	AvailableScopes []SettingsAgentScopeKind `json:"available_scopes"`
}

type SettingsLayeredSectionResponseMetaPayload struct {
	Section         SettingsSectionName        `json:"section"`
	Scope           SettingsLayeredScopeKind   `json:"scope"`
	WorkspaceID     string                     `json:"workspace_id,omitempty"`
	Profile         string                     `json:"profile,omitempty"`
	AvailableScopes []SettingsLayeredScopeKind `json:"available_scopes"`
}

type SettingsWorkspaceSectionResponseMetaPayload struct {
	Section         SettingsSectionName          `json:"section"`
	Scope           SettingsWorkspaceScopeKind   `json:"scope"`
	WorkspaceID     string                       `json:"workspace_id,omitempty"`
	AvailableScopes []SettingsWorkspaceScopeKind `json:"available_scopes"`
}

type SettingsUserCollectionResponseMetaPayload struct {
	Collection      SettingsCollectionName  `json:"collection"`
	Scope           SettingsUserScopeKind   `json:"scope"`
	AvailableScopes []SettingsUserScopeKind `json:"available_scopes"`
}

type SettingsLayeredCollectionResponseMetaPayload struct {
	Collection      SettingsCollectionName     `json:"collection"`
	Scope           SettingsLayeredScopeKind   `json:"scope"`
	WorkspaceID     string                     `json:"workspace_id,omitempty"`
	Profile         string                     `json:"profile,omitempty"`
	AvailableScopes []SettingsLayeredScopeKind `json:"available_scopes"`
}
