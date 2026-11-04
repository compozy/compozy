package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (b *nativeWorkspaceInputBinder) AuthorizeCallInput(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
) error {
	if descriptor.Backend.Kind != toolspkg.BackendNativeGo || nativeWorkspaceScopeHasOperatorAuthority(scope) {
		return nil
	}
	schema, acceptsWorkspace := nativeWorkspaceInputSchema(descriptor)
	if !acceptsWorkspace {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(call.Input, &payload); err != nil || payload == nil {
		return nativeWorkspaceObjectInputError(descriptor.ID, err)
	}
	targets := make(map[string]struct{})
	collectNativeWorkspaceTargets(schema, payload, targets)
	if nativeInputHasGlobalScope(payload) {
		targets[workspaceaccess.UserSelector] = struct{}{}
	}
	sessionID := nativeSessionTargetID(descriptor.ID, schema, payload)
	if len(targets) == 0 && sessionID == "" {
		return nil
	}
	actor, actorProfileID, err := b.nativeWorkspaceAccessActor(ctx, scope)
	if err != nil {
		return errors.Join(nativeWorkspaceAccessDeniedError(descriptor.ID), err)
	}
	otherCrossing := false
	for target := range targets {
		if nativeTargetCrossesActor(target, actor) {
			otherCrossing = true
		}
	}
	covered, err := b.authorizeNativeSessionTarget(ctx, scope, descriptor, call, nativeSessionTargetRequest{
		actor: actor, actorProfileID: actorProfileID, sessionID: sessionID, otherCrossing: otherCrossing,
	})
	if err != nil {
		return err
	}
	targetWorkspaceIDs := make([]string, 0, len(targets))
	for targetWorkspaceID := range targets {
		if covered != "" && targetWorkspaceID == covered {
			continue
		}
		targetWorkspaceIDs = append(targetWorkspaceIDs, targetWorkspaceID)
	}
	slices.Sort(targetWorkspaceIDs)
	for _, targetWorkspaceID := range targetWorkspaceIDs {
		if err := b.authorizeWorkspaceTarget(
			ctx,
			scope,
			descriptor,
			call,
			actor,
			targetWorkspaceID,
		); err != nil {
			return err
		}
	}
	return nil
}

// nativeTargetCrossesActor reports whether an explicit target names a scope other than the
// actor's own. The @user selector is the actor's own scope when the actor lives in the User
// scope, so naming it must not skip the unresolved-target check and reveal session existence.
func nativeTargetCrossesActor(target string, actor workspaceaccess.ActorRef) bool {
	if target == workspaceaccess.UserSelector {
		return actor.WorkspaceID != ""
	}
	return target != actor.WorkspaceID
}

func (b *nativeWorkspaceInputBinder) authorizeWorkspaceTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
	actor workspaceaccess.ActorRef,
	targetWorkspaceID string,
) error {
	if targetWorkspaceID != workspaceaccess.UserSelector && targetWorkspaceID == actor.WorkspaceID {
		return nil
	}
	if descriptor.ID == toolspkg.ToolIDSessionPrompt {
		var input sessionPromptInput
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return err
		}
		target, err := nativeSessionReplyTarget(ctx, b.sessions, scope, input)
		if err != nil {
			return err
		}
		if target != nil && nativeReplyWorkspace(target) == targetWorkspaceID {
			return nil
		}
	}
	_, err := b.authorizeAccessTarget(ctx, scope, descriptor, call, actor, nativeAccessTarget{
		ref:   workspaceaccess.ScopeTarget(targetWorkspaceID, scope.ProfileID),
		label: targetWorkspaceID,
	})
	return err
}

// nativeAccessTarget is one policy target plus the label its consent prompt shows.
type nativeAccessTarget struct {
	ref   workspaceaccess.TargetRef
	label string
}

