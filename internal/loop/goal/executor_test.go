package goal

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
)

func TestControlRevokedErrorShouldPublishSafeRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should retain transition identity and typed operator detail", func(t *testing.T) {
		t.Parallel()

		err := newControlRevokedError()
		if !errors.Is(err, loop.ErrTransitionConflict) {
			t.Fatalf("newControlRevokedError() = %v, want ErrTransitionConflict", err)
		}
		provider, ok := errors.AsType[loop.SafeActionFailureProvider](err)
		if !ok {
			t.Fatalf("newControlRevokedError() = %T, want SafeActionFailureProvider", err)
		}
		failure := provider.SafeActionFailure()
		if failure.Code != string(loop.ReasonCodeGoalControlRevokedInFlight) ||
			failure.Cause != controlRevokedCause || failure.Recovery != controlRevokedRecovery {
			t.Fatalf("SafeActionFailure() = %#v, want typed Goal revocation detail", failure)
		}
	})
}

func TestExecutorShouldProjectDurableRevocationAfterJudgeCancellation(t *testing.T) {
	t.Run("Should prefer committed Goal control over the canceled judge context", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		contextStore := &contextAwareJudgeStore{fakeExecutorStore: store}
		binder := newFakeManagedBinder(store, scriptedEndTurn("candidate", 10))
		judge := &cancelBlockingJudge{started: make(chan struct{})}
		budget := &fakeBudgetGuard{decisions: map[BudgetBoundary][]BudgetDecision{}}
		executor, err := NewExecutor(Dependencies{
			Store:    contextStore,
			Binder:   binder,
			Judge:    judge,
			Budget:   budget,
			Context:  &fakeContextHealth{},
			Recovery: &fakePromptRecovery{},
			Now:      func() time.Time { return time.Date(2026, 7, 13, 21, 0, 0, 0, time.UTC) },
		})
		if err != nil {
			t.Fatalf("NewExecutor() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		input := testGoalInput(t)
		errCh := make(chan error, 1)
		go func() {
			_, executeErr := executor.Execute(ctx, testGoalNode(2), input)
			errCh <- executeErr
		}()

		select {
		case <-judge.started:
		case <-time.After(time.Second):
			t.Fatal("judge did not start")
		}
		checkpoint, _, _ := store.snapshot()
		if _, err := store.RevokeGoalPrompt(context.Background(), RevokePromptRequest{
			Key:                  checkpoint.Key,
			ExpectedControlEpoch: checkpoint.ControlEpoch,
			ExpectedBindingEpoch: checkpoint.BindingEpoch,
			TaskRunID:            checkpoint.TaskRunID,
			QueueEntryID:         checkpoint.QueueEntryID,
			PromptID:             checkpoint.PromptID,
			Disposition:          loop.ActionDispositionPaused,
			Status:               goalStatusPaused,
			Cause:                loop.ReasonCodeGoalControlRevokedInFlight,
			ActorKind:            "human",
			ActorID:              "local-user",
			ProjectionCause:      SessionOutboxCauseClear,
			RevokedAt:            time.Now().UTC(),
		}); err != nil {
			t.Fatalf("RevokeGoalPrompt() error = %v", err)
		}
		cancel()

		select {
		case executeErr := <-errCh:
			provider, ok := errors.AsType[loop.SafeActionFailureProvider](executeErr)
			if !ok {
				t.Fatalf("Execute() error = %T %v, want typed Goal control failure", executeErr, executeErr)
			}
			failure := provider.SafeActionFailure()
			if failure.Code != string(loop.ReasonCodeGoalControlRevokedInFlight) ||
				failure.Cause != controlRevokedCause || failure.Recovery != controlRevokedRecovery {
				t.Fatalf("SafeActionFailure() = %#v, want durable Goal revocation", failure)
			}
		case <-time.After(time.Second):
			t.Fatal("Execute() did not return after judge cancellation")
		}
	})
}

type contextAwareJudgeStore struct {
	*fakeExecutorStore
}

func (s *contextAwareJudgeStore) CompleteJudgeAttempt(
	ctx context.Context,
	req CompleteJudgeAttemptRequest,
) (JudgeAttempt, error) {
	if err := ctx.Err(); err != nil {
		return JudgeAttempt{}, err
	}
	return s.fakeExecutorStore.CompleteJudgeAttempt(ctx, req)
}

type cancelBlockingJudge struct {
	started chan struct{}
}

func (j *cancelBlockingJudge) EvaluateGoal(ctx context.Context, _ JudgeRequest) (JudgeResult, error) {
	close(j.started)
	<-ctx.Done()
	return JudgeResult{}, ctx.Err()
}

func TestUsageTrackerShouldFailClosedOnOverflow(t *testing.T) {
	t.Run("Should saturate cumulative usage when one operation exceeds int64", func(t *testing.T) {
		t.Parallel()

		var reported atomic.Int64
		tracker := newUsageTracker(math.MaxInt64-2, loop.ActionUsageReporterFunc(func(tokens int64) {
			reported.Store(tokens)
		}))

		tracker.observeOperation(math.MaxInt64-2, 10, true)

		got, ok := tracker.snapshot()
		if !ok || got != math.MaxInt64 {
			t.Fatalf("snapshot = (%d, %t), want (%d, true)", got, ok, int64(math.MaxInt64))
		}
		if gotReported := reported.Load(); gotReported != math.MaxInt64 {
			t.Fatalf("reported usage = %d, want %d", gotReported, int64(math.MaxInt64))
		}
	})
}

func TestExecutorShouldConvergeOnThirdTurnWithDurableAudit(t *testing.T) {
	t.Run("Should serialize three turns and flush cumulative work and judge usage", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store,
			scriptedEndTurn("first", 10, 3, 10),
			scriptedEndTurn("second", 20, 8, 20),
			scriptedEndTurn("third", 30, 30),
		)
		judge := &fakeJudge{results: []JudgeResult{
			judgeResult(gate.VerdictOutcomeRejected, 1),
			judgeResult(gate.VerdictOutcomeRejected, 1),
			judgeResult(gate.VerdictOutcomeApproved, 1),
		}}
		budget := &fakeBudgetGuard{}
		executor := newTestExecutor(t, store, binder, judge, budget)
		var usageMu sync.Mutex
		var reported []int64
		input := testGoalInput(t)
		input.GoalContextNudgeRatio = new(0.37)
		input.PersistedTaskTokensUsed = 5
		input.UsageReporter = loop.ActionUsageReporterFunc(func(tokens int64) {
			usageMu.Lock()
			defer usageMu.Unlock()
			reported = append(reported, tokens)
		})

		raw, err := executor.Execute(context.Background(), testGoalNode(3), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
			t.Fatalf("Control = %#v, want succeeded", raw.Control)
		}
		if raw.Text != "third" || raw.TokensUsed != 68 {
			t.Fatalf("raw result = text:%q tokens:%d, want third/68", raw.Text, raw.TokensUsed)
		}
		checkpoint, turns, _ := store.snapshot()
		if checkpoint.Status != goalStatusComplete || checkpoint.Phase != checkpointPhaseTerminal {
			t.Fatalf("checkpoint = status:%q phase:%q, want complete/terminal", checkpoint.Status, checkpoint.Phase)
		}
		if checkpoint.ContextNudgeRatio != 0.37 {
			t.Fatalf("checkpoint ContextNudgeRatio = %v, want Run-pinned 0.37", checkpoint.ContextNudgeRatio)
		}
		if len(turns) != 3 || judge.callCount() != 3 {
			t.Fatalf("turns/judges = %d/%d, want 3/3", len(turns), judge.callCount())
		}
		requests := binder.preparedRequests()
		if got := promptKinds(requests); !slices.Equal(got, []string{"work", "continuation", "continuation"}) {
			t.Fatalf("prompt kinds = %v", got)
		}
		if requests[0].PromptID == requests[1].PromptID || requests[1].PromptID == requests[2].PromptID {
			t.Fatalf(
				"prompt IDs were reused: %q %q %q",
				requests[0].PromptID,
				requests[1].PromptID,
				requests[2].PromptID,
			)
		}
		snapshots := budget.boundaries()
		if len(snapshots) != 12 {
			t.Fatalf("budget boundaries = %d, want 12", len(snapshots))
		}
		for index := 1; index < len(snapshots); index++ {
			if snapshots[index].LiveTokensUsed < snapshots[index-1].LiveTokensUsed {
				t.Fatalf(
					"budget usage decreased at %d: %d < %d",
					index,
					snapshots[index].LiveTokensUsed,
					snapshots[index-1].LiveTokensUsed,
				)
			}
		}
		usageMu.Lock()
		defer usageMu.Unlock()
		if len(reported) == 0 || reported[len(reported)-1] != 68 {
			t.Fatalf("reported cumulative usage = %v, want terminal 68", reported)
		}
	})

	t.Run("Should reject a Goal action without a Run-pinned context policy", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		executor := newTestExecutor(
			t,
			store,
			newFakeManagedBinder(store),
			&fakeJudge{},
			&fakeBudgetGuard{},
		)
		input := testGoalInput(t)
		input.GoalContextNudgeRatio = nil

		_, err := executor.Execute(context.Background(), testGoalNode(3), input)
		if !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("Execute() error = %v, want ErrValidation", err)
		}
	})
}

func TestExecutorShouldRouteEveryPromptStopReason(t *testing.T) {
	tests := []struct {
		name             string
		scripts          []scriptedPrompt
		judges           []JudgeResult
		wantDisposition  loop.ActionDisposition
		wantCause        loop.ReasonCode
		wantJudgeCalls   int
		wantPromptKinds  []string
		wantBindingCount int
	}{
		{
			name:             "end turn",
			scripts:          []scriptedPrompt{scriptedStop(loop.ActionStopEndTurn)},
			judges:           []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)},
			wantDisposition:  loop.ActionDispositionSucceeded,
			wantJudgeCalls:   1,
			wantPromptKinds:  []string{"work"},
			wantBindingCount: 1,
		},
		{
			name:             "max turn requests",
			scripts:          []scriptedPrompt{scriptedStop(loop.ActionStopMaxTurnRequests)},
			judges:           []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)},
			wantDisposition:  loop.ActionDispositionSucceeded,
			wantJudgeCalls:   1,
			wantPromptKinds:  []string{"work"},
			wantBindingCount: 1,
		},
		{
			name:             "refusal",
			scripts:          []scriptedPrompt{scriptedStop(loop.ActionStopRefusal)},
			wantDisposition:  loop.ActionDispositionNeedsApproval,
			wantCause:        loop.ReasonCodeGoalAgentRefused,
			wantPromptKinds:  []string{"work"},
			wantBindingCount: 1,
		},
		{
			name:             "cancellation",
			scripts:          []scriptedPrompt{scriptedStop(loop.ActionStopCancelled)},
			wantDisposition:  loop.ActionDispositionNeedsApproval,
			wantCause:        loop.ReasonCodeGoalPromptFenced,
			wantPromptKinds:  []string{"work"},
			wantBindingCount: 1,
		},
		{
			name: "missing stop reason terminalized as invalid result",
			scripts: []scriptedPrompt{{result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeInvalidResult,
				ReasonCode: loop.ReasonCodeGoalStopReasonInvalid,
			}}},
			wantDisposition:  loop.ActionDispositionNeedsApproval,
			wantCause:        loop.ReasonCodeGoalStopReasonInvalid,
			wantPromptKinds:  []string{"work"},
			wantBindingCount: 1,
		},
		{
			name: "max tokens creates a new recovery continuation",
			scripts: []scriptedPrompt{
				scriptedStop(loop.ActionStopMaxTokens),
				scriptedStop(loop.ActionStopEndTurn),
			},
			judges:           []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)},
			wantDisposition:  loop.ActionDispositionSucceeded,
			wantJudgeCalls:   1,
			wantPromptKinds:  []string{"work", "continuation"},
			wantBindingCount: 2,
		},
	}
	for _, tc := range tests {
		t.Run("Should route "+tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			binder := newFakeManagedBinder(store, tc.scripts...)
			judge := &fakeJudge{results: append([]JudgeResult(nil), tc.judges...)}
			executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
			raw, err := executor.Execute(context.Background(), testGoalNode(4), testGoalInput(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != tc.wantDisposition ||
				raw.Control.Cause != tc.wantCause {
				t.Fatalf("Control = %#v, want disposition:%q cause:%q", raw.Control, tc.wantDisposition, tc.wantCause)
			}
			if got := judge.callCount(); got != tc.wantJudgeCalls {
				t.Fatalf("judge calls = %d, want %d", got, tc.wantJudgeCalls)
			}
			if got := promptKinds(binder.preparedRequests()); !slices.Equal(got, tc.wantPromptKinds) {
				t.Fatalf("prompt kinds = %v, want %v", got, tc.wantPromptKinds)
			}
			if got := len(binder.binds); got != tc.wantBindingCount {
				t.Fatalf("binding calls = %d, want %d", got, tc.wantBindingCount)
			}
			if tc.name == "max tokens creates a new recovery continuation" {
				requests := binder.preparedRequests()
				if requests[0].PromptID == requests[1].PromptID {
					t.Fatal("max_tokens recovery replayed the original prompt ID")
				}
			}
		})
	}
}

