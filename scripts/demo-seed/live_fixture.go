package main

import "encoding/json"

// Live behaviors: what a session does once its prompt arrives through the mock provider.
const (
	liveHold     = "hold"
	liveAsk      = "permission"
	liveClarify  = "clarify"
	liveWait     = "wait"
	liveDelegate = "delegate"

	mockClaudeAgent = "demo-live-claude"
	mockCodexAgent  = "demo-live-codex"
	mockClaudeModel = "claude-sonnet-5"
	mockCodexModel  = "gpt-5.6-sol"
	mockOpusModel   = "claude-opus-5"

	agentCheckout = "checkout-engineer"
	agentPlatform = "platform-engineer"
	keySPEI       = "spei"

	projectPayments = "payments-platform"
	fieldToolID     = "tool_id"
	fieldName       = "name"
	fieldSteps      = "steps"
	fieldKind       = "kind"
	fieldToolCallID = "tool_call_id"
	fieldRawInput   = "raw_input"
	fieldMatch      = "match"
	stepNativeTool  = "native_tool_call"
)

// liveScenario is one session the driver keeps in a live state.
type liveScenario struct {
	key, project, agent, name, parent, existing string
	prompt, opening, closing                    string
	behavior                                    string
	tasks                                       []liveTask
}

type liveTask struct{ title, agent, task string }

var liveScenarios = []liveScenario{
	{
		key: "cutover", project: projectPayments, agent: agentCheckout,
		name:     "Cut settlement reads over to ledger v2",
		prompt:   "Cut settlement reads over to ledger v2. Split the verification and wait for the results.",
		opening:  "Three checks can run side by side, so I'll delegate them and pick this up when they report.",
		closing:  "All three are running. Nothing needs you until they report back.",
		behavior: liveDelegate,
		tasks: []liveTask{
			{
				"Shadow-read the partner balance view",
				agentPlatform,
				"Shadow-read the partner balance view and compare it with v1.",
			},
			{
				"Compare October ledger totals",
				agentPlatform,
				"Compare October ledger totals between v1 and v2 for every partner.",
			},
			{
				"Draft the read cutover flag",
				agentCheckout,
				"Draft the read cutover flag with a per-partner rollout.",
			},
		},
	},
	{
		key: keySPEI, project: projectPayments, agent: agentCheckout,
		name:     "Ship the Mexico partial-refund fix",
		prompt:   "Issue #446: ship the Mexico partial-refund fix. Two workers, and tell me when both settle.",
		opening:  "One worker fixes the report, the other adds the replay case. I'll wait for both.",
		closing:  "Both workers are running. I'll pick this up when they settle.",
		behavior: liveWait,
	},
	{
		key: "spei-report", parent: keySPEI, project: projectPayments, agent: agentPlatform,
		name:     "Fix the SPEI report totals",
		prompt:   "Fix the SPEI settlement report so partial refunds reduce the daily total.",
		opening:  "Reading how the report sums refunds per settlement day.",
		behavior: liveHold,
	},
	{
		key: "spei-replay", parent: keySPEI, project: projectPayments, agent: agentCheckout,
		name:     "Add the partial-refund replay case",
		prompt:   "Add a settlement-replay case for a partially refunded SPEI payment.",
		opening:  "Building the replay fixture from the partner's October statement.",
		behavior: liveHold,
	},
	{
		key: "replay", project: projectPayments, agent: agentCheckout,
		name:     "Investigate the rc-2026.11.3 settlement-replay failure",
		prompt:   "settlement-replay failed on rc-2026.11.3 after the retry-budget change. Find the failing case.",
		opening:  "Reading the settlement-replay log and the retry-budget diff.",
		behavior: liveHold,
	},
	{
		key: "canary", project: "launch-hq", agent: "product-lead",
		name:     "Pause the Mexico canary",
		prompt:   "Mexico disputes rose again. Drop the Mexico canary to 0% and keep Brazil at 25%.",
		opening:  "I need to edit the rollout config to drop the Mexico canary to 0%.",
		behavior: liveAsk,
	},
	{
		key: "qr", project: "checkout-web", agent: agentCheckout,
		name:     "Pick the Pix QR layout for small screens",
		prompt:   "The Pix QR code starts below the fold at 360 pixels. Fix the layout.",
		opening:  "There are two reasonable layouts, and the choice changes what buyers see first.",
		behavior: liveClarify,
	},
	{
		key: "tokens", project: "checkout-web", agent: agentCheckout,
		name:     "Migrate the installment selector to the new tokens",
		prompt:   "Migrate the installment selector to the new design tokens.",
		opening:  "Replacing the hard-coded colors and checking the visual diffs.",
		behavior: liveHold,
	},
	{
		key: "backtest", project: "risk-engine", agent: "fraud-analyst",
		name:     "Rerun the October backtest by partition",
		prompt:   "Rerun the October dispute backtest one partition at a time.",
		opening:  "Running the Brazil partition first, then Mexico with a smaller batch.",
		behavior: liveHold,
	},
	{
		key: "rerun", project: "platform-infra", agent: agentPlatform,
		name:     "Rehearse the queue cutover again",
		prompt:   "Rehearse the queue cutover in staging again.",
		opening:  "Draining the staging queue and timing the handoff.",
		behavior: liveHold,
	},
	{
		key: "release-watch", existing: "sess_northstar_release_watch_main",
		prompt:   "Walk me through what blocks rc-2026.11.3 right now.",
		opening:  "Reading the rc-2026.11.3 settlement-replay log and the retry-budget diff.",
		behavior: liveHold,
	},
}

