//go:build integration && !windows

package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/worktree"
	"golang.org/x/sys/execabs"
)

func TestDaemonTaskPerRunWorktreeJourneyE2E002IT029IT040(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    "local-default",
			DefaultProvider: acpmock.ProviderName,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "network_local_fixture.json"),
			FixtureAgent: "local-default",
			AgentName:    "local-default",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	claimer := createFixtureBackedSession(t, ctx, harness, "local-default", "per-run-claimer")
	taskRecord := createTaskViaUDS(t, ctx, harness, "Per-run task worktree journey")

	var cliProfile compozycontract.TaskExecutionProfilePayload
	if err := harness.CLI.RunJSONInDir(
		ctx,
		root,
		&cliProfile,
		"task",
		"profile",
		"set-worktree",
		taskRecord.ID,
		"--mode",
		string(taskpkg.WorktreeModePerRun),
		"-o",
		"json",
	); err != nil {
		t.Fatalf("task profile set-worktree(per_run) error = %v", err)
	}
	if cliProfile.Worktree.Mode != taskpkg.WorktreeModePerRun {
		t.Fatalf("CLI worktree profile = %#v, want per_run", cliProfile.Worktree)
	}
	assertTaskWorktreeProfileParity(t, ctx, harness, taskRecord.ID, cliProfile)

	run := enqueueTaskRunViaUDS(t, ctx, harness, taskRecord.ID, "")
	if run.ResolvedWorktreeMode != taskpkg.WorktreeModePerRun || run.WorktreeID != "" {
		t.Fatalf("queued run worktree snapshot = %#v, want unresolved per_run", run)
	}

	var currentProfile compozycontract.TaskExecutionProfilePayload
	if err := harness.CLI.RunJSONInDir(
		ctx,
		root,
		&currentProfile,
		"task",
		"profile",
		"set-worktree",
		taskRecord.ID,
		"--mode",
		string(taskpkg.WorktreeModeNone),
		"-o",
		"json",
	); err != nil {
		t.Fatalf("post-enqueue task profile set-worktree(none) error = %v", err)
	}

	if _, err := harness.ClaimExactTaskRunForSession(ctx, run.ID, claimer); err != nil {
		t.Fatalf("ClaimExactTaskRunForSession(%s) error = %v", run.ID, err)
	}
	started, err := harness.StartClaimedTaskRunForSession(
		ctx,
		run.ID,
		claimer,
		compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + run.ID},
	)
	if err != nil {
		storedRuns, listErr := harness.ListTaskRuns(ctx, taskRecord.ID, nil)
		daemonLog, logErr := os.ReadFile(filepath.Join(harness.Artifacts.RootDir(), "daemon-process.log"))
		t.Fatalf(
			"StartTaskRun(%s) error = %v; runs=%#v list_error=%v log_error=%v daemon_log=%s",
			run.ID,
			err,
			storedRuns,
			listErr,
			logErr,
			daemonLog,
		)
	}
	if started.ResolvedWorktreeMode != taskpkg.WorktreeModePerRun || started.WorktreeID == "" {
		t.Fatalf("started run worktree = %#v, want frozen per_run materialization", started)
	}
	taskSession, err := harness.GetSession(ctx, started.SessionID)
	if err != nil {
		t.Fatalf("GetSession(%s) error = %v", started.SessionID, err)
	}
	if taskSession.WorktreeID != started.WorktreeID {
		t.Fatalf("session worktree_id = %q, want run worktree_id %q", taskSession.WorktreeID, started.WorktreeID)
	}

	inspection := waitForWorktreeE2EState(
		t,
		ctx,
		harness,
		harness.WorkspaceID,
		started.WorktreeID,
		worktree.StateReady,
	)
	if inspection.Worktree.Origin != string(worktree.OriginPerRun) || inspection.Worktree.RunID != run.ID ||
		!strings.HasPrefix(inspection.Worktree.Branch, "run/") {
		t.Fatalf("per-run worktree = %#v, want attributable run/ branch", inspection.Worktree)
	}
	branchCommand := execabs.CommandContext(ctx, "git", "branch", "--show-current")
	branchCommand.Dir = inspection.Worktree.Path
	branchOutput, err := branchCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --show-current in per-run worktree error = %v; output=%s", err, branchOutput)
	}
	if got := strings.TrimSpace(string(branchOutput)); got != inspection.Worktree.Branch {
		t.Fatalf("checked-out branch = %q, want %q", got, inspection.Worktree.Branch)
	}

	lease, err := harness.CompleteClaimedTaskRunForSession(
		ctx,
		run.ID,
		taskSession,
		compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
	)
	if err != nil {
		t.Fatalf("CompleteClaimedTaskRunForSession(%s) error = %v", run.ID, err)
	}
	if lease.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("completed lease status = %q, want completed", lease.Status)
	}
	runs, err := harness.ListTaskRuns(ctx, taskRecord.ID, nil)
	if err != nil {
		t.Fatalf("ListTaskRuns(%s) error = %v", taskRecord.ID, err)
	}
	if len(runs) != 1 || runs[0].WorktreeID != started.WorktreeID ||
		runs[0].ResolvedWorktreeMode != taskpkg.WorktreeModePerRun {
		t.Fatalf("terminal run history = %#v, want durable worktree binding", runs)
	}
	waitForWorktreeE2EState(t, ctx, harness, harness.WorkspaceID, started.WorktreeID, worktree.StateReady)

	secondClaimer := createFixtureBackedSession(t, ctx, harness, "local-default", "root-claimer")
	rootTask := createTaskViaUDS(t, ctx, harness, "Root snapshot authority")
	rootRun := enqueueTaskRunViaUDS(t, ctx, harness, rootTask.ID, "")
	if rootRun.ResolvedWorktreeMode != taskpkg.WorktreeModeNone {
		t.Fatalf("root queued run worktree mode = %q, want none", rootRun.ResolvedWorktreeMode)
	}
	if err := harness.CLI.RunJSONInDir(
		ctx,
		root,
		&currentProfile,
		"task",
		"profile",
		"set-worktree",
		rootTask.ID,
		"--mode",
		string(taskpkg.WorktreeModePerRun),
		"-o",
		"json",
	); err != nil {
		t.Fatalf("post-enqueue root task profile set-worktree(per_run) error = %v", err)
	}
	if _, err := harness.ClaimExactTaskRunForSession(ctx, rootRun.ID, secondClaimer); err != nil {
		t.Fatalf("ClaimExactTaskRunForSession(root %s) error = %v", rootRun.ID, err)
	}
	rootStarted, err := harness.StartClaimedTaskRunForSession(
		ctx,
		rootRun.ID,
		secondClaimer,
		compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + rootRun.ID},
	)
	if err != nil {
		t.Fatalf("StartTaskRun(root %s) error = %v", rootRun.ID, err)
	}
	if rootStarted.ResolvedWorktreeMode != taskpkg.WorktreeModeNone || rootStarted.WorktreeID != "" {
		t.Fatalf("started root run worktree = %#v, want frozen root execution", rootStarted)
	}
}

