package sessioncleanup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const testProfile = "profile-1"

type fakeLister struct {
	rows []store.SessionHistoryCleanupRow
}

func (l fakeLister) ListSessionHistoryCleanupRows(
	context.Context,
	store.SessionHistoryCleanupQuery,
) ([]store.SessionHistoryCleanupRow, error) {
	return slices.Clone(l.rows), nil
}

// fakeDeleter records deletes; results fail chosen sessions, and a non-nil gate
// holds every delete until the test releases it.
type fakeDeleter struct {
	delete  func(context.Context, store.ArchivedSessionDelete) error
	results map[string]error
	entered chan string
	gate    chan struct{}

	releaseOnce sync.Once
	mu          sync.Mutex
	deleted     []string
}

func (d *fakeDeleter) DeleteArchived(ctx context.Context, target store.ArchivedSessionDelete) error {
	if d.delete != nil {
		return d.delete(ctx, target)
	}
	if d.entered != nil {
		d.entered <- target.SessionID
	}
	if d.gate != nil {
		<-d.gate
	}
	if err := d.results[target.SessionID]; err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deleted = append(d.deleted, target.SessionID)
	return nil
}

// release opens the gate once; the harness also calls it before shutdown, so a
// failed assertion cannot leave a delete blocked and hang the test.
func (d *fakeDeleter) release() {
	d.releaseOnce.Do(func() {
		if d.gate != nil {
			close(d.gate)
		}
	})
}

func (d *fakeDeleter) deletedIDs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.deleted)
}

type serviceHarness struct {
	service  *Service
	deleter  *fakeDeleter
	notified chan struct{}
}

func newServiceHarness(
	t *testing.T,
	rows []store.SessionHistoryCleanupRow,
	deleter *fakeDeleter,
	opts ...Option,
) serviceHarness {
	t.Helper()
	notified := make(chan struct{}, 1)
	options := []Option{
		WithClock(func() time.Time { return planNow }),
		WithConcurrency(1),
		WithProgressInterval(0),
		WithPublisher(func(string) {
			select {
			case notified <- struct{}{}:
			default:
			}
		}),
	}
	service, err := NewService(fakeLister{rows: rows}, deleter, append(options, opts...)...)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if err := service.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	// Registered after Shutdown, so it runs first.
	t.Cleanup(deleter.release)
	return serviceHarness{service: service, deleter: deleter, notified: notified}
}

func (h serviceHarness) waitForSettled(t *testing.T) Run {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		if run, ok := h.service.Current(testProfile); ok && run.State != RunRunning {
			return run
		}
		select {
		case <-h.notified:
		case <-deadline:
			t.Fatal("cleanup run did not settle")
		}
	}
}

func serviceRequest() Request {
	request := everythingRequest()
	request.Scope.ProfileID = testProfile
	return request
}

