package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/deadentity"
	"github.com/compozy/agh/internal/diagnostics"
	mcppkg "github.com/compozy/agh/internal/mcp"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const defaultSettingsMCPProbeTimeout = 5 * time.Second

func (s *settingsRuntimeSurface) MCPServerRuntimeStatus(
	ctx context.Context,
	target mcpauth.Target,
	server aghconfig.MCPServer,
) (settingspkg.MCPServerRuntimeStatus, error) {
	status := settingspkg.MCPServerRuntimeStatus{Configured: true}
	key, tracked := settingsMCPDeadEntityKey(target, server.Name)
	if suppressed, err := s.suppressedMCPRuntimeStatus(ctx, key, tracked); err != nil {
		return settingspkg.MCPServerRuntimeStatus{}, err
	} else if suppressed != nil {
		return *suppressed, nil
	}

	if err := server.Validate("mcp_server"); err != nil {
		status.State = settingspkg.MCPServerRuntimeStateConfigError
		status.Probe = settingspkg.MCPServerProbeSkipped
		status.Reason = "config_error"
		status.Diagnostic = diagnostics.Redact(err.Error())
		return s.recordMCPProbeFailure(ctx, key, tracked, deadentity.FailurePermanent, status)
	}
	if server.Auth.Enabled() {
		authStatus, err := s.MCPAuthStatus(ctx, target, server)
		if err != nil {
			status.State = settingspkg.MCPServerRuntimeStateAuthRequired
			status.Probe = settingspkg.MCPServerProbeSkipped
			status.Reason = string(toolspkg.ReasonMCPAuthRequired)
			status.Diagnostic = diagnostics.Redact(err.Error())
			return s.recordMCPProbeFailure(ctx, key, tracked, deadentity.FailurePermanent, status)
		}
		if mapped, ok := runtimeStateFromMCPAuthStatus(authStatus); ok {
			status.State = mapped
			status.Probe = settingspkg.MCPServerProbeSkipped
			status.Reason = runtimeReasonFromMCPAuthState(mapped)
			status.Diagnostic = diagnostics.Redact(authStatus.Diagnostic)
			return s.recordMCPProbeFailure(ctx, key, tracked, deadentity.FailurePermanent, status)
		}
	}

	executor, err := s.newMCPProbeExecutor(target, server)
	if err != nil {
		status = runtimeStatusFromMCPProbeError(err)
		return s.recordMCPProbeFailure(ctx, key, tracked, deadentity.FailurePermanent, status)
	}

	tools, err := executor.ListTools(ctx, toolspkg.SourceRef{
		Kind:          toolspkg.SourceMCP,
		Owner:         strings.TrimSpace(server.Name),
		RawServerName: strings.TrimSpace(server.Name),
		RawToolName:   "*",
		WorkspaceID:   strings.TrimSpace(target.WorkspaceID),
	})
	if err != nil {
		status = runtimeStatusFromMCPProbeError(err)
		return s.recordMCPProbeFailure(
			ctx,
			key,
			tracked,
			toolspkg.MCPDiscoveryFailureClass(err),
			status,
		)
	}
	if tracked && s.deadEntities != nil {
		if err := s.deadEntities.RecordSuccess(ctx, key); err != nil {
			return settingspkg.MCPServerRuntimeStatus{}, fmt.Errorf("daemon: record MCP runtime recovery: %w", err)
		}
	}
	return settingspkg.MCPServerRuntimeStatus{
		Configured:  true,
		Initialized: true,
		State:       settingspkg.MCPServerRuntimeStateReady,
		Probe:       settingspkg.MCPServerProbeSucceeded,
		ToolCount:   len(tools),
	}, nil
}

func (s *settingsRuntimeSurface) newMCPProbeExecutor(
	target mcpauth.Target,
	server aghconfig.MCPServer,
) (*mcppkg.CallExecutor, error) {
	return mcppkg.NewMCPCallExecutor(
		mcppkg.ServerResolverFunc(func(
			context.Context,
			toolspkg.SourceRef,
		) (mcppkg.ResolvedServer, error) {
			return mcppkg.ResolvedServer{Server: server, Target: target}, nil
		}),
		mcppkg.WithTokenStore(s.mcpAuthStore),
		mcppkg.WithAuthMutationGeneration(s.mcpAuthGeneration),
		mcppkg.WithSecretLookup(s.lookupSecret),
		mcppkg.WithSecretResolver(s.secretRefs),
		mcppkg.WithTimeout(s.mcpProbeTimeout()),
	)
}

func (s *settingsRuntimeSurface) suppressedMCPRuntimeStatus(
	ctx context.Context,
	key store.DeadEntityKey,
	tracked bool,
) (*settingspkg.MCPServerRuntimeStatus, error) {
	if !tracked || s.deadEntities == nil {
		return nil, nil
	}
	decision, err := s.deadEntities.BeforeProbe(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("daemon: admit MCP runtime probe: %w", err)
	}
	if decision.Allowed {
		return nil, nil
	}
	status := deadMCPRuntimeStatus(decision.Reason)
	return &status, nil
}

