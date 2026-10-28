package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

// WorktreeBindingResolver holds checkout usage through the durable binding transition.
type WorktreeBindingResolver interface {
	AcquireSessionWorktreeBinding(context.Context, string, string, string) (string, string, func(), error)
	AcquireSessionWorktreeUnbinding(context.Context, string, string, string) (func(), error)
}

func (m *Manager) ChangeWorktreeBinding(ctx context.Context, id, ref string, expected *string) (*Info, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: worktree binding context is required", ErrValidation)
	}
	if _, held := m.restartInputsHeld.Load(id); held || m.sessionStartRun(id) != nil {
		return nil, ErrPromptInProgress
	}
	var changed *Info
	var previous string
	// Registered first: publish only after every session/checkout reservation releases.
	defer func() {
		if changed != nil {
			m.notifyWorktreeBindingChanged(ctx, changed, previous)
		}
	}()
	ctx, unlock, err := m.tryLockConversationOperation(ctx, id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	target, active := m.Get(id)
	if !active {
		return m.changeStoppedWorktreeBinding(ctx, id, ref, expected)
	}
	if !target.worktreeBindingAdmissionMu.TryLock() {
		return nil, ErrPromptInProgress
	}
	defer target.worktreeBindingAdmissionMu.Unlock()
	if err := target.reserveWorktreeBinding(); err != nil {
		return nil, err
	}
	defer target.releaseWorktreeBinding()
	meta := target.Meta()
	if err := m.recoverActiveWorktreeBinding(ctx, target, &meta); err != nil {
		return nil, err
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, id); err != nil {
		return nil, err
	}
	if expected != nil && *expected != meta.WorktreeIDValue() {
		return nil, store.ErrSessionWorktreeConflict
	}
	if err := m.requireWorktreeBindingIdle(ctx, id); err != nil {
		return nil, err
	}
	cwd, worktreeID, release, err := m.prepareWorktreeBindingDestination(ctx, &meta, ref)
	if err != nil {
		return nil, err
	}
	defer release()
	if worktreeID == meta.WorktreeIDValue() && cwd == meta.CWDValue() {
		return target.Info(), nil
	}
	if err := m.releaseWorktreeBindingRuntime(ctx, target); err != nil {
		return nil, err
	}
	info, err := m.persistWorktreeBinding(ctx, target, cwd, worktreeID)
	if err == nil {
		changed, previous = info, meta.WorktreeIDValue()
	}
	return info, err
}

func (m *Manager) reserveWorktreeBindingCheckouts(workspaceID string, ids ...string) (func(), error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	keys := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		key := workspaceID + "\x00" + id
		if m.worktreeDeliveryFences[key] {
			return nil, worktree.ErrOperationInProgress
		}
		keys[key] = struct{}{}
	}
	if m.worktreeBindingUses == nil {
		m.worktreeBindingUses = make(map[string]int)
	}
	for key := range keys {
		m.worktreeBindingUses[key]++
	}
	return sync.OnceFunc(func() {
		m.lifecycleMu.Lock()
		defer m.lifecycleMu.Unlock()
		for key := range keys {
			m.worktreeBindingUses[key]--
			if m.worktreeBindingUses[key] == 0 {
				delete(m.worktreeBindingUses, key)
			}
		}
	}), nil
}

func (s *Session) reserveWorktreeBinding() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.worktreeBindingRefusalLocked(); err != nil {
		return err
	}
	s.worktreeBindingReserved = true
	return nil
}

func (s *Session) releaseWorktreeBinding() {
	s.mu.Lock()
	s.worktreeBindingReserved = false
	s.mu.Unlock()
}

func (m *Manager) releaseWorktreeBindingRuntime(ctx context.Context, target *Session) error {
	target.mu.Lock()
	release := target.runtimeRelease
	if target.process != nil {
		target.captureProcessCapabilitiesLocked()
		release = &runtimeRelease{acpSessionID: target.ACPSessionID, process: target.process, done: make(chan struct{})}
		target.process = nil
		target.runtimeRelease = release
		target.RuntimeStatus = RuntimeStatusUnbound
		target.RuntimeTransition = RuntimeTransitionNone
		target.mu.Unlock()
		if err := m.persistSessionLifecycleState(ctx, target, false); err != nil {
			return errors.Join(err, m.stopReleasedRuntime(target, release))
		}
		if err := m.stopReleasedRuntime(target, release); err != nil {
			return err
		}
	} else {
		target.mu.Unlock()
		if err := m.waitRuntimeRelease(ctx, target, release); err != nil {
			return err
		}
	}
	if release != nil && target.clearReleasedRuntimeIdentity(release) {
		return m.persistSessionLifecycleState(ctx, target, false)
	}
	return nil
}

