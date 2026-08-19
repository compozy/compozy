package demoseed

import "time"

func scenarioAgents() []agentStory {
	return []agentStory{
		{
			Name:        productLeadAgent,
			Provider:    providerClaude,
			Permissions: approveReads,
			Tools:       []string{toolNetworkSend, toolTaskList, toolTaskRead},
			Prompt: "Own checkout launch decisions for Northstar Pay. Reconcile engineering, compliance, " +
				"support, and rollout evidence into short recommendations with explicit holds and rollback thresholds.",
		},
		{
			Name:        "platform-engineer",
			Provider:    providerCodex,
			Permissions: approveReads,
			Tools:       []string{toolNetworkSend, toolTaskRead},
			Prompt: "Investigate payment-platform reliability with evidence. Separate market-specific findings, " +
				"state data gaps plainly, and never promote a market without replay coverage.",
		},
		{
			Name:        "compliance-review",
			Provider:    providerClaude,
			Permissions: approveReads,
			Tools:       []string{toolNetworkSend, toolTaskRead},
			Prompt: "Review customer-facing payment language for Brazil and Mexico. Return approved wording, " +
				"required qualifiers, and any market-specific hold in a form the launch team can apply directly.",
		},
		{
			Name:        "support-lead",
			Provider:    providerClaude,
			Permissions: approveReads,
			Tools:       []string{toolNetworkSend, toolTaskRead},
			Prompt: "Keep checkout support operationally ready. Report only live queue counts, escalation coverage, " +
				"and the customer macros that are ready for the selected launch market.",
		},
		{
			Name:        "checkout-engineer",
			Provider:    providerCodex,
			Permissions: approveReads,
			Tools:       []string{toolNetworkSend, toolTaskList, toolTaskRead},
			Prompt: "Prepare checkout rollout and rollback controls. Keep changes inside the approved market, " +
				"canary percentage, and error-budget threshold.",
		},
	}
}

