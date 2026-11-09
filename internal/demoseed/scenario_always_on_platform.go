package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
)

const (
	sessionIssueTriagerMainID = "sess_northstar_issue_triager_main"
	sessionReleaseWatchMainID = "sess_northstar_release_watch_main"
	sessionLogTriageMainID    = "sess_northstar_log_triage_main"
	sessionRiskMonitorMainID  = "sess_northstar_risk_monitor_main"
	platformRepo              = "northstar-pay/payments-platform"
	issueURLPrefix            = "https://github.com/" + platformRepo + "/issues/"
)

func operatorSays(text string) transcriptStep { return transcriptStep{Kind: stepUser, Text: text} }
func agentSays(text string) transcriptStep    { return transcriptStep{Kind: stepAgent, Text: text} }
func agentThinks(text string) transcriptStep  { return transcriptStep{Kind: stepThinking, Text: text} }

func shellStep(command string, stdout string) transcriptStep {
	return transcriptStep{
		Kind:     stepTool,
		ToolName: toolBash,
		ToolKind: toolKindExecute,
		ToolInput: jsonObject(
			map[string]any{"command": command},
		),
		ToolResult: jsonObject(map[string]any{"stdout": stdout}),
	}
}

func scenarioInstances(clock timeline) []instanceStory {
	core := []instanceStory{
		issueTriagerInstance(clock), releaseWatchInstance(clock), logTriageInstance(clock), riskMonitorInstance(clock),
		orchestratorInstance(clock),
	}
	return append(core, rosterInstances(clock)...)
}

func draftReproAction() heartbeat.ActionDecl {
	return heartbeat.ActionDecl{
		ID: "draft-repro", Title: "Draft a reproduction note", AutoAccept: true,
		Instructions: "Reproduce the issue against main in a read-only session and write a short reproduction note: " +
			"steps, observed result, and the service that owns it. Do not change code.",
	}
}

func issueTriagerInstance(clock timeline) instanceStory {
	return instanceStory{
		Agent: agentIssueTriager, WorkspaceKey: workspaceKeyPlatform,
		MainID: sessionIssueTriagerMainID, MainName: displayIssueTriager,
		Provider: providerClaude, Model: modelClaude,
		CreatedAt: clock.daysHoursAgo(12, 3), Interval: 30 * time.Minute, ActiveFrom: 8, ActiveTo: 22, QuietEvery: 2,
		Actions: []heartbeat.ActionDecl{draftReproAction()},
		SeenAt:  clock.minutesAgo(95), NextCheckIn: 11 * time.Minute,
		Turns: []mainTurn{{At: clock.daysHoursAgo(12, 3).Add(2 * time.Minute), Steps: []transcriptStep{
			operatorSays("Watch payments-platform issues for me. Tell me only about the ones nobody has picked up, " +
				"and skip anything a PR already references."),
			agentThinks("Coverage has three places to look: open pull requests, running sessions, and open tasks. " +
				"I should check what is open today so the first check-in does not report old issues as new."),
			shellStep("gh issue list -R "+platformRepo+" --state open --limit 20 --json number,title,assignees",
				"#429 Settlement retry budget ignores partner 429s (assignee: platform-engineer)\n"+
					"#427 Docs: rollback drill still names the old queue (assignee: docs-steward)\n"+
					"#424 Flaky test: TestCaptureReplayMexico (no assignee, PR #425 open)"),
			agentSays("I'm watching northstar-pay/payments-platform from now on. The three open issues are covered: " +
				"#429 and #427 have owners, and PR #425 references #424. I'll check every 30 minutes between 08:00 and " +
				"22:00 and only write here when a new issue has no owner."),
		}}},
		Notes: []noteStory{
			{
				At:    clock.daysAgo(11),
				Scope: "coverage",
				Text:  "A PR body that says \"Fixes #N\" covers the issue even when the branch name does not mention it.",
			},
			{
				At:    clock.daysAgo(6),
				Scope: "routing",
				Text:  "Refund and webhook issues go to checkout-engineer; settlement and partner issues go to platform-engineer.",
			},
			{
				At:    clock.daysAgo(2),
				Scope: "operator",
				Text:  "Pedro wants the affected service named in the proposal title's first sentence, not only in the body.",
			},
		},
		Beats: issueTriagerBeats(clock),
	}
}

