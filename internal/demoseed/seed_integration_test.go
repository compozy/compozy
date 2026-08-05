//go:build integration

package demoseed

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/transcript"
)

func TestSeed(t *testing.T) {
	t.Parallel()

	t.Run("Should persist one coherent scenario and replace it without duplication", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		now := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
		homeDir := filepath.Join(t.TempDir(), "home")

		first, err := Seed(ctx, Options{HomeDir: homeDir, Now: now})
		if err != nil {
			t.Fatalf("Seed(first) error = %v", err)
		}
		if first.Counts.Sessions != 4 || first.Counts.Tasks != 7 || first.Counts.NetworkMessages != 5 {
			t.Fatalf("Seed(first).Counts = %#v, want 4 sessions, 7 tasks, and 5 Network messages", first.Counts)
		}

		if _, err := Seed(ctx, Options{HomeDir: homeDir, Now: now}); !errors.Is(err, ErrScenarioExists) {
			t.Fatalf("Seed(repeat without replace) error = %v, want ErrScenarioExists", err)
		}
		replaced, err := Seed(ctx, Options{HomeDir: homeDir, Replace: true, Now: now.Add(time.Hour)})
		if err != nil {
			t.Fatalf("Seed(replace) error = %v", err)
		}
		if replaced.WorkspaceID != first.WorkspaceID {
			t.Fatalf("Seed(replace).WorkspaceID = %q, want stable %q", replaced.WorkspaceID, first.WorkspaceID)
		}

		paths, err := config.ResolveHomePathsFrom(homeDir)
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		db, err := globaldb.OpenGlobalDB(ctx, paths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cancel()
			if err := db.Close(closeCtx); err != nil {
				t.Errorf("GlobalDB.Close() error = %v", err)
			}
		})

		assertWorkspaceRecords(t, ctx, db, replaced)
		assertTaskStory(t, ctx, db, replaced.WorkspaceID)
		assertNetworkStory(t, ctx, db, replaced.WorkspaceID)
		assertAutomationStory(t, ctx, db)
		assertLaunchTranscript(t, ctx, paths, replaced.WorkspaceID)
		assertLoopDefinition(t, replaced.WorkspaceRoot)
	})
}

func assertWorkspaceRecords(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, result Result) {
	t.Helper()
	workspaceRecord, err := db.GetWorkspace(ctx, result.WorkspaceID)
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if workspaceRecord.Name != workspaceName || workspaceRecord.DefaultAgent != "product-lead" {
		t.Fatalf("GetWorkspace() = %#v, want Northstar Pay with product-lead default", workspaceRecord)
	}
	sessions, err := db.ListSessions(ctx, store.SessionListQuery{WorkspaceID: result.WorkspaceID, Limit: 20})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 4 {
		t.Fatalf("len(ListSessions()) = %d, want 4", len(sessions))
	}
	for _, sessionRecord := range sessions {
		if sessionRecord.WorkspaceID != result.WorkspaceID || sessionRecord.State != "stopped" {
			t.Fatalf("session %q = %#v, want stopped and workspace-scoped", sessionRecord.ID, sessionRecord)
		}
	}
}

