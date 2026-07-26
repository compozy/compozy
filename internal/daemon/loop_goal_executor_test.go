package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	eventspkg "github.com/compozy/agh/internal/events"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/transcript"
)

func TestComposeLoopGoalExecutorShouldRegisterThroughParentActionBoundary(t *testing.T) {
	t.Run("Should resolve the child executor without a parent package import", func(t *testing.T) {
		t.Parallel()

		store, err := globaldb.OpenGlobalDB(context.Background(), filepath.Join(t.TempDir(), "agh.db"))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := store.Close(context.Background()); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		option, err := composeLoopGoalExecutor(
			store,
			inertLoopGoalRuntime{},
			inertGateEvaluator{},
			newLoopJudgeExecutionRegistry(),
		)
		if err != nil {
			t.Fatalf("composeLoopGoalExecutor() error = %v", err)
		}
		registry, err := looppkg.NewActionRegistry(inertActionToolRegistry{}, option)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		executor, err := registry.Resolve(context.Background(), tools.Scope{}, string(dsl.ActionGoal))
		if err != nil {
			t.Fatalf("Resolve(goal) error = %v", err)
		}
		if _, ok := executor.(*goalpkg.Executor); !ok {
			t.Fatalf("resolved Goal executor = %T", executor)
		}
	})

	t.Run("Should compose through the real production session Manager", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "agh.db"))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := store.Close(ctx); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		manager, err := session.NewManager(session.WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("session.NewManager() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(ctx); err != nil {
				t.Errorf("Manager.Shutdown() error = %v", err)
			}
		})

		runtime := newLoopGoalProductionRuntime(
			store,
			manager,
			t.TempDir(),
			&loopSessionPolicyGate{},
			time.Now,
			discardLogger(),
		)
		option, err := composeLoopGoalExecutor(
			store,
			runtime,
			inertGateEvaluator{},
			newLoopJudgeExecutionRegistry(),
		)
		if err != nil {
			t.Fatalf("composeLoopGoalExecutor(real Manager) error = %v", err)
		}
		registry, err := looppkg.NewActionRegistry(inertActionToolRegistry{}, option)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		executor, err := registry.Resolve(ctx, tools.Scope{}, string(dsl.ActionGoal))
		if err != nil {
			t.Fatalf("Resolve(goal) error = %v", err)
		}
		if _, ok := executor.(*goalpkg.Executor); !ok {
			t.Fatalf("resolved Goal executor = %T", executor)
		}
	})
}

func TestLoopGoalContextRuntimeShouldUseLatestTypedSessionEvidence(t *testing.T) {
	t.Run("Should return sequenced context usage and replace the advertised command set", func(t *testing.T) {
		t.Parallel()

		used := int64(8)
		size := int64(10)
		reader := staticLoopSessionEventReader{events: []store.SessionEvent{
			managedGoalContextEvent(t, 1, acp.AgentEvent{
				Type: acp.EventTypeAvailableCommands, Title: acp.SystemEventTitleAvailableCommandsUpdate,
				AvailableCommands: acp.NewAvailableCommandSet([]store.SessionAdvertisedCommand{{Name: "compact"}}),
			}),
			managedGoalContextEvent(t, 2, acp.AgentEvent{
				Type:  acp.EventTypeUsage,
				Usage: &acp.TokenUsage{ContextUsed: &used, ContextSize: &size},
			}),
			managedGoalContextEvent(t, 3, acp.AgentEvent{
				Type: acp.EventTypeAvailableCommands, Title: acp.SystemEventTitleAvailableCommandsUpdate,
				AvailableCommands: acp.NewAvailableCommandSet([]store.SessionAdvertisedCommand{{Name: "review"}}),
			}),
		}}
		runtime := &loopGoalContextRuntime{sessions: reader}
		binding := looppkg.ActionSessionBinding{SessionID: "session-context"}

		usage, err := runtime.Usage(testutil.Context(t), binding)
		if err != nil {
			t.Fatalf("Usage() error = %v", err)
		}
		if !usage.Known || usage.Used != used || usage.Size != size || usage.Sequence != 2 {
			t.Fatalf("Usage() = %#v", usage)
		}
		hasCompact, err := runtime.HasAdvertisedCommand(testutil.Context(t), binding, "compact")
		if err != nil {
			t.Fatalf("HasAdvertisedCommand(compact) error = %v", err)
		}
		if hasCompact {
			t.Fatal("HasAdvertisedCommand(compact) = true, want latest-set replacement")
		}
		hasReview, err := runtime.HasAdvertisedCommand(testutil.Context(t), binding, "review")
		if err != nil {
			t.Fatalf("HasAdvertisedCommand(review) error = %v", err)
		}
		if !hasReview {
			t.Fatal("HasAdvertisedCommand(review) = false")
		}
	})
}

