package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// SyncManagedDefinitions reconciles one daemon-managed automation source
// against the persisted store and runtime registries.
func (m *Manager) SyncManagedDefinitions(
	ctx context.Context,
	source JobSource,
	desiredJobs []Job,
	desiredTriggers []Trigger,
) (SyncStats, error) {
	if ctx == nil {
		return SyncStats{}, errors.New("automation: sync managed definitions context is required")
	}
	switch source {
	case JobSourceConfig, JobSourcePackage:
	default:
		return SyncStats{}, fmt.Errorf("automation: managed sync requires config or package source, got %q", source)
	}

	jobs := make([]Job, 0, len(desiredJobs))
	for _, job := range desiredJobs {
		next := cloneJob(job)
		next.Source = source
		jobs = append(jobs, next)
	}
	sortJobs(jobs)

	triggers := make([]Trigger, 0, len(desiredTriggers))
	for _, trigger := range desiredTriggers {
		next := cloneTrigger(trigger)
		next.Source = source
		triggers = append(triggers, next)
	}
	sortTriggers(triggers)

	if m.resourceDefinitionsEnabled() {
		return m.syncManagedResourceDefinitions(ctx, source, jobs, triggers)
	}

	jobsSynced, jobsRemoved, err := m.syncJobsForSource(ctx, source, jobs)
	if err != nil {
		return SyncStats{}, err
	}
	triggersSynced, triggersRemoved, err := m.syncTriggersForSource(ctx, source, triggers)
	if err != nil {
		return SyncStats{}, err
	}

	stats := SyncStats{
		JobsSynced:      jobsSynced,
		TriggersSynced:  triggersSynced,
		JobsRemoved:     jobsRemoved,
		TriggersRemoved: triggersRemoved,
		SyncedAt:        m.now().UTC(),
	}
	m.logger.Info(
		"automation.managed.sync",
		"source", source,
		"jobs_synced", stats.JobsSynced,
		"triggers_synced", stats.TriggersSynced,
		"jobs_removed", stats.JobsRemoved,
		"triggers_removed", stats.TriggersRemoved,
	)
	return stats, nil
}

func (m *Manager) syncJobsForSource(ctx context.Context, source JobSource, desired []Job) (int, int, error) {
	existing, err := m.store.ListJobDefinitions(ctx, source)
	if err != nil {
		return 0, 0, err
	}

	existingByID := make(map[string]Job, len(existing))
	for _, job := range existing {
		existingByID[job.ID] = job
	}

	desiredByID := make(map[string]Job, len(desired))
	synced := 0
	for _, job := range desired {
		if err := m.validateJobLoopTarget(ctx, job); err != nil {
			return 0, 0, err
		}
		desiredByID[job.ID] = job
		current, exists := existingByID[job.ID]
		switch {
		case !exists:
			if _, err := m.store.CreateJob(ctx, job); err != nil {
				return 0, 0, err
			}
		case current.Source != source:
			return 0, 0, fmt.Errorf("automation: managed job %q conflicts with non-%s source", job.ID, source)
		case !sameJobDefinition(current, job):
			if _, err := m.store.UpdateJob(ctx, job); err != nil {
				return 0, 0, err
			}
		}
		synced++
	}

	removed := 0
	for id := range existingByID {
		if _, ok := desiredByID[id]; ok {
			continue
		}
		if err := m.store.DeleteJob(ctx, id); err != nil {
			return 0, 0, err
		}
		removed++
	}

	return synced, removed, nil
}

func (m *Manager) syncTriggersForSource(
	ctx context.Context,
	source JobSource,
	desired []Trigger,
) (int, int, error) {
	existing, err := m.store.ListTriggerDefinitions(ctx, source)
	if err != nil {
		return 0, 0, err
	}

	existingByID := make(map[string]Trigger, len(existing))
	for _, trigger := range existing {
		existingByID[trigger.ID] = trigger
	}

	desiredByID := make(map[string]Trigger, len(desired))
	synced := 0
	for _, trigger := range desired {
		if err := m.validateTriggerLoopTarget(ctx, trigger); err != nil {
			return 0, 0, err
		}
		desiredByID[trigger.ID] = trigger
		current, exists := existingByID[trigger.ID]
		switch {
		case !exists:
			if _, err := m.store.CreateTrigger(ctx, trigger); err != nil {
				return 0, 0, err
			}
		case current.Source != source:
			return 0, 0, fmt.Errorf("automation: managed trigger %q conflicts with non-%s source", trigger.ID, source)
		case !sameTriggerDefinition(current, trigger):
			if _, err := m.store.UpdateTrigger(ctx, trigger); err != nil {
				return 0, 0, err
			}
		}
		synced++
	}

	removed := 0
	for id := range existingByID {
		if _, ok := desiredByID[id]; ok {
			continue
		}
		if err := m.store.DeleteTrigger(ctx, id); err != nil {
			return 0, 0, err
		}
		if err := m.deleteOwnedWebhookSecretIfPresent(ctx, existingByID[id]); err != nil {
			return 0, 0, err
		}
		removed++
	}

	return synced, removed, nil
}

