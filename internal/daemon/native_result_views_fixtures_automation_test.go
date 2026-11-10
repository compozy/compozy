package daemon

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesAutomation(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	job := contract.JobPayload{
		ID:          "job-view",
		ProfileID:   "prof-view",
		ProfileName: "Default",
		Name:        "Daily review",
		Scope:       "workspace",
		TargetKind:  "loop",
		AgentName:   "reviewer",
		WorkspaceID: "ws-view",
		Enabled:     true,
		Source:      "dynamic",
		Prompt: strings.Repeat(
			"Review pending changes. ",
			80,
		),
		ProfileColor: "blue",
		ProfileIcon:  "clock",
		CreatedAt:    now,
		UpdatedAt:    now,
		NextRun:      &now,
		Schedule: &automationpkg.ScheduleSpec{
			Mode:                "cron",
			Expr:                "0 9 * * *",
			CatchUpPolicy:       "coalesce",
			MisfireGraceSeconds: 60,
		},
		LoopTarget: &automationpkg.LoopTarget{
			WorkspaceID:  "ws-view",
			LoopName:     "review",
			Inputs:       map[string]any{"instructions": strings.Repeat("Review. ", 60)},
			InputMapping: map[string]string{"event": "event.id"},
		},
		Task: &automationpkg.JobTaskConfig{
			Title:       "Review changes",
			Description: strings.Repeat("Review requirements. ", 40),
		},
		Scheduler: &contract.AutomationSchedulerStatePayload{
			JobID:                     "job-view",
			Registered:                false,
			ConsecutiveResumeFailures: 3,
			MisfireCount:              2,
			LastMisfireAt:             &now,
			NextRunAt:                 &now,
			LastFireID:                "fire-view",
		},
		LastRun: &contract.AutomationLastRunPayload{
			ID:         "run-view",
			Status:     automationpkg.RunCancelled,
			SkipReason: automationpkg.SchedulerSkipReasonGraceExceeded,
			StartedAt:  &now,
			EndedAt:    &now,
		},
	}
	trigger := contract.TriggerPayload{
		ID:                  "trigger-view",
		ProfileID:           "prof-view",
		ProfileName:         "Default",
		Name:                "Review event",
		Scope:               "workspace",
		TargetKind:          "agent_instance",
		TargetInstanceScope: "workspace",
		AgentName:           "reviewer",
		WorkspaceID:         "ws-view",
		Event:               "hook.received",
		Enabled:             true,
		Source:              "dynamic",
		Prompt: strings.Repeat(
			"Review this event. ",
			80,
		),
		Filter:               map[string]string{"kind": "review"},
		LoopTarget:           job.LoopTarget,
		LastRun:              job.LastRun,
		WebhookID:            "webhook-view",
		EndpointSlug:         "review",
		WebhookSecretPresent: true,
		WebhookSecretHash:    "sha256:ref",
		Ingress: &contract.GatewayIngressPayload{
			SubjectKind:        "trigger",
			SubjectID:          "trigger-view",
			ScopeKind:          "workspace",
			WorkspaceID:        "ws-view",
			Reachability:       "unconfirmed",
			EndpointGeneration: 2,
			EnablePath:         "/gateway/enable",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	var rows []nativeResultViewFixture
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAutomationJobsCreate, toolspkg.ToolIDAutomationJobsDisable, toolspkg.ToolIDAutomationJobsEnable, toolspkg.ToolIDAutomationJobsGet, toolspkg.ToolIDAutomationJobsUpdate} {
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: id,
				Name:   "scheduler failure",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) { return jobRecordViews.result(view, job) },
				MustKeep: []string{
					"job.id",
					"job.enabled",
					"job.source",
					"job.loop_target.loop_name",
					"job.last_run.id",
					"job.last_run.status",
					"job.last_run.skip_reason",
					"job.scheduler.registered",
					"job.scheduler.consecutive_resume_failures",
					"job.scheduler.misfire_count",
					"job.scheduler.last_misfire_at",
				},
			},
			nativeResultViewFixture{
				ToolID: id,
				Name:   "optional fields absent",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return jobRecordViews.result(view, contract.JobPayload{ID: "job-new"})
				},
				MustKeep: []string{"job.id", "job.enabled"},
			},
		)
	}
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAutomationTriggersCreate, toolspkg.ToolIDAutomationTriggersDisable, toolspkg.ToolIDAutomationTriggersEnable, toolspkg.ToolIDAutomationTriggersGet, toolspkg.ToolIDAutomationTriggersUpdate} {
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: id,
				Name:   "skipped run and ingress state",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return triggerRecordViews.result(view, trigger)
				},
				MustKeep: []string{
					"trigger.id",
					"trigger.target_instance_scope",
					"trigger.webhook_secret_present",
					"trigger.last_run.status",
					"trigger.last_run.skip_reason",
					"trigger.ingress.reachability",
					"trigger.ingress.enable_path",
				},
			},
			nativeResultViewFixture{
				ToolID: id,
				Name:   "optional fields absent",
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return triggerRecordViews.result(view, contract.TriggerPayload{ID: "trigger-new"})
				},
				MustKeep: []string{"trigger.id", "trigger.enabled"},
			},
		)
	}
	page := contract.CountedCursorPagePayload{HasMore: true, NextCursor: "cursor-view", Total: 2, Limit: 1}
	rows = append(
		rows,
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationJobsList,
			Name:   "scheduler failure and cursor",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return jobListViews.result(view, contract.JobsResponse{Jobs: []contract.JobPayload{job}, Page: page})
			},
			MustKeep: []string{
				"jobs.0.last_run.skip_reason",
				"jobs.0.scheduler.consecutive_resume_failures",
				"jobs.0.scheduler.misfire_count",
				"page.next_cursor",
				"page.has_more",
				"page.limit",
				"page.total",
			},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationJobsList,
			Name:   "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return jobListViews.result(
					view,
					contract.JobsResponse{
						Jobs: []contract.JobPayload{},
						Page: contract.CountedCursorPagePayload{Limit: 50},
					},
				)
			},
			MustKeep: []string{"jobs", "page.has_more", "page.limit", "page.total"},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationTriggersList,
			Name:   "skipped run and cursor",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return triggerListViews.result(
					view,
					contract.TriggersResponse{Triggers: []contract.TriggerPayload{trigger}, Page: page},
				)
			},
			MustKeep: []string{
				"triggers.0.last_run.skip_reason",
				"triggers.0.ingress.reachability",
				"page.next_cursor",
				"page.has_more",
				"page.limit",
				"page.total",
			},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationTriggersList,
			Name:   "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return triggerListViews.result(
					view,
					contract.TriggersResponse{
						Triggers: []contract.TriggerPayload{},
						Page:     contract.CountedCursorPagePayload{Limit: 50},
					},
				)
			},
			MustKeep: []string{"triggers", "page.has_more", "page.limit", "page.total"},
		},
	)
	suggestion := contract.AutomationSuggestionPayload{
		ID:          "suggestion-view",
		ProfileID:   "prof-view",
		WorkspaceID: "ws-view",
		Source:      "catalog",
		Status:      "pending",
		DedupKey:    "daily-review",
		Payload:     job,
		CreatedAt:   now,
	}
	rows = append(
		rows,
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationSuggestionsList,
			Name:   "pending proposal",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return automationSuggestionListViews.result(
					view,
					contract.AutomationSuggestionsResponse{
						Suggestions: []contract.AutomationSuggestionPayload{suggestion},
					},
				)
			},
			MustKeep: []string{
				"suggestions.0.id",
				"suggestions.0.workspace_id",
				"suggestions.0.status",
				"suggestions.0.payload.last_run.skip_reason",
			},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationSuggestionsList,
			Name:   "empty list",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				return automationSuggestionListViews.result(
					view,
					contract.AutomationSuggestionsResponse{Suggestions: []contract.AutomationSuggestionPayload{}},
				)
			},
			MustKeep: []string{"suggestions"},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationSuggestionsAccept,
			Name:   "accepted proposal",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				accepted := suggestion
				accepted.Status = "accepted"
				accepted.ResolvedAt = &now
				return automationSuggestionAcceptViews.result(
					view,
					contract.AutomationSuggestionAcceptanceResponse{Suggestion: accepted, Job: job},
				)
			},
			MustKeep: []string{
				"suggestion.id",
				"suggestion.status",
				"suggestion.resolved_at",
				"job.id",
				"job.last_run.skip_reason",
			},
		},
		nativeResultViewFixture{
			ToolID: toolspkg.ToolIDAutomationSuggestionsDismiss,
			Name:   "dismissed proposal",
			Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
				dismissed := suggestion
				dismissed.Status = "dismissed"
				dismissed.ResolvedAt = &now
				return automationSuggestionViews.result(
					view,
					contract.AutomationSuggestionResponse{Suggestion: dismissed},
				)
			},
			MustKeep: []string{
				"suggestion.id",
				"suggestion.status",
				"suggestion.resolved_at",
				"suggestion.payload.last_run.skip_reason",
			},
		},
	)
	return rows
}
