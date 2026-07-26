package daemon

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
)

const windowManagerOperatorActor = "operator"

type windowManagerDesktopsResult struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    windowmanager.Revision    `json:"revision"`
	Desktops    []windowmanager.Desktop   `json:"desktops"`
}

type windowManagerWindowsResult struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    windowmanager.Revision    `json:"revision"`
	Windows     []windowmanager.Window    `json:"windows"`
}

type windowManagerClientsResult struct {
	WorkspaceID windowmanager.WorkspaceID  `json:"workspace_id"`
	Revision    windowmanager.Revision     `json:"revision"`
	Clients     []windowmanager.ClientView `json:"clients"`
}

type windowManagerSnapshotResult struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    windowmanager.Revision    `json:"revision"`
	Snapshot    windowmanager.Snapshot    `json:"snapshot"`
}

type windowManagerExportResult struct {
	WorkspaceID windowmanager.WorkspaceID    `json:"workspace_id"`
	Revision    windowmanager.Revision       `json:"revision"`
	Document    windowmanager.LayoutDocument `json:"document"`
}

type windowManagerValidationResult struct {
	WorkspaceID windowmanager.WorkspaceID  `json:"workspace_id"`
	Revision    windowmanager.Revision     `json:"revision"`
	Valid       bool                       `json:"valid"`
	Diagnostics []windowmanager.Diagnostic `json:"diagnostics"`
}

type windowManagerCommandResult struct {
	WorkspaceID windowmanager.WorkspaceID  `json:"workspace_id"`
	CommandID   windowmanager.CommandID    `json:"command_id"`
	Revision    windowmanager.Revision     `json:"revision"`
	Applied     bool                       `json:"applied"`
	Changes     windowmanager.ChangeSet    `json:"changes"`
	Diagnostics []windowmanager.Diagnostic `json:"diagnostics"`
	Client      *windowmanager.ClientView  `json:"client,omitempty"`
	RebasedFrom *windowmanager.Revision    `json:"rebased_from,omitempty"`
}

type windowManagerPreviewResult struct {
	WorkspaceID windowmanager.WorkspaceID  `json:"workspace_id"`
	CommandID   windowmanager.CommandID    `json:"command_id"`
	Revision    windowmanager.Revision     `json:"revision"`
	Changed     bool                       `json:"changed"`
	Changes     windowmanager.ChangeSet    `json:"changes"`
	Diagnostics []windowmanager.Diagnostic `json:"diagnostics"`
	Client      *windowmanager.ClientView  `json:"client,omitempty"`
	Snapshot    windowmanager.Snapshot     `json:"snapshot"`
}

func newWindowManagerCommandResult(
	commandID windowmanager.CommandID,
	result windowmanager.Result,
) windowManagerCommandResult {
	return windowManagerCommandResult{
		WorkspaceID: result.Snapshot.WorkspaceID,
		CommandID:   commandID,
		Revision:    result.Snapshot.Revision,
		Applied:     result.Applied,
		Changes:     result.Changes,
		Diagnostics: append([]windowmanager.Diagnostic{}, result.Diagnostics...),
		Client:      result.Client,
		RebasedFrom: result.RebasedFrom,
	}
}

func newWindowManagerPreviewResult(
	commandID windowmanager.CommandID,
	preview windowmanager.Preview,
) windowManagerPreviewResult {
	return windowManagerPreviewResult{
		WorkspaceID: preview.Snapshot.WorkspaceID,
		CommandID:   commandID,
		Revision:    preview.Snapshot.Revision,
		Changed:     preview.Changed,
		Changes:     preview.Changes,
		Diagnostics: append([]windowmanager.Diagnostic{}, preview.Diagnostics...),
		Client:      preview.Client,
		Snapshot:    preview.Snapshot,
	}
}

