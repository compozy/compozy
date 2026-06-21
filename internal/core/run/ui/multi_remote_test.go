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
	xansi "github.com/charmbracelet/x/ansi"
)

func TestMultiRunInitialSnapshotRendersQueuedTabsInOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should render queued tabs in request order", func(t *testing.T) {
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
	})
}

func TestRemoteMultiRunModelAppliesWorkspaceRootToParentAndChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should apply the trimmed workspace root to the parent and inherited children", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusRunning, RunID: "run-alpha"},
				},
			},
			WorkspaceRoot: "  /tmp/compozy-parent  ",
			LoadChildSnapshot: func(_ context.Context, runID string) (apicore.RunSnapshot, error) {
				return childSnapshotForTest(t, runID, "alpha", remoteRunStatusRunning, "alpha transcript"), nil
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}
		if mdl.cfg.WorkspaceRoot != "/tmp/compozy-parent" {
			t.Fatalf("parent workspace root = %q, want trimmed workspace root", mdl.cfg.WorkspaceRoot)
		}
		if len(mdl.tabs) != 1 || mdl.tabs[0].child == nil {
			t.Fatalf("expected hydrated child tab, got %#v", mdl.tabs)
		}
		if got := mdl.tabs[0].child.cfg.WorkspaceRoot; got != "/tmp/compozy-parent" {
			t.Fatalf("child workspace root = %q, want inherited workspace root", got)
		}
	})
}

func TestMultiRunChildStartUpdatesOnlyTargetTabState(t *testing.T) {
	t.Parallel()

	t.Run("Should update only the target tab when a child starts", func(t *testing.T) {
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
	})
}

func TestMultiRunCompletedTabRemainsNavigableAfterActiveAdvances(t *testing.T) {
	t.Parallel()

	t.Run("Should keep completed tab navigable after active tab advances", func(t *testing.T) {
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
	})
}

func TestMultiRunInitStartsClockWhenActiveTabHasNoChild(t *testing.T) {
	t.Parallel()

	mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
		Snapshot: apicore.TaskRunMultipleSnapshot{
			Run:   apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			Items: []apicore.TaskRunMultipleItem{{Slug: "alpha", Status: taskMultiStatusQueued}},
		},
	})
	if err != nil {
		t.Fatalf("newRemoteMultiRunModel() error = %v", err)
	}

	if cmd := mdl.Init(); cmd == nil {
		t.Fatal("expected multi-run init to start clock ticks without an active child")
	}
}

func TestMultiRunChildBootstrapRestartsSpinnerLoop(t *testing.T) {
	t.Parallel()

	t.Run("Should restart spinner loop after child bootstrap", func(t *testing.T) {
		t.Parallel()

		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{{
					Slug:   "alpha",
					RunID:  "run-alpha",
					Status: taskMultiStatusRunning,
				}},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}

		_, cmd := mdl.Update(multiRunChildBootstrapMsg{
			RunID: "run-alpha",
			Snapshot: apicore.RunSnapshot{
				Run: apicore.Run{RunID: "run-alpha", Status: remoteRunStatusRunning},
			},
		})
		if cmd == nil {
			t.Fatal("expected child bootstrap to restart spinner loop")
		}
		if !mdl.spinnerRunning {
			t.Fatal("expected spinner loop to be marked running")
		}
	})
}

func TestMultiRunClockTickContinuesAfterAdvancingToQueuedTab(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	mdl.handleChildEvent(multiRunChildEventMsg{
		RunID: "run-alpha",
		Event: mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunCompleted,
			kinds.RunCompletedPayload{JobsTotal: 1, JobsSucceeded: 1},
		),
	})
	if got := mdl.activeTab; got != 1 {
		t.Fatalf("expected active tab to advance to queued beta, got %d", got)
	}

	if _, cmd := mdl.Update(clockTickMsg{at: time.Unix(1, 0)}); cmd == nil {
		t.Fatal("expected clock loop to continue after the active tab advances to a queued child")
	}
}

func TestMultiRunTabNavigationDoesNotCycleChildPaneFocus(t *testing.T) {
	t.Parallel()

	t.Run("Should change tabs without cycling child pane focus", func(t *testing.T) {
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

		// Tab navigation may (re)start the queue-owned spinner loop, but it must
		// not drive the child models or cycle their pane focus.
		mdl.handleKey(keyCode(tea.KeyRight))
		if got := mdl.activeTab; got != 1 {
			t.Fatalf("expected active tab beta, got %d", got)
		}
		if got := mdl.tabs[0].child.focusedPane; got != uiPaneTimeline {
			t.Fatalf("expected alpha child focus to remain timeline, got %s", got)
		}
		if got := mdl.tabs[1].child.focusedPane; got != uiPaneJobs {
			t.Fatalf("expected beta child focus to remain jobs, got %s", got)
		}
	})
}

