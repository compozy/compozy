package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	"time"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/store"
)

func (h *HostAPIHandler) handleSandboxList(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}

	var params hostAPISandboxListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	filterWorkspaceID, filterWorkspaceRoot, err := h.resolveSandboxWorkspaceFilter(ctx, params.Workspace)
	if err != nil {
		return nil, err
	}

	infos, err := h.sessions.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := hostAPISandboxListResult{
		Sandboxes: make([]hostAPISandboxSummary, 0, len(infos)),
	}
	for _, info := range infos {
		if info == nil || info.Sandbox == nil || info.State == session.StateStopped {
			continue
		}
		if filterWorkspaceID != "" || filterWorkspaceRoot != "" {
			if info.WorkspaceID != filterWorkspaceID && info.Workspace != filterWorkspaceRoot {
				continue
			}
		}
		result.Sandboxes = append(result.Sandboxes, hostAPISandboxSummary{
			SessionID:  info.ID,
			SandboxID:  strings.TrimSpace(info.Sandbox.SandboxID),
			Backend:    strings.TrimSpace(info.Sandbox.Backend),
			Profile:    strings.TrimSpace(info.Sandbox.Profile),
			InstanceID: strings.TrimSpace(info.Sandbox.InstanceID),
			State:      strings.TrimSpace(info.Sandbox.State),
			SyncState:  hostAPISandboxSyncState(info.Sandbox),
		})
	}
	return result, nil
}

func (h *HostAPIHandler) handleSandboxInfo(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	var params hostAPISandboxInfoParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	info, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	if info == nil || info.Sandbox == nil {
		return nil, notFoundRPCError("sandbox", sessionID, errors.New("sandbox is not configured"))
	}
	return hostAPISandboxInfoResult{
		SandboxID:     strings.TrimSpace(info.Sandbox.SandboxID),
		Backend:       strings.TrimSpace(info.Sandbox.Backend),
		Profile:       strings.TrimSpace(info.Sandbox.Profile),
		InstanceID:    strings.TrimSpace(info.Sandbox.InstanceID),
		RuntimeRoot:   strings.TrimSpace(info.Sandbox.RuntimeRootDir),
		SyncState:     hostAPISandboxSyncState(info.Sandbox),
		CreatedAt:     info.CreatedAt,
		LastSyncError: strings.TrimSpace(info.Sandbox.LastSyncError),
	}, nil
}

func (h *HostAPIHandler) handleSandboxExec(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	var params hostAPISandboxExecParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	command := strings.TrimSpace(params.Command)
	if command == "" {
		return nil, invalidParamsRPCError(errors.New("command is required"))
	}
	if params.Timeout < 0 {
		return nil, invalidParamsRPCError(errors.New("timeout must be non-negative"))
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, sessionID); err != nil {
		return nil, err
	}
	result, err := h.sessions.ExecSandbox(ctx, session.SandboxExecRequest{
		SessionID: sessionID,
		Command:   command,
		Timeout:   time.Duration(params.Timeout) * time.Second,
	})
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil, notFoundRPCError("session", sessionID, err)
		}
		if errors.Is(err, session.ErrSessionNotActive) {
			return nil, unavailableRPCError(err)
		}
		return nil, err
	}
	return hostAPISandboxExecResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, nil
}

func (h *HostAPIHandler) resolveSandboxWorkspaceFilter(
	ctx context.Context,
	workspace string,
) (string, string, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return "", "", nil
	}
	if h.workspaces == nil {
		return workspace, workspace, nil
	}
	resolved, err := h.workspaces.Resolve(ctx, workspace)
	if err != nil {
		return "", "", err
	}
	workspaceID, err := hostAPIResolvedWorkspaceRegistrationID(&resolved)
	if err != nil {
		return "", "", err
	}
	return workspaceID, strings.TrimSpace(resolved.RootDir), nil
}

func hostAPISandboxSyncState(meta *store.SessionSandboxMeta) string {
	if meta == nil {
		return ""
	}
	if strings.TrimSpace(meta.LastSyncError) != "" {
		return extensionStateError
	}
	if meta.LastSyncAt != nil {
		return hostAPISandboxStateSynced
	}
	return hostAPISandboxStatePending
}
