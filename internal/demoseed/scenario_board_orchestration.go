package demoseed

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

const (
	sessionPixRefundsID       = "sess_northstar_pix_refunds"
	sessionPixStatesID        = "sess_northstar_pix_refund_states"
	sessionPixReconcileID     = "sess_northstar_pix_reconcile_tests"
	sessionPixRunbookID       = "sess_northstar_pix_runbook_review"
	pixRefundsMonitorID       = "mon_northstar_pix_refunds_checks"
	pixDelegateStepStates     = 2
	pixDelegateStepReconcile  = 3
	pixDelegateStepRunbook    = 4
	pixRefundsParentTurnLabel = "turn_" + sessionPixRefundsID
)

// subagentStory is one delegation card under the PIX refunds parent session.
type subagentStory struct {
	ID         string
	ChildID    string
	Title      string
	Role       string
	Agent      string
	Provider   string
	Model      string
	StepIndex  int
	Status     string
	Result     string
	Error      string
	StartedAt  time.Time
	SettledAt  time.Time
	TaskChars  int32
	InputUsed  int64
	OutputUsed int64
}

func boardOrchestrationSessions(clock timeline) []boardSessionStory {
	parent := boardSessionStory{
		ID: sessionPixRefundsID, Name: "Ship Brazil PIX refunds",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentCheckoutEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(3, 30), EndedAt: clock.minutesAgo(12),
		StopDetail: "Pull request #485 opened after three delegated reviews.",
		Input:      6120, Output: 2380, CostUSD: 0.274,
		Steps: pixRefundsSteps(),
		PullRequest: &boardPullRequest{
			Branch: pixRefundsBranch, Number: pixRefundsPR,
			Title:       "Add PIX refunds for Brazil checkout",
			ChecksState: "pending", Passed: 9, Pending: 3,
			ReviewDecision: prReviewRequired, Mergeable: prMergeable,
			UpdatedAt: clock.minutesAgo(25), ObservedAt: clock.minutesAgo(2),
		},
	}
	children := make([]boardSessionStory, 0, 3)
	for _, sub := range pixSubagents(clock) {
		children = append(children, boardSessionStory{sessionStory: pixChildSession(sub)})
	}
	return append([]boardSessionStory{parent}, children...)
}

const (
	pixSubagentStatesID    = "sub_northstar_pix_states"
	pixSubagentReconcileID = "sub_northstar_pix_reconcile"
	pixSubagentRunbookID   = "sub_northstar_pix_runbook"
)

func pixRefundsSteps() []transcriptStep {
	return []transcriptStep{
		{Kind: stepUser, Text: "Ship PIX refunds for Brazil checkout. Split the work: someone maps the refund " +
			"states, someone writes the reconciliation tests, someone checks the rollback runbook. Open one PR."},
		{Kind: stepAgent, Text: "I'll delegate the three pieces in parallel and keep the integration here."},
		pixDelegateStep(pixSubagentStatesID, "Map PIX refund states", agentPlatformEngineer,
			"Map every PIX refund state the PSP returns to our refund states, with the transitions we accept."),
		pixDelegateStep(pixSubagentReconcileID, "Write refund reconciliation tests", agentCheckoutEngineer,
			"Write reconciliation tests that compare refunds against the settlement file for the same day."),
		pixDelegateStep(pixSubagentRunbookID, "Review the refund rollback runbook", agentDocsSteward,
			"Check that the refund rollback runbook matches the new flag and the new refund states."),
		{
			Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
			ToolInput: `{"file_path":"internal/checkout/pix/refund.go",` +
				`"old_string":"case psp.RefundProcessing:\n\t\treturn RefundPending",` +
				`"new_string":"case psp.RefundProcessing, psp.RefundInAnalysis:\n\t\treturn RefundPending\n` +
				`\tcase psp.RefundReturned:\n\t\treturn RefundFailed"}`,
			ToolResult: `{"file_path":"internal/checkout/pix/refund.go","content":"1 edit applied"}`,
		},
		{
			Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
			ToolInput: `{"command":"go test ./internal/checkout/pix/... ./internal/settlement/... -count=1"}`,
			ToolResult: `{"stdout":"ok  \tgithub.com/northstar-pay/payments-platform/internal/checkout/pix\t0.88s\n` +
				`ok  \tgithub.com/northstar-pay/payments-platform/internal/settlement\t1.37s"}`,
		},
		{
			Kind:       stepTool,
			ToolName:   toolBash,
			ToolKind:   toolKindExecute,
			ToolInput:  `{"command":"gh pr create --title \"Add PIX refunds for Brazil checkout\" --head feat/pix-refunds"}`,
			ToolResult: `{"stdout":"https://github.com/northstar-pay/payments-platform/pull/485"}`,
		},
		{Kind: stepAgent, Text: "Opened #485. The state map and reconciliation tests are in. The runbook " +
			"review found that step 4 still names the old refund flag; I fixed it in the same PR. Checks are " +
			"still running."},
	}
}

func pixDelegateStep(id, title, agent, task string) transcriptStep {
	return transcriptStep{
		Kind:     stepTool,
		ToolName: "compozy__subagent_delegate",
		ToolKind: toolKindOther,
		ToolInput: fmt.Sprintf(`{"title":%q,"target":{"agent":%q},"task":%q,"wake":"settled_only"}`,
			title, agent, task),
		// The delegate tool's result names the roster row; the transcript's subagent card reads it.
		ToolResult: fmt.Sprintf(
			`{"content":"Delegated %s to %s.","rawOutput":{"subagent_id":%q,"title":%q,"runtime_agent":%q}}`,
			title,
			agent,
			id,
			title,
			agent,
		),
	}
}

