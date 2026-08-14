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

	"github.com/compozy/compozy/internal/agentidentity"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
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
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		eventsPath := loopRunEventsPath(harness.WorkspaceID, run.ID, 0)
		events := readLoopRunSSEUntil(t, ctx, harness, eventsPath, func(events []loopRunSSEEvent) bool {
			return loopSSEKinds(events).Contains(
				string(compozycontract.LoopRunEventStatusChanged),
				string(compozycontract.LoopRunEventNodeRunning),
				string(compozycontract.LoopRunEventNodeSucceeded),
				string(compozycontract.LoopRunEventChannelMsg),
				string(compozycontract.LoopRunEventTokenTick),
			)
		})
		assertLoopSSEWorkspace(t, events, harness.WorkspaceID, run.ID)
		assertLoopSSEPayloadContains(t, events, compozycontract.LoopRunEventChannelMsg, "loop channel result")
		assertLoopSSEPayloadContains(t, events, compozycontract.LoopRunEventTokenTick, `"terminal":true`)

		afterSeq := firstLoopEventSeq(t, events, compozycontract.LoopRunEventNodeRunning)
		resumed := readLoopRunSSEUntil(
			t,
			ctx,
			harness,
			loopRunEventsPath(harness.WorkspaceID, run.ID, afterSeq),
			func(events []loopRunSSEEvent) bool {
				return loopSSEKinds(events).Contains(
					string(compozycontract.LoopRunEventNodeSucceeded),
					string(compozycontract.LoopRunEventTokenTick),
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

	t.Run("Should resume a paused Loop into its next generation after restart", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		homePaths := e2etest.NewHomePaths(t)
		fixturePath := writeLoopControlRestartFixture(t)
		harnessOptions := e2etest.RuntimeHarnessOptions{
			HomePaths: homePaths,
			Workspace: e2etest.WorkspaceSeedOptions{Root: homePaths.HomeDir},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  fixturePath,
				FixtureAgent: "loop_control_restart",
				AgentName:    "loop-control-restart-agent",
			}},
		}
		harness := e2etest.StartRuntimeHarness(t, &harnessOptions)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		definition := loopEventsDefinition()
		definition.Meta.Name = "loop-control-restart"
		definition.Meta.Description = "Runtime E2E probe for durable Loop resume."
		definition.Contract.StopWhen = "generation > 1"
		definition.Contract.IterationCap = 3
		definition.Graph.Nodes[0].Params["agent"] = "loop-control-restart-agent"
		definition.Graph.Nodes[0].Params["prompt"] = "loop control restart probe"
		createLoopViaHTTP(t, ctx, harness, definition)
		run := runLoopViaHTTP(t, ctx, harness, definition.Meta.Name)
		mutateLoopRunViaHTTP(t, ctx, harness, run.ID, "pause")
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusPaused)

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := harness.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("Stop runtime harness error = %v", err)
		}
		stopCancel()

		restarted := e2etest.StartRuntimeHarness(t, &harnessOptions)
		mutateLoopRunViaHTTP(t, ctx, restarted, run.ID, "resume")
		waitForLoopRunStatus(t, ctx, restarted, run.ID, compozycontract.LoopRunStatusDone)

		var detail compozycontract.LoopRunResponse
		path := "/api/workspaces/" + url.PathEscape(restarted.WorkspaceID) +
			"/loop-runs/" + url.PathEscape(run.ID)
		if err := restarted.HTTPJSON(ctx, http.MethodGet, path, nil, &detail); err != nil {
			t.Fatalf("HTTP get resumed loop error = %v", err)
		}
		if detail.Run.Generation != 2 {
			t.Fatalf("resumed loop generation = %d, want 2", detail.Run.Generation)
		}
	})

	t.Run("Should restore ordered awaited children after restart", func(t *testing.T) {
		t.Parallel()

		homePaths := e2etest.NewHomePaths(t)
		harnessOptions := e2etest.RuntimeHarnessOptions{
			HomePaths: homePaths,
			Workspace: e2etest.WorkspaceSeedOptions{Root: homePaths.HomeDir},
		}
		harness := e2etest.StartRuntimeHarness(t, &harnessOptions)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		createLoopViaHTTP(t, ctx, harness, awaitedChildHoldDefinition())
		createLoopViaHTTP(t, ctx, harness, awaitedParentProbeDefinition())
		parent := runLoopViaHTTP(t, ctx, harness, "await-parent-probe")
		beforeRestart, firstChildID := waitForAwaitedChildNode(
			t,
			ctx,
			harness,
			parent.ID,
			"first_child",
			1,
		)
		assertAwaitedParentNodeStatus(t, beforeRestart, "second_child", "pending")

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := harness.Stop(stopCtx); err != nil {
			stopCancel()
			t.Fatalf("Stop runtime harness error = %v", err)
		}
		stopCancel()

		restarted := e2etest.StartRuntimeHarness(t, &harnessOptions)
		afterRestart, restartedChildID := waitForAwaitedChildNode(
			t,
			ctx,
			restarted,
			parent.ID,
			"first_child",
			1,
		)
		if restartedChildID != firstChildID {
			t.Fatalf("restarted child id = %q, want original %q", restartedChildID, firstChildID)
		}
		assertAwaitedParentNodeStatus(t, afterRestart, "second_child", "pending")

		resumeLoopWaitViaHTTP(t, ctx, restarted, firstChildID, "hold")
		withSecondChild, secondChildID := waitForAwaitedChildNode(
			t,
			ctx,
			restarted,
			parent.ID,
			"second_child",
			2,
		)
		if secondChildID == firstChildID {
			t.Fatalf("second child id = first child id %q, want distinct run", secondChildID)
		}
		assertAwaitedParentNodeStatus(t, withSecondChild, "first_child", "succeeded")

		resumeLoopWaitViaHTTP(t, ctx, restarted, secondChildID, "hold")
		waitForLoopRunStatus(t, ctx, restarted, parent.ID, compozycontract.LoopRunStatusDone)
		children := awaitedChildRunsViaHTTP(t, ctx, restarted)
		if len(children) != 2 {
			t.Fatalf("terminal child runs = %d, want 2", len(children))
		}
		for _, child := range children {
			if child.Status != compozycontract.LoopRunStatusDone {
				t.Fatalf("terminal child %q status = %q, want done", child.ID, child.Status)
			}
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
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusWatching)
		assertWatchEventsReadModelParity(t, ctx, harness, run.ID)
		foreignWorkspaceID := assertForeignWorkspaceLoopRunNotFound(t, ctx, harness, run.ID)
		quietBaseline := readLoopRunDetailViaHTTP(t, ctx, harness, run.ID)
		quietPromptCount := watchEventsMockPromptCount(t, harness, "watch-events-agent")

		unrelatedParent := createTaskViaUDS(t, ctx, harness, "Unrelated parent")
		unrelatedChild := createChildTaskViaUDS(t, ctx, harness, unrelatedParent.ID, "Unrelated child")
		blockTaskThroughStoreForWatchEventsE2E(t, ctx, harness.HomePaths, unrelatedChild.ID)
		quietBaseline = waitForWatchEventsCursorAdvanceAndRemainParked(
			t,
			ctx,
			harness,
			run.ID,
			quietBaseline,
			quietPromptCount,
			looppkg.WatchEventsTaskStream,
		)

		foreignParent := createTaskViaUDSInWorkspace(t, ctx, harness, foreignWorkspaceID, "Foreign parent")
		foreignChild := createChildTaskViaUDSInWorkspace(
			t,
			ctx,
			harness,
			foreignWorkspaceID,
			foreignParent.ID,
			"Foreign child",
		)
		blockTaskThroughStoreForWatchEventsE2E(t, ctx, harness.HomePaths, foreignChild.ID)
		assertNoWatchEventsGapWake(
			t,
			ctx,
			harness,
			run.ID,
			quietBaseline,
			quietPromptCount,
		)

		lease := claimTaskRunViaAgentUDS(t, ctx, harness, child.ID)
		blockTaskThroughStoreForWatchEventsE2E(t, ctx, harness.HomePaths, blockedChild.ID)
		completeClaimedTaskRunViaAgentUDS(t, ctx, harness, lease)

		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)
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
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusWatching)
		assertWatchEventsReadModelParity(t, ctx, harness, run.ID)

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

		waitForLoopRunStatus(t, ctx, restarted, run.ID, compozycontract.LoopRunStatusDone)
		assertWatchEventsMockPrompt(t, restarted, "watch-events-agent", []string{
			"task.status_changed",
			child.ID,
		})
	})
}

