package ui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	tea "charm.land/bubbletea/v2"
)

func TestMultiRunInitialSnapshotRendersQueuedTabsInOrder(t *testing.T) {
	t.Parallel()

	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusQueued},
				{Slug: "beta", Status: taskMultiStatusQueued},
				{Slug: "gamma", Status: taskMultiStatusQueued},
			},
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}
	mdl.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

	view := mdl.View().Content
	alpha := strings.Index(view, "alpha QUEUED")
	beta := strings.Index(view, "beta QUEUED")
	gamma := strings.Index(view, "gamma QUEUED")
	if alpha < 0 || beta < 0 || gamma < 0 {
		t.Fatalf("expected queued tabs for alpha, beta, gamma, got %q", view)
	}
	if alpha >= beta || beta >= gamma {
		t.Fatalf("expected queued tabs in request order, got alpha=%d beta=%d gamma=%d", alpha, beta, gamma)
	}
	if !strings.Contains(view, "Child run has not started yet.") {
		t.Fatalf("expected queued active tab message, got %q", view)
	}
}

func TestMultiRunChildStartUpdatesOnlyTargetTabState(t *testing.T) {
	t.Parallel()

	alphaSnapshot := childSnapshotForTest(t, "run-alpha", "alpha", remoteRunStatusRunning, "alpha transcript")
	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusRunning, RunID: "run-alpha"},
				{Slug: "beta", Status: taskMultiStatusQueued},
			},
		},
		LoadChildSnapshot: func(_ context.Context, runID string) (apicore.RunSnapshot, error) {
			if runID != "run-alpha" {
				t.Fatalf("unexpected child snapshot run id %q", runID)
			}
			return alphaSnapshot, nil
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}
	alphaEntries := append([]TranscriptEntry(nil), mdl.tabs[0].child.jobs[0].snapshot.Entries...)

	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleChildStarted,
		kinds.TaskRunMultiplePayload{
			Slug:       "beta",
			Index:      1,
			Total:      2,
			Status:     taskMultiStatusRunning,
			ChildRunID: "run-beta",
		},
	))

	if got := mdl.tabs[1].status; got != taskMultiStatusRunning {
		t.Fatalf("expected beta running, got %q", got)
	}
	if got := mdl.tabs[1].runID; got != "run-beta" {
		t.Fatalf("expected beta child run id, got %q", got)
	}
	if !reflect.DeepEqual(alphaEntries, mdl.tabs[0].child.jobs[0].snapshot.Entries) {
		t.Fatalf("alpha transcript changed after beta start: %#v", mdl.tabs[0].child.jobs[0].snapshot.Entries)
	}
}

func TestMultiRunCompletedTabRemainsNavigableAfterActiveAdvances(t *testing.T) {
	t.Parallel()

	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusRunning, RunID: "run-alpha"},
				{Slug: "beta", Status: taskMultiStatusQueued},
				{Slug: "gamma", Status: taskMultiStatusQueued},
			},
		},
		LoadChildSnapshot: func(context.Context, string) (apicore.RunSnapshot, error) {
			return childSnapshotForTest(t, "run-alpha", "alpha", remoteRunStatusRunning, "alpha transcript"), nil
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}

	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleChildCompleted,
		kinds.TaskRunMultiplePayload{
			Slug:       "alpha",
			Index:      0,
			Total:      3,
			Status:     taskMultiStatusCompleted,
			ChildRunID: "run-alpha",
		},
	))

	if got := mdl.activeTab; got != 1 {
		t.Fatalf("expected active tab to advance to beta, got %d", got)
	}
	if mdl.tabs[0].child == nil {
		t.Fatal("expected completed alpha child view to remain available")
	}
	mdl.moveActiveTab(-1)
	if got := mdl.activeTab; got != 0 {
		t.Fatalf("expected alpha completed tab to remain navigable, got active %d", got)
	}
}

