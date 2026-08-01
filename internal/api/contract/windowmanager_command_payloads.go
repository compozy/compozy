package contract

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/windowmanager"
)

type WindowManagerCreateDesktopPayload struct {
	DesktopID  windowmanager.DesktopID      `json:"desktop_id"`
	Name       string                       `json:"name"`
	Purpose    windowmanager.DesktopPurpose `json:"purpose"`
	FocusOwner *windowmanager.WindowID      `json:"focus_owner,omitempty"`
	AfterID    *windowmanager.DesktopID     `json:"after_id,omitempty"`
}

type WindowManagerUpdateDesktopPayload struct {
	DesktopID windowmanager.DesktopID `json:"desktop_id"`
	Name      string                  `json:"name"`
}

type WindowManagerReorderDesktopPayload struct {
	DesktopID windowmanager.DesktopID `json:"desktop_id"`
	Order     int                     `json:"order"`
}

type WindowManagerSwitchDesktopPayload struct {
	DesktopID windowmanager.DesktopID `json:"desktop_id"`
}

type WindowManagerDeleteDesktopPayload struct {
	DesktopID     windowmanager.DesktopID  `json:"desktop_id"`
	DestinationID *windowmanager.DesktopID `json:"destination_id,omitempty"`
}

type WindowManagerWindowSpecPayload struct {
	ID                  windowmanager.WindowID       `json:"id,omitempty"`
	App                 string                       `json:"app"`
	InstanceKey         *string                      `json:"instance_key,omitempty"`
	Route               windowmanager.RouteIntent    `json:"route"`
	DesktopID           windowmanager.DesktopID      `json:"desktop_id"`
	FloatingRect        windowmanager.NormalizedRect `json:"floating_rect"`
	InsertTiled         bool                         `json:"insert_tiled"`
	StackTargetWindowID *windowmanager.WindowID      `json:"stack_target_window_id,omitempty"`
}

type WindowManagerOpenWindowPayload struct {
	Window          WindowManagerWindowSpecPayload `json:"window"`
	RestoreWindowID *windowmanager.WindowID        `json:"restore_window_id,omitempty"`
}

type WindowManagerNavigateWindowPayload struct {
	WindowID windowmanager.WindowID     `json:"window_id"`
	Route    windowmanager.RouteIntent  `json:"route,omitzero"`
	Mode     windowmanager.NavigateMode `json:"mode,omitempty"`
}

type WindowManagerCloseWindowPayload struct {
	WindowID windowmanager.WindowID   `json:"window_id"`
	Minimize bool                     `json:"minimize"`
	Scope    windowmanager.CloseScope `json:"scope,omitempty"`
}

type WindowManagerGroupWindowsPayload struct {
	TargetWindowID windowmanager.WindowID   `json:"target_window_id"`
	WindowIDs      []windowmanager.WindowID `json:"window_ids"`
	InsertIndex    *int                     `json:"insert_index,omitempty"`
}

type WindowManagerReorderStackPayload struct {
	WindowID windowmanager.WindowID `json:"window_id"`
	Index    int                    `json:"index"`
}

type WindowManagerSetStackActivePayload struct {
	WindowID windowmanager.WindowID `json:"window_id"`
}

type WindowManagerPinWindowPayload struct {
	WindowID windowmanager.WindowID `json:"window_id"`
	Pinned   bool                   `json:"pinned"`
}

type WindowManagerReopenWindowPayload struct{}

type WindowManagerFocusWindowPayload struct {
	WindowID  *windowmanager.WindowID      `json:"window_id,omitempty"`
	Direction windowmanager.FocusDirection `json:"direction,omitempty"`
}

type WindowManagerMoveWindowPayload struct {
	WindowID             windowmanager.WindowID        `json:"window_id"`
	DestinationDesktopID windowmanager.DesktopID       `json:"destination_desktop_id"`
	TargetWindowID       *windowmanager.WindowID       `json:"target_window_id,omitempty"`
	Placement            windowmanager.DropPlacement   `json:"placement"`
	FloatingRect         *windowmanager.NormalizedRect `json:"floating_rect,omitempty"`
	MoveGroup            bool                          `json:"move_group"`
}