func TestExecutorShouldRouteJudgeOutcomes(t *testing.T) {
	tests := []struct {
		name            string
		outcomes        []gate.VerdictOutcome
		wantDisposition loop.ActionDisposition
		wantCause       loop.ReasonCode
	}{
		{
			name:            "approved",
			outcomes:        []gate.VerdictOutcome{gate.VerdictOutcomeApproved},
			wantDisposition: loop.ActionDispositionSucceeded,
		},
		{
			name:            "blocked",
			outcomes:        []gate.VerdictOutcome{gate.VerdictOutcomeBlocked},
			wantDisposition: loop.ActionDispositionBlocked,
			wantCause:       loop.ReasonCodeGoalReportedBlocked,
		},
		{
			name:            "unexpected awaiting approval",
			outcomes:        []gate.VerdictOutcome{gate.VerdictOutcomeAwaitingApproval},
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalJudgeOutcomeInvalid,
		},
		{
			name: "third broken outcome",
			outcomes: []gate.VerdictOutcome{
				gate.VerdictOutcomeError,
				gate.VerdictOutcomeTimeout,
				gate.VerdictOutcomeInvalidOutput,
			},
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalJudgeBroken,
		},
		{
			name: "rejected resets the broken streak",
			outcomes: []gate.VerdictOutcome{
				gate.VerdictOutcomeError,
				gate.VerdictOutcomeRejected,
				gate.VerdictOutcomeError,
				gate.VerdictOutcomeTimeout,
				gate.VerdictOutcomeInvalidOutput,
			},
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalJudgeBroken,
		},
	}
	for _, tc := range tests {
		t.Run("Should route "+tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			scripts := make([]scriptedPrompt, len(tc.outcomes))
			judges := make([]JudgeResult, len(tc.outcomes))
			for index, outcome := range tc.outcomes {
				scripts[index] = scriptedStop(loop.ActionStopEndTurn)
				judges[index] = judgeResult(outcome, 0)
			}
			binder := newFakeManagedBinder(store, scripts...)
			judge := &fakeJudge{results: judges}
			executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
			raw, err := executor.Execute(context.Background(), testGoalNode(5), testGoalInput(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != tc.wantDisposition ||
				raw.Control.Cause != tc.wantCause {
				t.Fatalf("Control = %#v, want disposition:%q cause:%q", raw.Control, tc.wantDisposition, tc.wantCause)
			}
			if tc.name == "unexpected awaiting approval" {
				_, turns, _ := store.snapshot()
				if len(turns) != 1 || turns[0].Verdict != nil {
					t.Fatalf("unexpected judge turn settlement = %#v, want null durable verdict", turns)
				}
				store.mu.Lock()
				attempts := make([]JudgeAttempt, 0, len(store.judgeAttempts))
				for _, attempt := range store.judgeAttempts {
					attempts = append(attempts, cloneJudgeAttempt(attempt))
				}
				store.mu.Unlock()
				if len(attempts) != 1 || !judgeAttemptWasUnexpectedAwaitingApproval(attempts[0]) {
					t.Fatalf("persisted unexpected judge attempts = %#v", attempts)
				}
			}
		})
	}
}

func TestExecutorShouldApplyExactTurnExhaustionPolicy(t *testing.T) {
	tests := []struct {
		name            string
		onExhausted     string
		wantDisposition loop.ActionDisposition
	}{
		{
			name:            "halt",
			onExhausted:     dsl.GoalOnExhaustedHalt,
			wantDisposition: loop.ActionDispositionExhausted,
		},
		{
			name:            "escalate",
			onExhausted:     dsl.GoalOnExhaustedEscalate,
			wantDisposition: loop.ActionDispositionNeedsApproval,
		},
	}
	for _, tc := range tests {
		t.Run("Should "+tc.name+" at the effective turn limit", func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
			judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeRejected, 0)}}
			node := testGoalNode(1)
			node.Params["on_exhausted"] = tc.onExhausted
			executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
			raw, err := executor.Execute(context.Background(), node, testGoalInput(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != tc.wantDisposition ||
				raw.Control.Cause != loop.ReasonCodeGoalTurnsExhausted {
				t.Fatalf("Control = %#v, want disposition:%q turn exhaustion", raw.Control, tc.wantDisposition)
			}
		})
	}
}

func TestExecutorShouldKeepReportIntentAtPromptBoundary(t *testing.T) {
	t.Run("Should consume an evidenced blocked report without invoking the judge", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedPrompt{
			result: scriptedStop(loop.ActionStopEndTurn).result,
			report: &ReportIntent{
				Status:      "blocked",
				EvidenceRef: "evidence:dependency",
				ActorKind:   "agent_session",
				ActorID:     "session-reporter",
			},
		})
		judge := &fakeJudge{}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
		raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionBlocked {
			t.Fatalf("Control = %#v, want blocked", raw.Control)
		}
		if judge.callCount() != 0 {
			t.Fatalf("judge calls = %d, want 0", judge.callCount())
		}
		_, turns, controls := store.snapshot()
		if len(turns) != 1 || len(controls) != 1 {
			t.Fatalf("turn/control writes = %d/%d, want 1/1", len(turns), len(controls))
		}
		if controls[0].ActorKind != "agent_session" || controls[0].ActorID != "session-reporter" {
			t.Fatalf("control actor = %s/%s", controls[0].ActorKind, controls[0].ActorID)
		}
	})

	for _, terminal := range []struct {
		name   string
		result loop.ActionPromptResult
	}{
		{
			name: "failed prompt",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeFailed,
				ReasonCode: loop.ReasonCodeGoalPromptRequestFailed,
			},
		},
		{
			name: "ambiguous prompt",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeAmbiguous,
				ReasonCode: loop.ReasonCodeGoalRecoveryAmbiguous,
			},
		},
	} {
		t.Run("Should preserve an evidenced blocked report over a "+terminal.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			binder := newFakeManagedBinder(store, scriptedPrompt{
				result: terminal.result,
				report: &ReportIntent{
					Status:      "blocked",
					EvidenceRef: "evidence:blocked-terminal-precedence",
					ActorKind:   "agent_session",
					ActorID:     "session-reporter",
				},
			})
			judge := &fakeJudge{}
			executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})

			raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionBlocked {
				t.Fatalf("Control = %#v, want blocked", raw.Control)
			}
			if judge.callCount() != 0 {
				t.Fatalf("judge calls = %d, want 0", judge.callCount())
			}
		})
	}

	t.Run("Should judge a complete report and continue after rejection", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store,
			scriptedPrompt{
				result: scriptedStop(loop.ActionStopEndTurn).result,
				report: &ReportIntent{Status: "complete", ActorKind: "agent_session", ActorID: "session-reporter"},
			},
			scriptedStop(loop.ActionStopEndTurn),
		)
		judge := &fakeJudge{results: []JudgeResult{
			judgeResult(gate.VerdictOutcomeRejected, 0),
			judgeResult(gate.VerdictOutcomeApproved, 0),
		}}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
		raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
			t.Fatalf("Control = %#v, want succeeded", raw.Control)
		}
		if judge.callCount() != 2 {
			t.Fatalf("judge calls = %d, want 2", judge.callCount())
		}
	})

	t.Run("Should settle an approved complete report before a concurrent pause", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedPrompt{
			result: scriptedStop(loop.ActionStopEndTurn).result,
			report: &ReportIntent{
				Status:    "complete",
				ActorKind: "agent_session",
				ActorID:   "session-reporter",
			},
			pauseActor: &causalActor{kind: "human", id: "operator:pause"},
		})
		judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})

		raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
			t.Fatalf("Control = %#v, want authoritative completion", raw.Control)
		}
		if judge.callCount() != 1 {
			t.Fatalf("judge calls = %d, want 1", judge.callCount())
		}
	})
}

func TestExecutorShouldProjectAuthoritativeTerminalStatus(t *testing.T) {
	t.Run("Should structure an approved plain-text candidate from the durable Goal control", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedEndTurn("GREEN", 1))
		executor := newTestExecutor(
			t,
			store,
			binder,
			&fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}},
			&fakeBudgetGuard{},
		)
		node := testGoalNode(1)
		node.Params["output_schema"] = map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"properties": map[string]any{
				"status": map[string]any{"type": "string", "enum": []any{"complete", "blocked"}},
			},
			"required": []any{"status"},
		}

		raw, err := executor.Execute(context.Background(), node, testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got, want := raw.Text, "GREEN"; got != want {
			t.Fatalf("ActionRawResult.Text = %q, want candidate %q", got, want)
		}
		if got, want := string(raw.Structured), `{"status":"complete"}`; got != want {
			t.Fatalf("ActionRawResult.Structured = %s, want %s", got, want)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
			t.Fatalf("ActionRawResult.Control = %#v, want succeeded", raw.Control)
		}
	})
}

func TestExecutorShouldSettlePromptBeforeYieldingAnOperatorPause(t *testing.T) {
	t.Run("Should preserve the dispatch actor, skip judging, and return the authenticated pause", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedPrompt{
			result:     scriptedStop(loop.ActionStopEndTurn).result,
			pauseActor: &causalActor{kind: "human", id: "operator:pause"},
		})
		judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})

		raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionPaused ||
			raw.Control.Cause != loop.ReasonCode(loop.TransitionCausePauseBoundary) {
			t.Fatalf("Control = %#v, want operator pause", raw.Control)
		}
		if judge.callCount() != 0 {
			t.Fatalf("judge calls = %d, want 0 after Pause won", judge.callCount())
		}
		checkpoint, turns, _ := store.snapshot()
		if len(turns) != 1 || turns[0].DispatchActorKind != "daemon" ||
			turns[0].DispatchActorID != "loop-action" {
			t.Fatalf("turn audit = %#v, want one daemon-dispatched settlement", turns)
		}
		if checkpoint.Phase != checkpointPhaseAwaitingControl || checkpoint.Status != goalStatusPaused ||
			checkpoint.ControlActorKind != "human" || checkpoint.ControlActorID != "operator:pause" {
			t.Fatalf("paused checkpoint = %#v", checkpoint)
		}
	})
}

