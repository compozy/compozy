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
	sessionpkg "github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/worktree"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sys/execabs"
)

func TestDaemonWorktreeExitJourneyE2E001IT033IT037(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	root := initWorktreeE2ERepository(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	runWorktreeE2EGit(t, context.Background(), t.TempDir(), "init", "--bare", remote)
	runWorktreeE2EGit(t, context.Background(), root, "remote", "add", "origin", remote)
	runWorktreeE2EGit(t, context.Background(), root, "push", "-u", "origin", "main")

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: root},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:    "local-default",
			DefaultProvider: acpmock.ProviderName,
			PermissionMode:  config.PermissionModeApproveAll,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "network_local_fixture.json"),
			FixtureAgent: "local-default",
			AgentName:    "local-default",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	created := createWorktreeE2E(t, ctx, harness, harness.WorkspaceID, "exit-journey")
	ready := waitForWorktreeE2EState(
		t, ctx, harness, harness.WorkspaceID, created.ID, worktree.StateReady,
	)
	session, err := harness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName: "local-default", Name: "worktree-exit-journey",
		Workspace: harness.WorkspaceID, Worktree: created.ID,
	})
	if err != nil {
		t.Fatalf("CreateSession(bound worktree) error = %v", err)
	}
	session, err = harness.WaitForSessionActive(ctx, session.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(bound worktree) error = %v", err)
	}
	if session.WorktreeID != created.ID || session.WorkspaceID != harness.WorkspaceID {
		t.Fatalf("bound session = %#v, want workspace/worktree %s/%s", session, harness.WorkspaceID, created.ID)
	}
	if _, err := harness.PromptSession(ctx, session.ID, "write exit journey file"); err != nil {
		t.Fatalf("PromptSession(write worktree file) error = %v", err)
	}
	markerPath := filepath.Join(ready.Worktree.Path, "agent.txt")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read bound-agent marker: %v", err)
	}
	if got := string(marker); got != "written by bound agent\n" {
		t.Fatalf("bound-agent marker = %q", got)
	}

	path := worktreeE2EPath(harness.WorkspaceID, created.ID)
	var pausedPlan compozycontract.WorktreeExitPlanResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path+"/exit", nil, &pausedPlan); err != nil {
		t.Fatalf("HTTP exit plan while session active error = %v", err)
	}
	if pausedPlan.GlobalPauseCause == "" {
		t.Fatalf("active-session exit plan = %#v, want global pause", pausedPlan)
	}
	for _, action := range pausedPlan.Actions {
		if action.Enabled {
			t.Fatalf("active-session exit action %q remained enabled: %#v", action.Action, pausedPlan)
		}
	}

	var dirty compozycontract.WorktreeStatusResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path+"/status?refresh=true", nil, &dirty); err != nil {
		t.Fatalf("HTTP refreshed dirty status error = %v", err)
	}
	if dirty.Status.DirtyFiles == nil || *dirty.Status.DirtyFiles != 1 ||
		dirty.Status.Insertions == nil || *dirty.Status.Insertions != 0 {
		t.Fatalf("dirty status = %#v, want one untracked file", dirty.Status)
	}

	if err := harness.StopSession(ctx, session.ID); err != nil {
		t.Fatalf("StopSession(bound worktree) error = %v", err)
	}
	waitForWorktreeE2ESessionState(t, ctx, harness, session.ID, sessionpkg.StateStopped)
	waitForWorktreeE2EExitUnpaused(t, ctx, harness, path)

	var httpPlan, udsPlan, cliPlan compozycontract.WorktreeExitPlanResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path+"/exit", nil, &httpPlan); err != nil {
		t.Fatalf("HTTP exit plan error = %v", err)
	}
	if err := harness.UDSJSON(ctx, http.MethodGet, path+"/exit", nil, &udsPlan); err != nil {
		t.Fatalf("UDS exit plan error = %v", err)
	}
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &cliPlan, "worktree", "exit", created.ID,
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("CLI worktree exit error = %v", err)
	}
	assertWorktreeE2EJSONParity(t, "exit plan HTTP/UDS", httpPlan, udsPlan)
	assertWorktreeE2EJSONParity(t, "exit plan HTTP/CLI", httpPlan, cliPlan)
	if httpPlan.Primary != "commit" || !worktreeE2EExitActionEnabled(httpPlan, "commit_push") ||
		httpPlan.CommitScope.ChangedFiles != 1 ||
		httpPlan.CommitScope.UntrackedTotal != 1 ||
		len(httpPlan.CommitScope.UntrackedFiles) != 1 || httpPlan.CommitScope.UntrackedFiles[0] != "agent.txt" {
		t.Fatalf("actionable exit plan = %#v, want named commit_push scope", httpPlan)
	}

	readyStream := make(chan struct{})
	streamEvents := make(chan []worktreeE2ESSEEvent, 1)
	go func() {
		streamEvents <- readWorktreeE2ESSEUntil(
			t, ctx, harness, path+"/stream?after_sequence=0", readyStream,
			func(events []worktreeE2ESSEEvent) bool {
				if len(events) == 0 {
					return false
				}
				switch events[len(events)-1].Name {
				case worktree.EventExitActionCompleted,
					worktree.EventExitActionFailed,
					worktree.EventExitActionCanceled:
					return true
				default:
					return false
				}
			},
		)
	}()
	select {
	case <-readyStream:
	case <-ctx.Done():
		t.Fatalf("wait for exit stream readiness: %v", ctx.Err())
	}

	var operation compozycontract.WorktreeExitOperationResponse
	if err := harness.HTTPJSON(
		ctx, http.MethodPost, path+"/exit/actions",
		compozycontract.RunWorktreeExitActionRequest{Action: "commit_push"}, &operation,
	); err != nil {
		t.Fatalf("HTTP commit_push action error = %v", err)
	}
	if operation.OperationID == "" {
		t.Fatal("commit_push operation ID is empty")
	}
	var events []worktreeE2ESSEEvent
	select {
	case events = <-streamEvents:
	case <-ctx.Done():
		t.Fatalf("wait for exit completion stream: %v", ctx.Err())
	}
	assertWorktreeE2EExitEvents(t, events, operation.OperationID)

	localHead := strings.TrimSpace(runWorktreeE2EGit(t, ctx, ready.Worktree.Path, "rev-parse", "HEAD"))
	remoteHead := strings.TrimSpace(runWorktreeE2EGit(
		t, ctx, root, "--git-dir="+remote, "rev-parse", "refs/heads/exit-journey",
	))
	if localHead != remoteHead {
		t.Fatalf("local/remote exit branch heads = %q/%q", localHead, remoteHead)
	}
	if got := runWorktreeE2EGit(
		t,
		ctx,
		root,
		"--git-dir="+remote,
		"show",
		remoteHead+":agent.txt",
	); got != "written by bound agent\n" {
		t.Fatalf("remote committed marker = %q", got)
	}

	var clean compozycontract.WorktreeStatusResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, path+"/status?refresh=true", nil, &clean); err != nil {
		t.Fatalf("UDS refreshed clean status error = %v", err)
	}
	if clean.Status.DirtyFiles == nil || *clean.Status.DirtyFiles != 0 ||
		clean.Status.Ahead == nil || *clean.Status.Ahead != 0 {
		t.Fatalf("post-push status = %#v, want clean and synchronized", clean.Status)
	}
	if err := harness.UDSJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		t.Fatalf("UDS clean removal error = %v", err)
	}
	if _, err := os.Stat(ready.Worktree.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed worktree path stat error = %v, want not-exist", err)
	}
	branchHead := strings.TrimSpace(runWorktreeE2EGit(t, ctx, root, "rev-parse", "refs/heads/exit-journey"))
	if branchHead != localHead {
		t.Fatalf("surviving branch head = %q, want %q", branchHead, localHead)
	}
	if got := runWorktreeE2EGit(t, ctx, root, "show", "exit-journey:agent.txt"); got != "written by bound agent\n" {
		t.Fatalf("surviving branch history marker = %q", got)
	}
}

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

	claimer := createBoundFixtureBackedSession(t, ctx, harness, "local-default", "per-run-claimer")
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
	if run.ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModePerRun || run.WorktreeID != "" {
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

	registration, ok := harness.MockAgentRegistration("local-default")
	if !ok {
		t.Fatal("MockAgentRegistration(local-default) = missing, want present")
	}
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(per-run claimer) error = %v", err)
	}
	claimerDiagnostics := acpmock.DiagnosticsForCompozySession(diagnostics, claimer.ID)
	client := startHostedMCPClient(
		t,
		ctx,
		requireHostedMCPStdioServer(t, claimerDiagnostics, hostedMCPServerEarliest),
	)
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("Close(hosted MCP client) error = %v", closeErr)
		}
	}()
	claimCall := &sdkmcp.CallToolParams{
		Name: toolspkg.ToolIDTaskRunClaimNext.String(),
		Arguments: map[string]any{
			"run_id": run.ID,
		},
	}
	claimResult, err := client.CallTool(ctx, claimCall)
	if err != nil {
		storedRuns, listErr := harness.ListTaskRuns(ctx, taskRecord.ID, nil)
		daemonLog, logErr := os.ReadFile(filepath.Join(harness.Artifacts.RootDir(), "daemon-process.log"))
		t.Fatalf(
			"CallTool(task_run_claim_next %s) error = %v; runs=%#v list_error=%v log_error=%v daemon_log=%s",
			run.ID,
			err,
			storedRuns,
			listErr,
			logErr,
			daemonLog,
		)
	}
	if claimResult == nil || claimResult.IsError {
		t.Fatalf("CallTool(task_run_claim_next) result = %#v, want success", claimResult)
	}
	structuredClaim, err := json.Marshal(claimResult.StructuredContent)
	if err != nil {
		t.Fatalf("json.Marshal(task_run_claim_next structured content) error = %v", err)
	}
	var claimEnvelope struct {
		Claimed bool                                  `json:"claimed"`
		Claim   compozycontract.AgentTaskClaimPayload `json:"claim"`
	}
	if err := json.Unmarshal(structuredClaim, &claimEnvelope); err != nil {
		t.Fatalf("json.Unmarshal(task_run_claim_next structured content) error = %v; content=%s", err, structuredClaim)
	}
	started := claimEnvelope.Claim.Run
	if !claimEnvelope.Claimed || started.Status.Normalize() != taskpkg.TaskRunStatusRunning {
		t.Fatalf("native claim = %#v, want claimed running run", claimEnvelope)
	}
	if started.ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModePerRun || started.WorktreeID == "" {
		t.Fatalf("started run worktree = %#v, want frozen per_run materialization", started)
	}
	taskSession, err := harness.GetSession(ctx, started.SessionID)
	if err != nil {
		t.Fatalf("GetSession(%s) error = %v", started.SessionID, err)
	}
	if taskSession.WorktreeID != started.WorktreeID {
		t.Fatalf("session worktree_id = %q, want run worktree_id %q", taskSession.WorktreeID, started.WorktreeID)
	}
	if _, err := harness.PromptSession(ctx, claimer.ID, "local session probe"); err != nil {
		t.Fatalf("PromptSession(bootstrap turn end) error = %v", err)
	}
	waitForWorktreeE2ESessionState(t, ctx, harness, claimer.ID, sessionpkg.StateStopped)

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
		&taskSession,
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
		runs[0].ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModePerRun {
		t.Fatalf("terminal run history = %#v, want durable worktree binding", runs)
	}
	waitForWorktreeE2EState(t, ctx, harness, harness.WorkspaceID, started.WorktreeID, worktree.StateReady)

	secondClaimer := createFixtureBackedSession(t, ctx, harness, "local-default", "root-claimer")
	rootTask := createTaskViaUDS(t, ctx, harness, "Root snapshot authority")
	rootRun := enqueueTaskRunViaUDS(t, ctx, harness, rootTask.ID, "")
	if rootRun.ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModeNone {
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
	if _, err := harness.ClaimExactTaskRunForSession(ctx, rootRun.ID, &secondClaimer); err != nil {
		t.Fatalf("ClaimExactTaskRunForSession(root %s) error = %v", rootRun.ID, err)
	}
	rootStarted, err := harness.StartClaimedTaskRunForSession(
		ctx,
		rootRun.ID,
		&secondClaimer,
		compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + rootRun.ID},
	)
	if err != nil {
		t.Fatalf("StartTaskRun(root %s) error = %v", rootRun.ID, err)
	}
	if rootStarted.ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModeNone || rootStarted.WorktreeID != "" {
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
		if queued.ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModePerRun ||
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
		if _, err := harness.ClaimExactTaskRunForSession(ctx, queued.ID, &claimer); err != nil {
			t.Fatalf("ClaimExactTaskRunForSession(%s) error = %v", queued.ID, err)
		}
		started, err := harness.StartClaimedTaskRunForSession(
			ctx,
			queued.ID,
			&claimer,
			compozycontract.StartTaskRunRequest{IdempotencyKey: "start-" + queued.ID},
		)
		if err != nil {
			t.Fatalf("StartTaskRun(%s) error = %v", queued.ID, err)
		}
		if started.WorktreeID == "" || started.ResolvedWorktreeMode != compozycontract.ResolvedWorktreeModePerRun {
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
			&taskSession,
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
		if _, err := harness.ClaimExactTaskRunForSession(ctx, queued.ID, &claimer); err != nil {
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
				&claimer,
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
			&claimer,
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
			&taskSession,
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
		ctx, root, &inspected, "worktree", "inspect", created.Name,
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
		ctx, root, &status, "worktree", "status", created.Name, "--refresh",
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
	cliRefusal := worktreeE2ECLIRemovalRefusal(t, ctx, harness, root, harness.WorkspaceID, created.Name)
	var apiRefusal compozycontract.WorktreeRemovalRefusalPayload
	statusCode := loopRuntimeRawJSON(
		t, ctx, harness.HTTPClient,
		harness.HTTPURL(worktreeE2EPath(harness.WorkspaceID, created.Name)),
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

	var reusable compozycontract.WorktreePayload
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &reusable, "worktree", "create", "cli-name-reuse",
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree create(name reuse fixture) error = %v", err)
	}
	waitForWorktreeE2EState(t, ctx, harness, harness.WorkspaceID, reusable.ID, worktree.StateReady)
	var removedByName struct {
		Action   string                          `json:"action"`
		Worktree compozycontract.WorktreePayload `json:"worktree"`
	}
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &removedByName, "worktree", "remove", reusable.Name,
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree remove by name error = %v", err)
	}
	if removedByName.Action != "removed" || removedByName.Worktree.ID != reusable.ID {
		t.Fatalf("worktree remove by name = %#v", removedByName)
	}
	var dismissedByName struct {
		Action   string                          `json:"action"`
		Worktree compozycontract.WorktreePayload `json:"worktree"`
	}
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &dismissedByName, "worktree", "dismiss", reusable.Name,
		"--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree dismiss by name error = %v", err)
	}
	if dismissedByName.Action != "dismissed" || dismissedByName.Worktree.ID != reusable.ID {
		t.Fatalf("worktree dismiss by name = %#v", dismissedByName)
	}
	var recreated compozycontract.WorktreePayload
	if err := harness.CLI.RunJSONInDir(
		ctx, root, &recreated, "worktree", "create", reusable.Name,
		"--existing-branch", reusable.Branch, "--workspace", harness.WorkspaceID, "-o", "json",
	); err != nil {
		t.Fatalf("worktree recreate dismissed name error = %v", err)
	}
	if recreated.ID == reusable.ID || recreated.Name != reusable.Name {
		t.Fatalf("recreated worktree = %#v, want a new id for %q", recreated, reusable.Name)
	}
	waitForWorktreeE2EState(t, ctx, harness, harness.WorkspaceID, recreated.ID, worktree.StateReady)

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
				name        string
				workspaceID string
				wantID      string
				rejectID    string
			}{
				{name: "workspace A", workspaceID: workspaceA, wantID: itemA.ID, rejectID: itemB.ID},
				{name: "workspace B", workspaceID: workspaceB, wantID: itemB.ID, rejectID: itemA.ID},
			} {
				t.Run("Should isolate "+target.name, func(t *testing.T) {
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
				})
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
		map[string]any{"workspace": harness.WorkspaceID, "ref": created.Worktree.Name},
		"allow-once",
	)
	var refusal compozycontract.WorktreeRemovalRefusalPayload
	decodeWorktreeE2EStructured(t, removeResult, &refusal)
	if !removeResult.IsError || refusal.Code != worktree.ErrDirtyRequiresForce.Error() ||
		refusal.Risk.ChangedFiles == 0 {
		t.Fatalf("native worktree removal refusal = %#v / %#v", removeResult, refusal)
	}
	cliRefusal := worktreeE2ECLIRemovalRefusal(
		t, ctx, harness, root, workspaceID, created.Worktree.Name,
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

func runWorktreeE2EGit(
	t testing.TB,
	ctx context.Context,
	dir string,
	args ...string,
) string {
	t.Helper()
	command := execabs.CommandContext(ctx, "git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %q error = %v; output=%s", strings.Join(args, " "), dir, err, output)
	}
	return string(output)
}

