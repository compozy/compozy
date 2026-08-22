package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/task"
)

const initiativeCheckoutLaunch = "checkout-launch"

func scenarioTasks(clock timeline) []taskStory {
	return append(launchTasks(clock), platformTasks(clock)...)
}

func launchTasks(clock timeline) []taskStory {
	return append(launchClosedTasks(clock), launchOpenTasks(clock)...)
}

func launchClosedTasks(clock timeline) []taskStory {
	return []taskStory{
		completedLaunchTask(completedTaskInput{
			id: "task_northstar_compliance_copy", identifier: "LAUNCH-102",
			title:       "Approve Brazil fallback language",
			description: "Confirm the timeout message is accurate before the canary.",
			priority:    taskPriorityHigh, sessionID: sessionComplianceCopyID,
			runID: "run_northstar_compliance_copy", closedAt: clock.hoursMinutesAgo(13, 26),
			createdAt: clock.hoursAgo(19), tokens: 1648,
			result: `{"market":"BR","copy_status":"approved","charge_claim":"no_charge_created"}`,
		}),
		completedLaunchTask(completedTaskInput{
			id: taskSupportID, identifier: "LAUNCH-103",
			title:       "Complete Brazil support handoff",
			description: "Publish the Portuguese macro and confirm launch escalation coverage.",
			priority:    taskPriorityHigh, sessionID: sessionSupportHandoffID,
			runID: "run_northstar_support_handoff", closedAt: clock.hoursMinutesAgo(6, 1),
			createdAt: clock.hoursAgo(12), tokens: 1987,
			result: `{"market":"BR","open_conversations":4,"macro":"published","coverage_minutes":90}`,
		}),
		launchDecisionTask(clock),
		completedLaunchTask(completedTaskInput{
			id: "task_northstar_chargeback_sweep", identifier: "LAUNCH-108",
			title:       "Sweep 30-day chargeback signals",
			description: "Confirm the canary does not raise the Brazil market risk level.",
			priority:    taskPriorityMedium, sessionID: sessionFraudSweepID,
			runID: "run_northstar_chargeback_sweep", closedAt: clock.daysHoursAgo(2, 2),
			createdAt: clock.daysHoursAgo(2, 5), tokens: 2644,
			result: `{"market":"BR","dispute_rate_per_1k":0.42,"threshold":0.60,"risk_level":"unchanged"}`,
		}),
		completedLaunchTask(completedTaskInput{
			id: "task_northstar_incident_postmortem", identifier: "LAUNCH-109",
			title:       "Publish the authorization dip postmortem",
			description: "Name an owner for every follow-up from the 2026-08-14 dip.",
			priority:    taskPriorityHigh, sessionID: sessionIncidentReviewID,
			runID: "run_northstar_incident_postmortem", closedAt: clock.daysHoursAgo(4, 2),
			createdAt: clock.daysHoursAgo(4, 6), tokens: 8260,
			result: `{"minutes_over_threshold":12,"causes":2,"follow_ups_with_owner":2}`,
		}),
	}
}

func launchDecisionTask(clock timeline) taskStory {
	story := completedLaunchTask(completedTaskInput{
		id: taskLaunchDecisionID, identifier: "LAUNCH-104",
		title:       "Produce checkout launch recommendation",
		description: "Synthesize replay, compliance, support, and rollout evidence.",
		priority:    taskPriorityUrgent, sessionID: sessionLaunchDecisionID,
		runID: "run_northstar_launch_decision", closedAt: clock.minutesAgo(41),
		createdAt: clock.hoursAgo(5), tokens: 3065,
		result: `{"br":"go_25_percent_after_approval","mx":"hold","rollback_auth_error_percent":1.0}`,
	})
	story.DependencyTaskIDs = []string{
		"task_northstar_compliance_copy",
		taskSupportID,
	}
	return story
}

