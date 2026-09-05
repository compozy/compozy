package session

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) finalizeRecoveredSandbox(ctx context.Context, snapshot *Session, meta *store.SessionMeta) error {
	if meta.Sandbox == nil {
		return nil
	}
	snapshot.Sandbox = cloneSessionSandboxMeta(meta.Sandbox)
	snapshot.CWD = meta.CWDValue()
	if meta.Sandbox.State == sandboxStateDestroyed || meta.Sandbox.State == sandboxStateStopped {
		return nil
	}
	workspace, err := m.resolveResumeWorkspace(ctx, *meta)
	if err != nil {
		return err
	}
	policy := workspace.Sandbox
	if meta.Sandbox.Profile != "" && meta.Sandbox.Profile != policy.Profile {
		policy, err = workspace.Config.ResolveSandbox(meta.Sandbox.Profile)
		if err != nil {
			return err
		}
	}
	if string(policy.Backend) != meta.Sandbox.Backend {
		return fmt.Errorf("session: recovered sandbox backend does not match profile for %s", meta.ID)
	}
	snapshot.sandboxDestroyOnStop = policy.DestroyOnStop
	return m.finalizeSandboxWithPersistence(
		ctx,
		snapshot,
		sandboxSyncReasonForStop(snapshot),
		func(session *Session) error {
			return m.persistRecoveredSandbox(ctx, session)
		},
	)
}

func (m *Manager) persistRecoveredSandbox(ctx context.Context, snapshot *Session) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	meta, err := m.readMetaWithContext(ctx, snapshot.ID)
	if err != nil {
		return err
	}
	info := snapshot.Info()
	if meta.Sandbox == nil || info.Sandbox == nil || meta.Sandbox.SandboxID != info.Sandbox.SandboxID {
		return fmt.Errorf("session: recovered sandbox identity changed for %s", snapshot.ID)
	}
	meta.Sandbox = cloneSessionSandboxMeta(info.Sandbox)
	meta.UpdatedAt = info.UpdatedAt
	if err := store.WriteSessionMeta(
		store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, snapshot.ID)),
		meta,
	); err != nil {
		return err
	}
	return m.persistRecoveryCatalog(ctx, &meta)
}
