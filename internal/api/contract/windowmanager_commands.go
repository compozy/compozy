package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/windowmanager"
)

// WindowManagerCommandID is the stable wire discriminator for semantic commands.
type WindowManagerCommandID string

const (
	WindowManagerCommandDesktopCreate        WindowManagerCommandID = "desktop.create"
	WindowManagerCommandDesktopUpdate        WindowManagerCommandID = "desktop.update"
	WindowManagerCommandDesktopReorder       WindowManagerCommandID = "desktop.reorder"
	WindowManagerCommandDesktopSwitch        WindowManagerCommandID = "desktop.switch"
	WindowManagerCommandDesktopDelete        WindowManagerCommandID = "desktop.delete"
	WindowManagerCommandWindowOpen           WindowManagerCommandID = "window.open"
	WindowManagerCommandWindowNavigate       WindowManagerCommandID = "window.navigate"
	WindowManagerCommandWindowClose          WindowManagerCommandID = "window.close"
	WindowManagerCommandWindowFocus          WindowManagerCommandID = "window.focus"
	WindowManagerCommandWindowMove           WindowManagerCommandID = "window.move"
	WindowManagerCommandWindowSwap           WindowManagerCommandID = "window.swap"
	WindowManagerCommandWindowToggleFloating WindowManagerCommandID = "window.toggle_floating"
	WindowManagerCommandWindowZoom           WindowManagerCommandID = "window.zoom"
	WindowManagerCommandLayoutArrange        WindowManagerCommandID = "layout.arrange"
	WindowManagerCommandLayoutResize         WindowManagerCommandID = "layout.resize"
	WindowManagerCommandLayoutBalance        WindowManagerCommandID = "layout.balance"
	WindowManagerCommandLayoutUndo           WindowManagerCommandID = "layout.undo"
	WindowManagerCommandLayoutRedo           WindowManagerCommandID = "layout.redo"
	WindowManagerCommandLayoutReplace        WindowManagerCommandID = "layout.replace"
)

// WindowManagerCommandIDValues returns every supported semantic command id.
func WindowManagerCommandIDValues() []string {
	return []string{
		string(WindowManagerCommandDesktopCreate), string(WindowManagerCommandDesktopUpdate),
		string(WindowManagerCommandDesktopReorder), string(WindowManagerCommandDesktopSwitch),
		string(WindowManagerCommandDesktopDelete), string(WindowManagerCommandWindowOpen),
		string(WindowManagerCommandWindowNavigate), string(WindowManagerCommandWindowClose),
		string(WindowManagerCommandWindowFocus),
		string(WindowManagerCommandWindowMove), string(WindowManagerCommandWindowSwap),
		string(WindowManagerCommandWindowToggleFloating), string(WindowManagerCommandWindowZoom),
		string(WindowManagerCommandLayoutArrange), string(WindowManagerCommandLayoutResize),
		string(WindowManagerCommandLayoutBalance), string(WindowManagerCommandLayoutUndo),
		string(WindowManagerCommandLayoutRedo), string(WindowManagerCommandLayoutReplace),
	}
}

// WindowManagerRebaseGuard proves stale source and target identities remain unambiguous.
type WindowManagerRebaseGuard struct {
	WindowID      *windowmanager.WindowID `json:"window_id,omitempty"`
	SourceNodeID  *windowmanager.NodeID   `json:"source_node_id,omitempty"`
	TargetNodeID  *windowmanager.NodeID   `json:"target_node_id,omitempty"`
	SplitID       *windowmanager.NodeID   `json:"split_id,omitempty"`
	BoundaryIndex *int                    `json:"boundary_index,omitempty"`
}

// WindowManagerCommandRequest binds one discriminated payload to a workspace revision.
type WindowManagerCommandRequest struct {
	WorkspaceID      windowmanager.WorkspaceID `json:"workspace_id"`
	CommandID        WindowManagerCommandID    `json:"command_id"`
	ExpectedRevision *WindowManagerRevision    `json:"expected_revision"`
	ClientID         *windowmanager.ClientID   `json:"client_id,omitempty"`
	Actor            WindowManagerActor        `json:"actor"`
	Origin           string                    `json:"origin"`
	Rebase           *WindowManagerRebaseGuard `json:"rebase,omitempty"`
	Payload          json.RawMessage           `json:"payload"`
}

// Domain validates the path/body identity and decodes the payload selected by command_id.
func (request WindowManagerCommandRequest) Domain(
	pathWorkspace windowmanager.WorkspaceID,
) (windowmanager.CommandRequest, error) {
	workspaceID := windowmanager.WorkspaceID(strings.TrimSpace(string(request.WorkspaceID)))
	if workspaceID == "" || workspaceID != pathWorkspace {
		return windowmanager.CommandRequest{}, fmt.Errorf(
			"workspace_id must match the request path: %w",
			windowmanager.ErrInvalidCommand,
		)
	}
	if request.ExpectedRevision == nil {
		return windowmanager.CommandRequest{}, fmt.Errorf(
			"expected_revision is required: %w",
			windowmanager.ErrInvalidCommand,
		)
	}
	if request.ClientID != nil && strings.TrimSpace(string(*request.ClientID)) == "" {
		return windowmanager.CommandRequest{}, fmt.Errorf(
			"client_id must not be empty: %w",
			windowmanager.ErrInvalidCommand,
		)
	}
	payload, err := decodeWindowManagerCommandPayload(request.CommandID, request.Payload)
	if err != nil {
		return windowmanager.CommandRequest{}, err
	}
	var rebase *windowmanager.RebaseGuard
	if request.Rebase != nil {
		rebase = &windowmanager.RebaseGuard{
			WindowID: request.Rebase.WindowID, SourceNodeID: request.Rebase.SourceNodeID,
			TargetNodeID: request.Rebase.TargetNodeID, SplitID: request.Rebase.SplitID,
			BoundaryIndex: request.Rebase.BoundaryIndex,
		}
	}
	return windowmanager.CommandRequest{
		WorkspaceID:      workspaceID,
		CommandID:        windowmanager.CommandID(request.CommandID),
		ExpectedRevision: windowmanager.Revision(*request.ExpectedRevision),
		ClientID:         request.ClientID,
		Actor: windowmanager.Actor{
			Kind: strings.TrimSpace(request.Actor.Kind),
			ID:   strings.TrimSpace(request.Actor.ID),
		},
		Origin:  strings.TrimSpace(request.Origin),
		Rebase:  rebase,
		Payload: payload,
	}, nil
}