func TestExecutorShouldFenceBudgetAtEveryExternalBoundary(t *testing.T) {
	tests := []struct {
		name         string
		boundary     BudgetBoundary
		judgeOutcome gate.VerdictOutcome
		wantPrompts  int
		wantTurns    int
		wantJudges   int
	}{
		{name: "before work", boundary: BudgetBeforeWork, wantPrompts: 0, wantTurns: 0, wantJudges: 0},
		{name: "after work", boundary: BudgetAfterWork, wantPrompts: 1, wantTurns: 0, wantJudges: 0},
		{name: "before judge", boundary: BudgetBeforeJudge, wantPrompts: 1, wantTurns: 0, wantJudges: 0},
		{
			name:         "after judge",
			boundary:     BudgetAfterJudge,
			judgeOutcome: gate.VerdictOutcomeRejected,
			wantPrompts:  1,
			wantTurns:    1,
			wantJudges:   1,
		},
	}
	for _, tc := range tests {
		t.Run("Should fence "+tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
			judgeOutcome := tc.judgeOutcome
			if judgeOutcome == "" {
				judgeOutcome = gate.VerdictOutcomeApproved
			}
			judge := &fakeJudge{results: []JudgeResult{judgeResult(judgeOutcome, 0)}}
			budget := &fakeBudgetGuard{decisions: map[BudgetBoundary][]BudgetDecision{
				tc.boundary: {{
					Allowed:     false,
					Disposition: loop.ActionDispositionNeedsApproval,
					Cause:       loop.ReasonCodeGoalBudgetFenced,
				}},
			}}
			executor := newTestExecutor(t, store, binder, judge, budget)
			raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionNeedsApproval ||
				raw.Control.GoalStatus != goalStatusBudgetLimited {
				t.Fatalf("Control = %#v, want budget approval", raw.Control)
			}
			_, turns, _ := store.snapshot()
			if got := len(binder.preparedRequests()); got != tc.wantPrompts {
				t.Fatalf("prompts = %d, want %d", got, tc.wantPrompts)
			}
			if len(turns) != tc.wantTurns || judge.callCount() != tc.wantJudges {
				t.Fatalf("turns/judges = %d/%d, want %d/%d", len(turns), judge.callCount(), tc.wantTurns, tc.wantJudges)
			}
		})
	}
}

func TestExecutorShouldFenceBudgetAroundCompaction(t *testing.T) {
	tests := []struct {
		name        string
		boundary    BudgetBoundary
		scripts     []scriptedPrompt
		wantPrompts int
	}{
		{name: "before compact", boundary: BudgetBeforeCompact},
		{
			name:        "after compact",
			boundary:    BudgetAfterCompact,
			scripts:     []scriptedPrompt{scriptedStop(loop.ActionStopEndTurn)},
			wantPrompts: 1,
		},
	}
	for _, tc := range tests {
		t.Run("Should fence "+tc.name, func(t *testing.T) {
			t.Parallel()

			input := testGoalInput(t)
			store := newFakeExecutorStore()
			store.installCheckpoint(Checkpoint{
				Key: TurnKey{
					WorkspaceID: input.WorkspaceID,
					LoopRunID:   input.LoopRunID,
					Generation:  input.Generation,
					NodeID:      input.NodeID,
					ItemIndex:   input.ItemIndex,
				},
				ControlEpoch:      1,
				Phase:             checkpointPhaseIdle,
				Status:            goalStatusActive,
				TurnLimit:         3,
				TaskRunID:         input.CorrelationID,
				BindingEpoch:      1,
				ContextState:      "known",
				ContextNudgeRatio: 0.8,
			})
			binder := newFakeManagedBinder(store, tc.scripts...)
			budget := &fakeBudgetGuard{decisions: map[BudgetBoundary][]BudgetDecision{
				tc.boundary: {{
					Allowed:     false,
					Disposition: loop.ActionDispositionNeedsApproval,
					Cause:       loop.ReasonCodeGoalBudgetFenced,
				}},
			}}
			contextHealth := &fakeContextHealth{
				usage:      ContextUsage{Known: true, Used: 9, Size: 10, Sequence: 1},
				hasCompact: true,
			}
			executor := newTestExecutorWithContext(
				t,
				store,
				binder,
				&fakeJudge{},
				budget,
				contextHealth,
			)
			raw, err := executor.Execute(context.Background(), testGoalNode(3), input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionNeedsApproval ||
				raw.Control.Cause != loop.ReasonCodeGoalBudgetFenced {
				t.Fatalf("Control = %#v, want budget approval", raw.Control)
			}
			if got, want := contextHealth.advertisedCommand(), "compact"; got != want {
				t.Fatalf("advertised command = %q, want %q", got, want)
			}
			requests := binder.preparedRequests()
			if len(requests) != tc.wantPrompts {
				t.Fatalf("compaction prompts = %d, want %d", len(requests), tc.wantPrompts)
			}
			if len(requests) == 1 && requests[0].Kind != promptKindCompact {
				t.Fatalf("prompt kind = %q, want compact", requests[0].Kind)
			}
			_, turns, _ := store.snapshot()
			if len(turns) != 0 {
				t.Fatalf("work turns = %#v, want none around compaction fence", turns)
			}
		})
	}
}

func TestExecutorShouldNeverReplayRecoveredJudgeEffects(t *testing.T) {
	tests := []struct {
		name            string
		criterionType   dsl.CriterionType
		attemptStatus   string
		attemptOutcome  string
		wantDisposition loop.ActionDisposition
		wantCause       loop.ReasonCode
	}{
		{
			name:            "running command attempt becomes ambiguous",
			criterionType:   dsl.CriterionCommand,
			attemptStatus:   "running",
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalRecoveryAmbiguous,
		},
		{
			name:            "running agent judge attempt becomes ambiguous",
			criterionType:   dsl.CriterionAgentJudge,
			attemptStatus:   "running",
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalRecoveryAmbiguous,
		},
		{
			name:            "running extension attempt becomes ambiguous",
			criterionType:   dsl.CriterionExtension,
			attemptStatus:   "running",
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalRecoveryAmbiguous,
		},
		{
			name:            "completed attempt resumes settlement",
			criterionType:   dsl.CriterionAgentJudge,
			attemptStatus:   "completed",
			attemptOutcome:  string(gate.VerdictOutcomeApproved),
			wantDisposition: loop.ActionDispositionSucceeded,
		},
	}
	for _, tc := range tests {
		t.Run("Should recover "+tc.name, func(t *testing.T) {
			t.Parallel()

			input := testGoalInput(t)
			key := TurnKey{
				WorkspaceID: input.WorkspaceID,
				LoopRunID:   input.LoopRunID,
				Generation:  input.Generation,
				NodeID:      input.NodeID,
				ItemIndex:   input.ItemIndex,
			}
			store := newFakeExecutorStore()
			checkpoint := Checkpoint{
				Key:               key,
				ControlEpoch:      1,
				Phase:             checkpointPhaseJudging,
				Status:            goalStatusActive,
				TurnsUsed:         1,
				TurnLimit:         3,
				TaskRunID:         input.CorrelationID,
				QueueEntryID:      "queue:recovered",
				PromptID:          "prompt:recovered",
				PromptKind:        promptKindWork,
				SessionID:         "session-1",
				BindingHandle:     "goal:handle",
				BindingEpoch:      1,
				JudgeAttemptID:    "judge:recovered",
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
			}
			store.installCheckpoint(checkpoint)
			store.judgeAttempts[checkpoint.JudgeAttemptID] = JudgeAttempt{
				AttemptID: checkpoint.JudgeAttemptID,
				Key:       key,
				Turn:      1,
				Status:    tc.attemptStatus,
				Outcome:   tc.attemptOutcome,
			}
			binder := newFakeManagedBinder(store)
			binder.terminals[checkpoint.PromptID] = loop.ActionPromptResult{
				PromptID:   checkpoint.PromptID,
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			}
			judge := &fakeJudge{}
			budget := &fakeBudgetGuard{}
			node := testGoalNode(3)
			node.Params["judge"] = []any{testJudgeCriterion(tc.criterionType)}
			executor := newTestExecutor(t, store, binder, judge, budget)
			raw, err := executor.Execute(context.Background(), node, input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != tc.wantDisposition ||
				raw.Control.Cause != tc.wantCause {
				t.Fatalf("Control = %#v, want disposition:%q cause:%q", raw.Control, tc.wantDisposition, tc.wantCause)
			}
			if judge.callCount() != 0 {
				t.Fatalf("judge calls = %d, want 0", judge.callCount())
			}
			if tc.attemptStatus == "completed" {
				boundaries := budget.boundaries()
				if len(boundaries) != 1 || boundaries[0].Boundary != BudgetAfterJudge {
					t.Fatalf("recovered judge budget boundaries = %#v, want one after_judge", boundaries)
				}
			} else {
				again, secondErr := executor.Execute(context.Background(), node, input)
				if secondErr != nil {
					t.Fatalf("Execute(after ambiguous judge commit) error = %v", secondErr)
				}
				if again.Control == nil || again.Control.Disposition != loop.ActionDispositionNeedsApproval ||
					again.Control.Cause != loop.ReasonCodeGoalRecoveryAmbiguous {
					t.Fatalf("reloaded ambiguous judge Control = %#v", again.Control)
				}
				if judge.callCount() != 0 || store.ambiguousCount() != 1 {
					t.Fatalf(
						"reloaded judge calls/ambiguities = %d/%d, want 0/1",
						judge.callCount(),
						store.ambiguousCount(),
					)
				}
			}
		})
	}
}

func TestExecutorShouldContinueAfterApprovedAmbiguousJudgeRecovery(t *testing.T) {
	t.Run("Should start the next turn without hydrating the ambiguous verdict", func(t *testing.T) {
		t.Parallel()

		input := testGoalInput(t)
		input.CorrelationID = "task-run-2"
		input.GoalSegmentEpoch = 2
		node := testGoalNode(3)
		var params dsl.GoalParams
		if err := node.Params.Decode(&params); err != nil {
			t.Fatalf("Decode(Goal params) error = %v", err)
		}
		key := TurnKey{
			WorkspaceID: input.WorkspaceID,
			LoopRunID:   input.LoopRunID,
			Generation:  input.Generation,
			NodeID:      input.NodeID,
			ItemIndex:   input.ItemIndex,
		}
		attemptID, _, err := deterministicJudgeIdentity(key, 1, params.Judge)
		if err != nil {
			t.Fatalf("deterministicJudgeIdentity() error = %v", err)
		}
		store := newFakeExecutorStore()
		store.installCheckpoint(Checkpoint{
			Key:               key,
			ControlEpoch:      2,
			Phase:             checkpointPhaseIdle,
			Status:            goalStatusActive,
			TurnsUsed:         1,
			TurnLimit:         3,
			TaskRunID:         input.CorrelationID,
			SessionID:         "session-recovered",
			BindingHandle:     "goal:recovered",
			BindingEpoch:      1,
			ContextState:      contextStateUnknown,
			ContextNudgeRatio: 0.8,
		})
		store.judgeAttempts[attemptID] = JudgeAttempt{
			AttemptID: attemptID,
			Key:       key,
			Turn:      1,
			Status:    judgeAttemptStatusAmbiguous,
		}
		binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
		judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})

		raw, err := executor.Execute(context.Background(), node, input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
			t.Fatalf("Control = %#v, want succeeded", raw.Control)
		}
		requests := binder.preparedRequests()
		if len(requests) != 1 || requests[0].Kind != promptKindContinuation {
			t.Fatalf("recovery successor requests = %#v, want one continuation", requests)
		}
		owner := requests[0].Owner
		if owner.Turn != 2 || owner.ControlEpoch != 2 || owner.TaskRunID != input.CorrelationID {
			t.Fatalf("recovery successor owner = %#v, want turn 2 at control epoch 2", owner)
		}
		if strings.Contains(requests[0].Message, "Last authoritative judge outcome") {
			t.Fatalf("continuation hydrated an ambiguous verdict:\n%s", requests[0].Message)
		}
		if got := judge.callCount(); got != 1 {
			t.Fatalf("judge calls = %d, want one new-turn evaluation", got)
		}
	})
}

