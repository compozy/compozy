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
	runtime := &schedulerRuntime{
		ctx:    runtimeCtx,
		cancel: runtimeCancel,
		done:   make(chan struct{}),
	}
	s.runtime = runtime
	for jobID := range s.registrations {
		s.startJobLoopLocked(jobID)
	}
	s.started = true
	go s.ownRuntime(runtime)
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
		runtime := s.runtime
		s.mu.Unlock()
		return s.waitForRuntime(ctx, runtime)
	}
	s.stopped = true
	s.started = false
	runtime := s.runtime
	for jobID, registration := range s.registrations {
		if registration.cancel != nil {
			registration.cancel()
			registration.cancel = nil
			s.registrations[jobID] = registration
		}
	}
	if runtime == nil {
		s.registrations = make(map[string]scheduledRegistration)
	}
	s.mu.Unlock()

	if runtime == nil {
		s.logger.Info("automation.scheduler.shutdown", "shutdown_duration_ms", int64(0))
		return nil
	}

	s.mu.Lock()
	if runtime.stoppedAt.IsZero() {
		runtime.stoppedAt = time.Now()
	}
	s.mu.Unlock()
	runtime.cancel()
	return s.waitForRuntime(ctx, runtime)
}

func (s *Scheduler) ownRuntime(runtime *schedulerRuntime) {
	<-runtime.ctx.Done()
	runtime.workers.Wait()

	s.mu.Lock()
	if s.runtime == runtime {
		s.registrations = make(map[string]scheduledRegistration)
		s.runtime = nil
	}
	stoppedAt := runtime.stoppedAt
	s.mu.Unlock()

	close(runtime.done)
	if stoppedAt.IsZero() {
		stoppedAt = time.Now()
	}
	s.logger.Info("automation.scheduler.shutdown", "shutdown_duration_ms", time.Since(stoppedAt).Milliseconds())
}

func (s *Scheduler) waitForRuntime(ctx context.Context, runtime *schedulerRuntime) error {
	if runtime == nil {
		return nil
	}
	select {
	case <-runtime.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("automation: shutdown scheduler runtime: %w", ctx.Err())
	}
}

// Shutdown is an alias for Stop to match daemon runtime conventions.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	return s.Stop(ctx)
}
