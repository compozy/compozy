package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const nativeSpawnRefusalMaxBytes = 2048

type nativeSessionSpawner interface {
	Spawn(context.Context, session.SpawnOpts) (*session.Session, error)
	ResolveSpawnWorkspace(context.Context, string, string) (string, error)
	SendSpawnPrompt(context.Context, session.SpawnPromptRequest) (session.SendPromptResult, error)
}

func (n *daemonNativeTools) sessionSpawn(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionSpawnInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	parentID, parent, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, scope.SessionID, false)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	spawner, ok := n.deps.Sessions.(nativeSessionSpawner)
	if !ok {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session spawn service is unavailable")
	}
	childWorkspace, worktreeTarget, err := n.resolveNativeSpawnPlacement(ctx, req.ToolID, parentID, parent, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	opts := nativeSpawnOpts(input, parentID, childWorkspace)
	opts.IsolatedWorktreeID = worktreeTarget.ID()
	child, err := spawner.Spawn(ctx, opts)
	if err != nil {
		// A child returned with an error is live and uses the worktree; keep its checkout.
		if child == nil {
			err = errors.Join(err, core.RollbackSpawnWorktree(ctx, n.deps.Worktrees, worktreeTarget))
		}
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	info := child.Info()
	if info == nil || info.Lineage == nil || info.Lineage.TTLExpiresAt == nil {
		return toolspkg.ToolResult{}, errors.New("daemon: spawned session is missing lineage")
	}
	payload := map[string]any{
		watchEventsPayloadSessionIDKey: info.ID, "spawn_role": info.Lineage.SpawnRole,
		"spawn_depth": info.Lineage.SpawnDepth, "ttl_expires_at": info.Lineage.TTLExpiresAt.UTC(),
		"permissions": contract.NormalizeSpawnPermissionPolicyPayload(
			contract.SessionLineagePayloadFromStore(info.Lineage).PermissionPolicy,
		),
	}
	if worktreePayload := core.SpawnWorktreePayloadFromTarget(worktreeTarget); worktreePayload != nil {
		payload["worktree"] = worktreePayload
	}
	if prompt := strings.TrimSpace(input.Prompt); prompt != "" {
		result, promptErr := spawner.SendSpawnPrompt(ctx, session.SpawnPromptRequest{
			ParentSessionID: parentID,
			ParentAgentName: scope.AgentName,
			ChildSessionID:  info.ID,
			Message:         prompt,
			Source:          req.ToolID.String(),
		})
		if promptErr != nil {
			payload["prompt"] = core.SpawnPromptFailurePayload(info.ID, promptErr)
		} else {
			payload["prompt"] = core.SpawnPromptPayloadFromResult(result)
		}
	}
	return structuredResult(payload, info.ID)
}

// resolveNativeSpawnPlacement picks the child workspace and, for a worktree-bound spawn,
// authorizes that workspace before resolving or creating the worktree in it.
func (n *daemonNativeTools) resolveNativeSpawnPlacement(
	ctx context.Context,
	toolID toolspkg.ToolID,
	parentID string,
	parent *session.Info,
	input nativeSessionSpawnInput,
) (string, core.SpawnWorktreeTarget, error) {
	workspaceRef := strings.TrimSpace(input.Workspace)
	worktreeRef := strings.TrimSpace(input.Worktree)
	if worktreeRef == "" && input.NewWorktree == nil {
		if workspaceRef != "" {
			return workspaceRef, core.SpawnWorktreeTarget{}, nil
		}
		return parent.WorkspaceID, core.SpawnWorktreeTarget{}, nil
	}
	if worktreeRef != "" && input.NewWorktree != nil {
		return "", core.SpawnWorktreeTarget{}, nativeSessionOrchestrationError(toolID, core.ErrSpawnWorktreeConflict)
	}
	workspaceID, err := core.ResolveSpawnWorkspace(ctx, n.deps.Sessions, parentID, workspaceRef)
	if err != nil {
		return "", core.SpawnWorktreeTarget{}, nativeSessionOrchestrationError(toolID, err)
	}
	target, err := core.ResolveSpawnWorktree(
		ctx, n.deps.Worktrees, workspaceID, parent.ProfileID, worktreeRef, input.NewWorktree,
	)
	if err != nil {
		return "", core.SpawnWorktreeTarget{}, nativeSpawnWorktreeError(toolID, err)
	}
	return workspaceID, target, nil
}

func nativeSpawnOpts(input nativeSessionSpawnInput, parentID string, workspaceID string) session.SpawnOpts {
	autoStop := true
	if input.AutoStopOnParent != nil {
		autoStop = *input.AutoStopOnParent
	}
	return session.SpawnOpts{
		ParentSessionID:  parentID,
		AgentName:        strings.TrimSpace(input.AgentName),
		Provider:         strings.TrimSpace(input.Provider),
		Model:            strings.TrimSpace(input.Model),
		ReasoningEffort:  strings.TrimSpace(input.ReasoningEffort),
		Speed:            speedpkg.Speed(strings.TrimSpace(input.Speed)),
		ACPOptions:       contract.ACPOptionSelectionsFromPayload(input.ACPOptions),
		Name:             strings.TrimSpace(input.Name),
		Workspace:        workspaceID,
		PromptOverlay:    strings.TrimSpace(input.PromptOverlay),
		SpawnRole:        strings.TrimSpace(input.SpawnRole),
		TTL:              time.Duration(input.TTLSeconds) * time.Second,
		AutoStopOnParent: autoStop,
		NotifyCreator:    input.NotifyCreator != nil && *input.NotifyCreator,
		NotifyCreatorSet: input.NotifyCreator != nil,
		PermissionPolicy: store.SessionPermissionPolicy{
			Tools:          trimNativeStrings(input.Tools),
			Skills:         trimNativeStrings(input.Skills),
			MCPServers:     trimNativeStrings(input.MCPServers),
			WorkspacePaths: trimNativeStrings(input.WorkspacePaths),
		},
		InheritPermissions: input.InheritPermissions,
		PermissionNarrowing: core.SpawnPermissionNarrowing(
			input.Tools, input.Skills, input.MCPServers, input.WorkspacePaths,
		),
		Toolsets:       trimNativeStrings(input.Toolsets),
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
	}
}

func nativeSpawnWorktreeError(id toolspkg.ToolID, err error) error {
	if errors.Is(err, session.ErrSpawnValidation) {
		return nativeSessionOrchestrationError(id, err)
	}
	if code := core.WorktreeErrorCode(err); code != "" {
		return toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeInvalidRequest,
			id,
			fmt.Sprintf("spawn worktree refused (%s): %v", code, err),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			code,
			"Pass a ready worktree of this workspace in worktree, or adjust new_worktree name/base_ref/branch.",
			toolspkg.ReasonConfigValidationFailed,
		)
	}
	return nativeSessionOrchestrationError(id, err)
}

