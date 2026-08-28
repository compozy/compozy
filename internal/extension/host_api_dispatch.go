package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/subprocess"
)

// Handle dispatches one Host API request for the named extension.
func (h *HostAPIHandler) Handle(
	ctx context.Context,
	extName string,
	method string,
	params json.RawMessage,
) (any, error) {
	if h == nil {
		return nil, errors.New("extension: host api handler is required")
	}
	if ctx == nil {
		return nil, errors.New("extension: host api context is required")
	}

	method = strings.TrimSpace(method)
	handler, ok := h.methods[method]
	if !ok {
		return nil, methodNotFoundRPCError(method)
	}

	capabilityGrantID := hostAPICapabilityGrantIDFromContext(ctx)
	if capabilityGrantID == "" {
		capabilityGrantID = extName
	}
	if err := h.capChecker.CheckHostAPI(capabilityGrantID, method); err != nil {
		return nil, rpcCapabilityDenied(err)
	}
	limiter := h.limiter
	if method == string(extensioncontract.HostAPIMethodViewPatch) {
		limiter = h.viewLimiter
	}
	if err := limiter.Allow(extName, method); err != nil {
		return nil, normalizeHostAPIRPCError(method, err)
	}

	hostCtx := withHostAPIExtensionName(ctx, extName)
	params, err := h.bindWorkspaceScopedParams(hostCtx, method, params)
	if err != nil {
		return nil, normalizeHostAPIRPCError(method, err)
	}
	result, err := handler(hostCtx, params)
	if err != nil {
		return nil, normalizeHostAPIRPCError(method, err)
	}
	return result, nil
}

// HandleMethod returns a subprocess-compatible handler for one Host API method.
func (h *HostAPIHandler) HandleMethod(method string) subprocess.HandlerFunc {
	method = strings.TrimSpace(method)
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		return h.Handle(ctx, hostAPIExtensionNameFromContext(ctx), method, params)
	}
}

// MethodHandlers returns the subprocess-compatible handler set for every Host API method.
func (h *HostAPIHandler) MethodHandlers() map[string]subprocess.HandlerFunc {
	out := make(map[string]subprocess.HandlerFunc, len(h.methods))
	for method := range h.methods {
		out[method] = h.HandleMethod(method)
	}
	return out
}

func withHostAPIBridgeRuntime(ctx context.Context, bridgeRuntime *subprocess.InitializeBridgeRuntime) context.Context {
	if ctx == nil || bridgeRuntime == nil {
		return ctx
	}
	return context.WithValue(
		ctx,
		hostAPIBridgeRuntimeContextKey,
		subprocess.CloneInitializeBridgeRuntime(bridgeRuntime),
	)
}

type hostAPIResourceSession struct {
	Actor resources.MutationActor
}

func withHostAPIResourceSession(ctx context.Context, session *hostAPIResourceSession) context.Context {
	if ctx == nil || session == nil {
		return ctx
	}
	cloned := &hostAPIResourceSession{Actor: cloneResourceMutationActor(session.Actor)}
	return context.WithValue(ctx, hostAPIResourceSessionContextKey, cloned)
}

func hostAPIResourceSessionFromContext(ctx context.Context) (*hostAPIResourceSession, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(hostAPIResourceSessionContextKey).(*hostAPIResourceSession)
	if !ok || value == nil {
		return nil, false
	}
	return &hostAPIResourceSession{Actor: cloneResourceMutationActor(value.Actor)}, true
}

func cloneResourceMutationActor(actor resources.MutationActor) resources.MutationActor {
	return resources.MutationActor{
		Kind:          actor.Kind,
		ID:            actor.ID,
		SessionNonce:  actor.SessionNonce,
		Source:        actor.Source,
		MaxScope:      actor.MaxScope,
		GrantedKinds:  append([]resources.ResourceKind(nil), actor.GrantedKinds...),
		GrantedScopes: append([]resources.ResourceScopeKind(nil), actor.GrantedScopes...),
	}
}

func withHostAPIInstanceKey(ctx context.Context, key InstanceKey) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, hostAPIInstanceKeyContextKey, key.Normalize())
}

func hostAPIInstanceKeyFromContext(ctx context.Context) (InstanceKey, bool) {
	if ctx == nil {
		return InstanceKey{}, false
	}
	key, ok := ctx.Value(hostAPIInstanceKeyContextKey).(InstanceKey)
	if !ok {
		return InstanceKey{}, false
	}
	return key.Normalize(), true
}
