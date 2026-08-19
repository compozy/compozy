package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAsyncApprovalCoordinator(t *testing.T) {
	t.Parallel()

	t.Run("Should reject secret-bearing arguments before persistence", func(t *testing.T) {
		t.Parallel()
		store := newMemoryApprovalStore()
		coordinator := newTestApprovalCoordinator(t, store, &recordingApprovalDispatcher{})
		_, err := coordinator.Begin(t.Context(), testApprovalRequest("secret-invocation"))
		if !errors.Is(err, ErrCannotDeferSecrets) {
			t.Fatalf("Begin(secret) error = %v, want ErrCannotDeferSecrets", err)
		}
		if got := store.count(); got != 0 {
			t.Fatalf("persisted approvals = %d, want 0", got)
		}
	})

	t.Run("Should resolve once and dispatch an approved invocation exactly once [E2E-024]", func(t *testing.T) {
		t.Parallel()
		store := newMemoryApprovalStore()
		releaseDispatch := make(chan struct{}, 1)
		t.Cleanup(func() {
			select {
			case releaseDispatch <- struct{}{}:
			default:
			}
		})
		dispatcher := &recordingApprovalDispatcher{
			result: json.RawMessage(`{"ok":true}`), release: releaseDispatch,
		}
		coordinator := newTestApprovalCoordinator(t, store, dispatcher)
		request := testApprovalRequest("invocation-one")
		request.ContainsSecretArguments = false
		ticket, err := coordinator.Begin(t.Context(), request)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if err := coordinator.Resolve(t.Context(), ticket.ApprovalID, ApprovalApproved); err != nil {
			t.Fatalf("Resolve(approved) error = %v", err)
		}
		claimed, err := coordinator.Status(t.Context(), ticket.ApprovalID)
		if err != nil {
			t.Fatalf("Status(claimed) error = %v", err)
		}
		if claimed.ExecutionStatus != ApprovalDispatching || !claimed.ResumeFence {
			t.Fatalf("Status(claimed) = %#v, want fenced dispatch before Resolve returns", claimed)
		}
		releaseDispatch <- struct{}{}
		if err := coordinator.Resolve(t.Context(), ticket.ApprovalID, ApprovalApproved); !errors.Is(
			err,
			ErrApprovalTerminal,
		) {
			t.Fatalf("Resolve(duplicate) error = %v, want ErrApprovalTerminal", err)
		}
		status := waitForApprovalStatus(t, coordinator, ticket.ApprovalID, ApprovalCompleted)
		if string(status.Result) != `{"ok":true}` || !status.ResumeFence {
			t.Fatalf("Status() = %#v, want fenced completed result", status)
		}
		if got := dispatcher.calls(); got != 1 {
			t.Fatalf("dispatch calls = %d, want 1", got)
		}
	})

	t.Run("Should recover ambiguous dispatches as uncertain without replay", func(t *testing.T) {
		t.Parallel()
		store := newMemoryApprovalStore()
		now := time.Now().UTC()
		store.seed(ApprovalStatus{
			ApprovalID: "apr_dispatching", WorkspaceID: "workspace-a", InvocationID: "invocation-a",
			Target: ApprovalTarget{Kind: ApprovalTargetTool, ToolID: "compozy__test"}, Args: json.RawMessage(`{}`),
			ApprovalStatus: ApprovalApproved, ExecutionStatus: ApprovalDispatching,
			RequestedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), ResumeFence: true,
		})
		store.seed(ApprovalStatus{
			ApprovalID: "apr_expired", WorkspaceID: "workspace-a", InvocationID: "invocation-b",
			Target: ApprovalTarget{Kind: ApprovalTargetTool, ToolID: "compozy__test"}, Args: json.RawMessage(`{}`),
			ApprovalStatus: ApprovalPending, RequestedAt: now.Add(-time.Minute), ExpiresAt: now.Add(-time.Second),
		})
		dispatcher := &recordingApprovalDispatcher{}
		coordinator := newTestApprovalCoordinator(t, store, dispatcher)
		if err := coordinator.Recover(t.Context()); err != nil {
			t.Fatalf("Recover() error = %v", err)
		}
		dispatching, err := coordinator.Status(t.Context(), "apr_dispatching")
		if err != nil {
			t.Fatalf("Status(dispatching) error = %v", err)
		}
		expired, err := coordinator.Status(t.Context(), "apr_expired")
		if err != nil {
			t.Fatalf("Status(expired) error = %v", err)
		}
		if dispatching.ExecutionStatus != ApprovalUncertain || expired.ApprovalStatus != ApprovalTimedOut {
			t.Fatalf("recovered statuses = %#v / %#v", dispatching, expired)
		}
		if got := dispatcher.calls(); got != 0 {
			t.Fatalf("dispatch calls after recovery = %d, want 0", got)
		}
	})

	t.Run("Should complete denial and timeout without dispatch [IT-009][IT-010][E2E-024]", func(t *testing.T) {
		t.Parallel()

		t.Run("Should deny and close the pending completion", func(t *testing.T) {
			t.Parallel()
			store := newMemoryApprovalStore()
			dispatcher := &recordingApprovalDispatcher{}
			coordinator := newTestApprovalCoordinator(t, store, dispatcher)
			request := testApprovalRequest("invocation-denied")
			request.ContainsSecretArguments = false
			ticket, err := coordinator.Begin(t.Context(), request)
			if err != nil {
				t.Fatalf("Begin() error = %v", err)
			}
			if err := coordinator.Resolve(t.Context(), ticket.ApprovalID, ApprovalDenied); err != nil {
				t.Fatalf("Resolve(denied) error = %v", err)
			}
			select {
			case <-ticket.Completion:
			case <-time.After(time.Second):
				t.Fatal("denied approval did not close completion")
			}
			status, err := coordinator.Status(t.Context(), ticket.ApprovalID)
			if err != nil || status.ApprovalStatus != ApprovalDenied || dispatcher.calls() != 0 {
				t.Fatalf("denied status = %#v, error = %v, dispatches = %d", status, err, dispatcher.calls())
			}
		})

		t.Run("Should time out and close the pending completion", func(t *testing.T) {
			t.Parallel()
			store := newMemoryApprovalStore()
			dispatcher := &recordingApprovalDispatcher{}
			coordinator := newTestApprovalCoordinator(t, store, dispatcher)
			request := testApprovalRequest("invocation-timeout")
			request.ContainsSecretArguments = false
			request.ExpiresAt = time.Now().UTC().Add(50 * time.Millisecond)
			ticket, err := coordinator.Begin(t.Context(), request)
			if err != nil {
				t.Fatalf("Begin() error = %v", err)
			}
			select {
			case <-ticket.Completion:
			case <-time.After(2 * time.Second):
				t.Fatal("timed-out approval did not close completion")
			}
			status, err := coordinator.Status(t.Context(), ticket.ApprovalID)
			if err != nil || status.ApprovalStatus != ApprovalTimedOut || dispatcher.calls() != 0 {
				t.Fatalf("timeout status = %#v, error = %v, dispatches = %d", status, err, dispatcher.calls())
			}
		})
	})

	t.Run("Should resolve a late disconnected-client dispatch exactly once [IT-010]", func(t *testing.T) {
		t.Parallel()
		store := newMemoryApprovalStore()
		dispatcher := &recordingApprovalDispatcher{err: errors.New("client disconnected")}
		coordinator := newTestApprovalCoordinator(t, store, dispatcher)
		request := testApprovalRequest("invocation-disconnected")
		request.ContainsSecretArguments = false
		request.Target = ApprovalTarget{Kind: ApprovalTargetClientOp, Payload: json.RawMessage(`{"client":"gone"}`)}
		ticket, err := coordinator.Begin(t.Context(), request)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		if err := coordinator.Resolve(t.Context(), ticket.ApprovalID, ApprovalApproved); err != nil {
			t.Fatalf("Resolve(approved) error = %v", err)
		}
		status := waitForApprovalStatus(t, coordinator, ticket.ApprovalID, ApprovalFailed)
		if dispatcher.calls() != 1 || !strings.Contains(string(status.Error), "client disconnected") {
			t.Fatalf("late dispatch = calls %d status %#v", dispatcher.calls(), status)
		}
		if err := coordinator.Resolve(t.Context(), ticket.ApprovalID, ApprovalApproved); !errors.Is(
			err,
			ErrApprovalTerminal,
		) {
			t.Fatalf("duplicate Resolve() error = %v, want terminal", err)
		}
	})
}