func TestMultiRunSpinnerSurvivesIdleActiveTab(t *testing.T) {
	t.Parallel()

	t.Run("Should keep ticking while any tab runs even on an idle active tab", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusRunning, RunID: "run-alpha"},
					{Slug: "beta", Status: taskMultiStatusQueued},
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

		// Switch to the queued tab, which has no active jobs of its own. A tick
		// handled here must still re-arm the loop because alpha is still running.
		// Previously the loop's continuation was delegated to the active child, so
		// this returned nil and the spinner froze permanently.
		mdl.activeTab = 1
		mdl.spinnerRunning = true
		if cmd := mdl.handleSpinnerTick(spinnerTickMsg{}); cmd == nil {
			t.Fatal("spinner must keep ticking while any tab runs, even on an idle active tab")
		}
	})
}

func TestMultiRunSpinnerStopsWhenNoTabActive(t *testing.T) {
	t.Parallel()

	t.Run("Should not start the spinner loop while every tab is queued", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusQueued},
					{Slug: "beta", Status: taskMultiStatusQueued},
				},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}
		if cmd := mdl.ensureSpinnerTick(); cmd != nil {
			t.Fatal("spinner loop must not start while every tab is queued/idle")
		}
	})
}

func TestMultiRunTabNavigationUsesHorizontalKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should navigate tabs with horizontal keys", func(t *testing.T) {
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
	})
}

func TestMultiRunTabsShowBrandOnce(t *testing.T) {
	t.Parallel()

	t.Run("Should render the brand on the tabs row exactly once", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusRunning},
					{Slug: "beta", Status: taskMultiStatusQueued},
				},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}

		tabs := mdl.renderTabs()
		if got := strings.Count(tabs, "COMPOZY"); got != 1 {
			t.Fatalf("expected the brand to share the tabs row exactly once, got %d in %q", got, tabs)
		}
	})
}

func TestMultiRunTabsRenderStatusGlyphs(t *testing.T) {
	t.Parallel()

	t.Run("Should prefix each tab with its status glyph", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusRunning},
					{Slug: "beta", Status: taskMultiStatusQueued},
					{Slug: "gamma", Status: taskMultiStatusCompleted},
					{Slug: "delta", Status: taskMultiStatusFailed},
				},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}

		view := mdl.renderTabs()
		for _, want := range []string{glyphActiveDot, jobIconPending, jobIconSuccess, jobIconFailed} {
			if !strings.Contains(view, want) {
				t.Fatalf("expected tab strip to contain status glyph %q, got %q", want, view)
			}
		}
	})
}

func TestMultiRunCloseTUIDoesNotRequestParentCancel(t *testing.T) {
	t.Parallel()

	t.Run("Should quit without requesting parent cancel when closing TUI", func(t *testing.T) {
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
	})
}

func TestMultiRunStopRunRequestsParentCancelOnceAndMarksQueuedCanceled(t *testing.T) {
	t.Parallel()

	t.Run("Should request parent cancel once and mark queued tabs canceled", func(t *testing.T) {
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
	})
}

func TestMultiRunQuitKeyDetachAndCompletedQueueCloseImmediately(t *testing.T) {
	t.Parallel()

	t.Run("Should close immediately for detach-only and completed queues", func(t *testing.T) {
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
	})
}

func TestMultiRunQuitDialogNavigationEscapeAndForceEscalation(t *testing.T) {
	t.Parallel()

	t.Run("Should support quit dialog navigation escape and force escalation", func(t *testing.T) {
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
	})
}

func TestMultiRunPayloadFallbacksAndQueueCompletion(t *testing.T) {
	t.Parallel()

	t.Run("Should apply payload fallbacks and mark queue completion", func(t *testing.T) {
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
	})
}

