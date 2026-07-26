package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestCreateFailsWhenSessionPreCreateDenied(t *testing.T) {
	t.Parallel()

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "deny-create",
			Event:        hookspkg.HookSessionPreCreate,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"deny-create": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreCreatePayload) (hookspkg.SessionCreatePatch, error) {
					return hookspkg.SessionCreatePatch{
						ControlPatch: hookspkg.ControlPatch{
							Deny:       true,
							DenyReason: "blocked",
						},
					}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	_, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err == nil {
		t.Fatal("Create() error = nil, want pre-create denial")
	}
	if len(h.manager.List()) != 0 {
		t.Fatalf("List() = %d active sessions, want 0", len(h.manager.List()))
	}
	if got := h.notifier.createdCount(); got != 0 {
		t.Fatalf("created notifications = %d, want 0", got)
	}
}

func TestCreateUsesPatchedSessionPreCreatePayload(t *testing.T) {
	t.Parallel()

	const patchedName = "patched-session"
	sessionName := patchedName
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-create",
			Event:        hookspkg.HookSessionPreCreate,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-create": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreCreatePayload) (hookspkg.SessionCreatePatch, error) {
					sessionType := string(SessionTypeDream)
					return hookspkg.SessionCreatePatch{
						SessionName: &sessionName,
						SessionType: &sessionType,
					}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "original",
		Workspace: h.workspaceID,
		Type:      SessionTypeUser,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := session.Info().Name; got != patchedName {
		t.Fatalf("session name = %q, want %q", got, patchedName)
	}
	if got := session.Info().Type; got != SessionTypeDream {
		t.Fatalf("session type = %q, want %q", got, SessionTypeDream)
	}
	if got := h.driver.startCalls[0].Permissions; got != aghconfig.PermissionModeApproveAll {
		t.Fatalf("start permissions = %q, want %q", got, aghconfig.PermissionModeApproveAll)
	}
}

func TestPostCreateHookFiresAfterSessionActive(t *testing.T) {
	t.Parallel()

	payloadCh := make(chan hookspkg.SessionPostCreatePayload, 1)
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "observe-post-create",
			Event:        hookspkg.HookSessionPostCreate,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"observe-post-create": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePatch, error) {
					payloadCh <- payload
					return hookspkg.SessionPostCreatePatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	select {
	case payload := <-payloadCh:
		if payload.SessionID != session.ID {
			t.Fatalf("payload.SessionID = %q, want %q", payload.SessionID, session.ID)
		}
		if payload.State != string(StateActive) {
			t.Fatalf("payload.State = %q, want %q", payload.State, StateActive)
		}
		if payload.ACPSessionID == "" {
			t.Fatal("payload.ACPSessionID = empty, want active ACP session id")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session.post_create hook")
	}
}

func TestPostCreateAsyncHookSurvivesRequestCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	observedErr := make(chan error, 1)
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "observe-post-create-after-request",
			Event:        hookspkg.HookSessionPostCreate,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"observe-post-create-after-request": hookspkg.NewTypedNativeExecutor(
				func(ctx context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePatch, error) {
					<-release
					observedErr <- ctx.Err()
					return hookspkg.SessionPostCreatePatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	requestCtx, cancelRequest := context.WithCancel(testutil.Context(t))
	session, err := h.manager.Create(requestCtx, CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	cancelRequest()
	close(release)

	select {
	case err := <-observedErr:
		if err != nil {
			t.Fatalf("async post-create context error = %v, want nil after request cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async session.post_create hook")
	}
}

func TestResumeUsesPatchedPreResumePayloadAndFiresPostResume(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	const patchedName = "resumed-patched"
	postResumeCh := make(chan hookspkg.SessionPostResumePayload, 1)
	dispatcher := &spyHookDispatcher{
		dispatchSessionPreResumeFn: func(_ context.Context, payload hookspkg.SessionPreResumePayload) (hookspkg.SessionPreResumePayload, error) {
			payload.SessionName = patchedName
			return payload, nil
		},
		dispatchSessionPostResumeFn: func(_ context.Context, payload hookspkg.SessionPostResumePayload) (hookspkg.SessionPostResumePayload, error) {
			postResumeCh <- payload
			return payload, nil
		},
	}

	h.manager = newManagerWithHarness(t, h, WithHookSet(fullHookSet(dispatcher)))
	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := resumed.Info().Name; got != patchedName {
		t.Fatalf("resumed name = %q, want %q", got, patchedName)
	}

	select {
	case payload := <-postResumeCh:
		if payload.SessionID != resumed.ID {
			t.Fatalf("payload.SessionID = %q, want %q", payload.SessionID, resumed.ID)
		}
		if payload.State != string(StateActive) {
			t.Fatalf("payload.State = %q, want %q", payload.State, StateActive)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session.post_resume hook")
	}
}

func TestResumeRejectsPatchedWorkspaceThatConflictsWithLiveSnapshot(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	liveSession := createLiveNetworkSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), liveSession.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	patchedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(primary workspace) error = %v", err)
	}
	patchedWorkspace.ID = "ws-pre-resume-patched"
	patchedWorkspace.WorkspaceID = patchedWorkspace.ID
	patchedWorkspace.Name = "pre-resume-patched"
	h.resolver.upsert(&patchedWorkspace)

	dispatcher := &spyHookDispatcher{
		dispatchSessionPreResumeFn: func(
			_ context.Context,
			payload hookspkg.SessionPreResumePayload,
		) (hookspkg.SessionPreResumePayload, error) {
			payload.WorkspaceID = patchedWorkspace.ID
			return payload, nil
		},
	}
	h.manager = newManagerWithHarness(t, h, WithHookSet(fullHookSet(dispatcher)))
	startCallsBeforeResume := len(h.driver.startCalls)

	_, err = h.manager.Resume(testutil.Context(t), liveSession.ID)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Resume(workspace mismatch) error = %v, want %v", err, ErrValidation)
	}
	if got := len(h.driver.startCalls); got != startCallsBeforeResume {
		t.Fatalf("driver start calls = %d, want unchanged %d", got, startCallsBeforeResume)
	}
	if got := len(h.manager.List()); got != 0 {
		t.Fatalf("active sessions after rejected resume = %d, want 0", got)
	}
}

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

func TestPromptUsesPatchedInputMessage(t *testing.T) {
	t.Parallel()

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-input",
			Event:        hookspkg.HookInputPreSubmit,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-input": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPatch, error) {
					message := "patched message"
					return hookspkg.InputPreSubmitPatch{Message: &message}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "original message")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	if got := h.driver.promptCalls[0].Message; got != "patched message" {
		t.Fatalf("prompt message = %q, want %q", got, "patched message")
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	userMessage := storedEventByType(t, stored, acp.EventTypeUserMessage)
	if !strings.Contains(userMessage.Content, `"text":"patched message"`) {
		t.Fatalf("stored user message content = %q, want patched text", userMessage.Content)
	}
	if !strings.Contains(userMessage.Content, `"authored_text":"original message"`) {
		t.Fatalf("stored user message content = %q, want exact authored text", userMessage.Content)
	}
}

func TestPromptNetworkUsesNetworkInputClass(t *testing.T) {
	t.Parallel()

	t.Run("ShouldUseNetworkInputClass", func(t *testing.T) {
		dispatcher := &spyHookDispatcher{}
		var (
			inputPayload     hookspkg.InputPreSubmitPayload
			turnStartPayload hookspkg.TurnStartPayload
		)
		dispatcher.dispatchInputPreSubmitFn = func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
			inputPayload = payload
			return payload, nil
		}
		dispatcher.dispatchTurnStartFn = func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			turnStartPayload = payload
			return payload, nil
		}

		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		session := createLiveNetworkSession(t, h)
		t.Cleanup(func() {
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		})

		eventsCh, err := h.manager.PromptNetwork(testutil.Context(t), session.ID, "network message")
		if err != nil {
			t.Fatalf("PromptNetwork() error = %v", err)
		}
		_ = collectEvents(t, eventsCh)

		if inputPayload.InputClass != hookInputClassNetworkMessage {
			t.Fatalf(
				"input.pre_submit input class = %q, want %q",
				inputPayload.InputClass,
				hookInputClassNetworkMessage,
			)
		}
		if turnStartPayload.InputClass != hookInputClassNetworkMessage {
			t.Fatalf("turn.start input class = %q, want %q", turnStartPayload.InputClass, hookInputClassNetworkMessage)
		}
		if turnStartPayload.UserMessage != "network message" {
			t.Fatalf("turn.start user message = %q, want %q", turnStartPayload.UserMessage, "network message")
		}
	})
}

