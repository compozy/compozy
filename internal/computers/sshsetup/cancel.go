package sshsetup

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/account"
)

func (m *Manager) Cancel(ctx context.Context, id string) error {
	identity, installation, err := m.identity()
	if err != nil {
		return err
	}
	for {
		m.mu.Lock()
		job := m.runs[id]
		if job == nil || job.identity != identity {
			err := m.cancelStored(ctx, identity, installation, id)
			m.mu.Unlock()
			return err
		}
		if job.row.Kind == KindAdd && job.status.ComputerID != "" {
			m.mu.Unlock()
			return ErrLinked
		}
		if linking := job.linking; linking != nil {
			m.mu.Unlock()
			if err := waitStopped(ctx, identity, linking); err != nil {
				return err
			}
			continue
		}
		job.cancel(errCanceled)
		m.mu.Unlock()
		if err := waitStopped(ctx, identity, job.done); err != nil {
			return err
		}
		m.mu.Lock()
		status := job.status
		m.mu.Unlock()
		if job.row.Kind == KindAdd && status.ComputerID != "" {
			return ErrLinked
		}
		if status.State == Canceled {
			return nil
		}
		if status.Error != nil {
			return status.Error
		}
		return ErrLinked
	}
}

func waitStopped(ctx, identity context.Context, stopped <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-identity.Done():
		return account.ErrSignInRequired
	case <-stopped:
		return nil
	}
}

func (m *Manager) cancelStored(ctx, identity context.Context, installation account.Installation, id string) error {
	row, err := m.load(ctx, installation.AccountID, id)
	if err != nil || row.Step == Canceled {
		return err
	}
	if row.Step == StepDone || (row.Kind == KindAdd && row.ComputerID != "") {
		return ErrLinked
	}
	if err := m.release(ctx, identity, row.ReservationID); err != nil {
		return err
	}
	if err := m.cancelRow(ctx, &row); err != nil {
		return errors.Join(errInterrupted, err)
	}
	m.registry.PublishSSHSetup(installation.AccountID, installation.AuthGeneration, storedStatus(row))
	return nil
}
