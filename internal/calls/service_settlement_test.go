package calls

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/store"
)

func TestServiceReturnSettlementPipeline(t *testing.T) {
	t.Parallel()

	t.Run("Should settle a valid typed return and preserve the first result", func(t *testing.T) {
		t.Parallel()
		service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, ChildSessionID: record.ChildSessionID,
			Result: json.RawMessage(`{"answer":42}`), ChildLive: true,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return() error = %v", err)
		}
		if settlement.Call.State != StateCompleted || settlement.Call.Verdict != VerdictReturned ||
			settlement.Call.ResultRef == "" || string(database.payloads[settlement.Call.ResultRef]) != `{"answer":42}` {
			t.Fatalf("Return() = %#v", settlement)
		}
		firstRef := settlement.Call.ResultRef
		_, err = service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`{"answer":99}`),
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if !IsCode(err, CodeAlreadySettled) {
			t.Fatalf("Return(second) error = %v, want %s", err, CodeAlreadySettled)
		}
		stored := database.calls[record.CallID]
		if stored.ResultRef != firstRef || stored.SupersededRef == "" {
			t.Fatalf("stored result refs = result %q superseded %q", stored.ResultRef, stored.SupersededRef)
		}
		readQuery := CallReadQuery{
			ReadScope: store.ReadScope{ProfileID: record.ProfileID},
			Scope:     record.Scope, WorkspaceID: record.WorkspaceID,
		}
		prompt, err := service.Prompt(context.Background(), readQuery, record.CallID)
		if err != nil || prompt.Text != "work" {
			t.Fatalf("Prompt() = %#v, %v", prompt, err)
		}
		superseded, err := service.Superseded(context.Background(), readQuery, record.CallID)
		if err != nil || string(superseded.Bytes) != `{"answer":99}` {
			t.Fatalf("Superseded() = %s, %v", superseded.Bytes, err)
		}
	})

	t.Run("Should reject an unbound or unauthorized return", func(t *testing.T) {
		t.Parallel()
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		_, err := service.Return(context.Background(), ReturnInput{
			Result: json.RawMessage(`{"answer":1}`),
			Actor:  SettlementActor{Kind: "agent_session", ID: "missing-child"},
		})
		if !IsCode(err, CodeReturnUnbound) {
			t.Fatalf("Return(unbound) error = %v, want %s", err, CodeReturnUnbound)
		}
		record := createContractedCall(t, service)
		_, err = service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`{"answer":1}`),
			Actor: SettlementActor{Kind: "agent_session", ID: "other-child"},
		})
		if !IsCode(err, CodeSettlementDenied) {
			t.Fatalf("Return(other child) error = %v, want %s", err, CodeSettlementDenied)
		}
	})

	t.Run("Should deliver exactly one repair and terminalize a second invalid result", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		first, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`{"wrong":true}`), ChildLive: true,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(first invalid) error = %v", err)
		}
		if first.Call.State != StateRunning || first.Call.RepairAttempts != 1 || first.RepairPrompt == "" ||
			len(database.repairDeliveries) != 1 || len(invoker.deliveries) != 0 {
			t.Fatalf("first invalid settlement = %#v durable=%#v immediate=%#v",
				first, database.repairDeliveries, invoker.deliveries)
		}
		second, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`{"still":"wrong"}`), ChildLive: true,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(second invalid) error = %v", err)
		}
		if second.Call.State != StateInvalidResult || len(database.repairDeliveries) != 1 ||
			database.calls[record.CallID].FailureCode != string(CodeResultInvalid) ||
			database.calls[record.CallID].SecondIssueText == "" {
			t.Fatalf("second invalid settlement = %#v deliveries=%d", second, len(database.repairDeliveries))
		}
	})

	t.Run("Should commit the repair before consulting delivery infrastructure", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		invoker.deliverErr = errors.New("delivery unavailable")
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`{"wrong":true}`), ChildLive: true,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil || settlement.Call.RepairAttempts != 1 || settlement.Call.State != StateRunning ||
			len(database.repairDeliveries) != 1 || len(invoker.deliveries) != 0 {
			t.Fatalf("Return(durable repair) = %#v, %v durable=%#v immediate=%#v",
				settlement, err, database.repairDeliveries, invoker.deliveries)
		}
	})
}

