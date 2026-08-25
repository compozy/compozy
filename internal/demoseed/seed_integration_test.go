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
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/notifications/presets"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/transcript"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

func TestSeed(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve hook order when local-day offsets are still in the future", func(t *testing.T) {
		t.Parallel()

		clock := newTimeline(time.Date(2026, 7, 27, 8, 0, 0, 0, time.UTC))
		hooks := hookDispatchSummaries(clock)
		if len(hooks) < 2 {
			t.Fatalf("hookDispatchSummaries() returned %d hooks, want at least 2", len(hooks))
		}
		for index := 1; index < len(hooks); index++ {
			if !hooks[index-1].At.Before(hooks[index].At) {
				t.Fatalf("hook timestamps[%d:%d] = %s then %s, want strict order",
					index-1, index, hooks[index-1].At, hooks[index].At)
			}
		}
		if hooks[len(hooks)-1].At.After(clock.Now()) {
			t.Fatalf("latest hook timestamp = %s, want no later than %s", hooks[len(hooks)-1].At, clock.Now())
		}
	})

	t.Run("Should persist one coherent scenario and replace it without duplication", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		now := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
		homeDir := filepath.Join(t.TempDir(), "home")

		first, err := Seed(ctx, Options{HomeDir: homeDir, Now: now})
		if err != nil {
			t.Fatalf("Seed(first) error = %v", err)
		}
		if first.Counts.Workspaces != 2 || first.Counts.Sessions != 12 || first.Counts.Tasks != 23 ||
			first.Counts.NetworkMessages != 6 || first.Counts.GoalTurns != 2 ||
			first.Counts.Worktrees != 1 || first.Counts.NotificationPresets != 3 {
			t.Fatalf("Seed(first).Counts = %#v, want the complete multi-surface scenario", first.Counts)
		}
		obsoleteFixture := filepath.Join(first.WorkspaceRoot, "obsolete-seed-fixture.md")
		if err := os.WriteFile(obsoleteFixture, []byte("obsolete\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(obsolete fixture) error = %v", err)
		}
		if err := writeSeedMarker(first.WorkspaceRoot, workspaceKeyLaunch); err != nil {
			t.Fatalf("writeSeedMarker(obsolete fixture) error = %v", err)
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
		if _, err := os.Stat(obsoleteFixture); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(obsolete fixture) error = %v, want removed by the seed manifest", err)
		}
		replacedAgain, err := Seed(ctx, Options{HomeDir: homeDir, Replace: true, Now: now.Add(2 * time.Hour)})
		if err != nil {
			t.Fatalf("Seed(second replace) error = %v", err)
		}
		if replacedAgain.WorkspaceID != first.WorkspaceID || replacedAgain.Counts != first.Counts {
			t.Fatalf(
				"Seed(second replace) = {workspace: %q, counts: %#v}, want {%q, %#v}",
				replacedAgain.WorkspaceID,
				replacedAgain.Counts,
				first.WorkspaceID,
				first.Counts,
			)
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

		assertWorkspaceRecords(t, ctx, db, replacedAgain)
		assertTaskStory(t, ctx, db, replacedAgain.WorkspaceID)
		assertNetworkStory(t, ctx, db, replacedAgain.WorkspaceID)
		assertAutomationStory(t, ctx, db)
		assertLaunchTranscript(t, ctx, paths, replacedAgain.WorkspaceID)
		assertLoopDefinition(t, replacedAgain.WorkspaceRoot)
		assertCompleteSeedSurfaces(t, ctx, db, paths, replacedAgain)
	})

	t.Run("Should resolve an existing workspace through a filesystem alias", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		root := t.TempDir()
		realRoot := filepath.Join(root, "real-workspace")
		if err := os.Mkdir(realRoot, 0o755); err != nil {
			t.Fatalf("Mkdir(real workspace) error = %v", err)
		}
		aliasRoot := filepath.Join(root, "alias-workspace")
		if err := os.Symlink(realRoot, aliasRoot); err != nil {
			t.Fatalf("Symlink(workspace alias) error = %v", err)
		}
		identity, err := compozyworkspace.EnsureIdentity(ctx, realRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity(real workspace) error = %v", err)
		}
		db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(root, "compozy.db"))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cancel()
			if err := db.Close(closeCtx); err != nil {
				t.Errorf("GlobalDB.Close() error = %v", err)
			}
		})
		workspace := compozyworkspace.Workspace{
			ID: identity.WorkspaceID, Name: "Alias workspace", RootDir: realRoot,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if err := db.InsertWorkspace(ctx, workspace); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}

		resolved, err := lookupExistingWorkspace(ctx, db, workspace.Name, aliasRoot)
		if err != nil {
			t.Fatalf("lookupExistingWorkspace(alias) error = %v", err)
		}
		if resolved.ID != workspace.ID {
			t.Fatalf("lookupExistingWorkspace(alias).ID = %q, want %q", resolved.ID, workspace.ID)
		}
	})

	t.Run("Should refuse to replace an unowned workspace root", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		now := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
		homeDir := filepath.Join(t.TempDir(), "home")
		story := scenarioWorkspaces(newTimeline(now))[0]
		workspaceRoot := filepath.Join(homeDir, filepath.FromSlash(story.Relative))
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(unowned workspace) error = %v", err)
		}
		sentinel := filepath.Join(workspaceRoot, "operator-owned.txt")
		if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(sentinel) error = %v", err)
		}

		if _, err := Seed(ctx, Options{HomeDir: homeDir, Replace: true, Now: now}); err == nil ||
			!strings.Contains(err.Error(), "refusing to replace unowned workspace root") {
			t.Fatalf("Seed(replace unowned workspace) error = %v, want ownership refusal", err)
		}
		if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "keep\n" {
			t.Fatalf("ReadFile(sentinel) = %q, %v; want preserved content", contents, err)
		}
	})
}