func decodeWindowManagerCommandPayload(
	commandID WindowManagerCommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("payload is required: %w", windowmanager.ErrInvalidCommand)
	}

	command, known, err := decodeKnownWindowManagerCommandPayload(commandID, raw)
	if !known {
		return nil, fmt.Errorf("unknown command_id %q: %w", commandID, windowmanager.ErrInvalidCommand)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"decode payload for %q: %w",
			commandID,
			errors.Join(windowmanager.ErrInvalidCommand, err),
		)
	}
	if command.CommandID() != windowmanager.CommandID(commandID) {
		return nil, fmt.Errorf("payload discriminator mismatch: %w", windowmanager.ErrInvalidCommand)
	}
	return command, nil
}

func decodeKnownWindowManagerCommandPayload(
	commandID WindowManagerCommandID,
	raw json.RawMessage,
) (windowmanager.Command, bool, error) {
	switch commandID {
	case WindowManagerCommandDesktopCreate,
		WindowManagerCommandDesktopUpdate,
		WindowManagerCommandDesktopReorder,
		WindowManagerCommandDesktopSwitch,
		WindowManagerCommandDesktopDelete:
		command, err := decodeDesktopCommandPayload(commandID, raw)
		return command, true, err
	case WindowManagerCommandWindowOpen,
		WindowManagerCommandWindowNavigate,
		WindowManagerCommandWindowClose,
		WindowManagerCommandWindowFocus,
		WindowManagerCommandWindowMove,
		WindowManagerCommandWindowSwap,
		WindowManagerCommandWindowToggleFloating,
		WindowManagerCommandWindowZoom:
		command, err := decodeWindowCommandPayload(commandID, raw)
		return command, true, err
	case WindowManagerCommandLayoutArrange,
		WindowManagerCommandLayoutResize,
		WindowManagerCommandLayoutBalance,
		WindowManagerCommandLayoutUndo,
		WindowManagerCommandLayoutRedo,
		WindowManagerCommandLayoutReplace:
		command, err := decodeLayoutCommandPayload(commandID, raw)
		return command, true, err
	default:
		return nil, false, nil
	}
}

func decodeDesktopCommandPayload(
	commandID WindowManagerCommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case WindowManagerCommandDesktopCreate:
		return decodeCreateDesktopPayload(raw)
	case WindowManagerCommandDesktopUpdate:
		return decodeUpdateDesktopPayload(raw)
	case WindowManagerCommandDesktopReorder:
		return decodeReorderDesktopPayload(raw)
	case WindowManagerCommandDesktopSwitch:
		return decodeSwitchDesktopPayload(raw)
	case WindowManagerCommandDesktopDelete:
		return decodeDeleteDesktopPayload(raw)
	default:
		return nil, fmt.Errorf(
			"unknown desktop command_id %q: %w",
			commandID,
			windowmanager.ErrInvalidCommand,
		)
	}
}

func decodeWindowCommandPayload(
	commandID WindowManagerCommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case WindowManagerCommandWindowOpen:
		return decodeOpenWindowPayload(raw)
	case WindowManagerCommandWindowNavigate:
		return decodeNavigateWindowPayload(raw)
	case WindowManagerCommandWindowClose:
		return decodeCloseWindowPayload(raw)
	case WindowManagerCommandWindowFocus:
		return decodeFocusWindowPayload(raw)
	case WindowManagerCommandWindowMove:
		return decodeMoveWindowPayload(raw)
	case WindowManagerCommandWindowSwap:
		return decodeSwapWindowsPayload(raw)
	case WindowManagerCommandWindowToggleFloating:
		return decodeToggleFloatingPayload(raw)
	case WindowManagerCommandWindowZoom:
		return decodeZoomWindowPayload(raw)
	default:
		return nil, fmt.Errorf(
			"unknown window command_id %q: %w",
			commandID,
			windowmanager.ErrInvalidCommand,
		)
	}
}

func decodeLayoutCommandPayload(
	commandID WindowManagerCommandID,
	raw json.RawMessage,
) (windowmanager.Command, error) {
	switch commandID {
	case WindowManagerCommandLayoutArrange:
		return decodeArrangeLayoutPayload(raw)
	case WindowManagerCommandLayoutResize:
		return decodeResizeLayoutPayload(raw)
	case WindowManagerCommandLayoutBalance:
		return decodeBalanceLayoutPayload(raw)
	case WindowManagerCommandLayoutUndo:
		return decodeUndoLayoutPayload(raw)
	case WindowManagerCommandLayoutRedo:
		return decodeRedoLayoutPayload(raw)
	case WindowManagerCommandLayoutReplace:
		return decodeReplaceLayoutPayload(raw)
	default:
		return nil, fmt.Errorf(
			"unknown layout command_id %q: %w",
			commandID,
			windowmanager.ErrInvalidCommand,
		)
	}
}