func (m *Manager) resolveConfigJob(ctx context.Context, raw aghconfig.AutomationJob) (Job, error) {
	workspaceID, err := m.resolveConfigWorkspace(ctx, raw.Scope, raw.Workspace)
	if err != nil {
		return Job{}, err
	}

	fireLimit := raw.FireLimit
	if fireLimit.Max == 0 || strings.TrimSpace(fireLimit.Window) == "" {
		fireLimit = m.config.DefaultFireLimit
	}
	if fireLimit.Max == 0 || strings.TrimSpace(fireLimit.Window) == "" {
		fireLimit = DefaultFireLimitConfig()
	}

	retry := raw.Retry
	if retry.Strategy == "" {
		retry = DefaultRetryConfig()
	}

	schedule := raw.Schedule
	job := Job{
		ID:          configJobID(raw.Scope, workspaceID, raw.Name),
		Scope:       raw.Scope,
		Name:        strings.TrimSpace(raw.Name),
		AgentName:   strings.TrimSpace(raw.AgentName),
		WorkspaceID: workspaceID,
		Prompt:      strings.TrimSpace(raw.Prompt),
		Schedule:    &schedule,
		Task:        cloneJobTaskConfig(raw.Task),
		TargetKind:  raw.TargetKind,
		LoopTarget:  cloneLoopTarget(raw.LoopTarget),
		Enabled:     raw.Enabled,
		Retry:       retry,
		FireLimit:   fireLimit,
		Source:      JobSourceConfig,
	}
	if err := job.Validate("job"); err != nil {
		return Job{}, fmt.Errorf("automation: resolve config job %q: %w", strings.TrimSpace(raw.Name), err)
	}
	return job, nil
}

func (m *Manager) resolveConfigTrigger(ctx context.Context, raw aghconfig.AutomationTrigger) (Trigger, error) {
	workspaceID, err := m.resolveConfigWorkspace(ctx, raw.Scope, raw.Workspace)
	if err != nil {
		return Trigger{}, err
	}

	fireLimit := raw.FireLimit
	if fireLimit.Max == 0 || strings.TrimSpace(fireLimit.Window) == "" {
		fireLimit = m.config.DefaultFireLimit
	}
	if fireLimit.Max == 0 || strings.TrimSpace(fireLimit.Window) == "" {
		fireLimit = DefaultFireLimitConfig()
	}

	retry := raw.Retry
	if retry.Strategy == "" {
		retry = DefaultRetryConfig()
	}

	trigger := Trigger{
		ID:               configTriggerID(raw.Scope, workspaceID, raw.Name),
		Scope:            raw.Scope,
		Name:             strings.TrimSpace(raw.Name),
		AgentName:        strings.TrimSpace(raw.AgentName),
		WorkspaceID:      workspaceID,
		Prompt:           strings.TrimSpace(raw.Prompt),
		Event:            strings.TrimSpace(raw.Event),
		Filter:           cloneFilter(raw.Filter),
		TargetKind:       raw.TargetKind,
		LoopTarget:       cloneLoopTarget(raw.LoopTarget),
		Enabled:          raw.Enabled,
		Retry:            retry,
		FireLimit:        fireLimit,
		Source:           JobSourceConfig,
		EndpointSlug:     strings.TrimSpace(raw.EndpointSlug),
		WebhookSecretRef: strings.TrimSpace(raw.WebhookSecretRef),
	}
	if strings.EqualFold(trigger.Event, "webhook") {
		trigger.WebhookID = configWebhookID(raw.Scope, workspaceID, raw.Name)
	}
	if err := trigger.Validate("trigger"); err != nil {
		return Trigger{}, fmt.Errorf("automation: resolve config trigger %q: %w", strings.TrimSpace(raw.Name), err)
	}
	return trigger, nil
}

func (m *Manager) resolveConfigWorkspace(
	ctx context.Context,
	scope Scope,
	workspaceRef string,
) (string, error) {
	if scope == AutomationScopeGlobal {
		return "", nil
	}

	trimmedRef := strings.TrimSpace(workspaceRef)
	if trimmedRef == "" {
		return "", errors.New("automation: workspace reference is required")
	}

	var (
		resolved workspacepkg.ResolvedWorkspace
		err      error
	)
	if isPathLikeWorkspaceRef(trimmedRef) {
		normalizedPath, normalizeErr := aghconfig.ResolvePath(trimmedRef)
		if normalizeErr != nil {
			return "", fmt.Errorf("automation: resolve config workspace %q: %w", trimmedRef, normalizeErr)
		}
		resolved, err = m.workspaceResolver.ResolveOrRegister(ctx, normalizedPath)
	} else {
		resolved, err = m.workspaceResolver.Resolve(ctx, trimmedRef)
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved.ID) == "" {
		return "", errors.New("automation: resolved workspace id is required")
	}
	return strings.TrimSpace(resolved.ID), nil
}

func (m *Manager) persistJobOverlay(ctx context.Context, definition Job, enabled bool) error {
	if !isOverlayManagedSource(definition.Source) {
		return ErrOverlayRequiresConfigSource
	}
	if enabled == definition.Enabled {
		return m.store.DeleteJobEnabledOverlay(ctx, definition.ID)
	}
	_, err := m.store.SetJobEnabledOverlay(ctx, JobEnabledOverlay{
		JobID:           definition.ID,
		EnabledOverride: enabled,
	})
	return err
}

func (m *Manager) persistTriggerOverlay(ctx context.Context, definition Trigger, enabled bool) error {
	if !isOverlayManagedSource(definition.Source) {
		return ErrOverlayRequiresConfigSource
	}
	if enabled == definition.Enabled {
		return m.store.DeleteTriggerEnabledOverlay(ctx, definition.ID)
	}
	_, err := m.store.SetTriggerEnabledOverlay(ctx, TriggerEnabledOverlay{
		TriggerID:       definition.ID,
		EnabledOverride: enabled,
	})
	return err
}