func loopEventsDefinition() compozycontract.LoopDefinitionDocument {
	return compozycontract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: compozycontract.LoopDefinitionMeta{
			Name:        "loop-events-probe",
			Description: "Runtime E2E probe for rich Loop run SSE events.",
			Catalog: compozycontract.LoopCatalogMeta{
				UseWhen:  "Testing Loop run event streaming.",
				Keywords: []string{"test", "events"},
				Category: "Testing",
			},
		},
		Contract: compozycontract.LoopContract{
			Goal:             "Emit rich Loop run events.",
			DefinitionOfDone: "The probe action completes.",
			StopWhen:         "nodes.probe.status == 'succeeded'",
			IterationCap:     1,
			NoProgress: compozycontract.LoopNoProgress{
				Window:     2,
				HashFields: []string{"delivery_artifact"},
			},
			Budget: compozycontract.LoopBudget{
				Tokens:       0,
				WallClockSec: 0,
				OnExceeded:   compozycontract.LoopBudgetExceededHalt,
			},
			TerminalStates: []string{"done", "failed", "blocked", "exhausted", "stalled"},
		},
		Graph: compozycontract.LoopGraph{
			Nodes: []compozycontract.LoopGraphNode{{
				ID:    "probe",
				Class: compozycontract.LoopNodeClassAction,
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
		Start: []compozycontract.LoopStartBinding{
			{Kind: "manual"},
			{Kind: "http"},
			{Kind: "uds"},
		},
	}
}

func awaitedChildHoldDefinition() compozycontract.LoopDefinitionDocument {
	return compozycontract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: compozycontract.LoopDefinitionMeta{
			Name:        "await-child-hold",
			Description: "Generic child Loop held by a durable wait.",
		},
		Contract: compozycontract.LoopContract{
			Goal:             "Hold a child run in a live state.",
			DefinitionOfDone: "The durable wait is released.",
			StopWhen:         "nodes.hold.status == 'succeeded'",
			IterationCap:     1,
			NoProgress: compozycontract.LoopNoProgress{
				Window:     2,
				HashFields: []string{"nodes.hold.status"},
			},
			Budget: compozycontract.LoopBudget{
				OnExceeded: compozycontract.LoopBudgetExceededHalt,
			},
			TerminalStates: []string{"done", "failed", "blocked", "exhausted", "stalled"},
		},
		Graph: compozycontract.LoopGraph{
			Nodes: []compozycontract.LoopGraphNode{{
				ID:     "hold",
				Class:  compozycontract.LoopNodeClassControl,
				Kind:   "wait",
				Params: map[string]any{"for": "10m"},
			}},
		},
		Start: []compozycontract.LoopStartBinding{{Kind: "manual"}, {Kind: "http"}},
	}
}

func awaitedParentProbeDefinition() compozycontract.LoopDefinitionDocument {
	return compozycontract.LoopDefinitionDocument{
		APIVersion:  "compozy.loop/v1",
		Kind:        "Loop",
		Concurrency: "allow",
		Meta: compozycontract.LoopDefinitionMeta{
			Name:        "await-parent-probe",
			Description: "Generic parent with two sequential awaited child Loops.",
		},
		Contract: compozycontract.LoopContract{
			Goal:             "Run two child Loops in strict sequence.",
			DefinitionOfDone: "Both awaited child Loops finish in authored order.",
			StopWhen:         "nodes.second_child.status == 'succeeded'",
			IterationCap:     1,
			NoProgress: compozycontract.LoopNoProgress{
				Window:     2,
				HashFields: []string{"nodes.first_child.status", "nodes.second_child.status"},
			},
			Budget: compozycontract.LoopBudget{
				OnExceeded: compozycontract.LoopBudgetExceededHalt,
			},
			TerminalStates: []string{"done", "failed", "blocked", "exhausted", "stalled"},
		},
		Graph: compozycontract.LoopGraph{
			Nodes: []compozycontract.LoopGraphNode{
				{
					ID:    "first_child",
					Class: compozycontract.LoopNodeClassAction,
					Kind:  "run-loop",
					Params: map[string]any{
						"loop": "await-child-hold",
						"mode": "await",
					},
				},
				{
					ID:    "second_child",
					Class: compozycontract.LoopNodeClassAction,
					Kind:  "run-loop",
					Params: map[string]any{
						"loop": "await-child-hold",
						"mode": "await",
					},
				},
			},
			Edges: []compozycontract.LoopGraphEdge{{From: "first_child", To: "second_child"}},
		},
		Start: []compozycontract.LoopStartBinding{{Kind: "manual"}, {Kind: "http"}},
	}
}

func waitForAwaitedChildNode(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	parentRunID string,
	nodeID string,
	wantChildRuns int,
) (compozycontract.LoopRunResponse, string) {
	t.Helper()
	var detail compozycontract.LoopRunResponse
	var childRunID string
	waitForRuntimeCondition(t, "awaited child node "+nodeID, 20*time.Second, func() bool {
		childRunID = ""
		detail = readLoopRunDetailViaHTTP(t, ctx, harness, parentRunID)
		if detail.Run.Status != compozycontract.LoopRunStatusRunning {
			return false
		}
		for _, generation := range detail.Generations {
			for _, output := range generation.Outputs {
				if output.NodeID == nodeID && output.Status == "awaiting_child" {
					childRunID = output.ChildLoopRunID
				}
			}
		}
		return childRunID != "" && len(awaitedChildRunsViaHTTP(t, ctx, harness)) == wantChildRuns
	})
	return detail, childRunID
}

func awaitedChildRunsViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) []compozycontract.LoopRunPayload {
	t.Helper()
	var response compozycontract.LoopRunsResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loop-runs"
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("HTTP list loop runs error = %v", err)
	}
	children := make([]compozycontract.LoopRunPayload, 0, len(response.Runs))
	for _, run := range response.Runs {
		if run.LoopName == "await-child-hold" {
			children = append(children, run)
		}
	}
	return children
}

