package settings

import mcpauth "github.com/compozy/agh/internal/mcp/auth"

// MCPSecretValues is the write-only secret material submitted with an MCP server mutation.
type MCPSecretValues struct {
	SecretEnv         map[string]string
	OAuthClientSecret *string
}

// MCPSecretPreservation identifies existing bindings to retain without revealing their refs.
type MCPSecretPreservation struct {
	SecretEnv         []string
	OAuthClientSecret bool
}

func (v MCPSecretValues) Empty() bool {
	return len(v.SecretEnv) == 0 && v.OAuthClientSecret == nil
}

// MCPAuthStatus is a redacted remote MCP authentication status.
type MCPAuthStatus = mcpauth.Status

// MCPServerRuntimeState reports the daemon-observed MCP server runtime state.
type MCPServerRuntimeState string

const (
	// MCPServerRuntimeStateReady reports a server that initialized and listed tools.
	MCPServerRuntimeStateReady MCPServerRuntimeState = "ready"
	// MCPServerRuntimeStateConfigError reports a malformed server definition.
	MCPServerRuntimeStateConfigError MCPServerRuntimeState = "config_error"
	// MCPServerRuntimeStateAuthRequired reports a remote server requiring login.
	MCPServerRuntimeStateAuthRequired MCPServerRuntimeState = "auth_required"
	// MCPServerRuntimeStateAuthExpired reports an expired remote auth token.
	MCPServerRuntimeStateAuthExpired MCPServerRuntimeState = "auth_expired"
	// MCPServerRuntimeStateAuthInvalid reports an invalid remote auth token.
	MCPServerRuntimeStateAuthInvalid MCPServerRuntimeState = "auth_invalid"
	// MCPServerRuntimeStateAuthRefreshFailed reports a failed auth refresh.
	MCPServerRuntimeStateAuthRefreshFailed MCPServerRuntimeState = "auth_refresh_failed"
	// MCPServerRuntimeStatePermissionDenied reports a permission failure while probing.
	MCPServerRuntimeStatePermissionDenied MCPServerRuntimeState = "permission_denied"
	// MCPServerRuntimeStateRuntimeUnavailable reports a configured server that could not be reached.
	MCPServerRuntimeStateRuntimeUnavailable MCPServerRuntimeState = "runtime_unavailable"
	// MCPServerRuntimeStateDead reports a durably suppressed server awaiting recovery.
	MCPServerRuntimeStateDead MCPServerRuntimeState = "dead"
)

// MCPServerProbeState reports whether the runtime probe actually touched the server.
type MCPServerProbeState string

const (
	// MCPServerProbeSkipped reports a real preflight state that prevented probing.
	MCPServerProbeSkipped MCPServerProbeState = "skipped"
	// MCPServerProbeSucceeded reports a successful MCP initialize/list-tools probe.
	MCPServerProbeSucceeded MCPServerProbeState = "succeeded"
	// MCPServerProbeFailed reports a failed MCP initialize/list-tools probe.
	MCPServerProbeFailed MCPServerProbeState = "failed"
)

// MCPServerRuntimeStatus is one daemon-backed MCP server probe result.
type MCPServerRuntimeStatus struct {
	Configured  bool
	Initialized bool
	State       MCPServerRuntimeState
	Probe       MCPServerProbeState
	ToolCount   int
	Reason      string
	Diagnostic  string
}