func scenarioSessions(now time.Time) []sessionStory {
	return []sessionStory{
		{
			ID:        scenarioSessionIDs[0],
			Name:      "Partner replay analysis",
			AgentName: "platform-engineer",
			Provider:  providerCodex,
			StartedAt: now.Add(-21*time.Hour - 18*time.Minute),
			EndedAt:   now.Add(-21*time.Hour - 7*time.Minute),
			Prompt: "Investigate duplicate authorization replays from the MercadoX connector. " +
				"Give me coverage by market and a concrete release implication.",
			Opening:   "I’ll reconcile the replay window against authorization and settlement records for each market.",
			ToolName:  toolTaskRead,
			ToolKind:  toolKindRead,
			ToolInput: `{"task_id":"task_northstar_partner_replay"}`,
			ToolResult: "Brazil: 10,214 replayed events reconciled with no duplicate charge or settlement drift. " +
				"Mexico: the partner export is missing 22 minutes between 14:08 and 14:30 UTC.",
			Conclusion: "Brazil replay coverage is clean and supports a controlled canary. Hold Mexico promotion " +
				"until MercadoX supplies the missing 22-minute export and the same reconciliation passes.",
			Input: 1840, Output: 612, CostUSD: 0.084,
		},
		{
			ID:        scenarioSessionIDs[1],
			Name:      "Brazil fallback copy review",
			AgentName: "compliance-review",
			Provider:  providerClaude,
			StartedAt: now.Add(-13*time.Hour - 34*time.Minute),
			EndedAt:   now.Add(-13*time.Hour - 26*time.Minute),
			Prompt: "Review the Brazil checkout fallback message for the canary. Keep it accurate, short, " +
				"and clear about whether a charge occurred.",
			Opening:   "I’ll verify the message against the authorization state and the support handoff.",
			ToolName:  toolTaskRead,
			ToolKind:  toolKindRead,
			ToolInput: `{"task_id":"task_northstar_compliance_copy"}`,
			ToolResult: "The fallback appears only after an authorization timeout, before settlement. " +
				"The retry action preserves the cart and payment method selection.",
			Conclusion: "Approved for Brazil: “Pagamento não concluído. Nenhuma cobrança foi feita. " +
				"Tente novamente em instantes.” Keep Mexico copy inactive while its replay evidence is incomplete.",
			Input: 1260, Output: 388, CostUSD: 0.057,
		},
		{
			ID:        scenarioSessionIDs[2],
			Name:      "Support readiness handoff",
			AgentName: "support-lead",
			Provider:  providerClaude,
			StartedAt: now.Add(-6*time.Hour - 11*time.Minute),
			EndedAt:   now.Add(-6*time.Hour - 1*time.Minute),
			Prompt: "Close the support readiness check for a Brazil canary and call out anything that " +
				"would block the launch decision.",
			Opening:   "I’ll check the launch-tagged queue, escalation coverage, and the approved fallback macro.",
			ToolName:  toolTaskRead,
			ToolKind:  toolKindRead,
			ToolInput: `{"task_id":"task_northstar_support_handoff"}`,
			ToolResult: "Launch-tagged conversations fell from 27 to 4. Portuguese macro is published. " +
				"Payments on-call and the MercadoX escalation contact cover the first 90 minutes.",
			Conclusion: "Support is ready for the Brazil canary with four open conversations and named escalation " +
				"coverage. The Mexico macro is drafted but remains inactive with the market hold.",
			Input: 1515, Output: 472, CostUSD: 0.068,
		},
		{
			ID:        scenarioSessionIDs[3],
			Name:      "Checkout launch recommendation",
			AgentName: productLeadAgent,
			Provider:  providerClaude,
			StartedAt: now.Add(-54 * time.Minute),
			EndedAt:   now.Add(-41 * time.Minute),
			Prompt: "Give me the final checkout launch recommendation. Reconcile partner replay, compliance, " +
				"support readiness, rollout size, and the operator decision still required.",
			Opening:   "I’ll consolidate the completed evidence and separate the Brazil decision from the Mexico hold.",
			ToolName:  toolTaskList,
			ToolKind:  toolKindRead,
			ToolInput: `{"workspace_id":"$WORKSPACE_ID","status":["completed","blocked","ready"]}`,
			ToolResult: "Completed: Brazil replay, fallback copy, support handoff. Ready: rollback monitor. " +
				"Blocked: Brazil operator approval; Mexico partner replay gap.",
			Conclusion: "Recommend Brazil go at a 25% canary after operator approval. Roll back if authorization " +
				"errors exceed 1.0% for five minutes. Hold Mexico until the 22-minute partner replay gap closes. " +
				"Support has four launch-tagged conversations open.",
			Input: 2360, Output: 705, CostUSD: 0.109,
		},
	}
}

func scenarioTasks(now time.Time) []taskStory {
	tasks := partnerScenarioTasks(now)
	tasks = append(tasks, readinessScenarioTasks(now)...)
	return append(tasks, openScenarioTasks(now)...)
}

func partnerScenarioTasks(now time.Time) []taskStory {
	return []taskStory{
		{
			ID:             "task_northstar_partner_replay",
			Identifier:     "LAUNCH-101",
			Title:          "Reconcile Brazil partner replay",
			Description:    "Verify MercadoX replay coverage before rollout.",
			Priority:       taskPriorityUrgent,
			Status:         taskStatusCompleted,
			ApprovalPolicy: approvalPolicyNone,
			ApprovalState:  approvalNotRequired,
			OwnerKind:      ownerAgentSession,
			OwnerRef:       scenarioSessionIDs[0],
			SessionID:      scenarioSessionIDs[0],
			RunID:          "run_northstar_partner_replay",
			CreatedAt:      now.Add(-25 * time.Hour),
			UpdatedAt:      now.Add(-21*time.Hour - 7*time.Minute),
			ClosedAt:       now.Add(-21*time.Hour - 7*time.Minute),
			TokensUsed:     2452,
			Result:         `{"market":"BR","replayed_events":10214,"duplicate_charges":0,"decision":"clear_for_canary"}`,
		},
		{
			ID:             "task_northstar_compliance_copy",
			Identifier:     "LAUNCH-102",
			Title:          "Approve Brazil fallback language",
			Description:    "Confirm the timeout message is accurate before the canary.",
			Priority:       taskPriorityHigh,
			Status:         taskStatusCompleted,
			ApprovalPolicy: approvalPolicyNone,
			ApprovalState:  approvalNotRequired,
			OwnerKind:      ownerAgentSession,
			OwnerRef:       scenarioSessionIDs[1],
			SessionID:      scenarioSessionIDs[1],
			RunID:          "run_northstar_compliance_copy",
			CreatedAt:      now.Add(-19 * time.Hour),
			UpdatedAt:      now.Add(-13*time.Hour - 26*time.Minute),
			ClosedAt:       now.Add(-13*time.Hour - 26*time.Minute),
			TokensUsed:     1648,
			Result:         `{"market":"BR","copy_status":"approved","charge_claim":"no_charge_created"}`,
		},
	}
}