func TestDaemonTaskFanOutPerRunIsolationIT029IT031(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    "local-default",
			DefaultProvider: acpmock.ProviderName,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "network_local_fixture.json"),
			FixtureAgent: "local-default",
			AgentName:    "local-default",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	taskRecord := createTaskViaUDS(t, ctx, harness, "Fan-out isolated workers")
	var fanOut compozycontract.FanOutTaskRunsResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		root,
		&fanOut,
		"task",
		"fan-out",
		taskRecord.ID,
		"--designation",
		"frontend",
		"--designation",
		"runtime",
		"--designation",
		"tests",
		"--idempotency-key",
		"fanout-worktrees",
		"--worktree-per-run",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("task fan-out --worktree-per-run error = %v", err)
	}
	if fanOut.DesignationGroupID == "" || len(fanOut.Runs) != 3 {
		t.Fatalf("fan-out response = %#v, want one group with three runs", fanOut)
	}

	worktreeIDs := make(map[string]string, len(fanOut.Runs))
	branches := make(map[string]string, len(fanOut.Runs))
	paths := make(map[string]string, len(fanOut.Runs))
	startedRuns := make([]compozycontract.TaskRunPayload, 0, len(fanOut.Runs))
	for index, queued := range fanOut.Runs {
		if queued.ResolvedWorktreeMode != taskpkg.WorktreeModePerRun ||
			queued.DesignationGroupID != fanOut.DesignationGroupID {
			t.Fatalf("queued fan-out run[%d] = %#v, want grouped per_run snapshot", index, queued)
		}
		claimer := createFixtureBackedSession(
			t,
			ctx,
			harness,
			"local-default",
			"fanout-claimer-"+strconv.Itoa(index+1),
		)
		if _, err := harness.ClaimExactTaskRunForSession(ctx, queued.ID, claimer); err != nil {
			t.Fatalf("ClaimExactTaskRunForSession(%s) error = %v", queued.ID, err)
		}
		started, err := harness.StartClaimedTaskRunForSession(
			ctx,
			queued.ID,
			claimer,
			compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + queued.ID},
		)
		if err != nil {
			t.Fatalf("StartTaskRun(%s) error = %v", queued.ID, err)
		}
		if started.WorktreeID == "" || started.ResolvedWorktreeMode != taskpkg.WorktreeModePerRun {
			t.Fatalf("started fan-out run[%d] = %#v, want per-run worktree", index, started)
		}
		inspection := waitForWorktreeE2EState(
			t,
			ctx,
			harness,
			harness.WorkspaceID,
			started.WorktreeID,
			worktree.StateReady,
		)
		if inspection.Worktree.RunID != queued.ID || inspection.Worktree.Origin != string(worktree.OriginPerRun) {
			t.Fatalf("fan-out worktree[%d] = %#v, want run %s attribution", index, inspection.Worktree, queued.ID)
		}
		if previous, duplicate := worktreeIDs[started.WorktreeID]; duplicate {
			t.Fatalf("worktree %q reused by runs %s and %s", started.WorktreeID, previous, queued.ID)
		}
		if previous, duplicate := branches[inspection.Worktree.Branch]; duplicate {
			t.Fatalf("branch %q reused by runs %s and %s", inspection.Worktree.Branch, previous, queued.ID)
		}
		if previous, duplicate := paths[inspection.Worktree.Path]; duplicate {
			t.Fatalf("path %q reused by runs %s and %s", inspection.Worktree.Path, previous, queued.ID)
		}
		worktreeIDs[started.WorktreeID] = queued.ID
		branches[inspection.Worktree.Branch] = queued.ID
		paths[inspection.Worktree.Path] = queued.ID

		events := readWorktreeE2ESSE(
			t,
			ctx,
			harness,
			worktreeE2EPath(harness.WorkspaceID, started.WorktreeID)+"/stream?after_sequence=0",
			1,
			nil,
		)
		if len(events) != 1 || events[0].Name != worktree.EventCreated {
			t.Fatalf("worktree events for run %s = %#v, want one created event", queued.ID, events)
		}
		var eventPayload struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal(events[0].Data, &eventPayload); err != nil {
			t.Fatalf("decode worktree.created for run %s: %v", queued.ID, err)
		}
		if eventPayload.RunID != queued.ID {
			t.Fatalf("worktree.created run_id = %q, want %q", eventPayload.RunID, queued.ID)
		}
		startedRuns = append(startedRuns, started)
	}

	var canceled compozycontract.TaskRunResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/task-runs/"+url.PathEscape(startedRuns[1].ID)+"/cancel",
		compozycontract.CancelTaskRunRequest{Reason: "fan-out cancellation retention"},
		&canceled,
	); err != nil {
		t.Fatalf("cancel fan-out run %s error = %v", startedRuns[1].ID, err)
	}
	if canceled.Run.Status.Normalize() != taskpkg.TaskRunStatusCanceled {
		t.Fatalf("canceled fan-out run status = %q, want canceled", canceled.Run.Status)
	}
	waitForWorktreeE2EState(
		t,
		ctx,
		harness,
		harness.WorkspaceID,
		startedRuns[1].WorktreeID,
		worktree.StateReady,
	)

	for _, started := range []compozycontract.TaskRunPayload{startedRuns[0], startedRuns[2]} {
		taskSession, err := harness.GetSession(ctx, started.SessionID)
		if err != nil {
			t.Fatalf("GetSession(%s) error = %v", started.SessionID, err)
		}
		if _, err := harness.CompleteClaimedTaskRunForSession(
			ctx,
			started.ID,
			taskSession,
			compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
		); err != nil {
			t.Fatalf("complete fan-out run %s error = %v", started.ID, err)
		}
	}
}

