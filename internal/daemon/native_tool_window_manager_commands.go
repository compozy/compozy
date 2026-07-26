package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

func (n *daemonNativeTools) windowManagerCommandRequest(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input windowManagerMutationInput,
	command windowmanager.Command,
) (windowmanager.CommandRequest, error) {
	workspaceID, err := n.windowManagerWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return windowmanager.CommandRequest{}, err
	}
	clientID := windowManagerOptionalString[windowmanager.ClientID](input.ClientID)
	if windowManagerCommandNeedsClient(command.CommandID()) && clientID == nil {
		return windowmanager.CommandRequest{}, nativeRequiredInputError(req.ToolID, "client_id")
	}
	return windowmanager.CommandRequest{
		WorkspaceID:      workspaceID,
		CommandID:        command.CommandID(),
		ExpectedRevision: input.ExpectedRevision,
		ClientID:         clientID,
		Actor:            windowManagerActor(scope),
		Origin:           windowManagerOrigin(req.ToolID, input.Origin),
		Rebase:           input.rebaseGuard(),
		Payload:          command,
	}, nil
}

func windowManagerCommandNeedsClient(commandID windowmanager.CommandID) bool {
	return commandID == windowmanager.CommandDesktopSwitch ||
		commandID == windowmanager.CommandWindowFocus ||
		commandID == windowmanager.CommandWindowZoom
}

func (payload windowManagerDesktopCreatePayload) command() windowmanager.Command {
	return windowmanager.CreateDesktopCommand{
		DesktopID:  windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID)),
		Name:       strings.TrimSpace(payload.Name),
		Purpose:    windowmanager.DesktopPurpose(strings.TrimSpace(payload.Purpose)),
		FocusOwner: windowManagerOptionalString[windowmanager.WindowID](payload.FocusOwner),
		AfterID:    windowManagerOptionalString[windowmanager.DesktopID](payload.AfterID),
	}
}

func (payload windowManagerDesktopUpdatePayload) command() windowmanager.Command {
	return windowmanager.UpdateDesktopCommand{
		DesktopID: windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID)),
		Name:      strings.TrimSpace(payload.Name),
	}
}

func (payload windowManagerDesktopReorderPayload) command() windowmanager.Command {
	return windowmanager.ReorderDesktopCommand{
		DesktopID: windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID)),
		Order:     payload.Order,
	}
}

func (payload windowManagerDesktopSwitchPayload) command() windowmanager.Command {
	return windowmanager.SwitchDesktopCommand{DesktopID: windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID))}
}

func (payload windowManagerDesktopDeletePayload) command() windowmanager.Command {
	return windowmanager.DeleteDesktopCommand{
		DesktopID:     windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID)),
		DestinationID: windowManagerOptionalString[windowmanager.DesktopID](payload.DestinationID),
	}
}

func (payload windowManagerWindowOpenPayload) command() windowmanager.Command {
	var rect windowmanager.NormalizedRect
	if payload.FloatingRect != nil {
		rect = *payload.FloatingRect
	}
	var route windowmanager.RouteIntent
	if payload.Route != nil {
		route = *payload.Route
	}
	return windowmanager.OpenWindowCommand{
		Window: windowmanager.WindowSpec{
			ID:           windowmanager.WindowID(strings.TrimSpace(payload.WindowID)),
			App:          strings.TrimSpace(payload.App),
			InstanceKey:  windowManagerOptionalString[string](payload.InstanceKey),
			Route:        route,
			DesktopID:    windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID)),
			FloatingRect: rect,
			InsertTiled:  payload.InsertTiled,
		},
		RestoreWindowID: windowManagerOptionalString[windowmanager.WindowID](payload.RestoreWindowID),
	}
}

func (payload windowManagerWindowNavigatePayload) command() windowmanager.Command {
	return windowmanager.NavigateWindowCommand{
		WindowID: windowmanager.WindowID(strings.TrimSpace(payload.WindowID)),
		Route:    payload.Route,
	}
}

