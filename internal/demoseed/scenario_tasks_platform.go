package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/task"
)

const initiativePaymentsPlatform = "payments-platform"

func platformTasks(clock timeline) []taskStory {
	tasks := platformClosedTasks(clock)
	tasks = append(tasks, platformFailedTasks(clock)...)
	return append(tasks, platformOpenTasks(clock)...)
}

func platformClosedTasks(clock timeline) []taskStory {
	return []taskStory{
		completedPlatformTask(completedTaskInput{
			id: "task_northstar_partner_replay", identifier: "PLAT-200",
			title:       "Reconcile Brazil partner replay",
			description: "Verify MercadoX replay coverage before rollout.",
			priority:    taskPriorityUrgent, sessionID: sessionPartnerReplayID,
			runID: "run_northstar_partner_replay", closedAt: clock.hoursMinutesAgo(21, 7),
			createdAt: clock.hoursAgo(25), tokens: 2452,
			result: `{"market":"BR","replayed_events":10214,"duplicate_charges":0,"decision":"clear_for_canary"}`,
		}),
		completedPlatformTask(completedTaskInput{
			id: "task_northstar_rollback_drill", identifier: "PLAT-203",
			title:       "Rehearse the Brazil rollback path",
			description: "Prove the rollback finishes inside the five-minute threshold window.",
			priority:    taskPriorityHigh, sessionID: sessionRollbackDrillID,
			runID: "run_northstar_rollback_drill", closedAt: clock.daysHoursAgo(1, 5),
			createdAt: clock.daysHoursAgo(1, 8), tokens: 3040,
			result: `{"env":"staging","rollback_seconds":48,"alert_window_minutes":5}`,
		}),
		completedPlatformTask(completedTaskInput{
			id: taskPlatformReleaseID, identifier: "PLAT-204",
			title:       "Close the payments release train",
			description: "Confirm a rollback path and named owner for every change in the window.",
			priority:    taskPriorityMedium, sessionID: sessionReleaseTrainID,
			runID: "run_northstar_release_train", closedAt: clock.daysHoursAgo(3, 6),
			createdAt: clock.daysHoursAgo(3, 9), tokens: 2330,
			result: `{"changes":4,"with_rollback":4,"with_owner":4}`,
		}),
		completedPlatformTask(completedTaskInput{
			id: taskPlatformDocsRefresh, identifier: "PLAT-205",
			title:       "Run the runbook freshness pass",
			description: "Flag payments runbooks whose steps no longer match the shipped controls.",
			priority:    taskPriorityLow, sessionID: sessionDocsRefreshID,
			runID: "run_northstar_docs_refresh", closedAt: clock.daysHoursAgo(6, 3),
			createdAt: clock.daysHoursAgo(6, 5), tokens: 1925,
			result: `{"runbooks_checked":11,"stale":2,"rewritten":0}`,
		}),
		completedPlatformTask(completedTaskInput{
			id: "task_northstar_checkpoint_backfill", identifier: "PLAT-210",
			title:       "Backfill settlement batch checkpoints",
			description: "Replay historical batches into the new checkpoint format.",
			priority:    taskPriorityMedium, sessionID: "",
			runID: "run_northstar_checkpoint_backfill", closedAt: clock.daysHoursAgo(8, 4),
			createdAt: clock.daysHoursAgo(8, 9), tokens: 4180,
			result: `{"batches":1204,"checkpointed":1204,"skipped":0}`,
		}),
		completedPlatformTask(completedTaskInput{
			id: "task_northstar_latency_audit", identifier: "PLAT-211",
			title:       "Audit connector latency percentiles",
			description: "Establish the p99 baseline the pool sizing forecast depends on.",
			priority:    taskPriorityMedium, sessionID: "",
			runID: "run_northstar_latency_audit", closedAt: clock.daysHoursAgo(11, 6),
			createdAt: clock.daysHoursAgo(11, 10), tokens: 3610,
			result: `{"p50_ms":180,"p99_ms":870,"sample_hours":168}`,
		}),
		completedPlatformTask(completedTaskInput{
			id: "task_northstar_cert_rotation", identifier: "PLAT-212",
			title:       "Rotate the MercadoX partner certificate",
			description: "Rotate before the 2026-09 expiry and verify the handshake.",
			priority:    taskPriorityLow, sessionID: "",
			runID: "run_northstar_cert_rotation", closedAt: clock.daysHoursAgo(13, 7),
			createdAt: clock.daysHoursAgo(13, 9), tokens: 1180,
			result: `{"rotated":true,"handshake_verified":true,"expires_in_days":365}`,
		}),
	}
}