func TestMultiRunTabNavigationDoesNotCycleChildPaneFocus(t *testing.T) {
	t.Parallel()

	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusRunning, RunID: "run-alpha"},
				{Slug: "beta", Status: taskMultiStatusRunning, RunID: "run-beta"},
			},
		},
		LoadChildSnapshot: func(_ context.Context, runID string) (apicore.RunSnapshot, error) {
			return childSnapshotForTest(
				t,
				runID,
				strings.TrimPrefix(runID, "run-"),
				remoteRunStatusRunning,
				runID+" transcript",
			), nil
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}
	mdl.tabs[0].child.focusedPane = uiPaneTimeline

	cmd := mdl.handleKey(keyCode(tea.KeyRight))
	if cmd != nil {
		t.Fatalf("expected tab navigation to stay local, got command %T", cmd())
	}
	if got := mdl.activeTab; got != 1 {
		t.Fatalf("expected active tab beta, got %d", got)
	}
	if got := mdl.tabs[0].child.focusedPane; got != uiPaneTimeline {
		t.Fatalf("expected alpha child focus to remain timeline, got %s", got)
	}
	if got := mdl.tabs[1].child.focusedPane; got != uiPaneJobs {
		t.Fatalf("expected beta child focus to remain jobs, got %s", got)
	}
}

func TestMultiRunTabNavigationUsesHorizontalKeys(t *testing.T) {
	t.Parallel()

	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusQueued},
				{Slug: "beta", Status: taskMultiStatusQueued},
				{Slug: "gamma", Status: taskMultiStatusQueued},
			},
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}

	mdl.handleKey(keyCode(tea.KeyRight))
	if got := mdl.activeTab; got != 1 {
		t.Fatalf("expected right arrow to move to beta tab, got %d", got)
	}
	mdl.handleKey(keyText("l"))
	if got := mdl.activeTab; got != 2 {
		t.Fatalf("expected l to move to gamma tab, got %d", got)
	}
	mdl.handleKey(keyCode(tea.KeyLeft))
	if got := mdl.activeTab; got != 1 {
		t.Fatalf("expected left arrow to move back to beta tab, got %d", got)
	}
	mdl.handleKey(keyText("h"))
	if got := mdl.activeTab; got != 0 {
		t.Fatalf("expected h to move back to alpha tab, got %d", got)
	}

	view := mdl.renderTabs()
	if !strings.Contains(view, "←→/HL") || !strings.Contains(view, "TABS") {
		t.Fatalf("expected tab help to advertise horizontal navigation, got %q", view)
	}
}