func TestDaemonTaskFanOutSecondMaterializationFailureIT031(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    "local-default",
			DefaultProvider: acpmock.ProviderName,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "network_local_fixture.json"),
			FixtureAgent: "local-default",
			AgentName:    "local-default",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	const taskTitle = "Fan-out isolated failure attribution"
	taskRecord := createTaskViaUDS(t, ctx, harness, taskTitle)
	var fanOut compozycontract.FanOutTaskRunsResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		root,
		&fanOut,
		"task",
		"fan-out",
		taskRecord.ID,
		"--designation",
		"first succeeds",
		"--designation",
		"second fails",
		"--designation",
		"third succeeds",
		"--idempotency-key",
		"fanout-second-materialization-fails",
		"--worktree-per-run",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("task fan-out --worktree-per-run error = %v", err)
	}
	if len(fanOut.Runs) != 3 {
		t.Fatalf("fan-out runs = %d, want 3", len(fanOut.Runs))
	}

	startedRuns := make([]compozycontract.TaskRunPayload, 0, 2)
	placementDir := ""
	for index, queued := range fanOut.Runs {
		claimer := createFixtureBackedSession(
			t,
			ctx,
			harness,
			"local-default",
			"fanout-failure-claimer-"+strconv.Itoa(index+1),
		)
		if _, err := harness.ClaimExactTaskRunForSession(ctx, queued.ID, claimer); err != nil {
			t.Fatalf("ClaimExactTaskRunForSession(%s) error = %v", queued.ID, err)
		}
		if index == 1 {
			if placementDir == "" {
				t.Fatal("placement directory is empty after first materialization")
			}
			blockedPath := filepath.Join(
				placementDir,
				worktree.RunWorktreeName(taskTitle, queued.ID),
			)
			if err := os.MkdirAll(blockedPath, 0o755); err != nil {
				t.Fatalf("pre-create second materialization path %q: %v", blockedPath, err)
			}
			if _, err := harness.StartClaimedTaskRunForSession(
				ctx,
				queued.ID,
				claimer,
				compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + queued.ID},
			); err == nil || !strings.Contains(err.Error(), worktree.ErrPerRunMaterialization.Error()) {
				t.Fatalf(
					"StartClaimedTaskRunForSession(second) error = %v, want %s",
					err,
					worktree.ErrPerRunMaterialization,
				)
			}
			continue
		}

		started, err := harness.StartClaimedTaskRunForSession(
			ctx,
			queued.ID,
			claimer,
			compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + queued.ID},
		)
		if err != nil {
			t.Fatalf("StartClaimedTaskRunForSession(%s) error = %v", queued.ID, err)
		}
		inspection := waitForWorktreeE2EState(
			t,
			ctx,
			harness,
			harness.WorkspaceID,
			started.WorktreeID,
			worktree.StateReady,
		)
		if placementDir == "" {
			placementDir = filepath.Dir(inspection.Worktree.Path)
		}
		startedRuns = append(startedRuns, started)
	}

	runs, err := harness.ListTaskRuns(ctx, taskRecord.ID, nil)
	if err != nil {
		t.Fatalf("ListTaskRuns(%s) error = %v", taskRecord.ID, err)
	}
	byID := make(map[string]compozycontract.TaskRunPayload, len(runs))
	for _, run := range runs {
		byID[run.ID] = run
	}
	if got := byID[fanOut.Runs[0].ID]; got.Status.Normalize() != taskpkg.TaskRunStatusRunning || got.WorktreeID == "" {
		t.Fatalf("first fan-out run = %#v, want running with worktree", got)
	}
	if got := byID[fanOut.Runs[1].ID]; got.Status.Normalize() != taskpkg.TaskRunStatusFailed ||
		got.WorktreeID != "" || !strings.Contains(got.Error, worktree.ErrPerRunMaterialization.Error()) {
		t.Fatalf("second fan-out run = %#v, want attributable materialization failure", got)
	}
	if got := byID[fanOut.Runs[2].ID]; got.Status.Normalize() != taskpkg.TaskRunStatusRunning || got.WorktreeID == "" {
		t.Fatalf("third fan-out run = %#v, want running with worktree", got)
	}

	var listing compozycontract.WorktreesResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodGet,
		"/api/workspaces/"+url.PathEscape(harness.WorkspaceID)+"/worktrees",
		nil,
		&listing,
	); err != nil {
		t.Fatalf("list worktrees after fan-out failure: %v", err)
	}
	for _, item := range listing.Worktrees {
		if item.RunID == fanOut.Runs[1].ID {
			t.Fatalf("failed second run left worktree row: %#v", item)
		}
	}

	for _, started := range startedRuns {
		taskSession, err := harness.GetSession(ctx, started.SessionID)
		if err != nil {
			t.Fatalf("GetSession(%s) error = %v", started.SessionID, err)
		}
		if _, err := harness.CompleteClaimedTaskRunForSession(
			ctx,
			started.ID,
			taskSession,
			compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
		); err != nil {
			t.Fatalf("complete fan-out run %s error = %v", started.ID, err)
		}
	}
}

