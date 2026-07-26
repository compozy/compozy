package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

// BuildJobResourceState builds the next scheduler plan from canonical automation.job records.
func (m *Manager) BuildJobResourceState(
	ctx context.Context,
	records []resources.Record[Job],
) (resources.ProjectionPlan, error) {
	if ctx == nil {
		return nil, errors.New("automation: job resource build context is required")
	}
	if m == nil {
		return nil, errors.New("automation: manager is required")
	}

	jobs := make([]Job, 0, len(records))
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		job := cloneJob(record.Spec)
		job.ID = strings.TrimSpace(record.ID)
		job.CreatedAt = record.CreatedAt.UTC()
		job.UpdatedAt = record.UpdatedAt.UTC()
		jobs = append(jobs, job)
	}
	sortJobs(jobs)

	effectiveJobs, err := m.applyJobOverlays(ctx, jobs)
	if err != nil {
		return nil, err
	}
	if m.jobResourceStateMatches(revision, jobs) {
		return &jobResourceProjectionPlan{
			revision:  revision,
			jobs:      cloneJobs(jobs),
			unchanged: true,
		}, nil
	}

	scheduler, err := m.buildSchedulerRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if err := m.loadSchedulerRegistrations(ctx, effectiveJobs, scheduler); err != nil {
		return nil, errors.Join(err, m.shutdownRuntimeComponent(ctx, "scheduler", scheduler))
	}

	return &jobResourceProjectionPlan{
		revision:   revision,
		operations: len(effectiveJobs),
		jobs:       cloneJobs(jobs),
		scheduler:  scheduler,
	}, nil
}

// ApplyJobResourceState atomically swaps the scheduler and desired job catalog.
func (m *Manager) ApplyJobResourceState(ctx context.Context, plan resources.ProjectionPlan) error {
	if ctx == nil {
		return errors.New("automation: job resource apply context is required")
	}
	if m == nil {
		return errors.New("automation: manager is required")
	}

	typed, ok := plan.(*jobResourceProjectionPlan)
	if !ok {
		return fmt.Errorf("automation: job resource plan has type %T", plan)
	}
	if typed.unchanged {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !m.jobResourceStateMatches(typed.revision, typed.jobs) {
			return errors.New("automation: unchanged job resource plan no longer matches live state")
		}
		return nil
	}
	if typed.scheduler == nil {
		return errors.New("automation: job resource plan scheduler is required")
	}

	m.mu.Lock()
	running := m.running
	m.mu.Unlock()

	if running {
		if err := typed.scheduler.Start(ctx); err != nil {
			return errors.Join(err, m.shutdownRuntimeComponent(ctx, "scheduler", typed.scheduler))
		}
	}

	nextJobs := jobMapFromSlice(typed.jobs)
	if !running {
		m.mu.Lock()
		m.projectedJobs = nextJobs
		m.jobRevision = typed.revision
		m.mu.Unlock()
		return m.shutdownRuntimeComponent(ctx, "scheduler", typed.scheduler)
	}

	m.mu.Lock()
	if running && !m.running {
		m.mu.Unlock()
		return errors.Join(ErrManagerNotRunning, m.shutdownRuntimeComponent(ctx, "scheduler", typed.scheduler))
	}
	oldScheduler := m.scheduler
	m.scheduler = typed.scheduler
	m.projectedJobs = nextJobs
	m.jobRevision = typed.revision
	m.mu.Unlock()

	if oldScheduler != nil {
		if err := m.shutdownRuntimeComponent(ctx, "scheduler", oldScheduler); err != nil {
			m.logger.Warn("automation.resource.job.cleanup_failed", "error", err)
		}
	}
	return nil
}

// BuildTriggerResourceState builds the next trigger-engine plan from canonical automation.trigger records.
func (m *Manager) BuildTriggerResourceState(
	ctx context.Context,
	records []resources.Record[Trigger],
) (resources.ProjectionPlan, error) {
	if ctx == nil {
		return nil, errors.New("automation: trigger resource build context is required")
	}
	if m == nil {
		return nil, errors.New("automation: manager is required")
	}

	triggers := make([]Trigger, 0, len(records))
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		trigger := cloneTrigger(record.Spec)
		trigger.ID = strings.TrimSpace(record.ID)
		trigger.CreatedAt = record.CreatedAt.UTC()
		trigger.UpdatedAt = record.UpdatedAt.UTC()
		if strings.EqualFold(strings.TrimSpace(trigger.Event), "webhook") &&
			strings.TrimSpace(trigger.WebhookID) == "" {
			trigger.WebhookID = stableConfigID("wbh", trigger.ID)
		}
		triggers = append(triggers, trigger)
	}
	sortTriggers(triggers)

	effectiveTriggers, err := m.applyTriggerOverlays(ctx, triggers)
	if err != nil {
		return nil, err
	}
	if m.triggerResourceStateMatches(revision, triggers) {
		return &triggerResourceProjectionPlan{
			revision:  revision,
			triggers:  cloneTriggers(triggers),
			unchanged: true,
		}, nil
	}

	engine, err := m.buildTriggerRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if err := m.loadTriggerRegistrations(effectiveTriggers, engine); err != nil {
		return nil, errors.Join(err, m.shutdownRuntimeComponent(ctx, "trigger engine", engine))
	}

	return &triggerResourceProjectionPlan{
		revision:   revision,
		operations: len(effectiveTriggers),
		triggers:   cloneTriggers(triggers),
		engine:     engine,
	}, nil
}

// ApplyTriggerResourceState atomically swaps the trigger engine and desired trigger catalog.
func (m *Manager) ApplyTriggerResourceState(ctx context.Context, plan resources.ProjectionPlan) error {
	if ctx == nil {
		return errors.New("automation: trigger resource apply context is required")
	}
	if m == nil {
		return errors.New("automation: manager is required")
	}

	typed, ok := plan.(*triggerResourceProjectionPlan)
	if !ok {
		return fmt.Errorf("automation: trigger resource plan has type %T", plan)
	}
	if typed.unchanged {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !m.triggerResourceStateMatches(typed.revision, typed.triggers) {
			return errors.New("automation: unchanged trigger resource plan no longer matches live state")
		}
		return nil
	}
	if typed.engine == nil {
		return errors.New("automation: trigger resource plan engine is required")
	}

	m.mu.Lock()
	running := m.running
	m.mu.Unlock()

	if running {
		if err := typed.engine.Start(ctx); err != nil {
			return errors.Join(err, m.shutdownRuntimeComponent(ctx, "trigger engine", typed.engine))
		}
	}

	nextTriggers := triggerMapFromSlice(typed.triggers)
	if !running {
		m.mu.Lock()
		m.projectedTriggers = nextTriggers
		m.triggerRevision = typed.revision
		m.mu.Unlock()
		return m.shutdownRuntimeComponent(ctx, "trigger engine", typed.engine)
	}

	m.mu.Lock()
	if running && !m.running {
		m.mu.Unlock()
		return errors.Join(ErrManagerNotRunning, m.shutdownRuntimeComponent(ctx, "trigger engine", typed.engine))
	}
	oldEngine := m.triggers
	m.triggers = typed.engine
	m.projectedTriggers = nextTriggers
	m.triggerRevision = typed.revision
	m.mu.Unlock()

	if oldEngine != nil {
		if err := m.shutdownRuntimeComponent(ctx, "trigger engine", oldEngine); err != nil {
			m.logger.Warn("automation.resource.trigger.cleanup_failed", "error", err)
		}
	}
	return nil
}