func TestServiceStart(t *testing.T) {
	t.Parallel()

	// Invariant: accepted deletes survive their caller but terminate at a per-delete deadline.
	// Owner: cleanup service lifetime; canonical suite: TestServiceStart.
	t.Run("Should bound a stalled delete and finish the failed thread", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			entered := false
			const timeout = 50 * time.Millisecond
			h := newServiceHarness(t, []store.SessionHistoryCleanupRow{archivedRow("root", "", 10)}, &fakeDeleter{
				delete: func(ctx context.Context, _ store.ArchivedSessionDelete) error {
					entered = true
					if _, ok := ctx.Deadline(); !ok {
						t.Error("delete has no deadline")
						return errors.New("missing deadline")
					}
					<-ctx.Done()
					return ctx.Err()
				},
			}, WithDeleteTimeout(timeout))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if _, _, err := h.service.Start(ctx, serviceRequest()); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			if !entered {
				t.Fatal("delete did not start")
			}
			cancel()
			synctest.Wait()
			if run, _ := h.service.Current(testProfile); run.State != RunRunning {
				t.Fatal("caller cancellation interrupted accepted deletion")
			}
			time.Sleep(timeout)
			synctest.Wait()
			run := h.waitForSettled(t)
			if run.FailedThreads != 1 || run.Deleted != 0 {
				t.Fatalf("deadline outcome=%+v", run)
			}
		})
	})

	t.Run("Should delete the planned sessions in the background and complete", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{archivedRow("root", "", 10), archivedRow("child", "root", 10)}
		h := newServiceHarness(t, rows, &fakeDeleter{})
		started, plan, err := h.service.Start(context.Background(), serviceRequest())
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if started.Total != 2 || plan.Counts.Total() != 2 || !started.StartedAt.Equal(planNow) {
			t.Fatalf("Start() = run %+v plan counts %+v", started, plan.Counts)
		}
		run := h.waitForSettled(t)
		if run.State != RunCompleted || run.Deleted != 2 || run.Done() != run.Total || run.FinishedAt == nil {
			t.Fatalf("run = %+v, want completed with 2 deleted", run)
		}
		if got := h.deleter.deletedIDs(); !slices.Equal(got, []string{"child", "root"}) {
			t.Fatalf("deleted = %v, want deepest first", got)
		}
	})

	t.Run("Should keep what was archived after the confirmed count", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("root", "", 10),
			// Archived after the count: then still in the list, so it stays.
			archivedRow("child", "root", 1),
			archivedRow("late", "", 1),
		}
		h := newServiceHarness(t, rows, &fakeDeleter{})
		request := serviceRequest()
		request.CountedAt = daysAgo(5)
		started, plan, err := h.service.Start(context.Background(), request)
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if started.Total != 1 || plan.KeptLive != 1 || !plan.CountedAt.Equal(*request.CountedAt) ||
			!started.CountedAt.Equal(plan.CountedAt) {
			t.Fatalf("Start() = run %+v plan %+v, want only root, child kept", started, plan)
		}
		h.waitForSettled(t)
		if got := h.deleter.deletedIDs(); !slices.Equal(got, []string{"root"}) {
			t.Fatalf("deleted = %v, want only the session the count saw archived", got)
		}
	})

	t.Run("Should count a session restored meanwhile as skipped", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{archivedRow("root", "", 10), archivedRow("child", "root", 10)}
		deleter := &fakeDeleter{results: map[string]error{
			"root": fmt.Errorf("restored: %w", store.ErrSessionNotArchived),
		}}
		h := newServiceHarness(t, rows, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run := h.waitForSettled(t)
		if run.State != RunCompleted || run.Deleted != 1 || run.Skipped != 1 || run.FailedThreads != 0 {
			t.Fatalf("run = %+v, want completed with 1 deleted and 1 skipped", run)
		}
	})

	// Invariant: an unverified not-found delete failure never counts a retained session as deleted.
	// Owner: cleanup orchestration; the existing service suite exercises the deleter I/O boundary.
	t.Run("Should report a not-found delete failure instead of counting it as deleted", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, []store.SessionHistoryCleanupRow{archivedRow("root", "", 10)}, &fakeDeleter{
			results: map[string]error{"root": store.ErrSessionNotFound},
		})
		if _, _, err := h.service.Start(t.Context(), serviceRequest()); err != nil {
			t.Fatal(err)
		}
		run := h.waitForSettled(t)
		if run.State != RunFailed || run.Deleted != 0 || run.FailedThreads != 1 || len(run.Failures) != 1 {
			t.Fatalf("run = %+v, want one failure and no deletion", run)
		}
	})

	t.Run("Should stop a failing thread and finish the others as partial", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("a", "", 10),
			archivedRow("a-child", "a", 10),
			archivedRow("b", "", 10),
		}
		// The reported reason is the root cause, without the daemon's wrapping path.
		stage := fmt.Errorf(
			"session: stage %q for deletion: %w", "/var/folders/x/sess_a-child", errors.New("disk full"),
		)
		deleter := &fakeDeleter{results: map[string]error{"a-child": stage}}
		h := newServiceHarness(t, rows, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run := h.waitForSettled(t)
		if run.State != RunPartial || run.FailedThreads != 1 || run.Deleted != 1 {
			t.Fatalf("run = %+v, want partial with one failed thread", run)
		}
		want := Failure{
			RootID:      "a",
			WorkspaceID: "ws-1",
			Title:       "Thread a",
			Code:        FailureDeleteFailed,
			Message:     "disk full",
		}
		if len(run.Failures) != 1 || run.Failures[0] != want {
			t.Fatalf("Failures = %+v, want [%+v]", run.Failures, want)
		}
		if got := h.deleter.deletedIDs(); !slices.Equal(got, []string{"b"}) {
			t.Fatalf("deleted = %v, want [b]: the failed thread keeps its main session", got)
		}
	})

	t.Run("Should report every root cause of a delete and its rollback, without paths", func(t *testing.T) {
		t.Parallel()
		failed := errors.Join(
			fmt.Errorf("session: stage %q: %w", "/var/folders/x/sess_a", errors.New("operation not permitted")),
			fmt.Errorf("session: roll back %q: %w", "/var/folders/x/sess_a", errors.New("database is locked")),
		)
		deleter := &fakeDeleter{results: map[string]error{"a": failed}}
		h := newServiceHarness(t, []store.SessionHistoryCleanupRow{archivedRow("a", "", 10)}, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run := h.waitForSettled(t)
		if len(run.Failures) != 1 || run.Failures[0].Message != "operation not permitted; database is locked" {
			t.Fatalf("Failures = %+v, want both root causes and no paths", run.Failures)
		}
	})

	// Invariant: a reader-wait timeout fails its root, keeps that thread reachable, and continues the run.
	// Owner: cleanup orchestration; canonical service suite at the Deleter I/O boundary.
	t.Run("Should record a delete timeout per root and continue other threads", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("a", "", 10),
			archivedRow("a-child", "a", 10),
			archivedRow("b", "", 10),
		}
		deleter := &fakeDeleter{results: map[string]error{
			"a-child": fmt.Errorf("quiesce read-only pool: %w", context.DeadlineExceeded),
		}}
		h := newServiceHarness(t, rows, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run := h.waitForSettled(t)
		if run.State != RunPartial || run.FailedThreads != 1 || run.Deleted != 1 || run.FinishedAt == nil {
			t.Fatalf("run = %+v, want settled partial with one failed thread and one deletion", run)
		}
		want := Failure{
			RootID: "a", WorkspaceID: "ws-1", Title: "Thread a",
			Code: FailureDeleteFailed, Message: context.DeadlineExceeded.Error(),
		}
		if len(run.Failures) != 1 || run.Failures[0] != want {
			t.Fatalf("Failures = %+v, want [%+v]", run.Failures, want)
		}
		if got := h.deleter.deletedIDs(); !slices.Equal(got, []string{"b"}) {
			t.Fatalf("deleted = %v, want [b] with the failed root retained", got)
		}
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start(after timeout run settled) error = %v", err)
		}
		h.waitForSettled(t)
	})

	t.Run("Should fail the run when nothing could be deleted", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{archivedRow("a", "", 10)}
		deleter := &fakeDeleter{results: map[string]error{"a": store.ErrSessionIsInstanceMain}}
		h := newServiceHarness(t, rows, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		run := h.waitForSettled(t)
		if run.State != RunFailed || len(run.Failures) != 1 || run.Failures[0].Code != FailureInstanceMain {
			t.Fatalf("run = %+v, want failed with an instance_main failure", run)
		}
	})

	t.Run("Should refuse a second run while one is running", func(t *testing.T) {
		t.Parallel()
		deleter := &fakeDeleter{entered: make(chan string, 1), gate: make(chan struct{})}
		h := newServiceHarness(t, []store.SessionHistoryCleanupRow{archivedRow("a", "", 10)}, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		<-deleter.entered
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); !errors.Is(err, ErrRunActive) {
			t.Fatalf("second Start() error = %v, want ErrRunActive", err)
		}
		deleter.release()
		if run := h.waitForSettled(t); run.State != RunCompleted {
			t.Fatalf("run = %+v, want completed", run)
		}
	})

	t.Run("Should refuse a request that cannot be planned", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, nil, &fakeDeleter{})
		tests := []struct {
			name   string
			mutate func(*Request)
		}{
			{name: "Should require a profile", mutate: func(r *Request) { r.Scope.ProfileID = " " }},
			{name: "Should require one workspace scope", mutate: func(r *Request) { r.Scope.AllWorkspaces = true }},
			{name: "Should require a selection", mutate: func(r *Request) { r.Selection = Selection{} }},
			{name: "Should cap retried session ids", mutate: func(r *Request) {
				r.RootIDs = make([]string, maxRootIDs+1)
			}},
			{name: "Should refuse retried session ids that are all blank", mutate: func(r *Request) {
				r.RootIDs = []string{"", "  "}
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				request := serviceRequest()
				tt.mutate(&request)
				if _, _, err := h.service.Start(context.Background(), request); !errors.Is(err, ErrInvalidRequest) {
					t.Fatalf("Start() error = %v, want ErrInvalidRequest", err)
				}
			})
		}
	})
}

