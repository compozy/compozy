package automation

import (
	"context"
	"errors"
	"fmt"
)

// Start synchronizes TOML definitions into persistence, loads effective
// automation state, and starts the runtime scheduler and trigger engine.
func (m *Manager) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("automation: manager start context is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	syncStats, err := m.syncConfigDefinitions(ctx)
	if err != nil {
		return fmt.Errorf("automation: sync config definitions: %w", err)
	}
	if err := m.recoverAcceptedSuggestionsLocked(ctx); err != nil {
		return err
	}

	jobs, triggers, err := m.loadStartupDefinitionsLocked(ctx)
	if err != nil {
		return err
	}

	runtimeCtx, runtimeCancel := context.WithCancel(context.WithoutCancel(ctx))
	scheduler, triggerEngine, err := m.buildRuntimes(ctx)
	if err != nil {
		runtimeCancel()
		return err
	}

	if err := m.loadSchedulerRegistrations(ctx, jobs, scheduler); err != nil {
		return errors.Join(
			fmt.Errorf("automation: register scheduler jobs: %w", err),
			m.shutdownStartupRuntime(ctx, runtimeCancel, scheduler, triggerEngine),
		)
	}
	if err := m.loadTriggerRegistrations(triggers, triggerEngine); err != nil {
		return errors.Join(
			fmt.Errorf("automation: register trigger definitions: %w", err),
			m.shutdownStartupRuntime(ctx, runtimeCancel, scheduler, triggerEngine),
		)
	}
	if err := triggerEngine.Start(ctx); err != nil {
		return errors.Join(
			fmt.Errorf("automation: start trigger engine: %w", err),
			m.shutdownStartupRuntime(ctx, runtimeCancel, scheduler, triggerEngine),
		)
	}
	if err := scheduler.Start(ctx); err != nil {
		return errors.Join(
			fmt.Errorf("automation: start scheduler: %w", err),
			m.shutdownStartupRuntime(ctx, runtimeCancel, scheduler, triggerEngine),
		)
	}

	m.running = true
	m.runtimeCtx = runtimeCtx
	m.runtimeCancel = runtimeCancel
	m.scheduler = scheduler
	m.triggers = triggerEngine
	m.lastSync = syncStats

	m.logger.Info(
		"automation.manager.started",
		"jobs_synced", syncStats.JobsSynced,
		"triggers_synced", syncStats.TriggersSynced,
		"jobs_removed", syncStats.JobsRemoved,
		"triggers_removed", syncStats.TriggersRemoved,
		"jobs_loaded", len(jobs),
		"triggers_loaded", len(triggers),
	)
	return nil
}

// Shutdown stops trigger ingestion, cancels in-flight work, and shuts down the
// runtime scheduler.
func (m *Manager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("automation: manager shutdown context is required")
	}

	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}

	cancel := m.runtimeCancel
	scheduler := m.scheduler
	triggerEngine := m.triggers
	m.running = false
	m.runtimeCtx = nil
	m.runtimeCancel = nil
	m.scheduler = nil
	m.triggers = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var errs []error
	if triggerEngine != nil {
		if err := triggerEngine.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if scheduler != nil {
		if err := scheduler.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	m.logger.Info("automation.manager.shutdown")
	return errors.Join(errs...)
}
