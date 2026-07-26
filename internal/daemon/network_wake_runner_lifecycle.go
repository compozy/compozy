package daemon

import (
	"context"
	"errors"
	"fmt"
)

// Ready closes only after the initial durable recovery scan succeeds.
func (r *networkWakeRunner) Ready() <-chan struct{} {
	if r == nil {
		return nil
	}
	return r.ready
}

func (r *networkWakeRunner) State() networkWakeRunnerState {
	if r == nil {
		return networkWakeRunnerStopped
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Start performs the durable recovery scan before exposing readiness.
func (r *networkWakeRunner) Start(ctx context.Context) error {
	if r == nil {
		return errors.New("daemon: network wake runner is required")
	}
	if ctx == nil {
		return errors.New("daemon: network wake runner context is required")
	}
	r.mu.Lock()
	if r.state != networkWakeRunnerConstructed {
		state := r.state
		r.mu.Unlock()
		return fmt.Errorf("daemon: network wake runner cannot start from %s", state)
	}
	r.state = networkWakeRunnerStarting
	enabled := r.enabled
	r.mu.Unlock()

	actor, err := deriveNetworkWakeRunnerActor()
	if err != nil {
		r.failStart()
		return err
	}
	var initial []networkWakePending
	if enabled {
		notifications, scanErr := r.loadDurableWakes(ctx)
		if scanErr != nil {
			r.failStart()
			return scanErr
		}
		initial = makeNetworkWakePending(notifications, r.now().UTC())
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	r.mu.Lock()
	r.runCancel = cancel
	r.runDone = done
	r.state = networkWakeRunnerReady
	r.readyOnce.Do(func() { close(r.ready) })
	r.mu.Unlock()

	go func() {
		runErr := r.run(runCtx, actor, initial)
		r.mu.Lock()
		r.state = networkWakeRunnerStopped
		r.mu.Unlock()
		done <- runErr
		close(done)
	}()
	return nil
}

func (r *networkWakeRunner) failStart() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = networkWakeRunnerStopped
}

// SetEnabled applies committed availability to claims and in-flight workers.
func (r *networkWakeRunner) SetEnabled(ctx context.Context, enabled bool) error {
	if r == nil {
		return errors.New("daemon: network wake runner is required")
	}
	if ctx == nil {
		return errors.New("daemon: network wake runner availability context is required")
	}
	r.mu.Lock()
	if r.state == networkWakeRunnerDraining || r.state == networkWakeRunnerStopped {
		state := r.state
		r.mu.Unlock()
		return fmt.Errorf("daemon: network wake runner cannot change availability from %s", state)
	}
	r.enabled = enabled
	state := r.state
	cancels := make([]context.CancelFunc, 0)
	if !enabled {
		cancels = make([]context.CancelFunc, 0, len(r.active))
		for _, cancel := range r.active {
			cancels = append(cancels, cancel)
		}
	}
	r.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	if !enabled || state != networkWakeRunnerReady {
		return nil
	}
	notifications, err := r.loadDurableWakes(ctx)
	if err != nil {
		return err
	}
	for _, notification := range notifications {
		r.NotifyNetworkWake(notification)
	}
	return nil
}

// Drain cancels workers and waits until every claimed wake has settled or released.
func (r *networkWakeRunner) Drain(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: network wake runner drain context is required")
	}
	r.mu.Lock()
	switch r.state {
	case networkWakeRunnerConstructed:
		r.state = networkWakeRunnerStopped
		r.mu.Unlock()
		return nil
	case networkWakeRunnerStopped:
		r.mu.Unlock()
		return nil
	case networkWakeRunnerStarting:
		r.mu.Unlock()
		return errors.New("daemon: network wake runner is still starting")
	case networkWakeRunnerReady:
		r.state = networkWakeRunnerDraining
	}
	cancel := r.runCancel
	done := r.runDone
	cancels := make([]context.CancelFunc, 0, len(r.active))
	for _, activeCancel := range r.active {
		cancels = append(cancels, activeCancel)
	}
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, activeCancel := range cancels {
		activeCancel()
	}
	if done == nil {
		return nil
	}
	select {
	case err := <-done:
		r.mu.Lock()
		r.runCancel = nil
		r.runDone = nil
		r.state = networkWakeRunnerStopped
		r.mu.Unlock()
		return err
	case <-ctx.Done():
		return fmt.Errorf("daemon: drain network wake runner: %w", ctx.Err())
	}
}

// Shutdown preserves the daemon shutdown target while using the bounded drain contract.
func (r *networkWakeRunner) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: network wake runner shutdown context is required")
	}
	return r.Drain(ctx)
}

func (r *networkWakeRunner) networkEnabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled && r.state != networkWakeRunnerDraining && r.state != networkWakeRunnerStopped
}

func (r *networkWakeRunner) registerActive(runID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[runID] = cancel
}

func (r *networkWakeRunner) unregisterActive(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.active, runID)
}

func (r *networkWakeRunner) cancelActive() {
	r.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(r.active))
	for _, cancel := range r.active {
		cancels = append(cancels, cancel)
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