func (payload windowManagerWindowClosePayload) command() windowmanager.Command {
	return windowmanager.CloseWindowCommand{
		WindowID: windowmanager.WindowID(strings.TrimSpace(payload.WindowID)),
		Minimize: payload.Minimize,
	}
}

func (payload windowManagerWindowFocusPayload) command() windowmanager.Command {
	return windowmanager.FocusWindowCommand{
		WindowID:  windowManagerOptionalString[windowmanager.WindowID](payload.WindowID),
		Direction: windowmanager.FocusDirection(strings.TrimSpace(payload.Direction)),
	}
}

func (payload windowManagerWindowMovePayload) command() windowmanager.Command {
	return windowmanager.MoveWindowCommand{
		WindowID:             windowmanager.WindowID(strings.TrimSpace(payload.WindowID)),
		DestinationDesktopID: windowmanager.DesktopID(strings.TrimSpace(payload.DestinationDesktopID)),
		TargetWindowID:       windowManagerOptionalString[windowmanager.WindowID](payload.TargetWindowID),
		Placement:            windowmanager.DropPlacement(strings.TrimSpace(payload.Placement)),
		FloatingRect:         payload.FloatingRect,
		MoveGroup:            payload.MoveGroup,
	}
}

func (payload windowManagerWindowSwapPayload) command() windowmanager.Command {
	return windowmanager.SwapWindowsCommand{
		FirstWindowID:  windowmanager.WindowID(strings.TrimSpace(payload.FirstWindowID)),
		SecondWindowID: windowmanager.WindowID(strings.TrimSpace(payload.SecondWindowID)),
	}
}

func (payload windowManagerWindowFloatPayload) command() windowmanager.Command {
	return windowmanager.ToggleFloatingCommand{
		WindowID:     windowmanager.WindowID(strings.TrimSpace(payload.WindowID)),
		FloatingRect: payload.FloatingRect,
	}
}

func (payload windowManagerWindowZoomPayload) command() windowmanager.Command {
	return windowmanager.ZoomWindowCommand{WindowID: windowmanager.WindowID(strings.TrimSpace(payload.WindowID))}
}

func (payload windowManagerLayoutArrangePayload) command() windowmanager.Command {
	var frame windowmanager.NormalizedRect
	if payload.Frame != nil {
		frame = *payload.Frame
	}
	return windowmanager.ArrangeLayoutCommand{
		DesktopID:   windowmanager.DesktopID(strings.TrimSpace(payload.DesktopID)),
		WindowIDs:   windowManagerStringIDs[windowmanager.WindowID](payload.WindowIDs),
		Arrangement: windowmanager.Arrangement(strings.TrimSpace(payload.Arrangement)),
		Frame:       frame,
		GroupID:     windowmanager.GroupID(strings.TrimSpace(payload.GroupID)),
		ResourceID:  strings.TrimSpace(payload.ResourceID),
	}
}

func (payload windowManagerLayoutResizePayload) command() windowmanager.Command {
	return windowmanager.ResizeLayoutCommand{
		SplitID:       windowmanager.NodeID(strings.TrimSpace(payload.SplitID)),
		BoundaryIndex: payload.BoundaryIndex,
		Delta:         payload.Delta,
	}
}

func (payload windowManagerLayoutBalancePayload) command() windowmanager.Command {
	return windowmanager.BalanceLayoutCommand{
		GroupID: windowManagerOptionalString[windowmanager.GroupID](payload.GroupID),
		SplitID: windowManagerOptionalString[windowmanager.NodeID](payload.SplitID),
	}
}

