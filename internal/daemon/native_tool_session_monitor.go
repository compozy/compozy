package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	monitorpkg "github.com/compozy/compozy/internal/monitor"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// monitorResultField is the structured result key carrying one monitor.
const monitorResultField = "monitor"

type nativeMonitorCreateInput struct {
	Kind           string `json:"kind"`
	Description    string `json:"description"`
	TerminalID     string `json:"terminal_id"`
	Until          string `json:"until"`
	Pattern        string `json:"pattern"`
	Branch         string `json:"branch"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	DelaySeconds   int    `json:"delay_seconds"`
	// TargetSessionIDs and States apply to session monitors.
	TargetSessionIDs []string `json:"target_session_ids"`
	States           []string `json:"states"`
}

type nativeMonitorCancelInput struct {
	MonitorID string `json:"monitor_id"`
}

func (n *daemonNativeTools) sessionMonitorToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDMonitorCreate: {call: n.monitorCreate, availability: availability},
		toolspkg.ToolIDMonitorList:   {call: n.monitorList, availability: availability},
		toolspkg.ToolIDMonitorCancel: {call: n.monitorCancel, availability: availability},
	}
}

func (n *daemonNativeTools) sessionMonitors() *monitorpkg.Service {
	if n == nil || n.deps == nil || n.deps.Monitors == nil {
		return nil
	}
	return n.deps.Monitors()
}

func (n *daemonNativeTools) monitorCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeMonitorCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service, workspaceID, sessionID, err := n.monitorScope(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	kind := monitorpkg.Kind(strings.TrimSpace(input.Kind))
	if kind == monitorpkg.KindSession {
		err = n.authorizeMonitorTargets(ctx, scope, req.ToolID, sessionID, input.TargetSessionIDs)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	created, err := service.Create(ctx, monitorpkg.Spec{
		WorkspaceID: workspaceID, ProfileID: strings.TrimSpace(scope.ProfileID), SessionID: sessionID,
		Kind: kind, Description: input.Description,
		TerminalID: input.TerminalID, Until: input.Until, Pattern: input.Pattern, Branch: input.Branch,
		Timeout: time.Duration(input.TimeoutSeconds) * time.Second,
		Delay:   time.Duration(input.DelaySeconds) * time.Second,
		Targets: input.TargetSessionIDs, States: input.States,
	})
	if err != nil {
		return toolspkg.ToolResult{}, monitorToolError(req.ToolID, err)
	}
	if created.Reused {
		return structuredResult(map[string]any{monitorResultField: created}, fmt.Sprintf(
			"Monitor %s already watches this request (status: %s); no duplicate was registered. "+
				"Continue independent work; end your turn only when every remaining step waits on a monitor or subagent.",
			created.ID, created.Status,
		))
	}
	fires := "when it fires"
	if created.Kind == monitorpkg.KindPullRequest {
		fires = "each time the pull request changes, until it merges or closes"
	}
	preview := fmt.Sprintf(
		"Monitor %s is active until %s. Continue independent work; end your turn only when every remaining "+
			"step waits on a monitor or subagent. This session will be woken with a "+
			"[COMPOZY MONITOR] message %s. Do not poll or sleep while waiting.",
		created.ID, created.DeadlineAt.Format(time.RFC3339), fires,
	)
	return structuredResult(map[string]any{monitorResultField: created}, preview)
}

// authorizeMonitorTargets admits only other sessions of the owner's workspace and profile. A monitor
// reports each target's name and state to anyone who can read the owner, so a target from another
// profile would expose a session its profile reads cannot see.
func (n *daemonNativeTools) authorizeMonitorTargets(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	ownerID string,
	targets []string,
) error {
	if len(targets) == 0 {
		return nil
	}
	_, owner, err := n.nativeOrchestrationTarget(ctx, scope, id, ownerID, false)
	if err != nil {
		return err
	}
	for _, target := range targets {
		_, info, err := n.nativeOrchestrationTarget(ctx, scope, id, target, true)
		if err != nil {
			return err
		}
		if info.ProfileID != owner.ProfileID {
			return nativeWorkspaceAccessDeniedError(id)
		}
	}
	return nil
}

func (n *daemonNativeTools) monitorList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	service, _, sessionID, err := n.monitorScope(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	monitors := service.List(sessionID)
	active := 0
	for _, item := range monitors {
		if item.Status == monitorpkg.StatusActive {
			active++
		}
	}
	return structuredResult(
		map[string]any{"monitors": monitors},
		fmt.Sprintf("%d active monitor(s), %d total", active, len(monitors)),
	)
}

func (n *daemonNativeTools) monitorCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeMonitorCancelInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	service, _, sessionID, err := n.monitorScope(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	canceled, err := service.Cancel(sessionID, strings.TrimSpace(input.MonitorID))
	if err != nil {
		return toolspkg.ToolResult{}, monitorToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{monitorResultField: canceled},
		fmt.Sprintf("Monitor %s is %s", canceled.ID, canceled.Status),
	)
}

// monitorScope binds monitor tools to the calling session: monitors are
// always owned by, and only wake, the session that registered them.
func (n *daemonNativeTools) monitorScope(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (*monitorpkg.Service, string, string, error) {
	service := n.sessionMonitors()
	if service == nil {
		return nil, "", "", toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, req.ToolID, "session monitors are unavailable",
			errors.New("daemon: session monitor service is not running"), toolspkg.ReasonBackendUnhealthy,
		)
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", scope.SessionID)
	if err != nil {
		return nil, "", "", err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, "", scope)
	if err != nil {
		return nil, "", "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return nil, "", "", nativeInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, sessionID)
	if err != nil {
		return nil, "", "", err
	}
	return service, info.WorkspaceID, info.ID, nil
}

func monitorToolError(id toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, monitorpkg.ErrInvalid):
		// Monitor spec validation authors static field guidance; it never includes input values.
		return toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeInvalidInput, id, "invalid monitor request",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			strings.TrimSpace(strings.TrimPrefix(err.Error(), monitorpkg.ErrInvalid.Error())),
			"Correct the indicated monitor field using the tool's input schema and retry.",
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, monitorpkg.ErrLimit):
		return toolspkg.NewToolError(toolspkg.ErrorCodeConflict, id, err.Error(), err)
	case errors.Is(err, monitorpkg.ErrNotFound):
		return toolspkg.NewToolError(toolspkg.ErrorCodeNotFound, id, err.Error(), err)
	case errors.Is(err, monitorpkg.ErrStopped):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, id, err.Error(), err, toolspkg.ReasonBackendUnhealthy,
		)
	}
	if _, ok := errors.AsType[*terminalpkg.Error](err); ok {
		return terminalToolError(id, err)
	}
	return toolspkg.NewToolError(toolspkg.ErrorCodeBackendFailed, id, err.Error(), err)
}