func TestDaemonWorktreeCLIJourneyE2E003(t *testing.T) {
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var empty []compozycontract.WorktreePayload
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &empty, "worktree", "list", "--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree list(empty) error = %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("worktree list(empty) = %#v, want non-nil empty array", empty)
	}

	var created compozycontract.WorktreePayload
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &created, "worktree", "create", "cli-journey",
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree create error = %v", err)
	}
	if created.ID == "" || created.State != string(worktree.StatePending) {
		t.Fatalf("worktree create = %#v, want accepted pending row", created)
	}
	ready := waitForWorktreeE2EState(t, ctx, harness, harness.WorkspaceID, created.ID, worktree.StateReady)

	var inspected compozycontract.WorktreeInspectionResponse
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &inspected, "worktree", "inspect", created.ID,
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree inspect error = %v", err)
	}
	if inspected.Worktree.ID != ready.Worktree.ID || inspected.Worktree.Path != ready.Worktree.Path ||
		!inspected.Repo.GitBacked || !inspected.Repo.GitAvailable {
		t.Fatalf("worktree inspect = %#v, want API-ready worktree with Git capability", inspected)
	}

	var status compozycontract.WorktreeStatusResponse
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &status, "worktree", "status", created.ID, "--refresh",
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree status error = %v", err)
	}
	if status.WorktreeID != created.ID || status.Status.Branch == nil || *status.Status.Branch != "cli-journey" {
		t.Fatalf("worktree status = %#v, want refreshed cli-journey branch", status)
	}

	if err := os.WriteFile(filepath.Join(ready.Worktree.Path, "dirty.txt"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty worktree file: %v", err)
	}
	cliRefusal := worktreeE2ECLIRemovalRefusal(t, ctx, harness, root, harness.WorkspaceID, created.ID)
	var apiRefusal compozycontract.WorktreeRemovalRefusalPayload
	statusCode := loopRuntimeRawJSON(
		t, ctx, harness.HTTPClient,
		harness.HTTPURL(worktreeE2EPath(harness.WorkspaceID, created.ID)),
		http.MethodDelete, nil, &apiRefusal,
	)
	if statusCode != http.StatusConflict || cliRefusal != apiRefusal ||
		cliRefusal.Code != worktree.ErrDirtyRequiresForce.Error() {
		t.Fatalf("removal refusal CLI/API = %#v/%#v status=%d", cliRefusal, apiRefusal, statusCode)
	}

	var removed struct {
		Action   string                          `json:"action"`
		Worktree compozycontract.WorktreePayload `json:"worktree"`
	}
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &removed, "worktree", "remove", created.ID, "--force",
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree remove(force) error = %v", err)
	}
	if removed.Action != "removed" || removed.Worktree.ID != created.ID {
		t.Fatalf("worktree remove(force) = %#v", removed)
	}

	_, stderr, err := harness.CLI.RunInDir(
		ctx, root, "worktree", "inspect", "missing-worktree",
		"--workspace", harness.WorkspaceID, "-o", "json",
	)
	if err == nil {
		t.Fatal("worktree inspect(missing) error = nil")
	}
	var notFound compozycontract.ErrorPayload
	if decodeErr := json.Unmarshal([]byte(stderr), &notFound); decodeErr != nil {
		t.Fatalf("decode CLI not-found: %v; stderr=%s", decodeErr, stderr)
	}
	if notFound.Code != "worktree_not_found" || strings.Contains(stderr, "root fallback") {
		t.Fatalf("worktree inspect(missing) = %#v; stderr=%s", notFound, stderr)
	}
}

