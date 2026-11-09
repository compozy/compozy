package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// Sessions that give every durable board column real work: what failed, what came back
// from Later, what waits in review, and what is parked with a reason and a date.
const (
	sessionChargebackBackfillID = "sess_northstar_chargeback_backfill"
	sessionDisputePlaybookID    = "sess_northstar_dispute_playbook"
	sessionPSPUpgradeID         = "sess_northstar_psp_sdk_upgrade"
	sessionRetryBudgetID        = "sess_northstar_settlement_retry_budget"
	// #nosec G101 -- a session id, not a credential.
	sessionWebhookSecretsID = "sess_northstar_webhook_secret_rotation"
	sessionLedgerFlakeID    = "sess_northstar_ledger_flake"
	sessionPagerSummaryID   = "sess_northstar_pager_summary"
)

const (
	prMergeable          = "mergeable"
	prReviewRequired     = "review_required"
	subagentRoleGeneral  = "general"
	northstarRepoKey     = "github.com/northstar-pay/payments-platform"
	northstarRepoURL     = "https://github.com/northstar-pay/payments-platform"
	retryBudgetBranch    = "fix/settlement-retry-budget"
	retryBudgetPR        = 482
	webhookSecretsBranch = "chore/rotate-psp-webhook-secrets" // #nosec G101 -- a branch name, not a credential.
	webhookSecretsPR     = 479
	pixRefundsBranch     = "feat/pix-refunds"
	pixRefundsPR         = 485
)

// boardLater parks a stopped session in Later. A ReturnAt already in the past is swept
// back by the same return path the daemon runs, so the session lands in Needs you.
type boardLater struct {
	Reason   string
	ParkedAt time.Time
	ReturnAt time.Time
}

// boardPullRequest is the forge observation the daemon's pull request cache would hold.
type boardPullRequest struct {
	Branch         string
	Number         int
	Title          string
	ChecksState    string
	Passed         int
	Failed         int
	Pending        int
	FailedNames    []string
	ReviewDecision string
	Mergeable      string
	UpdatedAt      time.Time
	ObservedAt     time.Time
}

type boardSessionStory struct {
	sessionStory
	Later       *boardLater
	PullRequest *boardPullRequest
	// BindWorktree binds the session to the seeded settlement worktree.
	BindWorktree bool
}

func boardSessions(clock timeline) []boardSessionStory {
	return []boardSessionStory{
		chargebackBackfillSession(clock),
		disputePlaybookSession(clock),
		pspUpgradeSession(clock),
		retryBudgetSession(clock),
		webhookSecretsSession(clock),
		ledgerFlakeSession(clock),
		pagerSummarySession(clock),
	}
}

func chargebackBackfillSession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionChargebackBackfillID, Name: "Backfill October chargeback reason codes",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentFraudAnalyst,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.minutesAgo(48), EndedAt: clock.minutesAgo(31),
		StopReason: string(store.StopError),
		StopDetail: "The provider process exited during the reason-code backfill.",
		Failure: &sessionFailureStory{
			Kind:    string(store.FailureProcess),
			Summary: "Provider process exited while mapping batch 3 of 5 of the October chargebacks.",
		},
		Input: 3120, Output: 640, CostUSD: 0.131,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Backfill the October chargebacks with the new reason-code mapping so the " +
				"dispute dashboard stops showing \"unmapped\" for Brazil."},
			{Kind: stepAgent, Text: "I'll read the mapping, dry-run it on one batch, then apply it batch by batch."},
			{
				Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
				ToolInput: `{"file_path":"risk/chargebacks/reason_codes.yaml"}`,
				ToolResult: `{"content":"visa:\n  \"10.4\": fraud_card_absent\n  \"13.1\": not_received\n` +
					`mastercard:\n  \"4837\": fraud_no_authorization\n  \"4853\": not_as_described\n` +
					`elo:\n  \"28\": fraud_card_absent\n"}`,
			},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"./bin/chargeback-backfill --month 2026-10 --market br --batch 1 --dry-run"}`,
				ToolResult: `{"stdout":"batch 1/5: 1,204 chargebacks, 1,198 mapped, 6 left unmapped (elo 31)\n` +
					`dry run: no rows written"}`,
			},
			{Kind: stepThinking, Text: "Six Elo chargebacks use code 31, which the mapping doesn't cover. That's " +
				"a mapping gap, not a reason to stop the backfill; I'll flag it and continue."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"./bin/chargeback-backfill --month 2026-10 --market br --batch 1-3"}`,
				ToolResult: `{"stdout":"batch 1/5: wrote 1,198 rows\nbatch 2/5: wrote 1,311 rows\n` +
					`batch 3/5: mapping...","stderr":"signal: killed"}`,
			},
		}}
}

