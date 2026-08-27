package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

type daemonCallSessionInvoker struct {
	sessions                callSessionManager
	isOperatorCallerSession func(context.Context, string) (bool, error)
	maxChildren             int
	maxDepth                int
}

type callSessionManager interface {
	Status(context.Context, string) (*session.Info, error)
	IsPrompting(string) bool
	Resume(context.Context, string) (*session.Session, error)
	SendPrompt(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error)
	QueuedInputDeliveryStatus(context.Context, string, string) (session.InputDeliveryStatus, error)
	StopWithCause(context.Context, string, session.StopCause, string) error
}

type callSessionSpawner interface {
	Spawn(context.Context, session.SpawnOpts) (*session.Session, error)
}

func (i *daemonCallSessionInvoker) SpawnChild(
	ctx context.Context,
	spec callspkg.ChildSpec,
) (callspkg.SessionRef, error) {
	desiredID := "ses_call_" + strings.TrimPrefix(spec.CallID, "call_")
	existing, err := i.sessions.Status(ctx, desiredID)
	if err == nil {
		if existing == nil || existing.AgentName != spec.AgentName || existing.Lineage == nil ||
			strings.TrimSpace(existing.Lineage.ParentSessionID) != strings.TrimSpace(spec.ParentSessionID) {
			return callspkg.SessionRef{}, errors.New("daemon: existing call child identity does not match activation")
		}
		if existing.State == session.StateStopped {
			if _, err := i.sessions.Resume(ctx, desiredID); err != nil {
				return callspkg.SessionRef{}, fmt.Errorf("daemon: resume call child %q: %w", desiredID, err)
			}
		}
		if err := i.sendCallRequest(ctx, desiredID, spec); err != nil {
			return callspkg.SessionRef{}, err
		}
		return callspkg.SessionRef{ID: desiredID}, nil
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		return callspkg.SessionRef{}, fmt.Errorf("daemon: inspect call child %q: %w", desiredID, err)
	}
	spawner, ok := i.sessions.(callSessionSpawner)
	if !ok {
		return callspkg.SessionRef{}, errors.New("daemon: session manager does not implement child spawn")
	}
	budget := &store.SessionSpawnBudget{MaxChildren: i.maxChildren, MaxDepth: i.maxDepth}
	child, err := spawner.Spawn(ctx, session.SpawnOpts{
		ParentSessionID: spec.ParentSessionID, DesiredSessionID: desiredID, AgentName: spec.AgentName,
		Provider: spec.Runtime.Provider, Model: spec.Runtime.Model,
		ReasoningEffort: spec.Runtime.ReasoningEffort, Speed: spec.Runtime.Speed,
		TTL: spec.IdleTTL, AutoStopOnParent: true, NotifyCreator: false, NotifyCreatorSet: true,
		PermissionPolicy: sessionPermissionPolicy(spec.Permissions.Policy()), GovernanceBudget: budget,
		AllowedToolsOverride: append([]string(nil), spec.Permissions.Tools...),
	})
	if err != nil {
		if errors.Is(err, session.ErrSpawnPermissionDenied) {
			return callspkg.SessionRef{}, &callspkg.Error{
				Code: callspkg.CodeWideningRejected, Message: "spawn hook widened caller permissions", Cause: err,
			}
		}
		return callspkg.SessionRef{}, err
	}
	if err := i.sendCallRequest(ctx, child.ID, spec); err != nil {
		cleanupErr := i.sessions.StopWithCause(ctx, child.ID, session.CauseFailed, "call activation prompt failed")
		return callspkg.SessionRef{}, errors.Join(err, cleanupErr)
	}
	return callspkg.SessionRef{ID: child.ID}, nil
}

func sessionPermissionPolicy(policy callspkg.PermissionPolicy) store.SessionPermissionPolicy {
	return store.SessionPermissionPolicy{
		Tools: policy.Tools, Skills: policy.Skills, MCPServers: policy.MCPServers,
		WorkspacePaths: policy.WorkspacePaths, NetworkChannels: policy.NetworkChannels,
		SandboxProfiles: policy.SandboxProfiles,
	}
}

func (i *daemonCallSessionInvoker) Revive(ctx context.Context, sessionID, prompt, callID string) error {
	metadata := &acp.PromptSyntheticMeta{
		CallID: callID, CallState: string(callspkg.StateRunning),
		ChildSessionID: sessionID, Reason: "call_follow_up",
	}
	var err error
	for range 2 {
		if _, err = i.sessions.Resume(ctx, sessionID); err != nil {
			return fmt.Errorf("daemon: resume call session %q: %w", sessionID, err)
		}
		if _, err = i.send(ctx, sessionID, prompt, callID, metadata); err == nil {
			return nil
		}
		if !errors.Is(err, session.ErrSessionNotActive) {
			break
		}
	}
	cleanupErr := i.sessions.StopWithCause(ctx, sessionID, session.CauseFailed, "call revival prompt failed")
	return errors.Join(err, cleanupErr)
}

func (i *daemonCallSessionInvoker) sendCallRequest(
	ctx context.Context,
	sessionID string,
	spec callspkg.ChildSpec,
) error {
	_, err := i.send(
		ctx,
		sessionID,
		callspkg.CallPromptWithRemainingDepth(spec.CallID, spec.Prompt, spec.RemainingDepth),
		spec.CallID,
		callRequestSyntheticMetadata(spec, sessionID, "call_request"),
	)
	return err
}

