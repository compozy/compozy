package contract

// SetExtensionEnablementRequest changes one profile-specific exception state.
type SetExtensionEnablementRequest struct {
	Profile string `json:"profile"`
	Enabled bool   `json:"enabled"`
}

// ExtensionEnablementPayload is the effective state for one named profile.
type ExtensionEnablementPayload struct {
	Profile string `json:"profile"`
	Enabled bool   `json:"enabled"`
}
