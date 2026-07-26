package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/admission"
	"github.com/compozy/agh/internal/api/contract"
)

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

// IsDraining reports whether new-work admission is closed.
func (d *Daemon) IsDraining() bool {
	return d != nil && d.DrainState() == DrainStateDraining
}

func fmtDrainContextError(operation string, err error) error {
	return fmt.Errorf("daemon: %s context ended: %w", operation, err)
}