func TestDaemonWorktreeStreamsAndIsolationIT034IT038(t *testing.T) {
	t.Parallel()

	rootA := initWorktreeE2ERepository(t)
	rootB := initWorktreeE2ERepository(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: rootA},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	workspaceA := harness.WorkspaceID
	resolvedB, err := harness.ResolveWorkspace(ctx, rootB)
	if err != nil {
		t.Fatalf("ResolveWorkspace(B) error = %v", err)
	}
	workspaceB := resolvedB.ID

	itemA := createWorktreeE2E(t, ctx, harness, workspaceA, "stream-alpha")
	itemB := createWorktreeE2E(t, ctx, harness, workspaceB, "stream-beta")
	itemA = waitForWorktreeE2EState(t, ctx, harness, workspaceA, itemA.ID, worktree.StateReady).Worktree
	itemB = waitForWorktreeE2EState(t, ctx, harness, workspaceB, itemB.ID, worktree.StateReady).Worktree

	for _, transport := range []struct {
		name string
		get  func(context.Context, string, string, any, any) error
	}{
		{name: "HTTP", get: harness.HTTPJSON},
		{name: "UDS", get: harness.UDSJSON},
	} {
		transport := transport
		t.Run("Should isolate "+transport.name+" worktree lists by workspace", func(t *testing.T) {
			for _, target := range []struct {
				workspaceID string
				wantID      string
				rejectID    string
			}{
				{workspaceID: workspaceA, wantID: itemA.ID, rejectID: itemB.ID},
				{workspaceID: workspaceB, wantID: itemB.ID, rejectID: itemA.ID},
			} {
				var listing compozycontract.WorktreesResponse
				path := "/api/workspaces/" + url.PathEscape(target.workspaceID) + "/worktrees"
				if err := transport.get(ctx, http.MethodGet, path, nil, &listing); err != nil {
					t.Fatalf("%s list(%s) error = %v", transport.name, target.workspaceID, err)
				}
				assertWorktreeE2EListIsolation(t, listing, target.wantID, target.rejectID)
				if err := transport.get(ctx, http.MethodGet, path, nil, &listing); err != nil {
					t.Fatalf("%s cached list(%s) error = %v", transport.name, target.workspaceID, err)
				}
				assertWorktreeE2EListIsolation(t, listing, target.wantID, target.rejectID)
			}
		})
	}

	var refreshed compozycontract.WorktreeStatusResponse
	if err := harness.HTTPJSON(
		ctx, http.MethodGet,
		worktreeE2EPath(workspaceA, itemA.ID)+"/status?refresh=true", nil, &refreshed,
	); err != nil {
		t.Fatalf("refresh worktree A status error = %v", err)
	}
	if err := harness.HTTPJSON(
		ctx, http.MethodDelete, worktreeE2EPath(workspaceA, itemA.ID), nil, nil,
	); err != nil {
		t.Fatalf("remove worktree A error = %v", err)
	}

	events := readWorktreeE2ESSE(
		t, ctx, harness, worktreeE2EPath(workspaceA, itemA.ID)+"/stream?after_sequence=0", 5, nil,
	)
	assertWorktreeE2EEventSequence(
		t, events, workspaceA, itemA.ID,
		[]string{
			worktree.EventCreated,
			worktree.EventStatusRefreshed,
			worktree.EventStatusRefreshed,
			worktree.EventStatusRefreshed,
			worktree.EventRemoved,
		},
	)
	firstSequence, err := strconv.ParseInt(events[0].ID, 10, 64)
	if err != nil {
		t.Fatalf("parse first worktree event id %q: %v", events[0].ID, err)
	}
	replayed := readWorktreeE2ESSE(
		t, ctx, harness,
		worktreeE2EPath(workspaceA, itemA.ID)+"/stream?after_sequence="+strconv.FormatInt(firstSequence, 10),
		4, nil,
	)
	assertWorktreeE2EEventSequence(
		t, replayed, workspaceA, itemA.ID,
		[]string{
			worktree.EventStatusRefreshed,
			worktree.EventStatusRefreshed,
			worktree.EventStatusRefreshed,
			worktree.EventRemoved,
		},
	)

	ready := make(chan struct{})
	catalogEvents := make(chan []worktreeE2ESSEEvent, 1)
	go func() {
		catalogEvents <- readWorktreeE2ESSE(
			t, ctx, harness, "/api/worktrees/catalog-stream", 1, ready,
		)
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatalf("wait for catalog stream readiness: %v", ctx.Err())
	}
	itemB2 := createWorktreeE2E(t, ctx, harness, workspaceB, "catalog-beta")
	select {
	case catalog := <-catalogEvents:
		if len(catalog) != 1 || catalog[0].Name != "worktree_catalog_changed" {
			t.Fatalf("catalog events = %#v", catalog)
		}
		var payload compozycontract.WorktreeCatalogEventPayload
		if err := json.Unmarshal(catalog[0].Data, &payload); err != nil {
			t.Fatalf("decode catalog event: %v", err)
		}
		if payload.WorkspaceID != workspaceB || payload.WorktreeID != itemB2.ID {
			t.Fatalf("catalog payload = %#v, want workspace B worktree", payload)
		}
	case <-ctx.Done():
		t.Fatalf("wait for catalog event: %v", ctx.Err())
	}
}