func TestAttachRemoteMultipleFollowsParentAndChildStreams(t *testing.T) {
	t.Run("Should follow both parent and child event streams", func(t *testing.T) {
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

func TestMultiRunForwardsParallelEventsToActiveChild(t *testing.T) {
	t.Parallel()

	t.Run("Should seed all aggregate tasks from the parent plan event", func(t *testing.T) {
		t.Parallel()

		mdl := newParallelAggregateStartedModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelPlanStarted,
			kinds.TaskParallelPlanPayload{
				Workflow:          "alpha",
				IntegrationBranch: "compozy/parallel-x",
				ParallelLimit:     2,
				Tasks: []kinds.TaskParallelPlanTask{
					{ID: "task_01", Number: 1, Title: "Task 1", File: "task_01.md", WaveIndex: 0},
					{
						ID:           "task_02",
						Number:       2,
						Title:        "Task 2",
						File:         "task_02.md",
						Dependencies: []string{"task_01"},
						WaveIndex:    1,
					},
				},
				Waves: []kinds.TaskParallelPlanWave{
					{Index: 0, TaskIDs: []string{"task_01"}},
					{Index: 1, TaskIDs: []string{"task_02"}},
				},
			},
		))

		child := mdl.tabs[0].child
		if child == nil || len(child.jobs) != 2 {
			t.Fatalf("plan-seeded aggregate jobs = %#v, want two jobs", child)
		}
		for idx := range child.jobs {
			if child.jobs[idx].state != jobPending {
				t.Fatalf("plan-seeded job %d state = %v, want pending", idx, child.jobs[idx].state)
			}
		}
		view := xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "WAVE 1") || !strings.Contains(view, "WAVE 2") ||
			!strings.Contains(view, "Task 1") || !strings.Contains(view, "Task 2") {
			t.Fatalf("expected seeded aggregate plan view, got:\n%s", view)
		}
	})

	t.Run("Should render parent parallel events without a child run id", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		if child == nil || child.parallel == nil {
			t.Fatal("expected parent parallel event to create an aggregate parallel child")
		}
		if !mdl.tabs[0].aggregateChild {
			t.Fatal("expected aggregate child marker while no real child run id exists")
		}
		if mdl.tabs[0].status != taskMultiStatusRunning {
			t.Fatalf("aggregate tab status = %q, want running after parent parallel event", mdl.tabs[0].status)
		}
		if len(child.jobs) != 1 || child.jobs[0].taskNumber != 1 {
			t.Fatalf("aggregate jobs = %#v, want only task_01", child.jobs)
		}
		if got := child.parallel.integrationBranch; got != "compozy/parallel-x" {
			t.Fatalf("integration branch = %q, want compozy/parallel-x", got)
		}
		view := xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "WAVE 1") || !strings.Contains(view, "task_01") {
			t.Fatalf("expected aggregate parallel view with wave and task, got:\n%s", view)
		}
		if strings.Contains(view, "Parallel task running") {
			t.Fatalf("aggregate parallel view must not synthesize a fake task transcript, got:\n%s", view)
		}
		if strings.Contains(view, "QUEUE.QUEUED") {
			t.Fatalf("parallel-tasks tab must not stay on queued placeholder, got:\n%s", view)
		}

		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelConflictDetected,
			kinds.TaskParallelPayload{
				WaveIndex:     0,
				TaskID:        "task_01",
				ConflictFiles: []string{"story.txt"},
				Attempt:       1,
				MaxAttempts:   3,
			},
		))
		if child.parallel.conflict == nil {
			t.Fatal("expected conflict to be recorded on the child parallel view")
		}
		if !child.parallel.expanded() {
			t.Fatal("expected the INTEGRATION pane to expand on conflict")
		}

		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelMerged,
			kinds.TaskParallelPayload{WaveIndex: 0, TaskID: "task_01", Status: "merged"},
		))
		if child.jobs[0].state != jobSuccess {
			t.Fatalf("aggregate task state = %v, want success after merged event", child.jobs[0].state)
		}
		if child.hasActiveJobs() {
			t.Fatal("aggregate child must stop spinning after merged event")
		}
		if got := child.jobs[0].snapshot.Entries[0].Title; got != "Parallel task completed: task_01" {
			t.Fatalf("aggregate task notice = %q, want completed", got)
		}
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskRunMultipleQueueCompleted,
			kinds.TaskRunMultiplePayload{Status: taskMultiStatusCompleted, Total: 1},
		))
		if mdl.tabs[0].status != taskMultiStatusCompleted || !mdl.tabs[0].terminal {
			t.Fatalf("tab after queue completion = %#v", mdl.tabs[0])
		}
	})

	t.Run("Should complete active aggregate jobs when parent run completes", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunCompleted,
			kinds.RunCompletedPayload{JobsTotal: 1, JobsSucceeded: 1},
		))

		if mdl.tabs[0].status != taskMultiStatusCompleted || !mdl.tabs[0].terminal {
			t.Fatalf("tab after parent completion = %#v", mdl.tabs[0])
		}
		if child.jobs[0].state != jobSuccess {
			t.Fatalf("aggregate task state = %v, want success after parent completion", child.jobs[0].state)
		}
		if child.hasActiveJobs() {
			t.Fatal("aggregate child must stop spinning after parent completion")
		}
		if got := child.jobs[0].snapshot.Entries[0].Title; got != "Parallel task completed: task_01" {
			t.Fatalf("aggregate task notice = %q, want completed", got)
		}
	})

	t.Run("Should fail aggregate jobs on rollback without a task id", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelRolledBack,
			kinds.TaskParallelPayload{WaveIndex: 0},
		))

		if child.jobs[0].state != jobFailed {
			t.Fatalf("aggregate task state = %v, want failed after rollback", child.jobs[0].state)
		}
		if child.hasActiveJobs() {
			t.Fatal("aggregate child must stop spinning after rollback")
		}
		if mdl.tabs[0].status != taskMultiStatusFailed || !mdl.tabs[0].terminal {
			t.Fatalf("tab after rollback = %#v", mdl.tabs[0])
		}
		if got := child.jobs[0].snapshot.Entries[0].Title; got != "Parallel task stopped: task_01" {
			t.Fatalf("aggregate task notice = %q, want stopped", got)
		}
	})

	t.Run("Should fail aggregate jobs when parent run fails", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunFailed,
			kinds.RunFailedPayload{Error: "merge failed"},
		))

		if mdl.tabs[0].status != taskMultiStatusFailed || !mdl.tabs[0].terminal {
			t.Fatalf("tab after parent failure = %#v", mdl.tabs[0])
		}
		if child.jobs[0].state != jobFailed {
			t.Fatalf("aggregate task state = %v, want failed after parent run failure", child.jobs[0].state)
		}
		if child.hasActiveJobs() {
			t.Fatal("aggregate child must stop spinning after parent run failure")
		}
		if mdl.tabs[0].errorText != "merge failed" {
			t.Fatalf("tab failure text = %q, want merge failed", mdl.tabs[0].errorText)
		}
		if got := child.jobs[0].snapshot.Entries[0].Title; got != "Parallel task stopped: task_01" {
			t.Fatalf("aggregate task notice = %q, want stopped", got)
		}
	})

	t.Run("Should bind parallel task child snapshot to the aggregate task row", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{
				WaveIndex:         0,
				WaveTotal:         1,
				TaskID:            "task_01",
				ChildRunID:        "child-task-01",
				WorktreePath:      "/tmp/task-01",
				IntegrationBranch: "compozy/parallel-x",
			},
		))
		if _, ok := mdl.parallelChildBinding("child-task-01"); !ok {
			t.Fatal("expected child run id to be bound to the aggregate task row")
		}

		mdl.handleChildBootstrap(multiRunChildBootstrapMsg{
			RunID: "child-task-01",
			Snapshot: childSnapshotForTest(
				t,
				"child-task-01",
				"task_01",
				remoteRunStatusRunning,
				"real task transcript",
			),
		})
		if child.jobs[0].state != jobRunning {
			t.Fatalf("aggregate task state = %v, want running from child snapshot", child.jobs[0].state)
		}
		if got := child.jobs[0].snapshot.Entries[0].Preview; got != "real task transcript" {
			t.Fatalf("task transcript = %q, want real child transcript", got)
		}
		view := xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "real task transcript") {
			t.Fatalf("expected selected task to render child transcript, got:\n%s", view)
		}
		if strings.Contains(view, "Parallel task running") {
			t.Fatalf("child-bound task must not show synthetic parallel transcript, got:\n%s", view)
		}
	})

	t.Run("Should route parent recovery events to the bound parallel task", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01", ChildRunID: "child-task-01"},
		))
		mdl.handleParentEvent(mustRuntimeEventWithRunIDUITest(
			t,
			"parent-run",
			eventspkg.EventKindRunRecoveryStarted,
			kinds.RunRecoveryStartedPayload{Attempt: 1, Strategy: "agentic", RecoveryRunID: "child-task-01"},
		))
		if child.jobs[0].state != jobRetrying || !child.jobs[0].retrying {
			t.Fatalf("job after recovery_started = %#v, want retrying", child.jobs[0])
		}
		if !strings.Contains(child.jobs[0].retryReason, "agentic") {
			t.Fatalf("retry reason = %q, want strategy", child.jobs[0].retryReason)
		}
		mdl.handleParentEvent(mustRuntimeEventWithRunIDUITest(
			t,
			"child-task-01",
			eventspkg.EventKindRunRecovered,
			kinds.RunRecoveredPayload{Attempts: 1},
		))
		if child.jobs[0].state != jobSuccess {
			t.Fatalf("job after run.recovered = %v, want succeeded", child.jobs[0].state)
		}

		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01", ChildRunID: "child-task-02"},
		))
		mdl.handleParentEvent(mustRuntimeEventWithRunIDUITest(
			t,
			"parent-run",
			eventspkg.EventKindRunRecoveryExhausted,
			kinds.RunRecoveryExhaustedPayload{Error: "still failing", RecoveryRunID: "child-task-02"},
		))
		if child.jobs[0].state != jobFailed {
			t.Fatalf("job after recovery_exhausted = %v, want failed", child.jobs[0].state)
		}
		if len(child.failures) == 0 || !strings.Contains(child.failures[0].Err.Error(), "still failing") {
			t.Fatalf("failures after recovery_exhausted = %#v, want recovery error", child.failures)
		}
	})

	t.Run("Should drop stale child bindings when a parallel task restarts", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01", ChildRunID: "child-task-old"},
		))
		mdl.handleChildBootstrap(multiRunChildBootstrapMsg{
			RunID:    "child-task-old",
			Snapshot: childSnapshotForTest(t, "child-task-old", "task_01", remoteRunStatusRunning, "old transcript"),
		})
		if got := child.jobs[0].snapshot.Entries[0].Preview; got != "old transcript" {
			t.Fatalf("old child transcript = %q, want old transcript", got)
		}

		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01", ChildRunID: "child-task-new"},
		))
		if _, ok := mdl.parallelChildBinding("child-task-old"); ok {
			t.Fatal("old child binding remained after task restart")
		}
		if _, ok := mdl.parallelChildBinding("child-task-new"); !ok {
			t.Fatal("new child binding missing after task restart")
		}
		mdl.handleChildBootstrap(multiRunChildBootstrapMsg{
			RunID:    "child-task-new",
			Snapshot: childSnapshotForTest(t, "child-task-new", "task_01", remoteRunStatusRunning, "new transcript"),
		})
		mdl.handleChildBootstrap(multiRunChildBootstrapMsg{
			RunID: "child-task-old",
			Snapshot: childSnapshotForTest(
				t,
				"child-task-old",
				"task_01",
				remoteRunStatusRunning,
				"late old transcript",
			),
		})
		if got := child.jobs[0].snapshot.Entries[0].Preview; got != "new transcript" {
			t.Fatalf("task transcript after stale event = %q, want new transcript", got)
		}
	})

	t.Run("Should keep concurrent child transcripts isolated by task selection", func(t *testing.T) {
		mdl := newParallelAggregateStartedModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelPlanStarted,
			kinds.TaskParallelPlanPayload{
				Workflow:          "alpha",
				IntegrationBranch: "compozy/parallel-x",
				ParallelLimit:     2,
				Tasks: []kinds.TaskParallelPlanTask{
					{ID: "task_01", Number: 1, Title: "Task 1", File: "task_01.md", WaveIndex: 0},
					{ID: "task_02", Number: 2, Title: "Task 2", File: "task_02.md", WaveIndex: 0},
				},
				Waves: []kinds.TaskParallelPlanWave{{Index: 0, TaskIDs: []string{"task_01", "task_02"}}},
			},
		))
		for _, task := range []struct {
			id      string
			childID string
			text    string
		}{
			{id: "task_01", childID: "child-task-01", text: "first task transcript"},
			{id: "task_02", childID: "child-task-02", text: "second task transcript"},
		} {
			mdl.handleParentEvent(mustRuntimeEventUITest(
				t,
				eventspkg.EventKindTaskParallelTaskStarted,
				kinds.TaskParallelPayload{WaveIndex: 0, WaveTotal: 1, TaskID: task.id, ChildRunID: task.childID},
			))
			mdl.handleChildBootstrap(multiRunChildBootstrapMsg{
				RunID:    task.childID,
				Snapshot: childSnapshotForTest(t, task.childID, task.id, remoteRunStatusRunning, task.text),
			})
		}
		child := mdl.tabs[0].child
		if child == nil || len(child.jobs) != 2 {
			t.Fatalf("aggregate child jobs = %#v, want two task rows", child)
		}
		child.selectedJob = 0
		view := xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "first task transcript") || strings.Contains(view, "second task transcript") {
			t.Fatalf("task 1 selection rendered wrong transcript:\n%s", view)
		}
		mdl.handleKey(keyText("j"))
		view = xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "second task transcript") || strings.Contains(view, "first task transcript") {
			t.Fatalf("task 2 selection rendered wrong transcript:\n%s", view)
		}
	})

	t.Run("Should advance spinner frames while a bound parallel child is running", func(t *testing.T) {
		mdl, child := newParallelAggregateMultiRunTestModel(t)
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskParallelTaskStarted,
			kinds.TaskParallelPayload{WaveIndex: 0, WaveTotal: 1, TaskID: "task_01", ChildRunID: "child-task-01"},
		))
		before := child.frame
		mdl.spinnerRunning = true
		if cmd := mdl.handleSpinnerTick(spinnerTickMsg{at: mdl.now.Add(uiSpinnerTickInterval)}); cmd == nil {
			t.Fatal("expected spinner loop to continue while bound parallel child is running")
		}
		if child.frame == before {
			t.Fatalf("spinner frame did not advance: before=%d after=%d", before, child.frame)
		}
	})
}

