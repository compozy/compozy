package demoseed

import (
	"fmt"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	briefingAutomationID     = "job_northstar_launch_briefing"
	releaseNotesTriggerID    = "trg_northstar_release_notes"
	supportMacroTriggerID    = "trg_northstar_support_macros"
	automationSessionPrefix  = "sess_northstar_auto_"
	automationBriefingHour   = 8
	automationBriefingRuns   = 8
	automationSettlementRuns = 5
	// automationSettlementSpan bounds one settlement watch run from its scheduled time to its end.
	automationSettlementSpan = 2 * time.Minute
	// automationBriefingSpan bounds a briefing slot, including a timed-out attempt and its retry.
	automationBriefingSpan = 20 * time.Minute
)

// automationDefinitionStory is one scheduled job or event trigger an operator created in the app.
type automationDefinitionStory struct {
	Job          *automation.Job
	Trigger      *automation.Trigger
	WorkspaceKey string
}

func (d automationDefinitionStory) id() string {
	if d.Job != nil {
		return d.Job.ID
	}
	return d.Trigger.ID
}

// automationRunStory is one recorded fire of a definition, with the session it ran in.
type automationRunStory struct {
	ID           string
	JobID        string
	TriggerID    string
	WorkspaceKey string
	AgentName    string
	FireID       string
	Attempt      int
	Status       automation.RunStatus
	ScheduledAt  time.Time
	StartedAt    time.Time
	EndedAt      time.Time
	Error        string
	Metadata     map[string]any
	// Summary is what the agent reported in the run's session.
	Summary string
}

func automationOperatorJob(job automation.Job) *automation.Job {
	job.ProfileID = store.DefaultProfileID
	job.Scope = automation.AutomationScopeWorkspace
	job.TargetKind = automation.TargetKindAgent
	job.Source = automation.JobSourceDynamic
	job.GrantCeiling = workspaceaccess.Unrestricted
	job.FireLimit = automation.DefaultFireLimitConfig()
	return &job
}

func automationOperatorTrigger(trigger automation.Trigger) *automation.Trigger {
	trigger.ProfileID = store.DefaultProfileID
	trigger.Scope = automation.AutomationScopeWorkspace
	trigger.TargetKind = automation.TargetKindAgent
	trigger.Source = automation.JobSourceDynamic
	trigger.GrantCeiling = workspaceaccess.Unrestricted
	trigger.FireLimit = automation.DefaultFireLimitConfig()
	return &trigger
}

