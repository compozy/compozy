package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const nativeWorkspaceInputKey = "workspace"

type nativeWorkspaceInputBinder struct {
	workspaces core.WorkspaceService
}

var _ toolspkg.CallInputBinder = (*nativeWorkspaceInputBinder)(nil)

func newNativeWorkspaceInputBinder(workspaces core.WorkspaceService) *nativeWorkspaceInputBinder {
	return &nativeWorkspaceInputBinder{workspaces: workspaces}
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
	trusted := strings.TrimSpace(scope.WorkspaceID)
	if trusted != "" &&
		!scope.Operator &&
		nativeInputHasGlobalScope(payload) {
		return nil, nativeScopeMismatchError(descriptor.ID, nativeWorkspaceInputKey)
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
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
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
		return nativeScopeMismatchError(id, nativeWorkspaceInputKey)
	}
	requestedWorkspace, err := b.workspaces.Resolve(ctx, requested)
	if err != nil {
		return nativeNetworkInputError(id, err)
	}
	trustedWorkspace, err := b.workspaces.Resolve(ctx, trusted)
	if err != nil {
		return nativeNetworkInputError(id, err)
	}
	if strings.TrimSpace(requestedWorkspace.ID) != strings.TrimSpace(trustedWorkspace.ID) {
		return nativeScopeMismatchError(id, nativeWorkspaceInputKey)
	}
	return setNativeWorkspaceInput(payload, trustedWorkspace.ID)
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
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
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