func TestServiceReturnExtractionStrictAndBudget(t *testing.T) {
	t.Parallel()

	t.Run("Should extract a valid candidate from final prose", func(t *testing.T) {
		t.Parallel()
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, FinalText: "Done.\n```json\n{\"answer\":7}\n```", ChildLive: false,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(extracted) error = %v", err)
		}
		if settlement.Call.State != StateCompleted || settlement.Call.Verdict != VerdictExtracted {
			t.Fatalf("Return(extracted) = %#v", settlement)
		}
	})

	t.Run("Should use an older valid candidate before repairing newer invalid prose", func(t *testing.T) {
		t.Parallel()
		service, _, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, FinalText: "{\"answer\":7}\nthen\n{\"wrong\":true}", ChildLive: true,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(extraction fallback) error = %v", err)
		}
		if settlement.Call.State != StateCompleted || settlement.Call.Verdict != VerdictExtracted ||
			len(invoker.deliveries) != 0 {
			t.Fatalf("Return(extraction fallback) = %#v deliveries=%#v", settlement, invoker.deliveries)
		}
	})

	t.Run("Should reject an invalid extracted candidate when the child is gone", func(t *testing.T) {
		t.Parallel()
		service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, FinalText: "Done. {\"wrong\":true}", ChildLive: false,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(invalid extraction) error = %v", err)
		}
		if settlement.Call.State != StateInvalidResult ||
			database.calls[record.CallID].FailureCode != string(CodeResultInvalid) {
			t.Fatalf("Return(invalid extraction) = %#v", settlement)
		}
	})

	t.Run("Should complete without result when strict or prose has no candidate", func(t *testing.T) {
		t.Parallel()
		for _, strict := range []bool{false, true} {
			service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
			input := validCreateInput("work", json.RawMessage(`{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}}}`), nil)
			input.Strict = strict
			record, err := service.Create(context.Background(), input)
			if err != nil {
				t.Fatalf("Create(strict=%t) error = %v", strict, err)
			}
			text := "No structured result is available."
			if strict {
				text = `{"answer":7}`
			}
			settlement, err := service.Return(context.Background(), ReturnInput{
				CallID: record.CallID, FinalText: text,
				Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
			})
			if err != nil {
				t.Fatalf("Return(strict=%t) error = %v", strict, err)
			}
			if settlement.Call.State != StateCompletedWithoutResult {
				t.Fatalf("Return(strict=%t) state = %s", strict, settlement.Call.State)
			}
		}
	})

	t.Run("Should apply the immutable per-call reject budget", func(t *testing.T) {
		t.Parallel()
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		input := validCreateInput("work", nil, nil)
		input.ResultBudget = &contracts.ByteBudget{MaxBytes: 4, Overflow: contracts.OverflowReject}
		record, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`"too long"`),
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(over budget) error = %v", err)
		}
		if settlement.Call.State != StateFailed || settlement.Call.FailureCode != string(CodeResultOverBudget) {
			t.Fatalf("Return(over budget) = %#v", settlement)
		}
	})

	t.Run("Should store the whole extracted uncontracted result under store overflow", func(t *testing.T) {
		t.Parallel()
		service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		input := validCreateInput("work", nil, nil)
		input.ResultBudget = &contracts.ByteBudget{MaxBytes: 4, Overflow: contracts.OverflowStore}
		record, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		settlement, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, FinalText: `Done: {"answer":"long value"}`,
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(store overflow) error = %v", err)
		}
		stored := database.payloads[settlement.Call.ResultRef]
		if settlement.Call.State != StateCompleted || settlement.Call.Verdict != VerdictExtracted ||
			len(stored) <= input.ResultBudget.MaxBytes || string(stored) != `{"answer":"long value"}` {
			t.Fatalf("Return(store overflow) = %#v payload=%q", settlement, stored)
		}
	})
}