func TestReconstructManagedGoalPromptResultShouldRequireExactOrderedTerminalWindow(t *testing.T) {
	t.Run(
		"Should recover ordered text structured output and reported zero usage across unrelated sequence gaps",
		func(t *testing.T) {
			t.Parallel()

			promptID := "goal-prompt-recovery"
			zero := int64(0)
			events := []store.SessionEvent{
				managedGoalRecoveryEvent(t, 11, promptID, acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, Text: `{"status":"complete"}`,
				}),
				managedGoalRecoveryEvent(t, 13, promptID, acp.AgentEvent{
					Type:             acp.EventTypeDone,
					StopReason:       string(looppkg.ActionStopEndTurn),
					PromptStopReason: acp.PromptStopReasonEndTurn,
					Usage:            &acp.TokenUsage{TotalTokens: &zero},
				}),
				managedGoalRecoveryEvent(t, 14, promptID, acp.AgentEvent{
					Type: eventspkg.TranscriptMarkerCreated, Text: "terminal metadata",
				}),
			}
			result, terminalAt, found, err := reconstructManagedGoalPromptResult(
				testutil.Context(t), staticLoopSessionEventReader{events: events}, "session-recovery", promptID,
			)
			if err != nil {
				t.Fatalf("reconstructManagedGoalPromptResult() error = %v", err)
			}
			if !found || result.PromptID != promptID || result.Outcome != looppkg.ActionPromptOutcomeCompleted ||
				result.StopReason != looppkg.ActionStopEndTurn || result.Text != `{"status":"complete"}` ||
				string(result.Structured) != `{"status":"complete"}` || result.EventStartSeq != 11 ||
				result.EventEndSeq != 13 || result.TokensUsed != 0 || !result.TokensReported ||
				!terminalAt.Equal(events[1].Timestamp) {
				t.Fatalf("recovered terminal = %#v at %s found=%t", result, terminalAt, found)
			}
		},
	)

	t.Run("Should reject negative or overflowing cumulative usage", func(t *testing.T) {
		t.Parallel()

		negative := int64(-1)
		maximum := int64(math.MaxInt64)
		one := int64(1)
		cases := []struct {
			name  string
			usage *acp.TokenUsage
		}{
			{name: "negative total", usage: &acp.TokenUsage{TotalTokens: &negative}},
			{name: "overflowing components", usage: &acp.TokenUsage{InputTokens: &maximum, OutputTokens: &one}},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()

				promptID := "goal-prompt-invalid-usage"
				events := []store.SessionEvent{
					managedGoalRecoveryEvent(t, 31, promptID, acp.AgentEvent{
						Type: acp.EventTypeAgentMessage, Text: "partial", Usage: tc.usage,
					}),
					managedGoalRecoveryEvent(t, 32, promptID, acp.AgentEvent{
						Type: acp.EventTypeDone, PromptStopReason: acp.PromptStopReasonEndTurn,
					}),
				}
				if _, _, _, err := reconstructManagedGoalPromptResult(
					testutil.Context(t), staticLoopSessionEventReader{events: events}, "session-recovery", promptID,
				); err == nil {
					t.Fatal("reconstructManagedGoalPromptResult(invalid usage) error = nil")
				}
			})
		}
	})

	t.Run("Should reject unordered foreign or multiply terminal windows", func(t *testing.T) {
		t.Parallel()

		promptID := "goal-prompt-invalid-window"
		validMessage := managedGoalRecoveryEvent(t, 21, promptID, acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, Text: "partial",
		})
		validDone := managedGoalRecoveryEvent(t, 22, promptID, acp.AgentEvent{
			Type:             acp.EventTypeDone,
			StopReason:       string(looppkg.ActionStopEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
		})
		foreign := managedGoalRecoveryEvent(t, 22, "goal-prompt-other", acp.AgentEvent{
			Type:             acp.EventTypeDone,
			StopReason:       string(looppkg.ActionStopEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
		})
		duplicate := validDone
		duplicate.Sequence = validMessage.Sequence
		descending := validDone
		descending.Sequence = validMessage.Sequence - 1
		secondTerminal := managedGoalRecoveryEvent(t, 23, promptID, acp.AgentEvent{
			Type: acp.EventTypeError, Error: "late duplicate terminal",
		})
		cases := []struct {
			name   string
			events []store.SessionEvent
		}{
			{name: "duplicate sequence", events: []store.SessionEvent{validMessage, duplicate}},
			{name: "descending sequence", events: []store.SessionEvent{validMessage, descending}},
			{name: "foreign prompt", events: []store.SessionEvent{validMessage, foreign}},
			{name: "multiple terminals", events: []store.SessionEvent{validMessage, validDone, secondTerminal}},
		}
		for _, tc := range cases {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()

				result, _, found, err := reconstructManagedGoalPromptResult(
					testutil.Context(t), staticLoopSessionEventReader{events: tc.events},
					"session-recovery", promptID,
				)
				if err != nil {
					t.Fatalf("reconstructManagedGoalPromptResult() error = %v", err)
				}
				if found || result.PromptID != "" {
					t.Fatalf("invalid recovery result = %#v found=%t", result, found)
				}
			})
		}
	})
}

