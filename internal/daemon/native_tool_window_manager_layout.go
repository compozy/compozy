package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

func (n *daemonNativeTools) layoutGet(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWorkspaceInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.windowManagerSnapshot(ctx, scope, req.ToolID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := windowManagerSnapshotResult{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		Snapshot:    snapshot,
	}
	return structuredResult(payload, fmt.Sprintf("read layout at revision %d", snapshot.Revision))
}

func (n *daemonNativeTools) layoutPreview(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutPreviewInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	command, err := decodeWindowManagerPreviewCommand(req.ToolID, input.CommandID, input.Payload)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.previewWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, command)
}

func (n *daemonNativeTools) layoutArrange(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutArrangeInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) layoutResize(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutResizeInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) layoutBalance(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutBalanceInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(ctx, scope, req, input.windowManagerMutationInput, input.command())
}

func (n *daemonNativeTools) layoutUndo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutHistoryInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(
		ctx,
		scope,
		req,
		input.windowManagerMutationInput,
		windowmanager.UndoLayoutCommand{},
	)
}

func (n *daemonNativeTools) layoutRedo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutHistoryInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.executeWindowManagerCommand(
		ctx,
		scope,
		req,
		input.windowManagerMutationInput,
		windowmanager.RedoLayoutCommand{},
	)
}

func (n *daemonNativeTools) layoutExport(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerWorkspaceInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.windowManagerSnapshot(ctx, scope, req.ToolID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := windowManagerExportResult{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		Document:    windowManagerLayoutDocument(snapshot),
	}
	return structuredResult(payload, fmt.Sprintf("exported layout at revision %d", snapshot.Revision))
}

func (n *daemonNativeTools) layoutValidate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutDocumentInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceRef := firstNonEmpty(input.WorkspaceID, string(input.Document.WorkspaceID))
	snapshot, err := n.windowManagerSnapshot(ctx, scope, req.ToolID, workspaceRef)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	validation, err := n.windowManagerService().ValidateLayout(ctx, snapshot.WorkspaceID, input.Document)
	if err != nil {
		return toolspkg.ToolResult{}, windowManagerToolError(req.ToolID, err)
	}
	payload := windowManagerValidationResult{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		Valid:       validation.Valid,
		Diagnostics: append([]windowmanager.Diagnostic{}, validation.Diagnostics...),
	}
	return structuredResult(payload, fmt.Sprintf("validated layout with %d diagnostics", len(payload.Diagnostics)))
}

func (n *daemonNativeTools) layoutApply(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input windowManagerLayoutApplyInput
	if err := decodeWindowManagerInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service := n.windowManagerService()
	if service == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "window manager is unavailable")
	}
	workspaceRef := firstNonEmpty(input.WorkspaceID, string(input.Document.WorkspaceID))
	workspaceID, err := n.windowManagerWorkspaceID(ctx, req.ToolID, workspaceRef, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := service.ReplaceLayout(ctx, windowmanager.ReplaceLayoutRequest{
		WorkspaceID:      workspaceID,
		ExpectedRevision: input.ExpectedRevision,
		ClientID:         windowManagerOptionalString[windowmanager.ClientID](input.ClientID),
		Actor:            windowManagerActor(scope),
		Origin:           windowManagerOrigin(req.ToolID, input.Origin),
		Document:         input.Document,
	})
	if err != nil {
		return toolspkg.ToolResult{}, windowManagerToolError(req.ToolID, err)
	}
	payload := newWindowManagerCommandResult(windowmanager.CommandLayoutReplace, result)
	return structuredResult(payload, fmt.Sprintf("applied layout at revision %d", payload.Revision))
}