func newParallelAggregateMultiRunTestModel(t *testing.T) (*multiRunModel, *uiModel) {
	t.Helper()

	mdl := newParallelAggregateStartedModel(t)
	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskParallelPlanStarted,
		kinds.TaskParallelPlanPayload{
			Workflow:          "alpha",
			IntegrationBranch: "compozy/parallel-x",
			ParallelLimit:     1,
			Tasks: []kinds.TaskParallelPlanTask{
				{ID: "task_01", Number: 1, Title: "task_01", File: "task_01.md", WaveIndex: 0},
			},
			Waves: []kinds.TaskParallelPlanWave{
				{Index: 0, TaskIDs: []string{"task_01"}},
			},
		},
	))
	mdl.handleParentEvent(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindTaskParallelWaveStarted,
		kinds.TaskParallelPayload{
			WaveIndex:         0,
			WaveTotal:         1,
			TaskID:            "task_01",
			IntegrationBranch: "compozy/parallel-x",
		},
	))
	return mdl, mdl.tabs[0].child
}

func newParallelAggregateStartedModel(t *testing.T) *multiRunModel {
	t.Helper()

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
		kinds.TaskRunMultiplePayload{Status: taskMultiStatusRunning, Slugs: []string{"alpha"}, Total: 1},
	))
	if len(mdl.tabs) != 1 {
		t.Fatalf("expected one tab, got %d", len(mdl.tabs))
	}
	return mdl
}

