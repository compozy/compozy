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
	if len(targets) == 0 {
		return nil
	}
	actor, err := b.nativeWorkspaceAccessActor(ctx, scope)
	if err != nil {
		return errors.Join(nativeWorkspaceAccessDeniedError(descriptor.ID), err)
	}
	targetWorkspaceIDs := make([]string, 0, len(targets))
	for targetWorkspaceID := range targets {
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

func (b *nativeWorkspaceInputBinder) authorizeWorkspaceTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
	actor workspaceaccess.ActorRef,
	targetWorkspaceID string,
) error {
	if targetWorkspaceID == strings.TrimSpace(actor.WorkspaceID) {
		return nil
	}
	if b == nil || b.workspaceAccess == nil {
		return nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	decision, err := b.workspaceAccess.Authorize(ctx, workspaceaccess.Request{
		Actor:             actor,
		TargetWorkspaceID: targetWorkspaceID,
		Seam:              workspaceaccess.SeamTool,
	})
	if err != nil {
		return errors.Join(
			nativeWorkspaceAccessDeniedError(descriptor.ID),
			fmt.Errorf("daemon: authorize native workspace target: %w", err),
		)
	}
	if decision.Allowed {
		return nil
	}
	if !decision.PromptEligible || b.prompter == nil {
		return nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	allowed, err := b.prompter.RequestWorkspaceAccess(ctx, scope, call, descriptor, targetWorkspaceID)
	if err != nil {
		return errors.Join(
			nativeWorkspaceAccessDeniedError(descriptor.ID),
			fmt.Errorf("daemon: prompt for native workspace access: %w", err),
		)
	}
	if !allowed {
		return nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	return nil
}

func nativeWorkspaceScopeHasOperatorAuthority(scope toolspkg.Scope) bool {
	return scope.Operator
}

func (b *nativeWorkspaceInputBinder) nativeWorkspaceAccessActor(
	ctx context.Context,
	scope toolspkg.Scope,
) (workspaceaccess.ActorRef, error) {
	if workspaceaccess.ActorKind(strings.TrimSpace(scope.ActorKind)) == workspaceaccess.ActorDaemon {
		workspaceID := strings.TrimSpace(scope.WorkspaceID)
		if workspaceID == "" {
			return workspaceaccess.ActorRef{}, errors.New(
				"daemon: native workspace access daemon scope has no workspace id",
			)
		}
		return workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorDaemon,
			WorkspaceID: workspaceID,
			AgentName:   strings.TrimSpace(scope.AgentName),
		}, nil
	}
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return workspaceaccess.ActorRef{}, errors.New("daemon: native workspace access requires a session id")
	}
	if b == nil || b.sessions == nil {
		return workspaceaccess.ActorRef{}, errors.New("daemon: native workspace access session lookup is unavailable")
	}
	info, err := b.sessions.Status(ctx, sessionID)
	if err != nil {
		return workspaceaccess.ActorRef{}, fmt.Errorf("daemon: validate native workspace access session: %w", err)
	}
	if info == nil {
		return workspaceaccess.ActorRef{}, errors.New("daemon: native workspace access session lookup returned nil")
	}
	if strings.TrimSpace(info.ID) != sessionID {
		return workspaceaccess.ActorRef{}, errors.New(
			"daemon: native workspace access session lookup returned a different session",
		)
	}
	if info.State != session.StateActive {
		return workspaceaccess.ActorRef{}, fmt.Errorf(
			"daemon: native workspace access session %q is %q",
			sessionID,
			info.State,
		)
	}
	workspaceID := strings.TrimSpace(info.WorkspaceID)
	if workspaceID == "" {
		return workspaceaccess.ActorRef{}, errors.New("daemon: native workspace access session has no workspace id")
	}
	return workspaceaccess.ActorRef{
		Kind:        workspaceaccess.ActorAgentSession,
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
		AgentName:   strings.TrimSpace(info.AgentName),
	}, nil
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

func nativeWorkspaceAccessDeniedError(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q denied: %s", id, workspaceaccess.DenialHint),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonWorkspaceAccessDenied,
	)
}
