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

func cleanupAutomationState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	replace bool,
) error {
	if !replace {
		return nil
	}
	run, runErr := db.GetRun(ctx, launchAutomationRun)
	if runErr != nil && !errors.Is(runErr, automation.ErrRunNotFound) {
		return fmt.Errorf("demo seed: inspect automation run: %w", runErr)
	}
	if runErr == nil && run.JobID != launchAutomationID {
		return fmt.Errorf(
			"demo seed: automation run %q belongs to job %q, not %q",
			launchAutomationRun,
			run.JobID,
			launchAutomationID,
		)
	}
	job, jobErr := db.GetJob(ctx, launchAutomationID)
	if jobErr != nil && !errors.Is(jobErr, automation.ErrJobNotFound) {
		return fmt.Errorf("demo seed: inspect automation job: %w", jobErr)
	}
	if jobErr == nil && job.WorkspaceID != workspaceID {
		return fmt.Errorf(
			"demo seed: automation job %q belongs to workspace %q, not %q",
			launchAutomationID,
			job.WorkspaceID,
			workspaceID,
		)
	}
	if runErr == nil {
		if err := db.DeleteRun(ctx, launchAutomationRun); err != nil {
			return fmt.Errorf("demo seed: replace automation run: %w", err)
		}
	}
	if jobErr == nil {
		if err := db.DeleteJob(ctx, launchAutomationID); err != nil {
			return fmt.Errorf("demo seed: replace automation job: %w", err)
		}
	}
	return nil
}

func seedNetwork(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	messages []networkMessageStory,
) error {
	createdAt := messages[0].At.Add(-time.Minute)
	if err := db.CreateNetworkChannel(ctx, store.NetworkChannelEntry{
		ProfileID: store.DefaultProfileID,
		Channel:   launchChannel, WorkspaceID: workspaceID,
		Purpose:      "Checkout launch decisions, market holds, and rollback evidence.",
		FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
		CreatedBy:    scenarioSessionIDs[3], CreatedAt: createdAt, UpdatedAt: messages[len(messages)-1].At,
	}); err != nil {
		return fmt.Errorf("demo seed: create Network channel: %w", err)
	}
	for _, story := range messages {
		body, err := json.Marshal(map[string]string{"kind": store.NetworkKindSay, jsonTextKey: story.Text})
		if err != nil {
			return fmt.Errorf("demo seed: encode Network message %q: %w", story.ID, err)
		}
		if _, err := db.WriteConversationMessage(ctx, store.NetworkConversationMessage{
			ProfileID: store.DefaultProfileID,
			MessageID: story.ID, SessionID: story.SessionID, WorkspaceID: workspaceID,
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

func seedAutomation(ctx context.Context, db *globaldb.GlobalDB, workspaceID string, now time.Time) error {
	job, err := db.CreateJob(ctx, automation.Job{
		ProfileID: store.DefaultProfileID,
		ID:        launchAutomationID, Scope: automation.AutomationScopeWorkspace,
		Name: "Brazil canary health digest", TargetKind: automation.TargetKindAgent,
		AgentName: productLeadAgent, WorkspaceID: workspaceID,
		Prompt:   "Summarize authorization errors, rollback threshold, and launch-tagged support conversations.",
		Schedule: &automation.ScheduleSpec{Mode: automation.ScheduleModeEvery, Interval: "15m"},
		Enabled:  false, Retry: automation.DefaultRetryConfig(), FireLimit: automation.DefaultFireLimitConfig(),
		Source: automation.JobSourceDynamic, CreatedAt: now.Add(-12 * time.Hour), UpdatedAt: now.Add(-39 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("demo seed: create automation job: %w", err)
	}
	scheduledAt := now.Add(-6*time.Hour - 10*time.Minute)
	startedAt := scheduledAt.Add(20 * time.Second)
	endedAt := startedAt.Add(4 * time.Minute)
	if _, err := db.CreateRun(ctx, automation.Run{
		ID: launchAutomationRun, JobID: job.ID, SessionID: scenarioSessionIDs[2],
		TaskID: taskSupportID, TaskRunID: "run_northstar_support_handoff",
		FireID: "fire_northstar_canary_preflight", Status: automation.RunCompleted, Attempt: 1,
		ScheduledAt: &scheduledAt, StartedAt: &startedAt, EndedAt: &endedAt,
		Metadata: map[string]any{"mode": "preflight", "market": "BR", "open_conversations": 4},
	}); err != nil {
		return fmt.Errorf("demo seed: create automation run: %w", err)
	}
	return nil
}
