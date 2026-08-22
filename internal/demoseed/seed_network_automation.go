package demoseed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const (
	automationJobCount = 2
	automationRunCount = 2
)

type automationFixture struct {
	jobID        string
	runID        string
	workspaceKey string
}

func automationFixtures() []automationFixture {
	return []automationFixture{
		{jobID: launchAutomationID, runID: launchAutomationRun, workspaceKey: workspaceKeyLaunch},
		{jobID: digestAutomationID, runID: digestAutomationRun, workspaceKey: workspaceKeyPlatform},
	}
}

func cleanupWorkspaceAutomation(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceKey string,
	workspaceID string,
) error {
	for _, fixture := range automationFixtures() {
		if fixture.workspaceKey != workspaceKey {
			continue
		}
		if err := cleanupAutomationFixture(ctx, db, fixture, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

func cleanupAutomationFixture(
	ctx context.Context,
	db *globaldb.GlobalDB,
	fixture automationFixture,
	workspaceID string,
) error {
	if err := cleanupAutomationRun(ctx, db, fixture); err != nil {
		return err
	}
	job, jobErr := db.GetJob(ctx, fixture.jobID)
	if jobErr != nil {
		if errors.Is(jobErr, automation.ErrJobNotFound) {
			return nil
		}
		return fmt.Errorf("demo seed: inspect automation job %q: %w", fixture.jobID, jobErr)
	}
	if job.WorkspaceID != workspaceID {
		return fmt.Errorf(
			"demo seed: automation job %q belongs to workspace %q, not %q",
			fixture.jobID, job.WorkspaceID, workspaceID,
		)
	}
	if err := db.DeleteJob(ctx, fixture.jobID); err != nil {
		return fmt.Errorf("demo seed: replace automation job %q: %w", fixture.jobID, err)
	}
	return nil
}

func cleanupAutomationRun(ctx context.Context, db *globaldb.GlobalDB, fixture automationFixture) error {
	run, runErr := db.GetRun(ctx, fixture.runID)
	if runErr != nil {
		if errors.Is(runErr, automation.ErrRunNotFound) {
			return nil
		}
		return fmt.Errorf("demo seed: inspect automation run %q: %w", fixture.runID, runErr)
	}
	if run.JobID != fixture.jobID {
		return fmt.Errorf(
			"demo seed: automation run %q belongs to job %q, not %q", fixture.runID, run.JobID, fixture.jobID,
		)
	}
	if err := db.DeleteRun(ctx, fixture.runID); err != nil {
		return fmt.Errorf("demo seed: replace automation run %q: %w", fixture.runID, err)
	}
	return nil
}

func seedNetwork(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	messages []networkMessageStory,
) error {
	record, err := state.recordFor(workspaceKeyLaunch)
	if err != nil {
		return err
	}
	createdAt := messages[0].At.Add(-time.Minute)
	if err := db.CreateNetworkChannel(ctx, store.NetworkChannelEntry{
		Channel: launchChannel, WorkspaceID: record.ID,
		Purpose:      "Checkout launch decisions, market holds, and rollback evidence.",
		FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
		CreatedBy:    sessionLaunchDecisionID, CreatedAt: createdAt, UpdatedAt: messages[len(messages)-1].At,
	}); err != nil {
		return fmt.Errorf("demo seed: create Network channel: %w", err)
	}
	for _, story := range messages {
		body, err := json.Marshal(map[string]string{"kind": store.NetworkKindSay, jsonTextKey: story.Text})
		if err != nil {
			return fmt.Errorf("demo seed: encode Network message %q: %w", story.ID, err)
		}
		if _, err := db.WriteConversationMessage(ctx, store.NetworkConversationMessage{
			MessageID: story.ID, SessionID: story.SessionID, WorkspaceID: record.ID,
			Channel: launchChannel, Surface: store.NetworkSurfaceThread, ThreadID: launchThreadID,
			Direction: "sent", PeerFrom: story.SessionID, Kind: store.NetworkKindSay,
			ReplyTo: story.ReplyTo, Text: story.Text, PreviewText: story.Text,
			Body: body, SizeBytes: int64(len(body)), Timestamp: story.At,
		}); err != nil {
			return fmt.Errorf("demo seed: write Network message %q: %w", story.ID, err)
		}
	}
	return nil
}

func seedAutomation(ctx context.Context, db *globaldb.GlobalDB, state *scenario) error {
	if err := seedLaunchAutomation(ctx, db, state); err != nil {
		return err
	}
	return seedSettlementAutomation(ctx, db, state)
}

func seedLaunchAutomation(ctx context.Context, db *globaldb.GlobalDB, state *scenario) error {
	record, err := state.recordFor(workspaceKeyLaunch)
	if err != nil {
		return err
	}
	clock := state.clock
	job, err := db.CreateJob(ctx, automation.Job{
		ID: launchAutomationID, Scope: automation.AutomationScopeWorkspace,
		Name: "Brazil canary health digest", TargetKind: automation.TargetKindAgent,
		AgentName: agentProductLead, WorkspaceID: record.ID,
		Prompt:   "Summarize authorization errors, rollback threshold, and launch-tagged support conversations.",
		Schedule: &automation.ScheduleSpec{Mode: automation.ScheduleModeEvery, Interval: "15m"},
		Enabled:  false, Retry: automation.DefaultRetryConfig(), FireLimit: automation.DefaultFireLimitConfig(),
		Source:    automation.JobSourceDynamic,
		CreatedAt: clock.hoursAgo(12), UpdatedAt: clock.minutesAgo(39),
	})
	if err != nil {
		return fmt.Errorf("demo seed: create launch automation job: %w", err)
	}
	scheduledAt := clock.hoursMinutesAgo(6, 10)
	startedAt := scheduledAt.Add(20 * time.Second)
	endedAt := startedAt.Add(4 * time.Minute)
	if _, err := db.CreateRun(ctx, automation.Run{
		ID: launchAutomationRun, JobID: job.ID, SessionID: sessionSupportHandoffID,
		TaskID: taskSupportID, TaskRunID: "run_northstar_support_handoff",
		FireID: "fire_northstar_canary_preflight", Status: automation.RunCompleted, Attempt: 1,
		ScheduledAt: &scheduledAt, StartedAt: &startedAt, EndedAt: &endedAt,
		Metadata: map[string]any{"mode": "preflight", keyMarket: "BR", "open_conversations": 4},
	}); err != nil {
		return fmt.Errorf("demo seed: create launch automation run: %w", err)
	}
	return nil
}

func seedSettlementAutomation(ctx context.Context, db *globaldb.GlobalDB, state *scenario) error {
	record, err := state.recordFor(workspaceKeyPlatform)
	if err != nil {
		return err
	}
	clock := state.clock
	job, err := db.CreateJob(ctx, automation.Job{
		ID: digestAutomationID, Scope: automation.AutomationScopeWorkspace,
		Name: "Settlement retry watch", TargetKind: automation.TargetKindAgent,
		AgentName: agentPlatformEngineer, WorkspaceID: record.ID,
		Prompt:   "Report settlement workers that exhausted their retry budget in the last hour.",
		Schedule: &automation.ScheduleSpec{Mode: automation.ScheduleModeEvery, Interval: "1h"},
		Enabled:  true, Retry: automation.DefaultRetryConfig(), FireLimit: automation.DefaultFireLimitConfig(),
		Source:    automation.JobSourceDynamic,
		CreatedAt: clock.daysAgo(9), UpdatedAt: clock.hoursAgo(1),
	})
	if err != nil {
		return fmt.Errorf("demo seed: create settlement automation job: %w", err)
	}
	scheduledAt := clock.hoursAgo(1)
	startedAt := scheduledAt.Add(15 * time.Second)
	endedAt := startedAt.Add(2 * time.Minute)
	if _, err := db.CreateRun(ctx, automation.Run{
		ID: digestAutomationRun, JobID: job.ID,
		FireID: "fire_northstar_settlement_watch", Status: automation.RunCompleted, Attempt: 1,
		ScheduledAt: &scheduledAt, StartedAt: &startedAt, EndedAt: &endedAt,
		Metadata: map[string]any{"workers_exhausted": 3, "endpoint": "mercadox/settlement"},
	}); err != nil {
		return fmt.Errorf("demo seed: create settlement automation run: %w", err)
	}
	return nil
}