func TestNewPromptTurnDispatchStateNormalizesInputClass(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		turnSource     TurnSource
		wantTurnSource TurnSource
		wantInputClass string
	}{
		{
			name:           "unknown source falls back to user",
			turnSource:     TurnSource("unexpected"),
			wantTurnSource: TurnSourceUser,
			wantInputClass: hookInputClassUserMessage,
		},
		{
			name:           "synthetic source keeps synthetic class",
			turnSource:     TurnSourceSynthetic,
			wantTurnSource: TurnSourceSynthetic,
			wantInputClass: hookInputClassSynthetic,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := newPromptTurnDispatchState(nil, " turn-1 ", tc.turnSource, "message")
			if got := state.turnSource; got != tc.wantTurnSource {
				t.Fatalf("turnSource = %q, want %q", got, tc.wantTurnSource)
			}
			if got := state.inputClass; got != tc.wantInputClass {
				t.Fatalf("inputClass = %q, want %q", got, tc.wantInputClass)
			}
			if got := state.turnID; got != "turn-1" {
				t.Fatalf("turnID = %q, want %q", got, "turn-1")
			}
		})
	}
}

func TestPromptSyntheticUsesSyntheticInputClass(t *testing.T) {
	t.Parallel()

	dispatcher := &spyHookDispatcher{}
	var (
		inputPayload     hookspkg.InputPreSubmitPayload
		turnStartPayload hookspkg.TurnStartPayload
	)
	dispatcher.dispatchInputPreSubmitFn = func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
		inputPayload = payload
		return payload, nil
	}
	dispatcher.dispatchTurnStartFn = func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
		turnStartPayload = payload
		return payload, nil
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "daemon wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
			Reason:    "task_run_completed",
			Summary:   "background work finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	if inputPayload.InputClass != hookInputClassSynthetic {
		t.Fatalf(
			"input.pre_submit input class = %q, want %q",
			inputPayload.InputClass,
			hookInputClassSynthetic,
		)
	}
	if turnStartPayload.InputClass != hookInputClassSynthetic {
		t.Fatalf("turn.start input class = %q, want %q", turnStartPayload.InputClass, hookInputClassSynthetic)
	}
	if turnStartPayload.UserMessage != "daemon wake-up" {
		t.Fatalf("turn.start user message = %q, want %q", turnStartPayload.UserMessage, "daemon wake-up")
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
}

func TestCreateUsesPatchedPrompt(t *testing.T) {
	t.Parallel()

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-prompt",
			Event:        hookspkg.HookPromptPostAssemble,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-prompt": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.PromptPayload) (hookspkg.PromptPatch, error) {
					prompt := "patched system prompt"
					return hookspkg.PromptPatch{Prompt: &prompt}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := h.driver.startCalls[0].SystemPrompt; got != "patched system prompt" {
		t.Fatalf("start system prompt = %q, want %q", got, "patched system prompt")
	}
}

func TestCreateAppliesStartupPromptOverlayAfterPromptPatch(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve daemon startup overlay after prompt hook replacement", func(t *testing.T) {
		t.Parallel()

		hooks := newNativeHookDispatcher(t,
			[]hookspkg.HookDecl{{
				Name:         "patch-prompt",
				Event:        hookspkg.HookPromptPostAssemble,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			}},
			map[string]hookspkg.Executor{
				"patch-prompt": hookspkg.NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ hookspkg.RegisteredHook,
						_ hookspkg.PromptPayload,
					) (hookspkg.PromptPatch, error) {
						prompt := "patched system prompt"
						return hookspkg.PromptPatch{Prompt: &prompt}, nil
					},
				),
			},
		)

		h := newHarness(
			t,
			WithHookSet(fullHookSet(hooks)),
			WithStartupPromptOverlay(
				startupPromptOverlayFunc(func(
					_ context.Context,
					_ StartupPromptContext,
					prompt string,
				) (string, error) {
					return "protected runtime envelope\n\n" + prompt, nil
				}),
			),
		)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		got := h.driver.startCalls[0].SystemPrompt
		want := "protected runtime envelope\n\npatched system prompt"
		if got != want {
			t.Fatalf("start system prompt = %q, want %q", got, want)
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
	waitForCondition(t, "session stop after crash", func() bool {
		_, ok := h.manager.Get(session.ID)
		return !ok
	})

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

func TestRecordEventDispatchesAroundPersistence(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 3)
	dispatcher := &spyHookDispatcher{
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			order = append(order, "pre:"+payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			order = append(order, "post:"+payload.RecordType)
			return payload, nil
		},
	}
	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	recorder := &orderedRecorder{
		onRecord: func(event store.SessionEvent) {
			order = append(order, "record:"+event.Type)
		},
	}
	now := h.manager.now()
	session := &Session{
		ID:          "sess-event",
		AgentName:   "coder",
		WorkspaceID: h.workspaceID,
		Workspace:   h.workspace,
		Type:        SessionTypeUser,
		State:       StateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		recorder:    recorder,
	}

	err := h.manager.recordEvent(testutil.Context(t), session, acp.AgentEvent{
		Type:      acp.EventTypeDone,
		TurnID:    "turn-1",
		Timestamp: now,
		Text:      "done",
	})
	if err != nil {
		t.Fatalf("recordEvent() error = %v", err)
	}

	want := []string{"pre:done", "record:done", "post:done"}
	if !testutil.EqualStringSlices(order, want) {
		t.Fatalf("dispatch order = %#v, want %#v", order, want)
	}
}

