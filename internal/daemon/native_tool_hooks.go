package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeToolHookRunner struct {
	hooks  session.ToolHooks
	binder *nativeWorkspaceInputBinder
	now    func() time.Time
}

var _ toolspkg.HookRunner = (*nativeToolHookRunner)(nil)

func (r *nativeToolHookRunner) PreCall(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
) (toolspkg.CallRequest, toolspkg.EffectiveToolDecision, error) {
	allowed := toolspkg.EffectiveToolDecision{Callable: true}
	if descriptor.Backend.Kind != toolspkg.BackendNativeGo {
		return call, allowed, nil
	}
	ctx, sessionCtx, err := r.context(ctx, call)
	if err != nil {
		return call, allowed, err
	}
	payload, err := r.hooks.DispatchToolPreCall(ctx, hookspkg.ToolPreCallPayload{
		PayloadBase: r.base(hookspkg.HookToolPreCall), SessionContext: sessionCtx,
		TurnID:      call.TurnID,
		ToolCallRef: nativeHookToolRef(descriptor, call), ToolInput: call.Input,
	})
	if err != nil {
		return call, allowed, err
	}
	if payload.ReadOnly != descriptor.ReadOnly {
		return call, allowed, errors.New("native tool hook cannot change read-only classification")
	}
	bound, err := r.binder.BindCallInput(ctx, scope, descriptor, payload.ToolInput)
	if err != nil {
		return call, allowed, err
	}
	if schema, accepts := nativeWorkspaceInputSchema(descriptor); accepts {
		var before, after map[string]json.RawMessage
		if err := json.Unmarshal(call.Input, &before); err != nil {
			return call, allowed, err
		}
		if err := json.Unmarshal(bound, &after); err != nil {
			return call, allowed, err
		}
		if nativeHookWorkspaceChanged(schema, before, after) {
			return call, allowed, errors.New("native tool hook cannot change a bound workspace")
		}
	}
	call.ToolID = toolspkg.ToolID(payload.ToolID)
	call.Input = bound
	return call, allowed, nil
}

func (r *nativeToolHookRunner) PostCall(
	ctx context.Context,
	_ toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
	result toolspkg.ToolResult,
) (toolspkg.ToolResult, error) {
	if descriptor.Backend.Kind != toolspkg.BackendNativeGo {
		return result, nil
	}
	ctx, sessionCtx, err := r.context(ctx, call)
	if err != nil {
		return result, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	payload, err := r.hooks.DispatchToolPostCall(ctx, hookspkg.ToolPostCallPayload{
		PayloadBase: r.base(hookspkg.HookToolPostCall), SessionContext: sessionCtx,
		TurnID:      call.TurnID,
		ToolCallRef: nativeHookToolRef(descriptor, call), ToolInput: call.Input,
		Title: result.Preview, ToolResult: encoded,
	})
	if err != nil {
		return result, err
	}
	var patched toolspkg.ToolResult
	if err := json.Unmarshal(payload.ToolResult, &patched); err != nil {
		return result, fmt.Errorf("decode native hook result: %w", err)
	}
	if payload.Title != result.Preview {
		patched.Preview = payload.Title
	}
	return patched, nil
}

func (r *nativeToolHookRunner) PostError(
	ctx context.Context,
	_ toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
	callErr error,
) (error, error) {
	if descriptor.Backend.Kind != toolspkg.BackendNativeGo {
		return callErr, nil
	}
	ctx, sessionCtx, err := r.context(ctx, call)
	if err != nil {
		return callErr, err
	}
	payload, err := r.hooks.DispatchToolPostError(ctx, hookspkg.ToolPostErrorPayload{
		PayloadBase: r.base(hookspkg.HookToolPostError), SessionContext: sessionCtx,
		TurnID:      call.TurnID,
		ToolCallRef: nativeHookToolRef(descriptor, call), ToolInput: call.Input, Error: callErr.Error(),
	})
	if err != nil || payload.Error == callErr.Error() {
		return callErr, err
	}
	if toolErr, ok := errors.AsType[*toolspkg.ToolError](callErr); ok {
		patched := *toolErr
		patched.Message = payload.Error
		return &patched, nil
	}
	return errors.New(payload.Error), nil
}

func (r *nativeToolHookRunner) context(
	ctx context.Context,
	call toolspkg.CallRequest,
) (context.Context, hookspkg.SessionContext, error) {
	if call.SessionID != "" {
		if r.binder.sessions == nil {
			return ctx, hookspkg.SessionContext{}, errors.New("native tool hook session context is unavailable")
		}
		info, err := r.binder.sessions.Status(ctx, call.SessionID)
		if err != nil {
			return ctx, hookspkg.SessionContext{}, err
		}
		if source, ok := r.binder.sessions.(interface {
			ToolHookContext(context.Context, string) context.Context
		}); ok {
			ctx = source.ToolHookContext(ctx, call.SessionID)
		}
		return ctx, hookSessionInfoContext(info), nil
	}
	value := hookspkg.SessionContext{
		ProfileID:   call.ProfileID,
		WorkspaceID: call.WorkspaceID,
		AgentName:   call.AgentName,
	}
	if call.WorkspaceID != "" && r.binder.workspaces != nil {
		resolved, err := r.binder.workspaces.Resolve(ctx, call.WorkspaceID)
		if err != nil {
			return ctx, value, err
		}
		value.Workspace = resolved.RootDir
	}
	return ctx, value, nil
}

func (r *nativeToolHookRunner) base(event hookspkg.HookEvent) hookspkg.PayloadBase {
	timestamp := time.Now().UTC()
	if r.now != nil {
		timestamp = r.now().UTC()
	}
	return hookspkg.PayloadBase{Event: event, Timestamp: timestamp}
}

func nativeHookToolRef(descriptor toolspkg.Descriptor, call toolspkg.CallRequest) hookspkg.ToolCallRef {
	return hookspkg.ToolCallRef{
		ToolID:     call.ToolID.String(),
		ToolCallID: call.ToolCallID,
		ReadOnly:   descriptor.ReadOnly,
	}
}

func nativeHookWorkspaceChanged(
	schema, before, after map[string]json.RawMessage,
) bool {
	properties := nativeWorkspaceSchemaProperties(schema)
	if _, exists := properties[nativeWorkspaceInputKey]; exists {
		beforeID, _ := nativeWorkspaceInputValue(before[nativeWorkspaceInputKey])
		afterID, _ := nativeWorkspaceInputValue(after[nativeWorkspaceInputKey])
		if strings.TrimSpace(beforeID) != strings.TrimSpace(afterID) ||
			nativeInputHasGlobalScope(before) != nativeInputHasGlobalScope(after) {
			return true
		}
	}
	for name, rawSchema := range properties {
		childSchema := nativeWorkspaceSchemaObject(rawSchema)
		if !nativeWorkspaceSchemaAcceptsWorkspace(childSchema) {
			continue
		}
		var beforeChild, afterChild map[string]json.RawMessage
		if err := json.Unmarshal(before[name], &beforeChild); err != nil {
			beforeChild = nil
		}
		if err := json.Unmarshal(after[name], &afterChild); err != nil {
			afterChild = nil
		}
		if nativeHookWorkspaceChanged(childSchema, beforeChild, afterChild) {
			return true
		}
	}
	for _, keyword := range nativeWorkspaceCompositeKeywords {
		for _, childSchema := range nativeWorkspaceCompositeSchemas(schema[keyword]) {
			if nativeHookWorkspaceChanged(childSchema, before, after) {
				return true
			}
		}
	}
	return false
}