func TestMultiRunCloseTUIDoesNotRequestParentCancel(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	var quitRequests []uiQuitRequest
	mdl.onQuit = func(req uiQuitRequest) {
		quitRequests = append(quitRequests, req)
	}

	if cmd := mdl.handleKey(keyText("q")); cmd != nil {
		t.Fatalf("expected q to open quit dialog, got %T", cmd())
	}
	cmd := mdl.handleKey(keyCode(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Close TUI confirmation to return quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected Close TUI to quit, got %T", cmd())
	}
	if len(quitRequests) != 0 {
		t.Fatalf("expected Close TUI not to request parent cancel, got %#v", quitRequests)
	}
}

func TestMultiRunStopRunRequestsParentCancelOnceAndMarksQueuedCanceled(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	var quitRequests []uiQuitRequest
	mdl.onQuit = func(req uiQuitRequest) {
		quitRequests = append(quitRequests, req)
	}

	mdl.handleKey(keyText("q"))
	mdl.handleKey(keyText(keyRight))
	cmd := mdl.handleKey(keyCode(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected Stop Run confirmation to return command")
	}
	if _, ok := cmd().(drainMsg); !ok {
		t.Fatalf("expected Stop Run command to drain, got %T", cmd())
	}
	if got := len(quitRequests); got != 1 {
		t.Fatalf("expected one parent cancel request, got %d", got)
	}
	if quitRequests[0] != uiQuitRequestDrain {
		t.Fatalf("expected drain quit request, got %v", quitRequests[0])
	}
	for idx, tab := range mdl.tabs {
		if got := tab.status; got != taskMultiStatusCanceled {
			t.Fatalf("tab %d status = %q, want canceled", idx, got)
		}
	}
}

func TestMultiRunQuitKeyDetachAndCompletedQueueCloseImmediately(t *testing.T) {
	t.Parallel()

	detachOnly := multiRunModelForQuitTest()
	detachOnly.cfg.DetachOnly = true
	cmd := detachOnly.handleKey(keyText("q"))
	if cmd == nil {
		t.Fatal("expected detach-only queue to quit immediately")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected detach-only quit command, got %T", cmd())
	}

	completed := multiRunModelForQuitTest()
	completed.parentRun.Status = remoteRunStatusCompleted
	cmd = completed.handleKey(keyText("q"))
	if cmd == nil {
		t.Fatal("expected completed queue to quit immediately")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected completed queue quit command, got %T", cmd())
	}
}

func TestMultiRunQuitDialogNavigationEscapeAndForceEscalation(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	var quitRequests []uiQuitRequest
	mdl.onQuit = func(req uiQuitRequest) {
		quitRequests = append(quitRequests, req)
	}

	mdl.handleKey(keyText("q"))
	mdl.handleKey(keyText(keyLeft))
	if got := mdl.quitDialog.Selected; got != quitDialogActionCancel {
		t.Fatalf("expected left from default to wrap to cancel, got %v", got)
	}
	if cmd := mdl.handleKey(keyText("esc")); cmd != nil {
		t.Fatalf("expected escape to close dialog locally, got %T", cmd())
	}
	if mdl.quitDialog.Active {
		t.Fatal("expected escape to close quit dialog")
	}

	mdl.shutdown = shutdownState{Phase: shutdownPhaseDraining}
	cmd := mdl.handleKey(keyText("q"))
	if cmd == nil {
		t.Fatal("expected q during draining to escalate")
	}
	if _, ok := cmd().(drainMsg); !ok {
		t.Fatalf("expected force escalation drain message, got %T", cmd())
	}
	if got := quitRequests[len(quitRequests)-1]; got != uiQuitRequestForce {
		t.Fatalf("expected force quit request, got %v", got)
	}
}

func TestMultiRunPayloadFallbacksAndQueueCompletion(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleChildStarted,
		kinds.TaskRunMultiplePayload{
			Slug:       "beta",
			Index:      99,
			Status:     taskMultiStatusRunning,
			ChildRunID: "run-beta",
		},
	))
	if got := mdl.tabs[1].runID; got != "run-beta" {
		t.Fatalf("expected slug fallback to update beta run id, got %q", got)
	}
	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleQueueCompleted,
		kinds.TaskRunMultiplePayload{Status: taskMultiStatusCompleted, Total: 2},
	))
	if got := mdl.parentRun.Status; got != remoteRunStatusCompleted {
		t.Fatalf("parent status = %q, want completed", got)
	}
	if !mdl.isQueueComplete() {
		t.Fatal("expected terminal parent to mark queue complete")
	}
}

