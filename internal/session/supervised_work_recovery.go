package session

import (
	"context"
	"errors"
)

// SupervisedWorkRecovery settles task ownership while the verified inactivity stop receipt still fences session reuse.
type SupervisedWorkRecovery func(context.Context, *Info, string) error

// SetSupervisedWorkRecovery installs the task settlement hook before stop replay or live supervision begins.
func (m *Manager) SetSupervisedWorkRecovery(recoverWork SupervisedWorkRecovery) {
	m.supervisedRecoveryMu.Lock()
	defer m.supervisedRecoveryMu.Unlock()
	m.supervisedWorkRecovery = recoverWork
}

// recoverSupervisedWork admits only inactivity stops and retains the receipt on task settlement failure.
func (m *Manager) recoverSupervisedWork(ctx context.Context, info *Info, turnID string) error {
	if info.StopCause != CauseInactivity {
		return nil
	}
	m.supervisedRecoveryMu.RLock()
	recoverWork := m.supervisedWorkRecovery
	m.supervisedRecoveryMu.RUnlock()
	if recoverWork == nil {
		return nil
	}
	if turnID == "" {
		return errors.Join(ErrRecoveryPersistence, errors.New("session: supervised stop has no durable turn"))
	}
	if err := recoverWork(ctx, info, "session-stop:"+turnID); err != nil {
		return errors.Join(ErrRecoveryPersistence, err)
	}
	return nil
}