func callRequestSyntheticMetadata(
	spec callspkg.ChildSpec,
	childSessionID string,
	reason string,
) *acp.PromptSyntheticMeta {
	return &acp.PromptSyntheticMeta{
		CallID: spec.CallID, CallState: string(callspkg.StateRunning),
		ChildSessionID: childSessionID, ChildAgentName: spec.AgentName, Reason: reason,
	}
}

func (i *daemonCallSessionInvoker) DeliverAtBoundary(
	ctx context.Context,
	delivery callspkg.Delivery,
) (callspkg.DeliveryOutcome, error) {
	if i.isOperatorCallerSession != nil {
		operatorCaller, err := i.isOperatorCallerSession(ctx, delivery.RecipientSessionID)
		if err != nil {
			return callspkg.DeliveryOutcome{}, fmt.Errorf("daemon: inspect operator call recipient: %w", err)
		}
		if operatorCaller {
			return callspkg.DeliveryOutcome{
				State:  callspkg.DeliveryStateAttention,
				Reason: "operator_attention",
			}, nil
		}
	}
	woken := false
	info, err := i.sessions.Status(ctx, delivery.RecipientSessionID)
	if err != nil {
		return callspkg.DeliveryOutcome{}, fmt.Errorf("daemon: inspect call delivery recipient: %w", err)
	}
	if info != nil && info.State == session.StateStopped {
		if _, err := i.sessions.Resume(ctx, delivery.RecipientSessionID); err != nil {
			return callspkg.DeliveryOutcome{}, fmt.Errorf("daemon: revive call delivery recipient: %w", err)
		}
		woken = true
	}
	if !woken && i.sessions.IsPrompting(delivery.RecipientSessionID) {
		return callspkg.DeliveryOutcome{State: callspkg.DeliveryStatePending, Reason: "recipient_busy"}, nil
	}
	identity := strings.TrimSpace(delivery.WakeEventID)
	if identity == "" {
		identity = delivery.CallID + ":" + string(delivery.Kind)
	}
	metadata := acp.PromptSyntheticMeta{
		CallID: delivery.Metadata.CallID, CallState: delivery.Metadata.CallState,
		ResultBytes: delivery.Metadata.ResultBytes, ContractDigest: delivery.Metadata.ContractDigest,
		MessageID: delivery.Metadata.MessageID, DeliveryKind: delivery.Metadata.DeliveryKind,
		Reason: delivery.Metadata.Reason, WakeEventID: delivery.Metadata.WakeEventID,
	}.Normalize()
	result, err := i.send(ctx, delivery.RecipientSessionID, delivery.Body, identity, &metadata)
	if err != nil {
		return callspkg.DeliveryOutcome{}, err
	}
	if result.QueueEntryID != "" {
		queued, statusErr := i.sessions.QueuedInputDeliveryStatus(ctx, delivery.RecipientSessionID, result.QueueEntryID)
		if statusErr != nil {
			return callspkg.DeliveryOutcome{}, fmt.Errorf("daemon: inspect queued call delivery: %w", statusErr)
		}
		switch queued.Status {
		case store.SessionInputQueueStatusQueued, store.SessionInputQueueStatusDispatching:
			return callspkg.DeliveryOutcome{State: callspkg.DeliveryStatePending, Reason: queued.Status}, nil
		case store.SessionInputQueueStatusFailed, store.SessionInputQueueStatusCanceled:
			reason := strings.TrimSpace(queued.FailureSummary)
			if reason == "" {
				reason = queued.Status
			}
			return callspkg.DeliveryOutcome{}, fmt.Errorf("daemon: queued call delivery %s: %s", queued.Status, reason)
		case store.SessionInputQueueStatusSent:
			return callspkg.DeliveryOutcome{State: callspkg.DeliveryStateInjected, Reason: queued.Status}, nil
		default:
			return callspkg.DeliveryOutcome{}, fmt.Errorf(
				"daemon: unsupported queued call delivery state %q",
				queued.Status,
			)
		}
	}
	state := callspkg.DeliveryStateInjected
	if woken || result.Delivery == store.SessionInputDeliveryDirect {
		state = callspkg.DeliveryStateWoken
	}
	return callspkg.DeliveryOutcome{State: state, Reason: result.Delivery}, nil
}

func (i *daemonCallSessionInvoker) send(
	ctx context.Context,
	sessionID, message, identity string,
	metadata *acp.PromptSyntheticMeta,
) (session.SendPromptResult, error) {
	identity = strings.TrimSpace(identity)
	return i.sessions.SendPrompt(ctx, sessionID, session.SendPromptOpts{
		Message: message, MessageID: "msg_" + identity, IdempotencyKey: "call:" + identity,
		Synthetic: metadata, Mode: session.BusyInputModeQueue,
	})
}

func (i *daemonCallSessionInvoker) StopManaged(ctx context.Context, sessionID, reason string) error {
	return i.sessions.StopWithCause(ctx, sessionID, session.CauseUserRequested, reason)
}

var _ callspkg.SessionInvoker = (*daemonCallSessionInvoker)(nil)
