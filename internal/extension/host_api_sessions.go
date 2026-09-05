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
	archive, err := normalizeHostAPISessionArchiveFilter(params.Archive)
	if err != nil {
		return nil, err
	}

	infos, err := h.sessions.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	filterWorkspaceID, filterWorkspaceRoot, err := h.resolveHostAPISessionListWorkspace(
		ctx,
		params.Workspace,
	)
	if err != nil {
		return nil, err
	}

	result := make([]hostAPISessionSummary, 0, len(infos))
	for _, info := range infos {
		info, err = h.hostAPISessionWithArchiveMetadata(ctx, info)
		if err != nil {
			return nil, err
		}
		if !hostAPISessionMatchesArchive(info, archive) {
			continue
		}
		if !hostAPISessionMatchesWorkspace(info, filterWorkspaceID, filterWorkspaceRoot) {
			continue
		}
		result = append(result, hostAPISessionSummary{
			ID:         info.ID,
			Name:       info.Name,
			Agent:      info.AgentName,
			Runtime:    hostAPISessionRuntimePayloadFromInfo(info),
			Workspace:  info.Workspace,
			State:      info.State,
			ArchivedAt: cloneHostAPITime(info.ArchivedAt),
			CreatedAt:  info.CreatedAt,
		})
	}

	return result, nil
}

func (h *HostAPIHandler) handleSessionsArchive(ctx context.Context, raw json.RawMessage) (any, error) {
	return h.handleSessionsArchiveState(ctx, raw, true)
}

func (h *HostAPIHandler) handleSessionsUnarchive(ctx context.Context, raw json.RawMessage) (any, error) {
	return h.handleSessionsArchiveState(ctx, raw, false)
}

func (h *HostAPIHandler) handleSessionsArchiveState(
	ctx context.Context,
	raw json.RawMessage,
	archived bool,
) (any, error) {
	var params hostAPISessionTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	info, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID)
	if err != nil {
		return nil, err
	}
	manager, ok := h.sessions.(hostAPISessionArchiveManager)
	if !ok {
		return nil, errors.New("extension: session archive manager is not configured")
	}
	if archived {
		info, err = manager.Archive(ctx, info.WorkspaceID, info.ID)
	} else {
		info, err = manager.Unarchive(ctx, info.WorkspaceID, info.ID)
	}
	if err != nil {
		return nil, err
	}
	return hostAPISessionStatusFromInfo(info), nil
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
	if strings.TrimSpace(params.MessageID) == "" {
		return nil, invalidParamsRPCError(errors.New("message_id is required"))
	}
	if strings.TrimSpace(params.IdempotencyKey) == "" {
		return nil, invalidParamsRPCError(errors.New("idempotency_key is required"))
	}
	mode := session.BusyInputMode(strings.TrimSpace(string(params.Mode)))
	if (mode == session.BusyInputModeSteer || mode == session.BusyInputModeInterrupt) &&
		strings.TrimSpace(params.ExpectedTurnID) == "" {
		return nil, invalidParamsRPCError(
			errors.New("expected_turn_id is required for steer and interrupt mode"),
		)
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}

	submission, err := h.submitPrompt(
		ctx,
		params.SessionID,
		hostAPIPromptRequest{
			Message:        params.Message,
			MessageID:      params.MessageID,
			IdempotencyKey: params.IdempotencyKey,
			Mode:           mode,
			ExpectedTurnID: params.ExpectedTurnID,
			Runtime:        hostAPIPromptRuntimeSelection(params.Runtime),
		},
	)
	if err != nil {
		return nil, err
	}

	return hostAPISessionPromptResultFromSubmission(submission), nil
}

func (h *HostAPIHandler) handleSessionsInputsList(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionInputsListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	manager, err := h.pendingInputManager()
	if err != nil {
		return nil, err
	}
	inputs, err := manager.ListPendingInputs(ctx, params.SessionID)
	if err != nil {
		return nil, err
	}
	result := hostAPISessionInputListResult{Inputs: make([]hostAPISessionInput, 0, len(inputs))}
	for _, input := range inputs {
		result.Inputs = append(result.Inputs, hostAPISessionInputFromPending(input))
	}
	return result, nil
}

