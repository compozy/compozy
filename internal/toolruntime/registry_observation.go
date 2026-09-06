package toolruntime

import (
	"context"
	"errors"
	"time"
)

// SweepInterval bounds the lifetime of a verified subprocess identity.
const SweepInterval = 15 * time.Second

// ProcessObservation is ephemeral PID evidence owned by registry reconciliation.
type ProcessObservation struct {
	Record     ProcessRecord
	VerifiedAt time.Time
}

// Observations returns evidence without renewing it. ACP agent processes are not tools.
func (r *Registry) Observations(sessionID string) []ProcessObservation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ProcessObservation, 0)
	for _, active := range r.active {
		if active.record.Owner.SessionID != sessionID || active.record.Source == ProcessSourceACPAgent {
			continue
		}
		if active.record.State != ProcessStateRunning && active.record.State != ProcessStateInterrupting {
			continue
		}
		record := active.record
		record.Args = append([]string(nil), record.Args...)
		result = append(result, ProcessObservation{Record: record, VerifiedAt: active.verifiedAt})
	}
	return result
}

// Reconcile verifies each current PID/start-time pair independently of readers.
func (r *Registry) Reconcile(ctx context.Context) error {
	r.mu.RLock()
	active := make([]activeProcess, 0, len(r.active))
	for _, process := range r.active {
		active = append(active, process)
	}
	r.mu.RUnlock()
	var errs []error
	for _, process := range active {
		if err := ctx.Err(); err != nil {
			return errors.Join(errors.Join(errs...), err)
		}
		record := process.record
		if record.PID <= 0 || isTerminalState(record.State) {
			continue
		}
		verified := r.validateRecovered(record)
		if err := r.applyProcessObservation(ctx, record, verified); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *Registry) applyProcessObservation(ctx context.Context, record ProcessRecord, verified bool) error {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	current, ok := r.active[record.ID]
	if !ok || current.record.PID != record.PID || !current.record.StartedAt.Equal(record.StartedAt) ||
		isTerminalState(current.record.State) {
		r.mu.Unlock()
		return nil
	}
	if verified {
		current.verifiedAt = r.now().UTC()
		r.active[record.ID] = current
	}
	r.mu.Unlock()
	if verified {
		return nil
	}
	completedAt := r.now().UTC()
	return r.updateStateLocked(
		ctx,
		record.ID,
		ProcessStateStale,
		nil,
		"registry reconcile: process identity no longer exists",
		&completedAt,
	)
}
