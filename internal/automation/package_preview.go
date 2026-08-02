package automation

import (
	"context"
	"errors"
)

// EffectivePackageAutomation applies persisted enabled overlays to candidate package definitions.
// It does not publish the definitions or change scheduler state.
func (m *Manager) EffectivePackageAutomation(
	ctx context.Context,
	jobs []Job,
	triggers []Trigger,
) ([]Job, []Trigger, error) {
	if ctx == nil {
		return nil, nil, errors.New("automation: package preview context is required")
	}
	if m == nil {
		return nil, nil, errors.New("automation: package preview manager is required")
	}
	effectiveJobs, err := m.applyJobOverlays(ctx, jobs)
	if err != nil {
		return nil, nil, err
	}
	effectiveTriggers, err := m.applyTriggerOverlays(ctx, triggers)
	if err != nil {
		return nil, nil, err
	}
	return effectiveJobs, effectiveTriggers, nil
}
