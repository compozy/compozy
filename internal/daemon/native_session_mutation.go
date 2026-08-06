package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionCreateInput struct {
	Workspace            string                 `json:"workspace,omitempty"`
	Agent                string                 `json:"agent"`
	Name                 string                 `json:"name,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

type sessionPromptInput struct {
	Workspace      string                                  `json:"workspace,omitempty"`
	SessionID      string                                  `json:"session_id"`
	Message        string                                  `json:"message"`
	MessageID      string                                  `json:"message_id"`
	IdempotencyKey string                                  `json:"idempotency_key"`
	Mode           string                                  `json:"mode,omitempty"`
	ExpectedTurnID string                                  `json:"expected_turn_id,omitempty"`
	Runtime        *contract.PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
}

type sessionRewindInput struct {
	Workspace           string `json:"workspace,omitempty"`
	SessionID           string `json:"session_id"`
	MessageID           string `json:"message_id"`
	IdempotencyKey      string `json:"idempotency_key"`
	ExpectedEpoch       *int64 `json:"expected_epoch"`
	ExpectedGeneration  *int64 `json:"expected_generation"`
	ExpectedMaxSequence *int64 `json:"expected_max_sequence"`
}

func (n *daemonNativeTools) sessionCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	agent, err := requiredNativeString(req.ToolID, "agent", input.Agent)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	acceptance, ok := n.deps.Sessions.(core.SessionAcceptanceManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: durable session acceptance is required")
	}
	opts := session.CreateOpts{
		AgentName: agent,
		Name:      strings.TrimSpace(input.Name),
		Workspace: workspaceID,
		Type:      session.SessionTypeUser,
		NetworkParticipation: participation.CloneRequest(
			input.NetworkParticipation,
		),
	}
	// The bound caller session is server-minted and not overridable through tool
	// input. Cross-workspace creates stay unlinked: provenance never crosses the
	// workspace isolation boundary.
	callerSessionID := strings.TrimSpace(scope.SessionID)
	callerWorkspaceID := strings.TrimSpace(scope.WorkspaceID)
	if callerSessionID != "" && callerWorkspaceID != "" && callerWorkspaceID == workspaceID {
		opts.Lineage = &store.SessionLineage{ParentSessionID: callerSessionID}
	}
	info, err := acceptance.CreateAccepted(ctx, session.CreateAcceptedOpts{Session: opts})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionPrompt(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionPromptInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	message, err := requiredNativeString(req.ToolID, "message", input.Message)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	messageID, err := requiredNativeString(req.ToolID, "message_id", input.MessageID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	mode := session.BusyInputMode(strings.TrimSpace(input.Mode))
	expectedTurnID := strings.TrimSpace(input.ExpectedTurnID)
	if (mode == session.BusyInputModeSteer || mode == session.BusyInputModeInterrupt) &&
		expectedTurnID == "" {
		return toolspkg.ToolResult{}, nativeNetworkInputError(
			req.ToolID,
			errors.New("expected_turn_id is required for steer and interrupt mode"),
		)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Sessions.SendPrompt(ctx, sessionID, session.SendPromptOpts{
		Message: message, MessageID: messageID, IdempotencyKey: idempotencyKey,
		Mode: mode, ExpectedTurnID: expectedTurnID,
		Runtime: core.PromptRuntimeSelectionFromPayload(input.Runtime),
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if result.Events != nil {
		if err := drainNativeSessionPromptEvents(result.Events); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	return n.nativeSessionPromptResult(result)
}

func (n *daemonNativeTools) sessionRewind(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionRewindInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	messageID, err := requiredNativeString(req.ToolID, "message_id", input.MessageID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	expectedEpoch, err := requiredNativeNonNegativeInt64(req.ToolID, "expected_epoch", input.ExpectedEpoch)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	expectedGeneration, err := requiredNativeNonNegativeInt64(
		req.ToolID,
		"expected_generation",
		input.ExpectedGeneration,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	expectedMaxSequence, err := requiredNativeNonNegativeInt64(
		req.ToolID,
		"expected_max_sequence",
		input.ExpectedMaxSequence,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Sessions.RewindConversation(ctx, sessionID, session.ConversationRewindOptions{
		MessageID: messageID, IdempotencyKey: idempotencyKey,
		ExpectedEpoch: expectedEpoch, ExpectedGeneration: expectedGeneration,
		ExpectedMaxSequence: expectedMaxSequence,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := contract.SessionConversationRewindResponse{
		Session: core.SessionPayloadFromInfo(result.Session.Info()),
		Rewind: contract.SessionConversationRewindPayload{
			TranscriptEpoch: result.TranscriptEpoch, TargetMessageID: result.TargetMessageID,
			ArchivedFrom: result.ArchivedFrom, ArchivedThrough: result.ArchivedThrough,
			ArchivedEvents: result.ArchivedEvents, Generation: result.Generation,
			MaxSequence: result.MaxSequence, DraftText: result.DraftText, Replayed: result.Replayed,
		},
	}
	return structuredResult(payload, payload.Rewind.TargetMessageID)
}

func requiredNativeNonNegativeInt64(id toolspkg.ToolID, field string, value *int64) (int64, error) {
	if value == nil {
		return 0, nativeRequiredInputError(id, field)
	}
	if *value < 0 {
		return 0, nativeNetworkInputError(id, fmt.Errorf("%s must not be negative", field))
	}
	return *value, nil
}
