//go:build integration && !windows

package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

func TestDaemonE2ELoopRunEventsShouldStreamRichFramesAndResume(t *testing.T) {
	t.Parallel()

	t.Run("Should stream rich frames resume and isolate by workspace", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "loop_events_fixture.json"),
				FixtureAgent: "loop_events",
				AgentName:    "loop-events-agent",
			}},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		createLoopViaHTTP(t, ctx, harness, loopEventsDefinition())
		run := runLoopViaHTTP(t, ctx, harness, "loop-events-probe")
		waitForLoopRunStatus(t, ctx, harness, run.ID, aghcontract.LoopRunStatusDone)

		eventsPath := loopRunEventsPath(harness.WorkspaceID, run.ID, 0)
		events := readLoopRunSSEUntil(t, ctx, harness, eventsPath, func(events []loopRunSSEEvent) bool {
			return loopSSEKinds(events).Contains(
				string(aghcontract.LoopRunEventStatusChanged),
				string(aghcontract.LoopRunEventNodeRunning),
				string(aghcontract.LoopRunEventNodeSucceeded),
				string(aghcontract.LoopRunEventChannelMsg),
				string(aghcontract.LoopRunEventTokenTick),
			)
		})
		assertLoopSSEWorkspace(t, events, harness.WorkspaceID, run.ID)
		assertLoopSSEPayloadContains(t, events, aghcontract.LoopRunEventChannelMsg, "loop channel result")
		assertLoopSSEPayloadContains(t, events, aghcontract.LoopRunEventTokenTick, `"terminal":true`)

		afterSeq := firstLoopEventSeq(t, events, aghcontract.LoopRunEventNodeRunning)
		resumed := readLoopRunSSEUntil(
			t,
			ctx,
			harness,
			loopRunEventsPath(harness.WorkspaceID, run.ID, afterSeq),
			func(events []loopRunSSEEvent) bool {
				return loopSSEKinds(events).Contains(
					string(aghcontract.LoopRunEventNodeSucceeded),
					string(aghcontract.LoopRunEventTokenTick),
				)
			},
		)
		for _, event := range resumed {
			if event.Seq <= afterSeq {
				t.Fatalf("resumed event seq = %d, want > %d: %#v", event.Seq, afterSeq, resumed)
			}
		}

		foreign := readLoopRunSSEForDuration(
			t,
			harness,
			loopRunEventsPath("foreign-workspace", run.ID, 0),
			250*time.Millisecond,
		)
		if len(foreign) != 0 {
			t.Fatalf("foreign workspace stream events = %#v, want none", foreign)
		}
	})
}

