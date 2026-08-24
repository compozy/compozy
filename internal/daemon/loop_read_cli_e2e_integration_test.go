//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const (
	loopReadApprovalLoopName   = "loop-read-approval"
	loopReadQuarantineLoopName = "loop-read-quarantine"
	loopReadWaitingLoopName    = "loop-read-waiting"
)

func TestDaemonE2ELoopRunReadCLIJourneys(t *testing.T) {
	t.Parallel()
	acpmock.RequireDriver(t)
	workspaceRoot := t.TempDir()
	seedLoopNodeLifecycleDefinitions(t, workspaceRoot)
	seedLoopReadDefinitions(t, workspaceRoot)
	loopEventsFixture := mockFixturePath(t, "loop_events_fixture.json")
	lifecycleFixture := mockFixturePath(t, "loop_node_lifecycle_fixture.json")
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath:  loopEventsFixture,
				FixtureAgent: "loop_events",
				AgentName:    "loop-events-agent",
			},
			{
				FixturePath:  lifecycleFixture,
				FixtureAgent: "lifecycle_retry",
				AgentName:    "lifecycle-retry-agent",
			},
			{
				FixturePath:  lifecycleFixture,
				FixtureAgent: "lifecycle_quarantine",
				AgentName:    "lifecycle-quarantine-agent",
			},
		},
	})
	setupCtx, setupCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer setupCancel()
	createLoopViaHTTP(t, setupCtx, harness, loopEventsDefinition())
	waitForLoopCatalogEntry(t, setupCtx, harness, loopReadApprovalLoopName)
	waitForLoopCatalogEntry(t, setupCtx, harness, loopReadQuarantineLoopName)
	waitForLoopCatalogEntry(t, setupCtx, harness, loopReadWaitingLoopName)
	run := runLoopViaHTTP(t, setupCtx, harness, "loop-events-probe")
	waitForLoopRunStatus(t, setupCtx, harness, run.ID, compozycontract.LoopRunStatusDone)
	setupCancel()
	workspace := harness.WorkspaceRoot
	t.Run("Should execute the golden run-read path E2E-001", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		if _, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"configure",
			"--name",
			loopReadApprovalLoopName,
			"--set",
			"human_gate_enabled=true",
			"--workspace",
			workspace,
		); err != nil {
			t.Fatalf("configure golden-path human gate error = %v stderr=%s", err, stderr)
		}
		started, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"run",
			"--name",
			loopReadApprovalLoopName,
			"--workspace",
			workspace,
		)
		if err != nil {
			t.Fatalf("golden-path loop run error = %v stderr=%s", err, stderr)
		}
		approvalRunID := assertGoldenLoopRunTranscript(t, started, loopReadApprovalLoopName)
		waitForLoopRosterNodeState(t, ctx, harness, approvalRunID, "prepare", looppkg.NodeStateRunning)

		taskList, stderr, err := harness.CLI.Run(
			ctx,
			"task",
			"list",
			"--workspace",
			workspace,
		)
		if err != nil {
			t.Fatalf("calm task list error = %v stderr=%s", err, stderr)
		}
		assertCalmTaskListTranscript(t, taskList, approvalRunID)
		calmJSON, stderr, err := harness.CLI.Run(
			ctx,
			"task",
			"list",
			"--workspace",
			workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("structured calm task list error = %v stderr=%s", err, stderr)
		}
		if strings.Contains(calmJSON, approvalRunID) {
			t.Fatalf("calm task list exposed Loop records: %s", taskList)
		}

		healthyTranscript, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"why",
			approvalRunID,
			"--workspace",
			workspace,
		)
		if err != nil {
			t.Fatalf("healthy human loop why error = %v stderr=%s", err, stderr)
		}
		assertHealthyWhyTranscript(t, healthyTranscript, approvalRunID)
		var healthy compozycontract.LoopBriefingResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&healthy,
			"loop",
			"why",
			approvalRunID,
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("healthy loop why error = %v", err)
		}
		if healthy.RunID != looppkg.RunID(approvalRunID) || healthy.Tone != looppkg.BriefingToneOK ||
			len(healthy.Blockers) != 0 {
			t.Fatalf("healthy briefing = %#v", healthy)
		}

		waitForLoopRunStatus(t, ctx, harness, approvalRunID, compozycontract.LoopRunStatusNeedsApproval)
		needsYouTranscript, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"why",
			approvalRunID,
			"--workspace",
			workspace,
		)
		if err != nil {
			t.Fatalf("needs-you human loop why error = %v stderr=%s", err, stderr)
		}
		assertNeedsYouWhyTranscript(t, needsYouTranscript, approvalRunID, harness.WorkspaceID, "approval")
		var briefing compozycontract.LoopBriefingResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&briefing,
			"loop",
			"why",
			approvalRunID,
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("approval loop why error = %v", err)
		}
		if briefing.RunID != looppkg.RunID(approvalRunID) ||
			briefing.Tone != looppkg.BriefingToneNeedsYou ||
			len(briefing.Blockers) == 0 {
			t.Fatalf("approval briefing = %#v", briefing)
		}
		if _, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"approve",
			approvalRunID,
			"--gate",
			"approval",
			"--workspace",
			workspace,
		); err != nil {
			t.Fatalf("loop approve error = %v stderr=%s", err, stderr)
		}
		waitForLoopRunStatus(t, ctx, harness, approvalRunID, compozycontract.LoopRunStatusDone)
		terminalTranscript, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"why",
			approvalRunID,
			"--workspace",
			workspace,
		)
		if err != nil {
			t.Fatalf("terminal human loop why error = %v stderr=%s", err, stderr)
		}
		assertTerminalWhyTranscript(t, terminalTranscript)

		settled, stderr, err := harness.CLI.Run(
			ctx,
			"task",
			"list",
			"--include-loop",
			"--loop-run",
			approvalRunID,
			"--workspace",
			workspace,
		)
		if err != nil {
			t.Fatalf("settled task list error = %v stderr=%s", err, stderr)
		}
		assertSettledTaskListTranscript(t, settled)
		settledJSON, stderr, err := harness.CLI.Run(
			ctx,
			"task",
			"list",
			"--include-loop",
			"--loop-run",
			approvalRunID,
			"--workspace",
			workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("structured settled task list error = %v stderr=%s", err, stderr)
		}
		assertSettledLoopTaskList(t, settledJSON, approvalRunID)
	})
	t.Run("Should disconnect resume and wait for the first event E2E-003", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		quietRun := runLoopWithHumanGate(t, ctx, harness)
		waitForLoopRunStatus(t, ctx, harness, quietRun.ID, compozycontract.LoopRunStatusNeedsApproval)
		var initial compozycontract.LoopTimelineResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&initial,
			"loop",
			"events",
			quietRun.ID,
			"--limit",
			"500",
			"--view",
			"all",
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("initial timeline error = %v", err)
		}

		type followResult struct {
			output string
			stderr string
			err    error
		}
		quietResult := make(chan followResult, 1)
		go func() {
			output, stderr, err := harness.CLI.Run(
				ctx,
				"loop",
				"events",
				quietRun.ID,
				"--after",
				strconv.FormatInt(initial.HeadSeq, 10),
				"--follow",
				"--view",
				"all",
				"--workspace",
				workspace,
				"-o",
				"jsonl",
			)
			quietResult <- followResult{output: output, stderr: stderr, err: err}
		}()
		select {
		case result := <-quietResult:
			t.Fatalf("eventless follow returned before the first event: %#v", result)
		case <-time.After(300 * time.Millisecond):
		}
		if _, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"approve",
			quietRun.ID,
			"--gate",
			"approval",
			"--workspace",
			workspace,
		); err != nil {
			t.Fatalf("approve eventless-follow run error = %v stderr=%s", err, stderr)
		}
		select {
		case result := <-quietResult:
			if result.err != nil || len(timelineJSONLSequences(t, result.output)) == 0 {
				t.Fatalf("eventless follow result = %#v, want first event and terminal exit 0", result)
			}
		case <-ctx.Done():
			t.Fatalf("eventless follow did not observe its first event: %v", ctx.Err())
		}

		followRun := runLoopWithHumanGate(t, ctx, harness)
		waitForLoopRunStatus(t, ctx, harness, followRun.ID, compozycontract.LoopRunStatusNeedsApproval)
		disconnectCtx, disconnectCancel := context.WithTimeout(ctx, 300*time.Millisecond)
		beforeDisconnect, disconnectStderr, disconnectErr := harness.CLI.Run(
			disconnectCtx,
			"loop",
			"events",
			followRun.ID,
			"--follow",
			"--view",
			"all",
			"--workspace",
			workspace,
			"-o",
			"jsonl",
		)
		disconnectCause := disconnectCtx.Err()
		disconnectCancel()
		if disconnectErr == nil {
			t.Fatal("mid-run follow returned before the forced disconnect")
		}
		if ctx.Err() != nil || !errors.Is(disconnectCause, context.DeadlineExceeded) {
			t.Fatalf(
				"mid-run follow disconnect cause = %v parent=%v stderr=%s, want local deadline",
				disconnectCause,
				ctx.Err(),
				disconnectStderr,
			)
		}
		beforeSequences := timelineJSONLSequences(t, beforeDisconnect)
		if len(beforeSequences) == 0 {
			t.Fatal("mid-run follow emitted no durable prefix before disconnect")
		}
		resumeAfter := beforeSequences[len(beforeSequences)-1]

		if _, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"approve",
			followRun.ID,
			"--gate",
			"approval",
			"--workspace",
			workspace,
		); err != nil {
			t.Fatalf("approve during disconnected follow error = %v stderr=%s", err, stderr)
		}
		resumed, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"events",
			followRun.ID,
			"--after",
			strconv.FormatInt(resumeAfter, 10),
			"--follow",
			"--view",
			"all",
			"--workspace",
			workspace,
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("loop events follow error = %v stderr=%s", err, stderr)
		}
		combined := append(beforeSequences, timelineJSONLSequences(t, resumed)...)
		waitForLoopRunStatus(t, ctx, harness, followRun.ID, compozycontract.LoopRunStatusDone)
		var complete compozycontract.LoopTimelineResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&complete,
			"loop",
			"events",
			followRun.ID,
			"--limit",
			"500",
			"--view",
			"all",
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("complete timeline error = %v", err)
		}
		assertTimelineResumeParity(t, combined, complete.Entries)
	})
	t.Run("Should execute the returned unblocker and clear the blocker E2E-004", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		blockedRun := runLoopWithHumanGate(t, ctx, harness)
		waitForLoopRunStatus(t, ctx, harness, blockedRun.ID, compozycontract.LoopRunStatusNeedsApproval)
		var briefing compozycontract.LoopBriefingResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&briefing,
			"loop",
			"why",
			blockedRun.ID,
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("loop why error = %v", err)
		}
		if len(briefing.Blockers) != 1 || briefing.Blockers[0].Unblocker == "" {
			t.Fatalf("briefing blockers = %#v", briefing.Blockers)
		}
		command := strings.Fields(briefing.Blockers[0].Unblocker)
		if len(command) < 2 || command[0] != "compozy" {
			t.Fatalf("unblocker = %q, want executable compozy command", briefing.Blockers[0].Unblocker)
		}
		if _, stderr, err := harness.CLI.Run(ctx, command[1:]...); err != nil {
			t.Fatalf("execute unblocker %q error = %v stderr=%s", briefing.Blockers[0].Unblocker, err, stderr)
		}
		waitForLoopRunStatus(t, ctx, harness, blockedRun.ID, compozycontract.LoopRunStatusDone)
		var after compozycontract.LoopBriefingResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&after,
			"loop",
			"why",
			blockedRun.ID,
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("loop why after unblock error = %v", err)
		}
		if len(after.Blockers) != 0 || after.Progress.StepsDone <= briefing.Progress.StepsDone {
			t.Fatalf("briefing before/after = %#v/%#v", briefing, after)
		}
	})
	t.Run("Should expose settled attempts through the roster E2E-005", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		retryRun := runLoopViaHTTP(t, ctx, harness, lifecycleRetryLoopName)
		waitForLoopRunStatus(t, ctx, harness, retryRun.ID, compozycontract.LoopRunStatusDone)
		var roster compozycontract.LoopRunNodesResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&roster,
			"loop",
			"nodes",
			"--run",
			retryRun.ID,
			"--all",
			"--state",
			"all",
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("loop nodes error = %v", err)
		}
		if len(roster.Nodes) != 1 || roster.Nodes[0].Attempt != 2 ||
			len(roster.Nodes[0].Attempts) != 2 ||
			roster.Nodes[0].State != looppkg.NodeStateSucceeded {
			t.Fatalf("roster = %#v", roster)
		}
	})
	t.Run("Should reject deterministic invalid positions E2E-006", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		_, unknownStderr, unknownErr := harness.CLI.Run(
			ctx,
			"loop",
			"why",
			"looprun-missing",
			"--workspace",
			workspace,
		)
		assertLoopReadCLIError(
			t,
			unknownErr,
			unknownStderr,
			1,
			"error: loop run \"looprun-missing\" not found\n",
		)

		_, guardStderr, guardErr := harness.CLI.Run(
			ctx,
			"loop",
			"nodes",
			"--state",
			"running",
			"--workspace",
			workspace,
		)
		assertLoopReadCLIError(
			t,
			guardErr,
			guardStderr,
			2,
			"error: --state running requires --run "+
				"(workspace inventory tracks exception states only)\n",
		)

		var timeline compozycontract.LoopTimelineResponse
		if err := harness.CLI.RunJSON(
			ctx,
			&timeline,
			"loop",
			"events",
			run.ID,
			"--workspace",
			workspace,
			"-o",
			"json",
		); err != nil {
			t.Fatalf("read timeline head for E2E-006 error = %v", err)
		}
		beyond := timeline.HeadSeq + 1
		_, stderr, err := harness.CLI.Run(
			ctx,
			"loop",
			"events",
			run.ID,
			"--after",
			strconv.FormatInt(beyond, 10),
			"--workspace",
			workspace,
			"-o",
			"jsonl",
		)
		assertLoopReadCLIError(
			t,
			err,
			stderr,
			1,
			fmt.Sprintf(
				"error: position %d is beyond this run's history (head: %d)\n",
				beyond,
				timeline.HeadSeq,
			),
		)
	})
	t.Run("Should return the exact timeline branch conflict over real SQL IT-022", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		first := runLoopWithHumanGate(t, ctx, harness)
		second := runLoopWithHumanGate(t, ctx, harness)
		waitForLoopRunStatus(t, ctx, harness, first.ID, compozycontract.LoopRunStatusNeedsApproval)
		waitForLoopRunStatus(t, ctx, harness, second.ID, compozycontract.LoopRunStatusNeedsApproval)

		var firstPage compozycontract.LoopTimelineResponse
		firstPath := loopReadRunPath(harness.WorkspaceID, first.ID) + "/timeline?limit=1&view=all"
		if err := harness.HTTPJSON(ctx, http.MethodGet, firstPath, nil, &firstPage); err != nil {
			t.Fatalf("HTTP first timeline page error = %v", err)
		}
		if firstPage.NextCursor == "" {
			t.Fatalf("first timeline page = %#v, want cursor", firstPage)
		}
		conflictPath := loopReadRunPath(harness.WorkspaceID, second.ID) + "/timeline?view=all&cursor=" +
			url.QueryEscape(firstPage.NextCursor)
		status, body := loopReadRawGET(t, ctx, harness.HTTPClient, harness.HTTPURL(conflictPath))
		if status != http.StatusConflict ||
			body != "{\"error\":\"timeline_branch_changed\",\"code\":\"timeline_branch_changed\"}" {
			t.Fatalf("timeline branch response = status %d body %q", status, body)
		}
	})
	t.Run("Should reflect a real node verb in the roster and reject its stale replay IT-027", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		quarantinedRun := runLoopViaHTTP(t, ctx, harness, loopReadQuarantineLoopName)
		waitForLoopRosterNodeState(
			t,
			ctx,
			harness,
			quarantinedRun.ID,
			"primary",
			looppkg.NodeStateQuarantined,
		)

		streamReady := make(chan struct{})
		streamResult := observeLoopReadSSE(
			ctx,
			harness,
			loopRunEventsPath(harness.WorkspaceID, quarantinedRun.ID, 0),
			streamReady,
		)
		select {
		case <-streamReady:
		case <-ctx.Done():
			t.Fatalf("open Loop SSE before requeue: %v", ctx.Err())
		}

		requeuePath := loopReadRunPath(harness.WorkspaceID, quarantinedRun.ID) + "/nodes/primary/requeue"
		request := compozycontract.LoopNodeMutationRequest{Reason: "operator repaired the target"}
		var mutation compozycontract.LoopMutationResponse
		if err := harness.HTTPJSON(ctx, http.MethodPost, requeuePath, request, &mutation); err != nil {
			t.Fatalf("HTTP requeue Loop node error = %v", err)
		}
		if !mutation.OK || mutation.NodeID != "primary" || mutation.Control == nil || mutation.Control.Quarantined {
			t.Fatalf("requeue mutation = %#v, want cleared primary quarantine", mutation)
		}

		observed := <-streamResult
		if observed.err != nil {
			t.Fatalf("observe node_requeued SSE error = %v", observed.err)
		}
		assertLoopReadRequeueSSE(t, observed.events, harness.WorkspaceID, quarantinedRun.ID)

		var roster compozycontract.LoopRunNodesResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			loopReadRunPath(harness.WorkspaceID, quarantinedRun.ID)+"/nodes?state=all&limit=500",
			nil,
			&roster,
		); err != nil {
			t.Fatalf("read roster on first poll after requeue error = %v", err)
		}
		assertRequeuedGenerationVisibleOnFirstPoll(t, roster, "primary", 3)

		status, body := loopReadRawJSON(
			t,
			ctx,
			harness.HTTPClient,
			harness.HTTPURL(requeuePath),
			http.MethodPost,
			compozycontract.LoopNodeMutationRequest{Reason: "stale replay"},
		)
		var conflict compozycontract.ErrorPayload
		if err := json.Unmarshal([]byte(body), &conflict); err != nil {
			t.Fatalf("decode stale requeue conflict body error = %v; body=%s", err, body)
		}
		assertStaleRequeueConflict(t, status, body, conflict, "primary", request.Reason)
	})
	t.Run("Should preserve ordered run summaries across HTTP UDS and CLI pages IT-032", func(t *testing.T) {
		ctx := loopReadJourneyContext(t)
		needsYou := runLoopWithHumanGate(t, ctx, harness)
		waitForLoopRunStatus(t, ctx, harness, needsYou.ID, compozycontract.LoopRunStatusNeedsApproval)
		activeIDs := make([]string, 0, 3)
		for range 3 {
			active := runLoopViaHTTP(t, ctx, harness, loopReadWaitingLoopName)
			activeIDs = append(activeIDs, active.ID)
			waitForLoopRunStatus(t, ctx, harness, active.ID, compozycontract.LoopRunStatusWatching)
		}

		httpRuns := drainLoopRunsTransport(t, ctx, harness, "http", 2)
		udsRuns := drainLoopRunsTransport(t, ctx, harness, "uds", 2)
		cliRuns := drainLoopRunsCLI(t, ctx, harness, 2)
		want := loopRunReadProjections(httpRuns)
		if got := loopRunReadProjections(udsRuns); !reflect.DeepEqual(got, want) {
			t.Fatalf("UDS run projections = %#v, want HTTP %#v", got, want)
		}
		if got := loopRunReadProjections(cliRuns); !reflect.DeepEqual(got, want) {
			t.Fatalf("CLI run projections = %#v, want HTTP %#v", got, want)
		}
		repeated := loopRunReadProjections(drainLoopRunsTransport(t, ctx, harness, "http", 2))
		if !reflect.DeepEqual(repeated, want) {
			t.Fatalf("repeated HTTP cursor walk = %#v, want %#v", repeated, want)
		}
		assertLoopRunReadOrdering(t, httpRuns)
		assertLoopRunReadSummary(t, httpRuns, needsYou.ID, "approval", 1, 1, 2)
		for _, activeID := range activeIDs {
			assertLoopRunReadSummary(t, httpRuns, activeID, "", 0, 0, 1)
		}
	})
}

func loopReadJourneyContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func assertLoopReadCLIError(
	t testing.TB,
	err error,
	stderr string,
	wantExit int,
	wantStderr string,
) {
	t.Helper()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != wantExit || stderr != wantStderr {
		t.Fatalf(
			"CLI error = %T %[1]v, exit=%v, stderr=%q; want exit %d stderr %q",
			err,
			exitErr,
			stderr,
			wantExit,
			wantStderr,
		)
	}
}

type loopReadSSEObservation struct {
	events []loopRunSSEEvent
	err    error
}

func observeLoopReadSSE(
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	ready chan<- struct{},
) <-chan loopReadSSEObservation {
	result := make(chan loopReadSSEObservation, 1)
	go func() {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
		if err != nil {
			close(ready)
			result <- loopReadSSEObservation{err: err}
			return
		}
		response, err := harness.HTTPClient.Do(request)
		if err != nil {
			close(ready)
			result <- loopReadSSEObservation{err: err}
			return
		}
		close(ready)
		if response.StatusCode != http.StatusOK {
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			result <- loopReadSSEObservation{err: errors.Join(
				fmt.Errorf("Loop SSE status %d: %s", response.StatusCode, bytes.TrimSpace(body)),
				readErr,
				closeErr,
			)}
			return
		}
		events, readErr := readLoopRunSSERecords(response.Body, func(events []loopRunSSEEvent) bool {
			for _, event := range events {
				if event.Kind == compozycontract.LoopRunEventNodeRequeued {
					return true
				}
			}
			return false
		})
		closeErr := response.Body.Close()
		result <- loopReadSSEObservation{events: events, err: errors.Join(readErr, closeErr)}
	}()
	return result
}

