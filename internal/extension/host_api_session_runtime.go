package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	apicontract "github.com/compozy/compozy/internal/api/contract"
)

func (h *HostAPIHandler) handleSessionsRuntimeSet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionRuntimeSetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateHostAPISessionRuntimeTarget(
		params.WorkspaceID,
		params.SessionID,
		params.ExpectedRevision,
	); err != nil {
		return nil, err
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	manager, err := h.runtimeSelectionManager()
	if err != nil {
		return nil, err
	}
	selection := apicontract.PromptRuntimeSelectionFromPayload(&params.Runtime)
	info, err := manager.SetRuntimeSelection(ctx, params.SessionID, *selection, *params.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	return hostAPISessionStatusFromInfo(info), nil
}

func (h *HostAPIHandler) handleSessionsRuntimeClear(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionRuntimeClearParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateHostAPISessionRuntimeTarget(
		params.WorkspaceID,
		params.SessionID,
		params.ExpectedRevision,
	); err != nil {
		return nil, err
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	manager, err := h.runtimeSelectionManager()
	if err != nil {
		return nil, err
	}
	info, err := manager.ClearRuntimeSelection(ctx, params.SessionID, *params.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	return hostAPISessionStatusFromInfo(info), nil
}

func validateHostAPISessionRuntimeTarget(workspaceID, sessionID string, expectedRevision *int64) error {
	if strings.TrimSpace(workspaceID) == "" {
		return invalidParamsRPCError(errors.New("workspace_id is required"))
	}
	if strings.TrimSpace(sessionID) == "" {
		return invalidParamsRPCError(errors.New("session_id is required"))
	}
	if expectedRevision == nil {
		return invalidParamsRPCError(errors.New("expected_revision is required"))
	}
	if *expectedRevision < 0 {
		return invalidParamsRPCError(errors.New("expected_revision must not be negative"))
	}
	return nil
}

func (h *HostAPIHandler) runtimeSelectionManager() (hostAPIRuntimeSelectionSessionManager, error) {
	if h == nil || h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	manager, ok := h.sessions.(hostAPIRuntimeSelectionSessionManager)
	if !ok {
		return nil, errors.New("extension: session manager does not support runtime selection")
	}
	return manager, nil
}
