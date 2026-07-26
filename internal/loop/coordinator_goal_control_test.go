package loop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func TestCoordinatorRunnerShouldQueueInitialGoalSegmentThroughWorkerPath(t *testing.T) {
	t.Run("Should suffix the first Goal worker identity with control epoch one", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, goalCoordinatorDefinition())
		loopRun := controlLoopRun("looprun-goal-initial-segment", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		run := plan.NodeRuns[0]
		generation := plan.Snapshot.Generation
		if got, want := run.RunKind, task.RunKindWorker; got != want {
			t.Fatalf("run kind = %q, want %q", got, want)
		}
		if got, want := run.RunID, GoalSegmentRunID(loopRun.ID, generation, "converge", 0, 1); got != want {
			t.Fatalf("run id = %q, want %q", got, want)
		}
		wantIdempotencyKey := GoalSegmentIdempotencyKey(loopRun.ID, generation, "converge", 0, 1)
		if got, want := run.IdempotencyKey, wantIdempotencyKey; got != want {
			t.Fatalf("idempotency key = %q, want %q", got, want)
		}
		var metadata map[string]any
		if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if got, want := int64(metadata["goal_segment_epoch"].(float64)), int64(1); got != want {
			t.Fatalf("goal_segment_epoch = %d, want %d", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldPreserveMixedGenerationAtGoalControlBoundary(t *testing.T) {
	t.Run("Should mutate only the Goal output without consuming a watch park or hiding live work", func(t *testing.T) {
		t.Parallel()

		definition := goalCoordinatorDefinition()
		watchDefinition := watchEventsDefinitionForTest("")
		definition.Graph.Nodes = append(definition.Graph.Nodes, watchDefinition.Graph.Nodes...)
		definition.Graph.Nodes = append(definition.Graph.Nodes, dsl.Node{
			ID:    "archive",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"archived": map[string]any{"value": true}},
			},
		})
		definition.Graph.Edges = append(definition.Graph.Edges, watchDefinition.Graph.Edges...)
		definition.Graph.Edges = append(
			definition.Graph.Edges,
			dsl.Edge{From: "watch_tasks", To: "converge"},
			dsl.Edge{From: "watch_tasks", To: "archive"},
		)
		resolved := compileCoordinatorControlDefinition(t, definition)
		loopRun := controlLoopRun("looprun-goal-mixed-generation", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		goalRun := task.Run{
			ID:        GoalSegmentRunID(loopRun.ID, 1, "converge", 0, 1),
			TaskID:    GoalNodeTaskID(loopRun.ID, 1, "converge", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusCompleted,
			Result: goalControlRunResult(t, ActionControl{
				Disposition:  ActionDispositionPaused,
				GoalStatus:   "paused",
				Cause:        "operator_pause",
				CheckpointID: "checkpoint-mixed-generation",
			}),
		}
		liveRun := controlWorkerRun(loopRun, "summarize", 0, task.TaskRunStatusRunning)
		watchRef := watchEventsPendingRefForTest(t, 7, "")
		ledger := &watchEventsLedgerForTest{
			rows: watchLargeTaskStatusEventsForTest(8, LoopMaxFanoutWidth),
		}
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				goalRun.ID:        goalRun,
				liveRun.ID:        liveRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "converge",
					Status:     generationOutputControlPending,
					TaskRunID:  goalRun.ID,
				},
				{
					Generation: 1,
					NodeID:     "watch_tasks",
					Status:     generationOutputPending,
					OutputRef:  watchRef,
				},
				{
					Generation: 1,
					NodeID:     "summarize",
					Status:     generationOutputEnqueued,
					TaskRunID:  liveRun.ID,
				},
				{
					Generation: 1,
					NodeID:     "archive",
					Status:     generationOutputPending,
				},
			}}},
			resolved,
		)
		runner.watchEventsLedger = ledger

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusPaused) {
			t.Fatalf("Terminal = %#v, want paused", plan.Terminal)
		}
		if !plan.GenerationInFlight {
			t.Fatal("GenerationInFlight = false, want live sibling preserved")
		}
		outputs := outputsByNodeForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		if outputs["converge"].Status != generationOutputAwaitingGoal {
			t.Fatalf("Goal output = %#v, want awaiting_goal", outputs["converge"])
		}
		if outputs["watch_tasks"].Status != generationOutputSucceeded ||
			!OutputRefLooksContentAddressed(outputs["watch_tasks"].OutputRef) {
			t.Fatalf("watch output = %#v, want content-addressed confirmation", outputs["watch_tasks"])
		}
		if outputs["summarize"].Status != generationOutputRunning ||
			outputs["summarize"].TaskRunID != liveRun.ID {
			t.Fatalf("live sibling changed = %#v", outputs["summarize"])
		}
		if got, want := len(ledger.cursorQueries), 0; got != want {
			t.Fatalf("watch cursor bootstrap queries = %d, want %d from pinned park", got, want)
		}
		if got, want := len(ledger.matchQueries), 1; got != want {
			t.Fatalf("watch match queries = %d, want %d", got, want)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if got, want := len(payload.OutputBlobs), 1; got != want {
			t.Fatalf("OutputBlobs len = %d, want %d", got, want)
		}
		if payload.OutputBlobs[0].OutputRef != outputs["watch_tasks"].OutputRef {
			t.Fatalf("watch blob ref = %q, want %q", payload.OutputBlobs[0].OutputRef, outputs["watch_tasks"].OutputRef)
		}
		confirmed := decodeWatchEventsOutputRefForTest(t, string(payload.OutputBlobs[0].Payload))
		wantCursor := int64(7 + LoopMaxFanoutWidth)
		if got := confirmed.Cursors[WatchEventsTaskStream]; got != wantCursor {
			t.Fatalf("confirmed watch cursor = %d, want %d", got, wantCursor)
		}
		if got, want := len(plan.PostCommitWakes), 1; got != want {
			t.Fatalf("PostCommitWakes len = %d, want %d", got, want)
		}
		if got, want := plan.PostCommitWakes[0].IdempotencyKey,
			watchEventsWakeKey(loopRun.ID, "watch_tasks"); got != want {
			t.Fatalf("wake key = %q, want %q", got, want)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("NodeRuns len = %d, want %d", got, want)
		}
		if plan.NodeRuns[0].TaskID != coordinatorNodeTaskID(loopRun.ID, 1, "archive", 0) {
			t.Fatalf("reserved sibling = %#v, want archive", plan.NodeRuns[0])
		}
		postReserve := coordinatorPostReservePayloadForTest(t, plan)
		postReserveOutputs := outputsByNodeForTest(postReserve.Outputs)
		if postReserveOutputs["archive"].Status != generationOutputEnqueued {
			t.Fatalf("post-reserve archive output = %#v, want enqueued", postReserveOutputs["archive"])
		}
		if got, want := len(postReserve.OutputBlobs), 1; got != want {
			t.Fatalf("post-reserve OutputBlobs len = %d, want %d", got, want)
		}
		if postReserve.OutputBlobs[0].OutputRef != payload.OutputBlobs[0].OutputRef {
			t.Fatalf(
				"post-reserve blob ref = %q, want %q",
				postReserve.OutputBlobs[0].OutputRef,
				payload.OutputBlobs[0].OutputRef,
			)
		}
	})
}