func pixSubagents(clock timeline) []subagentStory {
	return []subagentStory{
		{
			ID: pixSubagentStatesID, ChildID: sessionPixStatesID, StepIndex: pixDelegateStepStates,
			Title: "Map PIX refund states", Role: subagentRoleGeneral, Agent: agentPlatformEngineer,
			Provider: providerCodex, Model: modelCodex, Status: store.SubagentStatusCompleted,
			Result: "Seven PSP states map to four refund states. IN_ANALYSIS is new in v4 and maps to pending; " +
				"RETURNED means the receiving bank sent the money back and maps to failed.",
			StartedAt: clock.hoursMinutesAgo(3, 20), SettledAt: clock.hoursMinutesAgo(2, 55),
			TaskChars: 96, InputUsed: 1840, OutputUsed: 610,
		},
		{
			ID: pixSubagentReconcileID, ChildID: sessionPixReconcileID, StepIndex: pixDelegateStepReconcile,
			Title: "Write refund reconciliation tests", Role: subagentRoleGeneral, Agent: agentCheckoutEngineer,
			Provider: providerCodex, Model: modelCodex, Status: store.SubagentStatusCompleted,
			Result:    "Added 6 reconciliation tests against the 2026-10-28 settlement file. All pass.",
			StartedAt: clock.hoursMinutesAgo(3, 19), SettledAt: clock.hoursMinutesAgo(2, 31),
			TaskChars: 92, InputUsed: 2630, OutputUsed: 1240,
		},
		{
			ID: pixSubagentRunbookID, ChildID: sessionPixRunbookID, StepIndex: pixDelegateStepRunbook,
			Title: "Review the refund rollback runbook", Role: subagentRoleGeneral, Agent: agentDocsSteward,
			Provider: providerClaude, Model: modelClaude, Status: store.SubagentStatusCompleted,
			Result: "Step 4 still disables checkout.pix_refunds_v1; the new flag is checkout.pix_refunds. " +
				"Everything else matches.",
			StartedAt: clock.hoursMinutesAgo(3, 18), SettledAt: clock.hoursMinutesAgo(3, 2),
			TaskChars: 88, InputUsed: 1210, OutputUsed: 430,
		},
	}
}

func pixChildSession(sub subagentStory) sessionStory {
	return sessionStory{
		ID: sub.ChildID, Name: sub.Title,
		WorkspaceKey: workspaceKeyPlatform, AgentName: sub.Agent,
		Provider: sub.Provider, Model: sub.Model, SessionType: string(session.SessionTypeSpawned),
		ParentID: sessionPixRefundsID, SpawnRole: store.SubagentSpawnRole,
		StartedAt: sub.StartedAt, EndedAt: sub.SettledAt,
		StopDetail: "Delegated work finished; result returned to the parent session.",
		Input:      sub.InputUsed, Output: sub.OutputUsed,
		CostUSD: float64(sub.InputUsed+sub.OutputUsed) / 50000,
		Steps:   pixChildSteps(sub),
	}
}

func pixChildSteps(sub subagentStory) []transcriptStep {
	steps := []transcriptStep{{Kind: stepUser, Text: sub.Title + "."}}
	switch sub.ChildID {
	case sessionPixStatesID:
		steps = append(steps, transcriptStep{
			Kind:      stepTool,
			ToolName:  toolGrep,
			ToolKind:  toolKindSearch,
			ToolInput: `{"pattern":"Refund[A-Z][A-Za-z]+ +RefundStatus","path":"vendor/github.com/northstar-pay/psp-sdk-go"}`,
			ToolResult: `{"stdout":"refund.go:12 RefundCreated\nrefund.go:13 RefundProcessing\n` +
				`refund.go:14 RefundInAnalysis\nrefund.go:15 RefundCompleted\nrefund.go:16 RefundReturned\n` +
				`refund.go:17 RefundRejected\nrefund.go:18 RefundCanceled"}`,
		})
	case sessionPixReconcileID:
		steps = append(steps,
			transcriptStep{
				Kind:     stepTool,
				ToolName: toolWrite,
				ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"internal/settlement/pix_refund_reconcile_test.go","content":"package settlement\n\n` +
					`func TestReconcilePixRefunds(t *testing.T) {\n\tfile := loadSettlementFile(t, \"testdata/2026-10-28.csv\")\n` +
					`\tfor _, tc := range pixRefundCases {\n\t\tt.Run(tc.name, func(t *testing.T) { assertReconciled(t, file, tc) })\n\t}\n}\n"}`,
				ToolResult: `{"file_path":"internal/settlement/pix_refund_reconcile_test.go","content":"File written"}`,
			},
			transcriptStep{
				Kind:       stepTool,
				ToolName:   toolBash,
				ToolKind:   toolKindExecute,
				ToolInput:  `{"command":"go test ./internal/settlement/... -run TestReconcilePixRefunds -v -count=1"}`,
				ToolResult: `{"stdout":"=== RUN   TestReconcilePixRefunds\n--- PASS: TestReconcilePixRefunds (0.04s)\nPASS"}`,
			},
		)
	case sessionPixRunbookID:
		steps = append(steps, transcriptStep{
			Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
			ToolInput: `{"file_path":"docs/runbooks/pix-refund-rollback.md"}`,
			ToolResult: `{"content":"## Roll back PIX refunds\n1. Page the checkout on-call.\n` +
				`2. Pause the refund queue.\n3. Drain in-flight refunds.\n4. Disable checkout.pix_refunds_v1.\n"}`,
		})
	}
	return append(steps, transcriptStep{Kind: stepAgent, Text: sub.Result})
}

func pixSubagentToolCallID(sub subagentStory) string {
	return fmt.Sprintf("call_%s_%02d", sessionPixRefundsID, sub.StepIndex+1)
}