func platformFailedTasks(clock timeline) []taskStory {
	return []taskStory{
		{
			ID: taskPlatformQuarantine, WorkspaceKey: workspaceKeyPlatform, Identifier: "PLAT-202",
			Title:       "Add the settlement rate limiter",
			Description: "Keep MercadoX submits under 40 requests per second without reordering batches.",
			Priority:    taskPriorityUrgent, Status: taskStatusFailed,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerAgentSession, OwnerRef: sessionSettlementWorkerID,
			SessionID: sessionSettlementWorkerID, Initiative: initiativePaymentsPlatform,
			CreatedAt: clock.daysHoursAgo(7, 6), UpdatedAt: clock.hoursMinutesAgo(3, 12),
			ClosedAt: clock.hoursMinutesAgo(3, 12), TokensUsed: 3550,
			Result: `{"blocker":"batch_ordering_regression","contract_test":"TestContractBatchOrdering"}`,
			History: []taskRunStory{
				{
					ID: "run_northstar_settlement_retry_1", Status: task.TaskRunStatusFailed,
					Attempt:   1,
					StartedAt: clock.daysHoursAgo(7, 6), EndedAt: clock.daysHoursAgo(7, 5),
					TokensUsed: 910, Error: "connector returned 429 during the limiter smoke test",
				},
				{
					ID: "run_northstar_settlement_retry_2", Status: task.TaskRunStatusFailed,
					Attempt:   2,
					SessionID: sessionSettlementWorkerID,
					StartedAt: clock.hoursMinutesAgo(3, 50), EndedAt: clock.hoursMinutesAgo(3, 12),
					TokensUsed: 2640,
					Error:      "batch 2 settled before batch 1: limiter reorders concurrent submits",
				},
			},
		},
		{
			ID: "task_northstar_idempotency_sweep", WorkspaceKey: workspaceKeyPlatform, Identifier: "PLAT-213",
			Title:       "Sweep duplicate idempotency keys",
			Description: "Find settlement submits reusing a key across batches.",
			Priority:    taskPriorityMedium, Status: taskStatusFailed,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerPool, OwnerRef: poolPlatform, Initiative: initiativePaymentsPlatform,
			CreatedAt: clock.daysHoursAgo(5, 8), UpdatedAt: clock.daysHoursAgo(5, 6),
			ClosedAt: clock.daysHoursAgo(5, 6), TokensUsed: 1240,
			Result: `{"blocker":"partner_export_incomplete"}`,
			History: []taskRunStory{{
				ID: "run_northstar_idempotency_sweep", Status: task.TaskRunStatusFailed,
				StartedAt: clock.daysHoursAgo(5, 8), EndedAt: clock.daysHoursAgo(5, 6),
				TokensUsed: 1240, Error: "partner export incomplete for the sweep window",
			}},
		},
		{
			ID: "task_northstar_webhook_retirement", WorkspaceKey: workspaceKeyPlatform, Identifier: "PLAT-208",
			Title:       "Retire the legacy settlement webhook",
			Description: "Superseded by the checkpoint stream; withdrawn before the release train.",
			Priority:    taskPriorityLow, Status: taskStatusCanceled,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerAutomation, OwnerRef: digestAutomationID,
			Initiative: initiativePaymentsPlatform,
			CreatedAt:  clock.daysHoursAgo(9, 9), UpdatedAt: clock.daysHoursAgo(9, 7),
			ClosedAt: clock.daysHoursAgo(9, 7), TokensUsed: 420,
			Result: `{"reason":"superseded_by_checkpoint_stream"}`,
			History: []taskRunStory{{
				ID: "run_northstar_webhook_retirement", Status: task.TaskRunStatusCanceled,
				StartedAt: clock.daysHoursAgo(9, 9), EndedAt: clock.daysHoursAgo(9, 7),
				TokensUsed: 420,
			}},
		},
	}
}

func platformOpenTasks(clock timeline) []taskStory {
	return []taskStory{
		{
			ID: "task_northstar_settlement_investigation", WorkspaceKey: workspaceKeyPlatform,
			Identifier:  "PLAT-201",
			Title:       "Investigate stacking settlement retries",
			Description: "Isolate why three workers exhaust retries against the same endpoint.",
			Priority:    taskPriorityUrgent, Status: taskStatusInProgress,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerAgentSession, OwnerRef: sessionSettlementRetryID,
			SessionID: sessionSettlementRetryID, Initiative: initiativePaymentsPlatform,
			CreatedAt: clock.hoursMinutesAgo(4, 40), UpdatedAt: clock.hoursMinutesAgo(3, 40),
		},
		{
			ID: "task_northstar_pool_sizing", WorkspaceKey: workspaceKeyPlatform, Identifier: "PLAT-206",
			Title:       "Resize the connector pool from the forecast",
			Description: "Follow-up owned by platform-engineer from the 2026-08-14 postmortem.",
			Priority:    taskPriorityHigh, Status: taskStatusNeedsAttn,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerPool, OwnerRef: poolPlatform, Initiative: initiativePaymentsPlatform,
			CreatedAt: clock.daysHoursAgo(4, 1), UpdatedAt: clock.hoursAgo(2),
		},
		{
			ID: "task_northstar_alert_wiring", WorkspaceKey: workspaceKeyPlatform, Identifier: "PLAT-207",
			Title:       "Wire the 1.0% over five minutes alert",
			Description: "Follow-up owned by checkout-engineer from the 2026-08-14 postmortem.",
			Priority:    taskPriorityHigh, Status: taskStatusReady,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerPool, OwnerRef: poolCheckout, Initiative: initiativePaymentsPlatform,
			CreatedAt: clock.daysHoursAgo(4, 1), UpdatedAt: clock.daysHoursAgo(1, 4),
		},
		{
			ID: "task_northstar_dispute_export", WorkspaceKey: workspaceKeyPlatform, Identifier: "PLAT-209",
			Title:       "Export dispute evidence bundles",
			Description: "Package evidence for the open Brazil disputes ahead of the partner review.",
			Priority:    taskPriorityMedium, Status: taskStatusPending,
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: ownerPool, OwnerRef: "risk", Initiative: initiativePaymentsPlatform,
			CreatedAt: clock.hoursAgo(30), UpdatedAt: clock.hoursAgo(30),
		},
	}
}

func completedPlatformTask(input completedTaskInput) taskStory {
	story := completedTaskStory(workspaceKeyPlatform, input)
	story.Initiative = initiativePaymentsPlatform
	if input.sessionID == "" {
		story.OwnerKind = ownerAutomation
		story.OwnerRef = digestAutomationID
	}
	story.History[0].StartedAt = input.closedAt.Add(-14 * time.Minute)
	return story
}