func (h *HostAPIHandler) handleSessionsInputsReplace(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionInputReplaceParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateHostAPISessionInputTarget(params.WorkspaceID, params.SessionID, params.QueueEntryID); err != nil {
		return nil, err
	}
	request := apicontract.ReplaceSessionInputRequest{
		Text: params.Text, MessageID: params.MessageID, IdempotencyKey: params.IdempotencyKey,
	}
	if err := request.Validate(); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	manager, err := h.pendingInputManager()
	if err != nil {
		return nil, err
	}
	input, err := manager.ReplacePendingInput(
		ctx,
		params.SessionID,
		params.QueueEntryID,
		session.ReplacePendingInputOpts{
			Text: request.Text, MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	return hostAPISessionInputResult{Input: hostAPISessionInputFromPending(input)}, nil
}

func (h *HostAPIHandler) handleSessionsInputsCancel(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionInputTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateHostAPISessionInputTarget(params.WorkspaceID, params.SessionID, params.QueueEntryID); err != nil {
		return nil, err
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	manager, err := h.pendingInputManager()
	if err != nil {
		return nil, err
	}
	result, err := manager.CancelQueuedPrompt(ctx, params.SessionID, params.QueueEntryID)
	if err != nil {
		return nil, err
	}
	return hostAPISessionPromptResultFromAdmission(result), nil
}

func (h *HostAPIHandler) handleSessionsInputsPromote(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionInputPromoteParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateHostAPISessionInputTarget(params.WorkspaceID, params.SessionID, params.QueueEntryID); err != nil {
		return nil, err
	}
	request := apicontract.PromoteSessionInputRequest{
		Text: params.Text, MessageID: params.MessageID, IdempotencyKey: params.IdempotencyKey,
		ExpectedTurnID: params.ExpectedTurnID,
	}
	if err := request.Validate(); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, params.SessionID); err != nil {
		return nil, err
	}
	manager, err := h.pendingInputManager()
	if err != nil {
		return nil, err
	}
	result, err := manager.PromotePendingInputToSteer(
		ctx,
		params.SessionID,
		params.QueueEntryID,
		session.PromotePendingInputOpts{
			Text: request.Text, MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
			ExpectedTurnID: request.ExpectedTurnID,
		},
	)
	if err != nil {
		return nil, err
	}
	return hostAPISessionPromptResultFromAdmission(result), nil
}

func validateHostAPISessionInputTarget(workspaceID string, sessionID string, queueEntryID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return invalidParamsRPCError(errors.New("workspace_id is required"))
	}
	if strings.TrimSpace(sessionID) == "" {
		return invalidParamsRPCError(errors.New("session_id is required"))
	}
	if strings.TrimSpace(queueEntryID) == "" {
		return invalidParamsRPCError(errors.New("queue_entry_id is required"))
	}
	return nil
}

func (h *HostAPIHandler) pendingInputManager() (hostAPIPendingInputSessionManager, error) {
	if h == nil || h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	manager, ok := h.sessions.(hostAPIPendingInputSessionManager)
	if !ok {
		return nil, errors.New("extension: session manager does not support durable pending input")
	}
	return manager, nil
}

func hostAPISessionInputFromPending(input session.PendingInput) hostAPISessionInput {
	result := hostAPISessionInput{
		ID:              input.ID,
		SessionID:       input.SessionID,
		MessageID:       input.MessageID,
		IdempotencyKey:  input.IdempotencyKey,
		TargetTurnID:    input.TargetTurnID,
		Status:          input.Status,
		Mode:            apicontract.PromptMode(input.Mode),
		Delivery:        apicontract.PromptDelivery(input.Delivery),
		SteerDelivery:   input.SteerDelivery,
		Text:            input.Text,
		QueueGeneration: input.QueueGeneration,
		EnqueuedAt:      input.EnqueuedAt,
	}
	result.Runtime = apicontract.PromptRuntimeSelectionPayloadFromSelection(input.Runtime)
	return result
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
		turnID = admission.Outcome().TurnID
	}

	result := hostAPISessionPromptResult{
		Disposition:           admission.Outcome().Disposition,
		SteerDelivery:         admission.SteerDelivery,
		EntryID:               admission.QueueEntryID,
		Status:                strings.TrimSpace(admission.Status),
		Mode:                  apicontract.PromptMode(admission.Mode),
		Delivery:              apicontract.PromptDelivery(admission.Delivery),
		MessageID:             admission.MessageID,
		IdempotencyKey:        admission.IdempotencyKey,
		Replayed:              admission.Replayed,
		TurnID:                turnID,
		QueueEntryID:          strings.TrimSpace(admission.QueueEntryID),
		QueuePosition:         admission.QueuePosition,
		QueueGeneration:       admission.QueueGeneration,
		PreviousTurnID:        strings.TrimSpace(admission.PreviousTurnID),
		CanceledQueuedEntries: admission.CanceledQueuedEntries,
	}
	if admission.EstimatedSendAt != nil {
		estimated := admission.EstimatedSendAt.UTC()
		result.EstimatedSendAt = &estimated
	}
	return result
}

func hostAPISessionPromptResultFromAdmission(
	admission session.SendPromptResult,
) hostAPISessionPromptResult {
	return hostAPISessionPromptResultFromSubmission(hostAPIPromptSubmission{Admission: admission})
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
