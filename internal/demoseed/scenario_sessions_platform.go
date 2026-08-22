package demoseed

import (
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func platformSessions(clock timeline) []sessionStory {
	return []sessionStory{
		partnerReplaySession(clock),
		rollbackDrillSession(clock),
		settlementRetrySession(clock),
		settlementWorkerSession(clock),
		releaseTrainSession(clock),
		docsRefreshSession(clock),
	}
}

func partnerReplaySession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[0], Name: "Partner replay analysis",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentPlatformEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(21, 18), EndedAt: clock.hoursMinutesAgo(21, 7),
		StopDetail: "Replay coverage reported per market.",
		Input:      1840, Output: 612, CostUSD: 0.084,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Investigate duplicate authorization replays from the MercadoX connector. " +
				"Give me coverage by market and a concrete release implication."},
			{Kind: stepAgent, Text: "I'll reconcile the replay window against authorization and settlement " +
				"records for each market."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"./scripts/replay-audit.sh --market br --market mx --window 24h"}`,
				ToolResult: `{"stdout":"BR  replayed=10214  duplicates=0  settlement_drift=0.00\n` +
					`MX  replayed=7788   duplicates=0  settlement_drift=n/a",` +
					`"stderr":"MX export gap detected: 14:08-14:30 UTC (22 minutes)"}`,
			},
			{
				Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
				ToolInput: `{"file_path":"data/partners/mercadox/manifest.json"}`,
				ToolResult: `{"content":"{\n  \"batches\": 47,\n  \"last_complete\": \"2026-08-20T14:08:00Z\",\n` +
					"  \\\"gap_minutes\\\": 22\\n}\"}",
			},
			{Kind: stepAgent, Text: "Brazil replay coverage is clean and supports a controlled canary. Hold Mexico " +
				"promotion until MercadoX supplies the missing 22-minute export and the same reconciliation passes."},
		},
	}
}

func rollbackDrillSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[5], Name: "Rollback drill for the 25% canary",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentCheckoutEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(1, 6), EndedAt: clock.daysHoursAgo(1, 5),
		StopDetail: "Rollback path rehearsed end to end.",
		Input:      2210, Output: 830, CostUSD: 0.104,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Rehearse the Brazil rollback path against staging and prove it completes " +
				"inside the five-minute threshold window."},
			{Kind: stepThinking, Text: "The threshold is 1.0% for five minutes, so the rollback itself has to " +
				"finish well inside that window or the alert and the fix overlap."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput:  `{"command":"make canary-rollback ENV=staging MARKET=br"}`,
				ToolResult: `{"stdout":"draining canary pool...\nrouting 100% to stable\nrollback complete in 48s"}`,
			},
			{
				Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"config/checkout/alerts.yaml",` +
					`"old_string":"  window: 10m","new_string":"  window: 5m"}`,
				ToolResult: `{"file_path":"config/checkout/alerts.yaml","content":"1 edit applied"}`,
			},
			{Kind: stepAgent, Text: "Rollback completes in 48 seconds against staging, well inside the five-minute " +
				"threshold. The alert window was still 10 minutes and is now aligned at 5."},
		},
	}
}

func settlementRetrySession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[6], Name: "Settlement retry investigation",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentPlatformEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(4, 20), EndedAt: clock.hoursMinutesAgo(3, 40),
		StopDetail: "Root cause isolated; fix delegated to a worker session.",
		Input:      3120, Output: 1180, CostUSD: 0.148,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Settlement retries are stacking up on the MercadoX connector. Find the cause " +
				"and delegate the fix."},
			{
				Kind: stepTool, ToolName: toolGrep, ToolKind: toolKindSearch,
				ToolInput: `{"pattern":"retry_exhausted","glob":"*.log","path":"var/log/settlement"}`,
				ToolResult: `{"stdout":"var/log/settlement/worker-03.log\nvar/log/settlement/worker-07.log\n` +
					`var/log/settlement/worker-11.log"}`,
			},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"grep -c retry_exhausted var/log/settlement/*.log"}`,
				ToolResult: `{"stdout":"var/log/settlement/worker-03.log:214\n` +
					"var/log/settlement/worker-07.log:198\\nvar/log/settlement/worker-11.log:221\"}",
			},
			{Kind: stepAgent, Text: "Three workers exhaust retries against the same partner endpoint, which " +
				"returns 429 above 40 requests per second. I'm spawning a worker session to add the rate limiter."},
		},
	}
}