func TestRecordEventDispatchesSessionMessagePersistedAfterDurableAgentMessage(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 4)
	var persistedPayload hookspkg.SessionMessagePersistedPayload
	dispatcher := &spyHookDispatcher{
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			order = append(order, "pre:"+payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			order = append(order, "post:"+payload.RecordType)
			if payload.Sequence != 1 {
				t.Fatalf("event.post_record sequence = %d, want 1", payload.Sequence)
			}
			return payload, nil
		},
		dispatchSessionMessagePersistedFn: func(_ context.Context, payload hookspkg.SessionMessagePersistedPayload) (hookspkg.SessionMessagePersistedPayload, error) {
			order = append(order, "persisted:"+payload.Role)
			persistedPayload = payload
			return payload, nil
		},
	}
	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	recorder := &orderedRecorder{
		onRecord: func(event store.SessionEvent) {
			order = append(order, "record:"+event.Type)
		},
	}
	now := h.manager.now()
	session := &Session{
		ID:          "sess-event",
		AgentName:   "coder",
		WorkspaceID: h.workspaceID,
		Workspace:   h.workspace,
		Type:        SessionTypeUser,
		State:       StateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		recorder:    recorder,
	}

	err := h.manager.recordEvent(testutil.Context(t), session, acp.AgentEvent{
		Type:      acp.EventTypeAgentMessage,
		TurnID:    "turn-1",
		Timestamp: now,
		Text:      "durable reply",
		Raw:       []byte(`{"type":"agent_message","text":"durable reply"}`),
	})
	if err != nil {
		t.Fatalf("recordEvent() error = %v", err)
	}

	want := []string{"pre:agent_message", "record:agent_message", "post:agent_message", "persisted:assistant"}
	if !testutil.EqualStringSlices(order, want) {
		t.Fatalf("dispatch order = %#v, want %#v", order, want)
	}
	if persistedPayload.MessageSeq != 1 {
		t.Fatalf("message seq = %d, want 1", persistedPayload.MessageSeq)
	}
	if persistedPayload.Text != "durable reply" {
		t.Fatalf("payload text = %q, want durable reply", persistedPayload.Text)
	}
	if persistedPayload.RootSessionID != session.ID {
		t.Fatalf("root session id = %q, want %q", persistedPayload.RootSessionID, session.ID)
	}
	if persistedPayload.ParentSessionID != "" {
		t.Fatalf("parent session id = %q, want empty root session", persistedPayload.ParentSessionID)
	}
	if persistedPayload.ActorKind != "agent_root" {
		t.Fatalf("actor kind = %q, want agent_root", persistedPayload.ActorKind)
	}
}

func TestPromptDispatchesTurnAndMessageHooksAtACPBoundaries(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 6)
	var (
		turnStartPayload        hookspkg.TurnStartPayload
		messageStartPayload     hookspkg.MessageStartPayload
		messageDeltaPayload     hookspkg.MessageDeltaPayload
		messagePersistedPayload hookspkg.SessionMessagePersistedPayload
		messageEndPayload       hookspkg.MessageEndPayload
		turnEndPayload          hookspkg.TurnEndPayload
	)

	dispatcher := &spyHookDispatcher{
		dispatchTurnStartFn: func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			order = append(order, "turn.start")
			turnStartPayload = payload
			return payload, nil
		},
		dispatchMessageStartFn: func(_ context.Context, payload hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error) {
			order = append(order, "message.start")
			messageStartPayload = payload
			return payload, nil
		},
		dispatchMessageDeltaFn: func(_ context.Context, payload hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error) {
			order = append(order, "message.delta")
			messageDeltaPayload = payload
			return payload, nil
		},
		dispatchSessionMessagePersistedFn: func(_ context.Context, payload hookspkg.SessionMessagePersistedPayload) (hookspkg.SessionMessagePersistedPayload, error) {
			order = append(order, "session.message_persisted")
			messagePersistedPayload = payload
			return payload, nil
		},
		dispatchMessageEndFn: func(_ context.Context, payload hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error) {
			order = append(order, "message.end")
			messageEndPayload = payload
			return payload, nil
		},
		dispatchTurnEndFn: func(_ context.Context, payload hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error) {
			order = append(order, "turn.end")
			turnEndPayload = payload
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}

	wantOrder := []string{
		"turn.start",
		"message.start",
		"message.delta",
		"session.message_persisted",
		"message.end",
		"turn.end",
	}
	if !testutil.EqualStringSlices(order, wantOrder) {
		t.Fatalf("hook order = %#v, want %#v", order, wantOrder)
	}

	if turnStartPayload.UserMessage != "hello" {
		t.Fatalf("turn.start user message = %q, want %q", turnStartPayload.UserMessage, "hello")
	}
	if turnStartPayload.TurnID == "" {
		t.Fatal("turn.start turn id = empty, want populated turn id")
	}
	if turnStartPayload.InputClass != hookInputClassUserMessage {
		t.Fatalf("turn.start input class = %q, want %q", turnStartPayload.InputClass, hookInputClassUserMessage)
	}
	if messageStartPayload.MessageID == "" {
		t.Fatal("message.start message id = empty, want populated message id")
	}
	if messageStartPayload.Role != hookMessageRoleAssistant {
		t.Fatalf("message.start role = %q, want %q", messageStartPayload.Role, hookMessageRoleAssistant)
	}
	if messageStartPayload.DeltaType != hookMessageDeltaTypeFull {
		t.Fatalf("message.start delta type = %q, want %q", messageStartPayload.DeltaType, hookMessageDeltaTypeFull)
	}
	if messageStartPayload.Text != "reply" {
		t.Fatalf("message.start text = %q, want %q", messageStartPayload.Text, "reply")
	}
	if messageDeltaPayload.MessageID != messageStartPayload.MessageID {
		t.Fatalf("message.delta message id = %q, want %q", messageDeltaPayload.MessageID, messageStartPayload.MessageID)
	}
	if messageDeltaPayload.DeltaType != hookMessageDeltaTypeText {
		t.Fatalf("message.delta delta type = %q, want %q", messageDeltaPayload.DeltaType, hookMessageDeltaTypeText)
	}
	if messagePersistedPayload.MessageSeq <= 0 {
		t.Fatalf(
			"session.message_persisted sequence = %d, want durable positive sequence",
			messagePersistedPayload.MessageSeq,
		)
	}
	if messagePersistedPayload.Text != "reply" {
		t.Fatalf("session.message_persisted text = %q, want %q", messagePersistedPayload.Text, "reply")
	}
	if messagePersistedPayload.Role != hookMessageRoleAssistant {
		t.Fatalf(
			"session.message_persisted role = %q, want %q",
			messagePersistedPayload.Role,
			hookMessageRoleAssistant,
		)
	}
	if messageEndPayload.MessageID != messageStartPayload.MessageID {
		t.Fatalf("message.end message id = %q, want %q", messageEndPayload.MessageID, messageStartPayload.MessageID)
	}
	if messageEndPayload.Text != "reply" {
		t.Fatalf("message.end text = %q, want %q", messageEndPayload.Text, "reply")
	}
	if turnEndPayload.TurnID != turnStartPayload.TurnID {
		t.Fatalf("turn.end turn id = %q, want %q", turnEndPayload.TurnID, turnStartPayload.TurnID)
	}
}