func textStep(text string) map[string]any {
	return map[string]any{fieldKind: "assistant", "text": text}
}

func holdStep() map[string]any {
	return map[string]any{fieldKind: "driver_control", "driver_control": map[string]any{"action": "block_until_cancel"}}
}

func matchText(text string) map[string]any {
	return map[string]any{"user_text_contains": text}
}

func scenarioTurnSteps(scenario liveScenario) []map[string]any {
	steps := []map[string]any{textStep(scenario.opening)}
	switch scenario.behavior {
	case liveHold:
		return append(steps, holdStep())
	case liveAsk:
		return append(steps, map[string]any{
			fieldKind: "permission", fieldToolCallID: "live-permission-" + scenario.key,
			"title": "Edit config/checkout/rollout.yaml", "tool_kind": "edit", "path": "config/checkout/rollout.yaml",
			"status": "pending", "expect_decision": "allow-once", "emit_decision": true,
		}, textStep("Done. Mexico is at 0% and Brazil stays at 25%."))
	case liveClarify:
		return append(steps, map[string]any{
			fieldKind: stepNativeTool, fieldToolID: "compozy__clarify", fieldToolCallID: "live-clarify-" + scenario.key,
			fieldRawInput: map[string]any{
				"question": "Where should the Pix QR code go on small screens?",
				"choices":  []string{"Above the amount", "Below the amount, with a sticky copy button"},
			},
		}, textStep("Thanks, applying that layout."))
	case liveWait:
		return append(steps, map[string]any{
			fieldKind:       stepNativeTool,
			fieldToolID:     "compozy__monitor_create",
			fieldToolCallID: "live-wait-" + scenario.key,
			fieldRawInput: map[string]any{
				fieldKind: "timer", "delay_seconds": 21600, "description": "Wait for both workers to settle",
			},
		}, textStep(scenario.closing))
	case liveDelegate:
		for index, task := range scenario.tasks {
			steps = append(steps, map[string]any{
				fieldKind: stepNativeTool, fieldToolID: "compozy__subagent_delegate",
				fieldToolCallID: "live-delegate-" + scenario.key + "-" + string(rune('a'+index)),
				fieldRawInput: map[string]any{
					"title": task.title, "task": task.task, "target": map[string]any{"agent": task.agent},
				},
			})
		}
		return append(steps, textStep(scenario.closing))
	}
	return steps
}

func mockTurns() []map[string]any {
	turns := []map[string]any{}
	for _, scenario := range liveScenarios {
		turns = append(turns, map[string]any{
			fieldName: scenario.key, fieldMatch: matchText(scenario.prompt), fieldSteps: scenarioTurnSteps(scenario),
		})
		for index, task := range scenario.tasks {
			turns = append(turns, map[string]any{
				fieldName: scenario.key + "-task-" + string(rune('a'+index)), fieldMatch: matchText(task.task),
				fieldSteps: []map[string]any{textStep("Working through: " + task.title + "."), holdStep()},
			})
		}
	}
	reply := func(name string, match map[string]any, text string) map[string]any {
		return map[string]any{fieldName: name, fieldMatch: match, fieldSteps: []map[string]any{textStep(text)}}
	}
	return append(turns,
		reply("check-in", matchText("NOTHING_TO_REPORT"), "NOTHING_TO_REPORT"),
		reply("any-user", map[string]any{"turn_source": "user"}, "On it."),
		reply("any-wake", map[string]any{"turn_source": "synthetic"}, "Noted."),
	)
}

func configOption(id, name, current string, values ...string) map[string]any {
	choices := make([]map[string]any, 0, len(values))
	for _, value := range values {
		choices = append(choices, map[string]any{"value": value, "label": value})
	}
	return map[string]any{"id": id, fieldName: name, "current": current, "values": choices}
}

func mockAgent(name, provider, model string) map[string]any {
	return map[string]any{
		fieldName: name, "provider": provider, "permissions": "approve-reads",
		"config_options": []map[string]any{
			configOption("model", "Model", model, mockClaudeModel, mockOpusModel, mockCodexModel),
			configOption("mode", "Mode", "default", "default", "plan"),
			configOption("reasoning_effort", "Effort", "medium", "low", "medium", "high", "xhigh"),
		},
		"turns": mockTurns(),
	}
}

func mockFixture() ([]byte, error) {
	return json.MarshalIndent(map[string]any{
		"version": 2,
		"agents": []map[string]any{
			mockAgent(mockClaudeAgent, "claude", mockClaudeModel),
			mockAgent(mockCodexAgent, "codex", mockCodexModel),
		},
	}, "", "  ")
}
