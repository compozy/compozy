//go:build integration

package automation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestDispatcherIntegrationDifferentActivationKindsShareConcurrencyGate(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)

	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "integration-job", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	trigger, err := db.CreateTrigger(ctx, testTrigger(AutomationScopeGlobal, "integration-trigger", ""))
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}

	promptStarted := make(chan struct{}, 1)
	promptRelease := make(chan struct{})
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		promptStarted: promptStarted,
		promptRelease: promptRelease,
	})
	dispatcher := newTestDispatcher(t, creator, db, WithDispatcherMaxConcurrent(1))

	firstErrCh := make(chan error, 1)
	go func() {
		_, err := dispatcher.Dispatch(ctx, DispatchRequest{
			Kind: DispatchKindSchedule,
			Job:  &job,
		})
		firstErrCh <- err
	}()

	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first dispatch did not reach Prompt() in time")
	}

	_, err = dispatcher.Dispatch(ctx, DispatchRequest{
		Kind:     DispatchKindTrigger,
		Trigger:  &trigger,
		Envelope: pointerToEnvelope(testEnvelope(AutomationScopeGlobal, "")),
	})
	if !errors.Is(err, ErrConcurrencyLimitReached) {
		t.Fatalf("Dispatch(trigger) error = %v, want ErrConcurrencyLimitReached", err)
	}

	close(promptRelease)
	select {
	case err := <-firstErrCh:
		if err != nil {
			t.Fatalf("Dispatch(job) error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first dispatch did not finish after prompt release")
	}
}

func TestDispatcherIntegrationRunLifecycleStateTransitionsPersist(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)

	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "integration-lifecycle", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	createStarted := make(chan struct{}, 1)
	createRelease := make(chan struct{})
	promptStarted := make(chan struct{}, 1)
	promptRelease := make(chan struct{})
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		createStarted: createStarted,
		createRelease: createRelease,
		promptStarted: promptStarted,
		promptRelease: promptRelease,
	})
	dispatcher := newTestDispatcher(t, creator, db)

	resultCh := make(chan *Run, 1)
	errCh := make(chan error, 1)
	go func() {
		run, err := dispatcher.Dispatch(ctx, DispatchRequest{
			Kind: DispatchKindSchedule,
			Job:  &job,
		})
		resultCh <- run
		errCh <- err
	}()

	select {
	case <-createStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not reach Create() in time")
	}

	scheduledRuns, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns(scheduled) error = %v", err)
	}
	if got, want := len(scheduledRuns), 1; got != want {
		t.Fatalf("len(scheduledRuns) = %d, want %d", got, want)
	}
	if got, want := scheduledRuns[0].Status, RunScheduled; got != want {
		t.Fatalf("scheduledRuns[0].Status = %q, want %q", got, want)
	}
	if got := scheduledRuns[0].SessionID; got != "" {
		t.Fatalf("scheduledRuns[0].SessionID = %q, want empty", got)
	}
	if scheduledRuns[0].EndedAt != nil {
		t.Fatalf("scheduledRuns[0].EndedAt = %v, want nil", *scheduledRuns[0].EndedAt)
	}

	close(createRelease)

	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not reach Prompt() in time")
	}

	runningRuns, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns(running) error = %v", err)
	}
	if got, want := runningRuns[0].Status, RunRunning; got != want {
		t.Fatalf("runningRuns[0].Status = %q, want %q", got, want)
	}
	if got := runningRuns[0].SessionID; got == "" {
		t.Fatal("runningRuns[0].SessionID = empty, want populated")
	}

	close(promptRelease)

	var completedRun *Run
	select {
	case completedRun = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not finish after prompt release")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch error channel did not return")
	}

	reloaded, err := db.GetRun(ctx, completedRun.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if got, want := reloaded.Status, RunCompleted; got != want {
		t.Fatalf("GetRun().Status = %q, want %q", got, want)
	}
	if reloaded.EndedAt == nil {
		t.Fatal("GetRun().EndedAt = nil, want populated")
	}
}

func TestDispatcherIntegrationCancelledPromptPersistsCancelledRunState(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)

	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "integration-cancelled", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	promptStarted := make(chan struct{}, 1)
	promptRelease := make(chan struct{})
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		promptStarted: promptStarted,
		promptRelease: promptRelease,
	})
	dispatcher := newTestDispatcher(t, creator, db)

	dispatchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runCh := make(chan *Run, 1)
	errCh := make(chan error, 1)
	go func() {
		run, err := dispatcher.Dispatch(dispatchCtx, DispatchRequest{
			Kind: DispatchKindSchedule,
			Job:  &job,
		})
		runCh <- run
		errCh <- err
	}()

	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not reach Prompt() in time")
	}

	cancel()

	var run *Run
	select {
	case run = <-runCh:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not return after cancellation")
	}
	if run == nil {
		t.Fatal("Dispatch() run = nil, want populated")
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Dispatch() error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch error channel did not return")
	}

	reloaded, err := db.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if got, want := reloaded.Status, RunCancelled; got != want {
		t.Fatalf("GetRun().Status = %q, want %q", got, want)
	}
	if reloaded.EndedAt == nil {
		t.Fatal("GetRun().EndedAt = nil, want populated")
	}
}

func openAutomationIntegrationDB(t *testing.T, ctx context.Context) *globaldb.GlobalDB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	db, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return db
}
