//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const agentCallsE2EExpect = `{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}},"additionalProperties":false}`

func TestDaemonE2EAgentCallsRuntimeAndPublicSurfaces(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixture := mockFixturePath(t, "agent_calls_fixture.json")
	tools := []string{
		toolspkg.ToolIDAgentCall.String(),
		toolspkg.ToolIDCallReturn.String(),
		toolspkg.ToolIDAgentMessage.String(),
	}
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			{
				FixturePath: fixture, FixtureAgent: "reviewer", AgentName: "reviewer",
				Description: "Reviews delegated work and returns a structured answer.", Tools: tools,
			},
			{FixturePath: fixture, FixtureAgent: "silent", AgentName: "silent", Tools: tools},
			{FixturePath: fixture, FixtureAgent: "extractor", AgentName: "extractor", Tools: tools},
			{FixturePath: fixture, FixtureAgent: "blocker", AgentName: "blocker", Tools: tools},
			{FixturePath: fixture, FixtureAgent: "messenger", AgentName: "messenger", Tools: tools},
			{
				FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
				FixtureAgent: "alpha", AgentName: "task-worker",
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	t.Run("Should complete return follow-up idempotency repair and resumable await journeys E2E-001 E2E-002 E2E-005 E2E-006 E2E-007", func(t *testing.T) {
		// not parallel: the journey intentionally reuses one child and isolated runtime.
		golden := createAgentCallCLI(t, ctx, harness, "reviewer", "golden path", "--expect", agentCallsE2EExpect)
		settled := waitForAgentCallState(t, ctx, harness, golden.CallID, callspkg.StateCompleted)
		if settled.Verdict != string(callspkg.VerdictReturned) || settled.ChildSessionID == "" {
			t.Fatalf("golden call = %#v, want returned result and child", settled)
		}
		var result map[string]int
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&result,
			"call", "result", golden.CallID,
			"--workspace", harness.WorkspaceRoot,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI call result error = %v", err)
		}
		if result["answer"] != 42 {
			t.Fatalf("CLI call result = %#v, want answer 42", result)
		}

		first := createAgentCallCLI(t, ctx, harness, "reviewer", "first pass", "--expect", agentCallsE2EExpect)
		firstSettled := waitForAgentCallState(t, ctx, harness, first.CallID, callspkg.StateCompleted)
		followUp := createAgentCallCLI(
			t,
			ctx,
			harness,
			firstSettled.ChildSessionID,
			"one more thing",
			"--expect",
			agentCallsE2EExpect,
		)
		followUpSettled := waitForAgentCallState(t, ctx, harness, followUp.CallID, callspkg.StateCompleted)
		if followUpSettled.ChildSessionID != firstSettled.ChildSessionID {
			t.Fatalf("follow-up child = %q, want %q", followUpSettled.ChildSessionID, firstSettled.ChildSessionID)
		}

		idempotent := createAgentCallCLI(
			t,
			ctx,
			harness,
			"reviewer",
			"idempotent retry",
			"--idempotency-key",
			"e2e-idempotent",
		)
		replayed := createAgentCallCLI(
			t,
			ctx,
			harness,
			"reviewer",
			"idempotent retry",
			"--idempotency-key",
			"e2e-idempotent",
		)
		if idempotent.CallID != replayed.CallID || !replayed.Replayed {
			t.Fatalf("idempotent calls = %#v / %#v, want one replayed call", idempotent, replayed)
		}
		waitForAgentCallState(t, ctx, harness, idempotent.CallID, callspkg.StateCompleted)

		delayed := createAgentCallCLI(t, ctx, harness, "reviewer", "delayed result")
		stdout, stderr, err := harness.CLI.RunInDir(
			ctx,
			harness.WorkspaceRoot,
			"call", "await", delayed.CallID,
			"--workspace", harness.WorkspaceRoot,
			"--timeout", "50ms",
			"-o", "json",
		)
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
			t.Fatalf("CLI call await timeout error = %v stderr=%q, want exit 3", err, stderr)
		}
		var timeoutOutcome compozycontract.AwaitCallsResponse
		if decodeErr := json.Unmarshal([]byte(stdout), &timeoutOutcome); decodeErr != nil {
			t.Fatalf("decode await timeout output error = %v; stdout=%s", decodeErr, stdout)
		}
		if timeoutOutcome.Outcome != "timeout" || timeoutOutcome.Resume == "" {
			t.Fatalf("await timeout = %#v, want resume token", timeoutOutcome)
		}
		waitForAgentCallState(t, ctx, harness, delayed.CallID, callspkg.StateCompleted)
		var resumed compozycontract.AwaitCallsResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&resumed,
			"call", "await", delayed.CallID,
			"--workspace", harness.WorkspaceRoot,
			"--resume", timeoutOutcome.Resume,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI resumed await error = %v", err)
		}
		if resumed.Outcome != "complete" || len(resumed.Settled) != 1 {
			t.Fatalf("resumed await = %#v, want completed call", resumed)
		}

		repair := createAgentCallCLI(
			t,
			ctx,
			harness,
			"reviewer",
			"repair result",
			"--expect",
			agentCallsE2EExpect,
		)
		repaired := waitForAgentCallState(t, ctx, harness, repair.CallID, callspkg.StateCompleted)
		if repaired.Verdict != string(callspkg.VerdictRepaired) || repaired.RepairAttempts != 1 {
			t.Fatalf("repair call = %#v, want one repaired attempt", repaired)
		}
	})

	t.Run("Should settle silence extraction mailbox cancel and deadline journeys E2E-004 E2E-008 E2E-009 E2E-010 E2E-029", func(t *testing.T) {
		// not parallel: journeys share the daemon's deterministic calls budget.
		silent := createAgentCallCLI(t, ctx, harness, "silent", "finish silently")
		silentSettled := waitForAgentCallState(t, ctx, harness, silent.CallID, callspkg.StateCompletedWithoutResult)
		if !strings.Contains(silentSettled.FinalProsePreview, "Reviewed the change") {
			t.Fatalf("silent call = %#v, want prose preview", silentSettled)
		}

		extracted := createAgentCallCLI(
			t,
			ctx,
			harness,
			"extractor",
			"extract result",
			"--expect",
			agentCallsE2EExpect,
		)
		extractedSettled := waitForAgentCallState(t, ctx, harness, extracted.CallID, callspkg.StateCompleted)
		if extractedSettled.Verdict != string(callspkg.VerdictExtracted) {
			t.Fatalf("extracted call = %#v, want extracted verdict", extractedSettled)
		}

		messaged := createAgentCallCLI(t, ctx, harness, "messenger", "message parent")
		messagedSettled := waitForAgentCallState(t, ctx, harness, messaged.CallID, callspkg.StateCompleted)
		var messages compozycontract.CallMessagesResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&messages,
			"message", "list",
			"--workspace", harness.WorkspaceRoot,
			"--session", messagedSettled.ParentSessionID,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI message list error = %v", err)
		}
		if len(messages.Items) == 0 || messages.Items[0].CallID != messaged.CallID {
			t.Fatalf("message list = %#v, want call-bound parent message", messages)
		}

		blocking := createAgentCallCLI(t, ctx, harness, "blocker", "keep working")
		waitForAgentCallState(t, ctx, harness, blocking.CallID, callspkg.StateRunning)
		var canceled compozycontract.CancelCallResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&canceled,
			"call", "cancel", blocking.CallID,
			"--workspace", harness.WorkspaceRoot,
			"--reason", "operator canceled",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI call cancel error = %v", err)
		}
		if canceled.State != string(callspkg.StateCanceled) {
			t.Fatalf("cancel response = %#v", canceled)
		}
		waitForAgentCallState(t, ctx, harness, blocking.CallID, callspkg.StateCanceled)

		deadline := createAgentCallCLI(t, ctx, harness, "blocker", "keep working", "--deadline", "1s")
		waitForAgentCallState(t, ctx, harness, deadline.CallID, callspkg.StateTimeout)
	})

	t.Run("Should preserve batch HTTP and agent-definition contracts E2E-003 E2E-023 E2E-024 E2E-025", func(t *testing.T) {
		// not parallel: assertions observe one shared catalog and call page.
		path := fmt.Sprintf("/api/workspaces/%s/calls", url.PathEscape(harness.WorkspaceID))
		batch := compozycontract.CreateCallRequest{Tasks: []compozycontract.CreateCallItemRequest{
			{Target: compozycontract.CallTargetRequest{Agent: "reviewer"}, Prompt: "golden path"},
			{Target: compozycontract.CallTargetRequest{Agent: "extractor"}, Prompt: "extract result"},
			{Target: compozycontract.CallTargetRequest{Agent: "missing-agent"}, Prompt: "reject this"},
		}}
		var batchItems []compozycontract.CallBatchItemPayload
		if err := harness.HTTPJSON(ctx, http.MethodPost, path, batch, &batchItems); err != nil {
			t.Fatalf("HTTP batch create error = %v", err)
		}
		if len(batchItems) != 3 || batchItems[0].CallID == "" || batchItems[1].CallID == "" ||
			batchItems[2].Error == nil || batchItems[2].Error.Code != string(callspkg.CodeAgentUnknown) {
			t.Fatalf("HTTP batch response = %#v", batchItems)
		}
		waitForAgentCallState(t, ctx, harness, batchItems[0].CallID, callspkg.StateCompleted)
		waitForAgentCallState(t, ctx, harness, batchItems[1].CallID, callspkg.StateCompleted)

		status, unknown := postAgentCallHTTP[compozycontract.CallErrorResponse](
			t,
			ctx,
			harness,
			path,
			compozycontract.CreateCallRequest{CreateCallItemRequest: compozycontract.CreateCallItemRequest{
				Target: compozycontract.CallTargetRequest{Agent: "missing-agent"}, Prompt: "unknown",
			}},
		)
		if status != http.StatusNotFound || unknown.Code != string(callspkg.CodeAgentUnknown) || len(unknown.Available) == 0 {
			t.Fatalf("HTTP unknown agent = status %d payload %#v", status, unknown)
		}

		status, malformed := postAgentCallHTTP[compozycontract.CallErrorResponse](
			t,
			ctx,
			harness,
			path,
			compozycontract.CreateCallRequest{CreateCallItemRequest: compozycontract.CreateCallItemRequest{
				Target: compozycontract.CallTargetRequest{Agent: "reviewer"}, Prompt: "invalid expect",
				Expect: json.RawMessage(`{"type":`),
			}},
		)
		if status != http.StatusUnprocessableEntity || malformed.Code != string(callspkg.CodeExpectInvalid) {
			t.Fatalf("HTTP malformed expect = status %d payload %#v", status, malformed)
		}

		var agents []compozycontract.AgentPayload
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&agents,
			"agent", "list",
			"--workspace", harness.WorkspaceRoot,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI agent list error = %v", err)
		}
		found := false
		for _, agent := range agents {
			if agent.Name == "reviewer" && agent.Description == "Reviews delegated work and returns a structured answer." {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("CLI agents = %#v, want reviewer description", agents)
		}
	})

	t.Run("Should enforce TTL inert messages subtree drain and recursion walls E2E-011 E2E-012 E2E-013 E2E-014", func(t *testing.T) {
		// not parallel: the cases intentionally mutate one governed call tree.
		ttl := createAgentCallCLI(
			t,
			ctx,
			harness,
			"reviewer",
			"first pass",
			"--idle-ttl",
			"1s",
		)
		ttlSettled := waitForAgentCallState(t, ctx, harness, ttl.CallID, callspkg.StateCompleted)
		waitForAgentCallCLIErrorCode(
			t,
			ctx,
			harness,
			10*time.Second,
			callspkg.CodeTargetExpired,
			"call", ttlSettled.ChildSessionID, "one more thing",
			"--workspace", harness.WorkspaceRoot, "-o", "json",
		)

		messaged := createAgentCallCLI(t, ctx, harness, "messenger", "message parent")
		messagedCall := waitForAgentCallState(t, ctx, harness, messaged.CallID, callspkg.StateCompleted)
		var messages compozycontract.CallMessagesResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&messages,
			"message", "list",
			"--workspace", harness.WorkspaceRoot,
			"--session", messagedCall.ParentSessionID,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI inert message list error = %v", err)
		}
		if len(messages.Items) == 0 || messages.Items[0].Text != "Need an operator decision." {
			t.Fatalf("inert messages = %#v", messages.Items)
		}

		completed := createAgentCallCLI(t, ctx, harness, "reviewer", "golden path")
		completedCall := waitForAgentCallState(t, ctx, harness, completed.CallID, callspkg.StateCompleted)
		runningA := createAgentCallCLI(t, ctx, harness, "blocker", "keep working")
		runningB := createAgentCallCLI(t, ctx, harness, "blocker", "keep working")
		waitForAgentCallState(t, ctx, harness, runningA.CallID, callspkg.StateRunning)
		waitForAgentCallState(t, ctx, harness, runningB.CallID, callspkg.StateRunning)
		var drain struct {
			SessionID string `json:"session_id"`
			compozycontract.StopSessionSubtreeResponse
		}
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&drain,
			"session", "stop", completedCall.ParentSessionID,
			"--subtree",
			"--reason", "e2e drain",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI session stop --subtree error = %v", err)
		}
		if drain.ClosedCalls < 2 || drain.PreservedResults < 1 {
			t.Fatalf("subtree drain = %#v, want open calls closed and result preserved", drain)
		}
		var preserved map[string]int
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&preserved,
			"call", "result", completed.CallID,
			"--workspace", harness.WorkspaceRoot,
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI preserved result after drain error = %v", err)
		}

		root := createAgentCallCLI(t, ctx, harness, "blocker", "keep working")
		rootCall := waitForAgentCallState(t, ctx, harness, root.CallID, callspkg.StateRunning)
		depthTwo := createNestedAgentCall(t, ctx, harness, rootCall.ChildSessionID)
		depthTwoCall := waitForAgentCallState(t, ctx, harness, depthTwo.CallID, callspkg.StateRunning)
		depthThree := createNestedAgentCall(t, ctx, harness, depthTwoCall.ChildSessionID)
		depthThreeCall := waitForAgentCallState(t, ctx, harness, depthThree.CallID, callspkg.StateRunning)
		depthThreeClient := hostedMCPClientForSession(t, ctx, harness, "blocker", depthThreeCall.ChildSessionID)
		defer closeHostedMCPClient(t, depthThreeClient)
		listed, err := depthThreeClient.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools(depth wall) error = %v", err)
		}
		if sdkToolListContains(listed.Tools, toolspkg.ToolIDAgentCall.String()) {
			t.Fatalf("depth-wall tools = %#v, must omit %s", sdkToolNames(listed.Tools), toolspkg.ToolIDAgentCall)
		}
		registration, ok := harness.MockAgentRegistration("blocker")
		if !ok {
			t.Fatal("MockAgentRegistration(blocker) = missing")
		}
		records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics(blocker) error = %v", err)
		}
		visibleDepthWall := false
		for _, record := range acpmock.DiagnosticsForCompozySession(records, depthThreeCall.ChildSessionID) {
			if strings.Contains(record.Prompt, "You cannot delegate further.") {
				visibleDepthWall = true
				break
			}
		}
		if !visibleDepthWall {
			t.Fatalf(
				"depth-three diagnostics contain %d session records, want literal zero remaining depth prompt",
				len(acpmock.DiagnosticsForCompozySession(records, depthThreeCall.ChildSessionID)),
			)
		}
	})

	t.Run("Should apply the task result contract through CLI and agent lease surfaces E2E-026", func(t *testing.T) {
		// not parallel: the task run has one ordered claim/start/repair/completion lifecycle.
		var created compozycontract.TaskPayload
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&created,
			"task", "create",
			"--scope", "workspace",
			"--workspace", harness.WorkspaceRoot,
			"--title", "Return a contracted result",
			"--owner-kind", "pool",
			"--owner-ref", "task-worker",
			"--expect", agentCallsE2EExpect,
			"--result-budget", "256KiB",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI task create --expect error = %v", err)
		}
		if created.ID == "" || created.ExpectDigest == "" || created.ResultBudget == nil {
			t.Fatalf("created task = %#v, want contract digest and budget", created)
		}
		run := enqueueTaskRunViaUDS(t, ctx, harness, created.ID, "")
		worker := createBoundFixtureBackedSession(t, ctx, harness, "task-worker", "contract-worker")
		if _, err := harness.ClaimExactTaskRunForSession(ctx, run.ID, &worker); err != nil {
			t.Fatalf("ClaimExactTaskRunForSession(%s) error = %v", run.ID, err)
		}
		started, err := harness.StartClaimedTaskRunForSession(
			ctx,
			run.ID,
			&worker,
			compozycontract.StartTaskRunRequest{IdempotencyKey: "start-contract-task"},
		)
		if err != nil {
			t.Fatalf("StartClaimedTaskRunForSession(%s) error = %v", run.ID, err)
		}
		if started.ExpectDigest != created.ExpectDigest || started.ResultBudget == nil {
			t.Fatalf("started run = %#v, want task contract snapshot", started)
		}
		_, err = harness.CompleteClaimedTaskRunForSession(
			ctx,
			run.ID,
			&worker,
			compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"wrong":true}`)},
		)
		if err == nil || !strings.Contains(err.Error(), "task_result_invalid") || strings.Contains(err.Error(), "wrong") {
			t.Fatalf("invalid task completion error = %v, want sanitized typed rejection", err)
		}
		completed, err := harness.CompleteClaimedTaskRunForSession(
			ctx,
			run.ID,
			&worker,
			compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"answer":21}`)},
		)
		if err != nil {
			t.Fatalf("task result resubmission error = %v", err)
		}
		if completed.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("completed task lease = %#v", completed)
		}
		var read compozycontract.TaskResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			"/api/workspaces/"+url.PathEscape(harness.WorkspaceID)+"/tasks/"+url.PathEscape(created.ID),
			nil,
			&read,
		); err != nil {
			t.Fatalf("HTTP task read error = %v", err)
		}
		if read.Task.ExpectDigest != created.ExpectDigest || read.Task.ResultBudget == nil {
			t.Fatalf("HTTP task read = %#v, want contract projection", read.Task)
		}
		var runRead compozycontract.TaskRunDetailResponse
		if err := harness.HTTPJSON(
			ctx,
			http.MethodGet,
			"/api/task-runs/"+url.PathEscape(run.ID),
			nil,
			&runRead,
		); err != nil {
			t.Fatalf("HTTP task run read error = %v", err)
		}
		if runRead.Run.Run.ExpectDigest != created.ExpectDigest ||
			runRead.Run.Run.ResultBudget == nil ||
			!strings.Contains(string(runRead.Run.Run.Result), `"answer":21`) {
			t.Fatalf("HTTP task run read = %#v, want contract snapshot and result preview", runRead.Run.Run)
		}
	})
}