func TestAttachRemoteMultipleFollowsParentAndChildStreams(t *testing.T) {
	originalSetup := setupRemoteMultiRunUISession
	defer func() {
		setupRemoteMultiRunUISession = originalSetup
	}()

	var session *recordingMultiRunSession
	setupRemoteMultiRunUISession = func(ctx context.Context, mdl *multiRunModel) remoteWorkerSession {
		session = newRecordingMultiRunSession(ctx, mdl)
		return session
	}

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	parentStream := newBufferedClientRunStream(
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskRunMultipleChildStarted,
			kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Index:      0,
				Total:      2,
				Status:     taskMultiStatusRunning,
				ChildRunID: "run-alpha",
			},
		), 1, now)},
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Index:      0,
				Total:      2,
				Status:     taskMultiStatusCompleted,
				ChildRunID: "run-alpha",
			},
		), 2, now.Add(time.Second))},
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunCompleted,
			kinds.RunCompletedPayload{JobsTotal: 1, JobsSucceeded: 1},
		), 3, now.Add(2*time.Second))},
	)
	childStream := newBufferedClientRunStream(
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobQueued,
			kinds.JobQueuedPayload{Index: 0, TaskTitle: "Alpha child", SafeName: "alpha-child"},
		), 1, now)},
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobStarted,
			kinds.JobStartedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1}},
		), 2, now.Add(time.Second))},
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunCompleted,
			kinds.RunCompletedPayload{JobsTotal: 1, JobsSucceeded: 1},
		), 3, now.Add(2*time.Second))},
	)

	attached, err := AttachRemoteMultiple(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusQueued},
				{Slug: "beta", Status: taskMultiStatusQueued},
			},
		},
		LoadChildSnapshot: func(_ context.Context, runID string) (apicore.RunSnapshot, error) {
			return apicore.RunSnapshot{
				Run: apicore.Run{RunID: runID, WorkflowSlug: "alpha", Status: remoteRunStatusRunning},
			}, nil
		},
		OpenParentStream: func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error) {
			return parentStream, nil
		},
		OpenChildStream: func(_ context.Context, runID string, _ apicore.StreamCursor) (apiclient.RunStream, error) {
			if runID != "run-alpha" {
				t.Fatalf("unexpected child stream run id %q", runID)
			}
			return childStream, nil
		},
	})
	if err != nil {
		t.Fatalf("AttachRemoteMultiple() error = %v", err)
	}
	if attached == nil {
		t.Fatal("expected attached multi-run session")
	}
	if err := session.Wait(); err != nil {
		t.Fatalf("recording session wait: %v", err)
	}

	session.withModel(func(mdl *multiRunModel) {
		if got := mdl.tabs[0].status; got != taskMultiStatusCompleted {
			t.Fatalf("alpha status = %q, want completed", got)
		}
		if got := mdl.tabs[0].runID; got != "run-alpha" {
			t.Fatalf("alpha run id = %q, want run-alpha", got)
		}
		if mdl.tabs[0].child == nil {
			t.Fatal("expected alpha child model")
		}
		if got := mdl.tabs[0].child.jobs[0].taskTitle; got != "Alpha child" {
			t.Fatalf("alpha child title = %q, want Alpha child", got)
		}
	})
}

func TestRemoteMultiRunReattachRestoresCompletedActiveAndQueuedTabs(t *testing.T) {
	t.Parallel()

	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{
				{Slug: "alpha", Status: taskMultiStatusCompleted, RunID: "run-alpha"},
				{Slug: "beta", Status: taskMultiStatusRunning, RunID: "run-beta"},
				{Slug: "gamma", Status: taskMultiStatusQueued},
			},
		},
		LoadChildSnapshot: func(_ context.Context, runID string) (apicore.RunSnapshot, error) {
			switch runID {
			case "run-alpha":
				return childSnapshotForTest(t, runID, "alpha", remoteRunStatusCompleted, "alpha done"), nil
			case "run-beta":
				return childSnapshotForTest(t, runID, "beta", remoteRunStatusRunning, "beta work"), nil
			default:
				t.Fatalf("unexpected child snapshot run id %q", runID)
				return apicore.RunSnapshot{}, nil
			}
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}

	if got := mdl.activeTab; got != 1 {
		t.Fatalf("expected active tab to restore running beta, got %d", got)
	}
	if mdl.tabs[0].child == nil || mdl.tabs[1].child == nil {
		t.Fatalf("expected completed and running child models, got %#v", mdl.tabs)
	}
	if mdl.tabs[2].child != nil {
		t.Fatal("expected queued gamma tab to have no child model yet")
	}
	if got := mdl.tabs[0].child.jobs[0].snapshot.Entries[0].Preview; got != "alpha done" {
		t.Fatalf("expected alpha transcript restored, got %q", got)
	}
	if got := mdl.tabs[1].child.jobs[0].snapshot.Entries[0].Preview; got != "beta work" {
		t.Fatalf("expected beta transcript restored, got %q", got)
	}
}

