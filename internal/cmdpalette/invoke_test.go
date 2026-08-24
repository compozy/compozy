package cmdpalette

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Suite: daemon command-palette invocation.
// Invariant: exact IDs, declared schemas, targeted availability, policy concurrency, and approval fences gate every side effect.
func TestRegistryInvoke(t *testing.T) {
	t.Parallel()

	t.Run("Should fail closed without a lens and keep profile management local [UT-097][UT-098]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("profile.use")
		executor := &testExecutor{}
		service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
		_, err := service.Invoke(t.Context(), InvokeRequest{WorkspaceID: "ws-1", CommandID: descriptor.ID})
		requireErrorContains(t, err, "profile lens is required")
		if executor.callCount() != 0 {
			t.Fatalf("Invoke(missing lens) calls = %d, want fail closed", executor.callCount())
		}
		_, err = service.Invoke(t.Context(), InvokeRequest{
			ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID,
		})
		if !errors.Is(err, ErrProfileManagementForbidden) || executor.callCount() != 0 {
			t.Fatalf("Invoke(remote profile action) error/calls = %v/%d", err, executor.callCount())
		}
		_, err = service.Invoke(t.Context(), InvokeRequest{
			ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID,
			ManagementLocal: true,
		})
		if err != nil || executor.callCount() != 1 {
			t.Fatalf("Invoke(local profile action) error/calls = %v/%d", err, executor.callCount())
		}
	})

	t.Run("Should return the catalog-identical unavailability reason [UT-006]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("window.close")
		descriptor.Action = Action{Kind: ActionKindClientOp, Op: "window.close"}
		descriptor.When = []Predicate{{
			Key: ContextDesktopWindowCount, Operator: PredicateGreaterThanOrEqual,
			Value: 2, Reason: "needs two windows on this desktop",
		}}
		directory := &testClientDirectory{
			clients: []Client{{ID: "client-a"}},
			contexts: map[ClientID]ContextSnapshot{
				"client-a": {Revision: "ctx-1", Values: map[ContextKey]any{ContextDesktopWindowCount: 1}},
			},
		}
		service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, directory, nil, &testExecutor{})
		catalog, err := service.Catalog(t.Context(), testCatalogRequest("ws-1", "client-a"))
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		_, err = service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
		)
		var unavailable *UnavailableError
		if !errors.As(err, &unavailable) {
			t.Fatalf("Invoke() error = %v, want UnavailableError", err)
		}
		if unavailable.Reason != catalog.Commands[0].UnavailableReason {
			t.Fatalf(
				"invoke reason = %q, catalog reason = %q",
				unavailable.Reason,
				catalog.Commands[0].UnavailableReason,
			)
		}
	})

	t.Run("Should reject invalid arguments before execution [UT-007]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("note.capture")
		descriptor.Arguments = []Argument{{Name: "title", Type: ArgumentTypeText, Required: true}}
		executor := &testExecutor{}
		service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
		_, err := service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
		)
		var invalid *InvalidArgumentsError
		if !errors.As(err, &invalid) || invalid.Fields["title"] != "required" {
			t.Fatalf("Invoke() error = %#v, want required title field", err)
		}
		if executor.callCount() != 0 {
			t.Fatalf("executor calls = %d, want 0", executor.callCount())
		}
	})

	t.Run("Should reject an unknown exact ID without fuzzy resolution [UT-008]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("note.capture")
		executor := &testExecutor{}
		service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
		_, err := service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: "note.captur"},
		)
		if !errors.Is(err, ErrCommandNotFound) {
			t.Fatalf("Invoke() error = %v, want ErrCommandNotFound", err)
		}
		if executor.callCount() != 0 {
			t.Fatalf("executor calls = %d, want 0", executor.callCount())
		}
	})

	t.Run("Should enforce only the declared single-flight policy [UT-009]", func(t *testing.T) {
		t.Parallel()
		t.Run("Should reject the second single-flight invocation", func(t *testing.T) {
			t.Parallel()
			descriptor := testDescriptor("note.purge")
			descriptor.Policy = ExecutionPolicy{SingleFlight: true, RetrySafe: false}
			started := make(chan struct{}, 1)
			release := make(chan struct{})
			executor := &testExecutor{
				started: started,
				release: release,
				result:  ExecutionResult{Result: rawResult(map[string]bool{"ok": true})},
			}
			service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
			firstDone := make(chan error, 1)
			go func() {
				_, err := service.Invoke(
					t.Context(),
					InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
				)
				firstDone <- err
			}()
			<-started
			_, err := service.Invoke(
				t.Context(),
				InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
			)
			if !errors.Is(err, ErrAlreadyRunning) {
				t.Fatalf("second Invoke() error = %v, want ErrAlreadyRunning", err)
			}
			close(release)
			if err := <-firstDone; err != nil {
				t.Fatalf("first Invoke() error = %v", err)
			}
		})

		t.Run("Should run declared parallel retry-safe invocations together", func(t *testing.T) {
			t.Parallel()
			descriptor := testDescriptor("note.list")
			descriptor.Policy = ExecutionPolicy{SingleFlight: false, RetrySafe: true}
			started := make(chan struct{}, 2)
			release := make(chan struct{})
			executor := &testExecutor{started: started, release: release}
			service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
			errorsByCall := make(chan error, 2)
			var wait sync.WaitGroup
			for range 2 {
				wait.Go(func() {
					_, err := service.Invoke(
						t.Context(),
						InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
					)
					errorsByCall <- err
				})
			}
			<-started
			<-started
			close(release)
			wait.Wait()
			close(errorsByCall)
			for err := range errorsByCall {
				if err != nil {
					t.Fatalf("parallel Invoke() error = %v", err)
				}
			}
		})
	})

	t.Run("Should hold a destructive tool single-flight guard through approval pending [UT-010]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("note.purge")
		descriptor.Destructive = true
		descriptor.Confirmation = &Confirmation{Title: "Purge notes?", Confirm: "Purge"}
		descriptor.Policy = ExecutionPolicy{SingleFlight: true}
		completion := make(chan struct{})
		executor := &testExecutor{
			approval: true,
			result:   ExecutionResult{ApprovalID: "apr_test", Completion: completion},
		}
		service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
		result, err := service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
		)
		if err != nil {
			t.Fatalf("Invoke() error = %v", err)
		}
		if result.Status != InvokeStatusApprovalPending || result.ApprovalID != "apr_test" {
			t.Fatalf("Invoke() result = %#v, want approval_pending", result)
		}
		_, err = service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
		)
		if !errors.Is(err, ErrAlreadyRunning) {
			t.Fatalf("duplicate pending Invoke() error = %v, want ErrAlreadyRunning", err)
		}
		close(completion)
	})

	t.Run("Should release single-flight after a tool failure so retry can succeed [IT-010]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("note.capture")
		descriptor.Policy = ExecutionPolicy{SingleFlight: true, RetrySafe: true}
		executor := &flakyInvokeExecutor{}
		service := testRegistry(staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor)
		if _, err := service.Invoke(t.Context(), InvokeRequest{ProfileLens: testProfileLens,
			WorkspaceID: "ws-1", CommandID: descriptor.ID,
		}); !errors.Is(err, errFlakyInvoke) {
			t.Fatalf("first Invoke() error = %v, want errFlakyInvoke", err)
		}
		result, err := service.Invoke(t.Context(), InvokeRequest{ProfileLens: testProfileLens,
			WorkspaceID: "ws-1", CommandID: descriptor.ID,
		})
		if err != nil || result.Status != InvokeStatusOK || executor.callCount() != 2 {
			t.Fatalf("retry Invoke() = %#v, error = %v, calls = %d", result, err, executor.callCount())
		}
	})

	t.Run("Should record successful daemon usage without invocation arguments [UT-092]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("vault.unlock")
		descriptor.Arguments = []Argument{{Name: "password", Type: ArgumentTypePassword, Required: true}}
		store := &personalizationStoreStub{recordErr: errors.New("usage store unavailable")}
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, &testExecutor{},
			WithPersonalizationStore(store),
		)
		result, err := service.Invoke(t.Context(), InvokeRequest{ProfileLens: testProfileLens,
			WorkspaceID: "workspace-a",
			CommandID:   descriptor.ID,
			Args:        map[string]any{"password": "never-persist-this"},
		})
		if err != nil || result.Status != InvokeStatusOK {
			t.Fatalf("Invoke() = %#v, error = %v, want success despite usage failure", result, err)
		}
		if recorded := store.lastUsage(); recorded.WorkspaceID != "workspace-a" ||
			recorded.CommandID != descriptor.ID || recorded.Query != "" {
			t.Fatalf("recorded usage = %#v, want identifiers and no argument-derived data", recorded)
		}
	})

	t.Run("Should skip daemon usage recording while personalization is disabled", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("session.new")
		store := &personalizationStoreStub{}
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, &testExecutor{},
			WithPersonalizationStore(store),
			WithPersonalizationPolicy(personalizationPolicyFunc(func(
				context.Context,
				WorkspaceID,
			) (bool, error) {
				return false, nil
			})),
		)

		result, err := service.Invoke(t.Context(), InvokeRequest{ProfileLens: testProfileLens,
			WorkspaceID: "workspace-a", CommandID: descriptor.ID,
		})
		if err != nil || result.Status != InvokeStatusOK {
			t.Fatalf("Invoke() = %#v, error = %v, want success", result, err)
		}
		if recorded := store.lastUsage(); recorded.CommandID != "" {
			t.Fatalf("recorded usage = %#v, want no write while disabled", recorded)
		}
	})

	t.Run("Should release approval-held single-flight on denial and timeout [IT-010]", func(t *testing.T) {
		t.Parallel()
		for _, outcome := range []string{"denied", "timeout"} {
			t.Run("Should release after "+outcome, func(t *testing.T) {
				t.Parallel()
				descriptor := testDescriptor("note.purge." + CommandID(outcome))
				descriptor.Destructive = true
				descriptor.Confirmation = &Confirmation{Title: "Purge notes?", Confirm: "Purge"}
				descriptor.Policy = ExecutionPolicy{SingleFlight: true}
				completion := make(chan struct{})
				recorder := &recordingEventRecorder{wake: make(chan struct{}, 1)}
				executor := &approvalEventExecutor{
					testExecutor: &testExecutor{
						approval: true,
						result:   ExecutionResult{ApprovalID: "apr_" + outcome, Completion: completion},
					},
					outcome: outcome,
				}
				service := testRegistryWithOptions(
					staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor,
					WithEventRecorder(recorder),
				)
				if _, err := service.Invoke(t.Context(), InvokeRequest{ProfileLens: testProfileLens,
					WorkspaceID: "ws-1", CommandID: descriptor.ID,
				}); err != nil {
					t.Fatalf("first Invoke() error = %v", err)
				}
				close(completion)
				select {
				case <-recorder.wake:
				case <-time.After(time.Second):
					t.Fatalf("timed out waiting for %s completion event", outcome)
				}
				events := recorder.recorded()
				if len(events) != 1 || events[0].Outcome != outcome {
					t.Fatalf("completion events = %#v, want outcome %q", events, outcome)
				}
				deadline := time.Now().Add(time.Second)
				for {
					_, err := service.Invoke(t.Context(), InvokeRequest{ProfileLens: testProfileLens,
						WorkspaceID: "ws-1", CommandID: descriptor.ID,
					})
					if !errors.Is(err, ErrAlreadyRunning) {
						if err != nil {
							t.Fatalf("Invoke(after %s) error = %v", outcome, err)
						}
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("single-flight guard remained held after %s", outcome)
					}
					time.Sleep(time.Millisecond)
				}
			})
		}
	})

	t.Run("Should record terminal invocation correlation including resumed approval", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("note.purge")
		descriptor.Destructive = true
		descriptor.Confirmation = &Confirmation{Title: "Purge notes?", Confirm: "Purge"}
		completion := make(chan struct{})
		recorder := &recordingEventRecorder{wake: make(chan struct{}, 1)}
		executor := &approvalEventExecutor{
			testExecutor: &testExecutor{
				approval: true,
				result:   ExecutionResult{ApprovalID: "apr_test", Completion: completion},
			},
			outcome: "denied",
		}
		now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor,
			WithEventRecorder(recorder), WithClock(func() time.Time { return now }),
		)
		result, err := service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "ws-1", CommandID: descriptor.ID},
		)
		if err != nil {
			t.Fatalf("Invoke() error = %v", err)
		}
		if len(recorder.recorded()) != 0 {
			t.Fatal("approval-pending invocation emitted a non-terminal event")
		}
		close(completion)
		select {
		case <-recorder.wake:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for terminal approval event")
		}
		events := recorder.recorded()
		if len(events) != 1 || events[0].Name != EventCommandInvoked ||
			events[0].InvocationID != result.InvocationID || events[0].ApprovalID != "apr_test" ||
			events[0].Outcome != "denied" || events[0].CommandID != descriptor.ID {
			t.Fatalf("invocation events = %#v", events)
		}
	})

	t.Run("Should record usage when a resumed approval completes with ok", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("note.capture")
		store := &personalizationStoreStub{}
		completion := make(chan struct{})
		recorder := &recordingEventRecorder{wake: make(chan struct{}, 1)}
		executor := &approvalEventExecutor{
			testExecutor: &testExecutor{
				approval: true,
				result:   ExecutionResult{ApprovalID: "apr_ok", Completion: completion},
			},
			outcome: "ok",
		}
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{descriptor}}, nil, nil, executor,
			WithPersonalizationStore(store), WithEventRecorder(recorder),
		)
		result, err := service.Invoke(
			t.Context(),
			InvokeRequest{ProfileLens: testProfileLens, WorkspaceID: "workspace-a", CommandID: descriptor.ID},
		)
		if err != nil {
			t.Fatalf("Invoke() error = %v", err)
		}
		if result.Status != InvokeStatusApprovalPending {
			t.Fatalf("Invoke() = %#v, want approval pending", result)
		}
		if recorded := store.lastUsage(); recorded.CommandID != "" {
			t.Fatalf("recorded usage = %#v, want no write while approval is pending", recorded)
		}
		close(completion)
		select {
		case <-recorder.wake:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for approved completion event")
		}
		if recorded := store.lastUsage(); recorded.WorkspaceID != "workspace-a" ||
			recorded.CommandID != descriptor.ID || recorded.Query != "" {
			t.Fatalf("recorded usage = %#v, want one identifier-only usage after ok", recorded)
		}
	})
}

var errFlakyInvoke = errors.New("fixture tool crashed")

type flakyInvokeExecutor struct {
	mu    sync.Mutex
	calls int
}

func (e *flakyInvokeExecutor) ExecuteAction(
	context.Context,
	ExecutionRequest,
) (ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	if e.calls == 1 {
		return ExecutionResult{}, errFlakyInvoke
	}
	return ExecutionResult{Result: rawResult(map[string]bool{"ok": true})}, nil
}

func (e *flakyInvokeExecutor) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}
