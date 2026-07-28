package automation

import (
	"context"
	"fmt"
)

func (m *Manager) loadStartupDefinitionsLocked(ctx context.Context) ([]Job, []Trigger, error) {
	if m.resourceDefinitionsEnabled() {
		projectedJobs, jobRevision, err := m.loadProjectedJobDefinitionsFromStore(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("automation: load projected job resources: %w", err)
		}
		projectedTriggers, triggerRevision, err := m.loadProjectedTriggerDefinitionsFromStore(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("automation: load projected trigger resources: %w", err)
		}
		m.projectedJobs = jobMapFromSlice(projectedJobs)
		m.jobRevision = jobRevision
		m.projectedTriggers = triggerMapFromSlice(projectedTriggers)
		m.triggerRevision = triggerRevision

		jobs, err := m.applyJobOverlays(ctx, m.projectedJobDefinitionsLocked())
		if err != nil {
			return nil, nil, fmt.Errorf("automation: load effective jobs: %w", err)
		}
		triggers, err := m.applyTriggerOverlays(ctx, m.projectedTriggerDefinitionsLocked())
		if err != nil {
			return nil, nil, fmt.Errorf("automation: load effective triggers: %w", err)
		}
		return jobs, triggers, nil
	}

	jobs, err := m.loadEffectiveJobs(ctx, JobListQuery{})
	if err != nil {
		return nil, nil, fmt.Errorf("automation: load effective jobs: %w", err)
	}
	triggers, err := m.loadEffectiveTriggers(ctx, TriggerListQuery{})
	if err != nil {
		return nil, nil, fmt.Errorf("automation: load effective triggers: %w", err)
	}
	return jobs, triggers, nil
}