func waitForWorktreeE2ESessionState(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	want sessionpkg.State,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := harness.GetSession(ctx, sessionID)
		if err == nil && current.State == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf(
				"wait for session %q state %q: current=%#v error=%v context=%v",
				sessionID,
				want,
				current,
				err,
				ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func waitForWorktreeE2EExitUnpaused(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var plan compozycontract.WorktreeExitPlanResponse
		err := harness.HTTPJSON(ctx, http.MethodGet, path+"/exit", nil, &plan)
		if err == nil && plan.GlobalPauseCause == "" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for unpaused worktree exit plan: plan=%#v error=%v context=%v", plan, err, ctx.Err())
		case <-ticker.C:
		}
	}
}

func worktreeE2EExitActionEnabled(plan compozycontract.WorktreeExitPlanResponse, action string) bool {
	for _, row := range plan.Actions {
		if string(row.Action) == action {
			return row.Enabled
		}
	}
	return false
}

func assertWorktreeE2EJSONParity(t testing.TB, name string, got any, want any) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal %s got payload: %v", name, err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal %s want payload: %v", name, err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("%s differs:\ngot:  %s\nwant: %s", name, gotJSON, wantJSON)
	}
}

func assertWorktreeE2EExitEvents(t testing.TB, events []worktreeE2ESSEEvent, operationID string) {
	t.Helper()
	phaseStates := make(map[worktree.ExitPhase][]string)
	var started, completed bool
	startedAt := -1
	for index, event := range events {
		switch event.Name {
		case worktree.EventExitActionStarted,
			worktree.EventExitActionStep,
			worktree.EventExitActionCompleted,
			worktree.EventExitActionFailed,
			worktree.EventExitActionCanceled:
		default:
			continue
		}
		var payload worktree.ExitEventPayload
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatalf("decode exit event %q: %v; data=%s", event.Name, err, event.Data)
		}
		if payload.OperationID != operationID {
			t.Fatalf("exit event %q op_id = %q, want %q", event.Name, payload.OperationID, operationID)
		}
		switch event.Name {
		case worktree.EventExitActionStarted:
			started = true
			startedAt = index
			if payload.Action != worktree.ExitActionCommitPush || payload.State != "running" ||
				len(payload.Phases) != 2 || payload.Phases[0] != worktree.ExitPhaseCommit ||
				payload.Phases[1] != worktree.ExitPhasePush {
				t.Fatalf("exit started payload = %#v", payload)
			}
		case worktree.EventExitActionStep:
			phaseStates[payload.Phase] = append(phaseStates[payload.Phase], payload.State)
		case worktree.EventExitActionCompleted:
			completed = true
			if payload.State != "completed" || payload.Result == nil || len(payload.Result.Steps) != 2 ||
				payload.Result.CTA == nil {
				t.Fatalf("exit completed payload = %#v", payload)
			}
		case worktree.EventExitActionFailed, worktree.EventExitActionCanceled:
			t.Fatalf("exit action terminated as %q: %#v", event.Name, payload)
		}
	}
	if !started || !completed {
		t.Fatalf("exit events missing start/completion: %#v", events)
	}
	for _, phase := range []worktree.ExitPhase{worktree.ExitPhaseCommit, worktree.ExitPhasePush} {
		states := phaseStates[phase]
		if len(states) != 2 || states[0] != "running" || states[1] != "completed" {
			t.Fatalf("exit phase %q states = %#v, want running/completed", phase, states)
		}
	}
	refreshedAfterStart := false
	for index := startedAt + 1; index < len(events); index++ {
		if events[index].Name == worktree.EventStatusRefreshed {
			refreshedAfterStart = true
			break
		}
	}
	if !refreshedAfterStart {
		t.Fatalf("exit events have no post-action status refresh: %#v", events)
	}
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
	worktreeRef string,
) compozycontract.WorktreeRemovalRefusalPayload {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDir(
		ctx, root, "worktree", "remove", worktreeRef,
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

func worktreeE2EPath(workspaceID string, worktreeRef string) string {
	return "/api/workspaces/" + url.PathEscape(workspaceID) + "/worktrees/" + url.PathEscape(worktreeRef)
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
	return readWorktreeE2ESSEUntil(t, ctx, harness, path, ready, func(events []worktreeE2ESSEEvent) bool {
		return len(events) >= want
	})
}

func readWorktreeE2ESSEUntil(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	ready chan<- struct{},
	done func([]worktreeE2ESSEEvent) bool,
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
		return done(events)
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
	t.Fatalf("worktree SSE %q ended before the expected terminal event; events=%#v", path, events)
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