func disputePlaybookSession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionDisputePlaybookID, Name: "Draft the Q4 dispute evidence playbook",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentComplianceReview,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(3, 5), EndedAt: clock.daysHoursAgo(3, 4),
		StopDetail: "First draft ready; waiting on the acquirer's evidence template.",
		Input:      2480, Output: 1730, CostUSD: 0.142,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Draft the Q4 dispute evidence playbook for Brazil: what evidence we send " +
				"per reason code, who pulls it, and the deadline per acquirer."},
			{
				Kind:       stepTool,
				ToolName:   toolGrep,
				ToolKind:   toolKindSearch,
				ToolInput:  `{"pattern":"evidence_deadline_days","path":"risk/acquirers"}`,
				ToolResult: `{"stdout":"risk/acquirers/cielo.yaml:evidence_deadline_days: 7\nrisk/acquirers/stone.yaml:evidence_deadline_days: 10"}`,
			},
			{
				Kind:     stepTool,
				ToolName: toolWrite,
				ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"docs/risk/q4-dispute-playbook.md","content":"# Q4 dispute evidence playbook (Brazil)\n\n` +
					`| Reason | Evidence | Owner | Deadline |\n|---|---|---|---|\n` +
					`| fraud_card_absent | 3DS result, device fingerprint, delivery proof | Risk ops | Cielo 7d, Stone 10d |\n` +
					`| not_received | Carrier tracking, signed delivery | Support | Cielo 7d, Stone 10d |\n` +
					`| not_as_described | Order page snapshot, customer thread | Support | Cielo 7d, Stone 10d |\n"}`,
				ToolResult: `{"file_path":"docs/risk/q4-dispute-playbook.md","content":"File written"}`,
			},
			{Kind: stepAgent, Text: "The draft covers the three reason codes that make up most Brazil disputes. " +
				"Stone wants evidence in its own template, which they haven't sent yet; I left that row marked " +
				"as pending rather than guessing the format."},
		},
		Later: &boardLater{
			Reason:   "Pick this up once Stone sends their evidence template.",
			ParkedAt: clock.daysHoursAgo(3, 4).Add(5 * time.Minute),
			ReturnAt: clock.hoursAgo(2),
		},
	}
}

func pspUpgradeSession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionPSPUpgradeID, Name: "Upgrade settlement workers to PSP SDK v4",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentPlatformEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(1, 3), EndedAt: clock.daysHoursAgo(1, 2),
		StopDetail: "Upgrade plan written; held for the change freeze.",
		Input:      1960, Output: 880, CostUSD: 0.092,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Plan the PSP SDK v4 upgrade for the settlement workers. Don't merge " +
				"anything during the freeze."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput:  `{"command":"go list -m -u github.com/northstar-pay/psp-sdk-go"}`,
				ToolResult: `{"stdout":"github.com/northstar-pay/psp-sdk-go v3.9.2 [v4.0.1]"}`,
			},
			{
				Kind:       stepTool,
				ToolName:   toolGrep,
				ToolKind:   toolKindSearch,
				ToolInput:  `{"pattern":"psp\\.NewClient\\(","path":"internal/settlement"}`,
				ToolResult: `{"stdout":"internal/settlement/mercadox_client.go:41\ninternal/settlement/stone_client.go:37"}`,
			},
			{Kind: stepAgent, Text: "Two call sites construct the client. v4 drops the global retry option, " +
				"so both need the retry budget passed per call. The change is small, but it touches settlement, " +
				"so it should wait until the freeze ends."},
		},
		Later: &boardLater{
			Reason:   "Payments change freeze until Monday. Upgrade right after it lifts.",
			ParkedAt: clock.daysHoursAgo(1, 2).Add(3 * time.Minute),
			ReturnAt: clock.Now().Add(2*24*time.Hour + 3*time.Hour),
		},
	}
}