func TestReadManagedGoalPromptOutputShouldIgnoreOnlyCorrelatedAuxiliaryEvents(t *testing.T) {
	t.Run(
		"Should read an ordered prompt across foreign sequence gaps and trailing transcript metadata",
		func(t *testing.T) {
			t.Parallel()

			promptID := "goal-prompt-output"
			events := []store.SessionEvent{
				managedGoalRecoveryEvent(t, 31, promptID, acp.AgentEvent{
					Type: acp.EventTypeSyntheticReentry, Text: "managed objective",
				}),
				managedGoalRecoveryEvent(t, 33, promptID, acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, Text: `{"status":"complete"}`,
				}),
				managedGoalRecoveryEvent(t, 34, promptID, acp.AgentEvent{
					Type:             acp.EventTypeDone,
					PromptStopReason: acp.PromptStopReasonEndTurn,
				}),
				managedGoalRecoveryEvent(t, 35, promptID, acp.AgentEvent{
					Type: eventspkg.TranscriptMarkerCreated, Text: "terminal metadata",
				}),
			}
			text, structured, err := readManagedGoalPromptOutput(
				testutil.Context(t),
				staticLoopSessionEventReader{events: events},
				store.SessionInputQueueEntry{SessionID: "session-recovery", PromptID: promptID},
				looppkg.ActionPromptResult{PromptID: promptID, EventStartSeq: 31, EventEndSeq: 34},
			)
			if err != nil {
				t.Fatalf("readManagedGoalPromptOutput() error = %v", err)
			}
			if text != `{"status":"complete"}` || string(structured) != text {
				t.Fatalf("managed Goal output = text:%q structured:%s", text, structured)
			}
		},
	)

	t.Run("Should reject a result-bearing event after the terminal frame", func(t *testing.T) {
		t.Parallel()

		promptID := "goal-prompt-output-late"
		events := []store.SessionEvent{
			managedGoalRecoveryEvent(t, 41, promptID, acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "reply"}),
			managedGoalRecoveryEvent(t, 42, promptID, acp.AgentEvent{
				Type: acp.EventTypeDone, PromptStopReason: acp.PromptStopReasonEndTurn,
			}),
			managedGoalRecoveryEvent(t, 43, promptID, acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "late"}),
		}
		_, _, err := readManagedGoalPromptOutput(
			testutil.Context(t),
			staticLoopSessionEventReader{events: events},
			store.SessionInputQueueEntry{SessionID: "session-recovery", PromptID: promptID},
			looppkg.ActionPromptResult{PromptID: promptID, EventStartSeq: 41, EventEndSeq: 42},
		)
		if !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("readManagedGoalPromptOutput() error = %v, want transition conflict", err)
		}
	})
}

