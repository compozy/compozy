package demoseed

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/session"
)

// Bundled agents ship with CompozyOS (the spec-cycle extension and the general agent), so
// the seed writes no definition for them; it only gives them the history a team using them has.
const (
	agentOrchestrator    = "orchestrator"
	agentCodeImplementer = "code_implementer"
	agentReviewer        = "reviewer"
	agentSpecConductor   = "spec_conductor"
	agentGeneral         = "general"
	permissionsAuto      = "auto"

	sessionOrchestratorMainID = "sess_northstar_orchestrator_main"
	sessionExportRenameID     = "sess_northstar_export_column_rename"
	sessionRetryReviewID      = "sess_northstar_retry_budget_review"
	sessionExportSpecID       = "sess_northstar_settlement_export_spec"
	sessionIncidentDigestID   = "sess_northstar_incident_digest"
	exportRenameBranch        = "wl-20261106-01"
	workLogRelative           = ".compozy/work-log.md"
)

// bundledAgentPermissions is the permission mode each bundled definition runs at.
var bundledAgentPermissions = map[string]string{
	agentOrchestrator: permissionsAuto, agentCodeImplementer: approveReads,
	agentReviewer: approveReads, agentSpecConductor: approveReads, agentGeneral: approveReads,
}

func bundledSessionIDs() []string {
	return []string{sessionExportRenameID, sessionRetryReviewID, sessionExportSpecID, sessionIncidentDigestID}
}

func bundledAgentSessions(clock timeline) []sessionStory {
	return []sessionStory{
		exportRenameSession(clock), retryReviewSession(clock), exportSpecSession(clock), incidentDigestSession(clock),
	}
}

func bundledSession(id, name, agent string, started, ended time.Time, detail string) sessionStory {
	return sessionStory{
		ID: id, Name: name, WorkspaceKey: workspaceKeyPlatform, AgentName: agent,
		Provider: providerClaude, Model: modelClaudeOpus, SessionType: string(session.SessionTypeUser),
		StartedAt: started, EndedAt: ended, StopDetail: detail,
	}
}

func exportRenameSession(clock timeline) sessionStory {
	story := bundledSession(sessionExportRenameID, "wl-20261106-01 Rename the MXN settlement total column",
		agentCodeImplementer, clock.daysHoursAgo(2, 6), clock.daysHoursAgo(2, 5), "Delivered on PR #488.")
	story.Input, story.Output, story.CostUSD = 21400, 3900, 0.118
	story.Steps = []transcriptStep{
		operatorSays("Work item wl-20261106-01. In the worktree " + exportRenameBranch + ", rename the settlement " +
			"export column mx_total to total_mxn in services/settlement/export and its golden test. Run go test " +
			"./services/settlement/... and open a PR. Do not touch the partner clients."),
		{
			Kind:       stepTool,
			ToolName:   toolGrep,
			ToolKind:   toolKindSearch,
			ToolInput:  `{"pattern":"mx_total","path":"services/settlement"}`,
			ToolResult: `{"stdout":"services/settlement/export/columns.go:31: \"mx_total\",\nservices/settlement/export/testdata/daily.golden.csv:1: date,partner,mx_total"}`,
		},
		{
			Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
			ToolInput: `{"file_path":"services/settlement/export/columns.go","old_string":"\t\"mx_total\",",` +
				`"new_string":"\t\"total_mxn\","}`,
			ToolResult: `{"content":"Updated services/settlement/export/columns.go"}`,
		},
		shellStep("go test ./services/settlement/...",
			"ok  \tnorthstar/services/settlement/export\t0.412s\nok  \tnorthstar/services/settlement/ledger\t1.087s"),
		shellStep("gh pr create --title \"Rename the MXN settlement total column\" --base main",
			"https://github.com/"+platformRepo+"/pull/488"),
		agentSays(
			"Renamed mx_total to total_mxn in the export and its golden file. Settlement tests pass. Opened #488 " +
				"from " + exportRenameBranch + "; the partner clients are untouched.",
		),
	}
	return story
}

