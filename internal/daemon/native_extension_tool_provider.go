package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const devCycleImportTasksToolID toolspkg.ToolID = "ext__dev_cycle__import_tasks"

type daemonExtensionToolProvider struct {
	inner             toolspkg.Provider
	workspaceResolver workspacepkg.RuntimeResolver
}

var _ toolspkg.Provider = (*daemonExtensionToolProvider)(nil)

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

func (p *daemonExtensionToolProvider) List(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.Descriptor, error) {
	return p.inner.List(ctx, scope)
}

func (p *daemonExtensionToolProvider) Resolve(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	handle, ok, err := p.inner.Resolve(ctx, scope, id)
	if err != nil || !ok || handle == nil {
		return handle, ok, err
	}
	return &daemonExtensionToolHandle{
		inner:             handle,
		workspaceResolver: p.workspaceResolver,
	}, true, nil
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
	if req.ToolID != devCycleImportTasksToolID {
		return req, nil
	}
	if h.workspaceResolver == nil {
		return toolspkg.CallRequest{}, importTasksScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q has no workspace resolver", req.ToolID),
			toolspkg.ErrToolInvalidInput,
		)
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return toolspkg.CallRequest{}, importTasksScopeError(
			req.ToolID,
			fmt.Sprintf("tool %q requires workspace scope", req.ToolID),
			toolspkg.ErrToolInvalidInput,
		)
	}
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
	absolute, err := h.resolveImportTasksPattern(ctx, req.ToolID, workspaceID, pattern)
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

func (h *daemonExtensionToolHandle) resolveImportTasksPattern(
	ctx context.Context,
	toolID toolspkg.ToolID,
	workspaceID string,
	pattern string,
) (string, error) {
	resolved, err := h.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return "", toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolID,
			fmt.Sprintf("tool %q workspace %q is invalid", toolID, workspaceID),
			fmt.Errorf("%w: resolve workspace: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonScopeMismatch,
		)
	}
	root := strings.TrimSpace(resolved.RootDir)
	if root == "" {
		return "", toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolID,
			fmt.Sprintf("tool %q workspace %q has no root directory", toolID, workspaceID),
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonScopeMismatch,
		)
	}
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
