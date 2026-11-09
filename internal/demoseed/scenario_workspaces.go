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
	agents := append(launchAgents(), platformAgents()...)
	return append(agents, alwaysOnAgents()...)
}

func launchAgents() []agentStory {
	return []agentStory{
		{
			Name:         agentProductLead,
			Description:  "Turns engineering, compliance, and support evidence into launch recommendations.",
			WorkspaceKey: workspaceKeyLaunch,
			Provider:     providerClaude,
			Model:        modelClaude,
			Permissions:  approveReads,
			Tools:        []string{toolTaskList, toolTaskRead, toolLoopStatus},
			CategoryPath: []string{categoryOperations, "Launch"},
			Prompt: "Own checkout launch decisions for Northstar Pay. Reconcile engineering, compliance, " +
				"support, and rollout evidence into short recommendations with explicit holds and rollback thresholds.",
		},
		{
			Name: agentComplianceReview, Description: "Reviews customer-facing payment copy for Brazil and Mexico.",
			WorkspaceKey: workspaceKeyLaunch,
			Provider:     providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolTaskRead},
			CategoryPath: []string{categoryRisk, "Compliance"},
			Prompt: "Review customer-facing payment language for Brazil and Mexico. Return approved wording, " +
				"required qualifiers, and any market-specific hold in a form the launch team can apply directly.",
		},
		{
			Name: agentSupportLead, Description: "Keeps support queues, escalations, and launch macros ready.",
			WorkspaceKey: workspaceKeyLaunch,
			Provider:     providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolTaskRead, toolTaskUpdate},
			CategoryPath: []string{"Support"},
			Prompt: "Keep checkout support operationally ready. Report only live queue counts, escalation coverage, " +
				"and the customer macros that are ready for the selected launch market.",
		},
		{
			Name: agentFraudAnalyst, Description: "Separates confirmed fraud from authorization noise in dispute data.",
			WorkspaceKey: workspaceKeyLaunch,
			Provider:     providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolTaskRead},
			CategoryPath: []string{categoryRisk, "Fraud"},
			Prompt: "Score chargeback and dispute signals for the checkout rollout. Separate confirmed fraud from " +
				"authorization noise, and never raise a market risk level without naming the evidence window.",
		},
	}
}

func platformAgents() []agentStory {
	return []agentStory{
		{
			Name: agentPlatformEngineer, Description: "Investigates payment-platform reliability with replay evidence.",
			WorkspaceKey: workspaceKeyPlatform,
			Provider:     providerCodex, Model: modelCodex, Permissions: approveReads,
			Tools:        []string{toolTaskRead, toolTaskUpdate, toolSubagentDelegate, toolMonitorCreate},
			CategoryPath: []string{categoryEngineering, "Payments"},
			Prompt: "Investigate payment-platform reliability with evidence. Separate market-specific findings, " +
				"state data gaps plainly, and never promote a market without replay coverage.",
		},
		{
			Name:         agentCheckoutEngineer,
			Description:  "Ships checkout rollout and rollback changes inside the approved canary.",
			WorkspaceKey: workspaceKeyPlatform,
			Provider:     providerCodex,
			Model:        modelCodex,
			Permissions:  approveAll,
			Tools: []string{
				toolTaskList, toolTaskRead, toolTaskUpdate, toolSubagentDelegate, toolMonitorCreate,
			},
			CategoryPath: []string{categoryEngineering, "Checkout"},
			Prompt: "Prepare checkout rollout and rollback controls. Keep changes inside the approved market, " +
				"canary percentage, and error-budget threshold.",
		},
		{
			Name: agentReleaseManager, Description: "Drives the payments release train and its rollback paths.",
			WorkspaceKey: workspaceKeyPlatform,
			Provider:     providerCodex, Model: modelCodex, Permissions: approveReads,
			Tools:        []string{toolTaskList, toolTaskRead, toolLoopStatus},
			CategoryPath: []string{categoryEngineering, "Release"},
			Prompt: "Drive the payments release train. Confirm every candidate change has a rollback path and a " +
				"named owner before it enters a release window.",
		},
		{
			Name: agentDocsSteward, Description: "Keeps the payments runbooks true to the shipped system.",
			WorkspaceKey: workspaceKeyPlatform,
			Provider:     providerClaude, Model: modelClaude, Permissions: approveReads,
			Tools:        []string{toolTaskRead},
			CategoryPath: []string{categoryOperations, "Documentation"},
			Prompt: "Keep the payments runbooks true to the shipped system. Flag any page whose steps no longer " +
				"match the current rollout controls instead of rewriting it silently.",
		},
	}
}
