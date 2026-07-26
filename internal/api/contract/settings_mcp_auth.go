package contract

import "time"

// SettingsMCPAuthBeginMode selects callback delivery for one OAuth session.
type SettingsMCPAuthBeginMode string

const (
	// SettingsMCPAuthBeginModeAutomatic completes through the daemon loopback callback.
	SettingsMCPAuthBeginModeAutomatic SettingsMCPAuthBeginMode = "automatic"
	// SettingsMCPAuthBeginModeManual returns a copy/paste callback without requiring a listener.
	SettingsMCPAuthBeginModeManual SettingsMCPAuthBeginMode = "manual"
)

// SettingsMCPAuthBeginRequest selects automatic or manual callback delivery.
type SettingsMCPAuthBeginRequest struct {
	Mode SettingsMCPAuthBeginMode `json:"mode"`
}

// SettingsMCPAuthBeginResponse returns the verifier-free PKCE handoff.
type SettingsMCPAuthBeginResponse struct {
	AuthorizationURL string    `json:"authorization_url"`
	State            string    `json:"state"`
	ExpiresAt        time.Time `json:"expires_at"`
	CallbackURL      string    `json:"callback_url"`
	ManualSupported  bool      `json:"manual_supported"`
}

// SettingsMCPAuthExchangeRequest completes one login with exactly one input.
type SettingsMCPAuthExchangeRequest struct {
	Code        string `json:"code,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
}