func assertAwaitedParentNodeStatus(
	t testing.TB,
	detail compozycontract.LoopRunResponse,
	nodeID string,
	want string,
) {
	t.Helper()
	for _, generation := range detail.Generations {
		for _, output := range generation.Outputs {
			if output.NodeID == nodeID {
				if output.Status != want {
					t.Fatalf("%s status = %q, want %q", nodeID, output.Status, want)
				}
				return
			}
		}
	}
	t.Fatalf("node %q missing from parent detail", nodeID)
}

func resumeLoopWaitViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	nodeID string,
) {
	t.Helper()
	var response compozycontract.LoopMutationResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID) + "/nodes/" + url.PathEscape(nodeID) + "/resume"
	request := compozycontract.LoopNodeResumeRequest{Payload: json.RawMessage(`{}`)}
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("HTTP resume loop wait error = %v", err)
	}
	if !response.OK {
		t.Fatalf("HTTP resume loop wait response = %#v, want ok", response)
	}
}

func writeLoopControlRestartFixture(t testing.TB) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "loop-control-restart-fixture.json")
	fixture := map[string]any{
		"version": 2,
		"agents": []map[string]any{{
			"name":        "loop_control_restart",
			"provider":    "acpmock",
			"permissions": "approve-all",
			"prompt":      "You are a deterministic Loop control restart worker.",
			"turns": []map[string]any{{
				"name": "loop-control-restart",
				"match": map[string]any{
					"turn_source": "user",
					"user_text":   "loop control restart probe",
				},
				"steps": []map[string]any{
					{
						"kind": "driver_control",
						"driver_control": map[string]any{
							"action":   "delay",
							"delay_ms": 750,
						},
					},
					{
						"kind": "assistant",
						"text": "```json\n{\"summary\":\"control generation complete\",\"message\":\"continue\"}\n```",
					},
				},
			}},
		}},
	}
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("marshal Loop control restart fixture error = %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write Loop control restart fixture error = %v", err)
	}
	return path
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
	root := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.LoopsDirName)
	if _, _, err := looppkg.WriteDefinition(root, []byte(watchEventsE2ELoopYAML()), looppkg.WriteDefinitionOptions{
		Source: looppkg.SourceWorkspace,
	}); err != nil {
		t.Fatalf("WriteDefinition(watch-events loop) error = %v", err)
	}
}