func TestMessageStartPatchUpdatesFirstAssistantChunk(t *testing.T) {
	t.Parallel()

	patched := "patched reply"
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-message-start",
			Event:        hookspkg.HookMessageStart,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-message-start": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.MessageStartPayload) (hookspkg.MessageStartPatch, error) {
					return hookspkg.MessageStartPatch{Text: &patched}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Text != patched {
		t.Fatalf("first event text = %q, want %q", events[0].Text, patched)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	assistantMessage := storedEventByType(t, stored, acp.EventTypeAgentMessage)
	if !strings.Contains(assistantMessage.Content, patched) {
		t.Fatalf("stored assistant content = %q, want patched reply", assistantMessage.Content)
	}
}

func TestMessageDeltaAsyncHooksDoNotBlockPromptStreaming(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHook := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseHook)

	hooks := hookspkg.NewHooks(
		hookspkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		hookspkg.WithAsyncWorkerCount(1),
		hookspkg.WithAsyncQueueCapacity(1),
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
			Name:         "observe-message-delta",
			Event:        hookspkg.HookMessageDelta,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}}),
		hookspkg.WithExecutorResolver(func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
			if strings.TrimSpace(decl.Name) != "observe-message-delta" {
				return nil, errors.New("unexpected hook name")
			}
			return hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPatch, error) {
					select {
					case started <- struct{}{}:
					default:
					}
					<-release
					return hookspkg.MessageDeltaPatch{}, nil
				},
			), nil
		}),
	)
	if err := hooks.Rebuild(testutil.Context(t)); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	t.Cleanup(hooks.Close)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	select {
	case event, ok := <-eventsCh:
		if !ok {
			t.Fatal("first prompt event channel read closed early, want agent message")
		}
		if event.Type != acp.EventTypeAgentMessage {
			t.Fatalf("first prompt event type = %q, want %q", event.Type, acp.EventTypeAgentMessage)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first prompt event; message.delta hook blocked streaming")
	}

	select {
	case event, ok := <-eventsCh:
		if !ok {
			t.Fatal("second prompt event channel read closed early, want done event")
		}
		if event.Type != acp.EventTypeDone {
			t.Fatalf("second prompt event type = %q, want %q", event.Type, acp.EventTypeDone)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for done event; message.delta hook blocked prompt completion")
	}

	select {
	case _, ok := <-eventsCh:
		if ok {
			t.Fatal("prompt event channel still open after done event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prompt event channel close")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async message.delta hook to start")
	}
	releaseHook()
}

func TestContextCompactionDispatchesHooksAndUsesPatchedParams(t *testing.T) {
	t.Parallel()

	session := &Session{
		ID:          "sess-context",
		AgentName:   "coder",
		Workspace:   "/tmp/workspace",
		WorkspaceID: "ws-context",
		Type:        SessionTypeUser,
		State:       StateActive,
	}

	var (
		prePayload     hookspkg.ContextPreCompactPayload
		compactPayload hookspkg.ContextPreCompactPayload
		postPayload    hookspkg.ContextPostCompactPayload
	)

	dispatcher := &spyHookDispatcher{
		dispatchContextPreCompactFn: func(_ context.Context, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPayload, error) {
			prePayload = payload
			patchedReason := "token_limit"
			patchedStrategy := "summary"
			payload.Reason = patchedReason
			payload.Strategy = patchedStrategy
			payload.ContextBlocks = []hookspkg.ContextBlock{{Kind: "summary", Text: "patched"}}
			return payload, nil
		},
		dispatchContextPostCompactFn: func(_ context.Context, payload hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			postPayload = payload
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
	result, err := h.manager.runContextCompaction(
		testutil.Context(t),
		session,
		"turn-compact",
		"manual",
		"noop",
		"",
		[]hookspkg.ContextBlock{{Kind: "note", Text: "before"}},
		func(_ context.Context, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			compactPayload = payload
			return hookspkg.ContextPostCompactPayload{
				Summary:       "after",
				ContextBlocks: []hookspkg.ContextBlock{{Kind: "summary", Text: "after"}},
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("runContextCompaction() error = %v", err)
	}

	if prePayload.Reason != "manual" || prePayload.Strategy != "noop" {
		t.Fatalf("pre-compaction payload = %#v, want original reason/strategy", prePayload)
	}
	if compactPayload.Reason != "token_limit" || compactPayload.Strategy != "summary" {
		t.Fatalf("compaction payload = %#v, want patched reason/strategy", compactPayload)
	}
	if len(compactPayload.ContextBlocks) != 1 || compactPayload.ContextBlocks[0].Text != "patched" {
		t.Fatalf("compaction context blocks = %#v, want patched blocks", compactPayload.ContextBlocks)
	}
	if postPayload.Summary != "after" {
		t.Fatalf("post-compaction summary = %q, want %q", postPayload.Summary, "after")
	}
	if postPayload.Reason != "token_limit" || postPayload.Strategy != "summary" {
		t.Fatalf("post-compaction reason/strategy = %#v, want patched values", postPayload)
	}
	if result.Summary != "after" {
		t.Fatalf("result summary = %q, want %q", result.Summary, "after")
	}
}

func TestPressureCompactionArchivesCoveredReplaySpans(t *testing.T) {
	t.Parallel()

	t.Run("Should fire once above pressure and cover the summary before archive", func(t *testing.T) {
		t.Parallel()

		order := make([]string, 0, 3)
		var active *Session
		handler := &compactionHandlerStub{compact: func(
			_ context.Context,
			request CompactionRequest,
		) (CompactionResult, error) {
			order = append(order, "summary")
			if request.FromSequence != 1 || request.ToSequence != 2 || len(request.Events) != 2 {
				return CompactionResult{}, fmt.Errorf("request = %#v, want complete sequence range 1..2", request)
			}
			archived, err := active.recorderHandle().Query(
				testutil.Context(t),
				store.EventQuery{Archive: store.EventArchiveArchived},
			)
			if err != nil {
				return CompactionResult{}, err
			}
			if len(archived) != 0 {
				return CompactionResult{}, fmt.Errorf("archived before summary coverage: %#v", archived)
			}
			return CompactionResult{Summary: "Covered cobalt decision."}, nil
		}}
		dispatcher := &spyHookDispatcher{
			dispatchContextPreCompactFn: func(
				_ context.Context,
				payload hookspkg.ContextPreCompactPayload,
			) (hookspkg.ContextPreCompactPayload, error) {
				order = append(order, "pre")
				return payload, nil
			},
			dispatchContextPostCompactFn: func(
				_ context.Context,
				payload hookspkg.ContextPostCompactPayload,
			) (hookspkg.ContextPostCompactPayload, error) {
				archived, err := active.recorderHandle().Query(
					testutil.Context(t),
					store.EventQuery{Archive: store.EventArchiveArchived},
				)
				if err != nil {
					return payload, err
				}
				if len(archived) != 2 {
					return payload, fmt.Errorf("post hook archived rows = %d, want 2", len(archived))
				}
				order = append(order, "post")
				return payload, nil
			},
		}
		h := newHarness(
			t,
			WithHookSet(fullHookSet(dispatcher)),
			WithSessionCompactionConfig(aghconfig.SessionCompactionConfig{
				Enabled:            true,
				PressureThreshold:  0.85,
				MaxAttemptsPerTurn: 1,
				FailureCooldown:    10 * time.Minute,
			}),
			WithCompactionHandler(handler),
		)
		active = createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				active.ID,
			); err != nil && !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() error = %v", err)
			}
		})
		recordCompactionTurn(t, h.manager, active, "turn-old", "cobalt decision", true)
		used, size := int64(90), int64(100)
		usage := acp.TokenUsage{TurnID: "turn-current", ContextUsed: &used, ContextSize: &size}
		below := int64(80)
		if err := h.manager.maybeCompact(active, acp.TokenUsage{
			TurnID: "turn-current", ContextUsed: &below, ContextSize: &size,
		}); err != nil {
			t.Fatalf("maybeCompact(below) error = %v", err)
		}
		if handler.callCount() != 0 {
			t.Fatalf("handler calls below threshold = %d, want 0", handler.callCount())
		}
		if err := h.manager.observeRecordAndNotifyPromptEvent(
			testutil.Context(t),
			active,
			nil,
			&promptPumpLoopState{},
			acp.AgentEvent{
				Type:      acp.EventTypeUsage,
				TurnID:    usage.TurnID,
				Timestamp: time.Now().UTC(),
				Usage:     &usage,
			},
			false,
		); err != nil {
			t.Fatalf("observeRecordAndNotifyPromptEvent(usage) error = %v", err)
		}
		if err := h.manager.waitForCompactions(testutil.Context(t)); err != nil {
			t.Fatalf("waitForCompactions() error = %v", err)
		}
		if got := strings.Join(order, ","); got != "pre,summary,post" {
			t.Fatalf("compaction order = %q, want pre,summary,post", got)
		}
		if handler.callCount() != 1 {
			t.Fatalf("handler calls = %d, want 1", handler.callCount())
		}
		if err := h.manager.maybeCompact(active, usage); err != nil {
			t.Fatalf("maybeCompact(repeated turn) error = %v", err)
		}
		if handler.callCount() != 1 {
			t.Fatalf("handler calls after per-turn retry = %d, want 1", handler.callCount())
		}

		all, err := active.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if err != nil {
			t.Fatalf("Query(all) error = %v", err)
		}
		archived, err := active.recorderHandle().Query(
			testutil.Context(t),
			store.EventQuery{Archive: store.EventArchiveArchived},
		)
		if err != nil {
			t.Fatalf("Query(archived) error = %v", err)
		}
		if len(all) != 4 || len(archived) != 2 {
			t.Fatalf("all/archived rows = %d/%d, want 4/2", len(all), len(archived))
		}
		fired := storedEventByType(t, all, eventspkg.SessionCompactionFired)
		payload := decodeStoredEventPayload(t, fired)
		raw, ok := payload["raw"].(map[string]any)
		if !ok {
			t.Fatalf("compaction event raw = %#v, want object", payload["raw"])
		}
		if raw["workspace_id"] != active.WorkspaceID || raw["session_id"] != active.ID ||
			raw["from_sequence"] != float64(1) || raw["to_sequence"] != float64(2) {
			t.Fatalf("compaction event raw = %#v, want mandatory identity and range", raw)
		}
	})

	t.Run("Should enforce attempt caps and failure cooldown without archiving", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
		providerErr := errors.New("checkpoint provider unavailable")
		handler := &compactionHandlerStub{}
		handler.compact = func(_ context.Context, _ CompactionRequest) (CompactionResult, error) {
			if handler.callCount() == 1 {
				return CompactionResult{}, providerErr
			}
			return CompactionResult{Summary: "Recovered coverage."}, nil
		}
		h := newHarness(
			t,
			WithNow(func() time.Time { return now }),
			WithSessionCompactionConfig(aghconfig.SessionCompactionConfig{
				Enabled:            true,
				PressureThreshold:  0.85,
				MaxAttemptsPerTurn: 1,
				FailureCooldown:    10 * time.Minute,
			}),
			WithCompactionHandler(handler),
		)
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				active.ID,
			); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() error = %v", err)
			}
		})
		recordCompactionTurn(t, h.manager, active, "turn-old", "stable context", true)
		used, size := int64(90), int64(100)
		first := acp.TokenUsage{TurnID: "turn-failed", ContextUsed: &used, ContextSize: &size}
		if err := h.manager.maybeCompact(active, first); err != nil {
			t.Fatalf("maybeCompact(first) error = %v", err)
		}
		if err := h.manager.waitForCompactions(testutil.Context(t)); err != nil {
			t.Fatalf("waitForCompactions(first) error = %v", err)
		}
		archived, err := active.recorderHandle().Query(
			testutil.Context(t),
			store.EventQuery{Archive: store.EventArchiveArchived},
		)
		if err != nil {
			t.Fatalf("Query(after failure) error = %v", err)
		}
		if len(archived) != 0 {
			t.Fatalf("archived after failed summary = %#v, want none", archived)
		}

		if err := h.manager.maybeCompact(active, first); err != nil {
			t.Fatalf("maybeCompact(same turn) error = %v", err)
		}
		second := acp.TokenUsage{TurnID: "turn-cooldown", ContextUsed: &used, ContextSize: &size}
		if err := h.manager.maybeCompact(active, second); err != nil {
			t.Fatalf("maybeCompact(cooldown) error = %v", err)
		}
		if handler.callCount() != 1 {
			t.Fatalf("handler calls during cap/cooldown = %d, want 1", handler.callCount())
		}
		now = now.Add(11 * time.Minute)
		if err := h.manager.maybeCompact(active, second); err != nil {
			t.Fatalf("maybeCompact(after cooldown) error = %v", err)
		}
		if err := h.manager.waitForCompactions(testutil.Context(t)); err != nil {
			t.Fatalf("waitForCompactions(second) error = %v", err)
		}
		if handler.callCount() != 2 {
			t.Fatalf("handler calls after cooldown = %d, want 2", handler.callCount())
		}
	})

	t.Run("Should disable compaction and hooks at zero pressure", func(t *testing.T) {
		t.Parallel()

		preCalls := 0
		dispatcher := &spyHookDispatcher{dispatchContextPreCompactFn: func(
			_ context.Context,
			payload hookspkg.ContextPreCompactPayload,
		) (hookspkg.ContextPreCompactPayload, error) {
			preCalls++
			return payload, nil
		}}
		handler := &compactionHandlerStub{}
		h := newHarness(
			t,
			WithHookSet(fullHookSet(dispatcher)),
			WithSessionCompactionConfig(aghconfig.SessionCompactionConfig{
				Enabled:            true,
				PressureThreshold:  0,
				MaxAttemptsPerTurn: 1,
				FailureCooldown:    time.Minute,
			}),
			WithCompactionHandler(handler),
		)
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				active.ID,
			); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() error = %v", err)
			}
		})
		used, size := int64(100), int64(100)
		if err := h.manager.maybeCompact(active, acp.TokenUsage{
			TurnID: "turn-disabled", ContextUsed: &used, ContextSize: &size,
		}); err != nil {
			t.Fatalf("maybeCompact(disabled) error = %v", err)
		}
		if handler.callCount() != 0 || preCalls != 0 {
			t.Fatalf("disabled handler/pre-hook calls = %d/%d, want 0/0", handler.callCount(), preCalls)
		}
	})

	t.Run("Should stop at the first incomplete prior turn", func(t *testing.T) {
		t.Parallel()

		events := []store.SessionEvent{
			{Sequence: 1, TurnID: "turn-complete", Type: acp.EventTypeUserMessage},
			{Sequence: 2, TurnID: "turn-complete", Type: acp.EventTypeDone},
			{Sequence: 3, TurnID: "turn-incomplete", Type: acp.EventTypeAgentMessage},
			{Sequence: 4, TurnID: "turn-later", Type: acp.EventTypeUserMessage},
			{Sequence: 5, TurnID: "turn-later", Type: acp.EventTypeDone},
			{Sequence: 6, TurnID: "turn-current", Type: acp.EventTypeUsage},
		}
		span := completePriorTurnPrefix(events, "turn-current")
		if len(span) != 2 || span[0].Sequence != 1 || span[1].Sequence != 2 {
			t.Fatalf("completePriorTurnPrefix() = %#v, want only first complete turn", span)
		}
	})

	t.Run("Should include standalone markers before later complete turns", func(t *testing.T) {
		t.Parallel()

		events := []store.SessionEvent{
			{Sequence: 1, TurnID: "turn-marker", Type: eventspkg.TranscriptMarkerCreated},
			{Sequence: 2, TurnID: "clarify:req-1", Type: acp.EventTypeClarify},
			{Sequence: 3, TurnID: "turn-complete", Type: acp.EventTypeUserMessage},
			{Sequence: 4, TurnID: "turn-complete", Type: acp.EventTypeDone},
			{Sequence: 5, TurnID: "turn-current", Type: acp.EventTypeUsage},
		}
		span := completePriorTurnPrefix(events, "turn-current")
		if len(span) != 4 || span[0].Sequence != 1 || span[3].Sequence != 4 {
			t.Fatalf("completePriorTurnPrefix() = %#v, want standalone events plus complete turn", span)
		}
	})

	t.Run("Should join a canceled compaction before closing the recorder", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		canceled := make(chan struct{})
		release := make(chan struct{})
		handler := &compactionHandlerStub{compact: func(
			ctx context.Context,
			_ CompactionRequest,
		) (CompactionResult, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
			return CompactionResult{}, ctx.Err()
		}}
		h := newHarness(
			t,
			WithSessionCompactionConfig(aghconfig.DefaultSessionCompactionConfig()),
			WithCompactionHandler(handler),
		)
		active := createSession(t, h)
		recordCompactionTurn(t, h.manager, active, "turn-old", "durable context", true)
		used, size := int64(90), int64(100)
		if err := h.manager.maybeCompact(active, acp.TokenUsage{
			TurnID: "turn-current", ContextUsed: &used, ContextSize: &size,
		}); err != nil {
			t.Fatalf("maybeCompact() error = %v", err)
		}
		<-started

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.Stop(testutil.Context(t), active.ID)
		}()
		<-canceled
		select {
		case err := <-stopDone:
			t.Fatalf("Stop() returned before compaction joined: %v", err)
		default:
		}
		if _, err := active.recorderHandle().Query(testutil.Context(t), store.EventQuery{}); err != nil {
			t.Fatalf("Query() while stop waits for compaction error = %v", err)
		}
		close(release)
		if err := <-stopDone; err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if active.recorderHandle() != nil {
			t.Fatal("recorderHandle() != nil after joined stop")
		}
	})
}

