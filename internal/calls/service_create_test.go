package calls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/task"
)

func TestServiceCreateAdmissionAndActivation(t *testing.T) {
	t.Parallel()

	t.Run("Should persist byte-exact contracted prompt before dispatcher activation", func(t *testing.T) {
		t.Parallel()
		service, database, claimer, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		prompt := strings.Repeat("review carefully; ", 4096) + "  "
		expect := json.RawMessage(
			`{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}},"additionalProperties":false}`,
		)
		runtime := RuntimeSpec{Provider: "anthropic", Model: "opus", ReasoningEffort: "high", Speed: speed.SpeedFast}
		record, err := service.Create(context.Background(), validCreateInput(prompt, expect, &runtime))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if record.State != StateQueued || record.ChildSessionID != "" || record.ExpectDigest == "" {
			t.Fatalf("Create() = %#v, want durable queued admission", record)
		}
		if got := string(database.payloads[callPayloadKey(record.WorkspaceID, record.PromptRef)]); got != prompt {
			suffix := got
			if len(suffix) > 8 {
				suffix = suffix[len(suffix)-8:]
			}
			t.Fatalf(
				"prompt changed during admission: got length %d and %q suffix, want byte-exact payload",
				len(got),
				suffix,
			)
		}
		if record.IdleTTL != time.Hour || record.Runtime != runtime {
			t.Fatalf("record snapshots = ttl %s runtime %#v", record.IdleTTL, record.Runtime)
		}
		if len(claimer.criteria) != 0 || len(invoker.spawns) != 0 {
			t.Fatalf("Create() claims=%#v spawns=%#v, want admission only", claimer.criteria, invoker.spawns)
		}
		dispatched, err := service.DispatchQueued(t.Context(), 1)
		if err != nil {
			t.Fatalf("DispatchQueued() error = %v", err)
		}
		activated := database.calls[record.CallID]
		if dispatched != 1 || activated.State != StateRunning || activated.ChildSessionID == "" {
			t.Fatalf("DispatchQueued() = %d, call = %#v, want one running child", dispatched, activated)
		}
		if len(claimer.criteria) != 1 || claimer.criteria[0].RunID != record.ActivationRunID ||
			claimer.criteria[0].RunKind != task.RunKindCallActivation {
			t.Fatalf("claim criteria = %#v, want exact call activation", claimer.criteria)
		}
		if len(invoker.spawns) != 1 || len(invoker.spawns[0].Permissions.Skills) != 1 ||
			invoker.spawns[0].Permissions.Skills[0] != "review" {
			t.Fatalf("spawn specs = %#v, want narrowed skills", invoker.spawns)
		}
	})

	t.Run("Should distinguish cancellation before admission from cancellation after commit", func(t *testing.T) {
		t.Parallel()

		t.Run("Should reject cancellation before durable admission", func(t *testing.T) {
			t.Parallel()
			service, database, _, _ := newCallServiceHarness(
				t,
				config.DefaultCallsConfig(),
				validAgentTarget(),
			)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			_, err := service.Create(ctx, validCreateInput("canceled before commit", nil, nil))

			if !errors.Is(err, context.Canceled) || len(database.calls) != 0 {
				t.Fatalf("Create(pre-admission cancel) error/calls = %v/%d", err, len(database.calls))
			}
		})

		t.Run("Should return the durable identity when cancellation follows commit", func(t *testing.T) {
			t.Parallel()
			service, database, _, _ := newCallServiceHarness(
				t,
				config.DefaultCallsConfig(),
				validAgentTarget(),
			)
			ctx, cancel := context.WithCancel(t.Context())
			database.afterAdmit = cancel

			record, err := service.Create(ctx, validCreateInput("canceled after commit", nil, nil))

			if err != nil || ctx.Err() == nil || record.State != StateQueued ||
				database.calls[record.CallID].CallID != record.CallID {
				t.Fatalf("Create(post-admission cancel) record/error = %#v/%v", record, err)
			}
		})
	})

	t.Run("Should default helper tools and inherit omitted non-tool categories", func(t *testing.T) {
		t.Parallel()

		target := validAgentTarget()
		target.CallerPolicy = PermissionPolicy{
			Tools: append(
				append(boundChildBaseTools(), boundChildDelegationTools()...),
				"compozy__session_stop",
			),
			Skills:          []string{"code", "review"},
			WorkspacePaths:  []string{"internal/calls"},
			NetworkChannels: []string{"runtime"},
		}
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), target)
		input := validCreateInput("inherit permissions", nil, nil)
		input.Narrow = PermissionAtoms{}

		record, err := service.Create(t.Context(), input)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		record = activateCreatedCall(t, service, &record)
		if record.State != StateRunning || len(invoker.spawns) != 1 {
			t.Fatalf("activated call = %#v spawns=%d, want one running child", record, len(invoker.spawns))
		}
		want := normalizePermissionPolicy(target.CallerPolicy)
		want.Tools = normalizePermissionAtoms(append(boundChildBaseTools(), boundChildDelegationTools()...))
		got := invoker.spawns[0].Permissions.Policy()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("spawn permission policy = %#v, want inherited %#v", got, want)
		}
		if len(database.admissions) != 1 || !reflect.DeepEqual(database.admissions[0].Narrow.Policy(), want) {
			t.Fatalf("admission narrowing = %#v, want inherited %#v", database.admissions, want)
		}
	})

	t.Run("Should remove further delegation tools at the depth wall", func(t *testing.T) {
		t.Parallel()

		target := validAgentTarget()
		target.Depth = config.DefaultCallsConfig().MaxDepth
		service, _, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), target)

		record, err := service.Create(t.Context(), validCreateInput("depth wall", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		record = activateCreatedCall(t, service, &record)
		wantTools := normalizePermissionAtoms(boundChildBaseTools())
		if len(invoker.spawns) != 1 || !reflect.DeepEqual(invoker.spawns[0].Permissions.Tools, wantTools) {
			t.Fatalf("spawn tools at depth wall = %#v, want %v", invoker.spawns, wantTools)
		}
	})

	t.Run("Should keep uncontracted calls and distinguish keyless duplicates", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		input := validCreateInput("uncontracted", nil, nil)
		input.IdempotencyKey = ""
		first, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(first) error = %v", err)
		}
		second, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(second) error = %v", err)
		}
		if first.CallID == second.CallID || first.ExpectDigest != "" || second.ExpectDigest != "" {
			t.Fatalf("keyless records = %#v / %#v", first, second)
		}
		if len(database.calls) != 2 || len(invoker.spawns) != 0 {
			t.Fatalf(
				"side effects = %d calls, %d spawns; want admission only",
				len(database.calls),
				len(invoker.spawns),
			)
		}
	})

	t.Run("Should replay identical idempotency identity and reject changed payload", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		input := validCreateInput("stable", nil, nil)
		first, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(first) error = %v", err)
		}
		first = activateCreatedCall(t, service, &first)
		replay, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(replay) error = %v", err)
		}
		if replay.CallID != first.CallID || !replay.Replayed || len(invoker.spawns) != 1 || len(database.calls) != 1 {
			t.Fatalf("replay = %#v, spawns=%d calls=%d", replay, len(invoker.spawns), len(database.calls))
		}
		settled, err := service.Return(context.Background(), ReturnInput{
			Scope: first.OwnerScope(), CallID: first.CallID, Result: json.RawMessage(`{"done":true}`),
			Actor: SettlementActor{Kind: "agent_session", ID: first.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return() error = %v", err)
		}
		terminalReplay, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(terminal replay) error = %v", err)
		}
		if !terminalReplay.Replayed || terminalReplay.State != StateCompleted ||
			terminalReplay.ResultRef != settled.Call.ResultRef || len(invoker.spawns) != 1 {
			t.Fatalf("terminal replay = %#v, settlement=%#v spawns=%d", terminalReplay, settled, len(invoker.spawns))
		}
		input.Prompt = "changed"
		_, err = service.Create(context.Background(), input)
		if !IsCode(err, CodeIdempotencyConflict) {
			t.Fatalf("Create(conflict) error = %v, want %s", err, CodeIdempotencyConflict)
		}
	})

	t.Run("Should terminalize a failed activation with its typed cause", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		invoker.spawnErr = newError(CodeWideningRejected, "hook widened permissions", nil)
		record, err := service.Create(context.Background(), validCreateInput("work", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if _, err := service.DispatchQueued(t.Context(), 1); err != nil {
			t.Fatalf("DispatchQueued() error = %v", err)
		}
		record = database.calls[record.CallID]
		if record.State != StateFailed || record.FailureCode != string(CodeWideningRejected) ||
			database.calls[record.CallID].State != StateFailed {
			t.Fatalf("failed activation = %#v", record)
		}
	})

	t.Run("Should keep an admitted call queued when the exact activation claim races", func(t *testing.T) {
		t.Parallel()
		service, database, claimer, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		claimer.claimErr = errors.Join(task.ErrNoClaimableRun, errors.New("claim raced"))
		record, err := service.Create(context.Background(), validCreateInput("work", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		dispatched, err := service.DispatchQueued(t.Context(), 1)
		if err != nil || dispatched != 0 {
			t.Fatalf("DispatchQueued() = %d, %v, want queued race", dispatched, err)
		}
		if record.State != StateQueued || database.calls[record.CallID].State != StateQueued ||
			len(invoker.spawns) != 0 {
			t.Fatalf(
				"Create() = %#v stored=%#v spawns=%d, want one durable queued call",
				record,
				database.calls[record.CallID],
				len(invoker.spawns),
			)
		}
	})

	// Invariant: one raced activation cannot starve a later claimable call.
	t.Run("Should continue past an activation whose exact claim raced", func(t *testing.T) {
		t.Parallel()
		service, database, claimer, invoker := newCallServiceHarness(
			t,
			config.DefaultCallsConfig(),
			validAgentTarget(),
		)
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
		runIDs := []string{first.ActivationRunID, second.ActivationRunID}
		slices.Sort(runIDs)
		claimer.claimErrors = map[string]error{
			runIDs[0]: errors.Join(task.ErrNoClaimableRun, errors.New("claim raced")),
		}
		dispatched, err := service.DispatchQueued(t.Context(), 2)
		if err != nil || dispatched != 1 {
			t.Fatalf("DispatchQueued() = %d, %v, want one later activation", dispatched, err)
		}
		if database.calls[first.CallID].State == database.calls[second.CallID].State || len(invoker.spawns) != 1 {
			t.Fatalf(
				"call states = %s/%s spawns=%d, want one queued and one running",
				database.calls[first.CallID].State,
				database.calls[second.CallID].State,
				len(invoker.spawns),
			)
		}
	})

	t.Run("Should persist the target depth for each admission", func(t *testing.T) {
		t.Parallel()
		firstTarget := validAgentTarget()
		firstTarget.Depth = 3
		service, _, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), firstTarget)
		first, err := service.Create(context.Background(), validCreateInput("depth three", nil, nil))
		if err != nil {
			t.Fatalf("Create(depth three) error = %v", err)
		}
		raised := config.DefaultCallsConfig()
		raised.MaxDepth = 4
		secondTarget := validAgentTarget()
		secondTarget.Depth = 4
		secondService, _, _, _ := newCallServiceHarness(t, raised, secondTarget)
		secondInput := validCreateInput("depth four", nil, nil)
		secondInput.IdempotencyKey = "depth-four"
		second, err := secondService.Create(context.Background(), secondInput)
		if err != nil {
			t.Fatalf("Create(depth four) error = %v", err)
		}
		if first.Depth != 3 || second.Depth != 4 {
			t.Fatalf("durable depths = first %d second %d", first.Depth, second.Depth)
		}
	})

	t.Run("Should validate each call against the current narrowed caller set", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		current := PermissionPolicy{
			Tools: append(boundChildBaseTools(), boundChildDelegationTools()...), Skills: []string{"review"},
		}
		directory := routedCallDirectory(func(
			_ context.Context,
			_ CreateInput,
		) (TargetContext, []AgentRosterEntry, error) {
			mu.Lock()
			defer mu.Unlock()
			target := validAgentTarget()
			target.CallerPolicy = current
			return target, []AgentRosterEntry{{Name: "reviewer"}}, nil
		})
		service, database, _, _ := newCallServiceForDirectory(t, config.DefaultCallsConfig(), directory)
		first, err := service.Create(context.Background(), validCreateInput("first", nil, nil))
		if err != nil {
			t.Fatalf("Create(first) error = %v", err)
		}
		mu.Lock()
		current = PermissionPolicy{
			Tools: append(boundChildBaseTools(), boundChildDelegationTools()...), Skills: []string{"code"},
		}
		mu.Unlock()
		secondInput := validCreateInput("second", nil, nil)
		secondInput.IdempotencyKey = "second"
		_, err = service.Create(context.Background(), secondInput)
		if !IsCode(err, CodeWideningRejected) {
			t.Fatalf("Create(second) error = %v, want %s", err, CodeWideningRejected)
		}
		if database.calls[first.CallID].State != StateQueued || len(database.calls) != 1 {
			t.Fatalf("first call changed after caller policy update: %#v", database.calls[first.CallID])
		}
	})
}