func assertLoopReadRequeueSSE(
	t testing.TB,
	events []loopRunSSEEvent,
	workspaceID string,
	runID string,
) {
	t.Helper()
	for _, event := range events {
		if event.Kind != compozycontract.LoopRunEventNodeRequeued {
			continue
		}
		var payload struct {
			Generation int    `json:"generation"`
			NodeID     string `json:"node_id"`
			ActorKind  string `json:"actor_kind"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode node_requeued SSE payload error = %v; payload=%s", err, event.Payload)
		}
		if event.Event != string(compozycontract.LoopRunEventNodeRequeued) ||
			event.WorkspaceID != workspaceID || event.LoopRunID != runID || event.Seq < 1 ||
			payload.Generation != 3 || payload.NodeID != "primary" ||
			payload.ActorKind != "human" || payload.Reason != "operator repaired the target" {
			t.Fatalf("node_requeued SSE = %#v payload=%#v", event, payload)
		}
		return
	}
	t.Fatalf("Loop SSE events = %#v, want structured node_requeued", events)
}

func assertRequeuedGenerationVisibleOnFirstPoll(
	t testing.TB,
	roster compozycontract.LoopRunNodesResponse,
	nodeID looppkg.NodeID,
	generation int,
) {
	t.Helper()
	for _, node := range roster.Nodes {
		if node.NodeID != nodeID || node.Generation != generation {
			continue
		}
		if node.State == looppkg.NodeStatePending || node.State == looppkg.NodeStateQuarantined ||
			node.Attempt < 1 || node.StartedAt.IsZero() {
			t.Fatalf("first-poll requeued node = %#v, want an attempted generation", node)
		}
		return
	}
	t.Fatalf("first-poll roster = %#v, want %s in generation %d", roster.Nodes, nodeID, generation)
}

func assertStaleRequeueConflict(
	t testing.TB,
	status int,
	body string,
	conflict compozycontract.ErrorPayload,
	nodeID string,
	winnerReason string,
) {
	t.Helper()
	details := conflict.Details
	winnerAt := details[looppkg.ReasonMetaWinnerRequestedAt]
	if _, err := time.Parse(time.RFC3339Nano, winnerAt); err != nil {
		t.Fatalf("winner_requested_at = %q, want RFC3339Nano: %v", winnerAt, err)
	}
	wantError := fmt.Sprintf(
		"already_decided: loop: transition conflict: node %q was already requeued "+
			"(actual_state=active, allowed_transitions=pause,cancel,kill, winner_actor_id=local-user, "+
			"winner_actor_kind=human, winner_reason=%s, winner_requested_at=%s)",
		nodeID,
		winnerReason,
		winnerAt,
	)
	if status != http.StatusConflict || conflict.Code != string(looppkg.ReasonCodeAlreadyDecided) ||
		len(details) != 6 || details[looppkg.ReasonMetaActualState] != "active" ||
		details[looppkg.ReasonMetaAllowedTransitions] != "pause,cancel,kill" ||
		details[looppkg.ReasonMetaWinnerActorKind] != "human" ||
		details[looppkg.ReasonMetaWinnerActorID] != "local-user" ||
		details[looppkg.ReasonMetaWinnerReason] != winnerReason || winnerAt == "" ||
		conflict.Error != wantError {
		t.Fatalf("stale node requeue = status %d body %s", status, body)
	}
}

type loopRunReadProjection struct {
	ID        string
	LoopName  string
	Status    compozycontract.LoopRunStatus
	Attention *compozycontract.LoopRunAttention
	Progress  compozycontract.LoopRunProgress
}

func loopReadRunPath(workspaceID string, runID string) string {
	return "/api/workspaces/" + url.PathEscape(workspaceID) + "/loop-runs/" + url.PathEscape(runID)
}

func loopReadRawGET(
	t testing.TB,
	ctx context.Context,
	client *http.Client,
	target string,
) (int, string) {
	t.Helper()
	return loopReadRawJSON(t, ctx, client, target, http.MethodGet, nil)
}

func loopReadRawJSON(
	t testing.TB,
	ctx context.Context,
	client *http.Client,
	target string,
	method string,
	bodyValue any,
) (int, string) {
	t.Helper()
	var bodyReader *bytes.Reader
	if bodyValue == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		body, err := json.Marshal(bodyValue)
		if err != nil {
			t.Fatalf("encode %s %s body error = %v", method, target, err)
		}
		bodyReader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		t.Fatalf("create %s %s error = %v", method, target, err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, target, err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read %s %s response error = %v", method, target, err)
	}
	return response.StatusCode, strings.TrimSpace(string(body))
}

func drainLoopRunsTransport(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	transport string,
	limit int,
) []compozycontract.LoopRunPayload {
	t.Helper()
	cursor := ""
	runs := make([]compozycontract.LoopRunPayload, 0)
	for {
		query := url.Values{"limit": {strconv.Itoa(limit)}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loop-runs?" + query.Encode()
		var page compozycontract.LoopRunsResponse
		var err error
		switch transport {
		case "http":
			err = harness.HTTPJSON(ctx, http.MethodGet, path, nil, &page)
		case "uds":
			err = harness.UDSJSON(ctx, http.MethodGet, path, nil, &page)
		default:
			t.Fatalf("unsupported run-list transport %q", transport)
		}
		if err != nil {
			t.Fatalf("%s loop runs page error = %v", transport, err)
		}
		runs = append(runs, page.Runs...)
		if page.NextCursor == "" {
			return runs
		}
		cursor = page.NextCursor
	}
}

func drainLoopRunsCLI(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	limit int,
) []compozycontract.LoopRunPayload {
	t.Helper()
	cursor := ""
	runs := make([]compozycontract.LoopRunPayload, 0)
	for {
		args := []string{
			"loop",
			"runs",
			"--limit",
			strconv.Itoa(limit),
			"--workspace",
			harness.WorkspaceRoot,
			"-o",
			"json",
		}
		if cursor != "" {
			args = append(args, "--cursor", cursor)
		}
		var page struct {
			Items []struct {
				RunID     string                            `json:"run_id"`
				LoopName  string                            `json:"loop_name"`
				Status    compozycontract.LoopRunStatus     `json:"status"`
				Attention *compozycontract.LoopRunAttention `json:"attention,omitempty"`
				Progress  compozycontract.LoopRunProgress   `json:"progress"`
			} `json:"items"`
			NextCursor string `json:"next_cursor"`
		}
		if err := harness.CLI.RunJSON(ctx, &page, args...); err != nil {
			t.Fatalf("CLI loop runs page error = %v", err)
		}
		for _, item := range page.Items {
			runs = append(runs, compozycontract.LoopRunPayload{
				ID: item.RunID, LoopName: item.LoopName, Status: item.Status,
				Attention: item.Attention, Progress: item.Progress,
			})
		}
		if page.NextCursor == "" {
			return runs
		}
		cursor = page.NextCursor
	}
}

func loopRunReadProjections(runs []compozycontract.LoopRunPayload) []loopRunReadProjection {
	projections := make([]loopRunReadProjection, 0, len(runs))
	for _, run := range runs {
		projections = append(projections, loopRunReadProjection{
			ID:        run.ID,
			LoopName:  run.LoopName,
			Status:    run.Status,
			Attention: run.Attention,
			Progress:  run.Progress,
		})
	}
	return projections
}

func assertLoopRunReadOrdering(t testing.TB, runs []compozycontract.LoopRunPayload) {
	t.Helper()
	if len(runs) < 5 {
		t.Fatalf("loop runs = %d, want multiple pages", len(runs))
	}
	previousRank := 0
	seenRanks := map[int]bool{}
	for _, run := range runs {
		rank := loopRunReadRank(run)
		if rank < previousRank {
			t.Fatalf("loop run %q rank = %d after rank %d; order=%#v", run.ID, rank, previousRank, runs)
		}
		previousRank = rank
		seenRanks[rank] = true
	}
	if !seenRanks[1] || !seenRanks[2] || !seenRanks[3] {
		t.Fatalf("loop run order tiers = %#v, want needs-you active terminal", seenRanks)
	}
}

func loopRunReadRank(run compozycontract.LoopRunPayload) int {
	if run.Attention != nil {
		return 1
	}
	switch run.Status {
	case compozycontract.LoopRunStatusDone,
		compozycontract.LoopRunStatusFailed,
		compozycontract.LoopRunStatusCanceled,
		compozycontract.LoopRunStatusExhausted,
		compozycontract.LoopRunStatusNoOp,
		compozycontract.LoopRunStatusBlocked,
		compozycontract.LoopRunStatusStalled:
		return 3
	default:
		return 2
	}
}

func assertLoopRunReadSummary(
	t testing.TB,
	runs []compozycontract.LoopRunPayload,
	runID string,
	wantAttentionKind string,
	wantAttentionCount int,
	wantDone int,
	wantTotal int,
) {
	t.Helper()
	for _, run := range runs {
		if run.ID != runID {
			continue
		}
		if wantAttentionKind == "" && run.Attention != nil {
			t.Fatalf("loop run summary %q = unexpected attention %#v", runID, run.Attention)
		}
		if wantAttentionKind != "" && (run.Attention == nil ||
			run.Attention.Kind != wantAttentionKind || run.Attention.Count != wantAttentionCount ||
			run.Attention.Since.IsZero()) {
			t.Fatalf(
				"loop run summary %q = attention %#v, want kind=%q count=%d with timestamp",
				runID,
				run.Attention,
				wantAttentionKind,
				wantAttentionCount,
			)
		}
		if run.Progress.StepsDone != wantDone ||
			run.Progress.StepsTotal != wantTotal {
			t.Fatalf("loop run summary %q = attention %#v progress %#v", runID, run.Attention, run.Progress)
		}
		return
	}
	t.Fatalf("loop run %q missing from paged roster", runID)
}

func seedLoopReadDefinitions(t testing.TB, workspaceRoot string) {
	t.Helper()
	root := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.LoopsDirName)
	definitions := map[string]string{
		loopReadApprovalLoopName:   loopReadApprovalYAML(),
		loopReadQuarantineLoopName: loopReadQuarantineYAML(),
		loopReadWaitingLoopName:    loopReadWaitingYAML(),
	}
	for name, definition := range definitions {
		if _, _, err := looppkg.WriteDefinition(
			root,
			[]byte(definition),
			looppkg.WriteDefinitionOptions{Source: looppkg.SourceWorkspace},
		); err != nil {
			t.Fatalf("WriteDefinition(%s) error = %v", name, err)
		}
	}
}

func loopReadApprovalYAML() string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: loop-read-approval, description: "E2E human approval read probe." }
concurrency: allow
contract:
  goal: "Prepare, receive explicit approval, and finish."
  definition_of_done: "The approved final transform succeeds."
  stop_when: "nodes.finish.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 1
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: prepare
      class: action
      kind: run-agent
      timeout: 2s
      retry: { max_attempts: 2, backoff: { base: 10ms, max: 10ms } }
      params:
        agent: lifecycle-retry-agent
        prompt: "retry lifecycle"
        output_schema:
          type: object
          required: [summary, value]
          properties:
            summary: { type: string }
            value: { type: string }
    - id: approval
      class: control
      kind: gate
      verdict_policy: revise_until_clean
      criteria:
        - { id: operator, type: human }
    - id: finish
      class: action
      kind: transform
      params:
        map: { status: { value: approved } }
  edges:
    - { from: prepare, to: approval }
    - { from: approval, to: finish }
start: [{ kind: http }, { kind: uds }]
`
}

func loopReadWaitingYAML() string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: loop-read-waiting, description: "Integration active-run ordering probe." }
concurrency: allow
contract:
  goal: "Wait for a test-only event."
  definition_of_done: "The event arrives and the final transform succeeds."
  stop_when: "nodes.finish.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 1
  no_progress: { window: 2 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: wait_for_test
      class: source
      kind: watch-events
      events:
        - { kind: automation.run.failed }
    - id: finish
      class: action
      kind: transform
      params:
        map: { status: { value: released } }
  edges:
    - { from: wait_for_test, to: finish }
start: [{ kind: http }, { kind: uds }]
`
}

func loopReadQuarantineYAML() string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: loop-read-quarantine, description: "Integration quarantined node mutation probe." }
concurrency: allow
contract:
  goal: "Park one repeatedly failing node until an operator requeues it."
  definition_of_done: "The operator-requeued node succeeds."
  stop_when: "nodes.primary.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 4
  no_progress: { window: 3 }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: primary
      class: action
      kind: run-agent
      retry: { max_attempts: 0 }
      result_contract: { failure_field: error, message_field: error }
      params:
        agent: lifecycle-quarantine-agent
        prompt: "quarantine probe generation {{ .generation }}"
        output_schema:
          type: object
          required: [summary]
          properties:
            summary: { type: string }
            error: { type: string }
  edges: []
start: [{ kind: http }, { kind: uds }]
`
}

func runLoopWithHumanGate(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) compozycontract.LoopRunPayload {
	t.Helper()
	enabled := true
	request := compozycontract.RunLoopRequest{
		ConfigOverrides: &compozycontract.LoopConfig{HumanGateEnabled: &enabled},
	}
	var response compozycontract.RunLoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loops/" + url.PathEscape(loopReadApprovalLoopName) + "/run"
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("HTTP run human-gate loop error = %v", err)
	}
	if response.Run == nil {
		t.Fatalf("HTTP run human-gate response = %#v, want run", response)
	}
	return *response.Run
}