func readinessScenarioTasks(now time.Time) []taskStory {
	return []taskStory{
		{
			ID:             taskSupportID,
			Identifier:     "LAUNCH-103",
			Title:          "Complete Brazil support handoff",
			Description:    "Publish the Portuguese macro and confirm launch escalation coverage.",
			Priority:       taskPriorityHigh,
			Status:         taskStatusCompleted,
			ApprovalPolicy: approvalPolicyNone,
			ApprovalState:  approvalNotRequired,
			OwnerKind:      ownerAgentSession,
			OwnerRef:       scenarioSessionIDs[2],
			SessionID:      scenarioSessionIDs[2],
			RunID:          "run_northstar_support_handoff",
			CreatedAt:      now.Add(-12 * time.Hour),
			UpdatedAt:      now.Add(-6*time.Hour - time.Minute),
			ClosedAt:       now.Add(-6*time.Hour - time.Minute),
			TokensUsed:     1987,
			Result:         `{"market":"BR","open_conversations":4,"macro":"published","coverage_minutes":90}`,
		},
		{
			ID:             taskLaunchDecisionID,
			Identifier:     "LAUNCH-104",
			Title:          "Produce checkout launch recommendation",
			Description:    "Synthesize replay, compliance, support, and rollout evidence.",
			Priority:       taskPriorityUrgent,
			Status:         taskStatusCompleted,
			ApprovalPolicy: approvalPolicyNone,
			ApprovalState:  approvalNotRequired,
			OwnerKind:      ownerAgentSession,
			OwnerRef:       scenarioSessionIDs[3],
			SessionID:      scenarioSessionIDs[3],
			RunID:          "run_northstar_launch_decision",
			CreatedAt:      now.Add(-5 * time.Hour),
			UpdatedAt:      now.Add(-41 * time.Minute),
			ClosedAt:       now.Add(-41 * time.Minute),
			TokensUsed:     3065,
			Result:         `{"br":"go_25_percent_after_approval","mx":"hold","rollback_auth_error_percent":1.0}`,
			DependencyTaskIDs: []string{
				"task_northstar_partner_replay",
				"task_northstar_compliance_copy",
				taskSupportID,
			},
		},
	}
}

