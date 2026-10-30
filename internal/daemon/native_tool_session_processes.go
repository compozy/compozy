package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessionprocess"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeSessionProcessesInput struct {
	SessionID string                          `json:"session_id"`
	Targets   []contract.SessionProcessTarget `json:"targets"`
	All       bool                            `json:"all"`
}

func (n *daemonNativeTools) sessionProcesses() core.SessionProcessService {
	if n == nil || n.deps == nil || n.deps.Processes == nil {
		return nil
	}
	return n.deps.Processes()
}
func (n *daemonNativeTools) sessionProcessToolBindings() map[toolspkg.ToolID]nativeToolBinding {
	availability := n.dependencyAvailability(func() bool { return n.sessionProcesses() != nil })
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionProcessesList:      {call: n.sessionProcessesList, availability: availability},
		toolspkg.ToolIDSessionProcessesTerminate: {call: n.sessionProcessesTerminate, availability: availability},
	}
}

func (n *daemonNativeTools) sessionProcessesList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionProcessesInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	info, err := n.sessionProcessTarget(ctx, scope, req, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	snapshot, err := n.sessionProcesses().Refresh(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response := contract.SessionProcessesResponse{
		Supported: snapshot.Supported,
		ScannedAt: snapshot.ScannedAt,
		Items:     []contract.SessionProcessPayload{},
	}
	for _, p := range snapshot.Processes {
		if p.Owner.SessionID == info.ID && p.Owner.ProfileID == info.ProfileID &&
			p.Owner.WorkspaceID == info.WorkspaceID {
			response.Items = append(response.Items, core.SessionProcessPayload(p, snapshot.ScannedAt))
		}
	}
	return structuredResult(response, fmt.Sprintf("%d process(es)", len(response.Items)))
}

func (n *daemonNativeTools) sessionProcessesTerminate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionProcessesInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	info, err := n.sessionProcessTarget(ctx, scope, req, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	targets, err := core.ProcessTargets(input.Targets, input.All)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	results, err := n.sessionProcesses().Terminate(ctx, info.ID, targets, input.All, func(o sessionprocess.Owner) bool {
		return o.ProfileID == info.ProfileID && o.WorkspaceID == info.WorkspaceID
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		core.ProcessResults(n.sessionProcesses().Summary(info.ID).Supported, results),
		fmt.Sprintf("%d termination result(s)", len(results)),
	)
}

func (n *daemonNativeTools) sessionProcessTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	id string,
) (*session.Info, error) {
	if n.sessionProcesses() == nil {
		return nil, errors.New("daemon: session processes unavailable")
	}
	if strings.TrimSpace(id) == "" {
		id = scope.SessionID
	}
	_, info, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, id, false)
	return info, err
}

func (n *daemonNativeTools) prepareSessionProcessPolicy(
	ctx context.Context,
	id toolspkg.ToolID,
	policy contract.SessionProcessPolicy,
) error {
	if policy != "" && policy != contract.SessionProcessesKeep && policy != contract.SessionProcessesTerminate {
		return nativeInputError(id, errors.New("processes must be keep or terminate"))
	}
	service := n.sessionProcesses()
	if service == nil {
		if policy == contract.SessionProcessesTerminate {
			return nativeHTTPStatusToolError(
				id,
				errors.New("session processes unavailable"),
				http.StatusServiceUnavailable,
			)
		}
		return nil
	}
	snapshot, err := service.Refresh(ctx)
	if policy != contract.SessionProcessesTerminate {
		if err != nil && n.deps.Logger != nil {
			n.deps.Logger.Warn("Capture processes before keeping session descendants", "error", err)
		}
		return nil
	}
	if err != nil {
		return nativeHTTPStatusToolError(id, err, http.StatusInternalServerError)
	}
	if !snapshot.Supported {
		return nativeHTTPStatusToolError(
			id,
			errors.New("process termination unsupported on this host"),
			http.StatusServiceUnavailable,
		)
	}
	return nil
}

func (n *daemonNativeTools) finishSessionProcessPolicy(
	ctx context.Context,
	info *session.Info,
	policy contract.SessionProcessPolicy,
) error {
	if policy != contract.SessionProcessesTerminate {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	results, err := n.sessionProcesses().Terminate(cleanupCtx, info.ID, nil, true, func(o sessionprocess.Owner) bool {
		return o.ProfileID == info.ProfileID && o.WorkspaceID == info.WorkspaceID
	})
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Outcome != "terminated" && r.Outcome != "already_exited" {
			return fmt.Errorf("process %d termination is %s", r.PID, r.Outcome)
		}
	}
	return nil
}
