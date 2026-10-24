package session

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// QualifyRestartContinuation shares restart safety checks with Loop recovery, whose durable node transition owns retry.
func (m *Manager) QualifyRestartContinuation(ctx context.Context, id string) (bool, error) {
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return false, err
	}
	checkpoint, pending, err := m.pendingRestartCheckpoint(ctx, &meta)
	if err != nil || !pending {
		return false, err
	}
	reason, err := m.restartSkipReason(ctx, &meta, checkpoint)
	if err != nil {
		return false, err
	}
	if reason != "" {
		return false, m.skipRestartContinuation(ctx, &meta, checkpoint, reason)
	}
	return true, nil
}

func (m *Manager) restartSkipReason(
	ctx context.Context, meta *store.SessionMeta, checkpoint restartCheckpoint,
) (string, error) {
	if checkpoint.Reason != "" {
		return checkpoint.Reason, nil
	}
	if meta.Failure != nil {
		return "agent_failure", nil
	}
	if checkpoint.Delivery != PromptDeliveryNormal {
		return "maintenance_command", nil
	}
	if !sameRestartBinding(checkpoint, meta) {
		return "binding_changed", nil
	}
	workspace, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return "workspace_unavailable", nil
	}
	cfg := workspace.Config.Session.Restart
	if !cfg.AutoContinue {
		return "disabled", nil
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID); err != nil {
		return "archived", nil
	}
	if reason, err := m.restartRuntimeSkipReason(ctx, meta, checkpoint, &workspace); err != nil || reason != "" {
		return reason, err
	}
	return m.restartLimitReason(ctx, meta, cfg)
}

func (m *Manager) restartRuntimeSkipReason(
	ctx context.Context,
	meta *store.SessionMeta,
	checkpoint restartCheckpoint,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	worktree, root, err := m.resolveSessionWorktree(ctx, workspace.ID, meta.WorktreeIDValue())
	if err != nil || worktree != checkpoint.WorktreeID {
		return "worktree_changed", nil
	}
	if root == "" {
		root = workspace.RootDir
	}
	cwd, err := resumeSessionCWD(meta, root)
	if err != nil || cwd != checkpoint.CWD {
		return "worktree_changed", nil
	}
	spec, err := sessionStartSpecFromMeta(meta, workspace, cwd)
	if err != nil {
		return "runtime_unavailable", nil
	}
	spec.startAction, spec.acpSessionID = sessionStartActionResume, derefString(meta.ACPSessionID)
	spec.acceptedRoute = store.CloneSessionAcceptedRoute(meta.AcceptedRoute)
	runtime, err := m.resolveSessionStartRuntime(ctx, &spec, true)
	if err != nil {
		return "route_changed", nil
	}
	if spec.acceptedRoute == nil || spec.resumeReplayReason == acceptedRouteMissingReason {
		return "route_changed", nil
	}
	if string(m.startPermissions(runtime.agent.Permissions)) != checkpoint.Permissions {
		return "permissions_changed", nil
	}
	digest, err := restartPolicyDigest(runtime.agentDef)
	if err != nil {
		return "", err
	}
	if checkpoint.PolicyDigest != "" && digest != checkpoint.PolicyDigest {
		return "permissions_changed", nil
	}
	return "", nil
}

func (m *Manager) restartLimitReason(
	ctx context.Context, meta *store.SessionMeta, cfg compozyconfig.SessionRestartConfig,
) (string, error) {
	attempts, err := m.Events(ctx, meta.ID, store.EventQuery{Type: restartAttemptEvent,
		Since: m.now().Add(-cfg.Window), Limit: cfg.MaxAttempts})
	if err != nil {
		return "", fmt.Errorf("session: read restart attempts: %w", err)
	}
	if len(attempts) >= cfg.MaxAttempts {
		return "restart_limit", nil
	}
	return "", nil
}

func sameRestartBinding(checkpoint restartCheckpoint, meta *store.SessionMeta) bool {
	_, revision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelectionValue())
	if checkpoint.RuntimeRevision != revision ||
		checkpoint.WorktreeID != meta.WorktreeIDValue() || checkpoint.CWD != meta.CWDValue() ||
		checkpoint.Permissions != meta.EffectivePermissionsValue() {
		return false
	}
	left, right := checkpoint.Route, meta.AcceptedRoute
	if left == nil || right == nil {
		return false
	}
	return strings.TrimSpace(left.Provider) == strings.TrimSpace(right.Provider) &&
		left.CommandFingerprint == right.CommandFingerprint && left.AuthMode == right.AuthMode &&
		left.HomePolicy == right.HomePolicy
}
