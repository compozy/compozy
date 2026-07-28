// Package admission owns the daemon-wide new-work admission boundary.
package admission

import (
	"context"
	"errors"
	"sync/atomic"
)

// State is the closed set of daemon admission states.
type State string

const (
	StateActive   State = "active"
	StateDraining State = "draining"
)

// ErrDraining reports that the daemon is temporarily refusing new work.
var ErrDraining = errors.New("daemon is draining; new work admission is closed")

// Checker is the read-only admission surface injected into work owners.
type Checker interface {
	Check(context.Context) error
}

// Gate is a concurrency-safe, in-memory daemon admission gate.
type Gate struct {
	draining atomic.Bool
}

var _ Checker = (*Gate)(nil)

// Drain closes the gate and reports whether this call changed the state.
func (g *Gate) Drain() bool {
	if g == nil {
		return false
	}
	return !g.draining.Swap(true)
}

// Undrain opens the gate and reports whether this call changed the state.
func (g *Gate) Undrain() bool {
	if g == nil {
		return false
	}
	return g.draining.Swap(false)
}

// State returns the current admission state.
func (g *Gate) State() State {
	if g != nil && g.draining.Load() {
		return StateDraining
	}
	return StateActive
}

// Check admits active daemons and rejects new work while draining.
func (g *Gate) Check(ctx context.Context) error {
	if ctx == nil {
		return errors.New("admission: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if g.State() == StateDraining {
		return ErrDraining
	}
	return nil
}