func TestDaemonE2EAgentCallPublishBridge(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	fixture := mockFixturePath(t, "agent_calls_fixture.json")
	tools := []string{
		toolspkg.ToolIDAgentCall.String(),
		toolspkg.ToolIDCallReturn.String(),
	}
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		EnableNetwork: true,
		MockAgents: []e2etest.MockAgentSpec{
			{FixturePath: fixture, FixtureAgent: "blocker", AgentName: "publisher", Tools: tools},
			{FixturePath: fixture, FixtureAgent: "reviewer", AgentName: "reviewer", Tools: tools},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	channel := "call-publications"
	detail := mustCreateNetworkChannel(t, ctx, harness, channel, "publisher")
	publisher := requireChannelSession(t, detail, "publisher")

	t.Run("Should publish settled call evidence through CLI", func(t *testing.T) {
		completed := createAgentCallFromSession(t, ctx, harness, "publisher", publisher.ID, "reviewer", "golden path")
		waitForAgentCallState(t, ctx, harness, completed.CallID, callspkg.StateCompleted)
		var cliPublished compozycontract.PublishCallResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&cliPublished,
			"call", "publish", completed.CallID,
			"--workspace", harness.WorkspaceRoot,
			"--channel", channel,
			"--thread", "thread_cli_publish",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI call publish error = %v", err)
		}
		if !cliPublished.Published || cliPublished.NetworkMessageID == "" {
			t.Fatalf("CLI call publish = %#v, want published message", cliPublished)
		}
		waitForRuntimeCondition(t, "published call evidence", 10*time.Second, func() bool {
			return channelHasMessageID(ctx, harness, channel, cliPublished.NetworkMessageID)
		})
		messages := mustHTTPNetworkThreadMessages(t, ctx, harness, channel, "thread_cli_publish")
		if !networkTimelineHasCallEvidence(messages, cliPublished.NetworkMessageID, completed.CallID) {
			t.Fatalf("Network messages = %#v, want call evidence %s", messages, completed.CallID)
		}
	})

	t.Run("Should publish settled call evidence through HTTP", func(t *testing.T) {
		httpCompleted := createAgentCallFromSession(t, ctx, harness, "publisher", publisher.ID, "reviewer", "golden path")
		waitForAgentCallState(t, ctx, harness, httpCompleted.CallID, callspkg.StateCompleted)
		httpStatus, httpPublished := postAgentCallPublishHTTP(
			t,
			ctx,
			harness,
			httpCompleted.CallID,
			compozycontract.PublishCallRequest{Channel: channel, ThreadID: "thread_http_publish"},
		)
		if httpStatus != http.StatusOK || !httpPublished.Published || httpPublished.NetworkMessageID == "" {
			t.Fatalf("HTTP call publish = status %d payload %#v", httpStatus, httpPublished)
		}
		waitForRuntimeCondition(t, "HTTP-published call evidence", 10*time.Second, func() bool {
			return channelHasMessageID(ctx, harness, channel, httpPublished.NetworkMessageID)
		})
		messages := mustHTTPNetworkThreadMessages(t, ctx, harness, channel, "thread_http_publish")
		if !networkTimelineHasCallEvidence(messages, httpPublished.NetworkMessageID, httpCompleted.CallID) {
			t.Fatalf("HTTP Network messages = %#v, want call evidence %s", messages, httpCompleted.CallID)
		}
	})

	t.Run("Should reject non-terminal and canceled calls", func(t *testing.T) {
		running := createAgentCallFromSession(t, ctx, harness, "publisher", publisher.ID, "publisher", "keep working")
		waitForAgentCallState(t, ctx, harness, running.CallID, callspkg.StateRunning)
		status, nonTerminal := postAgentCallPublishHTTPError(
			t,
			ctx,
			harness,
			running.CallID,
			compozycontract.PublishCallRequest{Channel: channel},
		)
		if status != http.StatusConflict || nonTerminal.Code != string(callspkg.CodePublishNotSettled) {
			t.Fatalf("HTTP running publish = status %d payload %#v", status, nonTerminal)
		}
		assertAgentCallPublishCLIError(t, ctx, harness, running.CallID, channel, callspkg.CodePublishNotSettled)
		var canceled compozycontract.CancelCallResponse
		if err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&canceled,
			"call", "cancel", running.CallID,
			"--workspace", harness.WorkspaceRoot,
			"--reason", "publish cancellation check",
			"-o", "json",
		); err != nil {
			t.Fatalf("CLI cancel publication call error = %v", err)
		}
		waitForAgentCallState(t, ctx, harness, running.CallID, callspkg.StateCanceled)
		assertAgentCallPublishCLIError(t, ctx, harness, running.CallID, channel, callspkg.CodePublishNotSettled)
	})

	t.Run("Should reject calls without Network participation", func(t *testing.T) {
		operatorCall := createAgentCallCLI(t, ctx, harness, "reviewer", "golden path")
		waitForAgentCallState(t, ctx, harness, operatorCall.CallID, callspkg.StateCompleted)
		status, noParticipation := postAgentCallPublishHTTPError(
			t,
			ctx,
			harness,
			operatorCall.CallID,
			compozycontract.PublishCallRequest{Channel: channel},
		)
		if status != http.StatusUnprocessableEntity ||
			noParticipation.Code != string(callspkg.CodePublishNoParticipation) {
			t.Fatalf("HTTP no-participation publish = status %d payload %#v", status, noParticipation)
		}
	})

	t.Run("Should keep the reversed Network path unavailable", func(t *testing.T) {
		reversePath := fmt.Sprintf("/api/workspaces/%s/network/calls", url.PathEscape(harness.WorkspaceID))
		reverseRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(reversePath), nil)
		if err != nil {
			t.Fatalf("http.NewRequestWithContext(reverse path) error = %v", err)
		}
		reverseResponse, err := harness.HTTPClient.Do(reverseRequest)
		if err != nil {
			t.Fatalf("HTTP reverse Network path error = %v", err)
		}
		if closeErr := reverseResponse.Body.Close(); closeErr != nil {
			t.Fatalf("close reverse Network response error = %v", closeErr)
		}
		if reverseResponse.StatusCode != http.StatusNotFound {
			t.Fatalf("reverse Network path status = %d, want 404", reverseResponse.StatusCode)
		}
	})
}

