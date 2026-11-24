package sshsetup

import (
	"context"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) device(
	ctx, identity context.Context, call func(context.Context, cloudclient.Credentials) error,
) error {
	if identity.Err() != nil {
		return account.ErrSignInRequired
	}
	return m.accounts.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		current, _ := m.accounts.IdentityLifetime()
		if current != identity || identity.Err() != nil {
			return account.ErrSignInRequired
		}
		return call(ctx, creds)
	})
}

func (m *Manager) reserve(job *setupRun) error {
	row := &job.row
	if err := job.ctx.Err(); err != nil {
		return err
	}
	request, cancel := context.WithTimeout(context.WithoutCancel(job.ctx), commandBudget)
	defer cancel()
	var result cloudclient.ComputerReservation
	err := m.device(request, job.identity, func(ctx context.Context, creds cloudclient.Credentials) error {
		var err error
		result, err = m.cloud.ReserveComputer(ctx, creds, row.ReservationID)
		return err
	})
	if err != nil {
		fault := *describeFailure(err)
		fault.Step = StepAdmit
		return &fault
	}
	if result.ID == "" {
		return errInterrupted
	}
	prevRef := row.LinkTokenRef
	if row.ReservationID != result.ID {
		row.LinkTokenRef, row.LinkTokenExpiresAt = "", ""
	}
	row.ReservationID = result.ID
	job.reservationExpires = result.ExpiresAt
	if result.ComputerID != "" {
		row.ComputerID = string(result.ComputerID)
	}
	if err := m.persist(request, row); err != nil {
		return err
	}
	if prevRef != "" && row.LinkTokenRef == "" {
		m.discardToken(prevRef)
	}
	return job.ctx.Err()
}

func (m *Manager) ensureReservation(job *setupRun) error {
	if err := job.ctx.Err(); err != nil {
		return err
	}
	if job.row.Kind != KindAdd || job.row.ComputerID != "" || time.Until(job.reservationExpires) > time.Second {
		return nil
	}
	return m.reserve(job)
}

func (m *Manager) release(ctx, identity context.Context, reservation string) error {
	if reservation == "" {
		return nil
	}
	release, cancel := context.WithTimeout(context.WithoutCancel(ctx), commandBudget)
	defer cancel()
	err := m.device(release, identity, func(ctx context.Context, creds cloudclient.Credentials) error {
		return m.cloud.ReleaseComputerReservation(ctx, creds, reservation)
	})
	if cloudclient.FailureCode(err) == ErrLinked.Code {
		return ErrLinked
	}
	return err
}

func (m *Manager) finish(job *setupRun, runErr error) {
	row := &job.row
	save, cancel := context.WithTimeout(context.WithoutCancel(job.ctx), commandBudget)
	defer cancel()
	if runErr != nil || job.ctx.Err() != nil {
		runErr = m.finishFailure(save, job, runErr)
	}
	status := storedStatus(*row)
	if runErr != nil && status.State != Canceled {
		status.Error, status.State = describeFailure(runErr), Failed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job.answer = nil
	m.publish(job, status)
	delete(m.activeHosts, row.HostKey)
	delete(m.runs, row.ID)
	close(job.done)
}

func (m *Manager) finishFailure(ctx context.Context, job *setupRun, runErr error) error {
	row := &job.row
	userCanceled := errors.Is(context.Cause(job.ctx), errCanceled) || errors.Is(runErr, errDeclined)
	var releaseErr error
	if row.Kind == KindAdd && row.ComputerID == "" {
		releaseErr = m.release(ctx, job.identity, row.ReservationID)
	}
	if userCanceled && releaseErr == nil && (row.Kind == KindUpdate || row.ComputerID == "") {
		if err := m.cancelRow(ctx, row); err != nil {
			return errors.Join(errInterrupted, err)
		}
		return nil
	}
	if runErr == nil {
		runErr = errInterrupted
	}
	fault := describeFailure(runErr)
	if userCanceled && releaseErr != nil {
		fault = describeFailure(releaseErr)
	}
	recordSetupFailure(row, fault)
	m.logger.InfoContext(ctx, "computers.setup_step", "setup_id", row.ID, "step", row.Step, "result", fault.Code)
	return errors.Join(fault, m.persist(ctx, row))
}

func recordSetupFailure(row *store.ComputerSSHSetup, fault *Failure) {
	row.ErrorCode, row.ErrorDetail = fault.Code, fault.Message
	if len(fault.Details) != 0 || fault.Step != "" {
		if data, err := json.Marshal(fault); err == nil {
			row.ErrorDetail = string(data)
		}
	}
}

func (m *Manager) cancelRow(ctx context.Context, row *store.ComputerSSHSetup) error {
	ref := row.LinkTokenRef
	row.Step, row.ErrorCode, row.ErrorDetail = Canceled, "", ""
	row.LinkTokenRef, row.LinkTokenExpiresAt = "", ""
	if err := m.persist(ctx, row); err != nil {
		return err
	}
	m.discardToken(ref)
	return nil
}

func (m *Manager) discardToken(ref string) {
	if ref != "" {
		if err := m.accounts.DeleteHostSecret(ref); err != nil {
			m.logger.Warn("Could not delete SSH setup link credential")
		}
	}
}
