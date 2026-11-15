package toolruntime

import (
	"context"
	"errors"
	"fmt"
)

const bootReconcileReason = "daemon restart reconciliation"

// ReconcileBoot interrupts validated persisted processes and marks invalid records stale after restart.
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
		if record.StartedByPID <= 0 || !r.validateRecovered(record) {
			report.Stale++
			if updateErr := r.updateRecoveredState(
				ctx,
				record,
				ProcessStateStale,
				"boot reconciliation skipped: process ownership did not validate",
				new(r.now().UTC()),
			); updateErr != nil {
				errs = append(errs, updateErr)
			}
			continue
		}
		if updateErr := r.updateRecoveredState(
			ctx,
			record,
			ProcessStateInterrupting,
			bootReconcileReason,
			nil,
		); updateErr != nil {
			errs = append(errs, updateErr)
			continue
		}
		if interruptErr := r.interrupter.InterruptProcess(ctx, record); interruptErr != nil {
			if errors.Is(interruptErr, ErrOwnershipValidationFailed) {
				report.Stale++
				if updateErr := r.updateRecoveredState(
					ctx,
					record,
					ProcessStateStale,
					"boot reconciliation skipped: process ownership changed before signaling",
					new(r.now().UTC()),
				); updateErr != nil {
					errs = append(errs, updateErr)
				}
				continue
			}
			errs = append(
				errs,
				fmt.Errorf("toolruntime: interrupt prior process %q during boot: %w", record.ID, interruptErr),
			)
			continue
		}
		completedAt := r.now().UTC()
		if updateErr := r.updateRecoveredState(
			ctx,
			record,
			ProcessStateInterrupted,
			bootReconcileReason,
			&completedAt,
		); updateErr != nil {
			errs = append(errs, updateErr)
			continue
		}
		report.Interrupted++
	}
	return report, errors.Join(errs...)
}