type compactionHandlerStub struct {
	mu      sync.Mutex
	calls   []CompactionRequest
	compact func(context.Context, CompactionRequest) (CompactionResult, error)
}

func (s *compactionHandlerStub) CompactSessionContext(
	ctx context.Context,
	request CompactionRequest,
) (CompactionResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, request)
	compact := s.compact
	s.mu.Unlock()
	if compact == nil {
		return CompactionResult{}, nil
	}
	return compact(ctx, request)
}

func (s *compactionHandlerStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func recordCompactionTurn(
	t *testing.T,
	manager *Manager,
	session *Session,
	turnID string,
	text string,
	complete bool,
) {
	t.Helper()
	for _, event := range []acp.AgentEvent{
		{Type: acp.EventTypeUserMessage, TurnID: turnID, Text: text, Timestamp: time.Now().UTC()},
		{Type: acp.EventTypeDone, TurnID: turnID, Timestamp: time.Now().UTC()},
	} {
		if !complete && event.Type == acp.EventTypeDone {
			continue
		}
		if err := manager.recordEvent(testutil.Context(t), session, event); err != nil {
			t.Fatalf("recordEvent(%s) error = %v", event.Type, err)
		}
	}
}

func newNativeHookDispatcher(
	t *testing.T,
	decls []hookspkg.HookDecl,
	executors map[string]hookspkg.Executor,
) *hookspkg.Hooks {
	t.Helper()

	hooks := hookspkg.NewHooks(
		hookspkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		hookspkg.WithAsyncWorkerCount(1),
		hookspkg.WithAsyncQueueCapacity(16),
		hookspkg.WithNativeDeclarations(decls),
		hookspkg.WithExecutorResolver(func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
			executor := executors[strings.TrimSpace(decl.Name)]
			if executor == nil {
				return nil, errors.New("missing native executor")
			}
			return executor, nil
		}),
	)
	if err := hooks.Rebuild(testutil.Context(t)); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	t.Cleanup(hooks.Close)
	return hooks
}