func assertTaskStory(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, workspaceID string) {
	t.Helper()
	tasks, err := db.ListTasks(ctx, task.Query{WorkspaceID: workspaceID, Limit: 20})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 7 {
		t.Fatalf("len(ListTasks()) = %d, want 7", len(tasks))
	}
	byID := make(map[string]task.Summary, len(tasks))
	for _, summary := range tasks {
		if summary.WorkspaceID != workspaceID {
			t.Fatalf("task %q workspace = %q, want %q", summary.ID, summary.WorkspaceID, workspaceID)
		}
		byID[summary.ID] = summary
	}
	approval := byID["task_northstar_authorize_canary"]
	if approval.Status != task.TaskStatusBlocked || approval.ApprovalState != task.ApprovalStatePending {
		t.Fatalf("Brazil approval task = %#v, want blocked with pending approval", approval)
	}
	if byID["task_northstar_launch_decision"].Status != task.TaskStatusCompleted {
		t.Fatalf("launch decision task = %#v, want completed", byID["task_northstar_launch_decision"])
	}
	for taskID, runID := range map[string]string{
		"task_northstar_partner_replay":  "run_northstar_partner_replay",
		"task_northstar_compliance_copy": "run_northstar_compliance_copy",
		taskSupportID:                    "run_northstar_support_handoff",
		taskLaunchDecisionID:             "run_northstar_launch_decision",
	} {
		run, err := db.GetTaskRun(ctx, runID)
		if err != nil {
			t.Fatalf("GetTaskRun(%q) error = %v", runID, err)
		}
		if run.TaskID != taskID ||
			run.WorkspaceID != workspaceID ||
			run.Status.Normalize() != task.TaskRunStatusCompleted {
			t.Fatalf("GetTaskRun(%q) = %#v, want completed workspace history for %q", runID, run, taskID)
		}
		events, err := db.ListTaskEvents(ctx, task.EventQuery{
			TaskID:    taskID,
			RunID:     runID,
			EventType: eventspkg.TaskRunCompleted,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(%q) error = %v", runID, err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("completed history events for %q = %d, want %d", runID, got, want)
		}
	}
	blocks, err := db.ListTaskBlocks(ctx, "task_northstar_mexico_replay", false)
	if err != nil {
		t.Fatalf("ListTaskBlocks(Mexico) error = %v", err)
	}
	if len(blocks) != 1 || !strings.Contains(blocks[0].Reason, "14:08–14:30 UTC") {
		t.Fatalf("Mexico blocks = %#v, want the 22-minute replay gap", blocks)
	}
}

func assertNetworkStory(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, workspaceID string) {
	t.Helper()
	ref := store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: launchChannel}
	channel, err := db.GetNetworkChannel(ctx, ref)
	if err != nil {
		t.Fatalf("GetNetworkChannel() error = %v", err)
	}
	if !strings.Contains(channel.Purpose, "Checkout launch") {
		t.Fatalf("GetNetworkChannel().Purpose = %q, want checkout launch purpose", channel.Purpose)
	}
	thread, err := db.GetThread(ctx, ref, launchThreadID)
	if err != nil {
		t.Fatalf("GetThread() error = %v", err)
	}
	if thread.MessageCount != 5 || thread.ParticipantCount != 4 {
		t.Fatalf("GetThread() = %#v, want 5 messages and 4 participants", thread)
	}
	messages, err := db.ListConversationMessages(ctx, store.NetworkConversationRef{
		WorkspaceID: workspaceID, Channel: launchChannel,
		Surface: store.NetworkSurfaceThread, ThreadID: launchThreadID,
	}, store.NetworkConversationMessageQuery{Limit: 20})
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	var narrative strings.Builder
	for _, message := range messages {
		narrative.WriteString(message.Text)
		narrative.WriteByte(' ')
	}
	for _, fact := range []string{"25%", "10,214", "14:08–14:30 UTC", "four launch-tagged"} {
		if !strings.Contains(narrative.String(), fact) {
			t.Fatalf("Network narrative = %q, want fact %q", narrative.String(), fact)
		}
	}
}

func assertAutomationStory(t *testing.T, ctx context.Context, db *globaldb.GlobalDB) {
	t.Helper()
	job, err := db.GetJob(ctx, launchAutomationID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job.Enabled || job.AgentName != "product-lead" || !strings.Contains(job.Name, "Brazil canary") {
		t.Fatalf("GetJob() = %#v, want paused Brazil canary digest owned by product-lead", job)
	}
	run, err := db.GetRun(ctx, launchAutomationRun)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if run.Status != automation.RunCompleted || run.TaskID != "task_northstar_support_handoff" {
		t.Fatalf("GetRun() = %#v, want completed support preflight", run)
	}
}

func assertLaunchTranscript(t *testing.T, ctx context.Context, paths config.HomePaths, workspaceID string) {
	t.Helper()
	sessionID := scenarioSessionIDs[3]
	reader, err := sessiondb.OpenSessionDBReadOnly(
		ctx,
		store.SessionDBOwner{SessionID: sessionID, WorkspaceID: workspaceID},
		store.SessionDBFile(filepath.Join(paths.SessionsDir, sessionID)),
	)
	if err != nil {
		t.Fatalf("OpenSessionDBReadOnly() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		if err := reader.Close(closeCtx); err != nil {
			t.Errorf("ReadOnlySessionDB.Close() error = %v", err)
		}
	})
	events, err := reader.Query(ctx, store.EventQuery{Limit: 20})
	if err != nil {
		t.Fatalf("ReadOnlySessionDB.Query() error = %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("len(ReadOnlySessionDB.Query()) = %d, want 6", len(events))
	}
	var conclusion string
	for _, persisted := range events {
		event, err := transcript.UnmarshalAgentEvent(persisted.Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent(%q) error = %v", persisted.ID, err)
		}
		if event.Type == "agent_message" && strings.Contains(event.Text, "Recommend Brazil") {
			conclusion = event.Text
		}
	}
	if !strings.Contains(conclusion, "25%") || !strings.Contains(conclusion, "Hold Mexico") {
		t.Fatalf("launch conclusion = %q, want Brazil canary and Mexico hold", conclusion)
	}
}

func assertLoopDefinition(t *testing.T, workspaceRoot string) {
	t.Helper()
	path := filepath.Join(
		workspaceRoot,
		config.DirName,
		config.LoopsDirName,
		launchLoopName,
		looppkg.DefinitionFileName,
	)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(Loop) error = %v", err)
	}
	definition, err := dsl.Parse(data)
	if err != nil {
		t.Fatalf("dsl.Parse(Loop) error = %v", err)
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compiler.Compile(Loop) error = %v", err)
	}
	if resolved.Definition.Meta.Name != launchLoopName ||
		!strings.Contains(resolved.Definition.Contract.Goal, "rollout decision") {
		t.Fatalf("compiled Loop = %#v, want launch-readiness rollout contract", resolved.Definition)
	}
}