func (m *Manager) persistWorktreeBinding(ctx context.Context, target *Session, cwd, worktreeID string) (*Info, error) {
	target.persistMu.Lock()
	defer target.persistMu.Unlock()
	meta := target.Meta()
	update := func() {
		target.mu.Lock()
		defer target.mu.Unlock()
		target.WorktreeID, target.CWD = meta.WorktreeIDValue(), meta.CWDValue()
		target.worktreeTransition = store.CloneSessionWorktreeTransition(meta.WorktreeTransition)
		target.UpdatedAt = meta.UpdatedAt
	}
	if err := m.writeWorktreeBinding(ctx, &meta, cwd, worktreeID, update); err != nil {
		return nil, err
	}
	return target.Info(), nil
}

// Recent progress remains fresh after turn completion; it is not execution ownership.
func worktreeBindingOwned(state *SupervisionState) bool {
	for _, signal := range state.WorkSignals {
		if signal.Kind != WorkSignalAgentProgress {
			return true
		}
	}
	return false
}

func worktreeBindingSourceUnknown(state *SupervisionState) bool {
	for _, source := range state.Sources {
		if source.Kind != WorkSignalAgentProgress && source.State == signalSourceUnknown {
			return true
		}
	}
	return false
}

func (s *Session) worktreeBindingRefusalLocked() error {
	if s.stopCause != CauseNone || s.RuntimeStatus == RuntimeStatusBinding ||
		s.RuntimeStatus == RuntimeStatusReconfiguring || s.RuntimeStatus == RuntimeStatusRecovering {
		return ErrPromptInProgress
	}
	if s.process == nil && s.runtimeRelease == nil && s.Liveness != nil && s.Liveness.SubprocessPID != 0 {
		return ErrStopVerificationFailed
	}
	if s.isPromptingLocked() || s.worktreeBindingReserved || s.worktreeForkReserved || s.conversationRewindReserved ||
		len(s.PendingInteractions) != 0 || (s.process != nil && s.process.HasPendingPermission()) {
		return ErrPromptInProgress
	}
	if s.State != StateActive {
		return ErrSessionNotActive
	}
	return s.worktreeBindingEligibilityLocked()
}

func (s *Session) worktreeBindingEligibilityLocked() error {
	if (s.creationProfile != nil && s.creationProfile.RuntimeMode == RuntimeModeVerdictOnly) ||
		normalizeSessionType(s.Type) != SessionTypeUser ||
		s.WorkspaceID == "" ||
		(s.Lineage != nil &&
			(s.Lineage.SpawnRole != "" || s.Lineage.Kind == store.LineageKindSpawn)) {
		return fmt.Errorf("%w: only ordinary project user sessions can change worktrees", ErrValidation)
	}
	return nil
}

func (m *Manager) requireWorktreeBindingIdle(ctx context.Context, id string) error {
	if m.inputQueue != nil {
		pending, err := m.inputQueue.List(ctx, id)
		if err != nil {
			return err
		}
		if len(pending) != 0 {
			return ErrPromptInProgress
		}
	}
	if m.workSignals.hasExternalSources() {
		state := m.workSignals.Inspect(ctx, id, m.now())
		if worktreeBindingOwned(state) || worktreeBindingSourceUnknown(state) {
			return ErrPromptInProgress
		}
	}
	return nil
}

func (m *Manager) recoverActiveWorktreeBinding(ctx context.Context, target *Session, meta *store.SessionMeta) error {
	if meta.WorktreeTransition == nil || !meta.WorktreeTransition.Pending {
		return nil
	}
	catalog, ok := m.sessionCatalog.(store.SessionWorktreeBindingStore)
	if !ok {
		return errors.New("session: worktree binding recovery store is unavailable")
	}
	path := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target.ID))
	if err := store.RecoverSessionWorktreeTransition(ctx, catalog, path, meta); err != nil {
		return err
	}
	target.mu.Lock()
	defer target.mu.Unlock()
	target.WorktreeID, target.CWD = meta.WorktreeIDValue(), meta.CWDValue()
	target.worktreeTransition = store.CloneSessionWorktreeTransition(meta.WorktreeTransition)
	return nil
}