func fullHookSet(runtime interface {
	LifecycleHooks
	PromptHooks
	EventHooks
	AgentHooks
	ConversationHooks
	CompactionHooks
}) HookSet {
	return HookSet{
		Session:      runtime,
		Prompt:       runtime,
		Events:       runtime,
		Agent:        runtime,
		Conversation: runtime,
		Compaction:   runtime,
	}
}

type spyHookDispatcher struct {
	dispatchSessionPreCreateFn        func(context.Context, hookspkg.SessionPreCreatePayload) (hookspkg.SessionPreCreatePayload, error)
	dispatchSessionPostCreateFn       func(context.Context, hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePayload, error)
	dispatchSessionPreResumeFn        func(context.Context, hookspkg.SessionPreResumePayload) (hookspkg.SessionPreResumePayload, error)
	dispatchSessionPostResumeFn       func(context.Context, hookspkg.SessionPostResumePayload) (hookspkg.SessionPostResumePayload, error)
	dispatchSessionPreStopFn          func(context.Context, hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error)
	dispatchSessionPostStopFn         func(context.Context, hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error)
	dispatchInputPreSubmitFn          func(context.Context, hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error)
	dispatchPromptPostAssembleFn      func(context.Context, hookspkg.PromptPayload) (hookspkg.PromptPayload, error)
	dispatchEventPreRecordFn          func(context.Context, hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error)
	dispatchEventPostRecordFn         func(context.Context, hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error)
	dispatchAgentPreStartFn           func(context.Context, hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error)
	dispatchAgentSpawnedFn            func(context.Context, hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error)
	dispatchAgentCrashedFn            func(context.Context, hookspkg.AgentCrashedPayload) (hookspkg.AgentCrashedPayload, error)
	dispatchAgentStoppedFn            func(context.Context, hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error)
	dispatchTurnStartFn               func(context.Context, hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error)
	dispatchTurnEndFn                 func(context.Context, hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error)
	dispatchMessageStartFn            func(context.Context, hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error)
	dispatchMessageDeltaFn            func(context.Context, hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error)
	dispatchMessageEndFn              func(context.Context, hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error)
	dispatchSessionMessagePersistedFn func(
		context.Context,
		hookspkg.SessionMessagePersistedPayload,
	) (hookspkg.SessionMessagePersistedPayload, error)
	dispatchContextPreCompactFn  func(context.Context, hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPayload, error)
	dispatchContextPostCompactFn func(context.Context, hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPayload, error)
}