func (s *settingsRuntimeSurface) recordMCPProbeFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	tracked bool,
	class deadentity.FailureClass,
	status settingspkg.MCPServerRuntimeStatus,
) (settingspkg.MCPServerRuntimeStatus, error) {
	if !tracked || s.deadEntities == nil {
		return status, nil
	}
	reason := strings.TrimSpace(status.Diagnostic)
	if reason == "" {
		reason = strings.TrimSpace(status.Reason)
	}
	if err := s.deadEntities.RecordFailure(ctx, key, class, reason); err != nil {
		return settingspkg.MCPServerRuntimeStatus{}, fmt.Errorf("daemon: record MCP runtime failure: %w", err)
	}
	deadStatus, err := s.deadEntities.Status(ctx, key)
	if err != nil {
		return settingspkg.MCPServerRuntimeStatus{}, fmt.Errorf("daemon: read MCP dead status: %w", err)
	}
	if deadStatus.Dead {
		return deadMCPRuntimeStatus(deadStatus.Reason), nil
	}
	return status, nil
}

func deadMCPRuntimeStatus(reason string) settingspkg.MCPServerRuntimeStatus {
	return settingspkg.MCPServerRuntimeStatus{
		Configured: true,
		State:      settingspkg.MCPServerRuntimeStateDead,
		Probe:      settingspkg.MCPServerProbeSkipped,
		Reason:     string(toolspkg.ReasonBackendDead),
		Diagnostic: diagnostics.Redact(reason),
	}
}

func settingsMCPDeadEntityKey(target mcpauth.Target, serverName string) (store.DeadEntityKey, bool) {
	normalizedTarget := target.Normalize()
	if normalizedTarget.Scope != mcpauth.ScopeWorkspace {
		return store.DeadEntityKey{}, false
	}
	key := store.DeadEntityKey{
		WorkspaceID: normalizedTarget.WorkspaceID,
		Kind:        store.DeadEntityKindMCPSidecar,
		EntityID:    serverName,
	}.Normalize()
	return key, key.Validate() == nil
}

func runtimeStateFromMCPAuthStatus(
	status mcpauth.Status,
) (settingspkg.MCPServerRuntimeState, bool) {
	switch status.Status {
	case mcpauth.StatusNeedsLogin:
		return settingspkg.MCPServerRuntimeStateAuthRequired, true
	case mcpauth.StatusExpired:
		return settingspkg.MCPServerRuntimeStateAuthExpired, true
	case mcpauth.StatusInvalid:
		return settingspkg.MCPServerRuntimeStateAuthInvalid, true
	case mcpauth.StatusAuthenticated:
		return "", false
	default:
		if strings.TrimSpace(string(status.Status)) == "refresh_failed" {
			return settingspkg.MCPServerRuntimeStateAuthRefreshFailed, true
		}
		return settingspkg.MCPServerRuntimeStateAuthRequired, true
	}
}

func runtimeReasonFromMCPAuthState(state settingspkg.MCPServerRuntimeState) string {
	switch state {
	case settingspkg.MCPServerRuntimeStateAuthExpired:
		return string(toolspkg.ReasonMCPAuthExpired)
	case settingspkg.MCPServerRuntimeStateAuthInvalid:
		return string(toolspkg.ReasonMCPAuthInvalid)
	case settingspkg.MCPServerRuntimeStateAuthRefreshFailed:
		return string(toolspkg.ReasonMCPAuthRefreshFailed)
	default:
		return string(toolspkg.ReasonMCPAuthRequired)
	}
}

func runtimeStatusFromMCPProbeError(err error) settingspkg.MCPServerRuntimeStatus {
	status := settingspkg.MCPServerRuntimeStatus{
		Configured: true,
		State:      settingspkg.MCPServerRuntimeStateRuntimeUnavailable,
		Probe:      settingspkg.MCPServerProbeFailed,
		Reason:     string(toolspkg.ReasonMCPUnreachable),
		Diagnostic: diagnostics.Redact(err.Error()),
	}
	if errors.Is(err, os.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		status.State = settingspkg.MCPServerRuntimeStatePermissionDenied
		status.Reason = "permission_denied"
		return status
	}
	reason, ok := toolspkg.ReasonOf(err)
	if !ok {
		return status
	}
	status.Reason = string(reason)
	switch reason {
	case toolspkg.ReasonMCPAuthRequired, toolspkg.ReasonMCPAuthUnconfigured:
		status.State = settingspkg.MCPServerRuntimeStateAuthRequired
		status.Probe = settingspkg.MCPServerProbeSkipped
	case toolspkg.ReasonMCPAuthExpired:
		status.State = settingspkg.MCPServerRuntimeStateAuthExpired
		status.Probe = settingspkg.MCPServerProbeSkipped
	case toolspkg.ReasonMCPAuthInvalid:
		status.State = settingspkg.MCPServerRuntimeStateAuthInvalid
		status.Probe = settingspkg.MCPServerProbeSkipped
	case toolspkg.ReasonMCPAuthRefreshFailed:
		status.State = settingspkg.MCPServerRuntimeStateAuthRefreshFailed
		status.Probe = settingspkg.MCPServerProbeSkipped
	case toolspkg.ReasonPolicyDenied:
		status.State = settingspkg.MCPServerRuntimeStatePermissionDenied
	case toolspkg.ReasonSchemaInvalid:
		status.State = settingspkg.MCPServerRuntimeStateConfigError
		status.Probe = settingspkg.MCPServerProbeSkipped
	default:
		status.State = settingspkg.MCPServerRuntimeStateRuntimeUnavailable
	}
	return status
}