// nativeSpawnRefusalError keeps the daemon-composed refusal (field, value, rule) in
// the tool error instead of collapsing it to a generic denial.
func nativeSpawnRefusalError(id toolspkg.ToolID, err error) error {
	cause := diagnostics.RedactAndBound(err.Error(), nativeSpawnRefusalMaxBytes)
	switch {
	case errors.Is(err, session.ErrSpawnPermissionDenied):
		recovery := "Request only permissions the parent grants, or use inherit_permissions."
		if refusal, ok := errors.AsType[*session.SpawnPermissionError](err); ok {
			recovery = refusal.Recovery()
		}
		return toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeDenied, id, cause,
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			cause, recovery, toolspkg.ReasonPolicyDenied,
		)
	case errors.Is(err, session.ErrSpawnLimitExceeded):
		return toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeConflict, id, cause,
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			cause, nativeSpawnLimitRecovery(err), toolspkg.ReasonSpawnBudgetExhausted,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidRequest, id, cause,
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigValidationFailed,
		)
	}
}

// nativeSpawnLimitRecovery names the next step for the cap that refused the spawn.
func nativeSpawnLimitRecovery(err error) string {
	if depthErr, ok := errors.AsType[*session.SpawnDepthError](err); ok {
		return depthErr.Recovery()
	}
	if limitErr, ok := errors.AsType[*session.SpawnLimitError](err); ok {
		return limitErr.Recovery()
	}
	return "Stop a live child (compozy__session_stop) or wait for one to stop, then retry; " +
		"use compozy__subagent_delegate for answers your current turn waits on."
}

// nativeRuntimeOverrideError keeps the runtime-override rule in the tool error; the
// generic invalid-input mask otherwise hides why a model-only override was refused.
func nativeRuntimeOverrideError(id toolspkg.ToolID, err error) error {
	cause := diagnostics.RedactAndBound(err.Error(), nativeSpawnRefusalMaxBytes)
	return toolspkg.NewOperatorToolError(
		toolspkg.ErrorCodeInvalidRequest, id, cause,
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		cause, "Pass provider together with model or reasoning_effort; "+
			"compozy__provider_models_list lists the models of each provider.",
		toolspkg.ReasonConfigValidationFailed,
	)
}