func watchEventsE2ELoopYAML() string {
	return `apiVersion: compozy.loop/v1
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
	def compozycontract.LoopDefinitionDocument,
) {
	t.Helper()
	var response compozycontract.LoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops"
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		path,
		compozycontract.CreateLoopRequest{Definition: &def},
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
		var response compozycontract.LoopsResponse
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
) compozycontract.LoopRunPayload {
	t.Helper()
	var response compozycontract.RunLoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops/" + url.PathEscape(name) + "/run"
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, compozycontract.RunLoopRequest{}, &response); err != nil {
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
) compozycontract.LoopRunPayload {
	t.Helper()
	var response compozycontract.RunLoopResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loops/" + url.PathEscape(name) + "/run"
	request := compozycontract.RunLoopRequest{Inputs: inputs}
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		t.Fatalf("HTTP run loop with inputs error = %v", err)
	}
	if response.Run == nil {
		t.Fatalf("HTTP run loop response = %#v, want run", response)
	}
	return *response.Run
}

func mutateLoopRunViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	action string,
) {
	t.Helper()
	var response struct {
		OK bool `json:"ok"`
	}
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID) + "/" + url.PathEscape(action)
	if err := harness.HTTPJSON(ctx, http.MethodPost, path, nil, &response); err != nil {
		t.Fatalf("HTTP %s loop error = %v", action, err)
	}
	if !response.OK {
		t.Fatalf("HTTP %s loop response = %#v, want ok", action, response)
	}
}

func createTaskViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	title string,
) compozycontract.TaskPayload {
	t.Helper()
	return createTaskViaUDSInWorkspace(t, ctx, harness, harness.WorkspaceID, title)
}

func createWorkspaceViaUDS(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	rootDir string,
	name string,
) string {
	t.Helper()
	var response compozycontract.WorkspaceResponse
	if err := harness.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/workspaces",
		compozycontract.CreateWorkspaceRequest{RootDir: rootDir, Name: name},
		&response,
	); err != nil {
		t.Fatalf("UDS create workspace error = %v", err)
	}
	if response.Workspace.ID == "" || response.Workspace.ID == harness.WorkspaceID {
		t.Fatalf("UDS created workspace = %#v, want distinct id", response.Workspace)
	}
	return response.Workspace.ID
}

func createTaskViaUDSInWorkspace(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	title string,
) compozycontract.TaskPayload {
	t.Helper()
	var response compozycontract.TaskResponse
	request := compozycontract.CreateTaskRequest{
		Scope:     taskpkg.ScopeWorkspace,
		Workspace: workspaceID,
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
) compozycontract.TaskPayload {
	t.Helper()
	return createChildTaskViaUDSInWorkspace(t, ctx, harness, harness.WorkspaceID, parentTaskID, title)
}

func createChildTaskViaUDSInWorkspace(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspaceID string,
	parentTaskID string,
	title string,
) compozycontract.TaskPayload {
	t.Helper()
	var response compozycontract.TaskResponse
	request := compozycontract.CreateTaskChildRequest{
		Scope:     taskpkg.ScopeWorkspace,
		Workspace: workspaceID,
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
) compozycontract.TaskPayload {
	t.Helper()
	var response compozycontract.TaskResponse
	request := compozycontract.CreateTaskChildRequest{
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
	session compozycontract.SessionPayload
	claim   compozycontract.AgentTaskClaimPayload
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

	var response compozycontract.AgentTaskClaimResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		session,
		http.MethodPost,
		"/api/agent/tasks/claim-next",
		compozycontract.AgentTaskClaimNextRequest{
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
) compozycontract.TaskRunLeaseSummaryPayload {
	t.Helper()
	var response compozycontract.AgentTaskLeaseResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		lease.session,
		http.MethodPost,
		"/api/agent/tasks/"+url.PathEscape(lease.claim.Run.ID)+"/complete",
		compozycontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"ok":true}`)},
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
	session compozycontract.SessionPayload,
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
) compozycontract.TaskRunPayload {
	t.Helper()
	var response compozycontract.TaskRunResponse
	path := "/api/tasks/" + url.PathEscape(taskID) + "/runs"
	request := compozycontract.EnqueueTaskRunRequest{
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
	homePaths compozyconfig.HomePaths,
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

func assertWatchEventsReadModelParity(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) {
	t.Helper()

	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID)
	var httpDetail compozycontract.LoopRunResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpDetail); err != nil {
		t.Fatalf("HTTP get parked loop error = %v", err)
	}
	if httpDetail.WatchEvents == nil || len(httpDetail.WatchEvents.Subscriptions) == 0 {
		t.Fatalf("HTTP parked watch-events read-model = %#v, want subscriptions", httpDetail.WatchEvents)
	}
	if len(httpDetail.WatchEvents.Cursors) == 0 {
		t.Fatalf("HTTP parked watch-events read-model = %#v, want durable cursors", httpDetail.WatchEvents)
	}
	if httpDetail.WatchEvents.LastWakeAt == nil {
		t.Fatalf("HTTP parked watch-events read-model = %#v, want last_wake_at", httpDetail.WatchEvents)
	}

	var udsDetail compozycontract.LoopRunResponse
	if err := harness.UDSJSON(ctx, http.MethodGet, path, nil, &udsDetail); err != nil {
		t.Fatalf("UDS get parked loop error = %v", err)
	}

	var cliDetail compozycontract.LoopRunResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&cliDetail,
		"loop", "status",
		"--workspace", harness.WorkspaceID,
		"--run-id", runID,
		"-o", "json",
	); err != nil {
		t.Fatalf("CLI get parked loop error = %v", err)
	}
	httpJSON, err := json.Marshal(httpDetail.WatchEvents)
	if err != nil {
		t.Fatalf("marshal HTTP parked watch-events read-model error = %v", err)
	}
	cliJSON, err := json.Marshal(cliDetail.WatchEvents)
	if err != nil {
		t.Fatalf("marshal CLI parked watch-events read-model error = %v", err)
	}
	if !bytes.Equal(cliJSON, httpJSON) {
		t.Fatalf("CLI parked watch-events read-model = %s, want HTTP %s", cliJSON, httpJSON)
	}
	udsJSON, err := json.Marshal(udsDetail.WatchEvents)
	if err != nil {
		t.Fatalf("marshal UDS parked watch-events read-model error = %v", err)
	}
	if !bytes.Equal(udsJSON, httpJSON) {
		t.Fatalf("UDS parked watch-events read-model = %s, want HTTP %s", udsJSON, httpJSON)
	}

	nativeDetail := invokeLoopRuntimeNativeStatus(t, ctx, harness, harness.WorkspaceID, runID)
	nativeJSON, err := json.Marshal(nativeDetail.WatchEvents)
	if err != nil {
		t.Fatalf("marshal native parked watch-events read-model error = %v", err)
	}
	if !bytes.Equal(nativeJSON, httpJSON) {
		t.Fatalf("native parked watch-events read-model = %s, want HTTP %s", nativeJSON, httpJSON)
	}
}