// authorizeAccessTarget is the single decision path for every target of a native call: policy,
// approve-reads prompt, axis-specific denial, and the outside-scope mark that redacts the result.
// It reports whether the call was admitted.
func (b *nativeWorkspaceInputBinder) authorizeAccessTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
	actor workspaceaccess.ActorRef,
	target nativeAccessTarget,
) (bool, error) {
	if b == nil || b.workspaceAccess == nil {
		return false, nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	decision, err := b.workspaceAccess.Authorize(ctx, workspaceaccess.Request{
		Actor:    actor,
		Target:   target.ref,
		ReadOnly: descriptor.ReadOnly,
		Seam:     workspaceaccess.SeamTool,
	})
	if err != nil {
		return false, errors.Join(
			nativeWorkspaceAccessDeniedError(descriptor.ID),
			fmt.Errorf("daemon: authorize native workspace target: %w", err),
		)
	}
	if decision.Allowed {
		if decision.Source != workspaceaccess.SourceSameWorkspace {
			toolspkg.MarkOutsideCallerScope(ctx)
		}
		return true, nil
	}
	if !decision.PromptEligible || b.prompter == nil {
		return false, nativeWorkspaceAccessDecisionError(descriptor.ID, decision)
	}
	prompt := workspaceAccessPrompt{label: target.label, remember: !decision.NeedsGrantAfterConsent}
	if prompt.remember {
		prompt.grant = workspaceAccessReadGrantKey(scope, call, descriptor, actor, target.ref)
	}
	allowed, err := b.prompter.RequestWorkspaceAccess(ctx, scope, call, descriptor, prompt)
	if err != nil {
		return false, errors.Join(
			nativeWorkspaceAccessDecisionError(descriptor.ID, decision),
			fmt.Errorf("daemon: prompt for native workspace access: %w", err),
		)
	}
	if !allowed {
		return false, nativeWorkspaceAccessDecisionError(
			descriptor.ID,
			workspaceaccess.Decision{Denial: workspaceaccess.DenialConsentRejected},
		)
	}
	if decision.NeedsGrantAfterConsent {
		return false, nativeWorkspaceAccessDecisionError(
			descriptor.ID,
			workspaceaccess.Decision{Denial: workspaceaccess.DenialNeedsGrant},
		)
	}
	toolspkg.MarkOutsideCallerScope(ctx)
	return true, nil
}

func nativeWorkspaceScopeHasOperatorAuthority(scope toolspkg.Scope) bool {
	return scope.Operator
}

func (b *nativeWorkspaceInputBinder) nativeWorkspaceAccessActor(
	ctx context.Context,
	scope toolspkg.Scope,
) (workspaceaccess.ActorRef, string, error) {
	if workspaceaccess.ActorKind(strings.TrimSpace(scope.ActorKind)) == workspaceaccess.ActorDaemon {
		workspaceID := strings.TrimSpace(scope.WorkspaceID)
		if workspaceID == "" {
			return workspaceaccess.ActorRef{}, "", errors.New(
				"daemon: native workspace access daemon scope has no workspace id",
			)
		}
		return workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorDaemon,
			WorkspaceID: workspaceID,
			AgentName:   strings.TrimSpace(scope.AgentName),
		}, "", nil
	}
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return workspaceaccess.ActorRef{}, "", errors.New("daemon: native workspace access requires a session id")
	}
	if b == nil || b.sessions == nil {
		return workspaceaccess.ActorRef{}, "", errors.New(
			"daemon: native workspace access session lookup is unavailable",
		)
	}
	info, err := b.sessions.Status(ctx, sessionID)
	if err != nil {
		return workspaceaccess.ActorRef{}, "", fmt.Errorf("daemon: validate native workspace access session: %w", err)
	}
	if info == nil {
		return workspaceaccess.ActorRef{}, "", errors.New("daemon: native workspace access session lookup returned nil")
	}
	if strings.TrimSpace(info.ID) != sessionID {
		return workspaceaccess.ActorRef{}, "", errors.New(
			"daemon: native workspace access session lookup returned a different session",
		)
	}
	if info.State != session.StateActive {
		return workspaceaccess.ActorRef{}, "", fmt.Errorf(
			"daemon: native workspace access session %q is %q",
			sessionID,
			info.State,
		)
	}
	if strings.TrimSpace(scope.ProfileID) != "" &&
		strings.TrimSpace(info.ProfileID) != strings.TrimSpace(scope.ProfileID) {
		return workspaceaccess.ActorRef{}, "", errors.New(
			"daemon: native workspace access profile differs from the session profile",
		)
	}
	workspaceID := strings.TrimSpace(info.WorkspaceID)

	return workspaceaccess.ActorRef{
		Kind:        workspaceaccess.ActorAgentSession,
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
		AgentName:   strings.TrimSpace(info.AgentName),
	}, strings.TrimSpace(info.ProfileID), nil
}

func collectNativeWorkspaceTargets(
	schema map[string]json.RawMessage,
	payload map[string]json.RawMessage,
	targets map[string]struct{},
) {
	properties := nativeWorkspaceSchemaProperties(schema)
	if _, ok := properties[nativeWorkspaceInputKey]; ok {
		if value, valid := nativeWorkspaceInputValue(payload[nativeWorkspaceInputKey]); valid {
			if value = strings.TrimSpace(value); value != "" {
				targets[value] = struct{}{}
			}
		}
	}
	for name, rawSchema := range properties {
		if name == nativeWorkspaceInputKey {
			continue
		}
		childSchema := nativeWorkspaceSchemaObject(rawSchema)
		if !nativeWorkspaceSchemaAcceptsWorkspace(childSchema) {
			continue
		}
		var childPayload map[string]json.RawMessage
		if err := json.Unmarshal(payload[name], &childPayload); err == nil && childPayload != nil {
			collectNativeWorkspaceTargets(childSchema, childPayload, targets)
		}
	}
	for _, keyword := range nativeWorkspaceCompositeKeywords {
		for _, childSchema := range nativeWorkspaceCompositeSchemas(schema[keyword]) {
			collectNativeWorkspaceTargets(childSchema, payload, targets)
		}
	}
}

// nativeWorkspaceAccessDecisionError names the axis that denied the call and what would allow it.
// The message never names the target, so a missing target and a denied one read the same.
func nativeWorkspaceAccessDecisionError(id toolspkg.ToolID, decision workspaceaccess.Decision) error {
	return toolspkg.NewWorkspaceAccessDeniedError(id, workspaceaccess.DenialMessage(decision))
}

func nativeWorkspaceAccessDeniedError(id toolspkg.ToolID) error {
	return toolspkg.NewWorkspaceAccessDeniedError(id, workspaceaccess.DenialHint)
}