func TestDaemonE2ELoopWatchEventsShouldWakeAndRecover(t *testing.T) {
	t.Run("Should wake parked loop from task status and run completion events", func(t *testing.T) {
		acpmock.RequireDriver(t)

		workspaceRoot := t.TempDir()
		seedWatchEventsLoopDefinition(t, workspaceRoot)
		fixturePath := writeWatchEventsFixture(t)
		harness := startWatchEventsRuntimeHarness(t, e2etest.RuntimeHarnessOptions{
			Workspace:  e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
			MockAgents: watchEventsMockAgents(fixturePath),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		waitForLoopCatalogEntry(t, ctx, harness, watchEventsE2ELoopName)
		parent := createTaskViaUDS(t, ctx, harness, "Watched parent")
		child := createOwnedChildTaskViaUDS(t, ctx, harness, parent.ID, "Watched child", "general")
		blockedChild := createChildTaskViaUDS(t, ctx, harness, parent.ID, "Watched blocked child")
		run := runLoopViaHTTPWithInputs(
			t,
			ctx,
			harness,
			watchEventsE2ELoopName,
			watchEventsE2EInputs(parent.ID, child.ID),
		)
		waitForLoopRunStatus(t, ctx, harness, run.ID, aghcontract.LoopRunStatusWatching)

		lease := claimTaskRunViaAgentUDS(t, ctx, harness, child.ID)
		blockTaskThroughStoreForWatchEventsE2E(t, ctx, harness.HomePaths, blockedChild.ID)
		completeClaimedTaskRunViaAgentUDS(t, ctx, harness, lease)

		waitForLoopRunStatus(t, ctx, harness, run.ID, aghcontract.LoopRunStatusDone)
		assertWatchEventsMockPrompt(t, harness, "watch-events-agent", []string{
			"task.status_changed",
			"task.run.completed",
			child.ID,
			blockedChild.ID,
		})
	})

	t.Run("Should recover a parked watch-events loop from durable rows after restart", func(t *testing.T) {
		acpmock.RequireDriver(t)

		homePaths := e2etest.NewHomePaths(t)
		workspaceRoot := homePaths.HomeDir
		seedWatchEventsLoopDefinition(t, workspaceRoot)
		fixturePath := writeWatchEventsFixture(t)
		harness := startWatchEventsRuntimeHarness(t, e2etest.RuntimeHarnessOptions{
			HomePaths:  homePaths,
			Workspace:  e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
			MockAgents: watchEventsMockAgents(fixturePath),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		waitForLoopCatalogEntry(t, ctx, harness, watchEventsE2ELoopName)
		parent := createTaskViaUDS(t, ctx, harness, "Restart parent")
		child := createChildTaskViaUDS(t, ctx, harness, parent.ID, "Restart child")
		run := runLoopViaHTTPWithInputs(
			t,
			ctx,
			harness,
			watchEventsE2ELoopName,
			watchEventsE2EInputs(parent.ID, child.ID),
		)
		waitForLoopRunStatus(t, ctx, harness, run.ID, aghcontract.LoopRunStatusWatching)

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := harness.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("Stop runtime harness error = %v", err)
		}
		stopCancel()

		blockTaskThroughStoreForWatchEventsE2E(t, ctx, homePaths, child.ID)

		restarted := startWatchEventsRuntimeHarness(t, e2etest.RuntimeHarnessOptions{
			HomePaths: homePaths,
			Workspace: e2etest.WorkspaceSeedOptions{
				Root: workspaceRoot,
			},
			MockAgents: watchEventsMockAgents(fixturePath),
		})

		waitForLoopRunStatus(t, ctx, restarted, run.ID, aghcontract.LoopRunStatusDone)
		assertWatchEventsMockPrompt(t, restarted, "watch-events-agent", []string{
			"task.status_changed",
			child.ID,
		})
	})
}

func loopEventsDefinition() aghcontract.LoopDefinitionDocument {
	return aghcontract.LoopDefinitionDocument{
		APIVersion:  "agh.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: aghcontract.LoopDefinitionMeta{
			Name:        "loop-events-probe",
			Description: "Runtime E2E probe for rich Loop run SSE events.",
			Catalog: aghcontract.LoopCatalogMeta{
				UseWhen:  "Testing Loop run event streaming.",
				Keywords: []string{"test", "events"},
				Category: "Testing",
			},
		},
		Contract: aghcontract.LoopContract{
			Goal:             "Emit rich Loop run events.",
			DefinitionOfDone: "The probe action completes.",
			StopWhen:         "nodes.probe.status == 'succeeded'",
			IterationCap:     1,
			NoProgress: aghcontract.LoopNoProgress{
				Window:     2,
				HashFields: []string{"delivery_artifact"},
			},
			Budget: aghcontract.LoopBudget{
				Tokens:       0,
				WallClockSec: 0,
				OnExceeded:   aghcontract.LoopBudgetExceededHalt,
			},
			TerminalStates: []string{"done", "failed", "blocked", "exhausted", "stalled"},
		},
		Graph: aghcontract.LoopGraph{
			Nodes: []aghcontract.LoopGraphNode{{
				ID:    "probe",
				Class: aghcontract.LoopNodeClassAction,
				Kind:  "run-agent",
				Params: map[string]any{
					"agent":  "loop-events-agent",
					"prompt": "loop event probe",
					"output_schema": map[string]any{
						"type":     "object",
						"required": []any{"summary", "message"},
						"properties": map[string]any{
							"summary": map[string]any{"type": "string"},
							"message": map[string]any{"type": "string"},
						},
					},
				},
			}},
		},
		Start: []aghcontract.LoopStartBinding{
			{Kind: "manual"},
			{Kind: "http"},
			{Kind: "uds"},
		},
	}
}

const watchEventsE2ELoopName = "watch-events-e2e"

func startWatchEventsRuntimeHarness(
	t testing.TB,
	opts e2etest.RuntimeHarnessOptions,
) *e2etest.RuntimeHarness {
	t.Helper()
	if opts.StartTimeout == 0 {
		opts.StartTimeout = 30 * time.Second
	}
	return e2etest.StartRuntimeHarness(t, &opts)
}

func seedWatchEventsLoopDefinition(t testing.TB, workspaceRoot string) {
	t.Helper()
	root := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.LoopsDirName)
	if _, _, err := looppkg.WriteDefinition(root, []byte(watchEventsE2ELoopYAML()), looppkg.WriteDefinitionOptions{
		Source: looppkg.SourceWorkspace,
	}); err != nil {
		t.Fatalf("WriteDefinition(watch-events loop) error = %v", err)
	}
}

func watchEventsE2ELoopYAML() string {
	return `apiVersion: agh.loop/v1
kind: Loop
meta:
  name: watch-events-e2e
  description: Runtime E2E probe for daemon watch-events wake and recovery.
concurrency: queue
inputs:
  parent_task_id:
    type: string
    required: true
  child_task_id:
    type: string
    required: true
  runner:
    type: agent
    required: true
contract:
  goal: "Summarize child task activity for a watched parent task."
  definition_of_done: "The watched event batch was delivered to the summary agent."
  stop_when: "nodes.summarize.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 2
  no_progress: { window: 2, hash_fields: ["nodes.task_activity.output.events"] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: task_activity
      class: source
      kind: watch-events
      events:
        - kind: task.status_changed
          filter: "event.payload.parent_task_id == inputs.parent_task_id && event.payload.to_status == 'blocked'"
        - kind: task.run.completed
          filter: "event.task_id == inputs.child_task_id"
    - id: summarize
      class: action
      kind: run-agent
      params:
        agent: "{{ .inputs.runner }}"
        prompt: "Summarize the watched task activity: {{ toJson .nodes.task_activity.output.events }}"
        output_schema:
          type: object
          required: [summary, message]
          properties:
            summary: { type: string }
            message: { type: string }
  edges:
    - from: task_activity
      to: summarize
start:
  - kind: manual
  - kind: http
  - kind: uds
`
}

func watchEventsE2EInputs(parentTaskID string, childTaskID string) map[string]any {
	return map[string]any{
		"parent_task_id": parentTaskID,
		"child_task_id":  childTaskID,
		"runner":         "watch-events-agent",
	}
}

func watchEventsMockAgents(fixturePath string) []e2etest.MockAgentSpec {
	return []e2etest.MockAgentSpec{
		{
			FixturePath:  fixturePath,
			FixtureAgent: "watch_events",
			AgentName:    "watch-events-agent",
		},
		{
			FixturePath:  fixturePath,
			FixtureAgent: "watch_events",
			AgentName:    "general",
		},
	}
}

func writeWatchEventsFixture(t testing.TB) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watch-events-fixture.json")
	fixture := map[string]any{
		"version": 2,
		"agents": []map[string]any{{
			"name":        "watch_events",
			"provider":    "claude",
			"permissions": "approve-all",
			"prompt":      "You summarize watch-events batches.",
			"turns": []map[string]any{{
				"name":  "watch-events-batch",
				"match": map[string]any{"turn_source": "user"},
				"steps": []map[string]any{
					{
						"kind": "assistant",
						"text": "```json\n{\"summary\":\"watch events observed\",\"message\":\"watch batch complete\"}\n```",
					},
				},
			}},
		}},
	}
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("marshal watch-events fixture error = %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write watch-events fixture error = %v", err)
	}
	return path
}

func createLoopViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	def aghcontract.LoopDefinitionDocument,
) {
	t.Helper()
	var response aghcontract.LoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops"
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		path,
		aghcontract.CreateLoopRequest{Definition: &def},
		&response,
	); err != nil {
		t.Fatalf("HTTP create loop error = %v", err)
	}
	if response.Loop.Name != def.Meta.Name {
		t.Fatalf("created loop = %#v, want %q", response.Loop, def.Meta.Name)
	}
}

func waitForLoopCatalogEntry(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	name string,
) {
	t.Helper()
	waitForRuntimeCondition(t, "loop catalog entry "+name, 20*time.Second, func() bool {
		var response aghcontract.LoopsResponse
		path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops"
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			return false
		}
		for _, item := range response.Loops {
			if item.Name == name {
				return true
			}
		}
		return false
	})
}

func runLoopViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	name string,
) aghcontract.LoopRunPayload {
	t.Helper()
	var response aghcontract.RunLoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops/" + url.PathEscape(name) + "/run"
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, aghcontract.RunLoopRequest{}, &response); err != nil {
		t.Fatalf("HTTP run loop error = %v", err)
	}
	if response.Run == nil {
		t.Fatalf("HTTP run loop response = %#v, want run", response)
	}
	return *response.Run
}