func TestExecutorShouldRecoverPromptCheckpointsWithoutReplayingEffects(t *testing.T) {
	tests := []struct {
		name             string
		phase            string
		turnsUsed        int
		scripts          []scriptedPrompt
		awaitFailure     bool
		recoveryResult   loop.ActionPromptResult
		recoveryFound    bool
		wantDisposition  loop.ActionDisposition
		wantCause        loop.ReasonCode
		wantPrepareCalls int
		wantJudgeCalls   int
		wantAmbiguous    int
	}{
		{
			name:             "pre-claim prepare",
			phase:            checkpointPhasePreparing,
			scripts:          []scriptedPrompt{scriptedStop(loop.ActionStopEndTurn)},
			wantDisposition:  loop.ActionDispositionSucceeded,
			wantPrepareCalls: 1,
			wantJudgeCalls:   1,
		},
		{
			name:             "durably prepared ticket",
			phase:            checkpointPhaseQueued,
			scripts:          []scriptedPrompt{scriptedStop(loop.ActionStopEndTurn)},
			wantDisposition:  loop.ActionDispositionSucceeded,
			wantPrepareCalls: 1,
			wantJudgeCalls:   1,
		},
		{
			name:            "started effect without terminal proof",
			phase:           checkpointPhasePrompting,
			turnsUsed:       1,
			awaitFailure:    true,
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalRecoveryAmbiguous,
			wantAmbiguous:   1,
		},
		{
			name:         "started effect with correlated terminal proof",
			phase:        checkpointPhasePrompting,
			turnsUsed:    1,
			awaitFailure: true,
			recoveryResult: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
			recoveryFound:   true,
			wantDisposition: loop.ActionDispositionSucceeded,
			wantJudgeCalls:  1,
		},
	}
	for _, tc := range tests {
		t.Run("Should recover "+tc.name, func(t *testing.T) {
			t.Parallel()

			input := testGoalInput(t)
			key := TurnKey{
				WorkspaceID: input.WorkspaceID,
				LoopRunID:   input.LoopRunID,
				Generation:  input.Generation,
				NodeID:      input.NodeID,
				ItemIndex:   input.ItemIndex,
			}
			store := newFakeExecutorStore()
			checkpoint := Checkpoint{
				Key:               key,
				ControlEpoch:      1,
				Phase:             tc.phase,
				Status:            goalStatusActive,
				TurnsUsed:         tc.turnsUsed,
				TurnLimit:         3,
				TaskRunID:         input.CorrelationID,
				QueueEntryID:      "queue:recover-work",
				PromptID:          "prompt:recover-work",
				PromptKind:        promptKindWork,
				SessionID:         "session-1",
				BindingHandle:     "goal:recover",
				BindingEpoch:      1,
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
			}
			store.installCheckpoint(checkpoint)
			binder := newFakeManagedBinder(store, tc.scripts...)
			if tc.awaitFailure {
				binder.awaitErrors[checkpoint.PromptID] = errors.New("worker restarted")
			}
			recoveryResult := tc.recoveryResult
			if tc.recoveryFound {
				recoveryResult.PromptID = checkpoint.PromptID
			}
			recovery := &fakePromptRecovery{
				result: recoveryResult,
				found:  tc.recoveryFound,
			}
			judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
			budget := &fakeBudgetGuard{decisions: map[BudgetBoundary][]BudgetDecision{}}
			executor, err := NewExecutor(Dependencies{
				Store:    store,
				Binder:   binder,
				Judge:    judge,
				Budget:   budget,
				Context:  &fakeContextHealth{},
				Recovery: recovery,
			})
			if err != nil {
				t.Fatalf("NewExecutor() error = %v", err)
			}
			raw, err := executor.Execute(context.Background(), testGoalNode(3), input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != tc.wantDisposition ||
				raw.Control.Cause != tc.wantCause {
				t.Fatalf("Control = %#v, want disposition:%q cause:%q", raw.Control, tc.wantDisposition, tc.wantCause)
			}
			if got := len(binder.preparedRequests()); got != tc.wantPrepareCalls {
				t.Fatalf("prepare calls = %d, want %d", got, tc.wantPrepareCalls)
			}
			if got := judge.callCount(); got != tc.wantJudgeCalls {
				t.Fatalf("judge calls = %d, want %d", got, tc.wantJudgeCalls)
			}
			if got := store.ambiguousCount(); got != tc.wantAmbiguous {
				t.Fatalf("ambiguous settlements = %d, want %d", got, tc.wantAmbiguous)
			}
			if tc.awaitFailure && recovery.calls != 1 {
				t.Fatalf("recovery calls = %d, want 1", recovery.calls)
			}
			if tc.wantAmbiguous == 1 {
				again, secondErr := executor.Execute(context.Background(), testGoalNode(3), input)
				if secondErr != nil {
					t.Fatalf("Execute(after ambiguous prompt commit) error = %v", secondErr)
				}
				if again.Control == nil || again.Control.Disposition != loop.ActionDispositionNeedsApproval ||
					again.Control.Cause != loop.ReasonCodeGoalRecoveryAmbiguous {
					t.Fatalf("reloaded ambiguous prompt Control = %#v", again.Control)
				}
				if recovery.calls != 1 || store.ambiguousCount() != 1 || len(binder.preparedRequests()) != 0 {
					t.Fatalf(
						"reloaded prompt recovery/ambiguity/prepares = %d/%d/%d, want 1/1/0",
						recovery.calls,
						store.ambiguousCount(),
						len(binder.preparedRequests()),
					)
				}
			}
		})
	}
}

func TestExecutorShouldRejectAnUnnormalizedMissingStopReason(t *testing.T) {
	t.Run("Should fail closed before judging a completed result with no stop reason", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedPrompt{result: loop.ActionPromptResult{
			Outcome: loop.ActionPromptOutcomeCompleted,
		}})
		judge := &fakeJudge{}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
		_, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
		if err == nil || !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("Execute() error = %v, want validation failure", err)
		}
		if judge.callCount() != 0 {
			t.Fatalf("judge calls = %d, want 0", judge.callCount())
		}
	})
}

func TestExecutorShouldApplyContextTelemetryPolicy(t *testing.T) {
	tests := []struct {
		name    string
		context *fakeContextHealth
	}{
		{
			name:    "usage read failure",
			context: &fakeContextHealth{usageErr: errors.New("telemetry unavailable")},
		},
		{
			name: "command discovery failure",
			context: &fakeContextHealth{
				usage:      ContextUsage{Known: true, Used: 9, Size: 10},
				hasCompact: true,
				commandErr: errors.New("command catalog unavailable"),
			},
		},
	}
	for _, tc := range tests {
		t.Run("Should continue work after "+tc.name, func(t *testing.T) {
			t.Parallel()

			input := testGoalInput(t)
			store := newFakeExecutorStore()
			store.installCheckpoint(Checkpoint{
				Key: TurnKey{
					WorkspaceID: input.WorkspaceID,
					LoopRunID:   input.LoopRunID,
					Generation:  input.Generation,
					NodeID:      input.NodeID,
					ItemIndex:   input.ItemIndex,
				},
				ControlEpoch:      1,
				Phase:             checkpointPhaseIdle,
				Status:            goalStatusActive,
				TurnLimit:         3,
				TaskRunID:         input.CorrelationID,
				BindingEpoch:      1,
				ContextState:      "known",
				ContextNudgeRatio: 0.8,
			})
			binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
			judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
			budget := &fakeBudgetGuard{decisions: map[BudgetBoundary][]BudgetDecision{}}
			executor, err := NewExecutor(Dependencies{
				Store:    store,
				Binder:   binder,
				Judge:    judge,
				Budget:   budget,
				Context:  tc.context,
				Recovery: &fakePromptRecovery{},
			})
			if err != nil {
				t.Fatalf("NewExecutor() error = %v", err)
			}
			raw, err := executor.Execute(context.Background(), testGoalNode(3), input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
				t.Fatalf("Control = %#v, want succeeded", raw.Control)
			}
			if got := promptKinds(binder.preparedRequests()); !slices.Equal(got, []string{promptKindWork}) {
				t.Fatalf("prompt kinds = %v, want work without speculative compaction", got)
			}
		})
	}

	t.Run(
		"Should promote unknown context and restore known only from a newer post-compaction sequence",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 7, 10, 18, 0, 0, 0, time.UTC)
			before := ContextUsage{Known: true, Used: 9, Size: 10, Sequence: 10, ReportedAt: now}
			after := ContextUsage{Known: true, Used: 3, Size: 10, Sequence: 11, ReportedAt: now.Add(time.Second)}
			store := newFakeExecutorStore()
			binder := newFakeManagedBinder(
				store,
				scriptedStop(loop.ActionStopEndTurn),
				scriptedStop(loop.ActionStopEndTurn),
			)
			contextHealth := &fakeContextHealth{
				usage:      after,
				usages:     []ContextUsage{before, after},
				hasCompact: true,
			}
			executor := newTestExecutorWithContext(
				t,
				store,
				binder,
				&fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}},
				&fakeBudgetGuard{},
				contextHealth,
			)

			raw, err := executor.Execute(t.Context(), testGoalNode(3), testGoalInput(t))
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
				t.Fatalf("Control = %#v, want succeeded", raw.Control)
			}
			if got, want := promptKinds(
				binder.preparedRequests(),
			), []string{
				promptKindCompact,
				promptKindWork,
			}; !slices.Equal(
				got,
				want,
			) {
				t.Fatalf("prompt kinds = %#v, want %#v", got, want)
			}
			checkpoint, _, _ := store.snapshot()
			if checkpoint.ContextState != contextStateKnown || checkpoint.UsageSequence == nil ||
				*checkpoint.UsageSequence != after.Sequence || checkpoint.UsagePendingAfterSequence != nil {
				t.Fatalf("post-compaction context = %#v", checkpoint)
			}
			compactions := store.compactionSnapshot()
			if len(compactions) != 1 || compactions[0].Result.UsageSequence == nil ||
				*compactions[0].Result.UsageSequence != before.Sequence {
				t.Fatalf("compaction freshness floor = %#v, want sequence %d", compactions, before.Sequence)
			}
		},
	)
}

func TestExecutorShouldRestorePriorBlockingIssuesAfterRestart(t *testing.T) {
	t.Run(
		"Should render the effective limit and durable blockers without replaying the prior prompt",
		func(t *testing.T) {
			t.Parallel()

			input := testGoalInput(t)
			node := testGoalNode(5)
			var params dsl.GoalParams
			if err := node.Params.Decode(&params); err != nil {
				t.Fatalf("Decode(Goal params) error = %v", err)
			}
			key := TurnKey{
				WorkspaceID: input.WorkspaceID,
				LoopRunID:   input.LoopRunID,
				Generation:  input.Generation,
				NodeID:      input.NodeID,
				ItemIndex:   input.ItemIndex,
			}
			attemptID, _, err := deterministicJudgeIdentity(key, 1, params.Judge)
			if err != nil {
				t.Fatalf("deterministicJudgeIdentity() error = %v", err)
			}
			store := newFakeExecutorStore()
			store.installCheckpoint(Checkpoint{
				Key:               key,
				ControlEpoch:      1,
				Phase:             checkpointPhaseIdle,
				Status:            goalStatusActive,
				TurnsUsed:         1,
				TurnLimit:         5,
				TaskRunID:         input.CorrelationID,
				SessionID:         "session-restarted",
				BindingHandle:     "goal:restarted",
				BindingEpoch:      1,
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
			})
			store.judgeAttempts[attemptID] = JudgeAttempt{
				AttemptID: attemptID,
				Key:       key,
				Turn:      1,
				Status:    "completed",
				Outcome:   string(gate.VerdictOutcomeRejected),
				BlockingIssues: []gate.BlockingIssue{{
					ID:   "missing-proof",
					Note: "Attach the durable verification evidence",
				}},
			}
			binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
			judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
			executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})

			raw, err := executor.Execute(context.Background(), node, input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
				t.Fatalf("Control = %#v, want succeeded", raw.Control)
			}
			requests := binder.preparedRequests()
			if len(requests) != 1 || requests[0].Kind != promptKindContinuation {
				t.Fatalf("restarted prompt requests = %#v", requests)
			}
			message := requests[0].Message
			for _, want := range []string{
				"work turn 2 of 5",
				"This is a continuation",
				"do not replay an earlier prompt blindly",
				"Last authoritative judge outcome: rejected",
				"[missing-proof] Attach the durable verification evidence",
			} {
				if !strings.Contains(message, want) {
					t.Fatalf("restarted prompt missing %q:\n%s", want, message)
				}
			}
		},
	)
}

