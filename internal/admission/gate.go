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

// StartingCode identifies temporary boot admission refusals on public surfaces.
const StartingCode = "daemon_starting"

// ErrStarting reports that recovery has not yet published daemon readiness.
var ErrStarting = errors.New("daemon starting, retry when ready")

// Checker is the read-only admission surface injected into work owners.
type Checker interface {
	Check(context.Context) error
}

// Gate is a concurrency-safe, in-memory daemon admission gate.
type Gate struct {
	draining atomic.Bool
	booting  atomic.Bool
	load     atomic.Pointer[LoadMonitor]
	signIn   atomic.Pointer[func() error]
}

type bootOwnerKey struct{}

// BeginBoot permits only composition-root recovery until FinishBoot publishes readiness.
// The capability is an in-process context value and cannot be supplied by clients.
func (g *Gate) BeginBoot(ctx context.Context) context.Context {
	g.draining.Store(false)
	g.booting.Store(true)
	return context.WithValue(ctx, bootOwnerKey{}, g)
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

// FinishBoot publishes composition-root readiness independently of public drain control.
func (g *Gate) FinishBoot() {
	if g != nil {
		g.booting.Store(false)
	}
}

// SetLoadMonitor attaches host-load-aware admission for background work; nil detaches it.
func (g *Gate) SetLoadMonitor(monitor *LoadMonitor) {
	if g == nil {
		return
	}
	g.load.Store(monitor)
}

func (g *Gate) SetSignInCheck(check func() error) {
	if check == nil {
		g.signIn.Store(nil)
		return
	}
	g.signIn.Store(&check)
}

func (g *Gate) CheckSignIn() error {
	if g != nil {
		if check := g.signIn.Load(); check != nil {
			return (*check)()
		}
	}
	return nil
}

// State returns the current admission state.
func (g *Gate) State() State {
	if g != nil && (g.draining.Load() || g.booting.Load()) {
		return StateDraining
	}
	return StateActive
}

// Check admits active daemons and rejects new work while draining. Background
// work (see WithBackgroundWork) first waits, bounded, for host CPU pressure to clear.
func (g *Gate) Check(ctx context.Context) error {
	if ctx == nil {
		return errors.New("admission: context is required")
	}
	if err := g.checkState(ctx); err != nil {
		return err
	}
	if g == nil || !IsBackgroundWork(ctx) {
		return nil
	}
	monitor := g.load.Load()
	if monitor == nil {
		return nil
	}
	if err := monitor.Wait(ctx); err != nil {
		return err
	}
	return g.checkState(ctx)
}

func (g *Gate) checkState(ctx context.Context) error {
	if err := g.CheckRuntime(ctx); err != nil {
		return err
	}
	return g.CheckSignIn()
}

// CheckRuntime reports lifecycle admission for runtime management requests.
func (g *Gate) CheckRuntime(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if g != nil && g.draining.Load() {
		return ErrDraining
	}
	if g != nil && g.booting.Load() && ctx.Value(bootOwnerKey{}) != g {
		return ErrStarting
	}
	return nil
}
