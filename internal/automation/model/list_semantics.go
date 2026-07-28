package model

import (
	"sort"
	"strings"
)

// SortJobsForList orders jobs by source precedence, name, and id.
func SortJobsForList(jobs []Job) {
	sort.Slice(jobs, func(left int, right int) bool {
		leftJob := jobs[left]
		rightJob := jobs[right]
		return compareListKey(
			leftJob.Source,
			leftJob.Name,
			leftJob.ID,
			rightJob.Source,
			rightJob.Name,
			rightJob.ID,
		) < 0
	})
}

// SortTriggersForList orders triggers by source precedence, name, and id.
func SortTriggersForList(triggers []Trigger) {
	sort.Slice(triggers, func(left int, right int) bool {
		leftTrigger := triggers[left]
		rightTrigger := triggers[right]
		return compareListKey(
			leftTrigger.Source,
			leftTrigger.Name,
			leftTrigger.ID,
			rightTrigger.Source,
			rightTrigger.Name,
			rightTrigger.ID,
		) < 0
	})
}

func normalizeJobListQuery(query JobListQuery) JobListQuery {
	query.Scope = Scope(strings.TrimSpace(string(query.Scope)))
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.Source = JobSource(strings.TrimSpace(string(query.Source)))
	query.LoopName = strings.TrimSpace(query.LoopName)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Cursor = strings.TrimSpace(query.Cursor)
	return query
}

func normalizeTriggerListQuery(query TriggerListQuery) TriggerListQuery {
	query.Scope = Scope(strings.TrimSpace(string(query.Scope)))
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.Event = strings.TrimSpace(query.Event)
	query.Source = JobSource(strings.TrimSpace(string(query.Source)))
	query.LoopName = strings.TrimSpace(query.LoopName)
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Cursor = strings.TrimSpace(query.Cursor)
	return query
}

func jobMatchesListQuery(job Job, query JobListQuery) bool {
	if query.Scope != "" && job.Scope != query.Scope {
		return false
	}
	if query.WorkspaceID != "" && strings.TrimSpace(job.WorkspaceID) != query.WorkspaceID {
		return false
	}
	if query.Source != "" && job.Source != query.Source {
		return false
	}
	if query.LoopName != "" && !jobMatchesLoopName(job, query.LoopName) {
		return false
	}
	if query.Enabled != nil && job.Enabled != *query.Enabled {
		return false
	}
	if query.Search == "" {
		return true
	}
	searchFields := []string{
		job.Name,
		job.AgentName,
		job.Prompt,
		string(job.Scope),
		string(job.Source),
	}
	if job.Schedule != nil {
		searchFields = append(
			searchFields,
			string(job.Schedule.Mode),
			job.Schedule.Expr,
			job.Schedule.Interval,
			job.Schedule.Time,
		)
	}
	return listSearchMatches(query.Search, searchFields...)
}

// JobMatchesListQuery reports whether one job belongs to a public catalog query.
func JobMatchesListQuery(job Job, query JobListQuery) bool {
	return jobMatchesListQuery(job, normalizeJobListQuery(query))
}

func triggerMatchesListQuery(trigger Trigger, query TriggerListQuery) bool {
	if query.Scope != "" && trigger.Scope != query.Scope {
		return false
	}
	if query.WorkspaceID != "" && strings.TrimSpace(trigger.WorkspaceID) != query.WorkspaceID {
		return false
	}
	if query.Event != "" && strings.TrimSpace(trigger.Event) != query.Event {
		return false
	}
	if query.Source != "" && trigger.Source != query.Source {
		return false
	}
	if query.LoopName != "" && !triggerMatchesLoopName(trigger, query.LoopName) {
		return false
	}
	if query.Enabled != nil && trigger.Enabled != *query.Enabled {
		return false
	}
	if query.Search == "" {
		return true
	}
	if listSearchMatches(
		query.Search,
		trigger.Name,
		trigger.AgentName,
		trigger.Prompt,
		string(trigger.Scope),
		string(trigger.Source),
		trigger.Event,
		trigger.EndpointSlug,
		trigger.WebhookID,
	) {
		return true
	}
	for key, value := range trigger.Filter {
		if listSearchMatches(query.Search, key, value) {
			return true
		}
	}
	return false
}

// TriggerMatchesListQuery reports whether one trigger belongs to a public catalog query.
func TriggerMatchesListQuery(trigger Trigger, query TriggerListQuery) bool {
	return triggerMatchesListQuery(trigger, normalizeTriggerListQuery(query))
}

func listSearchMatches(search string, fields ...string) bool {
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), search) {
			return true
		}
	}
	return false
}

func jobMatchesLoopName(job Job, loopName string) bool {
	return job.IsLoopTarget() &&
		job.LoopTarget != nil &&
		strings.TrimSpace(job.LoopTarget.LoopName) == loopName
}

func triggerMatchesLoopName(trigger Trigger, loopName string) bool {
	return trigger.IsLoopTarget() &&
		trigger.LoopTarget != nil &&
		strings.TrimSpace(trigger.LoopTarget.LoopName) == loopName
}

func compareListKey(
	leftSource JobSource,
	leftName string,
	leftID string,
	rightSource JobSource,
	rightName string,
	rightID string,
) int {
	if sourceComparison := listSourceRank(leftSource) - listSourceRank(rightSource); sourceComparison != 0 {
		return sourceComparison
	}
	if nameComparison := strings.Compare(leftName, rightName); nameComparison != 0 {
		return nameComparison
	}
	return strings.Compare(leftID, rightID)
}

func listSourceRank(source JobSource) int {
	switch source {
	case JobSourceConfig:
		return 0
	case JobSourcePackage:
		return 1
	case JobSourceDynamic:
		return 2
	default:
		return 3
	}
}

// ListSourceRank returns the stable source precedence used by every automation catalog.
func ListSourceRank(source JobSource) int {
	return listSourceRank(source)
}
