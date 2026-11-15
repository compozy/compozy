package toolruntime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"strings"

	"time"
)

// updateRecoveredState retains execution evidence when a recovered owner is
// interrupted or becomes stale; recovery adds a reason rather than erasing it.
func (r *Registry) updateRecoveredState(
	ctx context.Context, record ProcessRecord, state ProcessState, reason string, completedAt *time.Time,
) error {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	return r.updateStateLocked(
		ctx,
		record.ID,
		state,
		record.ExitCode,
		processStateReason(record.Error, reason),
		completedAt,
	)
}

func processStateReason(prior, reason string) string {
	if prior == "" || strings.HasPrefix(reason, prior) {
		return reason
	}
	if reason == "" {
		return prior
	}
	return prior + "; " + reason
}

func (r *Registry) validateRecovered(record ProcessRecord) bool {
	if record.PID <= 0 || record.StartedAt.IsZero() {
		return false
	}
	return r.verifier(record.PID, record.StartedAt)
}

func (r *Registry) updateStateLocked(
	ctx context.Context,
	id string,
	state ProcessState,
	exitCode *int,
	errText string,
	completedAt *time.Time,
) error {
	if err := state.Validate(); err != nil {
		return err
	}
	update := ProcessStateUpdate{
		ID:          strings.TrimSpace(id),
		State:       state,
		ExitCode:    exitCode,
		Error:       trimBounded(errText),
		UpdatedAt:   r.now().UTC(),
		CompletedAt: completedAt,
	}
	if update.ID == "" {
		return errors.New("toolruntime: update process id is required")
	}
	r.mu.RLock()
	active, exists := r.active[update.ID]
	r.mu.RUnlock()
	if exists && active.retired {
		// Keep the execution's cleanup evidence when recovery adds its outcome.
		update.Error = trimBounded(processStateReason(active.record.Error, update.Error))
		if update.ExitCode == nil {
			update.ExitCode = active.record.ExitCode
		}
	}

	if r.store != nil {
		if err := r.store.UpdateProcessRecordState(ctx, update); err != nil {
			return fmt.Errorf("toolruntime: update process %q state: %w", update.ID, err)
		}
	}

	r.mu.Lock()
	if active, ok := r.active[update.ID]; ok {
		if active.retired && isTerminalState(update.State) {
			delete(r.active, update.ID)
		} else {
			active.record.State = update.State
			active.record.ExitCode = update.ExitCode
			active.record.Error = update.Error
			active.record.UpdatedAt = update.UpdatedAt
			active.record.CompletedAt = update.CompletedAt
			r.active[update.ID] = active
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) upsert(ctx context.Context, record ProcessRecord) error {
	if r.store == nil {
		return nil
	}
	if err := r.store.UpsertProcessRecord(ctx, record); err != nil {
		return fmt.Errorf("toolruntime: checkpoint process %q: %w", record.ID, err)
	}
	return nil
}

func newProcessID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("toolruntime: generate process id: %w", err)
	}
	return "proc_" + hex.EncodeToString(raw[:]), nil
}
