package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type daemonExtensionWorkspaceResolver interface {
	ResolveRegistration(ctx context.Context, ref string) (workspacepkg.Workspace, error)
}

type daemonExtensionToolProvider struct {
	inner             toolspkg.Provider
	workspaceResolver daemonExtensionWorkspaceResolver
}

var _ toolspkg.Provider = (*daemonExtensionToolProvider)(nil)
var _ toolspkg.ProjectionGenerationProvider = (*daemonExtensionToolProvider)(nil)
var _ toolspkg.ProjectionAvailabilityProvider = (*daemonExtensionToolProvider)(nil)
var _ toolspkg.ProjectionScopeProvider = (*daemonExtensionToolProvider)(nil)

// newDaemonScopedExtensionToolProvider validates workspace scope while preserving an absent provider as nil.
func newDaemonScopedExtensionToolProvider(
	inner toolspkg.Provider,
	workspaceResolver daemonExtensionWorkspaceResolver,
) toolspkg.Provider {
	if inner == nil {
		return nil
	}
	return &daemonExtensionToolProvider{
		inner:             inner,
		workspaceResolver: workspaceResolver,
	}
}

func (p *daemonExtensionToolProvider) ID() toolspkg.SourceRef {
	return p.inner.ID()
}

// ProjectionGeneration delegates generation reads through canonical workspace scope.
func (p *daemonExtensionToolProvider) ProjectionGeneration(
	ctx context.Context,
	scope toolspkg.Scope,
) (string, bool) {
	canonical, err := p.canonicalWorkspaceScope(ctx, scope)
	if err != nil {
		return "", false
	}
	source, ok := p.inner.(toolspkg.ProjectionGenerationProvider)
	if !ok {
		return "", false
	}
	return source.ProjectionGeneration(ctx, canonical)
}

// ProjectionScope forwards the inner listing scope; an inner provider without one keeps every field.
func (p *daemonExtensionToolProvider) ProjectionScope(scope toolspkg.Scope) toolspkg.Scope {
	if inner, ok := p.inner.(toolspkg.ProjectionScopeProvider); ok {
		return inner.ProjectionScope(scope)
	}
	return scope
}

func (p *daemonExtensionToolProvider) List(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.Descriptor, error) {
	canonical, err := p.canonicalWorkspaceScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	return p.inner.List(ctx, canonical)
}

// ProjectionAvailability preserves canonical workspace scope for request-local batches.
func (p *daemonExtensionToolProvider) ProjectionAvailability(
	ctx context.Context,
	scope toolspkg.Scope,
	ids []toolspkg.ToolID,
) map[toolspkg.ToolID]toolspkg.Availability {
	canonical, err := p.canonicalWorkspaceScope(ctx, scope)
	availability := make(map[toolspkg.ToolID]toolspkg.Availability, len(ids))
	if err != nil {
		if len(ids) > 0 {
			toolspkg.LogBackendFailure(ctx, ids[0], "tool_projection_workspace_resolve", err)
		}
		for _, id := range ids {
			availability[id] = toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
		}
		return availability
	}
	if source, ok := p.inner.(toolspkg.ProjectionAvailabilityProvider); ok {
		return source.ProjectionAvailability(ctx, canonical, ids)
	}
	loggedFailure := false
	for _, id := range ids {
		handle, found, resolveErr := p.inner.Resolve(ctx, canonical, id)
		switch {
		case resolveErr != nil:
			if !loggedFailure {
				toolspkg.LogBackendFailure(ctx, id, "tool_registry_resolve", resolveErr)
				loggedFailure = true
			}
			availability[id] = toolspkg.Unavailable(toolspkg.ReasonBackendUnhealthy)
		case found && handle != nil:
			availability[id] = handle.Availability(ctx, canonical)
		}
	}
	return availability
}

func (p *daemonExtensionToolProvider) Resolve(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	canonical, err := p.canonicalWorkspaceScope(ctx, scope)
	if err != nil {
		return nil, false, err
	}
	handle, ok, err := p.inner.Resolve(ctx, canonical, id)
	if err != nil || !ok || handle == nil {
		return handle, ok, err
	}
	return &daemonExtensionToolHandle{
		inner:             handle,
		workspaceResolver: p.workspaceResolver,
	}, true, nil
}

