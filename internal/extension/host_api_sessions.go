package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	"github.com/compozy/agh/internal/network/participation"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/store"
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
			Provider:  info.Provider,
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
		Provider:             strings.TrimSpace(params.Provider),
		Model:                strings.TrimSpace(params.Model),
		ReasoningEffort:      strings.TrimSpace(string(params.ReasoningEffort)),
		Workspace:            strings.TrimSpace(params.Workspace),
		NetworkParticipation: participation.CloneRequest(params.NetworkParticipation),
		Type:                 session.SessionTypeSystem,
	}
	prompt := strings.TrimSpace(params.Prompt)
	if prompt != "" {
		acceptance, ok := h.sessions.(hostAPISessionAcceptanceManager)
		if !ok {
			return nil, errors.New("extension: session manager does not support durable acceptance")
		}
		info, err := acceptance.CreateAccepted(ctx, session.CreateAcceptedOpts{
			Session:       createOpts,
			InitialPrompt: prompt,
		})
		if err != nil {
			return nil, err
		}
		return hostAPISessionCreateResult{SessionID: info.ID, Provider: info.Provider}, nil
	}

	sess, err := h.sessions.Create(ctx, createOpts)
	if err != nil {
		return nil, err
	}
	return hostAPISessionCreateResult{SessionID: sess.ID, Provider: sess.Info().Provider}, nil
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

	submission, err := h.submitPrompt(ctx, params.SessionID, params.Message)
	if err != nil {
		return nil, err
	}

	return hostAPISessionPromptResult{TurnID: submission.TurnID}, nil
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