func TestMultiRunParentRunFailedBeforeChildStartMarksQueuedTabFailed(t *testing.T) {
	t.Parallel()

	t.Run("Should show the parent failure without creating a fake child cockpit", func(t *testing.T) {
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
			kinds.TaskRunMultiplePayload{Status: taskMultiStatusRunning, Slugs: []string{"alpha"}, Total: 1},
		))
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunFailed,
			kinds.RunFailedPayload{Error: "missing task directory"},
		))
		if mdl.tabs[0].child != nil {
			t.Fatalf("run.failed before child start created child placeholder: %#v", mdl.tabs[0].child)
		}
		if mdl.tabs[0].status != taskMultiStatusFailed || mdl.tabs[0].errorText != "missing task directory" {
			t.Fatalf("tab after parent failure = %#v", mdl.tabs[0])
		}
		view := xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "QUEUE.FAILED") || !strings.Contains(view, "missing task directory") {
			t.Fatalf("expected failed queued panel with error, got %q", view)
		}
	})

	t.Run("Should show a parent crash as a terminal failed queued tab", func(t *testing.T) {
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
			kinds.TaskRunMultiplePayload{Status: taskMultiStatusRunning, Slugs: []string{"alpha"}, Total: 1},
		))
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunCrashed,
			kinds.RunCrashedPayload{Error: "daemon restarted"},
		))
		if mdl.parentRun.Status != remoteRunStatusCrashed {
			t.Fatalf("parent status = %q, want crashed", mdl.parentRun.Status)
		}
		if mdl.tabs[0].status != taskMultiStatusFailed || !mdl.tabs[0].terminal {
			t.Fatalf("tab after parent crash = %#v, want terminal failed", mdl.tabs[0])
		}
		view := xansi.Strip(mdl.View().Content)
		if !strings.Contains(view, "QUEUE.FAILED") || !strings.Contains(view, "daemon restarted") {
			t.Fatalf("expected failed queued panel with crash error, got %q", view)
		}
	})
}

