package session

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/acp"
)

func (m *Manager) prepareSessionLaunch(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	runtime *sessionStartRuntime,
) (acp.StartOpts, error) {
	startOpts := m.sessionStartOpts(spec, session, runtime.agent, runtime.mcpServers)
	startOpts, err := m.prepareProviderForStart(ctx, session, runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session provider startup failed", err)
	}
	startOpts, err = m.prepareSandboxForStart(ctx, spec, session, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session sandbox startup failed", err)
	}
	startOpts, err = m.dispatchAgentPreStart(ctx, session, runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session pre-start hook failed", err)
	}
	if spec.resumeReplay {
		spec.resumeReplayBlock, spec.resumeReplayMessageCount, err = m.buildResumeReplay(ctx, session)
		if err != nil {
			return acp.StartOpts{}, startupFailure("session replay preparation failed", err)
		}
	}
	session.setEffectivePermissions(string(startOpts.Permissions))
	if err := finalizeStartCreationIdentityIfEnabled(spec, session); err != nil {
		return acp.StartOpts{}, startupFailure("session creation identity changed", err)
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