// canonicalWorkspaceScope resolves workspace aliases to a current registration ID without loading resources.
func (p *daemonExtensionToolProvider) canonicalWorkspaceScope(
	ctx context.Context,
	scope toolspkg.Scope,
) (toolspkg.Scope, error) {
	workspaceRef := strings.TrimSpace(scope.WorkspaceID)
	if workspaceRef == "" {
		return scope, nil
	}
	if p.workspaceResolver == nil {
		return toolspkg.Scope{}, fmt.Errorf(
			"daemon: extension tool workspace %q cannot be resolved: %w",
			workspaceRef,
			workspacepkg.ErrWorkspaceResolverUnavailable,
		)
	}
	resolved, err := p.workspaceResolver.ResolveRegistration(ctx, workspaceRef)
	if err != nil {
		return toolspkg.Scope{}, fmt.Errorf(
			"daemon: resolve extension tool workspace %q: %w",
			workspaceRef,
			err,
		)
	}
	workspaceID, err := extensionWorkspaceRegistrationID(resolved)
	if err != nil {
		return toolspkg.Scope{}, fmt.Errorf(
			"daemon: resolved extension tool workspace %q has no registered runtime id: %w",
			workspaceRef,
			err,
		)
	}
	scope.WorkspaceID = workspaceID
	return scope, nil
}

type daemonExtensionToolHandle struct {
	inner             toolspkg.Handle
	workspaceResolver daemonExtensionWorkspaceResolver
}

var _ toolspkg.Handle = (*daemonExtensionToolHandle)(nil)

func (h *daemonExtensionToolHandle) Descriptor() toolspkg.Descriptor {
	return h.inner.Descriptor()
}

func (h *daemonExtensionToolHandle) Availability(
	ctx context.Context,
	scope toolspkg.Scope,
) toolspkg.Availability {
	return h.inner.Availability(ctx, scope)
}

func (h *daemonExtensionToolHandle) Call(
	ctx context.Context,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	patched, err := h.workspaceScopedCallRequest(ctx, req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return h.inner.Call(ctx, patched)
}

func (h *daemonExtensionToolHandle) workspaceScopedCallRequest(
	ctx context.Context,
	req toolspkg.CallRequest,
) (toolspkg.CallRequest, error) {
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return req, nil
	}
	return h.attachTrustedWorkspace(ctx, req)
}

// attachTrustedWorkspace validates registration and fills a missing trusted root before extension dispatch.
func (h *daemonExtensionToolHandle) attachTrustedWorkspace(
	ctx context.Context,
	req toolspkg.CallRequest,
) (toolspkg.CallRequest, error) {
	if h.workspaceResolver == nil {
		return toolspkg.CallRequest{}, extensionWorkspaceScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q has no workspace resolver", req.ToolID),
			toolspkg.ErrToolInvalidInput,
		)
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return toolspkg.CallRequest{}, extensionWorkspaceScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q requires workspace scope", req.ToolID),
			toolspkg.ErrToolInvalidInput,
		)
	}
	resolved, err := h.workspaceResolver.ResolveRegistration(ctx, workspaceID)
	if err != nil {
		return toolspkg.CallRequest{}, extensionWorkspaceScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q workspace %q is invalid", req.ToolID, workspaceID),
			fmt.Errorf("resolve workspace: %w", err),
		)
	}
	root := strings.TrimSpace(req.TrustedWorkspaceRoot)
	if root == "" {
		root = strings.TrimSpace(resolved.RootDir)
	}
	if root == "" {
		return toolspkg.CallRequest{}, extensionWorkspaceScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q workspace %q has no root directory", req.ToolID, workspaceID),
			toolspkg.ErrToolInvalidInput,
		)
	}
	req.WorkspaceID, err = extensionWorkspaceRegistrationID(resolved)
	if err != nil {
		return toolspkg.CallRequest{}, extensionWorkspaceScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q workspace %q has no registered runtime id", req.ToolID, workspaceID),
			err,
		)
	}
	req.TrustedWorkspaceRoot = root
	return req, nil
}

// extensionWorkspaceRegistrationID rejects empty registry IDs before they can become unscoped requests.
func extensionWorkspaceRegistrationID(ws workspacepkg.Workspace) (string, error) {
	workspaceID := strings.TrimSpace(ws.ID)
	if workspaceID == "" {
		return "", errors.New("daemon: resolved workspace registry id is empty")
	}
	return workspaceID, nil
}

func extensionWorkspaceScopeError(
	toolID toolspkg.ToolID,
	message string,
	cause error,
) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		toolID,
		message,
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, cause),
		toolspkg.ReasonScopeMismatch,
	)
}
