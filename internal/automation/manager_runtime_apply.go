package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/session"
)

func (m *Manager) applyJobToRuntime(ctx context.Context, job Job) error {
	scheduler := m.schedulerSnapshot()
	if scheduler == nil {
		return nil
	}

	_, err := scheduler.State(job.ID)
	switch {
	case err == nil:
		_, err = scheduler.Update(ctx, job)
		return err
	case errors.Is(err, ErrScheduledJobNotFound):
		if !job.Enabled {
			return nil
		}
		_, err = scheduler.Register(ctx, job)
		return err
	default:
		return err
	}
}

func (m *Manager) applyTriggerToRuntime(trigger Trigger) error {
	engine := m.triggerEngineSnapshot()
	if engine == nil {
		return nil
	}

	registration, shouldRegister := m.runtimeTriggerRegistration(trigger)
	if !shouldRegister {
		if err := engine.Unregister(trigger.ID); err != nil && !errors.Is(err, ErrTriggerNotFound) {
			return err
		}
		return nil
	}

	if err := engine.Update(registration); err != nil {
		if errors.Is(err, ErrTriggerNotFound) {
			return engine.Register(registration)
		}
		return err
	}
	return nil
}

func (m *Manager) runtimeTriggerRegistration(trigger Trigger) (TriggerRegistration, bool) {
	if !trigger.Enabled {
		return TriggerRegistration{}, false
	}

	registration := TriggerRegistration{Trigger: cloneTrigger(trigger)}
	return registration, true
}

func (m *Manager) isRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *Manager) schedulerSnapshot() *Scheduler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scheduler
}

func (m *Manager) triggerEngineSnapshot() *TriggerEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.triggers
}

func (m *Manager) lastSyncSnapshot() SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSync
}

func (m *Manager) triggerRuntime() (*TriggerEngine, context.Context, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running || m.triggers == nil || m.runtimeCtx == nil {
		return nil, nil, false
	}
	return m.triggers, m.runtimeCtx, true
}

func (m *Manager) fireSessionCreated(ctx context.Context, sess *session.Session) {
	if sess == nil {
		return
	}
	engine, runtimeCtx, ok := m.triggerRuntime()
	if !ok {
		return
	}
	mergedCtx, cancel := mergedRuntimeContext(ctx, runtimeCtx)
	defer cancel()
	if _, err := engine.FireSessionCreated(mergedCtx, sess); err != nil {
		m.logger.Warn(
			"automation.manager.session_created_trigger_failed",
			"session_id",
			strings.TrimSpace(sess.ID),
			"error",
			err,
		)
	}
}

func (m *Manager) fireSessionStopped(ctx context.Context, sess *session.Session) {
	if sess == nil {
		return
	}
	engine, runtimeCtx, ok := m.triggerRuntime()
	if !ok {
		return
	}
	mergedCtx, cancel := mergedRuntimeContext(ctx, runtimeCtx)
	defer cancel()
	if _, err := engine.FireSessionStopped(mergedCtx, sess); err != nil {
		m.logger.Warn(
			"automation.manager.session_stopped_trigger_failed",
			"session_id",
			strings.TrimSpace(sess.ID),
			"error",
			err,
		)
	}
}

func (m *Manager) cleanupCreatedJob(ctx context.Context, jobID string) error {
	var errs []error
	if err := m.store.DeleteJob(ctx, jobID); err != nil {
		errs = append(errs, fmt.Errorf("automation: delete created job %q: %w", jobID, err))
	}
	if scheduler := m.schedulerSnapshot(); scheduler != nil {
		if err := scheduler.Unregister(ctx, jobID); err != nil && !errors.Is(err, ErrScheduledJobNotFound) {
			errs = append(errs, fmt.Errorf("automation: unregister created job %q: %w", jobID, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) cleanupCreatedTrigger(ctx context.Context, triggerID string) error {
	var errs []error
	if current, err := m.store.GetTrigger(ctx, triggerID); err == nil {
		if secretErr := m.deleteOwnedWebhookSecretIfPresent(ctx, current); secretErr != nil {
			errs = append(
				errs,
				fmt.Errorf("automation: delete created trigger webhook secret %q: %w", triggerID, secretErr),
			)
		}
	}
	if err := m.store.DeleteTrigger(ctx, triggerID); err != nil {
		errs = append(errs, fmt.Errorf("automation: delete created trigger %q: %w", triggerID, err))
	}
	if engine := m.triggerEngineSnapshot(); engine != nil {
		if err := engine.Unregister(triggerID); err != nil && !errors.Is(err, ErrTriggerNotFound) {
			errs = append(errs, fmt.Errorf("automation: unregister created trigger %q: %w", triggerID, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) shutdownStartupRuntime(
	ctx context.Context,
	runtimeCancel context.CancelFunc,
	scheduler *Scheduler,
	triggerEngine *TriggerEngine,
) error {
	if runtimeCancel != nil {
		runtimeCancel()
	}

	var errs []error
	if err := m.shutdownRuntimeComponent(ctx, "trigger engine", triggerEngine); err != nil {
		errs = append(errs, err)
	}
	if err := m.shutdownRuntimeComponent(ctx, "scheduler", scheduler); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (m *Manager) shutdownRuntimeComponent(ctx context.Context, name string, component managerRuntimeComponent) error {
	if component == nil {
		return nil
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), managerRuntimeCleanupTimeout)
	defer cancel()

	if err := component.Shutdown(cleanupCtx); err != nil {
		return fmt.Errorf("automation: shutdown %s: %w", name, err)
	}
	return nil
}
