package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/network/participation"

	"github.com/compozy/compozy/internal/session"

	"github.com/compozy/compozy/internal/store"
)

func (h *HostAPIHandler) handleSessionsList(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}

	var params hostAPISessionsListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	infos, err := h.sessions.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	filterWorkspaceID := ""
	filterWorkspaceRoot := ""
	if workspaceRef := strings.TrimSpace(params.Workspace); workspaceRef != "" {
		if h.workspaces != nil {
			resolved, resolveErr := h.workspaces.Resolve(ctx, workspaceRef)
			if resolveErr != nil {
				return nil, resolveErr
			}
			filterWorkspaceID, resolveErr = hostAPIResolvedWorkspaceRegistrationID(&resolved)
			if resolveErr != nil {
				return nil, resolveErr
			}
			filterWorkspaceRoot = strings.TrimSpace(resolved.RootDir)
		} else {
			filterWorkspaceID = workspaceRef
			filterWorkspaceRoot = workspaceRef
		}
	}

	result := make([]hostAPISessionSummary, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		if filterWorkspaceID != "" || filterWorkspaceRoot != "" {
			if info.WorkspaceID != filterWorkspaceID && info.Workspace != filterWorkspaceRoot {
				continue
			}
		}
		result = append(result, hostAPISessionSummary{
			ID:        info.ID,
			Name:      info.Name,
			Agent:     info.AgentName,
			Runtime:   hostAPISessionRuntimePayloadFromInfo(info),
			Workspace: info.Workspace,
			State:     info.State,
			CreatedAt: info.CreatedAt,
		})
	}

	return result, nil
}

func (h *HostAPIHandler) handleSessionsCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}

	var params hostAPISessionCreateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Agent) == "" {
		return nil, invalidParamsRPCError(errors.New("agent is required"))
	}

	createOpts := session.CreateOpts{
		AgentName:            strings.TrimSpace(params.Agent),
		Workspace:            strings.TrimSpace(params.Workspace),
		NetworkParticipation: participation.CloneRequest(params.NetworkParticipation),
		Type:                 session.SessionTypeSystem,
	}
	acceptance, ok := h.sessions.(hostAPISessionAcceptanceManager)
	if !ok {
		return nil, errors.New("extension: session manager does not support durable acceptance")
	}
	info, err := acceptance.CreateAccepted(ctx, session.CreateAcceptedOpts{Session: createOpts})
	if err != nil {
		return nil, err
	}
	return hostAPISessionCreateResult{SessionID: info.ID}, nil
}

func (h *HostAPIHandler) handleSessionsPrompt(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionPromptParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	if strings.TrimSpace(params.Message) == "" {
		return nil, invalidParamsRPCError(errors.New("message is required"))
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}

	submission, err := h.submitPrompt(
		ctx,
		params.SessionID,
		params.Message,
		hostAPIPromptRuntimeSelection(params.Runtime),
	)
	if err != nil {
		return nil, err
	}

	return hostAPISessionPromptResultFromSubmission(submission), nil
}

func hostAPIPromptRuntimeSelection(
	payload *apicontract.PromptRuntimeSelectionPayload,
) *session.RuntimeSelection {
	return apicontract.PromptRuntimeSelectionFromPayload(payload)
}

func hostAPISessionPromptResultFromSubmission(
	submission hostAPIPromptSubmission,
) hostAPISessionPromptResult {
	admission := submission.Admission
	turnID := strings.TrimSpace(submission.TurnID)
	if turnID == "" {
		turnID = strings.TrimSpace(admission.NewTurnID)
	}

	result := hostAPISessionPromptResult{
		Status:                     strings.TrimSpace(admission.Status),
		Mode:                       admission.Mode,
		TurnID:                     turnID,
		QueueEntryID:               strings.TrimSpace(admission.QueueEntryID),
		QueuePosition:              admission.QueuePosition,
		QueueGeneration:            admission.QueueGeneration,
		PreviousTurnID:             strings.TrimSpace(admission.PreviousTurnID),
		Interrupted:                admission.Interrupted,
		Staged:                     admission.Staged,
		Queued:                     admission.Queued,
		CanceledQueuedEntries:      admission.CanceledQueuedEntries,
		FallbackModeIfNoToolResult: strings.TrimSpace(admission.FallbackModeIfNoToolResult),
	}
	if admission.EstimatedSendAt != nil {
		estimated := admission.EstimatedSendAt.UTC()
		result.EstimatedSendAt = &estimated
	}
	return result
}

func (h *HostAPIHandler) handleSessionsStop(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	if err := h.sessions.Stop(ctx, params.SessionID); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

func (h *HostAPIHandler) handleSessionsStatus(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}

	info, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID)
	if err != nil {
		return nil, err
	}
	return hostAPISessionStatusFromInfo(info), nil
}

func (h *HostAPIHandler) handleSessionsEvents(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionEventsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}

	events, err := h.sessions.Events(ctx, params.SessionID, store.EventQuery{
		Type:          strings.TrimSpace(params.Type),
		AgentName:     strings.TrimSpace(params.AgentName),
		TurnID:        strings.TrimSpace(params.TurnID),
		Since:         params.Since,
		Limit:         params.Limit,
		AfterSequence: params.Offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]hostAPISessionEvent, 0, len(events))
	for _, event := range events {
		result = append(result, hostAPISessionEvent{
			Type:      event.Type,
			Timestamp: event.Timestamp,
			Data:      decodeJSONValue(event.Content),
		})
	}
	return result, nil
}
