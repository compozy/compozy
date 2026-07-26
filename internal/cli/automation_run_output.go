package cli

import (
	"sort"
	"strconv"

	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func automationRunBundle(item RunRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Automation Run", []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: automationTargetValue, Value: stringOrDash(displayRunTarget(item))},
				{Label: "Job ID", Value: stringOrDash(item.JobID)},
				{Label: "Trigger ID", Value: stringOrDash(item.TriggerID)},
				{Label: sessionIDLabel, Value: stringOrDash(item.SessionID)},
				{Label: "Fire ID", Value: stringOrDash(item.FireID)},
				{Label: automationStatusValue, Value: stringOrDash(string(item.Status))},
				{Label: automationAttemptValue, Value: strconv.Itoa(item.Attempt)},
				{Label: "Scheduled", Value: stringOrDash(formatOptionalTime(item.ScheduledAt))},
				{Label: automationStartedValue, Value: stringOrDash(formatOptionalTime(item.StartedAt))},
				{Label: automationEndedValue, Value: stringOrDash(formatOptionalTime(item.EndedAt))},
				{Label: automationErrorValue, Value: stringOrDash(item.Error)},
				{Label: "Delivery Error", Value: stringOrDash(item.DeliveryError)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("automation_run", []string{
				"id",
				automationTargetKey,
				"job_id",
				"trigger_id",
				automationSessionIDKey,
				"fire_id",
				automationStatusKey,
				automationAttemptKey,
				"scheduled_at",
				automationStartedAtKey,
				automationEndedAtKey,
				automationErrorKey,
				"delivery_error",
			}, []string{
				item.ID,
				displayRunTarget(item),
				item.JobID,
				item.TriggerID,
				item.SessionID,
				item.FireID,
				string(item.Status),
				strconv.Itoa(item.Attempt),
				formatOptionalTime(item.ScheduledAt),
				formatOptionalTime(item.StartedAt),
				formatOptionalTime(item.EndedAt),
				item.Error,
				item.DeliveryError,
			}), nil
		},
	}
}

func automationRunListBundle(items []RunRecord) outputBundle {
	return listBundle(
		contract.RunsResponse{Runs: items},
		items,
		"Automation Runs",
		[]string{
			"ID",
			automationTargetValue,
			automationStatusValue,
			automationAttemptValue,
			automationSessionValue,
			"Scheduled",
			automationStartedValue,
			automationEndedValue,
			automationErrorValue,
			"Delivery Error",
		},
		"automation_runs",
		[]string{
			"id",
			automationTargetKey,
			automationStatusKey,
			automationAttemptKey,
			automationSessionIDKey,
			"scheduled_at",
			automationStartedAtKey,
			automationEndedAtKey,
			automationErrorKey,
			"delivery_error",
		},
		func(item RunRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(displayRunTarget(item)),
				stringOrDash(string(item.Status)),
				strconv.Itoa(item.Attempt),
				stringOrDash(item.SessionID),
				stringOrDash(formatOptionalTime(item.ScheduledAt)),
				stringOrDash(formatOptionalTime(item.StartedAt)),
				stringOrDash(formatOptionalTime(item.EndedAt)),
				stringOrDash(item.Error),
				stringOrDash(item.DeliveryError),
			}
		},
		func(item RunRecord) []string {
			return []string{
				item.ID,
				displayRunTarget(item),
				string(item.Status),
				strconv.Itoa(item.Attempt),
				item.SessionID,
				formatOptionalTime(item.ScheduledAt),
				formatOptionalTime(item.StartedAt),
				formatOptionalTime(item.EndedAt),
				item.Error,
				item.DeliveryError,
			}
		},
	)
}

func automationJobLastScheduledAt(item JobRecord) *time.Time {
	if item.Scheduler == nil {
		return nil
	}
	return item.Scheduler.LastScheduledAt
}

func automationJobLastFireID(item JobRecord) string {
	if item.Scheduler == nil {
		return ""
	}
	return item.Scheduler.LastFireID
}

func automationJobCatchUpPolicy(item JobRecord) string {
	if item.Scheduler == nil {
		return ""
	}
	return string(item.Scheduler.CatchUpPolicy)
}

func automationJobMisfireCount(item JobRecord) int {
	if item.Scheduler == nil {
		return 0
	}
	return item.Scheduler.MisfireCount
}

func automationFilterRows(filter map[string]string) [][]string {
	if len(filter) == 0 {
		return nil
	}
	keys := make([]string, 0, len(filter))
	for key := range filter {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, filter[key]})
	}
	return rows
}
