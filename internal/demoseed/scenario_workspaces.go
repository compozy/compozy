package demoseed

const (
	workspaceKeyLaunch   = "launch-hq"
	workspaceKeyPlatform = "platform"
)

func scenarioWorkspaces(clock timeline) []workspaceStory {
	return []workspaceStory{
		{
			Key:          workspaceKeyLaunch,
			Name:         launchWorkspaceName,
			Relative:     launchWorkspaceRelative,
			DefaultAgent: agentProductLead,
			CreatedAt:    clock.daysAgo(34),
			UpdatedAt:    clock.minutesAgo(27),
		},
		{
			Key:          workspaceKeyPlatform,
			Name:         platformWorkspaceName,
			Relative:     platformWorkspaceRelat,
			DefaultAgent: agentPlatformEngineer,
			CreatedAt:    clock.daysAgo(31),
			UpdatedAt:    clock.minutesAgo(52),
		},
	}
}

func scenarioAgents() []agentStory {
	return append(launchAgents(), platformAgents()...)
}

func launchAgents() []agentStory {
	return []agentStory{
		{
			Name: agentProductLead, WorkspaceKey: workspaceKeyLaunch,
			Provider: providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolNetworkSend, toolTaskList, toolTaskRead, toolLoopStatus},
			CategoryPath: []string{categoryOperations, "Launch"},
			Prompt: "Own checkout launch decisions for Northstar Pay. Reconcile engineering, compliance, " +
				"support, and rollout evidence into short recommendations with explicit holds and rollback thresholds.",
		},
		{
			Name: agentComplianceReview, WorkspaceKey: workspaceKeyLaunch,
			Provider: providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolNetworkSend, toolTaskRead},
			CategoryPath: []string{categoryRisk, "Compliance"},
			Prompt: "Review customer-facing payment language for Brazil and Mexico. Return approved wording, " +
				"required qualifiers, and any market-specific hold in a form the launch team can apply directly.",
		},
		{
			Name: agentSupportLead, WorkspaceKey: workspaceKeyLaunch,
			Provider: providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolNetworkSend, toolTaskRead, toolTaskUpdate},
			CategoryPath: []string{"Support"},
			Prompt: "Keep checkout support operationally ready. Report only live queue counts, escalation coverage, " +
				"and the customer macros that are ready for the selected launch market.",
		},
		{
			Name: agentFraudAnalyst, WorkspaceKey: workspaceKeyLaunch,
			Provider: providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolTaskRead, toolMemoryStore},
			CategoryPath: []string{categoryRisk, "Fraud"},
			Prompt: "Score chargeback and dispute signals for the checkout rollout. Separate confirmed fraud from " +
				"authorization noise, and never raise a market risk level without naming the evidence window.",
		},
	}
}

func platformAgents() []agentStory {
	return []agentStory{
		{
			Name: agentPlatformEngineer, WorkspaceKey: workspaceKeyPlatform,
			Provider: providerCodex, Model: modelCodex, Permissions: approveReads,
			Tools:        []string{toolNetworkSend, toolTaskRead, toolTaskUpdate},
			CategoryPath: []string{categoryEngineering, "Payments"},
			Prompt: "Investigate payment-platform reliability with evidence. Separate market-specific findings, " +
				"state data gaps plainly, and never promote a market without replay coverage.",
		},
		{
			Name: agentCheckoutEngineer, WorkspaceKey: workspaceKeyPlatform,
			Provider: providerCodex, Model: modelCodex, Permissions: approveAll,
			Tools:        []string{toolNetworkSend, toolTaskList, toolTaskRead, toolTaskUpdate},
			CategoryPath: []string{categoryEngineering, "Checkout"},
			Prompt: "Prepare checkout rollout and rollback controls. Keep changes inside the approved market, " +
				"canary percentage, and error-budget threshold.",
		},
		{
			Name: agentReleaseManager, WorkspaceKey: workspaceKeyPlatform,
			Provider: providerCodex, Model: modelCodex, Permissions: approveReads,
			Tools:        []string{toolTaskList, toolTaskRead, toolLoopStatus},
			CategoryPath: []string{categoryEngineering, "Release"},
			Prompt: "Drive the payments release train. Confirm every candidate change has a rollback path and a " +
				"named owner before it enters a release window.",
		},
		{
			Name: agentDocsSteward, WorkspaceKey: workspaceKeyPlatform,
			Provider: providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolTaskRead, toolMemoryStore},
			CategoryPath: []string{categoryOperations, "Documentation"},
			Prompt: "Keep the payments runbooks true to the shipped system. Flag any page whose steps no longer " +
				"match the current rollout controls instead of rewriting it silently.",
		},
	}
}