func retryBudgetSession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionRetryBudgetID, Name: "Cap settlement retries per partner window",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentCheckoutEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(2, 40), EndedAt: clock.hoursMinutesAgo(1, 55),
		StopDetail: "Pull request #482 opened; checks green.",
		Input:      4210, Output: 1540, CostUSD: 0.188,
		Steps:        retryBudgetSteps(),
		BindWorktree: true,
		PullRequest: &boardPullRequest{
			Branch: retryBudgetBranch, Number: retryBudgetPR,
			Title:       "Cap settlement retries per partner window",
			ChecksState: "passing", Passed: 14,
			ReviewDecision: prReviewRequired, Mergeable: prMergeable,
			UpdatedAt: clock.hoursMinutesAgo(1, 50), ObservedAt: clock.minutesAgo(3),
		},
	}
}

func retryBudgetSteps() []transcriptStep {
	return []transcriptStep{
		{Kind: stepUser, Text: "Settlement workers burn their whole retry budget against MercadoX when it " +
			"returns 429. Cap retries per partner window and open a PR."},
		{
			Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
			ToolInput: `{"file_path":"internal/settlement/retry.go"}`,
			ToolResult: `{"content":"func (r *Retrier) Next(attempt int) (time.Duration, bool) {\n` +
				`\tif attempt >= r.max {\n\t\treturn 0, false\n\t}\n\treturn r.backoff(attempt), true\n}\n"}`,
		},
		{
			Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
			ToolInput: `{"file_path":"internal/settlement/retry.go",` +
				`"old_string":"func (r *Retrier) Next(attempt int) (time.Duration, bool) {\n\tif attempt >= r.max {",` +
				`"new_string":"func (r *Retrier) Next(partner string, attempt int) (time.Duration, bool) {\n` +
				`\tif attempt >= r.max || !r.window.Allow(partner) {"}`,
			ToolResult: `{"file_path":"internal/settlement/retry.go","content":"1 edit applied"}`,
		},
		{
			Kind: stepTool, ToolName: toolWrite, ToolKind: toolKindEdit,
			ToolInput: `{"file_path":"internal/settlement/retry_window.go","content":"package settlement\n\n` +
				`// partnerWindow allows at most limit retries per partner in each window.\n` +
				`type partnerWindow struct {\n\tlimit  int\n\twindow time.Duration\n\tcounts map[string]windowCount\n}\n"}`,
			ToolResult: `{"file_path":"internal/settlement/retry_window.go","content":"File written"}`,
		},
		{
			Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
			ToolInput:  `{"command":"go test ./internal/settlement/... -run 'Retry|Window' -count=1"}`,
			ToolResult: `{"stdout":"ok  \tgithub.com/northstar-pay/payments-platform/internal/settlement\t1.42s"}`,
		},
		{
			Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
			ToolInput: `{"command":"gh pr create --title \"Cap settlement retries per partner window\" ` +
				`--base main --head fix/settlement-retry-budget"}`,
			ToolResult: `{"stdout":"https://github.com/northstar-pay/payments-platform/pull/482"}`,
		},
		{Kind: stepAgent, Text: "Opened #482. Retries now stop at 6 per partner per minute, so a 429 storm " +
			"from MercadoX no longer drains the budget for Stone. Checks are green; it needs a reviewer from " +
			"the settlement owners."},
	}
}