func waitForLoopRosterNodeState(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	nodeID looppkg.NodeID,
	want looppkg.NodeState,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var last []looppkg.RosterNode
	for {
		var roster compozycontract.LoopRunNodesResponse
		path := loopReadRunPath(harness.WorkspaceID, runID) + "/nodes?state=all&limit=500"
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &roster); err != nil {
			t.Fatalf("read Loop roster while waiting for %s: %v", want, err)
		}
		last = roster.Nodes
		for _, node := range roster.Nodes {
			if node.NodeID == nodeID && node.State == want {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Loop node %s state %s: %v; last roster=%#v", nodeID, want, ctx.Err(), last)
		case <-ticker.C:
		}
	}
}

func assertGoldenLoopRunTranscript(t testing.TB, raw string, loopName string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 2 || lines[0] != "Loop "+loopName+" run requested" {
		t.Fatalf("loop run transcript = %q, want request line followed by run URL", raw)
	}
	parsed, err := url.Parse(lines[1])
	if err != nil {
		t.Fatalf("parse loop run transcript URL error = %v; transcript=%q", err, raw)
	}
	const prefix = "/loop-runs/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		t.Fatalf("loop run transcript URL path = %q, want %s<run-id>", parsed.Path, prefix)
	}
	runID, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), prefix))
	if err != nil {
		t.Fatalf("unescape loop run transcript ID error = %v", err)
	}
	if strings.TrimSpace(runID) == "" {
		t.Fatalf("loop run transcript URL = %q, want run ID", lines[1])
	}
	return runID
}