func TestExecutorShouldRetryOnlyDurablyRejectedPreSubmitAttempts(t *testing.T) {
	t.Run("Should advance the durable prompt attempt without allocating a rejected turn", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(
			store,
			scriptedPrompt{result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeRejectedBeforeSubmit,
				ReasonCode: "goal_session_busy_queue_full",
			}},
			scriptedStop(loop.ActionStopEndTurn),
		)
		judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
		node := testGoalNode(3)
		node.Retry = &dsl.RetrySpec{MaxAttempts: 2, OnFailure: "fresh_session"}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})

		raw, err := executor.Execute(context.Background(), node, testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
			t.Fatalf("Control = %#v, want succeeded", raw.Control)
		}
		requests := binder.preparedRequests()
		if len(requests) != 2 || requests[0].Owner.PromptAttempt != 0 || requests[1].Owner.PromptAttempt != 1 ||
			requests[0].PromptID == requests[1].PromptID {
			t.Fatalf("pre-submit retry requests = %#v", requests)
		}
		if len(binder.binds) != 2 || binder.binds[0].TargetBindingEpoch != 1 ||
			binder.binds[1].TargetBindingEpoch != 2 || raw.SessionID != "session-2" {
			t.Fatalf("fresh-session pre-submit retry bindings = %#v, session=%q", binder.binds, raw.SessionID)
		}
		_, turns, _ := store.snapshot()
		if len(turns) != 1 || turns[0].PromptID != requests[1].PromptID {
			t.Fatalf("durable turns = %#v, want only the accepted attempt", turns)
		}
	})

	t.Run("Should yield approval when the total attempt limit is exhausted", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedPrompt{result: loop.ActionPromptResult{
			Outcome:    loop.ActionPromptOutcomeRejectedBeforeSubmit,
			ReasonCode: "goal_session_busy_queue_full",
		}})
		executor := newTestExecutor(t, store, binder, &fakeJudge{}, &fakeBudgetGuard{})
		raw, err := executor.Execute(context.Background(), testGoalNode(3), testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionNeedsApproval ||
			raw.Control.Cause != loop.ReasonCodeGoalPresubmitRetriesExhausted {
			t.Fatalf("Control = %#v, want pre-submit retries exhausted", raw.Control)
		}
		_, turns, _ := store.snapshot()
		if len(turns) != 0 {
			t.Fatalf("turns = %#v, want none before durable claim", turns)
		}
	})
}

func TestExecutorShouldOwnSessionCreationRetryBudget(t *testing.T) {
	t.Run("Should advance each known-false creation failure and activate a fresh third binding", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
		binder.bindErrors = []error{
			knownFalseActionSessionCreationError("create-1"),
			knownFalseActionSessionCreationError("create-2"),
		}
		node := testGoalNode(3)
		node.Retry = &dsl.RetrySpec{MaxAttempts: 3, OnFailure: dsl.RetryOnFailureFreshSession}
		executor := newTestExecutor(
			t,
			store,
			binder,
			&fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}},
			&fakeBudgetGuard{},
		)

		raw, err := executor.Execute(t.Context(), node, testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded ||
			raw.SessionID != "session-3" {
			t.Fatalf("result = control:%#v session:%q, want succeeded/session-3", raw.Control, raw.SessionID)
		}
		if len(binder.binds) != 3 || len(binder.retries) != 2 {
			t.Fatalf("binding attempts/retries = %d/%d, want 3/2", len(binder.binds), len(binder.retries))
		}
		for index, request := range binder.retries {
			if !request.RetryWithFreshSession || request.ExpectedPromptAttempt != index ||
				request.FailedBinding.BindingEpoch != int64(index+1) {
				t.Fatalf("retry[%d] = %#v", index, request)
			}
		}
		checkpoint, _, _ := store.snapshot()
		if checkpoint.PromptAttempt != 2 || checkpoint.BindingEpoch != 3 {
			t.Fatalf(
				"checkpoint retry identity = prompt:%d binding:%d, want 2/3",
				checkpoint.PromptAttempt,
				checkpoint.BindingEpoch,
			)
		}
	})

	t.Run("Should terminalize the final known-false attempt with the typed exhaustion control", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store)
		binder.bindErrors = []error{
			knownFalseActionSessionCreationError("create-1"),
			knownFalseActionSessionCreationError("create-2"),
		}
		node := testGoalNode(3)
		node.Retry = &dsl.RetrySpec{MaxAttempts: 2, OnFailure: dsl.RetryOnFailureFreshSession}
		executor := newTestExecutor(t, store, binder, &fakeJudge{}, &fakeBudgetGuard{})

		raw, err := executor.Execute(t.Context(), node, testGoalInput(t))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionNeedsApproval ||
			raw.Control.Cause != loop.ReasonCodeGoalPresubmitRetriesExhausted {
			t.Fatalf("Control = %#v, want typed pre-submit exhaustion", raw.Control)
		}
		if len(binder.binds) != 2 || len(binder.retries) != 2 ||
			binder.retries[1].RetryWithFreshSession {
			t.Fatalf("exhausted creation retry trace = binds:%d retries:%#v", len(binder.binds), binder.retries)
		}
	})
}

func knownFalseActionSessionCreationError(code string) error {
	return &loop.ActionSessionCreationError{
		EffectKnownFalse: true,
		Code:             code,
		Err:              errors.New("known-false session creation"),
	}
}

func TestExecutorShouldValidateEveryManagedPromptTerminalShape(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name   string
		result loop.ActionPromptResult
	}{
		{
			name: "completed",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
		},
		{
			name: "invalid result",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeInvalidResult,
				ReasonCode: loop.ReasonCodeGoalStopReasonInvalid,
			},
		},
		{
			name: "failed",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeFailed,
				ReasonCode: loop.ReasonCodeGoalPromptRequestFailed,
			},
		},
		{
			name: "ambiguous recovery",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeAmbiguous,
				ReasonCode: loop.ReasonCodeGoalRecoveryAmbiguous,
			},
		},
		{
			name: "ambiguous revoked control",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeAmbiguous,
				ReasonCode: loop.ReasonCodeGoalControlRevokedInFlight,
			},
		},
		{
			name: "budget fenced",
			result: loop.ActionPromptResult{
				Outcome:          loop.ActionPromptOutcomeBudgetFenced,
				FenceDisposition: loop.ActionDispositionNeedsApproval,
				ReasonCode:       loop.ReasonCodeGoalBudgetFenced,
			},
		},
		{
			name: "control fenced",
			result: loop.ActionPromptResult{
				Outcome:          loop.ActionPromptOutcomeControlFenced,
				FenceDisposition: loop.ActionDispositionPaused,
				ReasonCode:       loop.ReasonCode(loop.TransitionCausePauseBoundary),
			},
		},
		{
			name: "rejected before submit",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeRejectedBeforeSubmit,
				ReasonCode: loop.ReasonCodeGoalPresubmitRetriesExhausted,
			},
		},
	}
	for _, tc := range valid {
		t.Run("Should accept "+tc.name, func(t *testing.T) {
			t.Parallel()

			result := tc.result
			result.PromptID = "prompt-contract"
			if err := validatePromptTerminal("prompt-contract", result); err != nil {
				t.Fatalf("validatePromptTerminal() error = %v", err)
			}
		})
	}

	invalid := []struct {
		name   string
		result loop.ActionPromptResult
	}{
		{
			name: "changed correlation",
			result: loop.ActionPromptResult{
				PromptID:   "other",
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
		},
		{
			name: "negative tokens",
			result: loop.ActionPromptResult{
				PromptID:   "prompt-contract",
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
				TokensUsed: -1,
			},
		},
		{
			name: "reversed event range",
			result: loop.ActionPromptResult{
				PromptID:      "prompt-contract",
				Outcome:       loop.ActionPromptOutcomeCompleted,
				StopReason:    loop.ActionStopEndTurn,
				EventStartSeq: 2,
				EventEndSeq:   1,
			},
		},
		{
			name: "unreported nonzero tokens",
			result: loop.ActionPromptResult{
				PromptID:   "prompt-contract",
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
				TokensUsed: 1,
			},
		},
		{
			name:   "unknown outcome",
			result: loop.ActionPromptResult{PromptID: "prompt-contract", Outcome: loop.ActionPromptOutcome("unknown")},
		},
		{
			name: "completed with reason",
			result: loop.ActionPromptResult{
				PromptID:   "prompt-contract",
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
				ReasonCode: loop.ReasonCodeGoalPromptRequestFailed,
			},
		},
		{
			name: "invalid result with stop",
			result: loop.ActionPromptResult{
				PromptID:   "prompt-contract",
				Outcome:    loop.ActionPromptOutcomeInvalidResult,
				StopReason: loop.ActionStopEndTurn,
				ReasonCode: loop.ReasonCodeGoalStopReasonInvalid,
			},
		},
		{
			name: "ambiguous with unknown cause",
			result: loop.ActionPromptResult{
				PromptID:   "prompt-contract",
				Outcome:    loop.ActionPromptOutcomeAmbiguous,
				ReasonCode: "unknown",
			},
		},
		{
			name: "fenced without disposition",
			result: loop.ActionPromptResult{
				PromptID:   "prompt-contract",
				Outcome:    loop.ActionPromptOutcomeBudgetFenced,
				ReasonCode: loop.ReasonCodeGoalBudgetFenced,
			},
		},
		{
			name: "rejected without reason",
			result: loop.ActionPromptResult{
				PromptID: "prompt-contract",
				Outcome:  loop.ActionPromptOutcomeRejectedBeforeSubmit,
			},
		},
	}
	for _, tc := range invalid {
		t.Run("Should reject "+tc.name, func(t *testing.T) {
			t.Parallel()

			if err := validatePromptTerminal("prompt-contract", tc.result); err == nil ||
				!errors.Is(err, loop.ErrValidation) {
				t.Fatalf("validatePromptTerminal() error = %v, want validation failure", err)
			}
		})
	}
}

