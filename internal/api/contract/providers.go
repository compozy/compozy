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

// ProviderLoginSource identifies the safe source of configured login metadata.
type ProviderLoginSource string

const (
	ProviderLoginSourceAuthLoginCommand ProviderLoginSource = "auth_login_command"
)

// ProviderLoginPresence reports only the availability truth established by the current probe.
type ProviderLoginPresence string

const (
	ProviderLoginPresencePresent ProviderLoginPresence = "present"
	ProviderLoginPresenceMissing ProviderLoginPresence = "missing"
	ProviderLoginPresenceUnknown ProviderLoginPresence = "unknown"
)

// ProviderRecommendedAction is the safe agent-facing recovery action.
type ProviderRecommendedAction string

const (
	ProviderRecommendedActionNone       ProviderRecommendedAction = ""
	ProviderRecommendedActionInstallCLI ProviderRecommendedAction = "install_cli"
	ProviderRecommendedActionLogin      ProviderRecommendedAction = "login"
	ProviderRecommendedActionBindSecret ProviderRecommendedAction = "bind_secret"
	ProviderRecommendedActionRetry      ProviderRecommendedAction = "retry"
	ProviderRecommendedActionInspect    ProviderRecommendedAction = "inspect"
	ProviderRecommendedActionNoRetry    ProviderRecommendedAction = "no_retry"
)

// ProviderLoginDescriptorPayload is the only public projection of auth_login_command.
type ProviderLoginDescriptorPayload struct {
	Configured        bool                      `json:"configured"`
	Source            ProviderLoginSource       `json:"source,omitempty"`
	Executable        string                    `json:"executable,omitempty"`
	Presence          ProviderLoginPresence     `json:"presence"`
	RecommendedAction ProviderRecommendedAction `json:"recommended_action,omitempty"`
}

// ProviderListResponse is the canonical provider inventory payload.
type ProviderListResponse struct {
	Providers []ProviderSummaryPayload `json:"providers"`
}

// ProviderSummaryPayload describes one canonical provider and its declared auth state.
type ProviderSummaryPayload struct {
	Name            string                    `json:"name"`
	DisplayName     string                    `json:"display_name,omitempty"`
	RuntimeStrategy ProviderRuntimeStrategy   `json:"runtime_strategy,omitempty"`
	Default         bool                      `json:"default"`
	AuthStatus      ProviderAuthStatusPayload `json:"auth_status"`
}

// ProviderAuthStatusPayload is the shared provider-auth readiness payload.
type ProviderAuthStatusPayload struct {
	Mode        string                         `json:"mode"`
	EnvPolicy   string                         `json:"env_policy"`
	HomePolicy  string                         `json:"home_policy"`
	State       string                         `json:"state"`
	Code        string                         `json:"code,omitempty"`
	Message     string                         `json:"message,omitempty"`
	StatusCmd   string                         `json:"status_command,omitempty"`
	Login       ProviderLoginDescriptorPayload `json:"login"`
	LastProbeAt *time.Time                     `json:"last_probe_at,omitempty"`
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
