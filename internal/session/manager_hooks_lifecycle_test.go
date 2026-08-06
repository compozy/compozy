package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestFallbackLifecycleContextUsesManagerLifecycleContext(t *testing.T) {
	t.Parallel()

	type contextKey string

	lifecycleKey := contextKey("lifecycle")
	requestKey := contextKey("request")
	lifecycleCtx := context.WithValue(context.Background(), lifecycleKey, "manager")
	manager := &Manager{lifecycleCtx: lifecycleCtx}

	if got := manager.fallbackLifecycleContext().Value(lifecycleKey); got != "manager" {
		t.Fatalf("fallbackLifecycleContext() lifecycle value = %#v, want manager fallback", got)
	}

	requestCtx := context.WithValue(context.Background(), requestKey, "request")
	if got := manager.hookLifecycleContext(requestCtx).Value(requestKey); got != "request" {
		t.Fatalf("hookLifecycleContext(requestCtx) request value = %#v, want original request context", got)
	}
}

func TestSessionNetworkLifecycleHandling(t *testing.T) {
	t.Parallel()

	t.Run("ShouldFailCreateWhenNetworkJoinFails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		lifecycle := &recordingNetworkPeerLifecycle{
			joinErr: errors.New("join failed"),
		}
		h.manager.SetNetworkPeerLifecycle(lifecycle)

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:                    "coder",
			Name:                         "networked",
			Workspace:                    h.workspaceID,
			ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
		})
		if err == nil {
			t.Fatal("Create() error = nil, want join failure")
		}
		if !errors.Is(err, lifecycle.joinErr) {
			t.Fatalf("Create() error = %v, want wrapped join failure", err)
		}
		if got := lifecycle.joinCount(); got != 1 {
			t.Fatalf("join calls after failed Create() = %d, want 1", got)
		}
		if got := len(h.manager.List()); got != 0 {
			t.Fatalf("active sessions after failed Create() = %d, want 0", got)
		}
		if got := h.notifier.createdCount(); got != 0 {
			t.Fatalf("created notifications after failed Create() = %d, want 0", got)
		}
	})

	t.Run("ShouldRestoreStoppedMetadataWhenResumeJoinFails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:                    "coder",
			Workspace:                    h.workspaceID,
			ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		lifecycle := &recordingNetworkPeerLifecycle{
			joinErr: errors.New("resume join failed"),
		}
		h.manager.SetNetworkPeerLifecycle(lifecycle)

		if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
			t.Fatal("Resume() error = nil, want join failure")
		} else if !errors.Is(err, lifecycle.joinErr) {
			t.Fatalf("Resume() error = %v, want wrapped join failure", err)
		}

		meta := readMeta(t, session.MetaPath())
		if got, want := meta.State, string(StateStopped); got != want {
			t.Fatalf("restored meta state = %q, want %q", got, want)
		}
		if meta.StopReason == nil || *meta.StopReason != store.StopUserCanceled {
			t.Fatalf("restored meta stop reason = %v, want %q", meta.StopReason, store.StopUserCanceled)
		}
	})

	for _, tc := range []struct {
		name     string
		leaveErr error
	}{
		{
			name:     "ShouldIgnoreCanceledLeaveCleanupOnStop",
			leaveErr: context.Canceled,
		},
		{
			name:     "ShouldIgnoreDeadlineExceededLeaveCleanupOnStop",
			leaveErr: context.DeadlineExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			lifecycle := &recordingNetworkPeerLifecycle{leaveErr: tc.leaveErr}
			h.manager.SetNetworkPeerLifecycle(lifecycle)

			session, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName:                    "coder",
				Workspace:                    h.workspaceID,
				ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
			})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v, want leave cancellation to be ignored", err)
			}
			if got := lifecycle.leaveCount(); got != 1 {
				t.Fatalf("leave calls after Stop() = %d, want 1", got)
			}

			meta := readMeta(t, session.MetaPath())
			if got, want := meta.State, string(StateStopped); got != want {
				t.Fatalf("meta state after Stop() = %q, want %q", got, want)
			}
		})
	}
}

func TestStopWithCauseLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve stop hooks around a fatal prompt failure", func(t *testing.T) {
		t.Parallel()

		type lifecycleStep struct {
			name  string
			state State
		}

		var (
			stepsMu sync.Mutex
			steps   []lifecycleStep
			session *Session
		)
		recordStep := func(name string) {
			stepsMu.Lock()
			defer stepsMu.Unlock()
			steps = append(steps, lifecycleStep{name: name, state: session.Info().State})
		}

		dispatcher := &spyHookDispatcher{
			dispatchSessionPreStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPreStopPayload,
			) (hookspkg.SessionPreStopPayload, error) {
				recordStep("pre-stop")
				return payload, nil
			},
			dispatchSessionPostStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPostStopPayload,
			) (hookspkg.SessionPostStopPayload, error) {
				recordStep("post-stop")
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session = createSession(t, h)
		t.Cleanup(func() {
			if _, ok := h.manager.Get(session.ID); ok {
				reportSessionStop(t, h, session.ID)
			}
		})

		h.driver.stopHook = func(proc *fakeProcess) error {
			recordStep("driver-stop")
			proc.crash(errors.New("acp subprocess exited: exit status 23"), "codex stderr tail")
			return nil
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- acp.AgentEvent{
				Type:      acp.EventTypeError,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Error:     "ACP process disconnected",
				Failure: &store.SessionFailure{
					Kind:    store.FailureProcess,
					Summary: "ACP process disconnected",
				},
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		promptEvents := collectEvents(t, events)
		if got, want := len(promptEvents), 1; got != want {
			t.Fatalf("Prompt() events = %d, want %d", got, want)
		}
		if failure := promptEvents[0].Failure; failure == nil || failure.Kind != store.FailureProcess {
			t.Fatalf("Prompt() failure = %#v, want process_exit", failure)
		}
		h.notifier.waitForStopped(t, session.ID)

		stepsMu.Lock()
		gotSteps := append([]lifecycleStep(nil), steps...)
		stepsMu.Unlock()
		wantSteps := []lifecycleStep{
			{name: "pre-stop", state: StateActive},
			{name: "driver-stop", state: StateStopping},
			{name: "post-stop", state: StateStopped},
		}
		if len(gotSteps) != len(wantSteps) {
			t.Fatalf("fatal prompt lifecycle steps = %#v, want %#v", gotSteps, wantSteps)
		}
		for i := range wantSteps {
			if gotSteps[i] != wantSteps[i] {
				t.Fatalf("fatal prompt lifecycle step %d = %#v, want %#v", i, gotSteps[i], wantSteps[i])
			}
		}
		if got, want := h.driver.stopCalls, 1; got != want {
			t.Fatalf("driver stop calls = %d, want %d", got, want)
		}
		if got, want := h.notifier.stoppedCount(), 1; got != want {
			t.Fatalf("stopped notifications = %d, want %d", got, want)
		}
		if _, ok := h.manager.Get(session.ID); ok {
			t.Fatalf("Get(%q) found session after fatal prompt finalization", session.ID)
		}
	})

	t.Run("Should preserve lifecycle hooks when process exit wins prompt stream closure", func(t *testing.T) {
		t.Parallel()

		type lifecycleStep struct {
			name  string
			state State
		}

		var (
			stepsMu sync.Mutex
			steps   []lifecycleStep
			session *Session
		)
		recordStep := func(name string) {
			stepsMu.Lock()
			defer stepsMu.Unlock()
			steps = append(steps, lifecycleStep{name: name, state: session.Info().State})
		}

		dispatcher := &spyHookDispatcher{
			dispatchSessionPreStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPreStopPayload,
			) (hookspkg.SessionPreStopPayload, error) {
				recordStep("pre-stop")
				return payload, nil
			},
			dispatchSessionPostStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPostStopPayload,
			) (hookspkg.SessionPostStopPayload, error) {
				recordStep("post-stop")
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session = createSession(t, h)
		h.notifier.finalizingHook = func(context.Context, *Session) {
			recordStep("finalizing")
		}
		t.Cleanup(func() {
			if _, ok := h.manager.Get(session.ID); ok {
				reportSessionStop(t, h, session.ID)
			}
		})

		source := make(chan acp.AgentEvent)
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return source, nil
		}
		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		h.driver.lastProcess().crash(
			acp.WrapFailure(
				store.FailureProcess,
				"ACP subprocess exited unexpectedly",
				errors.New("acp subprocess exited: exit status 23"),
			),
			"codex stderr tail",
		)
		close(source)

		promptEvents := collectEvents(t, events)
		if got, want := len(promptEvents), 1; got != want {
			t.Fatalf("Prompt() events = %d, want %d", got, want)
		}
		if failure := promptEvents[0].Failure; failure == nil || failure.Kind != store.FailureProcess {
			t.Fatalf("Prompt() failure = %#v, want process_exit", failure)
		}
		h.notifier.waitForStopped(t, session.ID)

		stepsMu.Lock()
		gotSteps := append([]lifecycleStep(nil), steps...)
		stepsMu.Unlock()
		wantSteps := []lifecycleStep{
			{name: "pre-stop", state: StateActive},
			{name: "finalizing", state: StateStopping},
			{name: "post-stop", state: StateStopped},
		}
		if len(gotSteps) != len(wantSteps) {
			t.Fatalf("process-exit lifecycle steps = %#v, want %#v", gotSteps, wantSteps)
		}
		for i := range wantSteps {
			if gotSteps[i] != wantSteps[i] {
				t.Fatalf("process-exit lifecycle step %d = %#v, want %#v", i, gotSteps[i], wantSteps[i])
			}
		}
		if got := h.driver.stopCalls; got != 0 {
			t.Fatalf("driver stop calls = %d, want none for an already exited process", got)
		}
		if got, want := h.notifier.stoppedCount(), 1; got != want {
			t.Fatalf("stopped notifications = %d, want %d", got, want)
		}
		if _, ok := h.manager.Get(session.ID); ok {
			t.Fatalf("Get(%q) found session after process-exit finalization", session.ID)
		}
	})

	t.Run("Should reject a pre-stop workspace mutation before persistence", func(t *testing.T) {
		t.Parallel()

		var patchOnce sync.Once
		dispatcher := &spyHookDispatcher{
			dispatchSessionPreStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPreStopPayload,
			) (hookspkg.SessionPreStopPayload, error) {
				patchOnce.Do(func() {
					payload.WorkspaceID = "ws-pre-stop-patched"
				})
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session := createSession(t, h)
		t.Cleanup(func() {
			err := h.manager.Stop(testutil.Context(t), session.ID)
			if err != nil && !errors.Is(err, ErrSessionNotFound) {
				t.Errorf("cleanup Stop() error = %v", err)
			}
		})
		original := session.Info()

		err := h.manager.Stop(testutil.Context(t), session.ID)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Stop(workspace mutation) error = %v, want %v", err, ErrValidation)
		}
		current := session.Info()
		if current.WorkspaceID != original.WorkspaceID || current.Workspace != original.Workspace {
			t.Fatalf(
				"session workspace after rejected stop = {%q %q}, want {%q %q}",
				current.WorkspaceID,
				current.Workspace,
				original.WorkspaceID,
				original.Workspace,
			)
		}
		meta := readMeta(t, session.MetaPath())
		if meta.WorkspaceID != original.WorkspaceID {
			t.Fatalf("persisted workspace ID after rejected stop = %q, want %q", meta.WorkspaceID, original.WorkspaceID)
		}
	})

	t.Run("ShouldReturnImmediatelyWhenDriverStopFailsBeforeProcessExit", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		stopErr := errors.New("driver stop failed")
		h.driver.stopHook = func(*fakeProcess) error {
			return stopErr
		}

		stopCtx, cancel := context.WithTimeout(testutil.Context(t), time.Second)
		defer cancel()

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.StopWithCause(stopCtx, session.ID, CauseUserRequested, "")
		}()

		select {
		case err := <-stopDone:
			if !errors.Is(err, stopErr) {
				t.Fatalf("StopWithCause() error = %v, want wrapped driver stop failure", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("StopWithCause() blocked waiting for proc.Done after driver stop failure")
		}

		h.driver.stopHook = nil
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("cleanup Stop() error = %v", err)
		}
	})

	t.Run("ShouldIgnoreStopErrorWhenProcessExitsDuringStop", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		h.driver.stopHook = func(proc *fakeProcess) error {
			proc.exit()
			return errors.New("process already exited")
		}

		if err := h.manager.StopWithCause(testutil.Context(t), session.ID, CauseUserRequested, ""); err != nil {
			t.Fatalf("StopWithCause() error = %v, want nil after process exit during stop", err)
		}

		meta := readMeta(t, session.MetaPath())
		if got, want := meta.State, string(StateStopped); got != want {
			t.Fatalf("meta state after StopWithCause() = %q, want %q", got, want)
		}
	})

	t.Run("ShouldWaitForPostStopDispatchWhenWatcherFinalizesFirst", func(t *testing.T) {
		t.Parallel()

		postStopStarted := make(chan struct{})
		releasePostStop := make(chan struct{})
		dispatcher := &spyHookDispatcher{
			dispatchSessionPostStopFn: func(_ context.Context, payload hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error) {
				close(postStopStarted)
				<-releasePostStop
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session := createSession(t, h)

		h.driver.stopHook = func(proc *fakeProcess) error {
			proc.exit()
			select {
			case <-postStopStarted:
				return nil
			case <-time.After(time.Second):
				return errors.New("test: watcher did not reach session.post_stop")
			}
		}

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.StopWithCause(testutil.Context(t), session.ID, CauseShutdown, "daemon shutdown")
		}()

		select {
		case err := <-stopDone:
			t.Fatalf("StopWithCause() returned before session.post_stop completed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		close(releasePostStop)

		if err := <-stopDone; err != nil {
			t.Fatalf("StopWithCause() error = %v", err)
		}
		if got := h.notifier.stoppedCount(); got != 1 {
			t.Fatalf("stopped notifications = %d, want 1", got)
		}
	})

	t.Run("Should return watcher finalization error to concurrent stop", func(t *testing.T) {
		t.Parallel()

		postStopStarted := make(chan struct{})
		releasePostStop := make(chan struct{})
		dispatcher := &spyHookDispatcher{
			dispatchSessionPostStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPostStopPayload,
			) (hookspkg.SessionPostStopPayload, error) {
				close(postStopStarted)
				<-releasePostStop
				return payload, nil
			},
		}
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)), WithSessionCatalog(catalog))
		session := createSession(t, h)
		updateErr := errors.New("publish stopped session")
		h.driver.stopHook = func(proc *fakeProcess) error {
			catalog.setUpdateErr(updateErr)
			proc.exit()
			select {
			case <-postStopStarted:
				return nil
			case <-time.After(time.Second):
				return errors.New("test: watcher did not reach session.post_stop")
			}
		}

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.StopWithCause(
				testutil.Context(t),
				session.ID,
				CauseUserRequested,
				"",
			)
		}()

		select {
		case err := <-stopDone:
			t.Fatalf("StopWithCause() returned before watcher finalization completed: %v", err)
		case <-postStopStarted:
		}
		close(releasePostStop)

		if err := <-stopDone; !errors.Is(err, updateErr) {
			t.Fatalf("StopWithCause() error = %v, want watcher finalization error", err)
		}
	})

	t.Run("Should wait when watcher has marked session stopped before publishing", func(t *testing.T) {
		t.Parallel()

		stoppedWriteStarted := make(chan struct{})
		releaseStoppedWrite := make(chan struct{})
		updateErr := errors.New("publish stopped session")
		catalog := newRecordingSessionCatalog()
		catalog.updateHook = func(update store.SessionStateUpdate) error {
			if update.State != string(StateStopped) {
				return nil
			}
			close(stoppedWriteStarted)
			<-releaseStoppedWrite
			return updateErr
		}
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)

		h.driver.lastProcess().exit()
		select {
		case <-stoppedWriteStarted:
		case <-time.After(time.Second):
			t.Fatal("watcher did not begin stopped catalog publication")
		}

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.StopWithCause(
				testutil.Context(t),
				session.ID,
				CauseUserRequested,
				"",
			)
		}()

		select {
		case err := <-stopDone:
			t.Fatalf("StopWithCause() returned before stopped catalog publication completed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		close(releaseStoppedWrite)

		if err := <-stopDone; !errors.Is(err, updateErr) {
			t.Fatalf("StopWithCause() error = %v, want stopped catalog publication error", err)
		}
	})

	t.Run("Should wait after watcher removes active session", func(t *testing.T) {
		t.Parallel()

		postStopStarted := make(chan struct{})
		releasePostStop := make(chan struct{})
		dispatcher := &spyHookDispatcher{
			dispatchSessionPostStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPostStopPayload,
			) (hookspkg.SessionPostStopPayload, error) {
				close(postStopStarted)
				<-releasePostStop
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session := createSession(t, h)

		h.driver.lastProcess().exit()
		select {
		case <-postStopStarted:
		case <-time.After(time.Second):
			t.Fatal("watcher did not remove the active session before post-stop dispatch")
		}

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.StopWithCause(
				testutil.Context(t),
				session.ID,
				CauseUserRequested,
				"",
			)
		}()

		select {
		case err := <-stopDone:
			t.Fatalf("StopWithCause() returned before removed session finalization completed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		close(releasePostStop)

		if err := <-stopDone; err != nil {
			t.Fatalf("StopWithCause() error = %v, want completed watcher finalization", err)
		}
	})

	t.Run("Should bound shutdown while watcher finalization remains blocked", func(t *testing.T) {
		t.Parallel()

		postStopStarted := make(chan struct{})
		releasePostStop := make(chan struct{})
		dispatcher := &spyHookDispatcher{
			dispatchSessionPostStopFn: func(
				_ context.Context,
				payload hookspkg.SessionPostStopPayload,
			) (hookspkg.SessionPostStopPayload, error) {
				close(postStopStarted)
				<-releasePostStop
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		createSession(t, h)
		h.driver.lastProcess().exit()
		<-postStopStarted

		canceledCtx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if err := h.manager.Shutdown(canceledCtx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Shutdown(canceled) error = %v, want context cancellation", err)
		}

		shutdownDone := make(chan error, 1)
		go func() {
			shutdownDone <- h.manager.Shutdown(testutil.Context(t))
		}()
		select {
		case err := <-shutdownDone:
			t.Fatalf("Shutdown(retry) returned before watcher finalization completed: %v", err)
		default:
		}

		close(releasePostStop)
		if err := <-shutdownDone; err != nil {
			t.Fatalf("Shutdown(retry) error = %v", err)
		}
	})
}

func TestAgentCrashedHookFiresOnProcessCrash(t *testing.T) {
	t.Parallel()

	payloadCh := make(chan hookspkg.AgentCrashedPayload, 1)
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "observe-agent-crash",
			Event:        hookspkg.HookAgentCrashed,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"observe-agent-crash": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.AgentCrashedPayload) (hookspkg.AgentCrashedPatch, error) {
					payloadCh <- payload
					return hookspkg.AgentCrashedPatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)

	h.driver.lastProcess().crash(errors.New("boom"), "stderr trace")
	h.notifier.waitForStopped(t, session.ID)
	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) found session after stopped notification", session.ID)
	}

	select {
	case payload := <-payloadCh:
		if payload.SessionID != session.ID {
			t.Fatalf("payload.SessionID = %q, want %q", payload.SessionID, session.ID)
		}
		if payload.Error != "boom" {
			t.Fatalf("payload.Error = %q, want %q", payload.Error, "boom")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for agent.crashed hook")
	}
}
