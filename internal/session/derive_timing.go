package session

import (
	"context"
	"time"
)

type deriveTiming struct {
	started             time.Time
	lock, read, bound   time.Duration
	commit, nativeClone time.Duration
}

func (m *Manager) logDeriveTiming(
	ctx context.Context,
	spec deriveSpec,
	operation string,
	timing *deriveTiming,
	err error,
) {
	if m == nil || m.logger == nil || timing == nil {
		return
	}
	m.logger.InfoContext(
		ctx,
		"session.derive.timing",
		"operation",
		operation,
		"derive.kind",
		spec.kind,
		"source_session_id",
		spec.sourceID,
		"workspace_id",
		spec.workspaceID,
		"duration",
		time.Since(timing.started),
		"lock_wait",
		timing.lock,
		"read",
		timing.read,
		"bound",
		timing.bound,
		"commit",
		timing.commit,
		"native_clone",
		timing.nativeClone,
		"failed",
		err != nil,
	)
}
