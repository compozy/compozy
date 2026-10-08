package session

import (
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"

	"github.com/compozy/compozy/internal/acp"
)

func (m *Manager) prepareSessionLaunch(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	runtime *sessionStartRuntime,
	run *sessionStartRun,
) (acp.StartOpts, error) {
	startOpts, err := m.sessionStartOpts(spec, session, runtime.agent, runtime.mcpServers)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session runtime adapter failed", err)
	}
	startOpts.StartupManifest = runtime.startupManifest
	startOpts, err = m.prepareProviderForStart(ctx, session, runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session provider startup failed", err)
	}
	startOpts, err = m.dispatchAgentPreStart(ctx, session, runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session pre-start hook failed", err)
	}
	startOpts.Cwd, err = normalizeSessionLaunchCWD(spec, startOpts.Cwd)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session pre-start hook cwd is invalid", err)
	}
	startOpts, err = resolveProviderNativeCLI(runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session native provider startup failed", err)
	}
	startOpts = m.finalizeProviderProbeEnvForStart(session, runtime.agent, startOpts)
	if spec.resumeReplay || spec.resumeReplayBlock != "" {
		tools, toolErr := concreteDelegationTools(runtime.agent, m.toolsetCatalog, m.toolUniverse)
		if toolErr != nil {
			return acp.StartOpts{}, startupFailure("session replay tool surface resolution failed", toolErr)
		}
		historyAvailable := runtime.agent.SessionMCP && m.hostedMCP != nil &&
			slices.Contains(tools, toolspkg.ToolIDSessionHistory.String())
		meta := session.Meta()
		policy := store.NormalizeSessionLineage(meta.ID, meta.Lineage).PermissionPolicy
		if len(policy.Tools) > 0 {
			historyAvailable = historyAvailable && slices.Contains(policy.Tools, toolspkg.ToolIDSessionHistory.String())
		}
		options := rebuildReplayContext{
			workspace: &spec.workspace, historyAvailable: historyAvailable, reason: spec.resumeReplayReason,
		}
		if spec.resumeReplayBlock != "" {
			spec.resumeReplayBlock, spec.resumeReplayMessageCount, err = m.reboundResumeReplay(
				session, spec.resumeReplayBlock, options,
			)
		} else {
			spec.resumeReplayBlock, spec.resumeReplayMessageCount, err = m.buildResumeReplay(ctx, session, options)
		}
		if err != nil {
			return acp.StartOpts{}, startupFailure("session replay preparation failed", err)
		}
	}
	releaseCommit, err := acquireSessionStartLaunchCommit(ctx, run)
	if err != nil {
		return acp.StartOpts{}, fmt.Errorf("session: acquire launch commit for %q: %w", spec.sessionID, err)
	}
	defer releaseCommit()
	if err := context.Cause(ctx); err != nil {
		return acp.StartOpts{}, err
	}
	session.setEffectivePermissions(string(startOpts.Permissions))
	if err := finalizeStartCreationIdentityIfEnabled(spec, session); err != nil {
		return acp.StartOpts{}, startupFailure("session creation identity changed", err)
	}
	if err := m.revalidateSessionWorktree(ctx, spec); err != nil {
		return acp.StartOpts{}, startupFailure("session worktree binding changed", err)
	}
	if err := m.persistSessionLifecycleState(ctx, session, true); err != nil {
		m.sessionLogger(session).Warn("session.start.meta_write_failed", "phase", spec.startAction, "error", err)
		return acp.StartOpts{}, fmt.Errorf(
			"session: persist %s launch identity for %q: %w",
			spec.startAction,
			spec.sessionID,
			err,
		)
	}
	return startOpts, nil
}

func normalizeSessionLaunchCWD(spec *sessionStartSpec, requested string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("session: start spec is required")
	}
	cwd, err := resolveContainedDirectory(spec.executionRoot(), requested)
	if err != nil {
		return "", fmt.Errorf("%w: pre-start hook cwd escapes execution root: %w", ErrValidation, err)
	}
	return cwd, nil
}
