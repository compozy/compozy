package demoseed

import "github.com/compozy/compozy/internal/session"

func scenarioSessions(clock timeline) []sessionStory {
	return append(launchSessions(clock), platformSessions(clock)...)
}

func launchSessions(clock timeline) []sessionStory {
	return []sessionStory{
		launchDecisionSession(clock),
		complianceCopySession(clock),
		supportHandoffSession(clock),
		fraudSweepSession(clock),
		chargebackWatchSession(clock),
		incidentReviewSession(clock),
	}
}

func launchDecisionSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[3], Name: "Checkout launch recommendation",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentProductLead,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.minutesAgo(54), EndedAt: clock.minutesAgo(41),
		StopDetail: "Recommendation delivered and handed to the operator.",
		Input:      2360, Output: 705, CostUSD: 0.109,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Give me the final checkout launch recommendation. Reconcile partner replay, " +
				"compliance, support readiness, rollout size, and the operator decision still required."},
			{Kind: stepThinking, Text: "Four evidence streams have to agree before I can name a percentage. " +
				"Brazil and Mexico are not the same decision, so I should not average them."},
			{Kind: stepAgent, Text: "I'll consolidate the completed evidence and separate the Brazil decision " +
				"from the Mexico hold."},
			{
				Kind: stepTool, ToolName: toolTaskList, ToolKind: toolKindRead,
				ToolInput: `{"workspace_id":"$WORKSPACE_ID","status":["completed","blocked","ready"]}`,
				ToolResult: `{"content":"Completed: Brazil replay, fallback copy, support handoff. ` +
					`Ready: rollback monitor. Blocked: Brazil operator approval; Mexico partner replay gap."}`,
			},
			{
				Kind: stepTool, ToolName: toolGrep, ToolKind: toolKindSearch,
				ToolInput: `{"pattern":"rollback_threshold","glob":"*.yaml","path":"config/checkout"}`,
				ToolResult: `{"stdout":"config/checkout/rollout.yaml\nconfig/checkout/alerts.yaml\n` +
					`config/checkout/canary.yaml"}`,
			},
			{
				Kind: stepTool, ToolName: toolTodoWrite, ToolKind: toolKindOther,
				ToolInput: `{"todos":[` +
					`{"content":"Confirm Brazil replay coverage","status":"completed"},` +
					`{"content":"Confirm approved fallback copy","status":"completed"},` +
					`{"content":"Confirm support escalation window","status":"completed"},` +
					`{"content":"Request operator approval for 25% canary","status":"in_progress"},` +
					`{"content":"Close Mexico replay gap","status":"pending"}]}`,
				ToolResult: `{"content":"5 todos tracked, 3 completed."}`,
			},
			{Kind: stepAgent, Text: "Recommend Brazil go at a 25% canary after operator approval. Roll back if " +
				"authorization errors exceed 1.0% for five minutes. Hold Mexico until the 22-minute partner replay " +
				"gap closes. Support has four launch-tagged conversations open."},
		},
	}
}

func complianceCopySession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[1], Name: "Brazil fallback copy review",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentComplianceReview,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(13, 34), EndedAt: clock.hoursMinutesAgo(13, 26),
		StopDetail: "Copy approved for Brazil; Mexico left inactive.",
		Input:      1260, Output: 388, CostUSD: 0.057,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Review the Brazil checkout fallback message for the canary. Keep it accurate, " +
				"short, and clear about whether a charge occurred."},
			{Kind: stepAgent, Text: "I'll verify the message against the authorization state and the support handoff."},
			{
				Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
				ToolInput: `{"file_path":"content/checkout/pt-BR/fallback.md"}`,
				ToolResult: `{"content":"# Fallback\n\nPagamento não concluído.\nTente novamente.\n\n` +
					"<!-- charge state unstated -->\"}",
			},
			{Kind: stepThinking, Text: "The current wording never says whether money moved. That is the one " +
				"question a customer has at this moment, so it belongs in the second sentence."},
			{
				Kind: stepTool, ToolName: toolEdit, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"content/checkout/pt-BR/fallback.md",` +
					`"old_string":"Pagamento não concluído.\nTente novamente.",` +
					`"new_string":"Pagamento não concluído. Nenhuma cobrança foi feita.\n` +
					`Tente novamente em instantes."}`,
				ToolResult: `{"file_path":"content/checkout/pt-BR/fallback.md","content":"1 edit applied"}`,
			},
			{Kind: stepAgent, Text: "Approved for Brazil: “Pagamento não concluído. Nenhuma cobrança foi feita. " +
				"Tente novamente em instantes.” Keep Mexico copy inactive while its replay evidence is incomplete."},
		},
	}
}

func supportHandoffSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[2], Name: "Support readiness handoff",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentSupportLead,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.hoursMinutesAgo(6, 11), EndedAt: clock.hoursMinutesAgo(6, 1),
		StopDetail: "Support readiness confirmed for the Brazil canary.",
		Input:      1515, Output: 472, CostUSD: 0.068,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Close the support readiness check for a Brazil canary and call out anything " +
				"that would block the launch decision."},
			{Kind: stepAgent, Text: "I'll check the launch-tagged queue, escalation coverage, and the approved " +
				"fallback macro."},
			{
				Kind: stepTool, ToolName: toolTaskRead, ToolKind: toolKindRead,
				ToolInput: `{"task_id":"` + taskSupportID + `"}`,
				ToolResult: `{"content":"Launch-tagged conversations fell from 27 to 4. Portuguese macro is ` +
					`published. Payments on-call and the MercadoX escalation contact cover the first 90 minutes."}`,
			},
			{
				Kind: stepTool, ToolName: toolTodoWrite, ToolKind: toolKindOther,
				ToolInput: `{"todos":[` +
					`{"content":"Publish pt-BR fallback macro","status":"completed"},` +
					`{"content":"Staff payments on-call for 90 minutes","status":"completed"},` +
					`{"content":"Keep es-MX macro drafted and inactive","status":"in_progress"}]}`,
				ToolResult: `{"content":"3 todos tracked, 2 completed."}`,
			},
			{Kind: stepAgent, Text: "Support is ready for the Brazil canary with four open conversations and named " +
				"escalation coverage. The Mexico macro is drafted but remains inactive with the market hold."},
		},
	}
}

func fraudSweepSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[4], Name: "Chargeback signal sweep",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentFraudAnalyst,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(2, 3), EndedAt: clock.daysHoursAgo(2, 2),
		StopDetail: "Risk level unchanged for Brazil.",
		Input:      1980, Output: 664, CostUSD: 0.091,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Sweep the last 30 days of chargeback signals for Brazil and tell me whether " +
				"the canary raises the market risk level."},
			{
				Kind: stepTool, ToolName: toolGrep, ToolKind: toolKindSearch,
				ToolInput:  `{"pattern":"dispute_opened","glob":"*.jsonl","path":"data/risk/br"}`,
				ToolResult: `{"stdout":"data/risk/br/2026-07.jsonl\ndata/risk/br/2026-08.jsonl"}`,
			},
			{Kind: stepThinking, Text: "Two files means the window spans a month boundary. I need the rate per " +
				"1,000 authorizations, not the raw count, or the July partial month will look like a spike."},
			{
				Kind: stepTool, ToolName: toolBash, ToolKind: toolKindExecute,
				ToolInput:  `{"command":"jq -r '.reason' data/risk/br/*.jsonl | sort | uniq -c | sort -rn"}`,
				ToolResult: `{"stdout":"  41 product_not_received\n  18 fraudulent\n   9 duplicate\n   4 other"}`,
			},
			{Kind: stepAgent, Text: "Brazil disputes hold at 0.42 per 1,000 authorizations across the window, " +
				"inside the 0.60 threshold. Fraudulent claims are 25% of disputes and flat month over month. " +
				"The canary does not raise the market risk level."},
		},
	}
}

func chargebackWatchSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[10], Name: "Chargeback triage coordinator",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentFraudAnalyst,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeCoordinator),
		StartedAt: clock.hoursAgo(9), EndedAt: clock.hoursMinutesAgo(8, 42),
		StopDetail: "Parked the triage Loop on its declared settlement event.",
		Input:      840, Output: 262, CostUSD: 0.038,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Start the chargeback triage Loop and leave it parked until MercadoX publishes " +
				"the next settlement batch."},
			{
				Kind: stepTool, ToolName: toolNetworkSend, ToolKind: toolKindOther,
				ToolInput: `{"channel":"` + launchChannel + `","text":"chargeback triage parked on ` +
					`settlement.batch.published"}`,
				ToolResult: `{"content":"delivered to 4 members"}`,
			},
			{Kind: stepAgent, Text: "The triage Loop is parked on settlement.batch.published and costs nothing " +
				"while it waits. It wakes on the declared event only."},
		},
	}
}

func incidentReviewSession(clock timeline) sessionStory {
	return sessionStory{
		ID: scenarioSessionIDs[11], Name: "Authorization dip postmortem",
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentProductLead,
		Provider: providerClaude, Model: modelClaude, SessionType: string(session.SessionTypeUser),
		StartedAt: clock.daysHoursAgo(4, 5), EndedAt: clock.daysHoursAgo(4, 2),
		StopDetail: "Postmortem published with two owned follow-ups.",
		Input:      6120, Output: 2140, CostUSD: 0.286,
		Steps: []transcriptStep{
			{Kind: stepUser, Text: "Write the postmortem for the 12-minute authorization dip on 2026-08-14 and " +
				"give every follow-up a named owner."},
			{Kind: stepThinking, Text: "A postmortem without owners is a summary. The connector timeout and the " +
				"missing alert are separate causes and should not share one action item."},
			{
				Kind: stepTool, ToolName: toolRead, ToolKind: toolKindRead,
				ToolInput: `{"file_path":"incidents/2026-08-14-auth-dip/timeline.md"}`,
				ToolResult: `{"content":"14:02 connector latency crosses 900ms\n14:05 authorization errors reach ` +
					"1.4%\\n14:09 on-call paged manually\\n14:14 connector pool resized\\n14:17 errors back under 0.3%\"}",
			},
			{
				Kind: stepTool, ToolName: toolWrite, ToolKind: toolKindEdit,
				ToolInput: `{"file_path":"incidents/2026-08-14-auth-dip/postmortem.md",` +
					`"content":"# Authorization dip, 2026-08-14\n\n## Impact\n12 minutes above the 1.0% ` +
					`authorization error threshold.\n\n## Causes\n1. Connector pool sized for pre-launch volume.\n` +
					`2. No alert fired; the page was manual.\n\n## Follow-ups\n- Resize pool from traffic ` +
					`forecast — platform-engineer\n- Wire the 1.0%/5m alert — checkout-engineer\n"}`,
				ToolResult: `{"file_path":"incidents/2026-08-14-auth-dip/postmortem.md","content":"written"}`,
			},
			{Kind: stepAgent, Text: "Postmortem published. Twelve minutes above threshold, two distinct causes, " +
				"and two follow-ups with named owners. Detection was the slower half: the page was manual because " +
				"no alert existed for the 1.0% over five minutes rule."},
		},
	}
}
