package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

type windowManagerWorkspaceInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type windowManagerMutationInput struct {
	WorkspaceID      string                    `json:"workspace_id,omitempty"`
	ExpectedRevision windowmanager.Revision    `json:"expected_revision"`
	ClientID         string                    `json:"client_id,omitempty"`
	Origin           string                    `json:"origin,omitempty"`
	Rebase           *windowManagerRebaseInput `json:"rebase,omitempty"`
}

type windowManagerRebaseInput struct {
	WindowID      string `json:"window_id,omitempty"`
	SourceNodeID  string `json:"source_node_id,omitempty"`
	TargetNodeID  string `json:"target_node_id,omitempty"`
	SplitID       string `json:"split_id,omitempty"`
	BoundaryIndex *int   `json:"boundary_index,omitempty"`
}

type windowManagerDesktopCreatePayload struct {
	DesktopID  string `json:"desktop_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
	FocusOwner string `json:"focus_owner,omitempty"`
	AfterID    string `json:"after_id,omitempty"`
}

type windowManagerDesktopCreateInput struct {
	windowManagerMutationInput
	windowManagerDesktopCreatePayload
}

type windowManagerDesktopUpdatePayload struct {
	DesktopID string `json:"desktop_id"`
	Name      string `json:"name"`
}

type windowManagerDesktopUpdateInput struct {
	windowManagerMutationInput
	windowManagerDesktopUpdatePayload
}

type windowManagerDesktopReorderPayload struct {
	DesktopID string `json:"desktop_id"`
	Order     int    `json:"order"`
}

type windowManagerDesktopReorderInput struct {
	windowManagerMutationInput
	windowManagerDesktopReorderPayload
}

type windowManagerDesktopSwitchPayload struct {
	DesktopID string `json:"desktop_id"`
}

type windowManagerDesktopSwitchInput struct {
	windowManagerMutationInput
	windowManagerDesktopSwitchPayload
}

type windowManagerDesktopDeletePayload struct {
	DesktopID     string `json:"desktop_id"`
	DestinationID string `json:"destination_id,omitempty"`
}

type windowManagerDesktopDeleteInput struct {
	windowManagerMutationInput
	windowManagerDesktopDeletePayload
}

type windowManagerWindowListInput struct {
	WorkspaceID      string `json:"workspace_id,omitempty"`
	DesktopID        string `json:"desktop_id,omitempty"`
	IncludeMinimized bool   `json:"include_minimized,omitempty"`
}

type windowManagerWindowOpenPayload struct {
	WindowID        string                        `json:"window_id,omitempty"`
	App             string                        `json:"app,omitempty"`
	InstanceKey     string                        `json:"instance_key,omitempty"`
	DesktopID       string                        `json:"desktop_id,omitempty"`
	Route           *windowmanager.RouteIntent    `json:"route,omitempty"`
	FloatingRect    *windowmanager.NormalizedRect `json:"floating_rect,omitempty"`
	InsertTiled     bool                          `json:"insert_tiled,omitempty"`
	RestoreWindowID string                        `json:"restore_window_id,omitempty"`
}

type windowManagerWindowNavigatePayload struct {
	WindowID string                    `json:"window_id"`
	Route    windowmanager.RouteIntent `json:"route"`
}

type windowManagerWindowNavigateInput struct {
	windowManagerMutationInput
	windowManagerWindowNavigatePayload
}

type windowManagerWindowOpenInput struct {
	windowManagerMutationInput
	windowManagerWindowOpenPayload
}

type windowManagerWindowClosePayload struct {
	WindowID string `json:"window_id"`
	Minimize bool   `json:"minimize,omitempty"`
}

type windowManagerWindowCloseInput struct {
	windowManagerMutationInput
	windowManagerWindowClosePayload
}

type windowManagerWindowFocusPayload struct {
	WindowID  string `json:"window_id,omitempty"`
	Direction string `json:"direction,omitempty"`
}

type windowManagerWindowFocusInput struct {
	windowManagerMutationInput
	windowManagerWindowFocusPayload
}

type windowManagerWindowMovePayload struct {
	WindowID             string                        `json:"window_id"`
	DestinationDesktopID string                        `json:"destination_desktop_id"`
	TargetWindowID       string                        `json:"target_window_id,omitempty"`
	Placement            string                        `json:"placement,omitempty"`
	FloatingRect         *windowmanager.NormalizedRect `json:"floating_rect,omitempty"`
	MoveGroup            bool                          `json:"move_group,omitempty"`
}

func (payload windowManagerWindowMovePayload) validate() error {
	if !payload.MoveGroup {
		if strings.TrimSpace(payload.Placement) == "" {
			return errors.New("placement is required unless move_group is true")
		}
		return nil
	}
	if strings.TrimSpace(payload.TargetWindowID) != "" {
		return errors.New("move_group cannot be combined with target_window_id")
	}
	if strings.TrimSpace(payload.Placement) != "" {
		return errors.New("move_group cannot be combined with placement")
	}
	if payload.FloatingRect != nil {
		return errors.New("move_group cannot be combined with floating_rect")
	}
	return nil
}

type windowManagerWindowMoveInput struct {
	windowManagerMutationInput
	windowManagerWindowMovePayload
}

type windowManagerWindowSwapPayload struct {
	FirstWindowID  string `json:"first_window_id"`
	SecondWindowID string `json:"second_window_id"`
}

type windowManagerWindowSwapInput struct {
	windowManagerMutationInput
	windowManagerWindowSwapPayload
}

type windowManagerWindowFloatPayload struct {
	WindowID     string                        `json:"window_id"`
	FloatingRect *windowmanager.NormalizedRect `json:"floating_rect,omitempty"`
}

type windowManagerWindowFloatInput struct {
	windowManagerMutationInput
	windowManagerWindowFloatPayload
}

type windowManagerWindowZoomPayload struct {
	WindowID string `json:"window_id"`
}

type windowManagerWindowZoomInput struct {
	windowManagerMutationInput
	windowManagerWindowZoomPayload
}

type windowManagerLayoutPreviewInput struct {
	windowManagerMutationInput
	CommandID windowmanager.CommandID `json:"command_id"`
	Payload   json.RawMessage         `json:"payload"`
}

type windowManagerLayoutArrangePayload struct {
	DesktopID   string                        `json:"desktop_id"`
	WindowIDs   []string                      `json:"window_ids"`
	Arrangement string                        `json:"arrangement"`
	Frame       *windowmanager.NormalizedRect `json:"frame,omitempty"`
	GroupID     string                        `json:"group_id,omitempty"`
	ResourceID  string                        `json:"resource_id,omitempty"`
}

type windowManagerLayoutArrangeInput struct {
	windowManagerMutationInput
	windowManagerLayoutArrangePayload
}

type windowManagerLayoutResizePayload struct {
	SplitID       string  `json:"split_id"`
	BoundaryIndex int     `json:"boundary_index"`
	Delta         float64 `json:"delta"`
}

type windowManagerLayoutResizeInput struct {
	windowManagerMutationInput
	windowManagerLayoutResizePayload
}

type windowManagerLayoutBalancePayload struct {
	GroupID string `json:"group_id,omitempty"`
	SplitID string `json:"split_id,omitempty"`
}

type windowManagerLayoutBalanceInput struct {
	windowManagerMutationInput
	windowManagerLayoutBalancePayload
}

type windowManagerLayoutHistoryInput struct {
	windowManagerMutationInput
}

type windowManagerLayoutDocumentInput struct {
	WorkspaceID string                       `json:"workspace_id,omitempty"`
	Document    windowmanager.LayoutDocument `json:"document"`
}

type windowManagerLayoutApplyInput struct {
	WorkspaceID      string                       `json:"workspace_id,omitempty"`
	ExpectedRevision windowmanager.Revision       `json:"expected_revision"`
	ClientID         string                       `json:"client_id,omitempty"`
	Origin           string                       `json:"origin,omitempty"`
	Document         windowmanager.LayoutDocument `json:"document"`
}

func decodeWindowManagerInput(req toolspkg.CallRequest, destination any) error {
	return decodeWindowManagerJSON(req.ToolID, req.Input, destination)
}

func decodeWindowManagerJSON(id toolspkg.ToolID, raw json.RawMessage, destination any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return windowManagerInvalidInput(id, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return windowManagerInvalidInput(id, err)
	}
	return nil
}

func windowManagerInvalidInput(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		fmt.Sprintf("tool %q input is invalid: %v", id, err),
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonSchemaInvalid,
	)
}

func windowManagerOptionalString[T ~string](value string) *T {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	converted := T(trimmed)
	return &converted
}

func (input windowManagerMutationInput) rebaseGuard() *windowmanager.RebaseGuard {
	if input.Rebase == nil {
		return nil
	}
	return &windowmanager.RebaseGuard{
		WindowID:      windowManagerOptionalString[windowmanager.WindowID](input.Rebase.WindowID),
		SourceNodeID:  windowManagerOptionalString[windowmanager.NodeID](input.Rebase.SourceNodeID),
		TargetNodeID:  windowManagerOptionalString[windowmanager.NodeID](input.Rebase.TargetNodeID),
		SplitID:       windowManagerOptionalString[windowmanager.NodeID](input.Rebase.SplitID),
		BoundaryIndex: input.Rebase.BoundaryIndex,
	}
}

func windowManagerStringIDs[T ~string](values []string) []T {
	converted := make([]T, 0, len(values))
	for _, value := range values {
		converted = append(converted, T(strings.TrimSpace(value)))
	}
	return converted
}