func TestLoopGoalManagedLifecycleShouldClassifyClaimCommitEvidence(t *testing.T) {
	t.Run("Should recover a committed claim only from its exact durable token digest", func(t *testing.T) {
		t.Parallel()

		claimErr := errors.New("commit acknowledgement lost")
		token := "dispatch-token-committed"
		storeStub, owner := newManagedLifecycleClaimStore()
		storeStub.claimed = goalpkg.ClaimedPrompt{
			Checkpoint:    goalpkg.Checkpoint{TurnsUsed: 1},
			DispatchToken: token,
		}
		storeStub.claimErr = claimErr
		storeStub.entry.Status = store.SessionInputQueueStatusDispatching
		storeStub.entry.DispatchTokenHash = managedGoalDispatchTokenHash(token)
		lifecycle := &loopGoalManagedInputLifecycle{store: storeStub, now: time.Now}

		submission, err := lifecycle.BeginSubmission(testutil.Context(t), session.ManagedInputClaim{Owner: owner})
		if err != nil {
			t.Fatalf("BeginSubmission() error = %v", err)
		}
		if submission.Owner != owner || submission.DispatchToken != token {
			t.Fatalf("recovered submission = %#v", submission)
		}
	})

	t.Run("Should prove known false only from an unchanged prepared row", func(t *testing.T) {
		t.Parallel()

		claimErr := errors.New("claim rolled back")
		storeStub, owner := newManagedLifecycleClaimStore()
		storeStub.claimErr = claimErr
		lifecycle := &loopGoalManagedInputLifecycle{store: storeStub, now: time.Now}

		_, err := lifecycle.BeginSubmission(testutil.Context(t), session.ManagedInputClaim{Owner: owner})
		var effectErr *session.ManagedInputEffectError
		if !errors.As(err, &effectErr) || effectErr.Certainty != session.EffectKnownFalse ||
			effectErr.Finalized || !errors.Is(err, claimErr) {
			t.Fatalf("BeginSubmission() error = %#v, want non-final known-false claim", err)
		}
	})

	t.Run("Should classify unavailable durable proof as effect unknown", func(t *testing.T) {
		t.Parallel()

		claimErr := errors.New("claim outcome unknown")
		lookupErr := errors.New("durable lookup unavailable")
		storeStub, owner := newManagedLifecycleClaimStore()
		storeStub.claimErr = claimErr
		storeStub.entryErr = lookupErr
		storeStub.entryErrAfter = 1
		lifecycle := &loopGoalManagedInputLifecycle{store: storeStub, now: time.Now}

		_, err := lifecycle.BeginSubmission(testutil.Context(t), session.ManagedInputClaim{Owner: owner})
		var effectErr *session.ManagedInputEffectError
		if !errors.As(err, &effectErr) || effectErr.Certainty != session.EffectUnknown ||
			effectErr.Finalized || !errors.Is(err, claimErr) || !errors.Is(err, lookupErr) {
			t.Fatalf("BeginSubmission() error = %#v, want non-final effect-unknown claim", err)
		}
	})

	t.Run("Should durably finalize a conflicting intermediate claim as ambiguous", func(t *testing.T) {
		t.Parallel()

		claimErr := errors.New("claim acknowledgement conflicted")
		storeStub, owner := newManagedLifecycleClaimStore()
		storeStub.claimErr = claimErr
		storeStub.entry.Status = store.SessionInputQueueStatusDispatching
		storeStub.entry.DispatchTokenHash = managedGoalDispatchTokenHash("different-token")
		lifecycle := &loopGoalManagedInputLifecycle{store: storeStub, now: time.Now}

		_, err := lifecycle.BeginSubmission(testutil.Context(t), session.ManagedInputClaim{Owner: owner})
		var effectErr *session.ManagedInputEffectError
		if !errors.As(err, &effectErr) || effectErr.Certainty != session.EffectUnknown ||
			!effectErr.Finalized || storeStub.ambiguousCalls != 1 || !errors.Is(err, claimErr) {
			t.Fatalf(
				"BeginSubmission() error = %#v ambiguous calls = %d, want finalized effect-unknown",
				err,
				storeStub.ambiguousCalls,
			)
		}
	})
}

