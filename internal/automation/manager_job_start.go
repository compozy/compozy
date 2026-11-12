package automation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// DefaultJobWaitTimeout bounds an explicit completion wait.
const DefaultJobWaitTimeout = time.Minute

// ParseJobWaitTimeout validates the optional completion-wait budget.
func ParseJobWaitTimeout(wait bool, raw string) (time.Duration, error) {
	if !wait {
		if raw != "" {
			return 0, errors.New("automation: timeout requires wait=true")
		}
		return 0, nil
	}
	if raw == "" {
		return DefaultJobWaitTimeout, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 || timeout > 24*time.Hour {
		return 0, errors.New("automation: timeout must be a positive duration no greater than 24h")
	}
	return timeout, nil
}

type manualDispatchResult struct {
	run *Run
	err error
}

// StartJob returns the admission receipt, optionally waiting for completion within waitTimeout.
func (m *Manager) StartJob(ctx context.Context, id string, waitTimeout time.Duration) (Run, error) {
	if ctx == nil {
		return Run{}, errors.New("automation: start job context is required")
	}
	if waitTimeout < 0 || waitTimeout > 24*time.Hour {
		return Run{}, errors.New("automation: wait timeout must be between zero and 24h")
	}
	job, err := m.effectiveJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return Run{}, err
	}
	admitted := make(chan Run, 1)
	done := make(chan manualDispatchResult, 1)
	if err := m.startManualDispatch(ctx, job, admitted, done); err != nil {
		return Run{}, err
	}

	var receipt Run
	select {
	case receipt = <-admitted:
	case result := <-done:
		// Prefer the admission snapshot even if a short turn has already completed.
		select {
		case receipt = <-admitted:
			if waitTimeout == 0 {
				return receipt, nil
			}
		default:
		}
		return manualDispatchRun(result)
	case <-ctx.Done():
		return Run{}, ctx.Err()
	}
	if waitTimeout == 0 {
		return receipt, nil
	}
	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()
	select {
	case result := <-done:
		return manualDispatchRun(result)
	case <-timer.C:
		return receipt, nil
	case <-ctx.Done():
		return receipt, ctx.Err()
	}
}

func (m *Manager) startManualDispatch(
	ctx context.Context, job Job, admitted chan<- Run, done chan<- manualDispatchResult,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return ErrManagerNotRunning
	}
	runtimeCtx := m.runtimeCtx
	m.manualRuns.Go(func() {
		// Request deadlines bound the response; daemon shutdown owns the detached execution.
		dispatchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 168*time.Hour)
		defer cancel()
		stop := context.AfterFunc(runtimeCtx, cancel)
		defer stop()
		var once sync.Once
		run, err := m.dispatcher.Dispatch(dispatchCtx, DispatchRequest{
			Kind: DispatchKindManual, Job: &job,
			onAdmitted: func(run Run) { once.Do(func() { admitted <- run }) },
		})
		if err != nil {
			m.logger.Error("automation.manual_dispatch.failed", "job_id", job.ID, "error", err)
		}
		done <- manualDispatchResult{run: run, err: err}
	})
	return nil
}

func manualDispatchRun(result manualDispatchResult) (Run, error) {
	if result.run != nil {
		return *result.run, result.err
	}
	if result.err != nil {
		return Run{}, result.err
	}
	return Run{}, errors.New("automation: manual job dispatch returned no run")
}