func createAgentCallCLI(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	target string,
	prompt string,
	extra ...string,
) compozycontract.CallCreatePayload {
	t.Helper()
	args := []string{"call", target, prompt, "--workspace", harness.WorkspaceRoot}
	args = append(args, extra...)
	args = append(args, "-o", "json")
	var created compozycontract.CallCreatePayload
	if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, &created, args...); err != nil {
		t.Fatalf("CLI call create %q error = %v", prompt, err)
	}
	if created.CallID == "" {
		t.Fatalf("CLI call create %q = %#v, want call id", prompt, created)
	}
	return created
}

func waitForAgentCallState(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	callID string,
	want callspkg.State,
) compozycontract.CallPayload {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var last compozycontract.CallPayload
	var lastErr error
	for {
		err := harness.CLI.RunJSONInDir(
			ctx,
			harness.WorkspaceRoot,
			&last,
			"call", "show", callID,
			"--workspace", harness.WorkspaceRoot,
			"-o", "json",
		)
		if err == nil && last.State == string(want) {
			return last
		}
		lastErr = err
		if err == nil && isAgentCallTerminal(last.State) && last.State != string(want) {
			t.Fatalf("call %s settled %s, want %s; payload=%#v", callID, last.State, want, last)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for call %s state %s: %v; last=%#v; last read error=%v", callID, want, ctx.Err(), last, lastErr)
		case <-ticker.C:
		}
	}
}

