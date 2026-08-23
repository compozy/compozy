package demoseed

const (
	memoryScopeGlobal    = "global"
	memoryScopeWorkspace = "workspace"
	memoryScopeAgent     = "agent"
	memoryTypeUser       = "user"
	memoryTypeFeedback   = "feedback"
	memoryTypeProject    = "project"
	memoryTypeReference  = "reference"
)

func scenarioMemories(clock timeline) []memoryStory {
	return append(globalMemories(clock), scopedMemories(clock)...)
}

func globalMemories(clock timeline) []memoryStory {
	return []memoryStory{
		{
			Name: "operator-decision-style", Scope: memoryScopeGlobal, Type: memoryTypeUser,
			Description: "Pedro wants a recommendation with the threshold attached, not a menu of options.",
			Body: "Lead with the decision and the number that would reverse it. A recommendation without a " +
				"rollback threshold reads as an opinion.\n\n**Why:** every launch review has ended with the same " +
				"follow-up question.\n**How to apply:** name the metric, the limit, and the window in the first " +
				"two sentences.",
			CreatedAt: clock.daysAgo(28), UpdatedAt: clock.daysAgo(6),
		},
		{
			Name: "market-hold-language", Scope: memoryScopeGlobal, Type: memoryTypeFeedback,
			Description: "Never merge two markets into one go/no-go sentence.",
			Body: "Brazil and Mexico are separate decisions with separate evidence. Reporting them together " +
				"has twice produced a hold being read as an approval.\n\n**Why:** the 2026-07 checkout review " +
				"shipped a Mexico change nobody approved.\n**How to apply:** one market per recommendation line.",
			CreatedAt: clock.daysAgo(21), UpdatedAt: clock.daysAgo(21),
		},
		{
			Name: "mercadox-partner-contacts", Scope: memoryScopeGlobal, Type: memoryTypeReference,
			Description: "Where the MercadoX replay exports and escalation path live.",
			Body: "Replay exports land in `data/partners/mercadox/` with a `manifest.json` per batch. " +
				"Escalation goes through the partner integrations rotation, not the payments on-call.",
			CreatedAt: clock.daysAgo(17), UpdatedAt: clock.daysAgo(9),
		},
	}
}

func scopedMemories(clock timeline) []memoryStory {
	return []memoryStory{
		{
			Name: "checkout-launch-state", Scope: memoryScopeWorkspace, WorkspaceKey: workspaceKeyLaunch,
			Type:        memoryTypeProject,
			Description: "Where the checkout rollout stands and what is still blocking it.",
			Body: "Brazil is cleared for a 25% canary pending operator approval. Mexico is held on a " +
				"22-minute MercadoX export gap between 14:08 and 14:30 UTC.\n\nRollback threshold: " +
				"authorization errors above 1.0% for five minutes. Support carries four launch-tagged " +
				"conversations and 90 minutes of named escalation coverage.",
			CreatedAt: clock.daysAgo(3), UpdatedAt: clock.minutesAgo(41),
		},
		{
			Name: "settlement-retry-cause", Scope: memoryScopeWorkspace, WorkspaceKey: workspaceKeyPlatform,
			Type:        memoryTypeProject,
			Description: "Why settlement workers exhaust retries against MercadoX.",
			Body: "The partner endpoint returns 429 above 40 requests per second. Three workers exhaust " +
				"retries independently because each holds its own budget.\n\nThe first limiter attempt " +
				"regressed batch ordering, so ordering is a contract the fix has to preserve.",
			CreatedAt: clock.daysAgo(7), UpdatedAt: clock.hoursMinutesAgo(3, 12),
		},
		{
			Name: "dispute-scoring-window", Scope: memoryScopeAgent, WorkspaceKey: workspaceKeyLaunch,
			AgentName: agentFraudAnalyst, Type: memoryTypeProject,
			Description: "Dispute rates are only comparable per 1,000 authorizations.",
			Body: "Raw dispute counts across a month boundary make a partial month look like a spike. " +
				"Always normalize to disputes per 1,000 authorizations before naming a risk level.\n\n" +
				"Brazil threshold is 0.60; the current window sits at 0.42.",
			CreatedAt: clock.daysAgo(2), UpdatedAt: clock.daysAgo(2),
		},
		{
			Name: "runbook-freshness-rule", Scope: memoryScopeAgent, WorkspaceKey: workspaceKeyPlatform,
			AgentName: agentDocsSteward, Type: memoryTypeFeedback,
			Description: "Flag stale runbook steps instead of rewriting them.",
			Body: "A rewritten step that nobody verified is worse than a flagged one. Two runbooks are " +
				"currently flagged: `checkout-rollback.md` (10-minute wait against a 5-minute alert window) " +
				"and `mercadox-replay.md` (pre-checkpoint batch format).\n\n**Why:** an unverified rewrite " +
				"shipped a rollback step that did not work.\n**How to apply:** flag, name the owner, stop.",
			CreatedAt: clock.daysAgo(6), UpdatedAt: clock.daysHoursAgo(6, 3),
		},
	}
}