func openScenarioTasks(now time.Time) []taskStory {
	return []taskStory{
		{
			ID: "task_northstar_authorize_canary", Identifier: "LAUNCH-105",
			Title: "Authorize Brazil 25% canary", Description: "Operator approval required before traffic promotion.",
			Priority: taskPriorityUrgent, Status: "blocked", ApprovalPolicy: "manual", ApprovalState: "pending",
			OwnerKind: "human", OwnerRef: operatorRef,
			CreatedAt: now.Add(-39 * time.Minute), UpdatedAt: now.Add(-39 * time.Minute),
			DependencyTaskIDs: []string{taskLaunchDecisionID},
		},
		{
			ID:             "task_northstar_mexico_replay",
			Identifier:     "LAUNCH-106",
			Title:          "Close Mexico partner replay gap",
			Description:    "Reconcile the missing MercadoX export before Mexico promotion.",
			Priority:       taskPriorityHigh,
			Status:         "blocked",
			ApprovalPolicy: approvalPolicyNone,
			ApprovalState:  approvalNotRequired,
			OwnerKind:      "pool",
			OwnerRef:       "platform",
			CreatedAt:      now.Add(-20 * time.Hour),
			UpdatedAt:      now.Add(-35 * time.Minute),
			BlockReason:    "MercadoX export is missing the 14:08–14:30 UTC replay window.",
		},
		{
			ID: "task_northstar_rollback_monitor", Identifier: "LAUNCH-107",
			Title:       "Pre-stage Brazil rollback monitor",
			Description: "Alert when authorization errors exceed 1.0% for five minutes.",
			Priority:    taskPriorityHigh, Status: "ready",
			ApprovalPolicy: approvalPolicyNone, ApprovalState: approvalNotRequired,
			OwnerKind: "pool", OwnerRef: "checkout",
			CreatedAt: now.Add(-32 * time.Minute), UpdatedAt: now.Add(-32 * time.Minute),
			DependencyTaskIDs: []string{taskLaunchDecisionID},
		},
	}
}

func scenarioNetworkMessages(now time.Time) []networkMessageStory {
	return []networkMessageStory{
		{
			ID: networkRootMessageID, SessionID: scenarioSessionIDs[3], At: now.Add(-38 * time.Minute),
			Text: "Decision frame: Brazil at a 25% canary after operator approval; " +
				"Mexico held pending complete partner replay.",
		},
		{
			ID: "msg_northstar_02", SessionID: scenarioSessionIDs[0], ReplyTo: networkRootMessageID,
			At: now.Add(
				-36 * time.Minute,
			), Text: "Brazil is clean across 10,214 replayed events with zero duplicate charges. " +
				"Mexico still lacks the 14:08–14:30 UTC export.",
		},
		{
			ID: "msg_northstar_03", SessionID: scenarioSessionIDs[1], ReplyTo: networkRootMessageID,
			At: now.Add(-34 * time.Minute),
			Text: "Brazil fallback language is approved and makes the no-charge state explicit. " +
				"Mexico copy remains inactive with the hold.",
		},
		{
			ID: "msg_northstar_04", SessionID: scenarioSessionIDs[2], ReplyTo: networkRootMessageID,
			At: now.Add(-31 * time.Minute), Text: "Support is ready: four launch-tagged conversations remain, " +
				"the Portuguese macro is published, and escalation coverage is staffed for 90 minutes.",
		},
		{
			ID: "msg_northstar_05", SessionID: scenarioSessionIDs[3], ReplyTo: networkRootMessageID,
			At: now.Add(-27 * time.Minute), Text: "Recommendation locked: approve Brazil at 25%, " +
				"roll back above 1.0% authorization errors for five minutes, and keep Mexico on hold.",
		},
	}
}

const launchLoopDefinition = `apiVersion: compozy.loop/v1
kind: Loop
meta:
  name: launch-readiness
  description: Consolidate market evidence into a bounded checkout rollout decision.
  catalog:
    use_when: "A checkout market needs a go, hold, or rollback recommendation."
    keywords: [checkout, launch, readiness, payments]
    category: Operations
concurrency: forbid
inputs:
  market:
    type: string
    required: true
contract:
  goal: "Return a rollout decision for {{ .inputs.market }} from current launch evidence."
  definition_of_done: "The market has an explicit decision, rollout size, and rollback threshold."
  iteration_cap: 1
  no_progress: { window: 1 }
  budget: { tokens: 4000, wall_clock_sec: 300, on_exceeded: halt }
  terminal_states: [done, blocked, failed]
graph:
  nodes:
    - id: market
      class: source
      kind: input
      input_ref: market
    - id: decision
      class: action
      kind: transform
      params:
        map:
          market: { value: "{{ .inputs.market }}" }
          recommendation: { value: "25% canary after operator approval" }
          rollback_threshold: { value: "authorization errors above 1.0% for five minutes" }
  edges:
    - from: market
      to: decision
start:
  - kind: manual
  - kind: cli
  - kind: http
  - kind: uds
  - kind: native_tool
`