func TestServiceStop(t *testing.T) {
	t.Parallel()

	t.Run("Should stop between deletes and keep what was deleted", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("a", "", 10),
			archivedRow("a-child", "a", 10),
			archivedRow("b", "", 10),
		}
		deleter := &fakeDeleter{entered: make(chan string, 1), gate: make(chan struct{})}
		h := newServiceHarness(t, rows, deleter)
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if first := <-deleter.entered; first != "a-child" {
			t.Fatalf("first delete = %q, want a-child", first)
		}
		// A canceled wait returns at once, after Stop has already asked the run to stop.
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := h.service.Stop(canceled, testProfile, ""); !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop() error = %v, want the canceled wait", err)
		}
		deleter.release()
		run := h.waitForSettled(t)
		if run.State != RunStopped || run.Deleted != 1 {
			t.Fatalf("run = %+v, want stopped after the delete in flight", run)
		}
		if got := h.deleter.deletedIDs(); !slices.Equal(got, []string{"a-child"}) {
			t.Fatalf("deleted = %v, want only the delete in flight", got)
		}
	})

	t.Run("Should report no active run when nothing is running", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, nil, &fakeDeleter{})
		if _, err := h.service.Stop(context.Background(), testProfile, ""); !errors.Is(err, ErrNoActiveRun) {
			t.Fatalf("Stop() error = %v, want ErrNoActiveRun", err)
		}
	})
}

func TestServiceShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse new runs once shut down", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, []store.SessionHistoryCleanupRow{archivedRow("a", "", 10)}, &fakeDeleter{})
		if err := h.service.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if _, _, err := h.service.Start(context.Background(), serviceRequest()); !errors.Is(err, ErrShuttingDown) {
			t.Fatalf("Start() error = %v, want ErrShuttingDown", err)
		}
		if got := h.deleter.deletedIDs(); len(got) != 0 {
			t.Fatalf("deleted = %v, want nothing", got)
		}
	})
}