func TestMultiRunControllerLifecycleCoversSessionMethods(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	done := make(chan error)
	close(done)
	ctx, cancel := context.WithCancel(context.Background())
	ctrl := &multiRunController{model: mdl, done: done, ctx: ctx, cancel: cancel}
	quitCalls := 0
	ctrl.SetQuitHandler(func(uiQuitRequest) {
		quitCalls++
	})
	ctrl.requestQuit(uiQuitRequestDrain)
	ctrl.Enqueue(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleStarted,
		kinds.TaskRunMultiplePayload{Slugs: []string{"alpha", "beta"}, Total: 2, Status: taskMultiStatusRunning},
	))
	ctrl.StartRemoteWorker(func(context.Context) {})
	ctrl.CloseEvents()
	ctrl.Shutdown()
	if err := ctrl.Wait(); err != nil {
		t.Fatalf("multi-run controller wait: %v", err)
	}
	if quitCalls != 1 {
		t.Fatalf("quit handler calls = %d, want 1", quitCalls)
	}
}

func TestNewMultiRunControllerRunsWithInjectedProgram(t *testing.T) {
	originalProgram := newMultiRunTeaProgram
	defer func() {
		newMultiRunTeaProgram = originalProgram
	}()
	newMultiRunTeaProgram = func(mdl tea.Model) *tea.Program {
		return tea.NewProgram(mdl, tea.WithoutSignalHandler(), tea.WithoutRenderer(), tea.WithInput(nil))
	}

	session := newMultiRunController(context.Background(), multiRunModelForQuitTest())
	ctrl, ok := session.(*multiRunController)
	if !ok {
		t.Fatalf("expected multiRunController, got %T", session)
	}
	ctrl.Enqueue(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleStarted,
		kinds.TaskRunMultiplePayload{Slugs: []string{"alpha", "beta"}, Total: 2, Status: taskMultiStatusRunning},
	))
	ctrl.Shutdown()
	if err := ctrl.Wait(); err != nil {
		t.Fatalf("multi-run controller wait: %v", err)
	}
}

func TestMultiRunQuitDialogRenderAndCancelAction(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	mdl.handleKey(keyText("q"))
	if !mdl.quitDialog.Active {
		t.Fatal("expected quit dialog to open")
	}
	view := mdl.View().Content
	if !strings.Contains(view, "Leave Active Queue?") || !strings.Contains(view, "Stop Run") {
		t.Fatalf("expected queue quit dialog, got %q", view)
	}

	mdl.handleKey(keyText(keyRight))
	mdl.handleKey(keyText(keyRight))
	if got := mdl.quitDialog.Selected; got != quitDialogActionCancel {
		t.Fatalf("expected cancel action selected, got %v", got)
	}
	if cmd := mdl.handleKey(keyCode(tea.KeyEnter)); cmd != nil {
		t.Fatalf("expected Cancel action to return to UI without command, got %T", cmd())
	}
	if mdl.quitDialog.Active {
		t.Fatal("expected Cancel action to close dialog")
	}
	if mdl.shutdown.Active() {
		t.Fatalf("expected Cancel action not to start shutdown, got %#v", mdl.shutdown)
	}
}

func TestMultiRunParentStartedAndQueueCanceledEvents(t *testing.T) {
	t.Parallel()

	mdl := &multiRunModel{
		parentRun:  apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
		width:      120,
		height:     30,
		cfg:        &config{},
		quitDialog: newQuitDialogState(),
	}
	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleStarted,
		kinds.TaskRunMultiplePayload{
			Status: taskMultiStatusRunning,
			Slugs:  []string{"alpha", "beta"},
			Total:  2,
		},
	))
	if len(mdl.tabs) != 2 {
		t.Fatalf("expected started event to create two tabs, got %#v", mdl.tabs)
	}
	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskRunMultipleQueueCanceled,
		kinds.TaskRunMultiplePayload{Status: taskMultiStatusCanceled, Error: "stop requested"},
	))
	for idx, tab := range mdl.tabs {
		if tab.status != taskMultiStatusCanceled || tab.errorText != "stop requested" {
			t.Fatalf("tab %d after queue cancel = %#v", idx, tab)
		}
	}
	if got := mdl.parentRun.Status; got != remoteRunStatusCanceled {
		t.Fatalf("parent status = %q, want canceled", got)
	}
}