func TestMultiRunChildEventCreatesPlaceholderAndMapsTerminalRunStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should create placeholder with job-control callbacks and map terminal status", func(t *testing.T) {
		t.Parallel()

		var pausedRunID, pausedJobID string
		var messageRunID, messageJobID, messageBody string
		mdl := &multiRunModel{
			parentRun:  apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			width:      120,
			height:     30,
			cfg:        &config{},
			quitDialog: newQuitDialogState(),
			pauseRunJob: func(
				_ context.Context,
				runID string,
				jobID string,
			) (apicore.RunJobControlResponse, error) {
				pausedRunID = runID
				pausedJobID = jobID
				return apicore.RunJobControlResponse{
					RunID:  runID,
					JobID:  jobID,
					Status: string(model.JobControlStatusPausing),
				}, nil
			},
			sendRunJobMessage: func(
				_ context.Context,
				runID string,
				jobID string,
				req apicore.RunJobMessageRequest,
			) (apicore.RunJobControlResponse, error) {
				messageRunID = runID
				messageJobID = jobID
				messageBody = req.Message
				return apicore.RunJobControlResponse{
					RunID:  runID,
					JobID:  jobID,
					Status: string(model.JobControlStatusResumed),
				}, nil
			},
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
		if mdl.tabs[0].child.onJobControl == nil {
			t.Fatal("expected placeholder child to wire job-control callback")
		}
		if _, err := mdl.tabs[0].child.onJobControl(context.Background(), uiJobControlRequest{
			Action: uiJobControlPause,
			JobID:  "alpha-job",
		}); err != nil {
			t.Fatalf("placeholder pause control error = %v", err)
		}
		if pausedRunID != "run-alpha" || pausedJobID != "alpha-job" {
			t.Fatalf("pause callback = %q/%q, want run-alpha/alpha-job", pausedRunID, pausedJobID)
		}
		if _, err := mdl.tabs[0].child.onJobControl(context.Background(), uiJobControlRequest{
			Action:  uiJobControlMessage,
			JobID:   "alpha-job",
			Message: "continue",
		}); err != nil {
			t.Fatalf("placeholder message control error = %v", err)
		}
		if messageRunID != "run-alpha" || messageJobID != "alpha-job" || messageBody != "continue" {
			t.Fatalf("message callback = %q/%q/%q, want run-alpha/alpha-job/continue",
				messageRunID,
				messageJobID,
				messageBody,
			)
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
	})
}