type WindowManagerResizeWindowPayload struct {
	WindowID windowmanager.WindowID       `json:"window_id"`
	Frame    windowmanager.NormalizedRect `json:"frame"`
}

type WindowManagerSwapWindowsPayload struct {
	FirstWindowID  windowmanager.WindowID `json:"first_window_id"`
	SecondWindowID windowmanager.WindowID `json:"second_window_id"`
}

type WindowManagerToggleFloatingPayload struct {
	WindowID     windowmanager.WindowID        `json:"window_id"`
	FloatingRect *windowmanager.NormalizedRect `json:"floating_rect,omitempty"`
}

type WindowManagerZoomWindowPayload struct {
	WindowID windowmanager.WindowID `json:"window_id"`
}

type WindowManagerArrangeLayoutPayload struct {
	DesktopID   windowmanager.DesktopID      `json:"desktop_id"`
	WindowIDs   []windowmanager.WindowID     `json:"window_ids"`
	Arrangement windowmanager.Arrangement    `json:"arrangement"`
	Frame       windowmanager.NormalizedRect `json:"frame"`
	GroupID     windowmanager.GroupID        `json:"group_id"`
	ResourceID  string                       `json:"resource_id,omitempty"`
}

type WindowManagerResizeLayoutPayload struct {
	SplitID       windowmanager.NodeID `json:"split_id"`
	BoundaryIndex int                  `json:"boundary_index"`
	Delta         float64              `json:"delta"`
}

type WindowManagerGroupFrameEditPayload struct {
	GroupID windowmanager.GroupID        `json:"group_id"`
	Frame   windowmanager.NormalizedRect `json:"frame"`
}

type WindowManagerFrameResizeLayoutPayload struct {
	DesktopID windowmanager.DesktopID              `json:"desktop_id"`
	Edits     []WindowManagerGroupFrameEditPayload `json:"edits"`
}

type WindowManagerBalanceLayoutPayload struct {
	GroupID *windowmanager.GroupID `json:"group_id,omitempty"`
	SplitID *windowmanager.NodeID  `json:"split_id,omitempty"`
}

type WindowManagerUndoLayoutPayload struct{}
type WindowManagerRedoLayoutPayload struct{}

type WindowManagerReplaceLayoutPayload struct {
	Document WindowManagerLayoutDocument `json:"document"`
}

func decodeCreateDesktopPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerCreateDesktopPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.CreateDesktopCommand{
		DesktopID:  payload.DesktopID,
		Name:       payload.Name,
		Purpose:    payload.Purpose,
		FocusOwner: payload.FocusOwner,
		AfterID:    payload.AfterID,
	}, nil
}

func decodeUpdateDesktopPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerUpdateDesktopPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.UpdateDesktopCommand{DesktopID: payload.DesktopID, Name: payload.Name}, nil
}

func decodeReorderDesktopPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerReorderDesktopPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ReorderDesktopCommand{DesktopID: payload.DesktopID, Order: payload.Order}, nil
}

func decodeSwitchDesktopPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerSwitchDesktopPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.SwitchDesktopCommand{DesktopID: payload.DesktopID}, nil
}

func decodeDeleteDesktopPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerDeleteDesktopPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.DeleteDesktopCommand{DesktopID: payload.DesktopID, DestinationID: payload.DestinationID}, nil
}

func decodeOpenWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerOpenWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	spec := windowmanager.WindowSpec{
		ID: payload.Window.ID, App: payload.Window.App, InstanceKey: payload.Window.InstanceKey,
		Route: payload.Window.Route, DesktopID: payload.Window.DesktopID,
		FloatingRect: payload.Window.FloatingRect, InsertTiled: payload.Window.InsertTiled,
		StackTargetWindowID: payload.Window.StackTargetWindowID,
	}
	return windowmanager.OpenWindowCommand{Window: spec, RestoreWindowID: payload.RestoreWindowID}, nil
}

func decodeNavigateWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerNavigateWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Mode == "replace" {
		payload.Mode = windowmanager.NavigateReplace
	}
	return windowmanager.NavigateWindowCommand{
		WindowID: payload.WindowID, Route: payload.Route, Mode: payload.Mode,
	}, nil
}

func decodeCloseWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerCloseWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Scope == "tab" {
		payload.Scope = windowmanager.CloseScopeTab
	}
	return windowmanager.CloseWindowCommand{
		WindowID: payload.WindowID, Minimize: payload.Minimize, Scope: payload.Scope,
	}, nil
}

func decodeGroupWindowsPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerGroupWindowsPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.GroupWindowsCommand{
		TargetWindowID: payload.TargetWindowID, WindowIDs: payload.WindowIDs, InsertIndex: payload.InsertIndex,
	}, nil
}

func decodeReorderStackPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerReorderStackPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ReorderStackCommand{WindowID: payload.WindowID, Index: payload.Index}, nil
}

func decodeSetStackActivePayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerSetStackActivePayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.SetStackActiveCommand{WindowID: payload.WindowID}, nil
}

func decodePinWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerPinWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.PinWindowCommand{WindowID: payload.WindowID, Pinned: payload.Pinned}, nil
}

func decodeReopenWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerReopenWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ReopenCommand{}, nil
}

func decodeFocusWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerFocusWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.FocusWindowCommand{WindowID: payload.WindowID, Direction: payload.Direction}, nil
}

func decodeMoveWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerMoveWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.MoveWindowCommand{
		WindowID:             payload.WindowID,
		DestinationDesktopID: payload.DestinationDesktopID,
		TargetWindowID:       payload.TargetWindowID,
		Placement:            payload.Placement,
		FloatingRect:         payload.FloatingRect,
		MoveGroup:            payload.MoveGroup,
	}, nil
}

func decodeResizeWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerResizeWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ResizeWindowCommand{WindowID: payload.WindowID, Frame: payload.Frame}, nil
}

func decodeSwapWindowsPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerSwapWindowsPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.SwapWindowsCommand{
		FirstWindowID:  payload.FirstWindowID,
		SecondWindowID: payload.SecondWindowID,
	}, nil
}

func decodeToggleFloatingPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerToggleFloatingPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ToggleFloatingCommand{WindowID: payload.WindowID, FloatingRect: payload.FloatingRect}, nil
}

func decodeZoomWindowPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerZoomWindowPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ZoomWindowCommand{WindowID: payload.WindowID}, nil
}

func decodeArrangeLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerArrangeLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ArrangeLayoutCommand{
		DesktopID:   payload.DesktopID,
		WindowIDs:   payload.WindowIDs,
		Arrangement: payload.Arrangement,
		Frame:       payload.Frame,
		GroupID:     payload.GroupID,
		ResourceID:  payload.ResourceID,
	}, nil
}

func decodeResizeLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerResizeLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ResizeLayoutCommand{
		SplitID:       payload.SplitID,
		BoundaryIndex: payload.BoundaryIndex,
		Delta:         payload.Delta,
	}, nil
}

func decodeFrameResizeLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerFrameResizeLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	edits := make([]windowmanager.GroupFrameEdit, 0, len(payload.Edits))
	for _, edit := range payload.Edits {
		edits = append(edits, windowmanager.GroupFrameEdit{GroupID: edit.GroupID, Frame: edit.Frame})
	}
	return windowmanager.FrameResizeLayoutCommand{DesktopID: payload.DesktopID, Edits: edits}, nil
}

func decodeBalanceLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerBalanceLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.BalanceLayoutCommand{GroupID: payload.GroupID, SplitID: payload.SplitID}, nil
}

func decodeUndoLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerUndoLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.UndoLayoutCommand{}, nil
}

func decodeRedoLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerRedoLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.RedoLayoutCommand{}, nil
}

func decodeReplaceLayoutPayload(raw json.RawMessage) (windowmanager.Command, error) {
	var payload WindowManagerReplaceLayoutPayload
	if err := decodeStrictContractJSON(raw, &payload); err != nil {
		return nil, err
	}
	return windowmanager.ReplaceLayoutCommand{Document: payload.Document.Domain()}, nil
}