func scenarioAutomationDefinitions(clock timeline) []automationDefinitionStory {
	return []automationDefinitionStory{
		{WorkspaceKey: workspaceKeyLaunch, Job: automationOperatorJob(automation.Job{
			ID: briefingAutomationID, Name: "Weekday launch briefing", AgentName: agentProductLead,
			Prompt: "Before the 09:00 standup, summarize overnight checkout health for Brazil and Mexico: " +
				"authorization error rate against the 1.0% rollback threshold, open launch-tagged support " +
				"conversations, and any decision still waiting on the operator.",
			Schedule: &automation.ScheduleSpec{Mode: automation.ScheduleModeCron, Expr: "0 8 * * 1-5"},
			Enabled:  true,
			Retry: automation.RetryConfig{
				Strategy: automation.RetryStrategyBackoff, MaxRetries: 2, BaseDelay: "2m",
			},
			CreatedAt: clock.daysAgo(16), UpdatedAt: clock.daysAgo(16),
		})},
		{WorkspaceKey: workspaceKeyLaunch, Job: automationOperatorJob(automation.Job{
			ID: launchAutomationID, Name: "Brazil canary health digest", AgentName: agentProductLead,
			Prompt:   "Summarize authorization errors, rollback threshold, and launch-tagged support conversations.",
			Schedule: &automation.ScheduleSpec{Mode: automation.ScheduleModeEvery, Interval: "15m"},
			Enabled:  false, Retry: automation.DefaultRetryConfig(),
			CreatedAt: clock.daysAgo(9), UpdatedAt: clock.daysAgo(7),
		})},
		{WorkspaceKey: workspaceKeyPlatform, Job: automationOperatorJob(automation.Job{
			ID: digestAutomationID, Name: "Settlement retry watch", AgentName: agentPlatformEngineer,
			Prompt:   "Report settlement workers that exhausted their retry budget in the last hour.",
			Schedule: &automation.ScheduleSpec{Mode: automation.ScheduleModeEvery, Interval: "1h"},
			Enabled:  true,
			Retry: automation.RetryConfig{
				Strategy: automation.RetryStrategyBackoff, MaxRetries: 1, BaseDelay: "5m",
			},
			CreatedAt: clock.daysAgo(12), UpdatedAt: clock.daysAgo(3),
		})},
		{WorkspaceKey: workspaceKeyPlatform, Trigger: automationOperatorTrigger(automation.Trigger{
			ID: releaseNotesTriggerID, Name: "Draft release notes after a release session",
			AgentName: agentDocsSteward, Event: automation.TriggerEventSessionStopped,
			Filter: map[string]string{"data.agent_name": agentReleaseManager, "data.stop_reason": "completed"},
			Prompt: "A release-manager session just finished. Read its release window and draft the runbook " +
				"and changelog notes for every change it promoted. Flag any change without a rollback path.",
			Enabled: true, Retry: automation.DefaultBackoffRetryConfig(),
			CreatedAt: clock.daysAgo(11), UpdatedAt: clock.daysAgo(11),
		})},
		{WorkspaceKey: workspaceKeyLaunch, Trigger: automationOperatorTrigger(automation.Trigger{
			ID: supportMacroTriggerID, Name: "Check support macros after a compliance review",
			AgentName: agentSupportLead, Event: automation.TriggerEventSessionStopped,
			Filter: map[string]string{"data.agent_name": agentComplianceReview},
			Prompt: "Compliance just finished a wording review. Compare the approved wording with the " +
				"launch support macros and list every macro that needs an edit before customers see it.",
			Enabled: false, Retry: automation.DefaultRetryConfig(),
			CreatedAt: clock.daysAgo(10), UpdatedAt: clock.daysAgo(2),
		})},
	}
}

func scenarioAutomationRuns(clock timeline) []automationRunStory {
	runs := briefingRuns(clock)
	runs = append(runs, canaryDigestRuns(clock)...)
	runs = append(runs, settlementWatchRuns(clock)...)
	runs = append(runs, releaseNotesRuns(clock)...)
	return append(runs, supportMacroRuns(clock)...)
}

// weekdayBriefingSlots returns the most recent weekday 08:00 local fires that already happened.
func weekdayBriefingSlots(clock timeline, count int) []time.Time {
	slots := make([]time.Time, 0, count)
	for back := 0; len(slots) < count && back < 30; back++ {
		at := clock.dayStart(back).Add(automationBriefingHour * time.Hour)
		// A slot counts only once its longest run, a timed-out attempt plus its retry, has finished.
		if at.Add(automationBriefingSpan).After(clock.Now()) || at.Weekday() == time.Saturday ||
			at.Weekday() == time.Sunday {
			continue
		}
		slots = append(slots, at.UTC())
	}
	return slots
}

var briefingSummaries = []string{
	"Brazil canary at 25%: authorization errors 0.41%, under the 1.0% rollback threshold. " +
		"Three launch-tagged conversations open, none escalated. Mexico still held on the partner replay gap.",
	"Overnight was quiet. Brazil authorization errors 0.38%. Support closed two of the five launch " +
		"conversations. No decision is waiting on you.",
	"Brazil errors peaked at 0.72% around 02:10 UTC during the issuer maintenance window, then settled at " +
		"0.44%. Worth a look before raising the canary. Mexico replay gap unchanged.",
	"Brazil steady at 0.40%. Compliance approved the new fallback wording; support macros still use the " +
		"old copy. One decision waits on you: raise the Brazil canary from 25% to 50%.",
	"Brazil 0.36%, Mexico still held. Fraud flagged 4 chargebacks from one merchant; all are authorization " +
		"noise from a retry loop, not confirmed fraud.",
}

