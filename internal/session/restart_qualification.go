package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	restartBindingUnknownReason   = "binding_unknown"
	restartDisabledReason         = "disabled"
	restartInstanceInactiveReason = "instance_inactive"
	restartParentStoppedReason    = "parent_stopped"
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
	ctx context.Context, meta *store.SessionMeta, checkpoint store.SessionRestartCheckpoint,
) (string, error) {
	if checkpoint.Reason != "" {
		return checkpoint.Reason, nil
	}
	if restartFailureRequiresOperator(meta.Failure) {
		return "agent_failure", nil
	}
	if checkpoint.Delivery != string(PromptDeliveryNormal) {
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
		return restartDisabledReason, nil
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID); err != nil {
		return "archived", nil
	}
	if reason, err := m.restartRuntimeSkipReason(ctx, meta, checkpoint, &workspace); err != nil || reason != "" {
		return reason, err
	}
	if reason, err := m.restartLimitReason(ctx, meta, cfg); err != nil || reason != "" {
		return reason, err
	}
	return m.restartSpawnSkipReason(ctx, meta)
}

func (m *Manager) restartSpawnSkipReason(ctx context.Context, meta *store.SessionMeta) (string, error) {
	if normalizeSessionType(Type(meta.SessionType)) != SessionTypeSpawned {
		return "", nil
	}
	if !restartResumeOwns(SessionTypeSpawned, meta.Lineage) {
		// Delegated children enter through their service's recovery owner, which enforces
		// their turn lifetime instead of the long-lived worker TTL and spawn budget.
		return "", nil
	}
	lineage := meta.Lineage
	if lineage.TTLExpiresAt == nil || !lineage.TTLExpiresAt.After(m.now()) {
		return "expired", nil
	}
	parent, err := m.Status(ctx, lineage.ParentSessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return "", err
	}
	parentStopped := parent == nil || (parent.State != StateActive && parent.State != StateStarting)
	var governance spawnGovernance
	if parent == nil {
		// Independent workers retain their accepted bounds even after the parent is removed.
		parent = &Info{ID: lineage.ParentSessionID, Lineage: &store.SessionLineage{SpawnBudget: lineage.SpawnBudget}}
		governance = spawnGovernance{rootID: lineage.RootSessionID, depth: max(0, lineage.SpawnDepth-1)}
	} else {
		governance, err = m.spawnGovernanceForParent(ctx, parent)
		if err != nil {
			return "", err
		}
	}
	budget := store.EffectiveSessionSpawnBudget(store.NormalizeSessionLineage(parent.ID, parent.Lineage).SpawnBudget)
	if governance.depth+1 > budget.MaxDepth {
		return "spawn_budget", nil
	}
	if err := m.validateSpawnCaps(ctx, parent, governance, budget, meta.WorkspaceID); err != nil {
		if errors.Is(err, ErrSpawnLimitExceeded) {
			return "spawn_budget", nil
		}
		return "", err
	}
	// Report a stopped parent only after every other check, so monitor recovery can safely
	// reopen a required shutdown-stopped parent without reviving one for an ineligible child.
	if lineage.AutoStopOnParent && parentStopped {
		return restartParentStoppedReason, nil
	}
	return "", nil
}

// restartFailureRequiresOperator reports whether a failure retained on an interrupted session
// records a genuine agent failure. Launch, handshake, deadline and cancellation failures, and a
// transport loss outside a prompt, come from the infrastructure that starts a provider (hosted MCP
// bind, adapter prepare or initialize, a start canceled by daemon shutdown). They say nothing about
// the interrupted turn, so continuation proceeds and the successful resume clears them.
func restartFailureRequiresOperator(failure *store.SessionFailure) bool {
	if failure == nil || failure.IsZero() {
		return false
	}
	switch failure.Kind {
	case store.FailureStartup, store.FailureHandshake, store.FailureTimeout, store.FailureCanceled:
		return false
	case store.FailureTransport:
		return failure.ReasonCode == store.FailureReasonPromptStreamIncomplete ||
			failure.ReasonCode == store.FailureReasonPromptDeliveryReadFailed
	default:
		return true
	}
}

func (m *Manager) restartRuntimeSkipReason(
	ctx context.Context,
	meta *store.SessionMeta,
	checkpoint store.SessionRestartCheckpoint,
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
	if string(m.startPermissions(startSpecPermissions(&spec, runtime.agent.Permissions))) != checkpoint.Permissions {
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

func sameRestartBinding(checkpoint store.SessionRestartCheckpoint, meta *store.SessionMeta) bool {
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
