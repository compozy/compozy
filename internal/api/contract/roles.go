package contract

// RoleResolutionMode identifies how one configured background role finds its identity.
type RoleResolutionMode string

const (
	RoleResolutionModeBuiltin RoleResolutionMode = "builtin"
	RoleResolutionModeCatalog RoleResolutionMode = "catalog"
	RoleResolutionModeInherit RoleResolutionMode = "inherit"
)

// RoleFallbackStatus is one configured fallback route in declaration order.
type RoleFallbackStatus struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// RoleDiagnostic reports a current configuration-resolution problem without simulating an invocation.
type RoleDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Agent   string `json:"agent,omitempty"`
}

// RoleStatus is the effective configuration projection for one background role.
type RoleStatus struct {
	Role            string               `json:"role"`
	Enabled         bool                 `json:"enabled"`
	ResolutionMode  RoleResolutionMode   `json:"resolution_mode"`
	Agent           *string              `json:"agent"`
	Provider        *string              `json:"provider"`
	Model           *string              `json:"model"`
	ReasoningEffort *string              `json:"reasoning_effort"`
	Timeout         *string              `json:"timeout,omitempty"`
	FallbackChain   []RoleFallbackStatus `json:"fallback_chain"`
	Provenance      map[string]string    `json:"provenance"`
	Diagnostics     []RoleDiagnostic     `json:"diagnostics"`
}

// RolesResponse wraps the closed role roster.
type RolesResponse struct {
	Roles []RoleStatus `json:"roles"`
}

// RoleStatusResponse wraps one role projection.
type RoleStatusResponse struct {
	Role RoleStatus `json:"role"`
}
