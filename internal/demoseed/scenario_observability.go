package demoseed

import (
	"fmt"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
)

const (
	tokenUsageWindowDays = 90
	pulseWindowDays      = 21
)

// tokenUsageAgents pairs each contributing agent with its workspace and a stable
// daily weight, so the 7/30/90-day windows all carry a curve without randomness.
var tokenUsageAgents = []struct {
	workspaceKey string
	agentName    string
	weight       int64
}{
	{workspaceKeyLaunch, agentProductLead, 9},
	{workspaceKeyLaunch, agentComplianceReview, 4},
	{workspaceKeyLaunch, agentSupportLead, 3},
	{workspaceKeyLaunch, agentFraudAnalyst, 5},
	{workspaceKeyPlatform, agentPlatformEngineer, 8},
	{workspaceKeyPlatform, agentCheckoutEngineer, 6},
	{workspaceKeyPlatform, agentReleaseManager, 2},
	{workspaceKeyPlatform, agentDocsSteward, 2},
}

// dailyShape spreads volume across a week so the usage chart and pulse heatmap
// both show weekday relief instead of a flat band.
var dailyShape = [7]int64{3, 11, 13, 12, 14, 9, 4}

func scenarioTokenUsage() []tokenUsageStory {
	stories := make([]tokenUsageStory, 0, tokenUsageWindowDays*len(tokenUsageAgents))
	for daysBack := range tokenUsageWindowDays {
		shape := dailyShape[daysBack%len(dailyShape)]
		for index, agent := range tokenUsageAgents {
			if (daysBack+index)%3 == 0 && daysBack > 6 {
				continue
			}
			input := agent.weight * shape * 140
			output := agent.weight * shape * 46
			stories = append(stories, tokenUsageStory{
				DaysBack: daysBack, WorkspaceKey: agent.workspaceKey, AgentName: agent.agentName,
				Input: input, Output: output,
				CostUSD: float64(input+output) * 0.0000045,
			})
		}
	}
	return stories
}

// pulseHours are the working hours the fixture concentrates activity in.
var pulseHours = [5]int{9, 11, 14, 16, 19}

// pulseActors cycle the sessions that own generated activity, so every summary
// resolves to a real session and inherits its workspace and provider.
var pulseActors = []struct {
	sessionID string
	agentName string
}{
	{scenarioSessionIDs[3], agentProductLead},
	{scenarioSessionIDs[0], agentPlatformEngineer},
	{scenarioSessionIDs[2], agentSupportLead},
	{scenarioSessionIDs[5], agentCheckoutEngineer},
	{scenarioSessionIDs[4], agentFraudAnalyst},
	{scenarioSessionIDs[9], agentDocsSteward},
	{scenarioSessionIDs[1], agentComplianceReview},
	{scenarioSessionIDs[8], agentReleaseManager},
}

func scenarioEventSummaries(clock timeline) []eventSummaryStory {
	stories := pulseEventSummaries(clock)
	return append(stories, todayEventSummaries(clock)...)
}

func pulseEventSummaries(clock timeline) []eventSummaryStory {
	stories := make([]eventSummaryStory, 0, pulseWindowDays*len(pulseHours))
	for daysBack := 1; daysBack <= pulseWindowDays; daysBack++ {
		shape := dailyShape[daysBack%len(dailyShape)]
		for index, hour := range pulseHours {
			if (daysBack+index)%4 == 0 {
				continue
			}
			at := clock.dayStart(daysBack).Add(time.Duration(hour) * time.Hour).UTC()
			actor := pulseActors[(daysBack+index)%len(pulseActors)]
			stories = append(stories, eventSummaryStory{
				ID:        fmt.Sprintf("sum_pulse_%02d_%02d", daysBack, hour),
				SessionID: actor.sessionID, AgentName: actor.agentName,
				Type: pulseEventType(index),
				Summary: fmt.Sprintf(
					"Checkout platform activity, %d events in the hour", shape+int64(index),
				),
				At: at,
			})
		}
	}
	return stories
}

func pulseEventType(index int) string {
	switch index % 3 {
	case 0:
		return eventspkg.TaskCreated
	case 1:
		return eventspkg.TaskRunCompleted
	default:
		return eventspkg.TaskUpdated
	}
}

func todayEventSummaries(clock timeline) []eventSummaryStory {
	stories := []eventSummaryStory{
		{
			ID: "sum_today_launch_decision", WorkspaceKey: workspaceKeyLaunch,
			SessionID: scenarioSessionIDs[3], AgentName: agentProductLead,
			Type: eventspkg.TaskRunCompleted, Summary: "Checkout launch recommendation completed",
			At: clock.minutesAgo(41),
		},
		{
			ID: "sum_today_settlement_failed", WorkspaceKey: workspaceKeyPlatform,
			SessionID: scenarioSessionIDs[7], AgentName: agentCheckoutEngineer,
			Type: eventspkg.TaskRunFailed, Summary: "Settlement rate limiter run failed on batch ordering",
			At: clock.hoursMinutesAgo(3, 12),
		},
		{
			ID: "sum_today_support_handoff", WorkspaceKey: workspaceKeyLaunch,
			SessionID: scenarioSessionIDs[2], AgentName: agentSupportLead,
			Type: eventspkg.SessionStopped, Summary: "Support readiness handoff session stopped",
			At: clock.hoursMinutesAgo(6, 1),
		},
		{
			ID: "sum_today_canary_task", SessionID: scenarioSessionIDs[3], AgentName: agentProductLead,
			Type: eventspkg.TaskCreated, Summary: "Authorize Brazil 25% canary created",
			At: clock.minutesAgo(39),
		},
	}
	return append(stories, hookDispatchSummaries(clock)...)
}

func hookDispatchSummaries(clock timeline) []eventSummaryStory {
	const guardrailsHook = "launch-guardrails"
	hooks := []struct {
		name    string
		outcome string
		offset  time.Duration
	}{
		{guardrailsHook, string(eventspkg.OutcomeSuccess), 8 * time.Hour},
		{guardrailsHook, string(eventspkg.OutcomeSuccess), 9*time.Hour + 30*time.Minute},
		{"settlement-watch", string(eventspkg.OutcomeSuccess), 11 * time.Hour},
		{"settlement-watch", string(eventspkg.OutcomeFailure), 12*time.Hour + 15*time.Minute},
		{"runbook-freshness", string(eventspkg.OutcomeSuccess), 14 * time.Hour},
		{guardrailsHook, string(eventspkg.OutcomeSuccess), 15*time.Hour + 45*time.Minute},
	}
	timestamps := make([]time.Time, len(hooks))
	for index, hook := range hooks {
		timestamps[index] = clock.todayAt(hook.offset)
	}
	for index := len(timestamps) - 2; index >= 0; index-- {
		if !timestamps[index].Before(timestamps[index+1]) {
			timestamps[index] = timestamps[index+1].Add(-time.Minute)
		}
	}
	stories := make([]eventSummaryStory, 0, len(hooks))
	for index, hook := range hooks {
		actor := pulseActors[index%len(pulseActors)]
		stories = append(stories, eventSummaryStory{
			ID:        fmt.Sprintf("sum_hook_%02d", index),
			SessionID: actor.sessionID, AgentName: actor.agentName,
			Type:      eventspkg.HookDispatchComplete,
			HookEvent: "on_session_stopped", HookName: hook.name,
			Outcome: hook.outcome,
			Summary: fmt.Sprintf("Hook %s dispatched", hook.name),
			At:      timestamps[index],
		})
	}
	return stories
}
