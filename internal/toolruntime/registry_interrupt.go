package toolruntime

import (
	"context"

	"errors"
	"fmt"
)

// ID returns the durable process record ID.
func (h *Handle) ID() string {
	if h == nil {
		return ""
	}
	return h.id
}

// Checkpoint persists a state or owner update for the process.
func (h *Handle) Checkpoint(ctx context.Context, checkpoint ProcessCheckpoint) error {
	if h == nil || h.registry == nil {
		return nil
	}
	return h.registry.checkpoint(ctx, h.id, checkpoint)
}

// Complete records the terminal process state exactly once.
func (h *Handle) Complete(ctx context.Context, completion ProcessCompletion) error {
	if h == nil || h.registry == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.complete {
		return nil
	}
	if err := h.registry.complete(ctx, h.id, completion); err != nil {
		return err
	}
	h.complete = true
	return nil
}

// ReconcileBoot validates durable active records after daemon restart.
func (r *Registry) ReconcileBoot(ctx context.Context) (BootReconcileReport, error) {
	if r == nil {
		return BootReconcileReport{}, errors.New("toolruntime: registry is required")
	}
	if ctx == nil {
		return BootReconcileReport{}, errors.New("toolruntime: reconcile context is required")
	}
	if r.store == nil {
		return BootReconcileReport{}, nil
	}

	records, err := r.store.ListProcessRecords(ctx, ProcessQuery{States: activeStates()})
	if err != nil {
		return BootReconcileReport{}, fmt.Errorf("toolruntime: list process records for reconciliation: %w", err)
	}

	var report BootReconcileReport
	var errs []error
	for _, record := range records {
		report.Checked++
		if r.validateRecovered(record) {
			report.Recovered++
			if updateErr := r.store.UpdateProcessRecordState(ctx, ProcessStateUpdate{
				ID:        record.ID,
				State:     ProcessStateRunning,
				UpdatedAt: r.now().UTC(),
			}); updateErr != nil {
				errs = append(errs, updateErr)
			}
			continue
		}
		report.Stale++
		if updateErr := r.markStale(
			ctx,
			record.ID,
			"recovered process pid/start time did not validate",
		); updateErr != nil {
			errs = append(errs, updateErr)
		}
	}
	return report, errors.Join(errs...)
}

// Interrupt signals only processes matching the supplied scope.
func (r *Registry) Interrupt(ctx context.Context, scope InterruptScope) (InterruptReport, error) {
	if r == nil {
		return InterruptReport{}, errors.New("toolruntime: registry is required")
	}
	if ctx == nil {
		return InterruptReport{}, errors.New("toolruntime: interrupt context is required")
	}
	scope = scope.Normalize()
	if scope.IsZero() {
		return InterruptReport{}, errors.New("toolruntime: interrupt scope is required")
	}

	candidates, err := r.interruptCandidates(ctx, scope)
	if err != nil {
		return InterruptReport{}, err
	}
	if len(candidates) == 0 {
		return InterruptReport{}, ErrProcessNotFound
	}

	var report InterruptReport
	var errs []error
	for _, candidate := range candidates {
		report.Matched++
		if err := r.updateState(ctx, candidate.record.ID, ProcessStateInterrupting, nil, "", nil); err != nil {
			errs = append(errs, err)
			continue
		}
		if candidate.interrupt != nil {
			if err := candidate.interrupt(ctx, candidate.record); err != nil {
				errs = append(errs, fmt.Errorf("toolruntime: interrupt live process %q: %w", candidate.record.ID, err))
				continue
			}
			report.Signaled++
			continue
		}
		if !r.validateRecovered(candidate.record) {
			report.Stale++
			if err := r.markStale(
				ctx,
				candidate.record.ID,
				"interrupt skipped: process pid/start time did not validate",
			); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := r.interrupter.InterruptProcess(ctx, candidate.record); err != nil {
			errs = append(errs, fmt.Errorf("toolruntime: interrupt recovered process %q: %w", candidate.record.ID, err))
			continue
		}
		completedAt := r.now().UTC()
		if err := r.updateState(
			ctx,
			candidate.record.ID,
			ProcessStateInterrupted,
			nil,
			scope.Reason,
			&completedAt,
		); err != nil {
			errs = append(errs, err)
			continue
		}
		report.Signaled++
	}
	if report.Signaled == 0 && report.Stale == 0 && len(errs) == 0 {
		report.Unavailable = report.Matched
	}
	return report, errors.Join(errs...)
}