func launchOpenTasks(clock timeline) []taskStory {
	return []taskStory{
		{
			ID: "task_northstar_authorize_canary", WorkspaceKey: workspaceKeyLaunch, Identifier: "LAUNCH-105",
			Title: "Authorize Brazil 25% canary", Description: "Operator approval required before traffic promotion.",
			Priority: taskPriorityUrgent, Status: taskStatusBlocked,
			ApprovalPolicy: approvalPolicyManual, ApprovalState: approvalPending,
			OwnerKind: ownerHuman, OwnerRef: operatorRef, Initiative: initiativeCheckoutLaunch,
			CreatedAt: clock.minutesAgo(39), UpdatedAt: clock.minutesAgo(39),
			DependencyTaskIDs: []string{taskLaunchDecisionID},
		},
		{
			ID: "task_northstar_mexico_replay", WorkspaceKey: workspaceKeyLaunch, Identifier: "LAUNCH-106",
			Title:       "Close Mexico partner replay gap",
			Description: "Reconcile the missing MercadoX export before Mexico promotion.",
			Priority:    taskPriorityHigh, Status: taskStatusBlocked,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerPool, OwnerRef: poolPlatform, Initiative: initiativeCheckoutLaunch,
			CreatedAt: clock.hoursAgo(20), UpdatedAt: clock.minutesAgo(35),
			BlockReason: "MercadoX export is missing the 14:08–14:30 UTC replay window.",
			BlockDetails: `{"market":"MX","missing_window":"14:08-14:30 UTC","owner":"MercadoX",` +
				`"missing_minutes":22}`,
		},
		{
			ID: "task_northstar_rollback_monitor", WorkspaceKey: workspaceKeyLaunch, Identifier: "LAUNCH-107",
			Title:       "Pre-stage Brazil rollback monitor",
			Description: "Alert when authorization errors exceed 1.0% for five minutes.",
			Priority:    taskPriorityHigh, Status: taskStatusReady,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerPool, OwnerRef: poolCheckout, Initiative: initiativeCheckoutLaunch,
			CreatedAt: clock.minutesAgo(32), UpdatedAt: clock.minutesAgo(32),
			DependencyTaskIDs: []string{taskLaunchDecisionID},
		},
		{
			ID: "task_northstar_launch_comms", WorkspaceKey: workspaceKeyLaunch, Identifier: "LAUNCH-110",
			Title:       "Draft the Brazil launch note",
			Description: "Customer-facing note, held until the canary reaches 100%.",
			Priority:    taskPriorityLow, Status: taskStatusDraft,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerHuman, OwnerRef: operatorRef, Initiative: initiativeCheckoutLaunch,
			CreatedAt: clock.hoursAgo(3), UpdatedAt: clock.hoursAgo(3),
		},
	}
}

type completedTaskInput struct {
	id          string
	identifier  string
	title       string
	description string
	priority    string
	sessionID   string
	runID       string
	createdAt   time.Time
	closedAt    time.Time
	tokens      int64
	result      string
}

func completedLaunchTask(input completedTaskInput) taskStory {
	return completedTaskStory(workspaceKeyLaunch, input)
}

func completedTaskStory(workspaceKey string, input completedTaskInput) taskStory {
	return taskStory{
		ID: input.id, WorkspaceKey: workspaceKey, Identifier: input.identifier,
		Title: input.title, Description: input.description,
		Priority: input.priority, Status: taskStatusCompleted,
		ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
		OwnerKind: ownerAgentSession, OwnerRef: input.sessionID, SessionID: input.sessionID,
		Initiative: initiativeCheckoutLaunch,
		CreatedAt:  input.createdAt, UpdatedAt: input.closedAt, ClosedAt: input.closedAt,
		TokensUsed: input.tokens, Result: input.result,
		History: []taskRunStory{{
			ID: input.runID, Status: task.TaskRunStatusCompleted, SessionID: input.sessionID,
			StartedAt: input.closedAt.Add(-8 * time.Minute), EndedAt: input.closedAt,
			TokensUsed: input.tokens, Result: input.result,
		}},
	}
}