func TestMultiRunChildEventCreatesPlaceholderAndMapsTerminalRunStatus(t *testing.T) {
	t.Parallel()

	mdl := &multiRunModel{
		parentRun:  apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
		width:      120,
		height:     30,
		cfg:        &config{},
		quitDialog: newQuitDialogState(),
		tabs: []multiRunTab{{
			slug:       "alpha",
			status:     taskMultiStatusRunning,
			runID:      "run-alpha",
			translator: newUIEventTranslator(),
		}},
	}

	mdl.handleChildEvent(multiRunChildEventMsg{
		RunID: "run-alpha",
		Event: mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobQueued,
			kinds.JobQueuedPayload{Index: 0, SafeName: "alpha-job", TaskTitle: "Alpha"},
		),
	})
	if mdl.tabs[0].child == nil {
		t.Fatal("expected child event to create placeholder child model")
	}
	if got := mdl.tabs[0].child.jobs[0].taskTitle; got != "Alpha" {
		t.Fatalf("child task title = %q, want Alpha", got)
	}

	mdl.handleChildEvent(multiRunChildEventMsg{
		RunID: "run-alpha",
		Event: mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunFailed,
			kinds.RunFailedPayload{Error: "child failed"},
		),
	})
	if got := mdl.tabs[0].status; got != taskMultiStatusFailed {
		t.Fatalf("child terminal status = %q, want failed", got)
	}
}

func TestMultiRunUpdateRoutesResizeTicksParentAndChildMessages(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	if _, cmd := mdl.Update(tea.WindowSizeMsg{Width: 100, Height: 28}); cmd != nil {
		t.Fatalf("expected resize to stay local, got %T", cmd())
	}
	if got := mdl.tabs[0].child.width; got != 100 {
		t.Fatalf("child width = %d, want 100", got)
	}
	mdl.Update(clockTickMsg{at: time.Now()})
	mdl.Update(spinnerTickMsg{at: time.Now()})
	mdl.Update(multiRunChildBootstrapMsg{
		RunID:    "run-alpha",
		Snapshot: childSnapshotForTest(t, "run-alpha", "alpha", remoteRunStatusCompleted, "done"),
	})
	if got := mdl.tabs[0].status; got != taskMultiStatusCompleted {
		t.Fatalf("bootstrap status = %q, want completed", got)
	}
	mdl.Update(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindRunFailed,
		kinds.RunFailedPayload{Error: "parent failed"},
	))
	if got := mdl.parentRun.Status; got != remoteRunStatusFailed {
		t.Fatalf("parent status = %q, want failed", got)
	}
}

func TestAttachRemoteMultipleReturnsSetupAndParentStreamErrors(t *testing.T) {
	originalSetup := setupRemoteMultiRunUISession
	defer func() {
		setupRemoteMultiRunUISession = originalSetup
	}()

	setupRemoteMultiRunUISession = func(context.Context, *multiRunModel) remoteWorkerSession {
		return nil
	}
	_, err := AttachRemoteMultiple(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "remote multi-run ui session is required") {
		t.Fatalf("AttachRemoteMultiple nil setup error = %v", err)
	}

	setupRemoteMultiRunUISession = func(ctx context.Context, mdl *multiRunModel) remoteWorkerSession {
		return newRecordingMultiRunSession(ctx, mdl)
	}
	_, err = AttachRemoteMultiple(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
		},
		OpenParentStream: func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error) {
			return nil, context.Canceled
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AttachRemoteMultiple stream error = %v, want context.Canceled wrapper", err)
	}
}

func TestMultiRunStatusHelpersCoverAllStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
		taskMultiStatusQueued,
		taskMultiStatusRunning,
		taskMultiStatusCompleted,
		taskMultiStatusFailed,
		taskMultiStatusCanceled,
		"unknown",
	} {
		if multiRunStatusColor(status) == nil {
			t.Fatalf("expected color for status %q", status)
		}
	}
	for _, status := range []string{
		remoteRunStatusRunning,
		remoteRunStatusRetrying,
		remoteRunStatusCompleted,
		remoteRunStatusFailed,
		remoteRunStatusCrashed,
		remoteRunStatusCanceled,
		"pending",
	} {
		_ = taskMultiStatusFromRunStatus(status)
	}
	for _, kind := range []eventspkg.EventKind{
		eventspkg.EventKindRunCompleted,
		eventspkg.EventKindRunFailed,
		eventspkg.EventKindRunCancelled,
		eventspkg.EventKindJobQueued,
	} {
		_ = taskMultiStatusFromChildRunEvent(kind)
	}
}