func TestLoopGoalJudgeEvaluatorShouldReturnAggregateUsage(t *testing.T) {
	t.Run("Should return the exact gate judge token total including reported zero semantics", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 10, 21, 0, 0, 0, time.UTC)
		run := looppkg.Run{
			ID: "looprun-goal-judge-usage", WorkspaceID: "workspace-goal-judge-usage",
			LoopName: "goal-judge-usage", Status: looppkg.StatusRunning,
			CreatedAt: now, StartedAt: now, Inputs: map[string]any{},
		}
		wantParticipation := daemonTestLiveParticipation(string(run.WorkspaceID), "goal-judge")
		run.SetNetworkSpec(wantParticipation)
		applyLoopRunPinningForTest(t, &run, now)
		storeStub := goalJudgeLoopStore{
			run: run,
			snapshot: looppkg.DefinitionSnapshot{
				WorkspaceID: run.WorkspaceID, Digest: run.DefinitionDigest,
				Version: run.DefinitionVersion, Definition: run.DefinitionSnapshot,
			},
		}
		var gotJudgeReq gate.JudgeRequest
		evaluator := gate.NewEvaluator(gate.WithJudgeRunner(goalJudgeRunnerFunc(
			func(_ context.Context, req gate.JudgeRequest) (gate.JudgeResponse, error) {
				gotJudgeReq = req
				const evidenceHeading = "\nJudge rubric:\nCheck\n\n" +
					"Authoritative completed candidate evidence"
				if strings.Contains(req.Rubric, "Review attempt:") ||
					!strings.Contains(req.Rubric, evidenceHeading) ||
					!strings.Contains(req.Rubric, "Candidate text:\nThe implementation satisfies the criterion.") ||
					!strings.Contains(req.Rubric, "Candidate structured output:\n{\"status\":\"complete\"}") {
					t.Fatalf("Goal judge rubric = %q, want unchanged rubric plus authoritative candidate", req.Rubric)
				}
				if req.CorrelationID != "judge-attempt-usage" {
					t.Fatalf("Goal judge correlation id = %q, want judge-attempt-usage", req.CorrelationID)
				}
				return gate.JudgeResponse{
					Raw:        `{"verdict":"approved","evidence":{"checked":true}}`,
					TokensUsed: 9, TokensReported: true,
				}, nil
			},
		)))
		adapter := &loopGoalJudgeEvaluator{
			store: &storeStub, evaluator: evaluator, executions: newLoopJudgeExecutionRegistry(),
		}
		result, err := adapter.EvaluateGoal(testutil.Context(t), goalpkg.JudgeRequest{
			AttemptID: "judge-attempt-usage",
			Key: goalpkg.TurnKey{
				WorkspaceID: run.WorkspaceID, LoopRunID: run.ID,
				Generation: 1, NodeID: "converge", ItemIndex: 0,
			},
			Turn: 1,
			Criteria: []dsl.GateCriterion{{
				ID: "judge", Type: dsl.CriterionAgentJudge, Agent: "reviewer", Rubric: "Check",
			}},
			Result: looppkg.ActionPromptResult{
				Text:       "The implementation satisfies the criterion.",
				Structured: json.RawMessage(`{"status":"complete"}`),
			},
		})
		if err != nil {
			t.Fatalf("EvaluateGoal() error = %v", err)
		}
		if got, want := gotJudgeReq.LoopRunID, string(run.ID); got != want {
			t.Fatalf("JudgeRequest.LoopRunID = %q, want %q", got, want)
		}
		if gotJudgeReq.NetworkParticipation == nil || *gotJudgeReq.NetworkParticipation != wantParticipation {
			t.Fatalf(
				"JudgeRequest.NetworkParticipation = %#v, want %#v",
				gotJudgeReq.NetworkParticipation,
				wantParticipation,
			)
		}
		if result.Verdict.Outcome != gate.VerdictOutcomeApproved ||
			result.TokensUsed != 9 || !result.TokensReported {
			t.Fatalf("Goal judge result = %#v", result)
		}
	})
}

type goalJudgeLoopStore struct {
	looppkg.Store
	run      looppkg.Run
	snapshot looppkg.DefinitionSnapshot
}

func (s *goalJudgeLoopStore) GetLoopRun(
	context.Context,
	looppkg.WorkspaceID,
	looppkg.RunID,
) (looppkg.Run, error) {
	return s.run, nil
}

func (s *goalJudgeLoopStore) GetLoopDefinitionSnapshot(
	context.Context,
	looppkg.WorkspaceID,
	string,
) (looppkg.DefinitionSnapshot, error) {
	return s.snapshot, nil
}

type goalJudgeRunnerFunc func(context.Context, gate.JudgeRequest) (gate.JudgeResponse, error)

func (f goalJudgeRunnerFunc) Judge(
	ctx context.Context,
	req gate.JudgeRequest,
) (gate.JudgeResponse, error) {
	return f(ctx, req)
}