func assertCalmTaskListTranscript(t testing.TB, raw string, runID string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 4 || lines[0] != "Tasks" || lines[2] == "" ||
		!strings.Contains(lines[2], "ID") || !strings.Contains(lines[2], "STATUS") ||
		!strings.Contains(lines[2], "TITLE") {
		t.Fatalf("calm task list transcript = %q, want Tasks table with ID STATUS TITLE", raw)
	}
	if strings.Contains(raw, runID) || !strings.Contains(raw, "(empty)") {
		t.Fatalf("calm task list transcript = %q, want no Loop execution rows", raw)
	}
}

func assertHealthyWhyTranscript(t testing.TB, raw string, runID string) {
	t.Helper()
	want := strings.Join([]string{
		"RUNNING · round 1 — Running step prepare in round 1.",
		"0 of 2 steps are complete in round 1.",
		"Nothing needs you. 0 of 2 steps done.",
		"Watch: compozy loop events " + runID + " --follow",
	}, "\n")
	if strings.TrimSpace(raw) != want {
		t.Fatalf("healthy loop why transcript = %q, want %q", strings.TrimSpace(raw), want)
	}
}

func assertNeedsYouWhyTranscript(
	t testing.TB,
	raw string,
	runID string,
	workspaceID string,
	gateID string,
) {
	t.Helper()
	want := strings.Join([]string{
		"NEEDS YOU · round 1 — This run needs attention: approval.",
		"1 of 2 steps are complete in round 1.",
		"Unblock: compozy loop approve " + runID + " --workspace " + workspaceID + " --gate " + gateID,
		"Watch: compozy loop events " + runID + " --follow",
	}, "\n")
	if strings.TrimSpace(raw) != want {
		t.Fatalf("needs-you loop why transcript = %q, want %q", strings.TrimSpace(raw), want)
	}
}