func settlementWorkerSession(clock timeline) sessionStory {
	failure := &sessionFailureStory{
		Kind:    string(store.FailurePrompt),
		Summary: "Rate limiter patch failed contract tests: settlement batch ordering regressed.",
	}
	return sessionStory{
		ID: scenarioSessionIDs[7], Name: "Add settlement rate limiter",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentCheckoutEngineer,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeSpawned),
		ParentID: scenarioSessionIDs[6], SpawnRole: "worker",
		StartedAt: clock.hoursMinutesAgo(3, 50), EndedAt: clock.hoursMinutesAgo(3, 12),
		StopReason: string(store.StopError), StopDetail: "Contract tests failed after the patch.",
		Failure: failure,
		Input:   2640, Output: 910, CostUSD: 0.121,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Add a 40 requests-per-second limiter in front of the MercadoX settlement " +
				"client and keep batch ordering intact."},
			{
				Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"internal/settlement/mercadox_client.go",` +
					`"old_string":"func (c *Client) Submit(ctx context.Context, batch Batch) error {",` +
					`"new_string":"func (c *Client) Submit(ctx context.Context, batch Batch) error {\n\t` +
					`if err := c.limiter.Wait(ctx); err != nil {\n\t\treturn err\n\t}"}`,
				ToolResult: `{"file_path":"internal/settlement/mercadox_client.go","content":"1 edit applied"}`,
			},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"go test ./internal/settlement/... -run Contract"}`,
				ToolResult: `{"stdout":"--- FAIL: TestContractBatchOrdering (1.84s)","stderr":` +
					`"batch 2 settled before batch 1: limiter reorders concurrent submits","error":"exit status 1"}`,
			},
			{Kind: stepAgent, Text: "The limiter serializes at the wrong layer and reorders concurrent submits. " +
				"Batch ordering is a contract, so I'm stopping rather than shipping a passing-looking patch."},
		},
	}
}

func releaseTrainSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[8], Name: "Payments release train",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentReleaseManager,
		Provider: providerCodex, Model: modelCodex, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(3, 7), EndedAt: clock.daysHoursAgo(3, 6),
		StopDetail: "Release window closed with four changes.",
		Input:      1740, Output: 590, CostUSD: 0.079,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Close this week's payments release train and confirm every change has a " +
				"rollback path and a named owner."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput: `{"command":"git log --oneline origin/release..HEAD"}`,
				ToolResult: `{"stdout":"a91c4e2 fix: align canary alert window to 5m\n` +
					"7d2f118 feat: settlement batch checkpoints\\nc03ba5d fix: pt-BR fallback charge wording\\n" +
					"1f4e9aa perf: connector pool sizing from forecast\"}",
			},
			{
				Kind: stepTool, ToolName: toolTodoWrite, ToolKind: toolKindOther,
				ToolInput: `{"todos":[` +
					`{"content":"a91c4e2 rollback: revert alert window","status":"completed"},` +
					`{"content":"7d2f118 rollback: disable checkpoint flag","status":"completed"},` +
					`{"content":"c03ba5d rollback: restore prior copy","status":"completed"},` +
					`{"content":"1f4e9aa rollback: pin pool size","status":"completed"}]}`,
				ToolResult: `{"content":"4 todos tracked, 4 completed."}`,
			},
			{Kind: stepAgent, Text: "Four changes in the window, each with a rollback path and a named owner. " +
				"The connector pool sizing change is the only one that needs a forecast refresh before the next train."},
		},
	}
}

func docsRefreshSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[9], Name: "Runbook freshness pass",
		WorkspaceKey: workspaceKeyPlatform, AgentName: agentDocsSteward,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(6, 4), EndedAt: clock.daysHoursAgo(6, 3),
		StopDetail: "Two runbooks flagged as stale.",
		Input:      1420, Output: 505, CostUSD: 0.062,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Check the payments runbooks against the current rollout controls and flag " +
				"anything whose steps no longer match."},
			{
				Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
				ToolInput: `{"file_path":"runbooks/checkout-rollback.md"}`,
				ToolResult: `{"content":"# Checkout rollback\n\n1. Page payments on-call\n2. Set canary to 0%\n` +
					"3. Wait 10 minutes for the alert to clear\\n\"}",
			},
			{
				Kind: stepTool, ToolName: toolWrite, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"runbooks/STALE.md","content":"# Stale runbooks\n\n` +
					`- checkout-rollback.md: step 3 still says 10 minutes; the alert window is 5m\n` +
					`- mercadox-replay.md: references the pre-checkpoint batch format\n"}`,
				ToolResult: `{"file_path":"runbooks/STALE.md","content":"written"}`,
			},
			{Kind: stepAgent, Text: "Two runbooks are stale against the shipped controls. I flagged them instead " +
				"of rewriting them, because both changes need the owning engineer to confirm the new steps."},
		},
	}
}