type managedLifecycleClaimStore struct {
	loopGoalManagedRuntimeStore
	entry          store.SessionInputQueueEntry
	entryErr       error
	entryErrAfter  int
	entryCalls     int
	taskRun        taskpkg.Run
	run            looppkg.Run
	checkpoint     goalpkg.Checkpoint
	claimed        goalpkg.ClaimedPrompt
	claimErr       error
	ambiguousCalls int
}

func newManagedLifecycleClaimStore() (*managedLifecycleClaimStore, session.ManagedInputOwner) {
	runGeneration := int64(1)
	controlEpoch := int64(2)
	bindingEpoch := int64(3)
	owner := session.ManagedInputOwner{
		QueueEntryID: "goalq-claim", SessionID: "session-claim", OwnerKind: "goal",
		LoopRunID: "looprun-claim", TaskRunID: "taskrun-claim", RunGeneration: int(runGeneration),
		PromptAttempt: 0, ControlEpoch: controlEpoch, BindingEpoch: bindingEpoch,
		PromptID: "goal-prompt-claim", PromptKind: "work",
	}
	key := goalpkg.TurnKey{
		WorkspaceID: "workspace-claim", LoopRunID: looppkg.RunID(owner.LoopRunID),
		Generation: owner.RunGeneration, NodeID: "converge", ItemIndex: 0,
	}
	return &managedLifecycleClaimStore{
		entry: store.SessionInputQueueEntry{
			ID: owner.QueueEntryID, SessionID: owner.SessionID, Status: store.SessionInputQueueStatusQueued,
			OwnerKind: owner.OwnerKind, LoopRunID: owner.LoopRunID, TaskRunID: owner.TaskRunID,
			RunGeneration: &runGeneration, OwnerEpoch: &controlEpoch, BindingEpoch: &bindingEpoch,
			PromptID: owner.PromptID, PromptKind: owner.PromptKind, PromptAttempt: owner.PromptAttempt,
		},
		taskRun: taskpkg.Run{
			ID: owner.TaskRunID, LoopRunID: owner.LoopRunID,
			Metadata: json.RawMessage(
				`{"generation":1,"node_id":"converge","item_index":0,"goal_segment_epoch":2}`,
			),
		},
		run: looppkg.Run{ID: key.LoopRunID, WorkspaceID: key.WorkspaceID},
		checkpoint: goalpkg.Checkpoint{
			Key: key, ControlEpoch: controlEpoch, BindingEpoch: bindingEpoch,
			TaskRunID: owner.TaskRunID, QueueEntryID: owner.QueueEntryID,
			PromptID: owner.PromptID, PromptKind: owner.PromptKind, PromptAttempt: owner.PromptAttempt,
			SessionID: owner.SessionID, BindingHandle: "goal:claim", Phase: "queued", Status: "active",
			TurnLimit: 3, ContextState: loopManagedContextStateUnknown, ContextNudgeRatio: 0.8,
		},
	}, owner
}

func (s *managedLifecycleClaimStore) GetSessionInputQueueEntryByID(
	context.Context,
	string,
) (store.SessionInputQueueEntry, error) {
	s.entryCalls++
	if s.entryErr != nil && (s.entryErrAfter == 0 || s.entryCalls > s.entryErrAfter) {
		return store.SessionInputQueueEntry{}, s.entryErr
	}
	return s.entry, nil
}

func (s *managedLifecycleClaimStore) GetTaskRun(context.Context, string) (taskpkg.Run, error) {
	return s.taskRun, nil
}

func (s *managedLifecycleClaimStore) GetLoopRunByID(context.Context, looppkg.RunID) (looppkg.Run, error) {
	return s.run, nil
}

func (s *managedLifecycleClaimStore) LoadCheckpoint(
	context.Context,
	goalpkg.TurnKey,
) (goalpkg.Checkpoint, error) {
	return s.checkpoint, nil
}

func (s *managedLifecycleClaimStore) FlushAndCheck(
	context.Context,
	goalpkg.BudgetBoundarySnapshot,
) (goalpkg.BudgetDecision, error) {
	return goalpkg.BudgetDecision{Allowed: true, BudgetVersion: 7}, nil
}