func runLoopViaHTTPWithInputs(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	name string,
	inputs map[string]any,
) aghcontract.LoopRunPayload {
	t.Helper()
	var response aghcontract.RunLoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops/" + url.PathEscape(name) + "/run"
	request := aghcontract.RunLoopRequest{Inputs: inputs}
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("HTTP run loop with inputs error = %v", err)
	}
	if response.Run == nil {
		t.Fatalf("HTTP run loop response = %#v, want run", response)
	}
	return *response.Run
}

func createTaskViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	title string,
) aghcontract.TaskPayload {
	t.Helper()
	var response aghcontract.TaskResponse
	request := aghcontract.CreateTaskRequest{
		Scope:     taskpkg.ScopeWorkspace,
		Workspace: harness.WorkspaceID,
		Title:     title,
	}
	if err := harness.UDSJSON(ctx, http.MethodPost, "/api/tasks", request, &response); err != nil {
		t.Fatalf("UDS create task error = %v", err)
	}
	if response.Task.ID == "" {
		t.Fatalf("UDS create task response = %#v, want task id", response)
	}
	return response.Task
}

func createChildTaskViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	parentTaskID string,
	title string,
) aghcontract.TaskPayload {
	t.Helper()
	var response aghcontract.TaskResponse
	request := aghcontract.CreateTaskChildRequest{
		Scope:     taskpkg.ScopeWorkspace,
		Workspace: harness.WorkspaceID,
		Title:     title,
	}
	path := "/api/tasks/" + url.PathEscape(parentTaskID) + "/children"
	if err := harness.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("UDS create child task error = %v", err)
	}
	if response.Task.ID == "" || response.Task.ParentTaskID != parentTaskID {
		t.Fatalf("UDS create child task response = %#v, want child of %s", response, parentTaskID)
	}
	return response.Task
}

func createOwnedChildTaskViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	parentTaskID string,
	title string,
	ownerRef string,
) aghcontract.TaskPayload {
	t.Helper()
	var response aghcontract.TaskResponse
	request := aghcontract.CreateTaskChildRequest{
		Scope:     taskpkg.ScopeWorkspace,
		Workspace: harness.WorkspaceID,
		Title:     title,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindPool,
			Ref:  ownerRef,
		},
	}
	path := "/api/tasks/" + url.PathEscape(parentTaskID) + "/children"
	if err := harness.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("UDS create owned child task error = %v", err)
	}
	if response.Task.ID == "" || response.Task.ParentTaskID != parentTaskID {
		t.Fatalf("UDS create owned child task response = %#v, want child of %s", response, parentTaskID)
	}
	if response.Task.Owner == nil ||
		response.Task.Owner.Kind.Normalize() != taskpkg.OwnerKindPool ||
		strings.TrimSpace(response.Task.Owner.Ref) != ownerRef {
		t.Fatalf("UDS create owned child task owner = %#v, want pool %q", response.Task.Owner, ownerRef)
	}
	return response.Task
}

type watchEventsAgentLease struct {
	session aghcontract.SessionPayload
	claim   aghcontract.AgentTaskClaimPayload
}

func claimTaskRunViaAgentUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
) watchEventsAgentLease {
	t.Helper()
	session := createFixtureBackedSession(t, ctx, harness, "general", "watch-events-worker")
	waitForRuntimeCondition(t, "agent session active", 5*time.Second, func() bool {
		current, err := harness.GetSession(ctx, session.ID)
		return err == nil && current.ID == session.ID && strings.TrimSpace(string(current.State)) == "active"
	})
	if strings.TrimSpace(session.ID) == "" {
		t.Fatalf("agent session id is empty")
	}
	run := enqueueTaskRunViaUDS(t, ctx, harness, taskID, "")

	var response aghcontract.AgentTaskClaimResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		session,
		http.MethodPost,
		"/api/agent/tasks/claim-next",
		aghcontract.AgentTaskClaimNextRequest{
			WorkspaceID:    harness.WorkspaceID,
			LeaseSeconds:   30,
			IdempotencyKey: "agent-claim-" + run.ID,
		},
		&response,
	)
	if response.Claim.Run.ID != run.ID || response.Claim.Run.TaskID != taskID {
		t.Fatalf("agent claim = %#v, want run %s for task %s", response.Claim, run.ID, taskID)
	}
	return watchEventsAgentLease{session: session, claim: response.Claim}
}

func completeClaimedTaskRunViaAgentUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	lease watchEventsAgentLease,
) aghcontract.TaskRunLeaseSummaryPayload {
	t.Helper()
	var response aghcontract.AgentTaskLeaseResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		lease.session,
		http.MethodPost,
		"/api/agent/tasks/"+url.PathEscape(lease.claim.Run.ID)+"/complete",
		aghcontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
		&response,
	)
	if response.Lease.RunID != lease.claim.Run.ID ||
		response.Lease.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("agent complete lease = %#v, want completed run %s", response.Lease, lease.claim.Run.ID)
	}
	return response.Lease
}

func agentUDSJSON(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	session aghcontract.SessionPayload,
	method string,
	path string,
	body any,
	dest any,
) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal agent UDS request error = %v", err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, harness.UDSURL(path), reader)
	if err != nil {
		t.Fatalf("NewRequest(agent UDS %s %s) error = %v", method, path, err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set(agentidentity.HeaderSessionID, session.ID)
	request.Header.Set(agentidentity.HeaderAgent, session.AgentName)
	request.Header.Set(agentidentity.HeaderWorkspaceID, harness.WorkspaceID)
	response, err := harness.UDSClient.Do(request)
	if err != nil {
		t.Fatalf("agent UDS %s %s error = %v", method, path, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatalf("close agent UDS response body error = %v", closeErr)
		}
	}()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read agent UDS response body error = %v", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		t.Fatalf("agent UDS %s %s status = %d body = %s", method, path, response.StatusCode, string(payload))
	}
	if dest == nil || len(payload) == 0 {
		return
	}
	if err := json.Unmarshal(payload, dest); err != nil {
		t.Fatalf("decode agent UDS response error = %v body = %s", err, string(payload))
	}
}

func enqueueTaskRunViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	taskID string,
	networkChannel string,
) aghcontract.TaskRunPayload {
	t.Helper()
	var response aghcontract.TaskRunResponse
	path := "/api/tasks/" + url.PathEscape(taskID) + "/runs"
	request := aghcontract.EnqueueTaskRunRequest{
		IdempotencyKey:       "watch-events-" + taskID,
		NetworkParticipation: daemonTestNamedParticipationRequest(networkChannel),
	}
	if err := harness.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("UDS enqueue task run error = %v", err)
	}
	if response.Run.ID == "" || response.Run.TaskID != taskID {
		t.Fatalf("UDS enqueue task run response = %#v, want task %s", response, taskID)
	}
	return response.Run
}

func blockTaskThroughStoreForWatchEventsE2E(
	t testing.TB,
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	taskID string,
) {
	t.Helper()
	db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(%q) error = %v", homePaths.DatabaseFile, err)
	}
	defer func() {
		if closeErr := db.Close(ctx); closeErr != nil {
			t.Fatalf("Close GlobalDB error = %v", closeErr)
		}
	}()
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithManagerNow(func() time.Time { return time.Now().UTC() }),
	)
	if err != nil {
		t.Fatalf("NewManager(offline) error = %v", err)
	}
	actor, err := taskpkg.DeriveHumanActorContext("watch-events-e2e", taskpkg.OriginKindCLI, "watch-events-e2e")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext error = %v", err)
	}
	if _, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
		TaskID: taskID,
		Kind:   taskpkg.BlockKindTransient,
		Reason: "simulated daemon downtime gap",
	}, actor); err != nil {
		t.Fatalf("BlockTask(offline) error = %v", err)
	}
}