func TestCoordinatorRunnerShouldPreserveDeferredGoalTerminals(t *testing.T) {
	tests := []struct {
		name           string
		disposition    ActionDisposition
		goalStatus     string
		cause          ReasonCode
		terminalStatus Status
	}{
		{
			name:           "blocked",
			disposition:    ActionDispositionBlocked,
			goalStatus:     "blocked",
			cause:          ReasonCodeGoalReportedBlocked,
			terminalStatus: StatusBlocked,
		},
		{
			name:           "exhausted",
			disposition:    ActionDispositionExhausted,
			goalStatus:     "budget-limited",
			cause:          ReasonCodeGoalTurnsExhausted,
			terminalStatus: StatusExhausted,
		},
	}
	for _, tc := range tests {
		t.Run("Should settle "+tc.name+" after the live sibling completes", func(t *testing.T) {
			t.Parallel()

			definition := goalCoordinatorDefinition()
			definition.Graph.Nodes = append(definition.Graph.Nodes, dsl.Node{
				ID:    "summarize",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunAgent),
				Params: dsl.NodeParams{
					"agent":  "codex",
					"prompt": "Summarize the result",
				},
			})
			resolved := compileCoordinatorControlDefinition(t, definition)
			loopRun := controlLoopRun("looprun-goal-deferred-"+tc.name, map[string]any{})
			coordinatorRun := controlCoordinatorRun(loopRun, 1)
			goalRun := task.Run{
				ID:        GoalSegmentRunID(loopRun.ID, 1, "converge", 0, 1),
				TaskID:    GoalNodeTaskID(loopRun.ID, 1, "converge", 0),
				RunKind:   task.RunKindWorker,
				LoopRunID: string(loopRun.ID),
				Status:    task.TaskRunStatusCompleted,
				Result: goalControlRunResult(t, ActionControl{
					Disposition:  tc.disposition,
					GoalStatus:   tc.goalStatus,
					Cause:        tc.cause,
					CheckpointID: "checkpoint-deferred-" + tc.name,
				}),
			}
			liveRun := controlWorkerRun(loopRun, "summarize", 0, task.TaskRunStatusRunning)
			runs := map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				goalRun.ID:        goalRun,
				liveRun.ID:        liveRun,
			}
			outputStore := coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "converge",
					Status:     generationOutputControlPending,
					TaskRunID:  goalRun.ID,
				},
				{
					Generation: 1,
					NodeID:     "summarize",
					Status:     generationOutputRunning,
					TaskRunID:  liveRun.ID,
				},
			}}}
			runner := newCoordinatorRunnerForControlTest(
				t,
				loopRun,
				coordinatorRun,
				runs,
				outputStore,
				resolved,
			)

			deferred, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run(deferred) error = %v", err)
			}
			if deferred.Terminal == nil || deferred.Terminal.Status != string(tc.terminalStatus) ||
				!deferred.GenerationInFlight {
				t.Fatalf("deferred terminal/in-flight = %#v/%t", deferred.Terminal, deferred.GenerationInFlight)
			}
			deferredOutputs := coordinatorSnapshotPayloadForTest(t, deferred).Outputs
			if got := outputsByNodeForTest(deferredOutputs)["converge"].Status; got != generationOutputControlPending {
				t.Fatalf("deferred Goal status = %q, want %q", got, generationOutputControlPending)
			}

			outputStore.outputs[1] = deferredOutputs
			liveRun.Status = task.TaskRunStatusCompleted
			runs[liveRun.ID] = liveRun
			settled, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run(settled) error = %v", err)
			}
			if settled.Terminal == nil || settled.Terminal.Status != string(tc.terminalStatus) {
				t.Fatalf("settled terminal = %#v, want %q", settled.Terminal, tc.terminalStatus)
			}
			if settled.GenerationInFlight || len(settled.NodeRuns) != 0 || settled.NextCoordinator != nil {
				t.Fatalf(
					"settled plan reattempted: in-flight=%t node-runs=%d next=%#v",
					settled.GenerationInFlight,
					len(settled.NodeRuns),
					settled.NextCoordinator,
				)
			}
			goalOutput := outputsByNodeForTest(coordinatorSnapshotPayloadForTest(t, settled).Outputs)["converge"]
			if goalOutput.Status != generationOutputFailed || goalOutput.OutputRef != string(tc.cause) {
				t.Fatalf("settled Goal output = %#v", goalOutput)
			}
		})
	}
}

