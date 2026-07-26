package contract

import "time"

const (
	ProviderAuthStateAuthenticated     = "authenticated"
	ProviderAuthStateNeedsLogin        = "needs_login"
	ProviderAuthStateMissingCLI        = "missing_cli"
	ProviderAuthStateMissingCredential = "missing_credential"
	ProviderAuthStatePermissionDenied  = "permission_denied"
	ProviderAuthStateRateLimited       = "rate_limited"
	ProviderAuthStateTransient         = "transient"
	ProviderAuthStateNone              = "none"
	ProviderAuthStateUnknown           = "unknown"
)

// ProviderListResponse is the canonical provider inventory payload.
type ProviderListResponse struct {
	Providers []ProviderSummaryPayload `json:"providers"`
}

// ProviderSummaryPayload describes one canonical provider and its declared auth state.
type ProviderSummaryPayload struct {
	Name        string                    `json:"name"`
	DisplayName string                    `json:"display_name,omitempty"`
	Default     bool                      `json:"default"`
	AuthStatus  ProviderAuthStatusPayload `json:"auth_status"`
}

// ProviderAuthStatusPayload is the shared provider-auth readiness payload.
type ProviderAuthStatusPayload struct {
	Mode        string     `json:"mode"`
	EnvPolicy   string     `json:"env_policy"`
	HomePolicy  string     `json:"home_policy"`
	State       string     `json:"state"`
	Code        string     `json:"code,omitempty"`
	Message     string     `json:"message,omitempty"`
	StatusCmd   string     `json:"status_command,omitempty"`
	LoginCmd    string     `json:"login_command,omitempty"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
}

// ProviderAuthProbeResponse reports one live provider-auth probe result.
type ProviderAuthProbeResponse struct {
	Provider   string                    `json:"provider"`
	AuthStatus ProviderAuthStatusPayload `json:"auth_status"`
	Probe      *ProviderAuthProbeResult  `json:"probe,omitempty"`
}

// ProviderAuthProbeResult is the redacted raw output from a provider auth probe.
type ProviderAuthProbeResult struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}
