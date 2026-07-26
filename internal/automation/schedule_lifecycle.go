package automation

import (
	"context"
	"errors"
	"fmt"

	"time"
)

// Start begins scheduled-job execution.
func (s *Scheduler) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("automation: scheduler start context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return ErrSchedulerStopped
	}
	if s.started {
		return nil
	}

	runtimeCtx, runtimeCancel := context.WithCancel(context.WithoutCancel(ctx))
	s.runtimeCtx = runtimeCtx
	s.runtimeCancel = runtimeCancel
	for jobID := range s.registrations {
		s.startJobLoopLocked(jobID)
	}
	s.started = true
	s.logger.Info("automation.scheduler.started", "jobs_loaded", len(s.registrations))
	return nil
}

// Stop shuts the scheduler down and cancels in-flight dispatches.
func (s *Scheduler) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("automation: scheduler stop context is required")
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.started = false
	cancel := s.runtimeCancel
	for jobID, registration := range s.registrations {
		if registration.cancel != nil {
			registration.cancel()
			registration.cancel = nil
			s.registrations[jobID] = registration
		}
	}
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	startedAt := time.Now()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	var shutdownErr error
	select {
	case <-done:
	case <-ctx.Done():
		shutdownErr = ctx.Err()
	}
	s.mu.Lock()
	s.registrations = make(map[string]scheduledRegistration)
	s.runtimeCtx = nil
	s.runtimeCancel = nil
	s.mu.Unlock()

	s.logger.Info("automation.scheduler.shutdown", "shutdown_duration_ms", time.Since(startedAt).Milliseconds())
	if shutdownErr != nil {
		return fmt.Errorf("automation: shutdown scheduler runtime: %w", shutdownErr)
	}
	return nil
}

// Shutdown is an alias for Stop to match daemon runtime conventions.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	return s.Stop(ctx)
}