func briefingRuns(clock timeline) []automationRunStory {
	slots := weekdayBriefingSlots(clock, automationBriefingRuns)
	runs := make([]automationRunStory, 0, len(slots)+1)
	for index, scheduled := range slots {
		fire := fmt.Sprintf("fire_northstar_briefing_%02d", index+1)
		summary := briefingSummaries[index%len(briefingSummaries)]
		started := scheduled.Add(4 * time.Second)
		attempt := 1
		if index == 2 {
			failedEnd := started.Add(10 * time.Minute)
			runs = append(runs, automationRun(automationRunStory{
				ID: fmt.Sprintf("autorun_northstar_briefing_%02d_a1", index+1), JobID: briefingAutomationID,
				WorkspaceKey: workspaceKeyLaunch, AgentName: agentProductLead, FireID: fire, Attempt: 1,
				Status: automation.RunFailed, ScheduledAt: scheduled, StartedAt: started, EndedAt: failedEnd,
				Error:    "agent turn timed out after 10m while the support queue export was still loading",
				Metadata: map[string]any{"retry": "scheduled", "next_attempt_in": "2m"},
			}, "The support queue export is still loading; waiting for it before I summarize."))
			started, attempt = failedEnd.Add(2*time.Minute), 2
		}
		runs = append(runs, automationRun(automationRunStory{
			ID: fmt.Sprintf("autorun_northstar_briefing_%02d", index+1), JobID: briefingAutomationID,
			WorkspaceKey: workspaceKeyLaunch, AgentName: agentProductLead, FireID: fire, Attempt: attempt,
			Status: automation.RunCompleted, ScheduledAt: scheduled, StartedAt: started,
			EndedAt:  started.Add(time.Duration(3+index%3) * time.Minute),
			Metadata: map[string]any{keyMarket: "BR"},
		}, summary))
	}
	return runs
}

func canaryDigestRuns(clock timeline) []automationRunStory {
	runs := make([]automationRunStory, 0, 3)
	for index, hours := range []int{9*24 - 2, 8*24 + 3, 7*24 + 5} {
		scheduled := clock.hoursAgo(hours)
		started := scheduled.Add(6 * time.Second)
		id := fmt.Sprintf("autorun_northstar_canary_digest_%02d", index+1)
		if index == 0 {
			id = launchAutomationRun
		}
		runs = append(runs, automationRun(automationRunStory{
			ID: id, JobID: launchAutomationID, WorkspaceKey: workspaceKeyLaunch, AgentName: agentProductLead,
			FireID: fmt.Sprintf("fire_northstar_canary_%02d", index+1), Attempt: 1, Status: automation.RunCompleted,
			ScheduledAt: scheduled, StartedAt: started, EndedAt: started.Add(2 * time.Minute),
			Metadata: map[string]any{"mode": "preflight", keyMarket: "BR", "open_conversations": 4 - index},
		}, fmt.Sprintf("Brazil canary at 5%%: authorization errors %.2f%%, rollback threshold 1.0%%. "+
			"%d launch-tagged conversations open.", 0.52-0.06*float64(index), 4-index)))
	}
	return runs
}