func TestServiceCancelAwaitDeadlineAndDrain(t *testing.T) {
	t.Parallel()

	t.Run("Should stop a running child and make repeated cancel idempotent", func(t *testing.T) {
		t.Parallel()
		service, _, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record, err := service.Create(context.Background(), validCreateInput("work", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		canceled, err := service.Cancel(context.Background(), record.CallID, "operator stopped it", record.Actor)
		if err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}
		if canceled.State != StateCanceled || len(invoker.stops) != 1 || invoker.stops[0] != record.ChildSessionID {
			t.Fatalf("Cancel() = %#v stops=%#v", canceled, invoker.stops)
		}
		repeated, err := service.Cancel(context.Background(), record.CallID, "again", record.Actor)
		if err != nil || repeated.State != StateCanceled || len(invoker.stops) != 1 {
			t.Fatalf("Cancel(repeated) = %#v error=%v stops=%d", repeated, err, len(invoker.stops))
		}
	})

	t.Run("Should cancel a queued activation without starting a child", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		service.claimer = nil
		service.invoker = nil
		record, err := service.Create(context.Background(), validCreateInput("queued", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if record.State != StateQueued {
			t.Fatalf("Create() state = %s, want queued", record.State)
		}
		canceled, err := service.Cancel(context.Background(), record.CallID, "batch item removed", record.Actor)
		if err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}
		if canceled.State != StateCanceled || len(invoker.spawns) != 0 || database.calls[record.CallID].State != StateCanceled {
			t.Fatalf("Cancel(queued) = %#v spawns=%d", canceled, len(invoker.spawns))
		}
	})

	t.Run("Should return settled and pending calls with a resume token and clamp", func(t *testing.T) {
		t.Parallel()
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		first, err := service.Create(context.Background(), validCreateInput("first", nil, nil))
		if err != nil {
			t.Fatalf("Create(first) error = %v", err)
		}
		secondInput := validCreateInput("second", nil, nil)
		secondInput.IdempotencyKey = "second"
		second, err := service.Create(context.Background(), secondInput)
		if err != nil {
			t.Fatalf("Create(second) error = %v", err)
		}
		if _, err := service.Cancel(context.Background(), first.CallID, "done", first.Actor); err != nil {
			t.Fatalf("Cancel(first) error = %v", err)
		}
		outcome, err := service.Await(context.Background(), AwaitInput{
			ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
			CallIDs: []string{first.CallID, second.CallID}, Timeout: MaxAwaitDuration + time.Hour,
		})
		if err != nil {
			t.Fatalf("Await() error = %v", err)
		}
		if outcome.Outcome != "partial" || len(outcome.Settled) != 1 || len(outcome.Pending) != 1 ||
			outcome.Resume == "" || outcome.ClampedTimeout != MaxAwaitDuration {
			t.Fatalf("Await() = %#v", outcome)
		}
		_, err = service.Await(context.Background(), AwaitInput{
			ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1", CallIDs: []string{"missing"},
		})
		if !IsCode(err, CodeNotFound) {
			t.Fatalf("Await(missing) error = %v, want %s", err, CodeNotFound)
		}
	})

	t.Run("Should cap concurrent awaiters for one durable call", func(t *testing.T) {
		t.Parallel()
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record, err := service.Create(context.Background(), validCreateInput("await cap", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		results := make(chan error, MaxConcurrentAwait)
		for range MaxConcurrentAwait {
			go func() {
				_, awaitErr := service.Await(ctx, AwaitInput{
					ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
					CallIDs: []string{record.CallID}, Timeout: time.Minute,
				})
				results <- awaitErr
			}()
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			service.waitMu.Lock()
			registered := len(service.waiters[record.CallID])
			service.waitMu.Unlock()
			if registered == MaxConcurrentAwait {
				break
			}
			if time.Now().After(deadline) {
				cancel()
				t.Fatalf("registered awaiters = %d, want %d", registered, MaxConcurrentAwait)
			}
			time.Sleep(time.Millisecond)
		}
		_, err = service.Await(context.Background(), AwaitInput{
			ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
			CallIDs: []string{record.CallID}, Timeout: time.Second,
		})
		if !IsCode(err, CodeValidation) {
			cancel()
			t.Fatalf("Await(over cap) error = %v, want %s", err, CodeValidation)
		}
		cancel()
		for range MaxConcurrentAwait {
			if awaitErr := <-results; awaitErr != context.Canceled {
				t.Fatalf("Await(canceled) error = %v, want %v", awaitErr, context.Canceled)
			}
		}
	})

	t.Run("Should observe a settlement after waiter registration without losing the edge", func(t *testing.T) {
		t.Parallel()
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record, err := service.Create(context.Background(), validCreateInput("await edge", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		result := make(chan AwaitOutcome, 1)
		errorsCh := make(chan error, 1)
		go func() {
			outcome, awaitErr := service.Await(context.Background(), AwaitInput{
				ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
				CallIDs: []string{record.CallID}, Timeout: time.Minute,
			})
			result <- outcome
			errorsCh <- awaitErr
		}()
		deadline := time.Now().Add(5 * time.Second)
		for {
			service.waitMu.Lock()
			registered := len(service.waiters[record.CallID])
			service.waitMu.Unlock()
			if registered == 1 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("await waiter was not registered")
			}
			time.Sleep(time.Millisecond)
		}
		if _, err := service.Return(context.Background(), ReturnInput{
			CallID: record.CallID, Result: json.RawMessage(`{"done":true}`),
			Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		}); err != nil {
			t.Fatalf("Return() error = %v", err)
		}
		if awaitErr := <-errorsCh; awaitErr != nil {
			t.Fatalf("Await() error = %v", awaitErr)
		}
		outcome := <-result
		if outcome.Outcome != "complete" || len(outcome.Settled) != 1 || len(outcome.Pending) != 0 {
			t.Fatalf("Await() = %#v", outcome)
		}
	})

	t.Run("Should fence deadlines and stop running children", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record, err := service.Create(context.Background(), validCreateInput("work", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		database.due = []CallRecord{record}
		report, err := service.SweepDeadlines(context.Background(), time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("SweepDeadlines() error = %v", err)
		}
		if len(report.TimedOut) != 1 || database.calls[record.CallID].State != StateTimeout || len(invoker.stops) != 1 {
			t.Fatalf("SweepDeadlines() = %#v state=%s stops=%#v", report, database.calls[record.CallID].State, invoker.stops)
		}
	})

	t.Run("Should stop each subtree child once and close every open call", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		first, err := service.Create(context.Background(), validCreateInput("one", nil, nil))
		if err != nil {
			t.Fatalf("Create(first) error = %v", err)
		}
		second := first
		second.CallID = "call-second"
		second.ActivationRunID = "run-second"
		database.calls[second.CallID] = second
		database.subtree = []CallRecord{first, second}
		database.preservedResults = 1
		report, err := service.DrainSubtree(context.Background(), "root-1", Actor{Kind: "daemon", ID: "recovery"}, "parent terminal")
		if err != nil {
			t.Fatalf("DrainSubtree() error = %v", err)
		}
		if len(report.CanceledCalls) != 2 || len(report.Stopped) != 1 || report.PreservedResults != 1 || len(invoker.stops) != 1 {
			t.Fatalf("DrainSubtree() = %#v stops=%#v", report, invoker.stops)
		}
	})
}

func TestServiceReturnRacingDeadlinePreservesLateEvidence(t *testing.T) {
	t.Parallel()

	for iteration := 0; iteration < 50; iteration++ {
		service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		record := createContractedCall(t, service)
		database.due = []CallRecord{record}
		start := make(chan struct{})
		returnResult := make(chan error, 1)
		sweepResult := make(chan error, 1)
		go func() {
			<-start
			_, err := service.Return(context.Background(), ReturnInput{
				CallID: record.CallID, Result: json.RawMessage(`{"answer":42}`),
				Actor: SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
			})
			returnResult <- err
		}()
		go func() {
			<-start
			_, err := service.SweepDeadlines(context.Background(), time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC))
			sweepResult <- err
		}()
		close(start)
		returnErr, sweepErr := <-returnResult, <-sweepResult
		if sweepErr != nil {
			t.Fatalf("iteration %d SweepDeadlines() error = %v", iteration, sweepErr)
		}
		stored := database.calls[record.CallID]
		switch stored.State {
		case StateCompleted:
			if returnErr != nil || stored.ResultRef == "" {
				t.Fatalf("iteration %d completed winner: return error=%v record=%#v", iteration, returnErr, stored)
			}
		case StateTimeout:
			if !IsCode(returnErr, CodeAlreadySettled) || stored.SupersededRef == "" {
				t.Fatalf("iteration %d timeout winner: return error=%v record=%#v", iteration, returnErr, stored)
			}
		default:
			t.Fatalf("iteration %d terminal state = %s", iteration, stored.State)
		}
	}
}

func createContractedCall(t *testing.T, service *Service) CallRecord {
	t.Helper()
	expect := json.RawMessage(`{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}},"additionalProperties":false}`)
	record, err := service.Create(context.Background(), validCreateInput("work", expect, nil))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return record
}
