package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	nativeWorkspaceInputKey           = "workspace"
	nativeWorkspaceSchemaAllOfKeyword = "allOf"
	nativeWorkspaceSchemaAnyOfKeyword = "anyOf"
	nativeWorkspaceSchemaOneOfKeyword = "oneOf"
)

type nativeWorkspaceInputBinder struct {
	workspaces      core.WorkspaceService
	sessions        core.SessionManager
	workspaceAccess workspaceaccess.Policy
	prompter        workspaceAccessPrompter
}

var (
	_ toolspkg.CallInputBinder     = (*nativeWorkspaceInputBinder)(nil)
	_ toolspkg.CallInputAuthorizer = (*nativeWorkspaceInputBinder)(nil)
)

func newNativeWorkspaceInputBinder(
	workspaces core.WorkspaceService,
	sessions core.SessionManager,
	workspaceAccess workspaceaccess.Policy,
	prompter workspaceAccessPrompter,
) *nativeWorkspaceInputBinder {
	return &nativeWorkspaceInputBinder{
		workspaces:      workspaces,
		sessions:        sessions,
		workspaceAccess: workspaceAccess,
		prompter:        prompter,
	}
}

func (b *nativeWorkspaceInputBinder) BindCallInput(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	input json.RawMessage,
) (json.RawMessage, error) {
	if descriptor.Backend.Kind != toolspkg.BackendNativeGo {
		return input, nil
	}
	schema, acceptsWorkspace := nativeWorkspaceInputSchema(descriptor)
	if !acceptsWorkspace {
		return input, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, nativeWorkspaceObjectInputError(descriptor.ID, err)
	}
	if payload == nil {
		return nil, nativeWorkspaceObjectInputError(descriptor.ID, nil)
	}
	if err := b.bindNativeWorkspaceNode(ctx, scope, descriptor.ID, schema, payload); err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func (b *nativeWorkspaceInputBinder) bindNativeWorkspaceNode(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	schema map[string]json.RawMessage,
	payload map[string]json.RawMessage,
) error {
	properties := nativeWorkspaceSchemaProperties(schema)
	if _, ok := properties[nativeWorkspaceInputKey]; ok {
		if err := b.bindNativeWorkspaceField(ctx, scope, id, payload); err != nil {
			return err
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
		rawPayload, ok := payload[name]
		if !ok {
			continue
		}
		var childPayload map[string]json.RawMessage
		if err := json.Unmarshal(rawPayload, &childPayload); err != nil || childPayload == nil {
			return nativeWorkspaceObjectInputError(id, err)
		}
		if err := b.bindNativeWorkspaceNode(ctx, scope, id, childSchema, childPayload); err != nil {
			return err
		}
		encoded, err := json.Marshal(childPayload)
		if err != nil {
			return fmt.Errorf("daemon: encode bound native workspace input %q: %w", name, err)
		}
		payload[name] = encoded
	}
	for _, keyword := range []string{
		nativeWorkspaceSchemaAllOfKeyword,
		nativeWorkspaceSchemaAnyOfKeyword,
		nativeWorkspaceSchemaOneOfKeyword,
	} {
		for _, childSchema := range nativeWorkspaceCompositeSchemas(schema[keyword]) {
			if !nativeWorkspaceSchemaAcceptsWorkspace(childSchema) {
				continue
			}
			if err := b.bindNativeWorkspaceNode(ctx, scope, id, childSchema, payload); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *nativeWorkspaceInputBinder) bindNativeWorkspaceField(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	payload map[string]json.RawMessage,
) error {
	requested, validString := nativeWorkspaceInputValue(payload[nativeWorkspaceInputKey])
	if !validString {
		return nil
	}
	trusted := strings.TrimSpace(scope.WorkspaceID)
	if requested == "" {
		if trusted == "" || nativeInputHasGlobalScope(payload) {
			return nil
		}
		return setNativeWorkspaceInput(payload, trusted)
	}
	if scope.Operator {
		return b.resolveNativeWorkspaceInput(ctx, id, payload, requested)
	}
	if trusted == "" {
		return b.resolveNativeWorkspaceInput(ctx, id, payload, requested)
	}
	if requested == trusted {
		return setNativeWorkspaceInput(payload, trusted)
	}
	if b == nil || b.workspaces == nil {
		return nativeNetworkInputError(id, workspacepkg.ErrWorkspaceResolverUnavailable)
	}
	requestedWorkspace, err := b.workspaces.Resolve(ctx, requested)
	if err != nil {
		return nativeNetworkInputError(id, err)
	}
	trustedWorkspace, err := b.workspaces.Resolve(ctx, trusted)
	if err != nil {
		return nativeNetworkInputError(id, err)
	}
	if strings.TrimSpace(requestedWorkspace.ID) == strings.TrimSpace(trustedWorkspace.ID) {
		return setNativeWorkspaceInput(payload, trustedWorkspace.ID)
	}
	return setNativeWorkspaceInput(payload, requestedWorkspace.ID)
}

func (b *nativeWorkspaceInputBinder) AuthorizeCallInput(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
) error {
	if descriptor.Backend.Kind != toolspkg.BackendNativeGo || scope.Operator || nativeWorkspaceScopeUnbound(scope) {
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
	actor, live := b.nativeWorkspaceAccessActor(ctx, scope)
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
			live,
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
	live bool,
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
		return nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	if decision.Allowed {
		return nil
	}
	if !decision.PromptEligible || !live || b.prompter == nil {
		return nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	allowed, err := b.prompter.RequestWorkspaceAccess(ctx, scope, call, descriptor, targetWorkspaceID)
	if err != nil || !allowed {
		return nativeWorkspaceAccessDeniedError(descriptor.ID)
	}
	return nil
}

func nativeWorkspaceScopeUnbound(scope toolspkg.Scope) bool {
	return strings.TrimSpace(scope.SessionID) == "" &&
		strings.TrimSpace(scope.WorkspaceID) == "" &&
		strings.TrimSpace(scope.AgentName) == "" &&
		strings.TrimSpace(scope.ActorKind) == ""
}

func (b *nativeWorkspaceInputBinder) nativeWorkspaceAccessActor(
	ctx context.Context,
	scope toolspkg.Scope,
) (workspaceaccess.ActorRef, bool) {
	actor := workspaceaccess.ActorRef{
		Kind:        workspaceaccess.ActorAgentSession,
		SessionID:   strings.TrimSpace(scope.SessionID),
		WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		AgentName:   strings.TrimSpace(scope.AgentName),
	}
	if b == nil || b.sessions == nil || actor.SessionID == "" {
		return actor, false
	}
	info, err := b.sessions.Status(ctx, actor.SessionID)
	if err != nil || info == nil {
		return actor, false
	}
	actor.SessionID = strings.TrimSpace(info.ID)
	actor.WorkspaceID = strings.TrimSpace(info.WorkspaceID)
	actor.AgentName = strings.TrimSpace(info.AgentName)
	return actor, info.State == session.StateActive
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
	for _, keyword := range []string{
		nativeWorkspaceSchemaAllOfKeyword,
		nativeWorkspaceSchemaAnyOfKeyword,
		nativeWorkspaceSchemaOneOfKeyword,
	} {
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

func nativeInputHasGlobalScope(payload map[string]json.RawMessage) bool {
	raw, ok := payload["scope"]
	if !ok {
		return false
	}
	var scope string
	if err := json.Unmarshal(raw, &scope); err != nil {
		return false
	}
	switch strings.TrimSpace(scope) {
	case nativeBridgeScopeGlobal, nativeBridgeScopeAll:
		return true
	default:
		return false
	}
}

func (b *nativeWorkspaceInputBinder) resolveNativeWorkspaceInput(
	ctx context.Context,
	id toolspkg.ToolID,
	payload map[string]json.RawMessage,
	ref string,
) error {
	if b == nil || b.workspaces == nil {
		return nativeNetworkInputError(id, workspacepkg.ErrWorkspaceResolverUnavailable)
	}
	resolved, err := b.workspaces.Resolve(ctx, ref)
	if err != nil {
		return nativeNetworkInputError(id, err)
	}
	return setNativeWorkspaceInput(payload, resolved.ID)
}

func nativeWorkspaceInputSchema(
	descriptor toolspkg.Descriptor,
) (map[string]json.RawMessage, bool) {
	schema := nativeWorkspaceSchemaObject(descriptor.InputSchema)
	return schema, nativeWorkspaceSchemaAcceptsWorkspace(schema)
}

func nativeWorkspaceSchemaAcceptsWorkspace(schema map[string]json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	for name, rawSchema := range nativeWorkspaceSchemaProperties(schema) {
		if name == nativeWorkspaceInputKey ||
			nativeWorkspaceSchemaAcceptsWorkspace(nativeWorkspaceSchemaObject(rawSchema)) {
			return true
		}
	}
	for _, keyword := range []string{
		nativeWorkspaceSchemaAllOfKeyword,
		nativeWorkspaceSchemaAnyOfKeyword,
		nativeWorkspaceSchemaOneOfKeyword,
	} {
		if slices.ContainsFunc(
			nativeWorkspaceCompositeSchemas(schema[keyword]),
			nativeWorkspaceSchemaAcceptsWorkspace,
		) {
			return true
		}
	}
	return false
}

func nativeWorkspaceSchemaProperties(
	schema map[string]json.RawMessage,
) map[string]json.RawMessage {
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(schema["properties"], &properties); err != nil {
		return nil
	}
	return properties
}

func nativeWorkspaceCompositeSchemas(raw json.RawMessage) []map[string]json.RawMessage {
	var encoded []json.RawMessage
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil
	}
	schemas := make([]map[string]json.RawMessage, 0, len(encoded))
	for _, item := range encoded {
		if schema := nativeWorkspaceSchemaObject(item); len(schema) > 0 {
			schemas = append(schemas, schema)
		}
	}
	return schemas
}

func nativeWorkspaceSchemaObject(raw json.RawMessage) map[string]json.RawMessage {
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	return schema
}

func nativeWorkspaceObjectInputError(id toolspkg.ToolID, cause error) error {
	if cause == nil {
		cause = toolspkg.ErrToolInvalidInput
	} else {
		cause = fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, cause)
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		fmt.Sprintf("tool %q input must be a JSON object", id),
		cause,
		toolspkg.ReasonSchemaInvalid,
	)
}

func nativeWorkspaceInputValue(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", true
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func setNativeWorkspaceInput(
	payload map[string]json.RawMessage,
	workspace string,
) error {
	encoded, err := json.Marshal(strings.TrimSpace(workspace))
	if err != nil {
		return err
	}
	payload[nativeWorkspaceInputKey] = encoded
	return nil
}

func (n *daemonNativeTools) nativeWorkspaceRoot(
	ctx context.Context,
	id toolspkg.ToolID,
	ref string,
) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", nil
	}
	if n == nil || n.deps == nil || n.deps.Workspaces == nil {
		return "", nativeNetworkInputError(id, workspacepkg.ErrWorkspaceResolverUnavailable)
	}
	resolved, err := n.deps.Workspaces.Resolve(ctx, ref)
	if err != nil {
		return "", nativeNetworkInputError(id, err)
	}
	return strings.TrimSpace(resolved.RootDir), nil
}