func issueTriagerBeats(clock timeline) []checkInBeat {
	return []checkInBeat{
		{
			At: clock.daysHoursAgo(6, 4), Result: checkResultReported,
			Proposals: []proposalStory{{
				Title: "#431 has no owner", ItemKeys: []string{"new-issues:431"},
				Body: "Refunds service: the Pix refund webhook is processed twice when the partner retries within " +
					"two seconds, so one refund id posts two ledger entries. " + issueURLPrefix + "431",
			}},
			PickAt: clock.daysHoursAgo(6, 3),
			Reply: []transcriptStep{
				agentThinks("Re-check coverage first: the report is an hour old and someone may have opened a PR."),
				shellStep("gh pr list -R "+platformRepo+" --state open --search 431 --json number,title",
					"[]"),
				{
					Kind:       stepTool,
					ToolName:   toolGrep,
					ToolKind:   toolKindSearch,
					ToolInput:  `{"pattern":"refund_id","path":"services/refunds","glob":"*.go"}`,
					ToolResult: `{"stdout":"services/refunds/webhook_handler.go:58: id := payload.RefundID\nservices/refunds/ledger.go:91: entry.RefundID = id"}`,
				},
				{
					Kind:      stepTool,
					ToolName:  toolRead,
					ToolKind:  toolKindRead,
					ToolInput: `{"file_path":"services/refunds/webhook_handler.go","offset":50,"limit":20}`,
					ToolResult: `{"content":"func (h *WebhookHandler) Handle(ctx context.Context, payload Payload) error {\n` +
						`\tid := payload.RefundID\n\treturn h.ledger.Post(ctx, id, payload.Amount)\n}"}`,
				},
				agentSays("#431 is still uncovered. The handler posts to the ledger without checking whether the " +
					"refund id was already applied, so a partner retry inside the dedupe window posts twice. It belongs " +
					"to checkout-engineer, who owns services/refunds. Smallest next step: an idempotency check on " +
					"refund id before Post."),
			},
		},
		{
			At: clock.daysHoursAgo(2, 5), Result: checkResultReported,
			Proposals: []proposalStory{
				{
					Title:    "#438 has no owner",
					ItemKeys: []string{"new-issues:438"},
					Action:   "draft-repro",
					Body: "Checkout web: the boleto receipt shows the expiry time in UTC instead of the buyer's time zone. " +
						"The report includes steps to reproduce. " + issueURLPrefix + "438",
				},
			},
			Reply: []transcriptStep{
				shellStep("gh issue view 438 -R "+platformRepo+" --json title,body",
					"Boleto receipt shows expiry in UTC\nSteps: create a boleto at 21:30 BRT, open the receipt, "+
						"expiry reads 00:30 next day."),
				{
					Kind:       stepTool,
					ToolName:   toolGrep,
					ToolKind:   toolKindSearch,
					ToolInput:  `{"pattern":"ExpiresAt.Format","path":"apps/checkout-web","glob":"*.ts*"}`,
					ToolResult: `{"stdout":"apps/checkout-web/src/receipt/boleto-receipt.tsx:42: formatTime(boleto.expiresAt, \"UTC\")"}`,
				},
				agentSays(
					"Reproduction note for #438: a boleto created at 21:30 in São Paulo shows an expiry of 00:30 " +
						"the next day. boleto-receipt.tsx formats expiresAt with a fixed UTC zone instead of the buyer's zone. " +
						"Owner: checkout web. No code changed.",
				),
			},
		},
		{
			At: clock.hoursMinutesAgo(2, 40), Result: checkResultReported,
			Proposals: []proposalStory{
				{
					Title: "#446 has no owner", ItemKeys: []string{"new-issues:446"},
					Body: "Settlement reports: the Mexico SPEI settlement report leaves out partial refunds, so the " +
						"daily total is higher than the partner statement. " + issueURLPrefix + "446",
				},
				{
					Title: "#447 has no owner", ItemKeys: []string{"new-issues:447"},
					Body: "Checkout web: the card retry banner stays on screen after a successful 3DS challenge. " +
						"The report includes a screen recording. " + issueURLPrefix + "447",
				},
			},
		},
	}
}

