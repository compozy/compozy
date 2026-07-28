package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"time"
)

func (m *Manager) effectiveJob(ctx context.Context, id string) (Job, error) {
	if m.resourceDefinitionsEnabled() {
		job, err := m.projectedJobDefinition(id)
		if err != nil {
			return Job{}, err
		}
		return m.effectiveJobFromStored(ctx, job)
	}

	job, err := m.store.GetJob(ctx, id)
	if err != nil {
		return Job{}, err
	}
	return m.effectiveJobFromStored(ctx, job)
}

func (m *Manager) effectiveJobFromStored(ctx context.Context, job Job) (Job, error) {
	effective := cloneJob(job)
	if !isOverlayManagedSource(effective.Source) {
		return effective, nil
	}

	overlay, err := m.store.GetJobEnabledOverlay(ctx, effective.ID)
	switch {
	case err == nil:
		effective.Enabled = overlay.EnabledOverride
	case errors.Is(err, ErrJobOverlayNotFound):
	default:
		return Job{}, err
	}

	return effective, nil
}

func (m *Manager) effectiveTrigger(ctx context.Context, id string) (Trigger, error) {
	if m.resourceDefinitionsEnabled() {
		trigger, err := m.projectedTriggerDefinition(id)
		if err != nil {
			return Trigger{}, err
		}
		return m.effectiveTriggerFromStored(ctx, trigger)
	}

	trigger, err := m.store.GetTrigger(ctx, id)
	if err != nil {
		return Trigger{}, err
	}
	return m.effectiveTriggerFromStored(ctx, trigger)
}

func (m *Manager) effectiveTriggerFromStored(ctx context.Context, trigger Trigger) (Trigger, error) {
	effective := cloneTrigger(trigger)
	if !isOverlayManagedSource(effective.Source) {
		return effective, nil
	}

	overlay, err := m.store.GetTriggerEnabledOverlay(ctx, effective.ID)
	switch {
	case err == nil:
		effective.Enabled = overlay.EnabledOverride
	case errors.Is(err, ErrTriggerOverlayNotFound):
	default:
		return Trigger{}, err
	}

	return effective, nil
}

func (m *Manager) buildRuntimes(ctx context.Context) (*Scheduler, *TriggerEngine, error) {
	scheduler, err := m.buildSchedulerRuntime(ctx)
	if err != nil {
		return nil, nil, err
	}

	triggerEngine, err := m.buildTriggerRuntime(ctx)
	if err != nil {
		return nil, nil, errors.Join(
			err,
			m.shutdownRuntimeComponent(ctx, "scheduler", scheduler),
		)
	}

	return scheduler, triggerEngine, nil
}

func (m *Manager) buildSchedulerRuntime(_ context.Context) (*Scheduler, error) {
	location, err := time.LoadLocation(strings.TrimSpace(m.config.Timezone))
	if err != nil {
		return nil, fmt.Errorf("automation: load manager timezone %q: %w", m.config.Timezone, err)
	}

	schedulerOpts := []SchedulerOption{
		WithSchedulerLogger(m.logger),
		WithSchedulerLocation(location),
		WithSchedulerStore(m.store),
		WithSchedulerCatchUpPolicyResolver(m.defaultSchedulerCatchUpPolicy),
	}
	schedulerOpts = append(schedulerOpts, m.schedulerOptions...)
	scheduler, err := NewScheduler(m.dispatcher, schedulerOpts...)
	if err != nil {
		return nil, fmt.Errorf("automation: construct scheduler: %w", err)
	}
	return scheduler, nil
}

func (m *Manager) buildTriggerRuntime(_ context.Context) (*TriggerEngine, error) {
	triggerOpts := []TriggerEngineOption{
		WithTriggerEngineLogger(m.logger),
		WithTriggerEngineHookSessionResolver(m.sessions),
		WithTriggerEngineWebhookSecretResolver(m.webhookSecrets),
	}
	if webhookDeliveries, ok := m.store.(WebhookDeliveryStore); ok {
		triggerOpts = append(triggerOpts, WithTriggerEngineWebhookDeliveryStore(webhookDeliveries))
	}
	triggerOpts = append(triggerOpts, m.triggerOptions...)
	triggerEngine, err := NewTriggerEngine(m.dispatcher, triggerOpts...)
	if err != nil {
		return nil, fmt.Errorf("automation: construct trigger engine: %w", err)
	}
	return triggerEngine, nil
}

func (m *Manager) loadSchedulerRegistrations(ctx context.Context, jobs []Job, scheduler *Scheduler) error {
	for _, job := range jobs {
		if _, err := scheduler.Register(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) loadTriggerRegistrations(triggers []Trigger, engine *TriggerEngine) error {
	for _, trigger := range triggers {
		registration, shouldRegister := m.runtimeTriggerRegistration(trigger)
		if !shouldRegister {
			if strings.EqualFold(trigger.Event, "webhook") && trigger.Enabled {
				m.logger.Warn(
					"automation.trigger.skipped_webhook_registration",
					"trigger_id", trigger.ID,
					"trigger_name", trigger.Name,
				)
			}
			continue
		}
		if err := engine.Register(registration); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) syncConfigDefinitions(ctx context.Context) (SyncStats, error) {
	desiredJobs := make([]Job, 0, len(m.config.Jobs))
	for idx, raw := range m.config.Jobs {
		job, err := m.resolveConfigJob(ctx, raw)
		if err != nil {
			return SyncStats{}, fmt.Errorf("automation: resolve config job %d: %w", idx, err)
		}
		desiredJobs = append(desiredJobs, job)
	}
	sortJobs(desiredJobs)

	desiredTriggers := make([]Trigger, 0, len(m.config.Triggers))
	for idx, raw := range m.config.Triggers {
		trigger, err := m.resolveConfigTrigger(ctx, raw)
		if err != nil {
			return SyncStats{}, fmt.Errorf("automation: resolve config trigger %d: %w", idx, err)
		}
		desiredTriggers = append(desiredTriggers, trigger)
	}
	sortTriggers(desiredTriggers)

	return m.SyncManagedDefinitions(ctx, JobSourceConfig, desiredJobs, desiredTriggers)
}