func assertForeignWorkspaceLoopRunNotFound(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) string {
	t.Helper()

	foreignWorkspaceID := createWorkspaceViaUDS(t, ctx, harness, t.TempDir(), "foreign-workspace")
	foreignPath := "/api/workspaces/" + url.PathEscape(foreignWorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID)
	var foreign map[string]any
	status := loopRuntimeRawJSON(
		t,
		ctx,
		harness.HTTPClient,
		harness.HTTPURL(foreignPath),
		http.MethodGet,
		nil,
		&foreign,
	)
	if status != http.StatusNotFound {
		t.Fatalf("foreign workspace read = status:%d body:%#v, want 404", status, foreign)
	}
	return foreignWorkspaceID
}

func readLoopRunDetailViaHTTP(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) compozycontract.LoopRunResponse {
	t.Helper()
	var response compozycontract.LoopRunResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID)
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("HTTP get loop detail error = %v", err)
	}
	return response
}

func waitForWatchEventsCursorAdvanceAndRemainParked(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	baseline compozycontract.LoopRunResponse,
	promptCount int,
	stream string,
) compozycontract.LoopRunResponse {
	t.Helper()
	if baseline.WatchEvents == nil {
		t.Fatal("baseline parked watch-events read-model = nil")
	}
	baselineCursor := baseline.WatchEvents.Cursors[stream]
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		response := readLoopRunDetailViaHTTP(t, ctx, harness, runID)
		if loopRunStatusTerminal(response.Run.Status) {
			t.Fatalf("loop after non-matching event = terminal status %s, want watching", response.Run.Status)
		}
		if current := watchEventsMockPromptCount(t, harness, "watch-events-agent"); current != promptCount {
			t.Fatalf("watch-events prompt count after non-matching event = %d, want %d", current, promptCount)
		}
		if response.Run.Status == compozycontract.LoopRunStatusWatching &&
			response.WatchEvents != nil &&
			response.WatchEvents.Cursors[stream] > baselineCursor {
			return response
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context ended before non-matching event advanced cursor: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertNoWatchEventsGapWake(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	baseline compozycontract.LoopRunResponse,
	promptCount int,
) {
	t.Helper()
	db, err := globaldb.OpenGlobalDB(ctx, harness.HomePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(%q) error = %v", harness.HomePaths.DatabaseFile, err)
	}
	defer func() {
		if closeErr := db.Close(ctx); closeErr != nil {
			t.Fatalf("Close GlobalDB error = %v", closeErr)
		}
	}()
	actor, err := taskpkg.DeriveDaemonActorContext("watch-events-e2e", "watch_events_gap")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext error = %v", err)
	}
	runs, err := db.EnqueueWatchEventsGapWakes(ctx, actor.Origin, time.Now().UTC())
	if err != nil {
		t.Fatalf("EnqueueWatchEventsGapWakes error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("foreign-workspace gap wakes = %#v, want none", runs)
	}
	response := readLoopRunDetailViaHTTP(t, ctx, harness, runID)
	if response.Run.Status != compozycontract.LoopRunStatusWatching ||
		response.Run.Generation != baseline.Run.Generation {
		t.Fatalf(
			"loop after foreign event = status:%s generation:%d, want watching generation:%d",
			response.Run.Status,
			response.Run.Generation,
			baseline.Run.Generation,
		)
	}
	if current := watchEventsMockPromptCount(t, harness, "watch-events-agent"); current != promptCount {
		t.Fatalf("watch-events prompt count after foreign event = %d, want %d", current, promptCount)
	}
}

func watchEventsMockPromptCount(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	agentName string,
) int {
	t.Helper()
	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) missing", agentName)
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0
		}
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	return len(acpmock.PromptDiagnostics(records))
}