func isAgentCallTerminal(state string) bool {
	return callspkg.State(state).Terminal()
}

func createNestedAgentCall(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	parentSessionID string,
) compozycontract.CallCreatePayload {
	t.Helper()
	return createAgentCallFromSession(t, ctx, harness, "blocker", parentSessionID, "blocker", "keep working")
}

func createAgentCallFromSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	parentAgent string,
	parentSessionID string,
	targetAgent string,
	prompt string,
) compozycontract.CallCreatePayload {
	t.Helper()
	client := hostedMCPClientForSession(t, ctx, harness, parentAgent, parentSessionID)
	defer closeHostedMCPClient(t, client)
	var created compozycontract.CallCreatePayload
	callHostedMCPToolJSON(
		t,
		ctx,
		client,
		toolspkg.ToolIDAgentCall.String(),
		map[string]any{"agent": targetAgent, "prompt": prompt},
		&created,
	)
	if created.CallID == "" || created.ChildSessionID == "" {
		t.Fatalf("session agent call = %#v, want call and child IDs", created)
	}
	return created
}

func postAgentCallPublishHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	callID string,
	payload compozycontract.PublishCallRequest,
) (int, compozycontract.PublishCallResponse) {
	t.Helper()
	return postAgentCallPublishHTTPAs[compozycontract.PublishCallResponse](t, ctx, harness, callID, payload)
}

