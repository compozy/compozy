package contract

// SettingsAttentionPayload is the stable operator attention settings contract.
type SettingsAttentionPayload struct {
	Toasts          bool     `json:"toasts"`
	Sound           bool     `json:"sound"`
	System          bool     `json:"system"`
	MutedWorkspaces []string `json:"muted_workspaces"`
}

// UpdateSettingsAttentionPayload updates delivery channels and optionally replaces one profile's mutes.
type UpdateSettingsAttentionPayload struct {
	Toasts          bool      `json:"toasts"`
	Sound           bool      `json:"sound"`
	System          bool      `json:"system"`
	MutedWorkspaces *[]string `json:"muted_workspaces,omitempty"`
}

// UpdateSettingsAttentionRequest updates the attention section for the selected profile.
type UpdateSettingsAttentionRequest struct {
	Config UpdateSettingsAttentionPayload `json:"config"`
}

// SettingsAttentionResponse returns global delivery channels plus the selected profile's mutes.
type SettingsAttentionResponse struct {
	SettingsLayeredSectionResponseMetaPayload
	Config SettingsAttentionPayload `json:"config"`
}