func waitForLoopRunStatus(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
	want compozycontract.LoopRunStatus,
) {
	t.Helper()
	var lastRun compozycontract.LoopRunPayload
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
		var response compozycontract.LoopRunResponse
		path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/loop-runs/" + url.PathEscape(runID)
		if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
			lastErr = err
		} else {
			lastRun = response.Run
			lastErr = nil
			if response.Run.Status == want {
				return
			}
			if loopRunStatusTerminal(response.Run.Status) {
				logLoopRunTimeoutDebug(t, harness, runID, lastRun)
				t.Fatalf(
					"loop run reached terminal status %s while waiting for %s; run=%#v",
					response.Run.Status,
					want,
					response.Run,
				)
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

func loopRunStatusTerminal(status compozycontract.LoopRunStatus) bool {
	switch status {
	case compozycontract.LoopRunStatusDone,
		compozycontract.LoopRunStatusNoOp,
		compozycontract.LoopRunStatusBlocked,
		compozycontract.LoopRunStatusFailed,
		compozycontract.LoopRunStatusExhausted,
		compozycontract.LoopRunStatusStalled:
		return true
	default:
		return false
	}
}

func logLoopRunTimeoutDebug(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	runID string,
	lastRun compozycontract.LoopRunPayload,
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
		case compozycontract.LoopRunEventNodeRunning,
			compozycontract.LoopRunEventNodeSucceeded,
			compozycontract.LoopRunEventNodeFailed:
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
	Kind        compozycontract.LoopRunEventKind
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
	var payload compozycontract.LoopRunEventPayload
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
	kind compozycontract.LoopRunEventKind,
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
	kind compozycontract.LoopRunEventKind,
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