func TestGoalContractsShouldValidateIdentityDependenciesAndHarvest(t *testing.T) {
	t.Parallel()

	t.Run("Should validate binding, turn, and query identities", func(t *testing.T) {
		t.Parallel()

		if err := (BindingKey{WorkspaceID: "ws", LoopRunID: "run", Handle: "goal:handle"}).Validate(); err != nil {
			t.Fatalf("BindingKey.Validate() error = %v", err)
		}
		if err := (BindingKey{}).Validate(); err == nil || !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("BindingKey.Validate(empty) error = %v", err)
		}
		validKey := TurnKey{WorkspaceID: "ws", LoopRunID: "run", Generation: 1, NodeID: "goal"}
		if err := validKey.Validate(); err != nil {
			t.Fatalf("TurnKey.Validate() error = %v", err)
		}
		invalidKeys := []TurnKey{
			{LoopRunID: "run", Generation: 1, NodeID: "goal"},
			{WorkspaceID: "ws", Generation: 1, NodeID: "goal"},
			{WorkspaceID: "ws", LoopRunID: "run", NodeID: "goal"},
			{WorkspaceID: "ws", LoopRunID: "run", Generation: 1},
			{WorkspaceID: "ws", LoopRunID: "run", Generation: 1, NodeID: "goal", ItemIndex: -1},
		}
		for index, key := range invalidKeys {
			if err := key.Validate(); err == nil || !errors.Is(err, loop.ErrValidation) {
				t.Fatalf("TurnKey.Validate(invalid=%d) error = %v", index, err)
			}
		}
		itemIndex := 2
		if err := (TurnQuery{
			WorkspaceID: "ws",
			LoopRunID:   "run",
			NodeID:      "goal",
			ItemIndex:   &itemIndex,
			Limit:       200,
		}).Validate(); err != nil {
			t.Fatalf("TurnQuery.Validate() error = %v", err)
		}
		negativeItemIndex := -1
		invalidQueries := []TurnQuery{
			{WorkspaceID: "ws", LoopRunID: "run", AfterSeq: -1},
			{WorkspaceID: "ws", LoopRunID: "run", Limit: 201},
			{WorkspaceID: "ws", LoopRunID: "run", ItemIndex: &negativeItemIndex},
			{WorkspaceID: "ws", LoopRunID: "run", ItemIndex: &itemIndex},
			{WorkspaceID: "ws", LoopRunID: "run", NodeID: "   ", ItemIndex: &itemIndex},
		}
		for index, query := range invalidQueries {
			if err := query.Validate(); err == nil || !errors.Is(err, loop.ErrValidation) {
				t.Fatalf("TurnQuery.Validate(invalid=%d) error = %v", index, err)
			}
		}
	})

	t.Run("Should validate the closed session projection outbox contract", func(t *testing.T) {
		t.Parallel()

		boundSessionID := "session-bound"
		valid := EnqueueSessionOutboxRequest{
			EventID: "goal-event-1", WorkspaceID: "workspace-1",
			OriginSessionID: "session-origin", LoopRunID: "loop-1",
			BoundSessionID: &boundSessionID, Cause: SessionOutboxCauseStart,
			CreatedAt: time.Now().UTC(),
		}
		if err := valid.Validate(); err != nil {
			t.Fatalf("EnqueueSessionOutboxRequest.Validate(start) error = %v", err)
		}
		clearRequest := valid
		clearRequest.Cause = SessionOutboxCauseClear
		clearRequest.BoundSessionID = nil
		if err := clearRequest.Validate(); err != nil {
			t.Fatalf("EnqueueSessionOutboxRequest.Validate(clear) error = %v", err)
		}
		for _, cause := range []SessionOutboxCause{
			SessionOutboxCauseStart,
			SessionOutboxCauseReplace,
			SessionOutboxCauseStatus,
			SessionOutboxCauseClear,
			SessionOutboxCauseReseed,
		} {
			if !cause.Valid() {
				t.Fatalf("SessionOutboxCause(%q).Valid() = false", cause)
			}
		}

		invalid := []struct {
			name   string
			mutate func(*EnqueueSessionOutboxRequest)
		}{
			{name: "event id", mutate: func(req *EnqueueSessionOutboxRequest) { req.EventID = "" }},
			{name: "workspace", mutate: func(req *EnqueueSessionOutboxRequest) { req.WorkspaceID = "" }},
			{name: "origin session", mutate: func(req *EnqueueSessionOutboxRequest) { req.OriginSessionID = "" }},
			{name: "Loop Run", mutate: func(req *EnqueueSessionOutboxRequest) { req.LoopRunID = "" }},
			{name: "blank bound session", mutate: func(req *EnqueueSessionOutboxRequest) {
				blank := " "
				req.BoundSessionID = &blank
			}},
			{name: "unknown cause", mutate: func(req *EnqueueSessionOutboxRequest) {
				req.Cause = SessionOutboxCause("unknown")
			}},
			{name: "clear retaining a binding", mutate: func(req *EnqueueSessionOutboxRequest) {
				req.Cause = SessionOutboxCauseClear
			}},
			{name: "non-clear missing a binding", mutate: func(req *EnqueueSessionOutboxRequest) {
				req.BoundSessionID = nil
			}},
			{name: "creation time", mutate: func(req *EnqueueSessionOutboxRequest) {
				req.CreatedAt = time.Time{}
			}},
		}
		for _, tc := range invalid {
			t.Run("Should reject missing or invalid "+tc.name, func(t *testing.T) {
				t.Parallel()

				request := valid
				tc.mutate(&request)
				if err := request.Validate(); err == nil || !errors.Is(err, loop.ErrValidation) {
					t.Fatalf("EnqueueSessionOutboxRequest.Validate() error = %v", err)
				}
			})
		}
		if SessionOutboxCause("unknown").Valid() {
			t.Fatal("SessionOutboxCause(unknown).Valid() = true")
		}
	})

	t.Run("Should reject every missing executor dependency and apply defaults", func(t *testing.T) {
		t.Parallel()

		newDependencies := func() Dependencies {
			store := newFakeExecutorStore()
			return Dependencies{
				Store:    store,
				Binder:   newFakeManagedBinder(store),
				Judge:    &fakeJudge{},
				Budget:   &fakeBudgetGuard{},
				Context:  &fakeContextHealth{},
				Recovery: &fakePromptRecovery{},
			}
		}
		missing := []struct {
			name string
			err  error
			omit func(*Dependencies)
		}{
			{name: "store", err: errStoreRequired, omit: func(deps *Dependencies) { deps.Store = nil }},
			{name: "binder", err: errBinderRequired, omit: func(deps *Dependencies) { deps.Binder = nil }},
			{name: "judge", err: errJudgeRequired, omit: func(deps *Dependencies) { deps.Judge = nil }},
			{name: "budget", err: errBudgetRequired, omit: func(deps *Dependencies) { deps.Budget = nil }},
			{name: "context", err: errContextRequired, omit: func(deps *Dependencies) { deps.Context = nil }},
			{name: "recovery", err: errRecoveryRequired, omit: func(deps *Dependencies) { deps.Recovery = nil }},
		}
		for _, tc := range missing {
			deps := newDependencies()
			tc.omit(&deps)
			if _, err := NewExecutor(deps); err == nil || !errors.Is(err, tc.err) ||
				!errors.Is(err, loop.ErrActionDependencyMissing) {
				t.Fatalf("NewExecutor(missing %s) error = %v", tc.name, err)
			}
		}
		executor, err := NewExecutor(newDependencies())
		if err != nil {
			t.Fatalf("NewExecutor(defaults) error = %v", err)
		}
		if executor.now == nil || executor.compactionTimeout != defaultCompactionTimeout {
			t.Fatalf(
				"executor defaults = now-nil:%t timeout:%v",
				executor.now == nil,
				executor.compactionTimeout,
			)
		}
	})

	t.Run("Should harvest only succeeded control and clone structured output", func(t *testing.T) {
		t.Parallel()

		executor := &Executor{}
		if _, err := executor.Harvest(t.Context(), loop.ActionRawResult{}, dsl.Node{}); err == nil {
			t.Fatal("Harvest(no control) error = nil")
		}
		blocked := loop.ActionRawResult{Control: &loop.ActionControl{
			Disposition:  loop.ActionDispositionBlocked,
			GoalStatus:   goalStatusBlocked,
			Cause:        loop.ReasonCodeGoalReportedBlocked,
			CheckpointID: "checkpoint-blocked",
		}}
		if _, err := executor.Harvest(t.Context(), blocked, dsl.Node{}); err == nil {
			t.Fatal("Harvest(blocked) error = nil")
		}
		raw := loop.ActionRawResult{
			Structured: []byte(`{"status":"complete"}`),
			Text:       "complete",
			Control: &loop.ActionControl{
				Disposition:  loop.ActionDispositionSucceeded,
				GoalStatus:   goalStatusComplete,
				CheckpointID: "checkpoint-complete",
			},
		}
		output, err := executor.Harvest(t.Context(), raw, dsl.Node{})
		if err != nil {
			t.Fatalf("Harvest(succeeded) error = %v", err)
		}
		output.Structured[0] = '['
		if raw.Structured[0] == '[' || output.Text != raw.Text {
			t.Fatalf("Harvest() output aliases input or lost fields: %#v", output)
		}
	})
}

func TestExecutorShouldMapEveryPersistedAndFencedControl(t *testing.T) {
	t.Parallel()

	persisted := []struct {
		name            string
		status          string
		phase           string
		cause           loop.ReasonCode
		wantDisposition loop.ActionDisposition
		wantCause       loop.ReasonCode
		wantError       bool
	}{
		{
			name:            "complete",
			status:          goalStatusComplete,
			phase:           checkpointPhaseTerminal,
			wantDisposition: loop.ActionDispositionSucceeded,
		},
		{
			name:            "blocked default cause",
			status:          goalStatusBlocked,
			phase:           checkpointPhaseTerminal,
			wantDisposition: loop.ActionDispositionBlocked,
			wantCause:       loop.ReasonCodeGoalReportedBlocked,
		},
		{
			name:            "budget terminal default cause",
			status:          goalStatusBudgetLimited,
			phase:           checkpointPhaseTerminal,
			wantDisposition: loop.ActionDispositionExhausted,
			wantCause:       loop.ReasonCodeGoalBudgetFenced,
		},
		{
			name:            "budget approval",
			status:          goalStatusBudgetLimited,
			phase:           checkpointPhaseAwaitingControl,
			cause:           loop.ReasonCodeGoalTurnsExhausted,
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalTurnsExhausted,
		},
		{
			name:            "usage approval default cause",
			status:          goalStatusUsageLimited,
			phase:           checkpointPhaseAwaitingControl,
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalReseedConfirmationRequired,
		},
		{
			name:            "operator pause",
			status:          goalStatusPaused,
			phase:           checkpointPhaseAwaitingControl,
			cause:           loop.ReasonCode(loop.TransitionCausePauseBoundary),
			wantDisposition: loop.ActionDispositionPaused,
			wantCause:       loop.ReasonCode(loop.TransitionCausePauseBoundary),
		},
		{
			name:            "paused approval default cause",
			status:          goalStatusPaused,
			phase:           checkpointPhaseAwaitingControl,
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalPromptFenced,
		},
		{name: "active is not recoverable", status: goalStatusActive, phase: checkpointPhaseIdle, wantError: true},
	}
	for _, tc := range persisted {
		t.Run("Should recover "+tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			segment := directSegmentForTest(t, store)
			segment.checkpoint.Status = tc.status
			segment.checkpoint.Phase = tc.phase
			segment.checkpoint.ControlCause = tc.cause
			segment.checkpoint.ControlActorKind = "human"
			segment.checkpoint.ControlActorID = "operator:control"
			segment.checkpoint.ReportIntent = &ReportIntent{ActorKind: "agent", ActorID: "agent:report"}
			store.installCheckpoint(segment.checkpoint)
			executor := newTestExecutor(t, store, newFakeManagedBinder(store), &fakeJudge{}, &fakeBudgetGuard{})

			control, err := executor.recoveredControl(t.Context(), segment)
			if tc.wantError {
				if err == nil || !errors.Is(err, loop.ErrValidation) {
					t.Fatalf("recoveredControl() error = %v, want validation failure", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("recoveredControl() error = %v", err)
			}
			if control.Disposition != tc.wantDisposition || control.Cause != tc.wantCause ||
				(control.Disposition == loop.ActionDispositionNeedsApproval && control.GateID == "") {
				t.Fatalf("recovered control = %#v", control)
			}
		})
	}

	fenced := []struct {
		name            string
		result          loop.ActionPromptResult
		wantStatus      string
		wantDisposition loop.ActionDisposition
	}{
		{
			name: "budget approval",
			result: loop.ActionPromptResult{
				Outcome:          loop.ActionPromptOutcomeBudgetFenced,
				FenceDisposition: loop.ActionDispositionNeedsApproval,
				ReasonCode:       loop.ReasonCodeGoalBudgetFenced,
			},
			wantStatus:      goalStatusBudgetLimited,
			wantDisposition: loop.ActionDispositionNeedsApproval,
		},
		{
			name: "operator pause",
			result: loop.ActionPromptResult{
				Outcome:          loop.ActionPromptOutcomeControlFenced,
				FenceDisposition: loop.ActionDispositionPaused,
				ReasonCode:       loop.ReasonCode(loop.TransitionCausePauseBoundary),
			},
			wantStatus:      goalStatusPaused,
			wantDisposition: loop.ActionDispositionPaused,
		},
		{
			name: "reseed approval",
			result: loop.ActionPromptResult{
				Outcome:          loop.ActionPromptOutcomeControlFenced,
				FenceDisposition: loop.ActionDispositionNeedsApproval,
				ReasonCode:       loop.ReasonCodeGoalReseedConfirmationRequired,
			},
			wantStatus:      goalStatusUsageLimited,
			wantDisposition: loop.ActionDispositionNeedsApproval,
		},
	}
	for _, tc := range fenced {
		t.Run("Should map fenced "+tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			segment := directSegmentForTest(t, store)
			segment.checkpoint.ControlActorKind = "human"
			segment.checkpoint.ControlActorID = "operator:fence"
			store.installCheckpoint(segment.checkpoint)
			executor := newTestExecutor(t, store, newFakeManagedBinder(store), &fakeJudge{}, &fakeBudgetGuard{})
			control, err := executor.fencedPromptBoundary(t.Context(), segment, tc.result)
			if err != nil {
				t.Fatalf("fencedPromptBoundary() error = %v", err)
			}
			if control.GoalStatus != tc.wantStatus || control.Disposition != tc.wantDisposition {
				t.Fatalf("fenced control = %#v", control)
			}
		})
	}

	t.Run("Should reject invalid budget decisions and default the denial cause", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		executor := newTestExecutor(t, store, newFakeManagedBinder(store), &fakeJudge{}, &fakeBudgetGuard{})
		for _, decision := range []BudgetDecision{
			{Allowed: true, Disposition: loop.ActionDispositionNeedsApproval},
			{Allowed: false, Disposition: loop.ActionDispositionPaused},
		} {
			if _, err := executor.budgetBoundary(t.Context(), segment, decision); err == nil {
				t.Fatalf("budgetBoundary(%#v) error = nil", decision)
			}
		}
		control, err := executor.budgetBoundary(t.Context(), segment, BudgetDecision{
			Allowed:     false,
			Disposition: loop.ActionDispositionExhausted,
		})
		if err != nil {
			t.Fatalf("budgetBoundary(default cause) error = %v", err)
		}
		if control.Cause != loop.ReasonCodeGoalBudgetFenced ||
			control.Disposition != loop.ActionDispositionExhausted {
			t.Fatalf("budget control = %#v", control)
		}
	})
}

func TestExecutorShouldClassifyEveryCompactionTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		result    loop.ActionPromptResult
		canceled  bool
		want      CompactionOutcome
		wantCause loop.ReasonCode
	}{
		{
			name:      "invalid result",
			result:    loop.ActionPromptResult{Outcome: loop.ActionPromptOutcomeInvalidResult},
			want:      CompactionFailed,
			wantCause: loop.ReasonCodeGoalStopReasonInvalid,
		},
		{
			name:      "failed",
			result:    loop.ActionPromptResult{Outcome: loop.ActionPromptOutcomeFailed},
			want:      CompactionFailed,
			wantCause: loop.ReasonCodeGoalPromptRequestFailed,
		},
		{
			name:      "rejected",
			result:    loop.ActionPromptResult{Outcome: loop.ActionPromptOutcomeRejectedBeforeSubmit},
			want:      CompactionFailed,
			wantCause: loop.ReasonCodeGoalPresubmitRetriesExhausted,
		},
		{
			name:      "unknown outcome",
			result:    loop.ActionPromptResult{Outcome: loop.ActionPromptOutcome("unknown")},
			want:      CompactionFailed,
			wantCause: loop.ReasonCodeGoalCompactionCancelled,
		},
		{
			name: "completed without telemetry",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
			want: CompactionSucceeded,
		},
		{
			name: "effective telemetry",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopMaxTurnRequests,
			},
			want: CompactionSucceeded,
		},
		{
			name: "stale telemetry",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
			want: CompactionSucceeded,
		},
		{
			name: "max tokens",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopMaxTokens,
			},
			want: CompactionIneffective,
		},
		{
			name: "refusal",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopRefusal,
			},
			want: CompactionRefused,
		},
		{
			name: "timeout cancellation",
			result: loop.ActionPromptResult{
				PromptID:   "compact",
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopCancelled,
			},
			canceled: true,
			want:     CompactionTimedOut,
		},
		{
			name: "unexpected cancellation",
			result: loop.ActionPromptResult{
				PromptID:   "compact",
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopCancelled,
			},
			want:      CompactionCancelled,
			wantCause: loop.ReasonCodeGoalCompactionCancelled,
		},
		{
			name: "unknown stop",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopReason("unknown"),
			},
			want:      CompactionFailed,
			wantCause: loop.ReasonCodeGoalStopReasonInvalid,
		},
	}
	for _, tc := range tests {
		t.Run("Should classify "+tc.name, func(t *testing.T) {
			t.Parallel()

			outcome, cause := classifyCompactionResult(tc.result, tc.canceled)
			if outcome != tc.want || cause != tc.wantCause {
				t.Fatalf("classifyCompactionResult() = %q/%q, want %q/%q", outcome, cause, tc.want, tc.wantCause)
			}
		})
	}
}

func TestExecutorShouldSettleCompactionControlBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		result          loop.ActionPromptResult
		beforeUsage     *ContextUsage
		denyAfter       bool
		wantOutcome     CompactionOutcome
		wantCompactions int
		wantDisposition loop.ActionDisposition
		wantCause       loop.ReasonCode
		wantPending     bool
	}{
		{
			name: "success",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
			beforeUsage:     &ContextUsage{Known: true, Used: 9, Size: 10, Sequence: 10},
			wantOutcome:     CompactionSucceeded,
			wantCompactions: 1,
			wantPending:     true,
		},
		{
			name: "invalid result",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeInvalidResult,
				ReasonCode: loop.ReasonCodeGoalStopReasonInvalid,
			},
			wantOutcome:     CompactionFailed,
			wantCompactions: 1,
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalStopReasonInvalid,
		},
		{
			name: "post budget denial",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopEndTurn,
			},
			denyAfter:       true,
			wantOutcome:     CompactionSucceeded,
			wantCompactions: 1,
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalBudgetFenced,
		},
		{
			name: "pre-submit fence",
			result: loop.ActionPromptResult{
				Outcome:          loop.ActionPromptOutcomeBudgetFenced,
				FenceDisposition: loop.ActionDispositionNeedsApproval,
				ReasonCode:       loop.ReasonCodeGoalBudgetFenced,
			},
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalBudgetFenced,
		},
		{
			name: "ambiguous terminal",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeAmbiguous,
				ReasonCode: loop.ReasonCodeGoalRecoveryAmbiguous,
			},
			wantDisposition: loop.ActionDispositionNeedsApproval,
			wantCause:       loop.ReasonCodeGoalRecoveryAmbiguous,
		},
		{
			name: "ineffective reseed",
			result: loop.ActionPromptResult{
				Outcome:    loop.ActionPromptOutcomeCompleted,
				StopReason: loop.ActionStopMaxTokens,
			},
			wantOutcome:     CompactionIneffective,
			wantCompactions: 1,
		},
	}
	for _, tc := range tests {
		t.Run("Should settle "+tc.name, func(t *testing.T) {
			t.Parallel()

			store := newFakeExecutorStore()
			segment := directSegmentForTest(t, store)
			segment.checkpoint.Phase = checkpointPhaseCompacting
			segment.checkpoint.TurnsUsed = 1
			segment.checkpoint.QueueEntryID = "queue:compact"
			segment.checkpoint.PromptID = "prompt:compact"
			segment.checkpoint.PromptKind = promptKindCompact
			store.installCheckpoint(segment.checkpoint)
			binder := newFakeManagedBinder(store)
			budget := &fakeBudgetGuard{}
			if tc.denyAfter {
				budget.decisions = map[BudgetBoundary][]BudgetDecision{
					BudgetAfterCompact: {{
						Allowed:     false,
						Disposition: loop.ActionDispositionNeedsApproval,
						Cause:       loop.ReasonCodeGoalBudgetFenced,
					}},
				}
			}
			executor := newTestExecutor(t, store, binder, &fakeJudge{}, budget)
			result := tc.result
			result.PromptID = segment.checkpoint.PromptID
			boundary, err := executor.processCompactionResult(
				t.Context(),
				segment,
				loop.ActionPromptTicket{QueueEntryID: segment.checkpoint.QueueEntryID, PromptID: result.PromptID},
				result,
				0,
				tc.beforeUsage,
			)
			if err != nil {
				t.Fatalf("processCompactionResult() error = %v", err)
			}
			if boundary == nil {
				t.Fatal("processCompactionResult() boundary = nil")
			}
			if tc.wantDisposition == "" {
				if boundary.control != nil {
					t.Fatalf("compaction control = %#v, want nil", boundary.control)
				}
			} else if boundary.control == nil || boundary.control.Disposition != tc.wantDisposition ||
				boundary.control.Cause != tc.wantCause {
				t.Fatalf("compaction control = %#v", boundary.control)
			}
			compactions := store.compactionSnapshot()
			if len(compactions) != tc.wantCompactions {
				t.Fatalf("compactions = %d, want %d", len(compactions), tc.wantCompactions)
			}
			if len(compactions) == 1 && compactions[0].Result.Outcome != tc.wantOutcome {
				t.Fatalf("compaction outcome = %q, want %q", compactions[0].Result.Outcome, tc.wantOutcome)
			}
			if tc.wantPending {
				checkpoint, _, _ := store.snapshot()
				if checkpoint.ContextState != contextStatePending || checkpoint.CompactionBaselineUsed == nil ||
					*checkpoint.CompactionBaselineUsed != tc.beforeUsage.Used ||
					checkpoint.UsagePendingAfterSequence == nil ||
					*checkpoint.UsagePendingAfterSequence != tc.beforeUsage.Sequence {
					t.Fatalf("delayed compaction freshness = %#v", checkpoint)
				}
			}
		})
	}

	t.Run("Should require approval before reseeding a borrowed binding", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		segment.binding.Ownership = string(BindingOwnershipOriginBorrowed)
		store.installCheckpoint(segment.checkpoint)
		executor := newTestExecutor(t, store, newFakeManagedBinder(store), &fakeJudge{}, &fakeBudgetGuard{})
		boundary, err := executor.reseedOrEscalate(t.Context(), segment, loop.ActionPromptResult{})
		if err != nil {
			t.Fatalf("reseedOrEscalate() error = %v", err)
		}
		if boundary.control == nil || boundary.control.Cause != loop.ReasonCodeGoalReseedConfirmationRequired {
			t.Fatalf("reseed approval = %#v", boundary.control)
		}
	})
}

