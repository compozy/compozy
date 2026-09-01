package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	specCycleImportTasksToolID          toolspkg.ToolID = "ext__spec_cycle__import_tasks"
	specCycleWriteReviewArtifactsToolID toolspkg.ToolID = "ext__spec_cycle__write_review_artifacts"
	specCycleFinalizeReviewRoundToolID  toolspkg.ToolID = "ext__spec_cycle__finalize_review_round"
)

type daemonExtensionToolProvider struct {
	inner             toolspkg.Provider
	workspaceResolver workspacepkg.RuntimeResolver
}

var _ toolspkg.Provider = (*daemonExtensionToolProvider)(nil)
var _ toolspkg.ProjectionGenerationProvider = (*daemonExtensionToolProvider)(nil)

func newDaemonScopedExtensionToolProvider(
	inner toolspkg.Provider,
	workspaceResolver workspacepkg.RuntimeResolver,
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
	resolved, err := p.workspaceResolver.Resolve(ctx, workspaceRef)
	if err != nil {
		return toolspkg.Scope{}, fmt.Errorf(
			"daemon: resolve extension tool workspace %q: %w",
			workspaceRef,
			err,
		)
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
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
	workspaceResolver workspacepkg.RuntimeResolver
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
	switch req.ToolID {
	case specCycleImportTasksToolID:
		return h.workspaceScopedImportTasksCallRequest(ctx, req)
	case specCycleWriteReviewArtifactsToolID, specCycleFinalizeReviewRoundToolID:
		return h.attachTrustedWorkspace(ctx, req)
	default:
		if strings.TrimSpace(req.WorkspaceID) == "" {
			return req, nil
		}
		return h.attachTrustedWorkspace(ctx, req)
	}
}

func (h *daemonExtensionToolHandle) workspaceScopedImportTasksCallRequest(
	ctx context.Context,
	req toolspkg.CallRequest,
) (toolspkg.CallRequest, error) {
	scoped, err := h.attachTrustedWorkspace(ctx, req)
	if err != nil {
		return toolspkg.CallRequest{}, err
	}
	req = scoped
	var input struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return toolspkg.CallRequest{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			fmt.Sprintf("tool %q input is invalid", req.ToolID),
			fmt.Errorf("%w: decode input: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	pattern := strings.TrimSpace(input.Pattern)
	if pattern == "" {
		return toolspkg.CallRequest{}, importTasksScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q pattern is required", req.ToolID),
			fmt.Errorf("%w: pattern is required", toolspkg.ErrToolInvalidInput),
		)
	}
	if filepath.IsAbs(pattern) {
		return toolspkg.CallRequest{}, importTasksScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q pattern must be workspace-relative", req.ToolID),
			fmt.Errorf("%w: absolute pattern %q", toolspkg.ErrToolInvalidInput, pattern),
		)
	}
	absolute, err := h.resolveImportTasksPattern(req.ToolID, req.TrustedWorkspaceRoot, pattern)
	if err != nil {
		return toolspkg.CallRequest{}, err
	}
	input.Pattern = absolute
	encoded, err := json.Marshal(input)
	if err != nil {
		return toolspkg.CallRequest{}, fmt.Errorf("daemon: encode workspace-scoped extension tool input: %w", err)
	}
	req.Input = encoded
	return req, nil
}

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
	resolved, err := h.workspaceResolver.Resolve(ctx, workspaceID)
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
	req.WorkspaceID, err = nativeResolvedRegistryWorkspaceID(&resolved)
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

func (h *daemonExtensionToolHandle) resolveImportTasksPattern(
	toolID toolspkg.ToolID,
	root string,
	pattern string,
) (string, error) {
	absolute, err := workspaceRelativePattern(root, pattern)
	if err != nil {
		return "", toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolID,
			fmt.Sprintf("tool %q pattern escapes workspace root", toolID),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonScopeMismatch,
		)
	}
	return absolute, nil
}

func importTasksScopeError(
	toolID toolspkg.ToolID,
	message string,
	cause error,
) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		toolID,
		message,
		cause,
		toolspkg.ReasonScopeMismatch,
	)
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