func newTestApprovalCoordinator(
	t *testing.T,
	store ApprovalPendingStore,
	dispatcher ApprovalDispatcher,
) ApprovalCoordinator {
	t.Helper()
	coordinator, err := NewApprovalCoordinator(store, dispatcher)
	if err != nil {
		t.Fatalf("NewApprovalCoordinator() error = %v", err)
	}
	t.Cleanup(func() {
		if err := coordinator.Close(); err != nil {
			t.Errorf("ApprovalCoordinator.Close() error = %v", err)
		}
	})
	return coordinator
}

func testApprovalRequest(invocationID string) ApprovalRequest {
	return ApprovalRequest{
		WorkspaceID: "workspace-a", InvocationID: invocationID,
		Target: ApprovalTarget{Kind: ApprovalTargetTool, ToolID: "compozy__test"},
		Args:   json.RawMessage(`{"value":1}`), ExpiresAt: time.Now().UTC().Add(time.Hour),
		ContainsSecretArguments: true,
	}
}

func waitForApprovalStatus(
	t *testing.T,
	coordinator ApprovalCoordinator,
	approvalID string,
	want ApprovalExecutionStatus,
) ApprovalStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, err := coordinator.Status(t.Context(), approvalID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.ExecutionStatus == want {
			return status
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("approval %q did not reach %q", approvalID, want)
	return ApprovalStatus{}
}

type recordingApprovalDispatcher struct {
	mu      sync.Mutex
	count   int
	result  json.RawMessage
	err     error
	release <-chan struct{}
}

func (d *recordingApprovalDispatcher) DispatchApproval(
	_ context.Context,
	_ ApprovalStatus,
) (json.RawMessage, error) {
	d.mu.Lock()
	d.count++
	result := append(json.RawMessage(nil), d.result...)
	err := d.err
	release := d.release
	d.mu.Unlock()
	if release != nil {
		<-release
	}
	return result, err
}

func (d *recordingApprovalDispatcher) calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.count
}