func TestResolveGoalActionControlShouldMapEveryDisposition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		control            ActionControl
		wantOutputStatus   string
		wantOutputRef      string
		wantTerminalStatus string
		wantGateID         string
	}{
		{
			name: "succeeded",
			control: ActionControl{
				Disposition:  ActionDispositionSucceeded,
				GoalStatus:   "complete",
				CheckpointID: "checkpoint-succeeded",
			},
			wantOutputStatus: generationOutputSucceeded,
		},
		{
			name: "blocked",
			control: ActionControl{
				Disposition:  ActionDispositionBlocked,
				GoalStatus:   "blocked",
				Cause:        "goal_reported_blocked",
				CheckpointID: "checkpoint-blocked",
			},
			wantOutputStatus:   generationOutputFailed,
			wantOutputRef:      "goal_reported_blocked",
			wantTerminalStatus: string(StatusBlocked),
		},
		{
			name: "exhausted",
			control: ActionControl{
				Disposition:  ActionDispositionExhausted,
				GoalStatus:   "budget-limited",
				Cause:        "goal_turns_exhausted",
				CheckpointID: "checkpoint-exhausted",
			},
			wantOutputStatus:   generationOutputFailed,
			wantOutputRef:      "goal_turns_exhausted",
			wantTerminalStatus: string(StatusExhausted),
		},
		{
			name: "paused",
			control: ActionControl{
				Disposition:  ActionDispositionPaused,
				GoalStatus:   "paused",
				Cause:        "operator_pause",
				CheckpointID: "checkpoint-paused",
			},
			wantOutputStatus:   generationOutputAwaitingGoal,
			wantTerminalStatus: string(StatusPaused),
		},
		{
			name: "needs approval",
			control: ActionControl{
				Disposition:  ActionDispositionNeedsApproval,
				GoalStatus:   "usage-limited",
				Cause:        ReasonCodeGoalRecoveryAmbiguous,
				GateID:       "goal:converge:2:3:goal_recovery_ambiguous",
				CheckpointID: "checkpoint-needs-approval",
			},
			wantOutputStatus:   generationOutputAwaitingGoal,
			wantTerminalStatus: string(StatusNeedsApproval),
			wantGateID:         "goal:converge:2:3:goal_recovery_ambiguous",
		},
	}

	for _, tc := range tests {
		t.Run("Should map "+tc.name, func(t *testing.T) {
			t.Parallel()

			output := GenerationOutput{
				Generation: 2,
				NodeID:     "converge",
				ItemIndex:  3,
				Status:     generationOutputControlPending,
				TaskRunID:  "task-run-control",
			}
			refreshed, terminal, err := resolveGoalActionControl(
				Run{Status: StatusRunning},
				output,
				task.Run{Result: goalControlRunResult(t, tc.control)},
			)
			if err != nil {
				t.Fatalf("resolveGoalActionControl() error = %v", err)
			}
			if got := refreshed.Status; got != tc.wantOutputStatus {
				t.Fatalf("output status = %q, want %q", got, tc.wantOutputStatus)
			}
			if got := refreshed.OutputRef; got != tc.wantOutputRef {
				t.Fatalf("output ref = %q, want %q", got, tc.wantOutputRef)
			}
			if tc.wantTerminalStatus == "" {
				if terminal != nil {
					t.Fatalf("terminal = %#v, want nil", terminal)
				}
				return
			}
			if terminal == nil {
				t.Fatal("terminal = nil, want mapped terminal")
				return
			}
			if got := terminal.Status; got != tc.wantTerminalStatus {
				t.Fatalf("terminal status = %q, want %q", got, tc.wantTerminalStatus)
			}
			if got := terminal.GateID; got != tc.wantGateID {
				t.Fatalf("terminal gate_id = %q, want %q", got, tc.wantGateID)
			}
		})
	}
}