func TestMultiRunNilAndNarrowHelperBranches(t *testing.T) {
	t.Parallel()

	var nilController *multiRunController
	nilController.SetQuitHandler(nil)
	nilController.StartRemoteWorker(nil)
	nilController.Shutdown()

	var nilModel *multiRunModel
	if nilModel.activeChild() != nil {
		t.Fatal("expected nil model active child to be nil")
	}

	mdl := multiRunModelForQuitTest()
	if got := tabStatus(nil); got != taskMultiStatusQueued {
		t.Fatalf("nil tab status = %q, want queued", got)
	}
	if actions := mdl.renderQuitDialogActions(20, colorBgSurface); !strings.Contains(actions, "\n") {
		t.Fatalf("expected narrow quit actions to stack vertically, got %q", actions)
	}
	mdl.activeTab = -1
	if mdl.activeTabState() != nil {
		t.Fatal("expected invalid active tab to return nil")
	}
	mdl.resizeChild(nil)
}

func multiRunModelForQuitTest() *multiRunModel {
	mdl := &multiRunModel{
		parentRun:  apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
		width:      120,
		height:     30,
		cfg:        &config{},
		quitDialog: newQuitDialogState(),
		now:        time.Now(),
		tabs: []multiRunTab{
			{slug: "alpha", status: taskMultiStatusRunning, runID: "run-alpha", translator: newUIEventTranslator()},
			{slug: "beta", status: taskMultiStatusQueued, translator: newUIEventTranslator()},
		},
	}
	mdl.tabs[0].child = childModelFromRunSnapshot(apicore.RunSnapshot{
		Run: apicore.Run{RunID: "run-alpha", WorkflowSlug: "alpha", Status: remoteRunStatusRunning},
	}, mdl.cfg, mdl.childWidth(), mdl.childHeight())
	return mdl
}

func childSnapshotForTest(
	t *testing.T,
	runID string,
	slug string,
	status string,
	preview string,
) apicore.RunSnapshot {
	t.Helper()
	return apicore.RunSnapshot{
		Run: apicore.Run{RunID: runID, WorkflowSlug: slug, Status: status},
		Jobs: []apicore.RunJobState{{
			Index:  0,
			JobID:  slug + "-job",
			Status: status,
			Summary: &apicore.RunJobSummary{
				TaskTitle: slug,
				SafeName:  slug,
				Session: apiSessionSnapshot(buildSnapshotWithEntries(t, TranscriptEntry{
					ID:      slug + "-entry",
					Kind:    transcriptEntryAssistantMessage,
					Title:   "Assistant",
					Preview: preview,
					Blocks:  []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: preview})},
				})),
			},
		}},
	}
}

type recordingMultiRunSession struct {
	mu          sync.Mutex
	model       *multiRunModel
	ctx         context.Context
	cancel      context.CancelFunc
	workers     sync.WaitGroup
	quitHandler func(uiQuitRequest)
}

func newRecordingMultiRunSession(parent context.Context, mdl *multiRunModel) *recordingMultiRunSession {
	ctx, cancel := context.WithCancel(parent)
	mdl.onQuit = func(uiQuitRequest) {}
	return &recordingMultiRunSession{model: mdl, ctx: ctx, cancel: cancel}
}

func (s *recordingMultiRunSession) Enqueue(msg any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model.Update(msg)
}

func (s *recordingMultiRunSession) SetQuitHandler(fn func(uiQuitRequest)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quitHandler = fn
}

func (s *recordingMultiRunSession) CloseEvents() {}

func (s *recordingMultiRunSession) Shutdown() {
	s.cancel()
}

func (s *recordingMultiRunSession) Wait() error {
	s.workers.Wait()
	return nil
}

func (s *recordingMultiRunSession) StartRemoteWorker(fn func(context.Context)) {
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		fn(s.ctx)
	}()
}

func (s *recordingMultiRunSession) withModel(fn func(*multiRunModel)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.model)
}