type memoryApprovalStore struct {
	mu      sync.Mutex
	records map[string]ApprovalStatus
}

func newMemoryApprovalStore() *memoryApprovalStore {
	return &memoryApprovalStore{records: make(map[string]ApprovalStatus)}
}

func (s *memoryApprovalStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func (s *memoryApprovalStore) seed(status ApprovalStatus) {
	s.mu.Lock()
	s.records[status.ApprovalID] = cloneApprovalStatus(status)
	s.mu.Unlock()
}

func (s *memoryApprovalStore) CreateApproval(
	_ context.Context,
	approvalID string,
	request ApprovalRequest,
	requestedAt time.Time,
) (ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := ApprovalStatus{
		ApprovalID: approvalID, WorkspaceID: request.WorkspaceID, InvocationID: request.InvocationID,
		CommandID: request.CommandID, Target: request.Target, Args: request.Args,
		ApprovalStatus: ApprovalPending, RequestedAt: requestedAt, ExpiresAt: request.ExpiresAt,
	}
	s.records[approvalID] = cloneApprovalStatus(status)
	return cloneApprovalStatus(status), nil
}

func (s *memoryApprovalStore) GetApproval(_ context.Context, approvalID string) (ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.records[approvalID]
	if !ok {
		return ApprovalStatus{}, ErrApprovalNotFound
	}
	return cloneApprovalStatus(status), nil
}

func (s *memoryApprovalStore) ResolveApproval(
	_ context.Context,
	approvalID string,
	outcome ApprovalOutcome,
	resolvedAt time.Time,
) (ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.records[approvalID]
	if !ok {
		return ApprovalStatus{}, ErrApprovalNotFound
	}
	if status.ApprovalStatus != ApprovalPending {
		return ApprovalStatus{}, ErrApprovalTerminal
	}
	status.ApprovalStatus = outcome
	status.ResolvedAt = &resolvedAt
	if outcome == ApprovalApproved {
		status.ExecutionStatus = ApprovalDispatching
		status.ResumeFence = true
	}
	s.records[approvalID] = status
	return cloneApprovalStatus(status), nil
}

func (s *memoryApprovalStore) CompleteApprovalExecution(
	_ context.Context,
	approvalID string,
	executionStatus ApprovalExecutionStatus,
	result json.RawMessage,
	errorPayload json.RawMessage,
	executedAt time.Time,
) (ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.records[approvalID]
	if !ok {
		return ApprovalStatus{}, ErrApprovalNotFound
	}
	if status.ExecutionStatus != ApprovalDispatching || !status.ResumeFence {
		return ApprovalStatus{}, ErrApprovalDispatchFenced
	}
	status.ExecutionStatus = executionStatus
	status.Result = result
	status.Error = errorPayload
	status.ExecutedAt = &executedAt
	s.records[approvalID] = cloneApprovalStatus(status)
	return cloneApprovalStatus(status), nil
}

func (s *memoryApprovalStore) ExpireApprovals(
	_ context.Context,
	now time.Time,
) ([]ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var expired []ApprovalStatus
	for id, status := range s.records {
		if status.ApprovalStatus == ApprovalPending && !status.ExpiresAt.After(now) {
			status.ApprovalStatus = ApprovalTimedOut
			status.ResolvedAt = &now
			s.records[id] = status
			expired = append(expired, cloneApprovalStatus(status))
		}
	}
	return expired, nil
}

func (s *memoryApprovalStore) RecoverDispatchingApprovals(
	_ context.Context,
	now time.Time,
) ([]ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var recovered []ApprovalStatus
	for id, status := range s.records {
		if status.ApprovalStatus == ApprovalApproved && status.ExecutionStatus == ApprovalDispatching {
			status.ExecutionStatus = ApprovalUncertain
			status.ExecutedAt = &now
			s.records[id] = status
			recovered = append(recovered, cloneApprovalStatus(status))
		}
	}
	return recovered, nil
}

func (s *memoryApprovalStore) ListPendingApprovals(_ context.Context) ([]ApprovalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending []ApprovalStatus
	for _, status := range s.records {
		if status.ApprovalStatus == ApprovalPending {
			pending = append(pending, cloneApprovalStatus(status))
		}
	}
	return pending, nil
}