func (s *spyHookDispatcher) DispatchSessionPreCreate(
	ctx context.Context,
	payload hookspkg.SessionPreCreatePayload,
) (hookspkg.SessionPreCreatePayload, error) {
	if s.dispatchSessionPreCreateFn != nil {
		return s.dispatchSessionPreCreateFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPostCreate(
	ctx context.Context,
	payload hookspkg.SessionPostCreatePayload,
) (hookspkg.SessionPostCreatePayload, error) {
	if s.dispatchSessionPostCreateFn != nil {
		return s.dispatchSessionPostCreateFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPreResume(
	ctx context.Context,
	payload hookspkg.SessionPreResumePayload,
) (hookspkg.SessionPreResumePayload, error) {
	if s.dispatchSessionPreResumeFn != nil {
		return s.dispatchSessionPreResumeFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPostResume(
	ctx context.Context,
	payload hookspkg.SessionPostResumePayload,
) (hookspkg.SessionPostResumePayload, error) {
	if s.dispatchSessionPostResumeFn != nil {
		return s.dispatchSessionPostResumeFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPreStop(
	ctx context.Context,
	payload hookspkg.SessionPreStopPayload,
) (hookspkg.SessionPreStopPayload, error) {
	if s.dispatchSessionPreStopFn != nil {
		return s.dispatchSessionPreStopFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPostStop(
	ctx context.Context,
	payload hookspkg.SessionPostStopPayload,
) (hookspkg.SessionPostStopPayload, error) {
	if s.dispatchSessionPostStopFn != nil {
		return s.dispatchSessionPostStopFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchInputPreSubmit(
	ctx context.Context,
	payload hookspkg.InputPreSubmitPayload,
) (hookspkg.InputPreSubmitPayload, error) {
	if s.dispatchInputPreSubmitFn != nil {
		return s.dispatchInputPreSubmitFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchPromptPostAssemble(
	ctx context.Context,
	payload hookspkg.PromptPayload,
) (hookspkg.PromptPayload, error) {
	if s.dispatchPromptPostAssembleFn != nil {
		return s.dispatchPromptPostAssembleFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchEventPreRecord(
	ctx context.Context,
	payload hookspkg.EventPreRecordPayload,
) (hookspkg.EventPreRecordPayload, error) {
	if s.dispatchEventPreRecordFn != nil {
		return s.dispatchEventPreRecordFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchEventPostRecord(
	ctx context.Context,
	payload hookspkg.EventPostRecordPayload,
) (hookspkg.EventPostRecordPayload, error) {
	if s.dispatchEventPostRecordFn != nil {
		return s.dispatchEventPostRecordFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentPreStart(
	ctx context.Context,
	payload hookspkg.AgentPreStartPayload,
) (hookspkg.AgentPreStartPayload, error) {
	if s.dispatchAgentPreStartFn != nil {
		return s.dispatchAgentPreStartFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentSpawned(
	ctx context.Context,
	payload hookspkg.AgentSpawnedPayload,
) (hookspkg.AgentSpawnedPayload, error) {
	if s.dispatchAgentSpawnedFn != nil {
		return s.dispatchAgentSpawnedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentCrashed(
	ctx context.Context,
	payload hookspkg.AgentCrashedPayload,
) (hookspkg.AgentCrashedPayload, error) {
	if s.dispatchAgentCrashedFn != nil {
		return s.dispatchAgentCrashedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentStopped(
	ctx context.Context,
	payload hookspkg.AgentStoppedPayload,
) (hookspkg.AgentStoppedPayload, error) {
	if s.dispatchAgentStoppedFn != nil {
		return s.dispatchAgentStoppedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchTurnStart(
	ctx context.Context,
	payload hookspkg.TurnStartPayload,
) (hookspkg.TurnStartPayload, error) {
	if s.dispatchTurnStartFn != nil {
		return s.dispatchTurnStartFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchTurnEnd(
	ctx context.Context,
	payload hookspkg.TurnEndPayload,
) (hookspkg.TurnEndPayload, error) {
	if s.dispatchTurnEndFn != nil {
		return s.dispatchTurnEndFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchMessageStart(
	ctx context.Context,
	payload hookspkg.MessageStartPayload,
) (hookspkg.MessageStartPayload, error) {
	if s.dispatchMessageStartFn != nil {
		return s.dispatchMessageStartFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchMessageDelta(
	ctx context.Context,
	payload hookspkg.MessageDeltaPayload,
) (hookspkg.MessageDeltaPayload, error) {
	if s.dispatchMessageDeltaFn != nil {
		return s.dispatchMessageDeltaFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchMessageEnd(
	ctx context.Context,
	payload hookspkg.MessageEndPayload,
) (hookspkg.MessageEndPayload, error) {
	if s.dispatchMessageEndFn != nil {
		return s.dispatchMessageEndFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionMessagePersisted(
	ctx context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) (hookspkg.SessionMessagePersistedPayload, error) {
	if s.dispatchSessionMessagePersistedFn != nil {
		return s.dispatchSessionMessagePersistedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchContextPreCompact(
	ctx context.Context,
	payload hookspkg.ContextPreCompactPayload,
) (hookspkg.ContextPreCompactPayload, error) {
	if s.dispatchContextPreCompactFn != nil {
		return s.dispatchContextPreCompactFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchContextPostCompact(
	ctx context.Context,
	payload hookspkg.ContextPostCompactPayload,
) (hookspkg.ContextPostCompactPayload, error) {
	if s.dispatchContextPostCompactFn != nil {
		return s.dispatchContextPostCompactFn(ctx, payload)
	}
	return payload, nil
}

type orderedRecorder struct {
	onRecord func(store.SessionEvent)
	nextSeq  int64
	events   []store.SessionEvent
}

type recordingNetworkPeerLifecycle struct {
	joinErr  error
	leaveErr error
	joins    []networkJoinCall
	leaves   []string
}

type networkJoinCall struct {
	sessionID    string
	peerID       string
	channel      string
	capabilities []NetworkPeerCapability
}

func (r *recordingNetworkPeerLifecycle) JoinChannel(
	_ context.Context,
	join NetworkPeerJoin,
) error {
	r.joins = append(r.joins, networkJoinCall{
		sessionID:    join.SessionID,
		peerID:       join.PeerID,
		channel:      join.Channel,
		capabilities: cloneNetworkPeerCapabilities(join.Capabilities),
	})
	return r.joinErr
}

func (r *recordingNetworkPeerLifecycle) LeaveChannel(_ context.Context, sessionID string) error {
	r.leaves = append(r.leaves, sessionID)
	return r.leaveErr
}

func (r *recordingNetworkPeerLifecycle) joinCount() int {
	return len(r.joins)
}

func (r *recordingNetworkPeerLifecycle) leaveCount() int {
	return len(r.leaves)
}

func (r *orderedRecorder) Record(_ context.Context, event store.SessionEvent) error {
	_, err := r.RecordPersisted(context.Background(), event)
	return err
}

func (r *orderedRecorder) RecordPersisted(_ context.Context, event store.SessionEvent) (store.SessionEvent, error) {
	if event.ID == "" {
		event.ID = store.NewID("ev")
	}
	if event.Sequence <= 0 {
		r.nextSeq++
		event.Sequence = r.nextSeq
	}
	r.events = append(r.events, event)
	if r.onRecord != nil {
		r.onRecord(event)
	}
	return event, nil
}

func (r *orderedRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *orderedRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return append([]store.SessionEvent(nil), r.events...), nil
}

func (r *orderedRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *orderedRecorder) Close(context.Context) error {
	return nil
}