func (s *managedLifecycleClaimStore) ClaimPreparedWorkPrompt(
	context.Context,
	goalpkg.ClaimPreparedWorkPromptRequest,
) (goalpkg.ClaimedPrompt, error) {
	return s.claimed, s.claimErr
}

func (s *managedLifecycleClaimStore) MarkAmbiguous(
	context.Context,
	goalpkg.AmbiguousRequest,
) error {
	s.ambiguousCalls++
	return nil
}

type staticLoopSessionEventReader struct {
	events []store.SessionEvent
}

func (r staticLoopSessionEventReader) Events(
	context.Context,
	string,
	store.EventQuery,
) ([]store.SessionEvent, error) {
	return append([]store.SessionEvent(nil), r.events...), nil
}

func managedGoalContextEvent(t *testing.T, sequence int64, event acp.AgentEvent) store.SessionEvent {
	t.Helper()
	event.SessionID = "session-context"
	event.TurnID = "turn-context"
	event.Timestamp = time.Date(2026, 7, 10, 20, 0, int(sequence), 0, time.UTC)
	payload, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent() error = %v", err)
	}
	return store.SessionEvent{
		Sequence: sequence, SessionID: event.SessionID, TurnID: event.TurnID,
		Type: event.Type, Content: payload, Timestamp: event.Timestamp,
	}
}

func managedGoalRecoveryEvent(
	t *testing.T,
	sequence int64,
	promptID string,
	event acp.AgentEvent,
) store.SessionEvent {
	t.Helper()
	event.SessionID = "session-recovery"
	event.TurnID = promptID
	event.Timestamp = time.Date(2026, 7, 10, 21, 30, int(sequence%60), 0, time.UTC)
	payload, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent() error = %v", err)
	}
	return store.SessionEvent{
		Sequence: sequence, SessionID: event.SessionID, TurnID: promptID,
		Type: event.Type, Content: payload, Timestamp: event.Timestamp,
	}
}

type inertLoopGoalRuntime struct{}

func (inertLoopGoalRuntime) BindActionSession(
	context.Context,
	looppkg.ActionSessionBindRequest,
) (looppkg.ActionSessionBinding, error) {
	return looppkg.ActionSessionBinding{}, nil
}

func (inertLoopGoalRuntime) AdvanceActionSessionRetry(
	context.Context,
	*looppkg.ActionSessionRetryRequest,
) error {
	return nil
}

func (inertLoopGoalRuntime) PrepareActionPrompt(
	context.Context,
	looppkg.ActionSessionBinding,
	looppkg.ActionPromptRequest,
) (looppkg.ActionPromptTicket, error) {
	return looppkg.ActionPromptTicket{}, nil
}

func (inertLoopGoalRuntime) AwaitActionPrompt(
	context.Context,
	looppkg.ActionPromptTicket,
) (looppkg.ActionPromptResult, error) {
	return looppkg.ActionPromptResult{}, nil
}

func (inertLoopGoalRuntime) CancelActionPrompts(context.Context, looppkg.ActionPromptOwner) error {
	return nil
}

func (inertLoopGoalRuntime) Usage(
	context.Context,
	looppkg.ActionSessionBinding,
) (goalpkg.ContextUsage, error) {
	return goalpkg.ContextUsage{}, nil
}

func (inertLoopGoalRuntime) HasAdvertisedCommand(
	context.Context,
	looppkg.ActionSessionBinding,
	string,
) (bool, error) {
	return false, nil
}

func (inertLoopGoalRuntime) ReconcileTerminalFromEvents(
	context.Context,
	goalpkg.PromptRecoveryIdentity,
) (looppkg.ActionPromptResult, bool, error) {
	return looppkg.ActionPromptResult{}, false, nil
}

type inertGateEvaluator struct{}

func (inertGateEvaluator) Evaluate(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
	return gate.Verdict{}, nil
}

type inertActionToolRegistry struct{}

func (inertActionToolRegistry) List(context.Context, tools.Scope) ([]tools.ToolView, error) {
	return nil, nil
}

func (inertActionToolRegistry) Search(
	context.Context,
	tools.Scope,
	tools.SearchQuery,
) ([]tools.ToolView, error) {
	return nil, nil
}

func (inertActionToolRegistry) Get(context.Context, tools.Scope, tools.ToolID) (tools.ToolView, error) {
	return tools.ToolView{}, nil
}

func (inertActionToolRegistry) Call(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
	return tools.ToolResult{}, nil
}