func webhookSecretsSession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionWebhookSecretsID, Name: "Rotate PSP webhook signing secrets",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentReleaseManager,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(5, 20), EndedAt: clock.hoursMinutesAgo(4, 45),
		StopDetail: "Pull request #479 opened; one contract test failing.",
		Input:      2870, Output: 1120, CostUSD: 0.127,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Rotate the PSP webhook signing secrets and accept both the old and new " +
				"secret for 24 hours so no webhook is dropped during the switch."},
			{
				Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"internal/webhooks/verify.go",` +
					`"old_string":"return hmac.Equal(sig, expected(secret, body))",` +
					`"new_string":"return hmac.Equal(sig, expected(secret, body)) ||\n` +
					`\t(previous != nil && time.Now().Before(previous.ExpiresAt) && hmac.Equal(sig, expected(previous.Secret, body)))"}`,
				ToolResult: `{"file_path":"internal/webhooks/verify.go","content":"1 edit applied"}`,
			},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"go test ./internal/webhooks/... -count=1"}`,
				ToolResult: `{"stdout":"--- FAIL: TestVerifyRejectsExpiredPrevious (0.00s)\n` +
					`    verify_test.go:88: expired previous secret accepted\nFAIL"}`,
			},
			{Kind: stepAgent, Text: "Opened #479 with the dual-secret window. One contract test fails: the " +
				"test clock is fixed, but the check reads time.Now(). I'll inject the clock instead of " +
				"loosening the test."},
		},
		PullRequest: &boardPullRequest{
			Branch: webhookSecretsBranch, Number: webhookSecretsPR,
			Title:       "Rotate PSP webhook signing secrets with a 24h overlap",
			ChecksState: "failing", Passed: 12, Failed: 1, FailedNames: []string{"contract-tests"},
			ReviewDecision: prReviewRequired, Mergeable: prMergeable,
			UpdatedAt: clock.hoursMinutesAgo(4, 40), ObservedAt: clock.minutesAgo(4),
		},
	}
}

func ledgerFlakeSession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionLedgerFlakeID, Name: "Fix flaky ledger reconciliation test",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentPlatformEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(2, 7), EndedAt: clock.daysHoursAgo(2, 6),
		StopDetail: "Flake fixed; test now pins the ledger cutoff.",
		Input:      1730, Output: 690, CostUSD: 0.071,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "TestReconcileDailyLedger fails about once a day in CI. Find out why."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"go test ./internal/ledger/... -run TestReconcileDailyLedger -count=200"}`,
				ToolResult: `{"stdout":"--- FAIL: TestReconcileDailyLedger (0.01s)\n` +
					`    reconcile_test.go:54: entries=41, want 42\nFAIL (3 of 200 runs)"}`,
			},
			{
				Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"internal/ledger/reconcile_test.go",` +
					`"old_string":"cutoff := time.Now().Truncate(24 * time.Hour)",` +
					`"new_string":"cutoff := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)"}`,
				ToolResult: `{"file_path":"internal/ledger/reconcile_test.go","content":"1 edit applied"}`,
			},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput:  `{"command":"go test ./internal/ledger/... -run TestReconcileDailyLedger -count=200"}`,
				ToolResult: `{"stdout":"ok  \tgithub.com/northstar-pay/payments-platform/internal/ledger\t2.10s"}`,
			},
			{Kind: stepAgent, Text: "The test read the wall clock, so an entry written just after midnight UTC " +
				"fell into the next day. It now uses a fixed cutoff; 200 runs pass."},
		}}
}

func pagerSummarySession(clock timeline) boardSessionStory {
	return boardSessionStory{
		ID: sessionPagerSummaryID, Name: "Summarize weekend pager load",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentSupportLead,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(1, 9), EndedAt: clock.daysHoursAgo(1, 8),
		StopDetail: "Weekend summary posted for the Monday sync.",
		Input:      1420, Output: 560, CostUSD: 0.058,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Summarize what paged us over the weekend for the Monday sync."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"./bin/pager-export --since sat --team payments --format summary"}`,
				ToolResult: `{"stdout":"5 pages: 3 settlement-retry-exhausted (MercadoX), ` +
					`1 webhook-signature-mismatch, 1 ledger-cutoff-drift"}`,
			},
			{Kind: stepAgent, Text: "Three of the five pages were MercadoX settlement retries running out, the " +
				"issue #482 addresses. The webhook mismatch lines up with the secret rotation in #479. The ledger " +
				"drift page was the flaky test's real-world twin and is already fixed."},
		}}
}