func TestDaemonNativeWorktreeJourneyE2E004(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	foreignRoot := initWorktreeE2ERepository(t)
	mockSpec := e2etest.MockAgentSpec{
		FixturePath:  mockFixturePath(t, "tool_approval_grants_fixture.json"),
		FixtureAgent: "tool-approval-grants",
		AgentName:    "mock-worktree-tools",
	}
	denyMockSpec := e2etest.MockAgentSpec{
		FixturePath:  mockFixturePath(t, "workspace_access_fixture.json"),
		FixtureAgent: "hosted-deny",
		AgentName:    "mock-worktree-tools-deny",
	}
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
		ConfigSeed: e2etest.ConfigSeedOptions{
			PermissionMode: config.PermissionModeApproveReads,
			Mutate: func(cfg *config.Config) {
				cfg.Memory.Enabled = false
			},
		},
		MockAgents: []e2etest.MockAgentSpec{mockSpec, denyMockSpec},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	workspaceID := harness.WorkspaceID
	session, client := newWorkspaceAccessHostedSession(
		t, ctx, harness, mockSpec.AgentName, "worktree-native-tools",
	)
	foreignWorkspace, err := harness.ResolveWorkspace(ctx, foreignRoot)
	if err != nil {
		t.Fatalf("ResolveWorkspace(foreign) error = %v", err)
	}
	harness.WorkspaceID = workspaceID
	foreignItem := createWorktreeE2E(t, ctx, harness, foreignWorkspace.ID, "foreign-only")
	foreignItem = waitForWorktreeE2EState(
		t, ctx, harness, foreignWorkspace.ID, foreignItem.ID, worktree.StateReady,
	).Worktree

	listed := callHostedTool(
		t, ctx, client, toolspkg.ToolIDWorktreeList, "worktree-list-empty",
		map[string]any{"workspace": workspaceID},
	)
	var initial compozycontract.WorktreesResponse
	decodeWorktreeE2EStructured(t, listed, &initial)
	if listed.IsError || len(initial.Worktrees) != 0 {
		t.Fatalf("native worktree list(empty) = %#v / %#v", listed, initial)
	}

	createdResult := callApprovalToolWithDecision(
		t, ctx, harness, session.ID, client,
		toolspkg.ToolIDWorktreeCreate, "worktree-create-approved",
		map[string]any{"workspace": workspaceID, "name": "native-tools"},
		"allow-once",
	)
	var created compozycontract.WorktreeResponse
	decodeWorktreeE2EStructured(t, createdResult, &created)
	if createdResult.IsError || created.Worktree.ID == "" || created.Worktree.State != string(worktree.StatePending) {
		t.Fatalf("native worktree create = %#v / %#v", createdResult, created)
	}
	ready := waitForWorktreeE2EState(
		t, ctx, harness, workspaceID, created.Worktree.ID, worktree.StateReady,
	)
	if err := os.WriteFile(filepath.Join(ready.Worktree.Path, "dirty.txt"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write native worktree dirty file: %v", err)
	}

	removeResult := callApprovalToolWithDecision(
		t, ctx, harness, session.ID, client,
		toolspkg.ToolIDWorktreeRemove, "worktree-remove-approved",
		map[string]any{"workspace": harness.WorkspaceID, "ref": created.Worktree.ID},
		"allow-once",
	)
	var refusal compozycontract.WorktreeRemovalRefusalPayload
	decodeWorktreeE2EStructured(t, removeResult, &refusal)
	if !removeResult.IsError || refusal.Code != worktree.ErrDirtyRequiresForce.Error() ||
		refusal.Risk.ChangedFiles == 0 {
		t.Fatalf("native worktree removal refusal = %#v / %#v", removeResult, refusal)
	}
	cliRefusal := worktreeE2ECLIRemovalRefusal(
		t, ctx, harness, root, workspaceID, created.Worktree.ID,
	)
	if refusal != cliRefusal {
		t.Fatalf("native/CLI removal refusal = %#v / %#v, want byte-equivalent payloads", refusal, cliRefusal)
	}

	deniedResult := callApprovalToolWithDecision(
		t, ctx, harness, session.ID, client,
		toolspkg.ToolIDWorktreeCreate, "worktree-create-denied",
		map[string]any{"workspace": workspaceID, "name": "must-not-exist"},
		"reject-once",
	)
	if deniedResult == nil || !deniedResult.IsError {
		t.Fatalf(
			"denied native worktree create = %s, want deterministic denial",
			worktreeE2EResultJSON(t, deniedResult),
		)
	}
	listed = callHostedTool(
		t, ctx, client, toolspkg.ToolIDWorktreeList, "worktree-list-after-denial",
		map[string]any{"workspace": workspaceID},
	)
	var afterDenial compozycontract.WorktreesResponse
	decodeWorktreeE2EStructured(t, listed, &afterDenial)
	if len(afterDenial.Worktrees) != 1 || afterDenial.Worktrees[0].ID != created.Worktree.ID {
		t.Fatalf("native list after denied create = %#v, want only original worktree", afterDenial.Worktrees)
	}

	foreignDenied := callApprovalToolWithDecision(
		t, ctx, harness, session.ID, client,
		toolspkg.ToolIDWorktreeList, "worktree-list-foreign-denied",
		map[string]any{"workspace": foreignWorkspace.ID},
		"reject-once",
	)
	foreignDeniedJSON := worktreeE2EResultJSON(t, foreignDenied)
	if foreignDenied == nil || !foreignDenied.IsError ||
		!strings.Contains(foreignDeniedJSON, string(toolspkg.ReasonWorkspaceAccessDenied)) {
		t.Fatalf("foreign native worktree list = %s, want workspace denial", foreignDeniedJSON)
	}
	if strings.Contains(foreignDeniedJSON, foreignItem.ID) ||
		strings.Contains(foreignDeniedJSON, foreignItem.Path) ||
		strings.Contains(foreignDeniedJSON, foreignItem.Name) {
		t.Fatalf("foreign native worktree denial leaked target data: %s", foreignDeniedJSON)
	}

	denySession, denyClient := newWorkspaceAccessHostedSession(
		t, ctx, harness, denyMockSpec.AgentName, "worktree-native-tools-deny",
	)
	if denySession.ID == "" {
		t.Fatal("deny-mode worktree session ID is empty")
	}
	deniedByMode := callHostedTool(
		t, ctx, denyClient, toolspkg.ToolIDWorktreeCreate, "worktree-create-deny-mode",
		map[string]any{"workspace": workspaceID, "name": "deny-mode-must-not-exist"},
	)
	deniedByModeJSON := worktreeE2EResultJSON(t, deniedByMode)
	if deniedByMode == nil || !deniedByMode.IsError ||
		!strings.Contains(deniedByModeJSON, string(toolspkg.ReasonApprovalRequired)) {
		t.Fatalf("deny-mode native worktree create = %s, want deterministic denial", deniedByModeJSON)
	}
	var afterDenyMode compozycontract.WorktreesResponse
	if err := harness.HTTPJSON(
		ctx, http.MethodGet,
		"/api/workspaces/"+url.PathEscape(workspaceID)+"/worktrees",
		nil, &afterDenyMode,
	); err != nil {
		t.Fatalf("list after deny-mode create error = %v", err)
	}
	if len(afterDenyMode.Worktrees) != 1 || afterDenyMode.Worktrees[0].ID != created.Worktree.ID {
		t.Fatalf("list after deny-mode create = %#v, want no partial creation", afterDenyMode.Worktrees)
	}
}

type worktreeE2ESSEEvent struct {
	ID   string
	Name string
	Data json.RawMessage
}

func initWorktreeE2ERepository(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	commands := [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "worktree-e2e@example.com"},
		{"config", "user.name", "Worktree E2E"},
	}
	for _, args := range commands {
		command := execabs.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s error = %v; output=%s", strings.Join(args, " "), err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("write repository README: %v", err)
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-m", "initial"}} {
		command := execabs.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s error = %v; output=%s", strings.Join(args, " "), err, output)
		}
	}
	return root
}