func assertWatchEventsMockPrompt(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	agentName string,
	fragments []string,
) {
	t.Helper()
	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) missing", agentName)
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	prompts := acpmock.PromptDiagnostics(records)
	if len(prompts) == 0 {
		t.Fatalf("PromptDiagnostics(%q) = empty", registration.DiagnosticsPath)
	}
	for _, record := range prompts {
		matched := true
		for _, fragment := range fragments {
			if !strings.Contains(record.Prompt, fragment) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("watch-events prompts = %#v, want fragments %#v", prompts, fragments)
}

func waitForLoopRunStatus(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	want aghcontract.LoopRunStatus,
) {
	t.Helper()
	var lastRun aghcontract.LoopRunPayload
	var lastErr error
	waitBudget := 20 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		waitBudget = time.Until(deadline)
		if waitBudget <= 0 {
			t.Fatalf("context deadline already expired while waiting for loop run status %s", want)
		}
	}
	timer := time.NewTimer(waitBudget)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var response aghcontract.LoopRunResponse
		path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loop-runs/" + url.PathEscape(runID)
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			lastErr = err
		} else {
			lastRun = response.Run
			lastErr = nil
			if response.Run.Status == want {
				return
			}
		}
		select {
		case <-timer.C:
			logLoopRunTimeoutDebug(t, harness, runID, lastRun)
			t.Fatalf(
				"timed out waiting for loop run status %s; last status=%s run=%#v err=%v",
				want,
				lastRun.Status,
				lastRun,
				lastErr,
			)
		case <-ticker.C:
		}
	}
}

func logLoopRunTimeoutDebug(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	runID string,
	lastRun aghcontract.LoopRunPayload,
) {
	t.Helper()
	events := readLoopRunSSEForDuration(
		t,
		harness,
		loopRunEventsPath(harness.WorkspaceID, runID, 0),
		200*time.Millisecond,
	)
	t.Logf("loop run timeout events = %v", compactLoopRunEvents(events))
	for _, taskID := range appendLoopRunDebugTaskIDs(loopRunInputTaskIDs(lastRun.Inputs), loopRunEventTaskIDs(events)...) {
		debugCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		runs, err := harness.ListTaskRuns(debugCtx, taskID, url.Values{})
		cancel()
		if err != nil {
			t.Logf("loop run timeout task %s runs error = %v", taskID, err)
			continue
		}
		t.Logf("loop run timeout task %s runs = %#v", taskID, runs)
	}
}

func compactLoopRunEvents(events []loopRunSSEEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, fmt.Sprintf("%d:%s:%s", event.Seq, event.Kind, string(event.Payload)))
	}
	return out
}