func decodeWindowManagerPreviewCommand(
	id toolspkg.ToolID,
	commandID windowmanager.CommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case windowmanager.CommandDesktopCreate,
		windowmanager.CommandDesktopUpdate,
		windowmanager.CommandDesktopReorder,
		windowmanager.CommandDesktopSwitch,
		windowmanager.CommandDesktopDelete:
		return decodeWindowManagerDesktopPreviewCommand(id, commandID, raw)
	case windowmanager.CommandWindowOpen,
		windowmanager.CommandWindowNavigate,
		windowmanager.CommandWindowClose,
		windowmanager.CommandWindowFocus,
		windowmanager.CommandWindowMove,
		windowmanager.CommandWindowSwap,
		windowmanager.CommandWindowToggleFloating,
		windowmanager.CommandWindowZoom:
		return decodeWindowManagerWindowPreviewCommand(id, commandID, raw)
	case windowmanager.CommandLayoutArrange,
		windowmanager.CommandLayoutResize,
		windowmanager.CommandLayoutBalance,
		windowmanager.CommandLayoutUndo,
		windowmanager.CommandLayoutRedo:
		return decodeWindowManagerLayoutPreviewCommand(id, commandID, raw)
	default:
		return unsupportedWindowManagerPreviewCommand(id, commandID)
	}
}

func decodeWindowManagerDesktopPreviewCommand(
	id toolspkg.ToolID,
	commandID windowmanager.CommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case windowmanager.CommandDesktopCreate:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerDesktopCreatePayload.command)
	case windowmanager.CommandDesktopUpdate:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerDesktopUpdatePayload.command)
	case windowmanager.CommandDesktopReorder:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerDesktopReorderPayload.command)
	case windowmanager.CommandDesktopSwitch:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerDesktopSwitchPayload.command)
	case windowmanager.CommandDesktopDelete:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerDesktopDeletePayload.command)
	default:
		return unsupportedWindowManagerPreviewCommand(id, commandID)
	}
}

func decodeWindowManagerWindowPreviewCommand(
	id toolspkg.ToolID,
	commandID windowmanager.CommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case windowmanager.CommandWindowOpen:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowOpenPayload.command)
	case windowmanager.CommandWindowNavigate:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowNavigatePayload.command)
	case windowmanager.CommandWindowClose:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowClosePayload.command)
	case windowmanager.CommandWindowFocus:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowFocusPayload.command)
	case windowmanager.CommandWindowMove:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowMovePayload.command)
	case windowmanager.CommandWindowSwap:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowSwapPayload.command)
	case windowmanager.CommandWindowToggleFloating:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowFloatPayload.command)
	case windowmanager.CommandWindowZoom:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerWindowZoomPayload.command)
	default:
		return unsupportedWindowManagerPreviewCommand(id, commandID)
	}
}

func decodeWindowManagerLayoutPreviewCommand(
	id toolspkg.ToolID,
	commandID windowmanager.CommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case windowmanager.CommandLayoutArrange:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerLayoutArrangePayload.command)
	case windowmanager.CommandLayoutResize:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerLayoutResizePayload.command)
	case windowmanager.CommandLayoutBalance:
		return decodeWindowManagerPreviewPayload(id, raw, windowManagerLayoutBalancePayload.command)
	case windowmanager.CommandLayoutUndo:
		return decodeWindowManagerEmptyPreviewCommand(id, raw, windowmanager.UndoLayoutCommand{})
	case windowmanager.CommandLayoutRedo:
		return decodeWindowManagerEmptyPreviewCommand(id, raw, windowmanager.RedoLayoutCommand{})
	default:
		return unsupportedWindowManagerPreviewCommand(id, commandID)
	}
}

func decodeWindowManagerEmptyPreviewCommand(
	id toolspkg.ToolID,
	raw json.RawMessage,
	command windowmanager.Command,
) (windowmanager.Command, error) {
	var payload struct{}
	if err := decodeWindowManagerJSON(id, raw, &payload); err != nil {
		return nil, err
	}
	return command, nil
}

func unsupportedWindowManagerPreviewCommand(
	id toolspkg.ToolID,
	commandID windowmanager.CommandID,
) (windowmanager.Command, error) {
	return nil, windowManagerInvalidInput(id, fmt.Errorf("unsupported command_id %q", commandID))
}

func decodeWindowManagerPreviewPayload[T any](
	id toolspkg.ToolID,
	raw json.RawMessage,
	build func(T) windowmanager.Command,
) (windowmanager.Command, error) {
	var payload T
	if err := decodeWindowManagerJSON(id, raw, &payload); err != nil {
		return nil, err
	}
	return build(payload), nil
}
