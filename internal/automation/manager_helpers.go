package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	modelpkg "github.com/compozy/agh/internal/automation/model"
	taskpkg "github.com/compozy/agh/internal/task"
)

func countEnabledJobs(jobs []Job) int {
	count := 0
	for _, job := range jobs {
		if job.Enabled {
			count++
		}
	}
	return count
}

func countEnabledTriggers(triggers []Trigger) int {
	count := 0
	for _, trigger := range triggers {
		if trigger.Enabled {
			count++
		}
	}
	return count
}

func configJobID(scope Scope, workspaceID string, name string) string {
	return stableConfigID("jobcfg", string(scope), workspaceID, name)
}

func configTriggerID(scope Scope, workspaceID string, name string) string {
	return stableConfigID("trgcfg", string(scope), workspaceID, name)
}

func configWebhookID(scope Scope, workspaceID string, name string) string {
	return stableConfigID("wbh", string(scope), workspaceID, name)
}

func stableConfigID(prefix string, parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	if prefix == "wbh" {
		return "wbh_" + hex.EncodeToString(sum[:8])
	}
	return prefix + "_" + hex.EncodeToString(sum[:8])
}

func isPathLikeWorkspaceRef(ref string) bool {
	trimmedRef := strings.TrimSpace(ref)
	return filepath.IsAbs(trimmedRef) ||
		strings.HasPrefix(trimmedRef, ".") ||
		strings.HasPrefix(trimmedRef, "~") ||
		strings.ContainsAny(trimmedRef, `/\`)
}

func sameJobDefinition(left Job, right Job) bool {
	leftKind := targetKindOrDefault(left.TargetKind, left.LoopTarget)
	rightKind := targetKindOrDefault(right.TargetKind, right.LoopTarget)
	return left.ID == right.ID &&
		left.Scope == right.Scope &&
		left.Name == right.Name &&
		leftKind == rightKind &&
		left.AgentName == right.AgentName &&
		left.WorkspaceID == right.WorkspaceID &&
		left.Prompt == right.Prompt &&
		sameSchedule(left.Schedule, right.Schedule) &&
		sameJobTaskConfig(left.Task, right.Task) &&
		sameLoopTarget(left.LoopTarget, right.LoopTarget) &&
		left.Enabled == right.Enabled &&
		left.Retry == right.Retry &&
		left.FireLimit == right.FireLimit &&
		left.Source == right.Source
}

func sameTriggerDefinition(left Trigger, right Trigger) bool {
	leftKind := targetKindOrDefault(left.TargetKind, left.LoopTarget)
	rightKind := targetKindOrDefault(right.TargetKind, right.LoopTarget)
	return left.ID == right.ID &&
		left.Scope == right.Scope &&
		left.Name == right.Name &&
		leftKind == rightKind &&
		left.AgentName == right.AgentName &&
		left.WorkspaceID == right.WorkspaceID &&
		left.Prompt == right.Prompt &&
		left.Event == right.Event &&
		sameStringMap(left.Filter, right.Filter) &&
		sameLoopTarget(left.LoopTarget, right.LoopTarget) &&
		left.Enabled == right.Enabled &&
		left.Retry == right.Retry &&
		left.FireLimit == right.FireLimit &&
		left.Source == right.Source &&
		left.WebhookID == right.WebhookID &&
		left.EndpointSlug == right.EndpointSlug &&
		left.WebhookSecretRef == right.WebhookSecretRef
}

func sameSchedule(left *ScheduleSpec, right *ScheduleSpec) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func sameStringMap(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func scheduledJobStatesFromDurable(states []SchedulerState) []ScheduledJobState {
	scheduled := make([]ScheduledJobState, 0, len(states))
	for _, state := range states {
		scheduled = append(scheduled, stateFromDurableState(state, false))
	}
	sort.Slice(scheduled, func(i, j int) bool {
		return scheduled[i].JobID < scheduled[j].JobID
	})
	return scheduled
}

func sortJobs(jobs []Job) {
	modelpkg.SortJobsForList(jobs)
}

func sortTriggers(triggers []Trigger) {
	modelpkg.SortTriggersForList(triggers)
}

func cloneJob(job Job) Job {
	cloned := job
	if job.Schedule != nil {
		schedule := *job.Schedule
		cloned.Schedule = &schedule
	}
	cloned.Task = cloneJobTaskConfig(job.Task)
	cloned.LoopTarget = cloneLoopTarget(job.LoopTarget)
	return cloned
}

func cloneJobTaskConfig(config *JobTaskConfig) *JobTaskConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	if config.Owner != nil {
		owner := *config.Owner
		cloned.Owner = &owner
	}
	cloned.NetworkParticipation = cloneParticipationRequest(config.NetworkParticipation)
	return &cloned
}

func sameJobTaskConfig(left *JobTaskConfig, right *JobTaskConfig) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Title == right.Title &&
			left.Description == right.Description &&
			reflect.DeepEqual(left.NetworkParticipation, right.NetworkParticipation) &&
			sameTaskOwnership(left.Owner, right.Owner)
	}
}

func sameTaskOwnership(left *taskpkg.Ownership, right *taskpkg.Ownership) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Kind == right.Kind && left.Ref == right.Ref
	}
}

func cloneTrigger(trigger Trigger) Trigger {
	cloned := trigger
	cloned.Filter = cloneFilter(trigger.Filter)
	cloned.LoopTarget = cloneLoopTarget(trigger.LoopTarget)
	return cloned
}

func cloneFilter(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}