func retryReviewSession(clock timeline) sessionStory {
	story := bundledSession(sessionRetryReviewID, "Review #482 settlement retry cap", agentReviewer,
		clock.hoursMinutesAgo(2, 10), clock.hoursMinutesAgo(1, 52), "Two findings recorded on #482.")
	story.Input, story.Output, story.CostUSD = 18200, 2600, 0.094
	story.Steps = []transcriptStep{
		operatorSays("Review #482 against the settlement retry runbook. Record findings with file and line."),
		shellStep("gh pr diff 482 -R "+platformRepo+" --name-only",
			"services/settlement/retry/budget.go\nservices/settlement/retry/budget_test.go"),
		{
			Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
			ToolInput: `{"file_path":"services/settlement/retry/budget.go","offset":40,"limit":20}`,
			ToolResult: `{"content":"func (b *Budget) Allow(partner string, now time.Time) bool {\n` +
				`\twindow := b.windows[partner]\n\treturn window.count < b.limit\n}"}`,
		},
		agentSays("Two findings on #482. budget.go:42 reads b.windows[partner] without a lock while Record writes it " +
			"from the worker pool, so the cap can be exceeded under load. budget_test.go covers one partner only; the " +
			"runbook's MercadoX case with two overlapping windows has no test. Neither blocks the canary flag, both " +
			"block merge."),
	}
	return story
}

func exportSpecSession(clock timeline) sessionStory {
	story := bundledSession(sessionExportSpecID, "Deliver the settlement export v2 spec", agentSpecConductor,
		clock.daysHoursAgo(4, 8), clock.daysHoursAgo(4, 3), "Spec delivered: 3 slices merged.")
	story.Input, story.Output, story.CostUSD = 64800, 9100, 0.341
	story.Steps = []transcriptStep{
		operatorSays("Deliver the settlement export v2 spec in docs/specs/settlement-export-v2. Three slices, one PR."),
		{
			Kind: stepTool, ToolName: toolTodoWrite, ToolKind: toolKindOther,
			ToolInput: `{"todos":[{"content":"Slice 1: per-partner totals","status":"completed"},` +
				`{"content":"Slice 2: currency columns","status":"completed"},` +
				`{"content":"Slice 3: partial refunds line","status":"completed"},` +
				`{"content":"Integrate and verify","status":"completed"}]}`,
			ToolResult: `{"content":"4 todos tracked, 4 completed."}`,
		},
		shellStep("go test ./services/settlement/export/... && gh pr checks 474 -R "+platformRepo,
			"ok  \tnorthstar/services/settlement/export\t0.618s\nAll checks were successful: 9 passed"),
		agentSays(
			"Settlement export v2 is delivered. The three slices ran in their own worktrees, integrated on #474, " +
				"and all nine checks pass. The partial refunds line from slice 3 is the base #446 will need.",
		),
	}
	return story
}

func incidentDigestSession(clock timeline) sessionStory {
	story := bundledSession(sessionIncidentDigestID, "Summarize this week's settlement incidents", agentGeneral,
		clock.daysHoursAgo(1, 4), clock.daysHoursAgo(1, 3), "Summary delivered.")
	story.Input, story.Output, story.CostUSD = 9600, 1400, 0.041
	story.Steps = []transcriptStep{
		operatorSays("Summarize this week's settlement incidents in five lines for the Friday review."),
		{
			Kind:       stepTool,
			ToolName:   toolGrep,
			ToolKind:   toolKindSearch,
			ToolInput:  `{"pattern":"severity:","path":"ops/incidents","glob":"2026-11-*.md"}`,
			ToolResult: `{"stdout":"ops/incidents/2026-11-03-mercadox-502.md: severity: 2\nops/incidents/2026-11-05-ledger-lock.md: severity: 3"}`,
		},
		agentSays("Two incidents this week. MercadoX batch close returned 502 from the deprecated v2 endpoint for " +
			"six hours (sev 2); retries exhausted the budget until the client moved to v3. Ledger lock timeouts during " +
			"the 02:00 close (sev 3) retried and succeeded; no customer impact. Follow-ups: #482 caps retries per " +
			"partner window, and the queue cutover is paused until Wednesday."),
	}
	return story
}

