package automation

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// WithLoopStarter injects the loop start boundary used for loop-target automations.
func WithLoopStarter(starter LoopStarter) Option {
	return func(opts *managerOptions) {
		opts.loopStarter = starter
	}
}

func (m *Manager) validateJobLoopTarget(ctx context.Context, job Job) error {
	if !job.IsLoopTarget() {
		return nil
	}
	return m.validateLoopTarget(ctx, job.LoopTarget, loopStartKindForAutomation(&job, nil))
}

func (m *Manager) validateTriggerLoopTarget(ctx context.Context, trigger Trigger) error {
	if !trigger.IsLoopTarget() {
		return nil
	}
	return m.validateLoopTarget(ctx, trigger.LoopTarget, loopStartKindForAutomation(nil, &trigger))
}

func (m *Manager) validateLoopTarget(ctx context.Context, target *LoopTarget, kind LoopStartKind) error {
	if target == nil {
		return nil
	}
	if m == nil || m.loopStarter == nil {
		return fmt.Errorf("%w: loop target validation requires loop starter", ErrManagerNotRunning)
	}
	return validateLoopTargetWithStarter(ctx, m.loopStarter, target, kind)
}

func (m *Manager) defaultSchedulerCatchUpPolicy(
	ctx context.Context,
	job Job,
) (SchedulerCatchUpPolicy, error) {
	if !job.IsLoopTarget() {
		return SchedulerCatchUpPolicySkipMissed, nil
	}
	resolver, ok := m.loopStarter.(LoopCatchUpPolicyResolver)
	if !ok || resolver == nil || job.LoopTarget == nil {
		return SchedulerCatchUpPolicySkipMissed, nil
	}
	return resolver.DefaultLoopCatchUpPolicy(ctx, LoopCatchUpPolicyRequest{
		WorkspaceID: strings.TrimSpace(job.LoopTarget.WorkspaceID),
		LoopName:    strings.TrimSpace(job.LoopTarget.LoopName),
		Kind:        loopStartKindForAutomation(&job, nil),
	})
}

func targetKindOrDefault(kind TargetKind, target *LoopTarget) TargetKind {
	normalized := TargetKind(strings.TrimSpace(string(kind)))
	if normalized != "" {
		return normalized
	}
	if target != nil {
		return TargetKindLoop
	}
	return TargetKindAgent
}

func cloneLoopTarget(target *LoopTarget) *LoopTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	cloned.Inputs = cloneJSONMap(target.Inputs)
	cloned.InputMapping = cloneStringMap(target.InputMapping)
	cloned.NetworkParticipation = cloneParticipationRequest(target.NetworkParticipation)
	return &cloned
}

func sameLoopTarget(left *LoopTarget, right *LoopTarget) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.WorkspaceID == right.WorkspaceID &&
			left.LoopName == right.LoopName &&
			sameJSONMap(left.Inputs, right.Inputs) &&
			sameStringMap(left.InputMapping, right.InputMapping) &&
			reflect.DeepEqual(left.NetworkParticipation, right.NetworkParticipation)
	}
}

func sameJSONMap(left map[string]any, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if !reflect.DeepEqual(value, right[key]) {
			return false
		}
	}
	return true
}