func TestFollowRemoteMultiRunChildFallsBackToStreamWhenBootstrapSnapshotFails(t *testing.T) {
	t.Parallel()

	mdl := multiRunModelForQuitTest()
	session := newRecordingMultiRunSession(context.Background(), mdl)
	streamOpened := false
	now := time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC)

	followRemoteMultiRunChild(context.Background(), session, RemoteMultiRunAttachOptions{
		LoadChildSnapshot: func(context.Context, string) (apicore.RunSnapshot, error) {
			return apicore.RunSnapshot{}, errors.New("snapshot unavailable")
		},
		OpenChildStream: func(_ context.Context, runID string, _ apicore.StreamCursor) (apiclient.RunStream, error) {
			if runID != "run-alpha" {
				t.Fatalf("unexpected child stream run id %q", runID)
			}
			streamOpened = true
			return newBufferedClientRunStream(apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
				t,
				eventspkg.EventKindRunCompleted,
				kinds.RunCompletedPayload{JobsTotal: 1, JobsSucceeded: 1},
			), 1, now)}), nil
		},
	}, "run-alpha", apicore.StreamCursor{}, true)

	if !streamOpened {
		t.Fatal("expected child stream to open after bootstrap snapshot failure")
	}
	session.withModel(func(mdl *multiRunModel) {
		if got := mdl.tabs[0].status; got != taskMultiStatusCompleted {
			t.Fatalf("alpha status = %q, want completed", got)
		}
	})
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
	runStatusCases := []struct {
		input string
		want  string
	}{
		{input: remoteRunStatusRunning, want: taskMultiStatusRunning},
		{input: remoteRunStatusPausing, want: taskMultiStatusRunning},
		{input: remoteRunStatusPaused, want: taskMultiStatusRunning},
		{input: remoteRunStatusRetrying, want: taskMultiStatusRunning},
		{input: remoteRunStatusCompleted, want: taskMultiStatusCompleted},
		{input: remoteRunStatusFailed, want: taskMultiStatusFailed},
		{input: remoteRunStatusCrashed, want: taskMultiStatusFailed},
		{input: remoteRunStatusCanceled, want: taskMultiStatusCanceled},
		{input: "pending", want: ""},
	}
	for _, tc := range runStatusCases {
		tc := tc
		t.Run("Should map run status "+tc.input, func(t *testing.T) {
			if got := taskMultiStatusFromRunStatus(tc.input); got != tc.want {
				t.Fatalf("taskMultiStatusFromRunStatus(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
	childEventCases := []struct {
		input eventspkg.EventKind
		want  string
	}{
		{input: eventspkg.EventKindRunCompleted, want: taskMultiStatusCompleted},
		{input: eventspkg.EventKindRunFailed, want: taskMultiStatusFailed},
		{input: eventspkg.EventKindRunCrashed, want: taskMultiStatusFailed},
		{input: eventspkg.EventKindRunCancelled, want: taskMultiStatusCanceled},
		{input: eventspkg.EventKindJobQueued, want: ""},
	}
	for _, tc := range childEventCases {
		tc := tc
		t.Run("Should map child event "+string(tc.input), func(t *testing.T) {
			if got := taskMultiStatusFromChildRunEvent(tc.input); got != tc.want {
				t.Fatalf("taskMultiStatusFromChildRunEvent(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
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
	if actions := mdl.renderQuitDialogActions(20); !strings.Contains(actions, "\n") {
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

func (s *recordingMultiRunSession) SetJobControlHandler(
	func(context.Context, uiJobControlRequest) (jobControlResponse, error),
) {
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

func TestMultiRunInitialSnapshotRendersWorktreeForSelectedChild(t *testing.T) {
	t.Parallel()

	t.Run("Should render worktree path and preservation status from the snapshot", func(t *testing.T) {
		worktreePath := "/home/dev/.compozy/state/worktrees/abc123/parent01/01-alpha"
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{
						Slug:           "alpha",
						Status:         taskMultiStatusRunning,
						RunID:          "run-alpha",
						WorktreePath:   worktreePath,
						BaseBranch:     "main",
						BaseCommit:     "deadbeef",
						WorktreeStatus: "preserved",
					},
					{Slug: "beta", Status: taskMultiStatusQueued},
				},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}
		mdl.handleWindowSize(tea.WindowSizeMsg{Width: 200, Height: 40})

		if got := mdl.tabs[0].worktreePath; got != worktreePath {
			t.Fatalf("tab worktree path = %q, want %q", got, worktreePath)
		}
		view := mdl.View().Content
		for _, want := range []string{worktreePath, "preserved", "branch main", "run run-alpha"} {
			if !strings.Contains(view, want) {
				t.Fatalf("expected view to contain %q, got %q", want, view)
			}
		}
	})
}

func TestMultiRunChildStartedEventAppliesWorktreeMetadataToTab(t *testing.T) {
	t.Parallel()

	t.Run("Should apply worktree metadata from a child-started event to an existing tab", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusQueued},
				},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}
		mdl.handleWindowSize(tea.WindowSizeMsg{Width: 200, Height: 40})

		worktreePath := "/home/dev/.compozy/state/worktrees/abc123/parent01/01-alpha"
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskRunMultipleChildStarted,
			kinds.TaskRunMultiplePayload{
				Slug:           "alpha",
				Index:          0,
				Status:         taskMultiStatusRunning,
				ChildRunID:     "run-alpha",
				WorktreePath:   worktreePath,
				BaseBranch:     "main",
				BaseCommit:     "deadbeef",
				WorktreeStatus: "preserved",
			},
		))

		tab := mdl.tabs[0]
		if tab.worktreePath != worktreePath || tab.baseBranch != "main" || tab.worktreeStatus != "preserved" {
			t.Fatalf("worktree metadata not applied to tab: %#v", tab)
		}
		if tab.runID != "run-alpha" {
			t.Fatalf("tab run id = %q, want run-alpha", tab.runID)
		}
		if view := mdl.View().Content; !strings.Contains(view, worktreePath) {
			t.Fatalf("expected view to contain worktree path %q, got %q", worktreePath, view)
		}
	})

	t.Run("Should preserve worktree metadata when a later event omits it", func(t *testing.T) {
		mdl := &multiRunModel{
			parentRun:  apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
			width:      200,
			height:     40,
			cfg:        &config{},
			quitDialog: newQuitDialogState(),
			tabs: []multiRunTab{{
				slug:           "alpha",
				status:         taskMultiStatusRunning,
				runID:          "run-alpha",
				worktreePath:   "/wt/01-alpha",
				worktreeStatus: "preserved",
				translator:     newUIEventTranslator(),
			}},
		}
		mdl.handleParentEvent(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Index:      0,
				Status:     taskMultiStatusCompleted,
				ChildRunID: "run-alpha",
			},
		))
		if got := mdl.tabs[0].worktreePath; got != "/wt/01-alpha" {
			t.Fatalf("worktree path overwritten by metadata-free event: %q", got)
		}
		if got := mdl.tabs[0].status; got != taskMultiStatusCompleted {
			t.Fatalf("status = %q, want completed", got)
		}
	})
}

func TestMultiRunSnapshotWithoutWorktreeMetadataOmitsWorktreeRow(t *testing.T) {
	t.Parallel()

	t.Run("Should omit the worktree row and not panic when metadata is absent", func(t *testing.T) {
		mdl, _, err := newRemoteMultiRunModel(context.Background(), RemoteMultiRunAttachOptions{
			Snapshot: apicore.TaskRunMultipleSnapshot{
				Run: apicore.Run{RunID: "parent-run", Status: remoteRunStatusRunning},
				Items: []apicore.TaskRunMultipleItem{
					{Slug: "alpha", Status: taskMultiStatusQueued},
				},
			},
		})
		if err != nil {
			t.Fatalf("newRemoteMultiRunModel() error = %v", err)
		}
		mdl.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

		if view := mdl.View().Content; strings.Contains(view, "worktree") {
			t.Fatalf("expected empty worktree metadata row to be omitted, got %q", view)
		}
	})
}

func TestFormatMultiRunWorktreeSummary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tab  *multiRunTab
		want string
	}{
		{name: "Should render empty summary for nil tab", tab: nil, want: ""},
		{name: "Should render empty summary when no metadata", tab: &multiRunTab{slug: "alpha"}, want: ""},
		{
			name: "Should render status and path",
			tab:  &multiRunTab{worktreePath: "/wt/01", worktreeStatus: "preserved"},
			want: "worktree preserved /wt/01",
		},
		{
			name: "Should render path only",
			tab:  &multiRunTab{worktreePath: "/wt/01"},
			want: "worktree /wt/01",
		},
		{
			name: "Should render status only",
			tab:  &multiRunTab{worktreeStatus: "preserved"},
			want: "worktree preserved",
		},
		{
			name: "Should append branch and run id",
			tab:  &multiRunTab{worktreePath: "/wt/01", worktreeStatus: "preserved", baseBranch: "main", runID: "run-1"},
			want: "worktree preserved /wt/01   branch main   run run-1",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatMultiRunWorktreeSummary(tc.tab); got != tc.want {
				t.Fatalf("formatMultiRunWorktreeSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}
