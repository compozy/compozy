package demoseed

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func logTriageInstance(clock timeline) instanceStory {
	return instanceStory{
		Agent: agentLogTriage, WorkspaceKey: workspaceKeyPlatform,
		MainID: sessionLogTriageMainID, MainName: "Log Triage",
		Provider: providerClaude, Model: modelClaude,
		CreatedAt: clock.daysHoursAgo(15, 4), Interval: time.Hour, ActiveFrom: 8, ActiveTo: 20, QuietEvery: 3,
		PauseAt: clock.daysHoursAgo(1, 2), SeenAt: clock.daysHoursAgo(1, 2),
		Turns: []mainTurn{
			{At: clock.daysHoursAgo(15, 4).Add(4 * time.Minute), Steps: []transcriptStep{
				operatorSays("Read the settlement worker error feed and group new failures by cause. I only care " +
					"about causes that are new, growing, or tied to a partner endpoint."),
				shellStep(
					"sh .compozy/agents/log-triage/sources/settlement-errors | head -c 400",
					`{"items":[{"key":"sig:ledger-lock-timeout","title":"ledger lock timeout","summary":"3 since last export"}]}`,
				),
				agentSays("I'm reading the settlement error feed every hour from 08:00 to 20:00. Today it has one " +
					"signature, a ledger lock timeout that retried and succeeded, so there is nothing to report yet."),
			}},
			{At: clock.daysHoursAgo(1, 2).Add(-3 * time.Minute), Steps: []transcriptStep{
				operatorSays("I'm pausing your check-ins while we move the settlement workers to the new queue. " +
					"The feed will be noisy until Wednesday. I'll resume you after the cutover."),
				agentSays("Understood. I'll stop checking the feed while I'm paused. When you resume me, my first " +
					"check reads everything that arrived during the cutover, so I'll separate migration noise from new " +
					"causes before reporting."),
			}},
		},
		Notes: []noteStory{
			{
				At:    clock.daysAgo(13),
				Scope: "noise",
				Text:  "Ledger lock timeouts that succeed on the second retry are expected during the 02:00 batch close.",
			},
			{
				At:    clock.daysAgo(5),
				Scope: "partners",
				Text:  "MercadoX returns 502 during their maintenance window, Tuesdays 01:00-02:00 São Paulo time.",
			},
		},
		Beats: []checkInBeat{
			{
				At: clock.daysHoursAgo(5, 9), Result: checkResultReported,
				Proposals: []proposalStory{{
					Title: "New failure: MercadoX settlement batch close returns 502",
					Body: "First seen 09:12 today, 14 times since. Every failure is the batch-close call to " +
						"mercadox/settlement; retries do not succeed. This is outside their maintenance window.",
					ItemKeys: []string{"settlement-errors:sig:mercadox-502-batch-close"},
				}},
				PickAt: clock.daysHoursAgo(5, 8),
				Reply: []transcriptStep{
					shellStep(
						"rg -n \"batch-close\" services/settlement/partners/mercadox --type go",
						"services/settlement/partners/mercadox/client.go:77: "+
							"req, err := http.NewRequest(\"POST\", c.base+\"/v2/batch-close\", body)",
					),
					agentSays("The 502s all come from the v2 batch-close endpoint, which MercadoX announced for " +
						"deprecation last month; v3 is live. This needs the partner client updated, so it belongs to " +
						"platform-engineer. Until then, the settlement retry worker exhausts its budget on every batch."),
				},
			},
			{
				At: clock.daysHoursAgo(3, 5), Result: checkResultReported,
				Proposals: []proposalStory{{
					Title: "Settlement retry exhaustion is growing",
					Body: "Retry-budget exhaustion went from 3 to 11 workers since yesterday. All of them are " +
						"MercadoX batches, the same cause as the 502 report.",
					ItemKeys: []string{"settlement-errors:sig:retry-budget-exhausted"},
				}},
			},
		},
	}
}