func TestExecutorShouldRecoverPreparedErrorsAndTimedOutCompaction(t *testing.T) {
	t.Parallel()

	t.Run("Should return a prepare error when no durable prompt exists", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		executor := newTestExecutor(t, store, newFakeManagedBinder(store), &fakeJudge{}, &fakeBudgetGuard{})
		prepareErr := errors.New("prepare failed before persistence")
		if _, err := executor.handlePreparedPromptError(t.Context(), segment, prepareErr); !errors.Is(err, prepareErr) {
			t.Fatalf("handlePreparedPromptError() error = %v", err)
		}
	})

	t.Run("Should reattach a durable prompt after the prepare caller loses its response", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		segment.checkpoint.Phase = checkpointPhasePrompting
		segment.checkpoint.TurnsUsed = 1
		segment.checkpoint.QueueEntryID = "queue:reattach"
		segment.checkpoint.PromptID = "prompt:reattach"
		segment.checkpoint.PromptKind = promptKindWork
		store.installCheckpoint(segment.checkpoint)
		binder := newFakeManagedBinder(store)
		binder.terminals[segment.checkpoint.PromptID] = loop.ActionPromptResult{
			PromptID:   segment.checkpoint.PromptID,
			Outcome:    loop.ActionPromptOutcomeCompleted,
			StopReason: loop.ActionStopEndTurn,
		}
		judge := &fakeJudge{results: []JudgeResult{judgeResult(gate.VerdictOutcomeApproved, 0)}}
		executor := newTestExecutor(t, store, binder, judge, &fakeBudgetGuard{})
		boundary, err := executor.handlePreparedPromptError(t.Context(), segment, errors.New("prepare response lost"))
		if err != nil {
			t.Fatalf("handlePreparedPromptError() error = %v", err)
		}
		if boundary.control == nil || boundary.control.Disposition != loop.ActionDispositionSucceeded ||
			judge.callCount() != 1 {
			t.Fatalf("reattached boundary/judges = %#v/%d", boundary, judge.callCount())
		}
	})

	t.Run("Should persist cancel before reattaching a timed out compaction", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		segment.checkpoint.Phase = checkpointPhaseCompacting
		segment.checkpoint.TurnsUsed = 1
		segment.checkpoint.QueueEntryID = "queue:timeout"
		segment.checkpoint.PromptID = "prompt:timeout"
		segment.checkpoint.PromptKind = promptKindCompact
		store.installCheckpoint(segment.checkpoint)
		binder := newFakeManagedBinder(store)
		binder.blockFirst[segment.checkpoint.PromptID] = true
		binder.terminals[segment.checkpoint.PromptID] = loop.ActionPromptResult{
			PromptID:   segment.checkpoint.PromptID,
			Outcome:    loop.ActionPromptOutcomeCompleted,
			StopReason: loop.ActionStopCancelled,
		}
		executor, err := NewExecutor(Dependencies{
			Store:             store,
			Binder:            binder,
			Judge:             &fakeJudge{},
			Budget:            &fakeBudgetGuard{},
			Context:           &fakeContextHealth{},
			Recovery:          &fakePromptRecovery{},
			CompactionTimeout: time.Millisecond,
		})
		if err != nil {
			t.Fatalf("NewExecutor() error = %v", err)
		}
		result, err := executor.awaitCompaction(
			t.Context(),
			segment,
			loop.ActionPromptTicket{
				QueueEntryID: segment.checkpoint.QueueEntryID,
				PromptID:     segment.checkpoint.PromptID,
			},
			segment.promptOwner(segment.checkpoint.TurnsUsed),
		)
		if err != nil {
			t.Fatalf("awaitCompaction() error = %v", err)
		}
		checkpoint, _, _ := store.snapshot()
		if result.StopReason != loop.ActionStopCancelled || checkpoint.CompactionCancel == nil ||
			checkpoint.CompactionCancel.PromptID != result.PromptID || binder.cancelCount() != 1 {
			t.Fatalf(
				"timed out compaction = result:%#v checkpoint:%#v cancels:%d",
				result,
				checkpoint,
				binder.cancelCount(),
			)
		}
	})

	t.Run("Should replay a persisted timeout cancel and drain after restart", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		segment.checkpoint.Phase = checkpointPhaseCompacting
		segment.checkpoint.TurnsUsed = 1
		segment.checkpoint.QueueEntryID = "queue:recovered-timeout"
		segment.checkpoint.PromptID = "prompt:recovered-timeout"
		segment.checkpoint.PromptKind = promptKindCompact
		segment.checkpoint.CompactionCancel = &CompactionCancelIntent{
			PromptID: segment.checkpoint.PromptID, Cause: "timeout", RequestedAt: time.Now().UTC(),
		}
		store.installCheckpoint(segment.checkpoint)
		binder := newFakeManagedBinder(store)
		binder.terminals[segment.checkpoint.PromptID] = loop.ActionPromptResult{
			PromptID: segment.checkpoint.PromptID, Outcome: loop.ActionPromptOutcomeCompleted,
			StopReason: loop.ActionStopCancelled,
		}
		executor := newTestExecutor(t, store, binder, &fakeJudge{}, &fakeBudgetGuard{})

		boundary, err := executor.recoverPendingPrompt(t.Context(), segment)
		if err != nil {
			t.Fatalf("recoverPendingPrompt() error = %v", err)
		}
		if boundary == nil || binder.cancelCount() != 1 {
			t.Fatalf("recovered timeout boundary/cancels = %#v/%d", boundary, binder.cancelCount())
		}
		compactions := store.compactionSnapshot()
		if len(compactions) != 1 || compactions[0].Result.Outcome != CompactionTimedOut {
			t.Fatalf("recovered timeout compactions = %#v", compactions)
		}
	})

	t.Run("Should rehydrate a queued compaction baseline after restart", func(t *testing.T) {
		t.Parallel()

		store := newFakeExecutorStore()
		segment := directSegmentForTest(t, store)
		sequence := int64(17)
		used := int64(42)
		segment.checkpoint.Phase = checkpointPhaseQueued
		segment.checkpoint.QueueEntryID = "queue:recovered-baseline"
		segment.checkpoint.PromptID = "prompt:recovered-baseline"
		segment.checkpoint.PromptKind = promptKindCompact
		segment.checkpoint.UsageSequence = &sequence
		segment.checkpoint.CompactionBaselineUsed = &used
		store.installCheckpoint(segment.checkpoint)
		binder := newFakeManagedBinder(store, scriptedStop(loop.ActionStopEndTurn))
		executor := newTestExecutor(t, store, binder, &fakeJudge{}, &fakeBudgetGuard{})

		if _, err := executor.recoverPendingPrompt(t.Context(), segment); err != nil {
			t.Fatalf("recoverPendingPrompt(queued compaction baseline) error = %v", err)
		}
		requests := binder.preparedRequests()
		if len(requests) != 1 || requests[0].ContextUsageSequence == nil ||
			*requests[0].ContextUsageSequence != sequence || requests[0].ContextUsageUsed == nil ||
			*requests[0].ContextUsageUsed != used {
			t.Fatalf("recovered compaction request = %#v", requests)
		}
	})

	t.Run("Should mark a compaction ambiguous when its canceled provider does not drain", func(t *testing.T) {
		t.Parallel()

		input := testGoalInput(t)
		key := TurnKey{
			WorkspaceID: input.WorkspaceID, LoopRunID: input.LoopRunID, Generation: input.Generation,
			NodeID: input.NodeID, ItemIndex: input.ItemIndex,
		}
		promptID, err := deterministicPromptID(key, 0, promptKindCompact, 0)
		if err != nil {
			t.Fatalf("deterministicPromptID() error = %v", err)
		}
		store := newFakeExecutorStore()
		binder := newFakeManagedBinder(store)
		binder.blockFirst[promptID] = true
		binder.cancelBlock = true
		executor, err := NewExecutor(Dependencies{
			Store: store, Binder: binder, Judge: &fakeJudge{}, Budget: &fakeBudgetGuard{},
			Context: &fakeContextHealth{
				usage: ContextUsage{
					Known: true, Used: 9, Size: 10, Sequence: 1, ReportedAt: time.Now().UTC(),
				},
				hasCompact: true,
			},
			Recovery:          &fakePromptRecovery{},
			CompactionTimeout: time.Millisecond,
			CompactionDrain:   time.Millisecond,
		})
		if err != nil {
			t.Fatalf("NewExecutor() error = %v", err)
		}

		raw, err := executor.Execute(t.Context(), testGoalNode(3), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionNeedsApproval ||
			raw.Control.Cause != loop.ReasonCodeGoalRecoveryAmbiguous {
			t.Fatalf("Control = %#v, want recovery ambiguity", raw.Control)
		}
		if binder.cancelCount() != 1 || store.ambiguousCount() != 1 {
			t.Fatalf("cancel/ambiguity counts = %d/%d, want 1/1", binder.cancelCount(), store.ambiguousCount())
		}
	})
}

func directSegmentForTest(t *testing.T, store *fakeExecutorStore) *segmentState {
	t.Helper()

	input := testGoalInput(t)
	node := testGoalNode(3)
	var params dsl.GoalParams
	if err := node.Params.Decode(&params); err != nil {
		t.Fatalf("Decode(Goal params) error = %v", err)
	}
	key := TurnKey{
		WorkspaceID: input.WorkspaceID,
		LoopRunID:   input.LoopRunID,
		Generation:  input.Generation,
		NodeID:      input.NodeID,
		ItemIndex:   input.ItemIndex,
	}
	checkpoint := Checkpoint{
		Key:               key,
		ControlEpoch:      1,
		Phase:             checkpointPhaseIdle,
		Status:            goalStatusActive,
		TurnLimit:         3,
		TaskRunID:         input.CorrelationID,
		SessionID:         "session-direct",
		BindingHandle:     "goal:direct",
		BindingEpoch:      1,
		ContextState:      contextStateUnknown,
		ContextNudgeRatio: 0.8,
	}
	store.installCheckpoint(checkpoint)
	return &segmentState{
		input:      input,
		key:        key,
		node:       node,
		params:     params,
		checkpoint: checkpoint,
		binding: loop.ActionSessionBinding{
			SessionID:    checkpoint.SessionID,
			Handle:       checkpoint.BindingHandle,
			ControlEpoch: checkpoint.ControlEpoch,
			BindingEpoch: checkpoint.BindingEpoch,
			State:        string(BindingStateActive),
			Ownership:    string(BindingOwnershipRunOwned),
		},
		usage: newUsageTracker(0, nil),
	}
}

func testGoalNode(maxTurns int) dsl.Node {
	return dsl.Node{
		ID:    "converge",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionGoal),
		Session: &dsl.SessionSpec{
			Mode: dsl.SessionModeContinuous,
		},
		Params: dsl.NodeParams{
			"agent":        "worker",
			"objective":    "Finish the durable objective",
			"judge":        []any{map[string]any{"id": "done", "type": "agent-judge", "rubric": "Approve when done"}},
			"max_turns":    maxTurns,
			"on_exhausted": dsl.GoalOnExhaustedHalt,
		},
	}
}

func testJudgeCriterion(criterionType dsl.CriterionType) map[string]any {
	criterion := map[string]any{
		"id":   "done",
		"type": criterionType,
	}
	switch criterionType {
	case dsl.CriterionCommand:
		criterion["check"] = "make verify"
	case dsl.CriterionAgentJudge:
		criterion["rubric"] = "Approve when done"
	case dsl.CriterionExtension:
		criterion["tool"] = "ext__quality__gate"
	}
	return criterion
}

func testGoalInput(t *testing.T) loop.ActionExecutionInput {
	t.Helper()
	actor, err := task.DeriveDaemonActorContext("loop-action", "test")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	return loop.ActionExecutionInput{
		WorkspaceID:           "workspace-1",
		LoopRunID:             "loop-1",
		Generation:            1,
		NodeID:                "converge",
		ItemIndex:             0,
		Actor:                 actor,
		CorrelationID:         "task-run-1",
		WorkerModel:           "worker-model",
		JudgeModel:            "judge-model",
		GoalContextNudgeRatio: new(0.8),
		GoalSegmentEpoch:      1,
	}
}

func scriptedEndTurn(text string, tokens int64, callbacks ...int64) scriptedPrompt {
	return scriptedPrompt{
		result: loop.ActionPromptResult{
			Outcome:        loop.ActionPromptOutcomeCompleted,
			Text:           text,
			TokensUsed:     tokens,
			TokensReported: true,
			StopReason:     loop.ActionStopEndTurn,
		},
		usageCallbacks: callbacks,
	}
}

func scriptedStop(reason loop.ActionStopReason) scriptedPrompt {
	return scriptedPrompt{result: loop.ActionPromptResult{
		Outcome:    loop.ActionPromptOutcomeCompleted,
		StopReason: reason,
	}}
}

func judgeResult(outcome gate.VerdictOutcome, tokens int64) JudgeResult {
	result := JudgeResult{Verdict: gate.Verdict{Outcome: outcome}}
	if outcome != gate.VerdictOutcomeApproved {
		result.Verdict.BlockingIssues = []gate.BlockingIssue{{ID: string(outcome), Note: "not converged"}}
	}
	if outcome == gate.VerdictOutcomeError || outcome == gate.VerdictOutcomeTimeout ||
		outcome == gate.VerdictOutcomeInvalidOutput {
		result.Verdict.Broken = true
	}
	if tokens >= 0 {
		result.TokensUsed = tokens
		result.TokensReported = tokens > 0
	}
	return result
}

func promptKinds(requests []loop.ActionPromptRequest) []string {
	kinds := make([]string, len(requests))
	for index, request := range requests {
		kinds[index] = request.Kind
	}
	return kinds
}