func TestServiceCreateRejectsBeforeAdmission(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		input         CreateInput
		target        TargetContext
		roster        []AgentRosterEntry
		code          ErrorCode
		wantAvailable []AgentRosterEntry
		wantWidening  []string
	}{
		{
			name:   "Should reject a blank prompt",
			input:  validCreateInput(" \n\t ", nil, nil),
			target: validAgentTarget(),
			code:   CodePromptRequired,
		},
		{
			name:          "Should return the current roster for an unknown agent",
			input:         validCreateInput("work", nil, nil),
			target:        TargetContext{ProfileID: "default", WorkspaceID: "ws-1", Allowed: true},
			roster:        []AgentRosterEntry{{Name: "coder", Description: "Writes code"}},
			code:          CodeAgentUnknown,
			wantAvailable: []AgentRosterEntry{{Name: "coder", Description: "Writes code"}},
		},
		{
			name:   "Should reject the live child wall",
			input:  validCreateInput("work", nil, nil),
			target: withTarget(validAgentTarget(), func(value *TargetContext) { value.LiveChildren = 5 }),
			code:   CodeChildrenCap,
		},
		{
			name: "Should reject widening atoms exactly",
			input: withInput(validCreateInput("work", nil, nil), func(value *CreateInput) {
				value.Narrow.Skills = []string{"admin"}
				value.Narrow.Tools = []string{"shell"}
			}),
			target:       validAgentTarget(),
			code:         CodeWideningRejected,
			wantWidening: []string{"skills:admin", "tools:shell"},
		},
		{
			name:   "Should reject a cross workspace target",
			input:  validCreateInput("work", nil, nil),
			target: withTarget(validAgentTarget(), func(value *TargetContext) { value.WorkspaceID = "ws-2" }),
			code:   CodeWorkspaceDenied,
		},
		{
			name:   "Should reject a target outside lineage",
			input:  validCreateInput("work", nil, nil),
			target: withTarget(validAgentTarget(), func(value *TargetContext) { value.Allowed = false }),
			code:   CodeTargetDenied,
		},
		{
			name:   "Should reject a depth beyond the durable wall",
			input:  validCreateInput("work", nil, nil),
			target: withTarget(validAgentTarget(), func(value *TargetContext) { value.Depth = 4 }),
			code:   CodeDepthExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service, database, _, invoker := newCallServiceHarnessWithRoster(
				t,
				config.DefaultCallsConfig(),
				tc.target,
				tc.roster,
			)
			_, err := service.Create(context.Background(), tc.input)
			if !IsCode(err, tc.code) {
				t.Fatalf("Create() error = %v, want %s", err, tc.code)
			}
			var callErr *Error
			if !errors.As(err, &callErr) {
				t.Fatalf("Create() error type = %T, want *calls.Error", err)
			}
			if !reflect.DeepEqual(callErr.Available, tc.wantAvailable) ||
				!reflect.DeepEqual(callErr.Widening, tc.wantWidening) {
				t.Fatalf("Create() structured error = %#v", callErr)
			}
			if len(database.calls) != 0 || len(invoker.spawns) != 0 {
				t.Fatalf(
					"rejected create wrote %d calls and spawned %d children",
					len(database.calls),
					len(invoker.spawns),
				)
			}
		})
	}

	t.Run("Should reject an invalid contract before spawn", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		_, err := service.Create(context.Background(), validCreateInput("work", json.RawMessage(`{"type":`), nil))
		if !IsCode(err, CodeExpectInvalid) {
			t.Fatalf("Create() error = %v, want %s", err, CodeExpectInvalid)
		}
		if len(database.calls) != 0 || len(invoker.spawns) != 0 {
			t.Fatalf(
				"invalid contract wrote %d calls and spawned %d children",
				len(database.calls),
				len(invoker.spawns),
			)
		}
	})

	t.Run("Should enforce exact global ownership tuples", func(t *testing.T) {
		t.Parallel()
		globalTarget := validAgentTarget()
		globalInput := validCreateInput("global work", nil, nil)
		globalInput.Scope = ScopeGlobal
		globalInput.WorkspaceID = ""
		service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), globalTarget)
		globalRecord, err := service.Create(context.Background(), globalInput)
		if err != nil {
			t.Fatalf("Create(global) error = %v", err)
		}
		if globalRecord.Scope != ScopeGlobal || globalRecord.WorkspaceID != "" {
			t.Fatalf("Create(global) = %#v, want empty call workspace owner", globalRecord)
		}

		invalidGlobal := globalInput
		invalidGlobal.IdempotencyKey = "invalid-global-owner"
		invalidGlobal.WorkspaceID = "ws-1"
		if _, err := service.Create(context.Background(), invalidGlobal); !IsCode(err, CodeValidation) {
			t.Fatalf("Create(global with call workspace) error = %v, want %s", err, CodeValidation)
		}
		if len(database.calls) != 1 {
			t.Fatalf("global ownership rejections changed admitted calls: %d", len(database.calls))
		}
	})
}

func TestServiceCreateBatchAndSessionTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should admit parallel prompts without synchronous activation", func(t *testing.T) {
		t.Parallel()
		service, database, claimer, invoker := newCallServiceHarness(
			t,
			config.DefaultCallsConfig(),
			validAgentTarget(),
		)
		const total = 8
		start := make(chan struct{})
		outcomes := make(chan CallRecord, total)
		errorsCh := make(chan error, total)
		var group sync.WaitGroup
		for index := range total {
			group.Go(func() {
				<-start
				input := validCreateInput("parallel prompt "+fmt.Sprint(index), nil, nil)
				input.IdempotencyKey = "parallel-" + fmt.Sprint(index)
				record, err := service.Create(t.Context(), input)
				outcomes <- record
				errorsCh <- err
			})
		}
		close(start)
		group.Wait()
		close(outcomes)
		close(errorsCh)

		for err := range errorsCh {
			if err != nil {
				t.Fatalf("Create(parallel) error = %v", err)
			}
		}
		for record := range outcomes {
			if record.State != StateQueued || record.CallID == "" || record.ActivationRunID == "" {
				t.Fatalf("Create(parallel) = %#v, want durable queued identity", record)
			}
		}
		if len(database.calls) != total || len(claimer.criteria) != 0 || len(invoker.spawns) != 0 {
			t.Fatalf(
				"parallel admission = calls %d claims %d spawns %d, want %d/0/0",
				len(database.calls),
				len(claimer.criteria),
				len(invoker.spawns),
				total,
			)
		}
	})

	t.Run("Should preserve ordered isolated batch outcomes", func(t *testing.T) {
		t.Parallel()
		cfg := config.DefaultCallsConfig()
		service, database, _, _ := newCallServiceHarness(t, cfg, validAgentTarget())
		inputs := []CreateInput{
			validCreateInput("one", json.RawMessage(`{"type":"object","properties":{"one":{"type":"integer"}}}`), nil),
			validCreateInput("two", json.RawMessage(`{"type":"object","properties":{"two":{"type":"string"}}}`), nil),
			validCreateInput("three", nil, nil),
		}
		for index := range inputs {
			inputs[index].IdempotencyKey = "batch-" + inputs[index].Prompt
		}
		outcomes, err := service.CreateBatch(context.Background(), inputs)
		if err != nil {
			t.Fatalf("CreateBatch() error = %v", err)
		}
		if len(outcomes) != 3 || outcomes[0].Call == nil || outcomes[1].Call == nil || outcomes[2].Call == nil {
			t.Fatalf("outcomes = %#v", outcomes)
		}
		batchID := outcomes[0].Call.BatchID
		if batchID == "" || outcomes[1].Call.BatchID != batchID || outcomes[2].Call.BatchID != batchID {
			t.Fatalf("batch ids = %q %q %q", batchID, outcomes[1].Call.BatchID, outcomes[2].Call.BatchID)
		}
		if outcomes[0].Call.ExpectDigest == outcomes[1].Call.ExpectDigest || len(database.calls) != 3 {
			t.Fatalf("distinct contracts/call count were not preserved")
		}
	})

	t.Run("Should reject empty and over-cap batches without writes", func(t *testing.T) {
		t.Parallel()
		cfg := config.DefaultCallsConfig()
		cfg.MaxBatch = 2
		service, database, _, _ := newCallServiceHarness(t, cfg, validAgentTarget())
		if _, err := service.CreateBatch(context.Background(), nil); !IsCode(err, CodeBatchEmpty) {
			t.Fatalf("CreateBatch(empty) error = %v", err)
		}
		items := []CreateInput{
			validCreateInput("1", nil, nil),
			validCreateInput("2", nil, nil),
			validCreateInput("3", nil, nil),
		}
		if _, err := service.CreateBatch(context.Background(), items); !IsCode(err, CodeBatchOverCap) {
			t.Fatalf("CreateBatch(over cap) error = %v", err)
		}
		if len(database.calls) != 0 {
			t.Fatalf("over-cap batch wrote %d calls", len(database.calls))
		}
	})

	t.Run("Should isolate an unknown middle item and accept its neighbors", func(t *testing.T) {
		t.Parallel()
		directory := routedCallDirectory(func(
			_ context.Context,
			input CreateInput,
		) (TargetContext, []AgentRosterEntry, error) {
			target := validAgentTarget()
			if input.Prompt == "two" {
				target.AgentName = ""
			}
			return target, []AgentRosterEntry{{Name: "reviewer", Description: "Reviews work"}}, nil
		})
		service, database, _, invoker := newCallServiceForDirectory(t, config.DefaultCallsConfig(), directory)
		inputs := []CreateInput{
			validCreateInput("one", nil, nil),
			validCreateInput("two", nil, nil),
			validCreateInput("three", nil, nil),
		}
		for index := range inputs {
			inputs[index].IdempotencyKey = inputs[index].Prompt
		}
		outcomes, err := service.CreateBatch(context.Background(), inputs)
		if err != nil {
			t.Fatalf("CreateBatch() error = %v", err)
		}
		if outcomes[0].Call == nil || !IsCode(outcomes[1].Error, CodeAgentUnknown) || outcomes[2].Call == nil {
			t.Fatalf("CreateBatch() outcomes = %#v", outcomes)
		}
		if len(database.calls) != 2 || len(invoker.spawns) != 0 {
			t.Fatalf(
				"isolated batch side effects = calls %d spawns %d, want admission without spawn",
				len(database.calls),
				len(invoker.spawns),
			)
		}
	})

	t.Run(
		"Should reject items after the per-parent child wall while preserving earlier admissions",
		func(t *testing.T) {
			t.Parallel()
			cfg := config.DefaultCallsConfig()
			cfg.MaxChildren = 2
			var mu sync.Mutex
			liveChildren := 0
			directory := routedCallDirectory(func(
				_ context.Context,
				_ CreateInput,
			) (TargetContext, []AgentRosterEntry, error) {
				mu.Lock()
				defer mu.Unlock()
				target := validAgentTarget()
				target.LiveChildren = liveChildren
				liveChildren++
				return target, []AgentRosterEntry{{Name: "reviewer"}}, nil
			})
			service, database, _, invoker := newCallServiceForDirectory(t, cfg, directory)
			inputs := []CreateInput{
				validCreateInput("one", nil, nil),
				validCreateInput("two", nil, nil),
				validCreateInput("three", nil, nil),
				validCreateInput("four", nil, nil),
			}
			for index := range inputs {
				inputs[index].IdempotencyKey = inputs[index].Prompt
			}
			outcomes, err := service.CreateBatch(context.Background(), inputs)
			if err != nil {
				t.Fatalf("CreateBatch() error = %v", err)
			}
			if outcomes[0].Call == nil || outcomes[1].Call == nil ||
				!IsCode(outcomes[2].Error, CodeChildrenCap) || !IsCode(outcomes[3].Error, CodeChildrenCap) {
				t.Fatalf("CreateBatch() outcomes = %#v", outcomes)
			}
			if len(database.calls) != 2 || len(invoker.spawns) != 0 {
				t.Fatalf("child-wall side effects = calls %d spawns %d", len(database.calls), len(invoker.spawns))
			}
		},
	)

	t.Run("Should reject an 800-item batch on the validation-only path", func(t *testing.T) {
		t.Parallel()
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), validAgentTarget())
		items := make([]CreateInput, 800)
		for index := range items {
			items[index] = validCreateInput("work", nil, nil)
		}
		_, err := service.CreateBatch(context.Background(), items)
		if !IsCode(err, CodeBatchOverCap) {
			t.Fatalf("CreateBatch(800) error = %v", err)
		}
		if len(database.calls) != 0 || len(invoker.spawns) != 0 {
			t.Fatalf("CreateBatch(800) calls=%d spawns=%d", len(database.calls), len(invoker.spawns))
		}
	})

	t.Run("Should address an active child without spawning and revive a parked child", func(t *testing.T) {
		t.Parallel()
		active := validAgentTarget()
		active.AgentName = ""
		active.ChildSessionID = "child-existing"
		active.State = "active"
		service, database, _, invoker := newCallServiceHarness(t, config.DefaultCallsConfig(), active)
		input := validCreateInput("follow up", nil, nil)
		input.Target = Target{SessionID: active.ChildSessionID}
		record, err := service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(active follow-up) error = %v", err)
		}
		if record.State != StateRunning || len(invoker.spawns) != 0 ||
			database.admissions[0].FollowUp == nil ||
			database.admissions[0].FollowUp.Kind != DeliveryKindFollowUp {
			t.Fatalf("active follow-up record/admission = %#v / %#v", record, database.admissions[0])
		}

		parked := active
		parked.State = "parked"
		service, _, _, invoker = newCallServiceHarness(t, config.DefaultCallsConfig(), parked)
		record, err = service.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create(parked follow-up) error = %v", err)
		}
		record = activateCreatedCall(t, service, &record)
		if record.State != StateRunning || len(invoker.revives) != 1 || invoker.revives[0] != parked.ChildSessionID {
			t.Fatalf("parked follow-up = %#v, revives=%#v", record, invoker.revives)
		}
	})

	// Invariant: one parked child processes at most one revived call at a time.
	t.Run("Should serialize queued follow-ups for the same parked child", func(t *testing.T) {
		t.Parallel()
		parked := validAgentTarget()
		parked.AgentName = ""
		parked.ChildSessionID = "child-serialized"
		parked.State = TargetStateParked
		service, database, _, invoker := newCallServiceHarness(
			t,
			config.DefaultCallsConfig(),
			parked,
		)
		inputs := make([]CreateInput, 3)
		for index := range inputs {
			inputs[index] = validCreateInput(fmt.Sprintf("follow up %d", index), nil, nil)
			inputs[index].Target = Target{SessionID: parked.ChildSessionID}
			inputs[index].IdempotencyKey = fmt.Sprintf("follow-up-%d", index)
		}
		outcomes, err := service.CreateBatch(t.Context(), inputs)
		if err != nil {
			t.Fatalf("CreateBatch() error = %v", err)
		}
		for index := range outcomes {
			if outcomes[index].Error != nil || outcomes[index].Call == nil {
				t.Fatalf("CreateBatch() outcome %d = %#v", index, outcomes[index])
			}
		}

		dispatched, err := service.DispatchQueued(t.Context(), 100)
		if err != nil {
			t.Fatalf("DispatchQueued() error = %v", err)
		}
		running := 0
		queued := 0
		for callID := range database.calls {
			switch database.calls[callID].State {
			case StateRunning:
				running++
			case StateQueued:
				queued++
			}
		}
		if dispatched != 1 || running != 1 || queued != 2 || len(invoker.revives) != 1 {
			t.Fatalf(
				"DispatchQueued() = %d, running=%d queued=%d revives=%d, want 1/1/2/1",
				dispatched,
				running,
				queued,
				len(invoker.revives),
			)
		}
	})

	for _, state := range []struct {
		name  string
		state TargetState
		code  ErrorCode
	}{{"Should distinguish an expired target", TargetStateExpired, CodeTargetExpired}, {"Should distinguish an unknown target", TargetStateMissing, CodeNotFound}} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			target := validAgentTarget()
			target.AgentName = ""
			target.ChildSessionID = "child-target"
			target.State = state.state
			target.ExpiredAt = time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
			service, database, _, _ := newCallServiceHarness(t, config.DefaultCallsConfig(), target)
			input := validCreateInput("follow up", nil, nil)
			input.Target = Target{SessionID: target.ChildSessionID}
			_, err := service.Create(context.Background(), input)
			if !IsCode(err, state.code) || len(database.calls) != 0 {
				t.Fatalf("Create() error = %v, calls=%d, want %s and no writes", err, len(database.calls), state.code)
			}
			if state.code == CodeTargetExpired {
				var callErr *Error
				if !errors.As(err, &callErr) || callErr.ExpiredAt == "" ||
					callErr.Suggestion != "call the agent fresh" {
					t.Fatalf("Create(expired) error = %#v, want timestamp and fresh-call suggestion", err)
				}
			}
		})
	}
}

func withTarget(target TargetContext, mutate func(*TargetContext)) TargetContext {
	mutate(&target)
	return target
}

func withInput(input CreateInput, mutate func(*CreateInput)) CreateInput {
	mutate(&input)
	return input
}
