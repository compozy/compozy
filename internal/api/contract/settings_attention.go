package contract

// SettingsAttentionPayload is the stable operator attention settings contract.
type SettingsAttentionPayload struct {
	Toasts          bool     `json:"toasts"`
	Sound           bool     `json:"sound"`
	System          bool     `json:"system"`
	MutedWorkspaces []string `json:"muted_workspaces"`
}

// UpdateSettingsAttentionRequest replaces the complete attention section.
type UpdateSettingsAttentionRequest struct {
	Config SettingsAttentionPayload `json:"config"`
}

// SettingsAttentionResponse returns the effective global attention settings.
type SettingsAttentionResponse struct {
	SettingsUserSectionResponseMetaPayload
	Config SettingsAttentionPayload `json:"config"`
}