func settlementWatchRuns(clock timeline) []automationRunStory {
	runs := make([]automationRunStory, 0, automationSettlementRuns+2)
	exhausted := []int{3, 0, 1, 0, 2}
	// The newest slot is the latest hour whose run has already finished; older slots follow it.
	latest := clock.Now().Add(-automationSettlementSpan).Truncate(time.Hour)
	for index := range automationSettlementRuns {
		scheduled := latest.Add(-time.Duration(index) * time.Hour)
		started := scheduled.Add(3 * time.Second)
		summary := "No settlement worker exhausted its retry budget in the last hour."
		if exhausted[index] > 0 {
			summary = fmt.Sprintf("%d settlement workers exhausted their retry budget against "+
				"mercadox/settlement in the last hour. All failed on the same 409 duplicate-batch response; "+
				"the batches settled on the next cycle, so nothing is stuck.", exhausted[index])
		}
		id := fmt.Sprintf("autorun_northstar_settlement_watch_%02d", index+1)
		if index == 0 {
			id = digestAutomationRun
		}
		runs = append(runs, automationRun(automationRunStory{
			ID: id, JobID: digestAutomationID, WorkspaceKey: workspaceKeyPlatform, AgentName: agentPlatformEngineer,
			FireID: fmt.Sprintf("fire_northstar_settlement_%02d", index+1), Attempt: 1,
			Status: automation.RunCompleted, ScheduledAt: scheduled, StartedAt: started,
			EndedAt:  started.Add(90 * time.Second),
			Metadata: map[string]any{"workers_exhausted": exhausted[index], "endpoint": "mercadox/settlement"},
		}, summary))
	}
	scheduled := clock.hoursAgo(19).Truncate(time.Hour)
	for attempt := 1; attempt <= 2; attempt++ {
		started := scheduled.Add(time.Duration(attempt-1)*5*time.Minute + 3*time.Second)
		runs = append(runs, automationRun(automationRunStory{
			ID:    fmt.Sprintf("autorun_northstar_settlement_watch_outage_a%d", attempt),
			JobID: digestAutomationID, WorkspaceKey: workspaceKeyPlatform, AgentName: agentPlatformEngineer,
			FireID: "fire_northstar_settlement_outage", Attempt: attempt, Status: automation.RunFailed,
			ScheduledAt: scheduled, StartedAt: started, EndedAt: started.Add(40 * time.Second),
			Error:    "settlement log reader returned 503 from logs.northstar.internal; no worker data for the hour",
			Metadata: map[string]any{"endpoint": "mercadox/settlement"},
		}, "I can't read the settlement worker logs: logs.northstar.internal returns 503. "+
			"I have no data for this hour, so I am not reporting a count."))
	}
	return runs
}

func releaseNotesRuns(clock timeline) []automationRunStory {
	notes := []string{
		"Drafted runbook notes for 3 promoted changes: settlement retry jitter, Brazil fallback copy, " +
			"canary alert routing. All three name a rollback path.",
		"Drafted notes for 2 promoted changes. The Mexico partner timeout bump has no rollback step in its " +
			"runbook; I flagged it instead of writing one.",
		"Drafted notes for the settlement retry quarantine. Runbook page 'Settlement retries' now matches " +
			"the shipped retry budget of 5.",
	}
	runs := make([]automationRunStory, 0, len(notes))
	for index, days := range []int{9, 5, 1} {
		started := clock.daysHoursAgo(days, 3+index)
		runs = append(runs, automationRun(automationRunStory{
			ID:        fmt.Sprintf("autorun_northstar_release_notes_%02d", index+1),
			TriggerID: releaseNotesTriggerID, WorkspaceKey: workspaceKeyPlatform, AgentName: agentDocsSteward,
			FireID: fmt.Sprintf("fire_northstar_release_notes_%02d", index+1), Attempt: 1,
			Status: automation.RunCompleted, StartedAt: started, EndedAt: started.Add(6 * time.Minute),
			Metadata: map[string]any{keyEvent: automation.TriggerEventSessionStopped},
		}, notes[index]))
	}
	return runs
}

func supportMacroRuns(clock timeline) []automationRunStory {
	started := clock.daysHoursAgo(4, 6)
	return []automationRunStory{automationRun(automationRunStory{
		ID: "autorun_northstar_support_macros_01", TriggerID: supportMacroTriggerID,
		WorkspaceKey: workspaceKeyLaunch, AgentName: agentSupportLead,
		FireID: "fire_northstar_support_macros_01", Attempt: 1, Status: automation.RunCompleted,
		StartedAt: started, EndedAt: started.Add(4 * time.Minute),
		Metadata: map[string]any{keyEvent: automation.TriggerEventSessionStopped},
	}, "Two macros still promise 'instant refunds' in Brazil; compliance approved 'refunds within 2 business "+
		"days'. Edits for BR-REFUND-01 and BR-REFUND-03 are ready for review.")}
}

func automationRun(run automationRunStory, summary string) automationRunStory {
	run.Summary = summary
	return run
}
