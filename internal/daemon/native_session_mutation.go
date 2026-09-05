package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/worktree"
)

type sessionCreateWorktreeInput struct {
	Name string `json:"name,omitempty"`
}

type sessionCreateInput struct {
	Workspace            string                      `json:"workspace,omitempty"`
	Agent                string                      `json:"agent"`
	Name                 string                      `json:"name,omitempty"`
	Worktree             string                      `json:"worktree,omitempty"`
	NewWorktree          *sessionCreateWorktreeInput `json:"new_worktree,omitempty"`
	NetworkParticipation *participation.Request      `json:"network_participation,omitempty"`
}

type sessionPromptInput struct {
	Workspace      string                                  `json:"workspace,omitempty"`
	SessionID      string                                  `json:"session_id"`
	Message        string                                  `json:"message"`
	Attachments    []string                                `json:"attachments,omitempty"`
	MessageID      string                                  `json:"message_id"`
	IdempotencyKey string                                  `json:"idempotency_key"`
	Mode           string                                  `json:"mode,omitempty"`
	ExpectedTurnID string                                  `json:"expected_turn_id,omitempty"`
	Wait           bool                                    `json:"wait,omitempty"`
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
	worktreeTarget, err := n.resolveNativeSessionWorktree(ctx, workspaceID, scope.ProfileID, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	opts := session.CreateOpts{
		AgentName: agent,
		Name:      strings.TrimSpace(input.Name),
		Workspace: workspaceID,
		Worktree:  worktreeTarget.ID,
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
		return toolspkg.ToolResult{}, errors.Join(err, n.rollbackNativeSessionWorktree(ctx, worktreeTarget))
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

const (
	nativeSessionWorktreeRollbackTimeout = 30 * time.Second
	nativeSessionMessageIDKey            = "message_id"
)

type nativeSessionWorktreeTarget struct {
	ID          string
	WorkspaceID string
	Created     bool
}

func (n *daemonNativeTools) resolveNativeSessionWorktree(
	ctx context.Context,
	workspaceID string,
	profileID string,
	input sessionCreateInput,
) (nativeSessionWorktreeTarget, error) {
	ref := strings.TrimSpace(input.Worktree)
	if ref != "" && input.NewWorktree != nil {
		return nativeSessionWorktreeTarget{}, errors.New("daemon: worktree and new_worktree are mutually exclusive")
	}
	if input.NewWorktree == nil {
		return nativeSessionWorktreeTarget{ID: ref}, nil
	}
	if n.deps.Worktrees == nil {
		return nativeSessionWorktreeTarget{}, errors.New("daemon: worktree creation is unavailable")
	}
	item, err := n.deps.Worktrees.CreateReady(ctx, workspaceID, worktree.CreateOptions{
		ProfileID: profileID,
		Name:      strings.TrimSpace(input.NewWorktree.Name),
		Origin:    worktree.OriginManual,
	})
	if err != nil {
		return nativeSessionWorktreeTarget{}, err
	}
	return nativeSessionWorktreeTarget{ID: item.ID, WorkspaceID: workspaceID, Created: true}, nil
}

func (n *daemonNativeTools) rollbackNativeSessionWorktree(
	ctx context.Context,
	target nativeSessionWorktreeTarget,
) error {
	if !target.Created || n.deps.Worktrees == nil {
		return nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nativeSessionWorktreeRollbackTimeout)
	defer cancel()
	if _, err := n.deps.Worktrees.Remove(rollbackCtx, target.WorkspaceID, target.ID, true); err != nil {
		return fmt.Errorf("daemon: roll back materialized session worktree: %w", err)
	}
	return nil
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
	message := strings.TrimSpace(input.Message)
	if message == "" && len(input.Attachments) == 0 {
		return toolspkg.ToolResult{}, nativeNetworkInputError(
			req.ToolID,
			errors.New("message or attachments is required"),
		)
	}
	messageID, err := requiredNativeString(req.ToolID, nativeSessionMessageIDKey, input.MessageID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	mode := session.BusyInputMode(strings.TrimSpace(input.Mode))
	expectedTurnID := strings.TrimSpace(input.ExpectedTurnID)
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
	attachments, err := n.resolveNativePromptAttachments(
		ctx,
		req.ToolID,
		workspaceID,
		nativeWorkspaceAttachmentRoots(&resolved),
		sessionID,
		input.Attachments,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	deliveryCtx, cancelDelivery := context.WithCancel(ctx)
	defer cancelDelivery()
	result, err := n.deps.Sessions.SendPrompt(ctx, sessionID, session.SendPromptOpts{
		Message: message, MessageID: messageID, IdempotencyKey: idempotencyKey,
		Mode: mode, ExpectedTurnID: expectedTurnID,
		DeliveryContext: deliveryCtx,
		Runtime:         core.PromptRuntimeSelectionFromPayload(input.Runtime), Attachments: attachments,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Wait && result.Events != nil {
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
	messageID, err := requiredNativeString(req.ToolID, nativeSessionMessageIDKey, input.MessageID)
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