func assertTerminalWhyTranscript(t testing.TB, raw string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "DONE · finished ") ||
		!strings.Contains(lines[0], " after 1 round (") ||
		!strings.HasPrefix(lines[1], "Spent ") || !strings.HasPrefix(lines[2], "Produced: ") {
		t.Fatalf("terminal loop why transcript = %q, want terminal outcome, usage, and artifacts", raw)
	}
}

func assertSettledTaskListTranscript(t testing.TB, raw string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 5 || lines[0] != "Tasks" || !strings.Contains(lines[2], "STATUS") ||
		!strings.Contains(lines[2], "LOOP") || !strings.Contains(raw, "completed") {
		t.Fatalf("settled task list transcript = %q, want completed Loop rows", raw)
	}
	for _, unsettled := range []string{"in_progress", "ready", "needs_attention"} {
		if strings.Contains(raw, unsettled) {
			t.Fatalf("settled task list transcript contains %q: %s", unsettled, raw)
		}
	}
}

func assertSettledLoopTaskList(t testing.TB, raw string, runID string) {
	t.Helper()
	var response struct {
		Tasks []struct {
			Status string `json:"status"`
			Loop   *struct {
				RunID string `json:"run_id"`
			} `json:"loop"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("decode settled task list error = %v; body=%s", err, raw)
	}
	if len(response.Tasks) == 0 {
		t.Fatal("settled task list is empty")
	}
	for _, item := range response.Tasks {
		if item.Loop == nil || item.Loop.RunID != runID {
			t.Fatalf("settled task provenance = %#v, want run %q", item.Loop, runID)
		}
		switch item.Status {
		case "completed", "failed", "canceled":
		default:
			t.Fatalf("settled task status = %q, want terminal", item.Status)
		}
	}
}

func timelineJSONLSequences(t testing.TB, raw string) []int64 {
	t.Helper()
	sequences := make([]int64, 0)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry looppkg.TimelineEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode timeline JSONL error = %v; line=%s", err, line)
		}
		sequences = append(sequences, entry.Seq)
	}
	return sequences
}

func assertTimelineResumeParity(
	t testing.TB,
	combined []int64,
	complete []looppkg.TimelineEntry,
) {
	t.Helper()
	want := make([]int64, len(complete))
	for index := range complete {
		want[index] = complete[index].Seq
	}
	sort.Slice(combined, func(i, j int) bool { return combined[i] < combined[j] })
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if len(combined) != len(want) {
		t.Fatalf("resumed/complete sequence counts = %d/%d; got=%v want=%v", len(combined), len(want), combined, want)
	}
	for index := range want {
		if combined[index] != want[index] {
			t.Fatalf("resumed sequences = %v, want %v", combined, want)
		}
		if index > 0 && combined[index] == combined[index-1] {
			t.Fatalf("resumed sequence %d duplicated in %v", combined[index], combined)
		}
	}
}