func orchestratorInstance(clock timeline) instanceStory {
	created := clock.daysHoursAgo(6, 5)
	return instanceStory{
		Agent: agentOrchestrator, WorkspaceKey: workspaceKeyPlatform, Bundled: true,
		MainID: sessionOrchestratorMainID, MainName: "Orchestrator",
		Provider: providerClaude, Model: modelClaudeOpus,
		CreatedAt: created, Interval: time.Hour, ActiveFrom: 0, ActiveTo: 24, QuietEvery: 3,
		SeenAt: clock.hoursAgo(3), NextCheckIn: 34 * time.Minute,
		Turns: []mainTurn{
			{At: clock.daysHoursAgo(2, 6).Add(-4 * time.Minute), Steps: []transcriptStep{
				operatorSays("Small one: rename the MXN settlement total column from mx_total to total_mxn in the " +
					"export and its golden test. Partner clients stay as they are."),
				{
					Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
					ToolInput: `{"file_path":"` + workLogRelative + `","old_string":"# Work log\n",` +
						`"new_string":"# Work log\n\n- [ ] wl-20261106-01 | Rename the MXN settlement total column | source: chat\n"}`,
					ToolResult: `{"content":"Updated ` + workLogRelative + `"}`,
				},
				nativeStep(
					"compozy__worktree_create",
					`{"workspace_id":"$WORKSPACE_ID","name":"`+exportRenameBranch+`"}`,
					`{"worktree":{"name":"`+exportRenameBranch+`","branch":"`+exportRenameBranch+`","state":"ready"}}`,
				),
				nativeStep(
					"compozy__session_create",
					`{"agent":"code_implementer","worktree":"`+exportRenameBranch+`","name":"wl-20261106-01 Rename the MXN settlement total column"}`,
					`{"session_id":"`+sessionExportRenameID+`"}`,
				),
				nativeStep(
					"compozy__monitor_create",
					`{"kind":"session","target_session_ids":["`+sessionExportRenameID+`"],"description":"wl-20261106-01 executor settles"}`,
					`{"monitor":{"status":"active","kind":"session"}}`,
				),
				agentSays("Accepted as wl-20261106-01 and dispatched to a code_implementer in its own worktree. " +
					"I'll check the result when it settles."),
			}},
			{At: clock.daysHoursAgo(2, 5).Add(6 * time.Minute), Steps: []transcriptStep{
				operatorSays("The executor finished. Is wl-20261106-01 done?"),
				nativeStep("compozy__session_history", `{"session_id":"`+sessionExportRenameID+`","limit":5}`,
					`{"content":"Renamed mx_total to total_mxn. Settlement tests pass. Opened #488."}`),
				shellStep("gh pr checks 488 -R "+platformRepo, "All checks were successful: 9 passed"),
				agentSays("Done. #488 renames the column in the export and the golden file, settlement tests pass, " +
					"and all nine checks are green. I closed wl-20261106-01 with the PR as evidence. It's ready for " +
					"your merge."),
			}},
		},
	}
}

func nativeStep(tool, input, result string) transcriptStep {
	return transcriptStep{Kind: stepTool, ToolName: tool, ToolKind: toolKindOther, ToolInput: input, ToolResult: result}
}

// writeWorkLog writes the Orchestrator's work log; its check-in collector reads this file.
func writeWorkLog(state *scenario) error {
	record, err := state.recordFor(workspaceKeyPlatform)
	if err != nil {
		return err
	}
	content := "# Work log\n\n" +
		"- [x] wl-20261106-01 | " + state.clock.daysHoursAgo(2, 6).Format(time.RFC3339) +
		" | Rename the MXN settlement total column | evidence: PR #488, 9 checks passed\n"
	path := filepath.Join(record.RootDir, filepath.FromSlash(workLogRelative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("demo seed: create work log folder: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("demo seed: write work log: %w", err)
	}
	return nil
}