func createWorktreeE2E(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	name string,
) compozycontract.WorktreePayload {
	t.Helper()
	var response compozycontract.WorktreeResponse
	if err := harness.HTTPJSON(
		ctx, http.MethodPost,
		"/api/workspaces/"+url.PathEscape(workspaceID)+"/worktrees",
		compozycontract.CreateWorktreeRequest{Name: name}, &response,
	); err != nil {
		t.Fatalf("create worktree %q in %q error = %v", name, workspaceID, err)
	}
	if response.Worktree.ID == "" || response.Worktree.State != string(worktree.StatePending) {
		t.Fatalf("create worktree %q response = %#v, want pending", name, response)
	}
	return response.Worktree
}

func assertTaskWorktreeProfileParity(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	want compozycontract.TaskExecutionProfilePayload,
) {
	t.Helper()

	path := "/api/tasks/" + url.PathEscape(taskID) + "/execution-profile"
	for _, transport := range []struct {
		name string
		get  func(context.Context, string, string, any, any) error
	}{
		{name: "HTTP", get: harness.HTTPJSON},
		{name: "UDS", get: harness.UDSJSON},
	} {
		var response compozycontract.TaskExecutionProfileResponse
		if err := transport.get(ctx, http.MethodGet, path, nil, &response); err != nil {
			t.Fatalf("%s get task execution profile error = %v", transport.name, err)
		}
		gotJSON, err := json.Marshal(response.Profile)
		if err != nil {
			t.Fatalf("marshal %s task execution profile: %v", transport.name, err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal CLI task execution profile: %v", err)
		}
		if !bytes.Equal(gotJSON, wantJSON) {
			t.Fatalf("%s profile = %s, want CLI profile %s", transport.name, gotJSON, wantJSON)
		}
	}
}

func worktreeE2ECLIRemovalRefusal(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	root string,
	workspaceID string,
	worktreeID string,
) compozycontract.WorktreeRemovalRefusalPayload {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDir(
		ctx, root, "worktree", "remove", worktreeID,
		"--workspace", workspaceID, "-o", "json",
	)
	if err == nil || strings.TrimSpace(stdout) != "" {
		t.Fatalf("worktree remove(dirty) = stdout %q, error %v, want structured refusal", stdout, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 65 {
		t.Fatalf("worktree remove(dirty) error = %v, want exit 65", err)
	}
	var refusal compozycontract.WorktreeRemovalRefusalPayload
	if decodeErr := json.Unmarshal([]byte(stderr), &refusal); decodeErr != nil {
		t.Fatalf("decode CLI removal refusal: %v; stderr=%s", decodeErr, stderr)
	}
	return refusal
}

func waitForWorktreeE2EState(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	worktreeID string,
	want worktree.State,
) compozycontract.WorktreeInspectionResponse {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var response compozycontract.WorktreeInspectionResponse
		err := harness.HTTPJSON(
			ctx, http.MethodGet, worktreeE2EPath(workspaceID, worktreeID), nil, &response,
		)
		if err == nil && response.Worktree.State == string(want) {
			return response
		}
		select {
		case <-ctx.Done():
			t.Fatalf(
				"wait for worktree %s/%s state %q: last response=%#v error=%v context=%v",
				workspaceID, worktreeID, want, response, err, ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func worktreeE2EPath(workspaceID string, worktreeID string) string {
	return "/api/workspaces/" + url.PathEscape(workspaceID) + "/worktrees/" + url.PathEscape(worktreeID)
}

func assertWorktreeE2EListIsolation(
	t testing.TB,
	listing compozycontract.WorktreesResponse,
	wantID string,
	rejectID string,
) {
	t.Helper()
	if len(listing.Worktrees) != 1 || listing.Worktrees[0].ID != wantID {
		t.Fatalf("worktree list = %#v, want only %q", listing.Worktrees, wantID)
	}
	for _, item := range listing.Worktrees {
		if item.ID == rejectID {
			t.Fatalf("worktree list leaked %q: %#v", rejectID, listing.Worktrees)
		}
	}
}

func readWorktreeE2ESSE(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	want int,
	ready chan<- struct{},
) (events []worktreeE2ESSEEvent) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
	if err != nil {
		t.Fatalf("new worktree SSE request: %v", err)
	}
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatalf("open worktree SSE %q: %v", path, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close worktree SSE %q: %v", path, closeErr)
		}
	}()
	if response.StatusCode != http.StatusOK {
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			t.Fatalf("read worktree SSE failure: %v", readErr)
		}
		t.Fatalf("worktree SSE %q status = %d; body=%s", path, response.StatusCode, payload)
	}
	if ready != nil {
		close(ready)
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var event worktreeE2ESSEEvent
	var data bytes.Buffer
	flush := func() bool {
		if event.Name == "" && data.Len() == 0 {
			return false
		}
		event.Data = append(json.RawMessage(nil), bytes.TrimSpace(data.Bytes())...)
		events = append(events, event)
		event = worktreeE2ESSEEvent{}
		data.Reset()
		return len(events) >= want
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if flush() {
				return events
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "id: "):
			event.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			event.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("scan worktree SSE %q: %v", path, err)
	}
	if flush() {
		return events
	}
	t.Fatalf("worktree SSE %q ended with %d events, want %d", path, len(events), want)
	return nil
}

func assertWorktreeE2EEventSequence(
	t testing.TB,
	events []worktreeE2ESSEEvent,
	workspaceID string,
	worktreeID string,
	wantNames []string,
) {
	t.Helper()
	if len(events) != len(wantNames) {
		t.Fatalf("worktree events = %#v, want %d", events, len(wantNames))
	}
	lastSequence := int64(0)
	for index, event := range events {
		if event.Name != wantNames[index] {
			t.Fatalf("worktree event[%d] name = %q, want %q", index, event.Name, wantNames[index])
		}
		sequence, err := strconv.ParseInt(event.ID, 10, 64)
		if err != nil || sequence <= lastSequence {
			t.Fatalf("worktree event[%d] id = %q after %d; error=%v", index, event.ID, lastSequence, err)
		}
		lastSequence = sequence
		var payload struct {
			WorkspaceID string `json:"workspace_id"`
			WorktreeID  string `json:"worktree_id"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatalf("decode worktree event[%d]: %v; data=%s", index, err, event.Data)
		}
		if payload.WorkspaceID != workspaceID || payload.WorktreeID != worktreeID {
			t.Fatalf("worktree event[%d] payload = %#v, want %s/%s", index, payload, workspaceID, worktreeID)
		}
	}
}

func decodeWorktreeE2EStructured(t testing.TB, result any, dest any) {
	t.Helper()
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal native worktree result: %v", err)
	}
	var envelope struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode native worktree result envelope: %v; payload=%s", err, payload)
	}
	if len(envelope.StructuredContent) == 0 || string(envelope.StructuredContent) == "null" {
		t.Fatalf("native worktree result has no structured content: %s", payload)
	}
	if err := json.Unmarshal(envelope.StructuredContent, dest); err != nil {
		t.Fatalf("decode native worktree structured content: %v; payload=%s", err, envelope.StructuredContent)
	}
}

func worktreeE2EResultJSON(t testing.TB, result any) string {
	t.Helper()
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal native worktree result: %v", err)
	}
	return string(payload)
}
