package session

import (
	"context"
	"errors"
)

type SupervisedWorkRecovery func(context.Context, *Info, string) error

func (m *Manager) SetSupervisedWorkRecovery(recoverWork SupervisedWorkRecovery) {
	m.supervisedRecoveryMu.Lock()
	defer m.supervisedRecoveryMu.Unlock()
	m.supervisedWorkRecovery = recoverWork
}

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