func postAgentCallPublishHTTPError(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	callID string,
	payload compozycontract.PublishCallRequest,
) (int, compozycontract.CallErrorResponse) {
	t.Helper()
	return postAgentCallPublishHTTPAs[compozycontract.CallErrorResponse](t, ctx, harness, callID, payload)
}

func postAgentCallPublishHTTPAs[T any](
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	callID string,
	payload compozycontract.PublishCallRequest,
) (int, T) {
	t.Helper()
	path := fmt.Sprintf(
		"/api/workspaces/%s/calls/%s/publish",
		url.PathEscape(harness.WorkspaceID),
		url.PathEscape(callID),
	)
	return postAgentCallJSON[T](t, ctx, harness, path, payload)
}

func assertAgentCallPublishCLIError(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	callID string,
	channel string,
	want callspkg.ErrorCode,
) {
	t.Helper()
	_, stderr, err := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"call", "publish", callID,
		"--workspace", harness.WorkspaceRoot,
		"--channel", channel,
		"-o", "json",
	)
	assertAgentCallCLIErrorCode(t, err, stderr, want)
}

func waitForAgentCallCLIErrorCode(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	timeout time.Duration,
	want callspkg.ErrorCode,
	args ...string,
) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	var lastCode string
	for {
		_, stderr, err := harness.CLI.RunInDir(waitCtx, harness.WorkspaceRoot, args...)
		lastErr = err
		if code, ok := agentCallCLIErrorCode(err, stderr); ok {
			lastCode = code
			if code == string(want) {
				return
			}
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"wait for CLI error code %s: %v; last code=%q; last command error=%v",
				want,
				waitCtx.Err(),
				lastCode,
				lastErr,
			)
		case <-ticker.C:
		}
	}
}