func riskMonitorInstance(clock timeline) instanceStory {
	return instanceStory{
		Agent: agentRiskMonitor, WorkspaceKey: workspaceKeyLaunch,
		MainID: sessionRiskMonitorMainID, MainName: "Risk Monitor",
		Provider: providerClaude, Model: modelClaude,
		CreatedAt: clock.daysHoursAgo(8, 1), Interval: 30 * time.Minute, ActiveFrom: 6, ActiveTo: 24, QuietEvery: 4,
		SeenAt: clock.hoursMinutesAgo(2, 5), NextCheckIn: 9 * time.Minute,
		Turns: []mainTurn{{At: clock.daysHoursAgo(8, 1).Add(2 * time.Minute), Steps: []transcriptStep{
			operatorSays("Watch dispute and chargeback alerts for Brazil and Mexico during the checkout launch. " +
				"Flag a market only when its dispute rate leaves the launch threshold, and always name the window."),
			{
				Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
				ToolInput:  `{"file_path":"` + riskAlertsFeed + `"}`,
				ToolResult: `{"content":"{\"items\":[]}"}`,
			},
			agentSays("Watching the dispute export for Brazil and Mexico every 30 minutes. The launch threshold is a " +
				"0.40% dispute rate per 4-hour window. The export is empty right now, so there is nothing to flag."),
		}}},
		Notes: []noteStory{
			{
				At:    clock.daysAgo(3),
				Scope: "brazil",
				Text: "Issuer retries after a soft decline show up as disputes for about two hours; " +
					"check authorization logs before calling it fraud.",
			},
		},
		Beats: []checkInBeat{
			{
				At: clock.daysHoursAgo(3, 6), Result: checkResultReported,
				Proposals: []proposalStory{
					{
						Title: "Brazil dispute rate crossed 0.45% in the 14:00 window",
						Body: "Brazil disputes reached 0.46% in the 14:00-18:00 window, above the 0.40% launch threshold. " +
							"Mexico stayed at 0.12%. Alert key br-disputes-1400.",
						ItemKeys: []string{"dispute-alerts:br-disputes-1400"},
					},
				},
				PickAt: clock.daysHoursAgo(3, 5),
				Reply: []transcriptStep{
					{
						Kind:       stepTool,
						ToolName:   toolGrep,
						ToolKind:   toolKindSearch,
						ToolInput:  `{"pattern":"soft_decline","path":"ops/risk","glob":"*.json"}`,
						ToolResult: `{"stdout":"ops/risk/auth-retries-1400.json: 212 issuer retries after soft_decline"}`,
					},
					agentSays(
						"Most of the 14:00 rise is authorization noise, not fraud: 212 issuer retries after soft " +
							"declines were filed as disputes and cleared within two hours. Confirmed fraud stayed flat. I " +
							"recommend keeping the Brazil canary at 25%. I added a note so I check authorization logs first next time.",
					),
				},
			},
			{
				At: clock.minutesAgo(52), Result: checkResultFailed, Reason: reasonTriageTimeout,
				Failure: `{"reason":"triage_timeout","detail":"The proactive check timed out."}`,
			},
			{
				At: clock.minutesAgo(21), Result: checkResultFailed, Reason: reasonTriageTimeout,
				Failure: `{"reason":"triage_timeout","detail":"The proactive check timed out."}`,
			},
		},
	}
}

// writeSourceFeeds writes the exports the check-in source scripts read, so a live check works.
func writeSourceFeeds(state *scenario) error {
	feeds := []struct{ workspaceKey, path, content string }{
		{
			workspaceKeyLaunch,
			riskAlertsFeed,
			`{"items":[{"key":"mx-disputes-1800","title":"Mexico dispute rate 0.21%",` +
				`"summary":"18:00-22:00 window, below the 0.40% launch threshold"}]}` + "\n",
		},
		{workspaceKeyPlatform, settlementLogFeed, `{"items":[{"key":"sig:queue-handoff-timeout",` +
			`"title":"settlement queue handoff timeout","summary":"7 since the cutover started"}]}` + "\n"},
	}
	for _, feed := range feeds {
		record, err := state.recordFor(feed.workspaceKey)
		if err != nil {
			return err
		}
		path := filepath.Join(record.RootDir, filepath.FromSlash(feed.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("demo seed: create source feed folder: %w", err)
		}
		if err := os.WriteFile(path, []byte(feed.content), 0o600); err != nil {
			return fmt.Errorf("demo seed: write source feed %q: %w", feed.path, err)
		}
	}
	return nil
}
