package contract

type SettingsSourceRefPayload struct {
	Kind        SettingsSourceKind `json:"kind"`
	Scope       SettingsScopeKind  `json:"scope"`
	WorkspaceID string             `json:"workspace_id,omitempty"`
	Profile     string             `json:"profile,omitempty"`
	AgentName   string             `json:"agent_name,omitempty"`
}

type SettingsSourceMetadataPayload struct {
	EffectiveSource  SettingsSourceRefPayload   `json:"effective_source"`
	ShadowedSources  []SettingsSourceRefPayload `json:"shadowed_sources,omitempty"`
	AvailableTargets []SettingsWriteTargetKind  `json:"available_targets"`
}