func loopRunEventTaskIDs(events []loopRunSSEEvent) []string {
	var out []string
	for _, event := range events {
		switch event.Kind {
		case aghcontract.LoopRunEventNodeRunning,
			aghcontract.LoopRunEventNodeSucceeded,
			aghcontract.LoopRunEventNodeFailed:
		default:
			continue
		}
		var payload struct {
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(payload.TaskID); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func appendLoopRunDebugTaskIDs(base []string, extra ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, taskID := range append(base, extra...) {
		trimmed := strings.TrimSpace(taskID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func loopRunInputTaskIDs(inputs map[string]any) []string {
	var out []string
	for _, key := range []string{"parent_task_id", "child_task_id"} {
		value, ok := inputs[key].(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func loopRunEventsPath(workspaceID string, runID string, afterSeq int64) string {
	path := "/api/workspaces/" + url.PathEscape(workspaceID) + "/loop-runs/" + url.PathEscape(runID) + "/events"
	if afterSeq > 0 {
		path += "?after_sequence=" + strconv.FormatInt(afterSeq, 10)
	}
	return path
}

type loopRunSSEEvent struct {
	ID          string
	Event       string
	LoopRunID   string
	WorkspaceID string
	Seq         int64
	Kind        aghcontract.LoopRunEventKind
	Payload     json.RawMessage
}

func readLoopRunSSEUntil(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	done func([]loopRunSSEEvent) bool,
) []loopRunSSEEvent {
	t.Helper()
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events, err := streamLoopRunSSE(streamCtx, harness, path, func(events []loopRunSSEEvent) bool {
		if done(events) {
			cancel()
			return true
		}
		return false
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("streamLoopRunSSE(%q) error = %v", path, err)
	}
	if !done(events) {
		t.Fatalf("streamLoopRunSSE(%q) events = %#v, predicate not satisfied", path, events)
	}
	return events
}

func readLoopRunSSEForDuration(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	path string,
	duration time.Duration,
) []loopRunSSEEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	events, err := streamLoopRunSSE(ctx, harness, path, func([]loopRunSSEEvent) bool { return false })
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("streamLoopRunSSE(%q) error = %v", path, err)
	}
	return events
}

func streamLoopRunSSE(
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	path string,
	done func([]loopRunSSEEvent) bool,
) (events []loopRunSSEEvent, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := harness.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close loop SSE response body: %w", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read loop SSE failure response: %w", readErr)
		}
		return nil, fmt.Errorf("loop SSE status %d: %s", resp.StatusCode, bytes.TrimSpace(payload))
	}
	return readLoopRunSSERecords(resp.Body, done)
}

func readLoopRunSSERecords(
	reader io.Reader,
	done func([]loopRunSSEEvent) bool,
) ([]loopRunSSEEvent, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	events := make([]loopRunSSEEvent, 0, 8)
	var id string
	var name string
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			id = ""
			name = ""
			return nil
		}
		event, err := decodeLoopRunSSEEvent(id, name, data.String())
		if err != nil {
			return err
		}
		events = append(events, event)
		id = ""
		name = ""
		data.Reset()
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return events, err
			}
			if done(events) {
				return events, nil
			}
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch key {
		case "id":
			id = value
		case "event":
			name = value
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		}
	}
	if err := flush(); err != nil {
		return events, err
	}
	if err := scanner.Err(); err != nil {
		return events, err
	}
	return events, nil
}

func decodeLoopRunSSEEvent(id string, name string, raw string) (loopRunSSEEvent, error) {
	var payload aghcontract.LoopRunEventPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return loopRunSSEEvent{}, err
	}
	return loopRunSSEEvent{
		ID:          id,
		Event:       name,
		LoopRunID:   payload.LoopRunID,
		WorkspaceID: payload.WorkspaceID,
		Seq:         payload.Seq,
		Kind:        payload.Kind,
		Payload:     payload.Payload,
	}, nil
}

type loopEventKindSet map[string]struct{}

func loopSSEKinds(events []loopRunSSEEvent) loopEventKindSet {
	kinds := make(loopEventKindSet, len(events))
	for _, event := range events {
		kinds[event.Event] = struct{}{}
		kinds[string(event.Kind)] = struct{}{}
	}
	return kinds
}

func (s loopEventKindSet) Contains(kinds ...string) bool {
	for _, kind := range kinds {
		if _, ok := s[kind]; !ok {
			return false
		}
	}
	return true
}

func assertLoopSSEWorkspace(t testing.TB, events []loopRunSSEEvent, workspaceID string, runID string) {
	t.Helper()
	for _, event := range events {
		if event.ID != strconv.FormatInt(event.Seq, 10) {
			t.Fatalf("event id/seq = %q/%d, want matching SSE id", event.ID, event.Seq)
		}
		if event.Event != string(event.Kind) {
			t.Fatalf("event name/kind = %q/%q, want matching named SSE kind", event.Event, event.Kind)
		}
		if event.WorkspaceID != workspaceID || event.LoopRunID != runID {
			t.Fatalf("event workspace/run = %s/%s, want %s/%s", event.WorkspaceID, event.LoopRunID, workspaceID, runID)
		}
	}
}

func assertLoopSSEPayloadContains(
	t testing.TB,
	events []loopRunSSEEvent,
	kind aghcontract.LoopRunEventKind,
	fragment string,
) {
	t.Helper()
	matched := make([]loopRunSSEEvent, 0)
	for _, event := range events {
		if event.Kind != kind {
			continue
		}
		matched = append(matched, event)
		if strings.Contains(string(event.Payload), fragment) {
			return
		}
	}
	if len(matched) > 0 {
		t.Fatalf("%s payloads = %#v, want fragment %q", kind, matched, fragment)
	}
	t.Fatalf("events = %#v, want kind %s", events, kind)
}

func firstLoopEventSeq(
	t testing.TB,
	events []loopRunSSEEvent,
	kind aghcontract.LoopRunEventKind,
) int64 {
	t.Helper()
	for _, event := range events {
		if event.Kind == kind {
			return event.Seq
		}
	}
	t.Fatalf("events = %#v, want kind %s", events, kind)
	return 0
}