func TestDecodeGoalActionControlShouldRejectInvalidPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "missing envelope", raw: nil},
		{name: "unknown owner", raw: json.RawMessage(`{"kind":"other","payload":{}}`)},
		{name: "missing disposition", raw: goalControlRunResult(t, ActionControl{})},
		{
			name: "success with a failure cause",
			raw: goalControlRunResult(t, ActionControl{
				Disposition:  ActionDispositionSucceeded,
				GoalStatus:   "complete",
				Cause:        ReasonCodeGoalReportedBlocked,
				CheckpointID: "checkpoint-invalid-success",
			}),
		},
		{
			name: "unknown control field",
			raw: json.RawMessage(
				`{"kind":"loop_action","payload":{"disposition":"paused","goal_status":"paused","cause":"operator_pause","checkpoint_id":"cp","unexpected":true}}`,
			),
		},
	}

	for _, tc := range tests {
		t.Run("Should reject "+tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := DecodeActionControlResult(tc.raw)
			if err == nil {
				t.Fatal("DecodeActionControlResult() error = nil, want invalid control")
			}
			var reasonErr *ReasonError
			if !errors.As(err, &reasonErr) {
				t.Fatalf("error = %v, want ReasonError", err)
			}
			if got, want := reasonErr.Code, ReasonCodeGoalActionControlInvalid; got != want {
				t.Fatalf("reason code = %q, want %q", got, want)
			}
		})
	}
}

func TestSyntheticGoalGateIDShouldIncludeNodeInstanceAndCause(t *testing.T) {
	t.Run("Should produce the exact approval correlation key", func(t *testing.T) {
		t.Parallel()

		got := syntheticGoalGateID(GenerationOutput{
			Generation: 9,
			NodeID:     "verify",
			ItemIndex:  4,
		}, ReasonCodeGoalBudgetFenced)
		want := dsl.NodeID("goal:verify:9:4:goal_budget_fenced")
		if got != want {
			t.Fatalf("syntheticGoalGateID() = %q, want %q", got, want)
		}
	})
}

func goalControlRunResult(t *testing.T, control ActionControl) json.RawMessage {
	t.Helper()

	payload, err := json.Marshal(control)
	if err != nil {
		t.Fatalf("json.Marshal(control) error = %v", err)
	}
	envelope, err := json.Marshal(task.CoordinatorControlResult{
		Kind:    coordinatorControlKindLoopAction,
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("json.Marshal(envelope) error = %v", err)
	}
	return envelope
}

func goalCoordinatorDefinition() dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "goal-coordinator", Version: 1},
		Inputs: map[string]dsl.Input{
			"items": {Type: dsl.InputTypeRef},
		},
		Contract: dsl.Contract{
			Goal:             "Converge",
			DefinitionOfDone: "Judge approves",
			TerminalStates:   []dsl.TerminalState{dsl.TerminalDone, dsl.TerminalFailed},
			IterationCap:     1,
			NoProgress:       dsl.NoProgress{Window: 1},
			Budget: dsl.Budget{
				Tokens:       100,
				WallClockSec: 10,
				OnExceeded:   dsl.BudgetExceededHalt,
			},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{{
				ID:    "converge",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionGoal),
				Params: dsl.NodeParams{
					"agent":     "codex",
					"objective": "Make verification pass",
					"judge": []any{
						map[string]any{"id": "verify", "type": "command", "check": "make verify"},
					},
					"max_turns": 10,
					"output_schema": map[string]any{
						"status": map[string]any{
							"type": "string",
							"enum": []any{"complete", "blocked"},
						},
					},
				},
			}},
			Edges: []dsl.Edge{},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartManual}},
		},
	}
}
