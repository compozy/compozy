package demoseed

func scenarioNetworkMessages(clock timeline) []networkMessageStory {
	return []networkMessageStory{
		{
			ID: networkRootMessageID, SessionID: sessionLaunchDecisionID, At: clock.minutesAgo(38),
			Text: "Decision frame: Brazil at a 25% canary after operator approval; " +
				"Mexico held pending complete partner replay.",
		},
		{
			ID: "msg_northstar_02", SessionID: sessionLaunchDecisionID, ReplyTo: networkRootMessageID,
			At: clock.minutesAgo(36),
			Text: "Relaying the platform replay audit: Brazil is clean across 10,214 replayed events " +
				"with zero duplicate charges. Mexico still lacks the 14:08–14:30 UTC export.",
		},
		{
			ID: "msg_northstar_03", SessionID: sessionComplianceCopyID, ReplyTo: networkRootMessageID,
			At: clock.minutesAgo(34),
			Text: "Brazil fallback language is approved and makes the no-charge state explicit. " +
				"Mexico copy remains inactive with the hold.",
		},
		{
			ID: "msg_northstar_04", SessionID: sessionSupportHandoffID, ReplyTo: networkRootMessageID,
			At: clock.minutesAgo(31),
			Text: "Support is ready: four launch-tagged conversations remain, " +
				"the Portuguese macro is published, and escalation coverage is staffed for 90 minutes.",
		},
		{
			ID: "msg_northstar_05", SessionID: sessionFraudSweepID, ReplyTo: networkRootMessageID,
			At: clock.minutesAgo(29),
			Text: "Dispute rate holds at 0.42 per 1,000 authorizations, inside the 0.60 threshold. " +
				"No risk-level change from the canary.",
		},
		{
			ID: "msg_northstar_06", SessionID: sessionLaunchDecisionID, ReplyTo: networkRootMessageID,
			At: clock.minutesAgo(27),
			Text: "Recommendation locked: approve Brazil at 25%, " +
				"roll back above 1.0% authorization errors for five minutes, and keep Mexico on hold.",
		},
	}
}