func assertAgentCallCLIErrorCode(
	t testing.TB,
	err error,
	stderr string,
	want callspkg.ErrorCode,
) {
	t.Helper()
	code, ok := agentCallCLIErrorCode(err, stderr)
	if !ok || code != string(want) {
		t.Fatalf("CLI error = %v code=%q, want exit 2 code %s", err, code, want)
	}
}

func agentCallCLIErrorCode(err error, stderr string) (string, bool) {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		return "", false
	}
	var payload compozycontract.ErrorPayload
	if json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload) != nil {
		return "", false
	}
	return payload.Code, payload.Code != ""
}

func networkTimelineHasCallEvidence(
	messages []compozycontract.NetworkConversationMessagePayload,
	messageID string,
	callID string,
) bool {
	for _, message := range messages {
		if message.MessageID == messageID && message.Kind == "say" && strings.Contains(message.Text, callID) {
			return true
		}
	}
	return false
}

func postAgentCallHTTP[T any](
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	payload compozycontract.CreateCallRequest,
) (int, T) {
	t.Helper()
	return postAgentCallJSON[T](t, ctx, harness, path, payload)
}

func postAgentCallJSON[T any, P any](
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	payload P,
) (int, T) {
	t.Helper()
	var decoded T
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(agent call request) error = %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, harness.HTTPURL(path), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(agent call request) error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatalf("HTTP agent call request error = %v", err)
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&decoded)
	closeErr := response.Body.Close()
	if err := errors.Join(decodeErr, closeErr); err != nil {
		t.Fatalf("decode/close HTTP agent call response error = %v", err)
	}
	return response.StatusCode, decoded
}
