package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
)

type activeClaimCounter interface {
	CountActiveTaskRunClaims(context.Context) (int, error)
}

type sessionPromptStateReader interface {
	IsPrompting(string) bool
}

// DrainState is the public daemon admission state.
type DrainState = contract.DrainState

const (
	DrainStateActive   = contract.DrainStateActive
	DrainStateDraining = contract.DrainStateDraining
)

// Drain closes new-work admission without interrupting already admitted work.
func (d *Daemon) Drain(ctx context.Context) error {
	if d == nil {
		return errors.New("daemon: drain requires a daemon")
	}
	if ctx == nil {
		return errors.New("daemon: drain context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmtDrainContextError("drain", err)
	}
	if d.admission.Drain() {
		d.runtimeLogger().Info("daemon: new-work admission closed", "state", DrainStateDraining)
	}
	return nil
}

// Undrain restores new-work admission.
func (d *Daemon) Undrain(ctx context.Context) error {
	if d == nil {
		return errors.New("daemon: undrain requires a daemon")
	}
	if ctx == nil {
		return errors.New("daemon: undrain context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmtDrainContextError("undrain", err)
	}
	if d.admission.Undrain() {
		d.runtimeLogger().Info("daemon: new-work admission restored", "state", DrainStateActive)
	}
	return nil
}

// DrainState returns the current in-memory daemon admission state.
func (d *Daemon) DrainState() DrainState {
	if d == nil || d.admission.State() == admission.StateActive {
		return DrainStateActive
	}
	return DrainStateDraining
}

// DrainStatus returns the daemon-owned quiesce readout used before process stop.
func (d *Daemon) DrainStatus(ctx context.Context) (contract.DrainStatusResponse, error) {
	if d == nil {
		return contract.DrainStatusResponse{}, errors.New("daemon: drain status requires a daemon")
	}
	if ctx == nil {
		return contract.DrainStatusResponse{}, errors.New("daemon: drain status context is required")
	}
	if err := ctx.Err(); err != nil {
		return contract.DrainStatusResponse{}, fmtDrainContextError("read drain status", err)
	}

	d.mu.Lock()
	sessions := d.sessions
	var claimCounter activeClaimCounter
	claimsKnown := d.tasks == nil
	if d.tasks != nil && d.tasks.store != nil {
		if counter, ok := d.tasks.store.(activeClaimCounter); ok {
			claimCounter = counter
			claimsKnown = true
		}
	}
	d.mu.Unlock()

	activeExecutions := 0
	executionsKnown := sessions == nil
	if promptState, ok := sessions.(sessionPromptStateReader); ok {
		executionsKnown = true
		for _, info := range sessions.List() {
			if info != nil && promptState.IsPrompting(info.ID) {
				activeExecutions++
			}
		}
	}
	activeClaims := 0
	if claimCounter != nil {
		count, err := claimCounter.CountActiveTaskRunClaims(ctx)
		if err != nil {
			return contract.DrainStatusResponse{}, fmt.Errorf("daemon: count active task claims: %w", err)
		}
		activeClaims = count
	}

	state := d.DrainState()
	return composeDrainStatus(
		state,
		activeExecutions,
		activeClaims,
		executionsKnown,
		claimsKnown,
	), nil
}

func composeDrainStatus(
	state DrainState,
	activeExecutions int,
	activeClaims int,
	executionsKnown bool,
	claimsKnown bool,
) contract.DrainStatusResponse {
	claimsSettled := claimsKnown && activeClaims == 0
	executionSettled := executionsKnown && activeExecutions == 0
	admissionClosed := state == DrainStateDraining
	return contract.DrainStatusResponse{
		State:                      state,
		AdmissionClosed:            admissionClosed,
		ActiveSessionExecutions:    activeExecutions,
		ActiveTaskClaims:           activeClaims,
		AuthoritativeClaimsSettled: claimsSettled,
		SessionExecutionSettled:    executionSettled,
		SafeToStop:                 admissionClosed && claimsSettled && executionSettled,
	}
}

// IsDraining reports whether new-work admission is closed.
func (d *Daemon) IsDraining() bool {
	return d != nil && d.DrainState() == DrainStateDraining
}

func fmtDrainContextError(operation string, err error) error {
	return fmt.Errorf("daemon: %s context ended: %w", operation, err)
}