func releaseWatchInstance(clock timeline) instanceStory {
	return instanceStory{
		Agent: agentReleaseWatch, WorkspaceKey: workspaceKeyPlatform,
		MainID: sessionReleaseWatchMainID, MainName: displayReleaseWatch,
		Provider: providerCodex, Model: modelCodex,
		CreatedAt: clock.daysHoursAgo(9, 2), Interval: 45 * time.Minute, ActiveFrom: 0, ActiveTo: 24, QuietEvery: 2,
		SeenAt: clock.hoursMinutesAgo(10, 20), NextCheckIn: 26 * time.Minute,
		Turns: []mainTurn{
			{At: clock.daysHoursAgo(9, 2).Add(3 * time.Minute), Steps: []transcriptStep{
				operatorSays(
					"Follow the payments release train. Tell me when a candidate goes red, loses its rollback " +
						"path, or is ready and waiting on someone. Stay quiet otherwise.",
				),
				{
					Kind:       stepTool,
					ToolName:   toolLoopStatus,
					ToolKind:   toolKindRead,
					ToolInput:  `{"workspace_id":"$WORKSPACE_ID","name":"release-train"}`,
					ToolResult: `{"content":"release-train: 1 run in progress (rc-2026.11.1), 3 candidate changes, all with rollback notes."}`,
				},
				agentSays("Following the release train. rc-2026.11.1 is in progress with three changes, each with a " +
					"rollback note. I'll check every 45 minutes, around the clock, since builds and canaries run overnight."),
			}},
			{At: clock.daysHoursAgo(3, 6), Steps: []transcriptStep{
				operatorSays("rc-2026.11.2 shipped. Anything left open from it?"),
				agentSays(
					"Nothing open. Its four changes merged, the canary held for the full hour, and the rollback " +
						"drill task closed this morning. The next candidate, rc-2026.11.3, starts collecting changes now.",
				),
			}},
		},
		Notes: []noteStory{
			{
				At:    clock.daysAgo(8),
				Scope: "release",
				Text:  "A candidate is ready only after the settlement-replay check passes; unit checks alone do not count.",
			},
			{
				At:    clock.daysAgo(3),
				Scope: "operator",
				Text:  "Pedro reviews release candidates after 10:00 São Paulo time; morning reports can wait for him.",
			},
		},
		Beats: releaseWatchBeats(clock),
	}
}

func releaseWatchBeats(clock timeline) []checkInBeat {
	return []checkInBeat{
		{
			At: clock.daysHoursAgo(5, 7), Result: checkResultReported,
			Proposals: []proposalStory{{
				Title: "rc-2026.11.2 is ready and waiting on you",
				Body: "All checks passed on rc-2026.11.2, including settlement-replay. The release task needs an " +
					"operator approval before the canary starts.",
				ItemKeys: []string{"release:rc-2026.11.2"},
			}},
			PickAt: clock.daysHoursAgo(5, 5),
			Reply: []transcriptStep{
				{
					Kind:       stepTool,
					ToolName:   toolTaskRead,
					ToolKind:   toolKindRead,
					ToolInput:  `{"workspace_id":"$WORKSPACE_ID","id":"task_northstar_release_train"}`,
					ToolResult: `{"content":"Release train: rc-2026.11.2, 4 changes, approval pending (operator)."}`,
				},
				agentSays(
					"rc-2026.11.2 is still green and the approval is the only thing left. Approve the release " +
						"task when you're ready; the canary starts at 5% and the rollback threshold is the usual 1.0% " +
						"authorization errors for five minutes.",
				),
			},
		},
		{
			At: clock.hoursMinutesAgo(7, 10), Result: checkResultReported,
			Proposals: []proposalStory{{
				Title: "rc-2026.11.3 has a red check",
				Body: "settlement-replay failed on rc-2026.11.3 after the retry-budget change merged. The last " +
					"green run was rc-2026.11.2. The failing case is the Mexico partial-refund replay.",
				ItemKeys: []string{"release:rc-2026.11.3", "check:settlement-replay"},
			}},
		},
		{
			At: clock.hoursMinutesAgo(4, 25), Result: checkResultReported,
			Proposals: []proposalStory{{
				Title: "The retry-budget change has no rollback note",
				Body: "The settlement retry-budget change in rc-2026.11.3 is the only change in the candidate " +
					"without a rollback note. The release train needs one before the canary.",
				ItemKeys: []string{"release:rc-2026.11.3", "change:settlement-retry-budget"},
			}},
		},
	}
}