func windowManagerSortedWindows(
	snapshot windowmanager.Snapshot,
	desktopID windowmanager.DesktopID,
	includeMinimized bool,
) []windowmanager.Window {
	windows := make([]windowmanager.Window, 0, len(snapshot.Windows))
	for _, window := range snapshot.Windows {
		if desktopID != "" && window.DesktopID != desktopID {
			continue
		}
		if !includeMinimized && window.Minimized {
			continue
		}
		windows = append(windows, window)
	}
	sort.Slice(windows, func(left, right int) bool {
		return windows[left].ID < windows[right].ID
	})
	return windows
}

func windowManagerLayoutDocument(snapshot windowmanager.Snapshot) windowmanager.LayoutDocument {
	return windowmanager.LayoutDocument{
		Version:     snapshot.Version,
		WorkspaceID: snapshot.WorkspaceID,
		Desktops:    snapshot.Desktops,
		Windows:     snapshot.Windows,
		Overrides:   snapshot.Overrides,
	}
}

func (n *daemonNativeTools) windowManagerWorkspaceID(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceRef string,
	scope toolspkg.Scope,
) (windowmanager.WorkspaceID, error) {
	resolved, err := n.nativeResolvedWorkspace(ctx, id, workspaceRef, scope)
	if err != nil {
		return "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return "", windowManagerToolError(id, err)
	}
	return windowmanager.WorkspaceID(workspaceID), nil
}

func windowManagerActor(scope toolspkg.Scope) windowmanager.Actor {
	kind := strings.TrimSpace(scope.ActorKind)
	if kind == "" {
		if scope.Operator {
			kind = windowManagerOperatorActor
		} else {
			kind = daemonAgentField
		}
	}
	id := firstNonEmpty(scope.AgentName, scope.SessionID)
	if id == "" && scope.Operator {
		id = windowManagerOperatorActor
	}
	return windowmanager.Actor{Kind: kind, ID: id}
}

func windowManagerOrigin(id toolspkg.ToolID, input string) string {
	if origin := strings.TrimSpace(input); origin != "" {
		return origin
	}
	return "native:" + id.String()
}

func windowManagerToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	code := toolspkg.ErrorCodeBackendFailed
	cause := toolspkg.ErrToolBackendFailed
	reason := toolspkg.ReasonBackendUnhealthy
	switch {
	case errors.Is(err, windowmanager.ErrWorkspaceNotFound),
		errors.Is(err, windowmanager.ErrDesktopNotFound),
		errors.Is(err, windowmanager.ErrWindowNotFound),
		errors.Is(err, windowmanager.ErrClientNotFound),
		errors.Is(err, windowmanager.ErrLayoutResourceNotFound):
		code, cause, reason = toolspkg.ErrorCodeNotFound, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown
	case errors.Is(err, windowmanager.ErrRevisionConflict),
		errors.Is(err, windowmanager.ErrFinalDesktop),
		errors.Is(err, windowmanager.ErrHistoryBoundary):
		code, cause, reason = toolspkg.ErrorCodeConflict, toolspkg.ErrToolConflict, toolspkg.ReasonConflictedID
	case errors.Is(err, windowmanager.ErrInvalidCommand),
		errors.Is(err, windowmanager.ErrInvalidTopology),
		errors.Is(err, windowmanager.ErrDestinationRequired):
		code, cause, reason = toolspkg.ErrorCodeInvalidInput, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid
	case errors.Is(err, windowmanager.ErrClosed):
		code, cause, reason = toolspkg.ErrorCodeUnavailable, toolspkg.ErrToolUnavailable, toolspkg.ReasonDependencyMissing
	}
	message := fmt.Sprintf("tool %q window-manager call failed: %v", id, err)
	var topology *windowmanager.TopologyError
	if errors.As(err, &topology) && len(topology.Diagnostics) > 0 {
		message = fmt.Sprintf("%s (%s at %s)", message, topology.Diagnostics[0].Code, topology.Diagnostics[0].Path)
	}
	return toolspkg.NewToolError(code, id, message, fmt.Errorf("%w: %w", cause, err), reason)
}