func assertCompleteSeedSurfaces(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	paths config.HomePaths,
	result Result,
) {
	t.Helper()
	if len(result.WorkspaceIDs) != 2 {
		t.Fatalf("WorkspaceIDs = %#v, want two workspaces", result.WorkspaceIDs)
	}
	var runCount int
	for _, workspaceID := range result.WorkspaceIDs {
		runs, err := db.ListLoopRuns(ctx, looppkg.RunListQuery{WorkspaceID: looppkg.WorkspaceID(workspaceID), Limit: 100})
		if err != nil {
			t.Fatalf("ListLoopRuns(%q) error = %v", workspaceID, err)
		}
		for _, run := range runs {
			if !run.Historical || len(run.Inputs) == 0 {
				t.Fatalf("seeded Loop run %q = %#v, want historical with resolved inputs", run.ID, run)
			}
		}
		runCount += len(runs)
		live := true
		liveRuns, err := db.ListLoopRuns(ctx, looppkg.RunListQuery{
			WorkspaceID: looppkg.WorkspaceID(workspaceID), Live: &live, Limit: 100,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns(live, %q) error = %v", workspaceID, err)
		}
		if len(liveRuns) != 0 {
			t.Fatalf("ListLoopRuns(live, %q) = %#v, want no historical rows", workspaceID, liveRuns)
		}
	}
	if runCount != result.Counts.LoopRuns {
		t.Fatalf("persisted Loop runs = %d, want count %d", runCount, result.Counts.LoopRuns)
	}
	turns, err := db.ListGoalTurns(ctx, goal.TurnQuery{
		WorkspaceID: looppkg.WorkspaceID(result.WorkspaceID), LoopRunID: loopGoalRunID, Limit: 20,
	})
	if err != nil {
		t.Fatalf("ListGoalTurns() error = %v", err)
	}
	if len(turns.Turns) != 2 {
		t.Fatalf("ListGoalTurns() returned %d turns, want 2: %#v", len(turns.Turns), turns)
	}
	for index, turn := range turns.Turns {
		if turn.SessionID != sessionIncidentReviewID {
			t.Fatalf("ListGoalTurns().Turns[%d].SessionID = %q, want %q", index, turn.SessionID, sessionIncidentReviewID)
		}
	}
	worktrees, err := db.WorktreeStore().List(ctx, result.WorkspaceIDs[1])
	if err != nil {
		t.Fatalf("WorktreeStore.List() error = %v", err)
	}
	if len(worktrees) != 1 || worktrees[0].State != worktree.StateReady {
		t.Fatalf("WorktreeStore.List() = %#v, want one ready Git worktree", worktrees)
	}
	if _, err := os.Stat(worktrees[0].Path); err != nil {
		t.Fatalf("Stat(seed worktree) error = %v", err)
	}
	items, err := db.ListPresetsForProfile(ctx, presets.Query{}, store.DefaultProfileID)
	if err != nil {
		t.Fatalf("ListPresets() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("ListPresets() = %#v, want three built-in presets", items)
	}
	assertSeedMemoriesReadable(t, ctx, db, paths, result)
	meta, err := store.ReadSessionMeta(store.SessionMetaFile(filepath.Join(paths.SessionsDir, sessionRollbackDrillID)))
	if err != nil {
		t.Fatalf("ReadSessionMeta(checkout engineer) error = %v", err)
	}
	if meta.EffectivePermissionsValue() != approveAll {
		t.Fatalf("checkout engineer permissions = %q, want %q", meta.EffectivePermissionsValue(), approveAll)
	}
}

func assertSeedMemoriesReadable(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	paths config.HomePaths,
	result Result,
) {
	t.Helper()
	memoryStore := memory.NewStore(paths.MemoryDir)
	global, err := memoryStore.Scan(ctx, memcontract.ScopeGlobal)
	if err != nil {
		t.Fatalf("Memory.Scan(global) error = %v", err)
	}
	readable := len(global)
	for _, workspaceID := range result.WorkspaceIDs {
		workspaceRecord, err := db.GetWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("GetWorkspace(%q) error = %v", workspaceID, err)
		}
		workspaceStore := memoryStore.ForWorkspace(workspaceRecord.RootDir)
		workspaceMemories, err := workspaceStore.Scan(ctx, memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("Memory.Scan(workspace %q) error = %v", workspaceID, err)
		}
		readable += len(workspaceMemories)
		for _, agent := range scenarioAgents() {
			if stateKeyForWorkspaceName(workspaceRecord.Name) != agent.WorkspaceKey {
				continue
			}
			agentMemories, err := workspaceStore.ForAgent(
				workspaceID, agent.Name, memcontract.AgentTierWorkspace,
			).Scan(ctx, memcontract.ScopeAgent)
			if err != nil {
				t.Fatalf("Memory.Scan(agent %q) error = %v", agent.Name, err)
			}
			readable += len(agentMemories)
		}
	}
	if readable != result.Counts.Memories {
		t.Fatalf("readable memories = %d, want %d", readable, result.Counts.Memories)
	}
}

func stateKeyForWorkspaceName(name string) string {
	if name == launchWorkspaceName {
		return workspaceKeyLaunch
	}
	return workspaceKeyPlatform
}

func assertWorkspaceRecords(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, result Result) {
	t.Helper()
	workspaceRecord, err := db.GetWorkspace(ctx, result.WorkspaceID)
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if workspaceRecord.Name != launchWorkspaceName || workspaceRecord.DefaultAgent != "product-lead" {
		t.Fatalf("GetWorkspace() = %#v, want Northstar Pay with product-lead default", workspaceRecord)
	}
	sessions, err := db.ListSessions(ctx, store.SessionListQuery{WorkspaceID: result.WorkspaceID, Limit: 20})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 6 {
		t.Fatalf("len(ListSessions()) = %d, want 6 launch sessions", len(sessions))
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
	if len(tasks) != 9 {
		t.Fatalf("len(ListTasks()) = %d, want 9 launch tasks", len(tasks))
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
	readScope := store.ReadScope{ProfileID: store.DefaultProfileID}
	channel, err := db.GetNetworkChannel(ctx, readScope, ref)
	if err != nil {
		t.Fatalf("GetNetworkChannel() error = %v", err)
	}
	if channel.ProfileID != store.DefaultProfileID || !strings.Contains(channel.Purpose, "Checkout launch") {
		t.Fatalf("GetNetworkChannel().Purpose = %q, want checkout launch purpose", channel.Purpose)
	}
	thread, err := db.GetThread(ctx, readScope, ref, launchThreadID)
	if err != nil {
		t.Fatalf("GetThread() error = %v", err)
	}
	if thread.ProfileID != store.DefaultProfileID || thread.MessageCount != 6 || thread.ParticipantCount != 4 {
		t.Fatalf("GetThread() = %#v, want 6 messages and 4 participants", thread)
	}
	messages, err := db.ListConversationMessages(ctx, store.NetworkConversationRef{
		WorkspaceID: workspaceID, Channel: launchChannel,
		Surface: store.NetworkSurfaceThread, ThreadID: launchThreadID,
	}, store.NetworkConversationMessageQuery{ReadScope: readScope, Limit: 20})
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	var narrative strings.Builder
	for _, message := range messages {
		if message.ProfileID != store.DefaultProfileID {
			t.Fatalf(
				"network message %q ProfileID = %q, want %q",
				message.ID,
				message.ProfileID,
				store.DefaultProfileID,
			)
		}
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
	sessionID := sessionLaunchDecisionID
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
	if len(events) != 11 {
		t.Fatalf("len(ReadOnlySessionDB.Query()) = %d, want 11", len(events))
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
		loopLaunchReadiness,
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
	if resolved.Definition.Meta.Name != loopLaunchReadiness ||
		!strings.Contains(resolved.Definition.Contract.Goal, "rollout decision") {
		t.Fatalf("compiled Loop = %#v, want launch-readiness rollout contract", resolved.Definition)
	}
}
