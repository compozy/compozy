package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/network/participation"
	skillspkg "github.com/compozy/compozy/internal/skills"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	skillbundled "github.com/compozy/compozy/skills"
	shellquote "github.com/kballard/go-shellquote"
)

type blockingSessionCreatedNotifier struct {
	Notifier
	worktreeID string
	entered    chan struct{}
	release    chan struct{}
}

func (n *blockingSessionCreatedNotifier) OnSessionCreated(ctx context.Context, session *Session) {
	if session.Info().WorktreeID == n.worktreeID {
		close(n.entered)
		<-n.release
	}
	n.Notifier.OnSessionCreated(ctx, session)
}

func TestSupervisionForSessionShouldDisableOnlyLoopInactivityTimers(t *testing.T) {
	t.Parallel()

	configured := compozyconfig.SessionSupervisionConfig{
		ActivityHeartbeatInterval: 5 * time.Second,
		ProgressNotifyInterval:    7 * time.Second,
		PromptDeadline:            time.Hour,
		InactivityWarningAfter:    15 * time.Minute,
		InactivityTimeout:         30 * time.Minute,
		TimeoutCancelGrace:        5 * time.Second,
	}
	loopSession := &Session{NetworkOwnerKey: participation.OwnerKey(participation.OwnerRef{
		Kind: participation.OwnerKindLoopRun,
		ID:   "run-1",
	})}
	got := supervisionForSession(loopSession, configured)
	if got.InactivityWarningAfter != 0 || got.InactivityTimeout != 0 {
		t.Fatalf("Loop inactivity supervision = %#v, want warning and timeout disabled", got)
	}
	got.InactivityWarningAfter = configured.InactivityWarningAfter
	got.InactivityTimeout = configured.InactivityTimeout
	if got != configured {
		t.Fatalf("Loop supervision changed unrelated fields: got %#v want %#v", got, configured)
	}

	nonLoop := supervisionForSession(&Session{NetworkOwnerKey: "session:sess-1"}, configured)
	if nonLoop != configured {
		t.Fatalf("non-Loop supervision = %#v, want configured %#v", nonLoop, configured)
	}
}

func TestCreateOpensStoreRegistersSessionAndActivates(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "primary",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := session.Info().State; got != StateActive {
		t.Fatalf("Create() state = %q, want %q", got, StateActive)
	}
	if got, ok := h.manager.Get(session.ID); !ok || got != session {
		t.Fatalf("Get(%q) = (%v, %v), want created session", session.ID, got, ok)
	}
	if got := h.notifier.createdCount(); got != 1 {
		t.Fatalf("created notifications = %d, want 1", got)
	}
	if meta := readMeta(t, session.MetaPath()); meta.State != string(StateActive) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateActive)
	}
	if got := session.Info().WorkspaceID; got != h.workspaceID {
		t.Fatalf("session workspace id = %q, want %q", got, h.workspaceID)
	}
	if meta := readMeta(t, session.MetaPath()); meta.WorkspaceID != h.workspaceID {
		t.Fatalf("meta workspace id = %q, want %q", meta.WorkspaceID, h.workspaceID)
	}
	canonicalWorkspace, err := canonicalDirectory(h.workspace)
	if err != nil {
		t.Fatalf("canonicalDirectory(workspace) error = %v", err)
	}
	if got := h.driver.startCalls[0].Cwd; got != canonicalWorkspace {
		t.Fatalf("start cwd = %q, want %q", got, canonicalWorkspace)
	}
	if got := session.Info().Type; got != SessionTypeUser {
		t.Fatalf("Create() type = %q, want %q", got, SessionTypeUser)
	}
	if got, want := session.Info().NetworkParticipation, participation.LocalSpec(); got != want {
		t.Fatalf("Create() participation = %#v, want %#v", got, want)
	}
	if meta := readMeta(t, session.MetaPath()); meta.SessionType != string(SessionTypeUser) {
		t.Fatalf("meta session type = %q, want %q", meta.SessionType, SessionTypeUser)
	}
	if meta := readMeta(t, session.MetaPath()); meta.NetworkSpecSnapshot() != participation.LocalSpec() {
		t.Fatalf("meta participation = %#v, want Local", meta.NetworkSpecSnapshot())
	}
	if got := len(h.resolver.resolveCalls); got != 2 {
		t.Fatalf("resolver Resolve() calls = %d, want 2", got)
	}
	for index, got := range h.resolver.resolveCalls {
		if got != h.workspaceID {
			t.Fatalf("resolver Resolve() call %d arg = %q, want %q", index, got, h.workspaceID)
		}
	}
	if got := h.resolver.profileResolveNames; !slices.Equal(got, []string{"default"}) {
		t.Fatalf("resolver profile names = %#v, want default for session creation", got)
	}
	if got := len(h.resolver.resolveOrRegisterCalls); got != 0 {
		t.Fatalf("resolver ResolveOrRegister() calls = %d, want 0", got)
	}
}

func TestSessionWorktreeBinding(t *testing.T) {
	t.Parallel()

	t.Run("Should start a bound session at the worktree root and persist the binding", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		worktreeRoot := filepath.Join(t.TempDir(), "worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		resolver := &fakeSessionWorktreeResolver{id: "wt-ready", root: worktreeRoot}
		h.manager = newManagerWithHarness(t, h, WithWorktreeResolver(resolver))

		created, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "ready-ref",
		})
		if err != nil {
			t.Fatalf("Create(bound) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, created.ID) })

		canonicalRoot, err := canonicalDirectory(worktreeRoot)
		if err != nil {
			t.Fatalf("canonicalDirectory(worktree root) error = %v", err)
		}
		if got := h.driver.startCalls[0].Cwd; got != canonicalRoot {
			t.Fatalf("bound start cwd = %q, want %q", got, canonicalRoot)
		}
		if got := h.driver.startCalls[0].AdditionalDirs; got != nil {
			t.Fatalf("bound additional dirs = %#v, want nil", got)
		}
		if got := created.Info().WorktreeID; got != "wt-ready" {
			t.Fatalf("Info.WorktreeID = %q, want wt-ready", got)
		}
		if got := readMeta(t, created.MetaPath()).WorktreeIDValue(); got != "wt-ready" {
			t.Fatalf("meta.WorktreeID = %q, want wt-ready", got)
		}
		if got, want := resolver.callsSnapshot(), []sessionWorktreeResolveCall{
			{workspaceID: h.workspaceID, ref: "ready-ref"},
			{workspaceID: h.workspaceID, ref: "wt-ready"},
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("worktree resolver calls = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject a bound cwd outside the worktree without falling back to the workspace", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		worktreeRoot := filepath.Join(t.TempDir(), "worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		h.manager = newManagerWithHarness(t, h, WithWorktreeResolver(&fakeSessionWorktreeResolver{
			id: "wt-ready", root: worktreeRoot,
		}))

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-ready", CWD: h.workspace,
		})
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "escapes root") {
			t.Fatalf("Create(bound escape) error = %v, want ErrValidation escape", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver starts after bound escape = %d, want 0", got)
		}
	})

	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "not found", err: errors.New("worktree_not_found")},
		{name: "not ready", err: errors.New("worktree_not_ready")},
		{name: "missing", err: errors.New("worktree_missing")},
	} {
		t.Run("Should refuse a "+test.name+" binding before runtime start", func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			h.manager = newManagerWithHarness(t, h, WithWorktreeResolver(&fakeSessionWorktreeResolver{err: test.err}))
			_, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder", Workspace: h.workspaceID, Worktree: "target",
			})
			if !errors.Is(err, test.err) {
				t.Fatalf("Create(%s binding) error = %v, want %v", test.name, err, test.err)
			}
			if got := len(h.driver.startCalls); got != 0 {
				t.Fatalf("driver starts after %s binding = %d, want 0", test.name, got)
			}
		})
	}

	t.Run("Should refuse a binding removed after selection but before runtime start", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		worktreeRoot := filepath.Join(t.TempDir(), "worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		missingErr := errors.New("worktree_missing")
		resolveCalls := 0
		resolver := &fakeSessionWorktreeResolver{resolve: func(
			_ context.Context,
			_ string,
			_ string,
		) (string, string, error) {
			resolveCalls++
			if resolveCalls == 1 {
				return "wt-race", worktreeRoot, nil
			}
			return "", "", missingErr
		}}
		h.manager = newManagerWithHarness(t, h, WithWorktreeResolver(resolver))

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-race",
		})
		if !errors.Is(err, missingErr) {
			t.Fatalf("Create(removed before start) error = %v, want %v", err, missingErr)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver starts after removal race = %d, want 0", got)
		}
		if got := len(h.manager.List()); got != 0 {
			t.Fatalf("active sessions after removal race = %d, want 0", got)
		}
	})

	t.Run("Should reserve an idle origin until the worktree fork child is accepted", func(t *testing.T) {
		t.Parallel()

		worktreeRoot := filepath.Join(t.TempDir(), "fork-worktree")
		if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(worktree root) error = %v", err)
		}
		childAccepted := make(chan struct{})
		releaseChildAcceptance := make(chan struct{})
		notifier := &blockingSessionCreatedNotifier{
			Notifier: newFakeNotifier(), worktreeID: "wt-fork",
			entered: childAccepted, release: releaseChildAcceptance,
		}
		h := newHarness(
			t,
			WithWorktreeResolver(&fakeSessionWorktreeResolver{id: "wt-fork", root: worktreeRoot}),
			WithNotifier(notifier),
		)
		origin := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, origin.ID) })

		createdResult := make(chan *Info, 1)
		errResult := make(chan error, 1)
		go func() {
			created, err := h.manager.CreateWorktreeForkAccepted(
				testutil.Context(t),
				origin.ID,
				CreateAcceptedOpts{Session: CreateOpts{
					AgentName: "coder", Workspace: h.workspaceID, Worktree: "wt-fork", Type: SessionTypeUser,
				}},
			)
			createdResult <- created
			errResult <- err
		}()

		select {
		case <-childAccepted:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for fork child acceptance")
		}
		if _, err := origin.beginExclusivePromptSetup(); !errors.Is(err, ErrPromptInProgress) {
			t.Fatalf("beginExclusivePromptSetup(during fork) error = %v, want ErrPromptInProgress", err)
		}
		close(releaseChildAcceptance)
		if err := <-errResult; err != nil {
			t.Fatalf("CreateWorktreeForkAccepted() error = %v", err)
		}
		child := <-createdResult
		if child == nil || child.WorktreeID != "wt-fork" {
			t.Fatalf("fork child = %#v, want wt-fork binding", child)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })
	})
}

func TestManagerPublishClarifyEvent(t *testing.T) {
	t.Run("Should preserve typed clarification evidence in the canonical transcript payload", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		askedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
		choice := 1
		clarifyEvent := toolspkg.ClarifyEvent{
			Status: toolspkg.ClarifyStatusResolved,
			Request: toolspkg.ClarifyPending{
				RequestID:   "clarify-request-1",
				WorkspaceID: h.workspaceID,
				SessionID:   sess.ID,
				AgentName:   sess.Info().AgentName,
				Question:    "Which release channel?",
				Choices:     []string{"Stable", "Preview"},
				AskedAt:     askedAt,
				Deadline:    askedAt.Add(5 * time.Minute),
			},
			Answer: &toolspkg.ClarifyAnswer{Choice: &choice},
			At:     askedAt.Add(time.Minute),
		}

		if err := h.manager.PublishClarifyEvent(testutil.Context(t), clarifyEvent); err != nil {
			t.Fatalf("PublishClarifyEvent() error = %v", err)
		}
		stored, err := sess.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if got, want := len(stored), 1; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}

		decoded, err := transcript.UnmarshalAgentEvent(stored[0].Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := decoded.Type, EventTypeClarify; got != want {
			t.Fatalf("event type = %q, want %q", got, want)
		}
		if got, want := decoded.RequestIDValue(), clarifyEvent.Request.RequestID; got != want {
			t.Fatalf("request id = %q, want %q", got, want)
		}
		var persisted toolspkg.ClarifyEvent
		if err := json.Unmarshal(decoded.Raw, &persisted); err != nil {
			t.Fatalf("json.Unmarshal(raw clarify event) error = %v", err)
		}
		if got, want := persisted.Status, clarifyEvent.Status; got != want {
			t.Fatalf("clarification status = %q, want %q", got, want)
		}
		if persisted.Answer == nil || persisted.Answer.Choice == nil || *persisted.Answer.Choice != 1 {
			t.Fatalf("clarification answer = %#v, want choice 1", persisted.Answer)
		}
	})

	t.Run(
		"Should persist terminal clarification evidence before stop finalization closes the recorder",
		func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			sess := createSession(t, h)
			askedAt := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)
			published := make(chan error, 1)
			h.notifier.finalizingHook = func(ctx context.Context, stopping *Session) {
				published <- h.manager.PublishClarifyEvent(ctx, toolspkg.ClarifyEvent{
					Status: toolspkg.ClarifyStatusCanceled,
					Request: toolspkg.ClarifyPending{
						RequestID:   "clarify-request-stop",
						WorkspaceID: h.workspaceID,
						SessionID:   stopping.ID,
						AgentName:   stopping.Info().AgentName,
						Question:    "Continue after shutdown?",
						AskedAt:     askedAt,
						Deadline:    askedAt.Add(5 * time.Minute),
					},
					At: askedAt.Add(time.Minute),
				})
			}

			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
			if err := <-published; err != nil {
				t.Fatalf("PublishClarifyEvent(finalizing) error = %v", err)
			}
			events, err := h.manager.Events(testutil.Context(t), sess.ID, store.EventQuery{Type: EventTypeClarify})
			if err != nil {
				t.Fatalf("Events(clarify) error = %v", err)
			}
			if got, want := len(events), 1; got != want {
				t.Fatalf("len(clarify events) = %d, want %d", got, want)
			}
			decoded, err := transcript.UnmarshalAgentEvent(events[0].Content)
			if err != nil {
				t.Fatalf("UnmarshalAgentEvent() error = %v", err)
			}
			var persisted toolspkg.ClarifyEvent
			if err := json.Unmarshal(decoded.Raw, &persisted); err != nil {
				t.Fatalf("json.Unmarshal(raw clarify event) error = %v", err)
			}
			if got, want := persisted.Status, toolspkg.ClarifyStatusCanceled; got != want {
				t.Fatalf("clarification status = %q, want %q", got, want)
			}
		})
}

func TestManagerWorkAdmission(t *testing.T) {
	t.Run("Should preserve an admitted prompt while rejecting new work", func(t *testing.T) {
		t.Parallel()

		gate := &admission.Gate{}
		h := newHarness(t, WithWorkAdmissionChecker(gate))
		sess := createSession(t, h)
		source := make(chan acp.AgentEvent, 1)
		promptStarted := make(chan struct{})
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			close(promptStarted)
			return source, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "finish this turn")
		if err != nil {
			t.Fatalf("Prompt(first) error = %v", err)
		}
		select {
		case <-promptStarted:
		case <-time.After(time.Second):
			t.Fatal("first prompt did not start")
		}
		if changed := gate.Drain(); !changed {
			t.Fatal("Drain() changed = false, want true")
		}

		if _, err := h.manager.Prompt(
			testutil.Context(t),
			sess.ID,
			"new turn",
		); !errors.Is(err, admission.ErrDraining) {
			t.Fatalf("Prompt(second) error = %v, want ErrDraining", err)
		}
		if _, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		}); !errors.Is(err, admission.ErrDraining) {
			t.Fatalf("Create() error = %v, want ErrDraining", err)
		}

		h.driver.mu.Lock()
		promptRequests := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
		h.driver.mu.Unlock()
		if len(promptRequests) == 0 {
			t.Fatal("driver Prompt() requests = 0, want the admitted prompt")
		}
		source <- acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           promptRequests[0].TurnID,
			Timestamp:        time.Now().UTC(),
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
		}
		close(source)
		var terminal acp.AgentEvent
		for event := range events {
			terminal = event
		}
		if terminal.Type != acp.EventTypeDone {
			t.Fatalf("admitted prompt terminal event = %q, want %q", terminal.Type, acp.EventTypeDone)
		}
	})

	t.Run("Should restore new work after undrain", func(t *testing.T) {
		t.Parallel()

		gate := &admission.Gate{}
		gate.Drain()
		h := newHarness(t, WithWorkAdmissionChecker(gate))
		if _, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		}); !errors.Is(err, admission.ErrDraining) {
			t.Fatalf("Create(draining) error = %v, want ErrDraining", err)
		}
		if changed := gate.Undrain(); !changed {
			t.Fatal("Undrain() changed = false, want true")
		}
		sess, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create(active) error = %v", err)
		}
		if stopErr := h.manager.Stop(testutil.Context(t), sess.ID); stopErr != nil {
			t.Fatalf("Stop() error = %v", stopErr)
		}
	})

	t.Run("Should finish an admitted dream continuation while public work is draining", func(t *testing.T) {
		t.Parallel()

		gate := &admission.Gate{}
		gate.Drain()
		h := newHarness(t, WithWorkAdmissionChecker(gate))
		if _, err := h.manager.CreateLifecycleContinuation(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		}); err == nil || !strings.Contains(err.Error(), string(SessionTypeDream)) {
			t.Fatalf("CreateLifecycleContinuation(user) error = %v, want dream-only validation", err)
		}

		continuation, err := h.manager.CreateLifecycleContinuation(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeDream,
		})
		if err != nil {
			t.Fatalf("CreateLifecycleContinuation(dream) error = %v", err)
		}
		if _, err := h.manager.Prompt(
			testutil.Context(t),
			continuation.ID,
			"public turn",
		); !errors.Is(err, admission.ErrDraining) {
			t.Fatalf("Prompt(public) error = %v, want ErrDraining", err)
		}

		var continuationRequest acp.PromptRequest
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			continuationRequest = req
			source := make(chan acp.AgentEvent, 1)
			source <- acp.AgentEvent{
				Type:             acp.EventTypeDone,
				TurnID:           req.TurnID,
				Timestamp:        time.Now().UTC(),
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}
			close(source)
			return source, nil
		}
		events, err := h.manager.PromptLifecycleContinuation(
			testutil.Context(t),
			continuation.ID,
			"finish accepted checkpoint",
		)
		if err != nil {
			t.Fatalf("PromptLifecycleContinuation() error = %v", err)
		}
		var terminal acp.AgentEvent
		for event := range events {
			terminal = event
		}
		if terminal.Type != acp.EventTypeDone {
			t.Fatalf("continuation terminal event = %q, want %q", terminal.Type, acp.EventTypeDone)
		}
		if continuationRequest.Meta.TurnSource != acp.PromptTurnSourceSynthetic {
			t.Fatalf(
				"continuation turn source = %q, want %q",
				continuationRequest.Meta.TurnSource,
				acp.PromptTurnSourceSynthetic,
			)
		}
		if continuationRequest.Meta.Synthetic == nil ||
			continuationRequest.Meta.Synthetic.Reason != promptReasonLifecycleContinuation {
			t.Fatalf(
				"continuation synthetic metadata = %#v, want lifecycle continuation reason",
				continuationRequest.Meta.Synthetic,
			)
		}
		if err := h.manager.Stop(testutil.Context(t), continuation.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})
}

func TestCreateAppliesRuntimeModelOverride(t *testing.T) {
	t.Parallel()

	t.Run("Should layer agent runtime defaults under explicit session ACP options", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[index].Name != "coder" {
				continue
			}
			resolvedWorkspace.Agents[index].SetSpeed(speedpkg.SpeedFast)
			resolvedWorkspace.Agents[index].SetACPOptions([]compozyconfig.ACPOptionSelection{
				{ID: "context", ValueID: "1m"},
				{ID: "thinking", BoolValue: new(true)},
			})
		}
		h.resolver.upsert(&resolvedWorkspace)

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			ACPOptions: []acp.SessionConfigOptionSelection{{
				ID: "context", ValueID: "200k",
			}},
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})

		start := h.driver.startCalls[0]
		if start.Speed != speedpkg.SpeedFast || len(start.ACPOptions) != 2 ||
			start.ACPOptions[0].ID != "context" || start.ACPOptions[0].ValueID != "200k" ||
			start.ACPOptions[1].ID != "thinking" || start.ACPOptions[1].BoolValue == nil ||
			!*start.ACPOptions[1].BoolValue {
			t.Fatalf(
				"StartOpts runtime = speed %q options %#v, want inherited fast plus session overlay",
				start.Speed,
				start.ACPOptions,
			)
		}
		info := session.Info()
		meta := readMeta(t, session.MetaPath())
		if info.Speed != speedpkg.SpeedFast || len(info.ACPOptions) != 2 ||
			meta.Speed != speedpkg.SpeedFast || len(meta.ACPOptionsValue()) != 2 {
			t.Fatalf(
				"effective runtime info=%#v meta=%#v, want materialized defaults",
				info.ACPOptions,
				meta.ACPOptions,
			)
		}
		creationProfile := session.Meta().CreationProfile
		if creationProfile == nil || creationProfile.Speed != speedpkg.SpeedFast ||
			len(creationProfile.ACPOptions) != 2 || creationProfile.ACPOptions[0].ID != "context" ||
			creationProfile.ACPOptions[0].ValueID != "200k" || creationProfile.ACPOptions[1].ID != "thinking" ||
			creationProfile.ACPOptions[1].BoolValue == nil || !*creationProfile.ACPOptions[1].BoolValue {
			t.Fatalf("creation profile runtime = %#v, want immutable effective defaults", creationProfile)
		}
	})

	t.Run("Should compile a logical Cursor model and options into the launch command", func(t *testing.T) {
		t.Parallel()

		const cursorModel = "grok-4.6"
		const transportModel = "cursor-grok-4.6-high-fast"
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
			ProviderID:        "cursor",
			ModelID:           cursorModel,
			AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
			SupportsReasoning: new(true),
			ReasoningEfforts:  []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortHigh},
			TransportBindings: []modelcatalog.ModelTransportBinding{{
				TransportModelID: transportModel,
				ReasoningEffort:  new(modelcatalog.ReasoningEffortHigh),
				Fast:             new(true),
			}},
			Sources: []modelcatalog.SourceRef{{
				SourceID:   modelcatalog.SourceKindProviderLiveID("cursor"),
				SourceKind: modelcatalog.SourceKindProviderLive,
			}},
		}}}))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:       "coder",
			Provider:        "cursor",
			Model:           cursorModel,
			ReasoningEffort: "high",
			Speed:           speedpkg.SpeedFast,
			Workspace:       h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})
		if got := h.driver.startCalls[0].PreferredModel; got != cursorModel {
			t.Fatalf("StartOpts.PreferredModel = %q, want %q", got, cursorModel)
		}
		if got := h.driver.startCalls[0].RuntimeStrategy; got != acp.RuntimeApplicationLaunchArg {
			t.Fatalf("StartOpts.RuntimeStrategy = %q, want launch_arg", got)
		}
		if got := h.driver.startCalls[0].LaunchModelID; got != transportModel {
			t.Fatalf("StartOpts.LaunchModelID = %q, want %q", got, transportModel)
		}
		argv, splitErr := shellquote.Split(h.driver.startCalls[0].Command)
		if splitErr != nil {
			t.Fatalf("shellquote.Split(StartOpts.Command) error = %v", splitErr)
		}
		wantArgv := []string{
			"cursor-agent",
			"--model",
			transportModel,
			"acp",
		}
		if !slices.Equal(argv, wantArgv) {
			t.Fatalf("StartOpts.Command argv = %#v, want %#v", argv, wantArgv)
		}
		if got := readMeta(t, session.MetaPath()).Model; got != cursorModel {
			t.Fatalf("meta.Model = %q, want %q", got, cursorModel)
		}
	})

	t.Run("Should resolve a logical Claude model to its live transport alias at start", func(t *testing.T) {
		t.Parallel()

		const logicalModel = "claude-opus-5"
		const transportModel = "opus"
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
			ProviderID:        runtimeProviderClaude,
			ModelID:           logicalModel,
			AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
			TransportBindings: []modelcatalog.ModelTransportBinding{{TransportModelID: transportModel}},
			Sources: []modelcatalog.SourceRef{{
				SourceID:   modelcatalog.SourceKindProviderLiveID(runtimeProviderClaude),
				SourceKind: modelcatalog.SourceKindProviderLive,
			}},
		}}}))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Provider:  runtimeProviderClaude,
			Model:     logicalModel,
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})
		if got := h.driver.startCalls[0].PreferredModel; got != transportModel {
			t.Fatalf("StartOpts.PreferredModel = %q, want private alias %q", got, transportModel)
		}
		if got := session.Info().Model; got != logicalModel {
			t.Fatalf("session.Info().Model = %q, want logical model %q", got, logicalModel)
		}
		if got := readMeta(t, session.MetaPath()).Model; got != logicalModel {
			t.Fatalf("meta.Model = %q, want logical model %q", got, logicalModel)
		}
	})

	t.Run("Should reject an aliased Cursor agent default before session reservation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
			ProviderID:        "cursor",
			ModelID:           "grok-4.5",
			AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
			TransportBindings: []modelcatalog.ModelTransportBinding{{
				TransportModelID: "cursor-grok-4.5-high",
				ReasoningEffort:  new(modelcatalog.ReasoningEffortHigh),
				Fast:             new(false),
			}},
			Sources: []modelcatalog.SourceRef{{
				SourceID:   modelcatalog.SourceKindProviderLiveID("cursor"),
				SourceKind: modelcatalog.SourceKindProviderLive,
			}},
		}}}))
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[index].Name == "coder" {
				resolvedWorkspace.Agents[index].Provider = "cursor"
				resolvedWorkspace.Agents[index].Model = "cursor-grok-4.5-high"
			}
		}
		h.resolver.upsert(&resolvedWorkspace)

		_, err = h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		})
		if err == nil || !strings.Contains(err.Error(), "not advertised by the live ACP catalog") {
			t.Fatalf("Create() error = %v, want explicit Cursor catalog-membership rejection", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want no ACP launch", got)
		}
		if _, ok := h.manager.Get("sess-1"); ok {
			t.Fatal("Create() registered an aliased Cursor session, want no active session")
		}
	})

	t.Run("Should start an advertised Cursor agent default exactly", func(t *testing.T) {
		t.Parallel()

		const cursorModel = "grok-4.5"
		const transportModel = "cursor-grok-4.5-high"
		var requestedView modelcatalog.CatalogView
		h := newHarness(t, WithModelCatalog(modelCatalogStub{
			onList: func(opts modelcatalog.ListOptions) {
				requestedView = opts.View
			},
			models: []modelcatalog.Model{{
				ProviderID:             "cursor",
				ModelID:                cursorModel,
				AvailabilityState:      modelcatalog.AvailabilityStateAvailableLive,
				DefaultReasoningEffort: new(modelcatalog.ReasoningEffortHigh),
				TransportBindings: []modelcatalog.ModelTransportBinding{{
					TransportModelID: transportModel,
					ReasoningEffort:  new(modelcatalog.ReasoningEffortHigh),
					Fast:             new(false),
				}},
				Sources: []modelcatalog.SourceRef{{
					SourceID:   modelcatalog.SourceKindProviderLiveID("cursor"),
					SourceKind: modelcatalog.SourceKindProviderLive,
				}},
			}}}))
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[index].Name == "coder" {
				resolvedWorkspace.Agents[index].Provider = "cursor"
				resolvedWorkspace.Agents[index].Model = cursorModel
			}
		}
		h.resolver.upsert(&resolvedWorkspace)

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})
		if got := h.driver.startCalls[0].PreferredModel; got != cursorModel {
			t.Fatalf("StartOpts.PreferredModel = %q, want %q", got, cursorModel)
		}
		if got := h.driver.startCalls[0].LaunchModelID; got != transportModel {
			t.Fatalf("StartOpts.LaunchModelID = %q, want %q", got, transportModel)
		}
		if got := readMeta(t, session.MetaPath()).Model; got != cursorModel {
			t.Fatalf("meta.Model = %q, want %q", got, cursorModel)
		}
		if requestedView != modelcatalog.CatalogViewAll {
			t.Fatalf("model catalog view = %q, want all live discovered models", requestedView)
		}
	})

	t.Run("Should preserve native default startup and persist the provider current model", func(t *testing.T) {
		t.Parallel()

		const cursorModel = "grok-4.5[effort=high,fast=true]"
		h := newHarness(t)
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[index].Name == "coder" {
				resolvedWorkspace.Agents[index].Provider = "cursor"
				resolvedWorkspace.Agents[index].Model = ""
			}
		}
		h.resolver.upsert(&resolvedWorkspace)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if opts.PreferredModel != "" {
				t.Fatalf("StartOpts.PreferredModel = %q, want provider-native default", opts.PreferredModel)
			}
			proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-cursor-native")
			proc.handle.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
				ID:             "model",
				Kind:           acp.SessionConfigOptionKindSelect,
				CurrentValueID: cursorModel,
				Values:         []acp.SessionConfigOptionValue{{Value: cursorModel}},
			}}})
			return proc, nil
		}

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})
		if got := session.Info().Model; got != "" {
			t.Fatalf("session.Info().Model = %q, want no private Cursor alias", got)
		}
		if got := readMeta(t, session.MetaPath()).Model; got != "" {
			t.Fatalf("meta.Model = %q, want no private Cursor alias", got)
		}
	})

	t.Run("Should resolve the session with the explicit model override", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Provider:  "codex",
			Model:     "task-profile-model",
			Name:      "profiled-worker",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
		})

		if got, want := session.Info().Model, "task-profile-model"; got != want {
			t.Fatalf("session.Info().Model = %q, want %q", got, want)
		}
		if meta := readMeta(t, session.MetaPath()); meta.Model != "task-profile-model" {
			t.Fatalf("meta.Model = %q, want task-profile-model", meta.Model)
		}
	})

	t.Run("Should reject model override without provider override", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Model:     "task-profile-model",
			Workspace: h.workspaceID,
		})
		if !errors.Is(err, ErrInvalidRuntimeOverride) {
			t.Fatalf("Create() error = %v, want ErrInvalidRuntimeOverride", err)
		}
	})

	t.Run("Should persist supported reasoning effort override", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:       "coder",
			Provider:        "codex",
			ReasoningEffort: "high",
			Name:            "reasoned-worker",
			Workspace:       h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
		})

		if got := session.Info().ReasoningEffort; got != "high" {
			t.Fatalf("session.Info().ReasoningEffort = %q, want high", got)
		}
		if meta := readMeta(t, session.MetaPath()); meta.ReasoningEffort != "high" {
			t.Fatalf("meta.ReasoningEffort = %q, want high", meta.ReasoningEffort)
		}
	})

	t.Run("Should persist reasoning effort without provider-level support flag", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if got := opts.ReasoningEffort; got != "high" {
				t.Fatalf("StartOpts.ReasoningEffort = %q, want high", got)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-reasoning"), nil
		}
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:       "coder",
			Provider:        "claude",
			ReasoningEffort: "high",
			Name:            "reasoned-claude-worker",
			Workspace:       h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
		})

		if got := session.Info().ReasoningEffort; got != "high" {
			t.Fatalf("session.Info().ReasoningEffort = %q, want high", got)
		}
		if meta := readMeta(t, session.MetaPath()); meta.ReasoningEffort != "high" {
			t.Fatalf("meta.ReasoningEffort = %q, want high", meta.ReasoningEffort)
		}
	})

	t.Run("Should resolve the agent reasoning default into StartOpts and persisted session state", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for idx := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[idx].Name == "coder" {
				resolvedWorkspace.Agents[idx].ReasoningEffort = "max"
			}
		}
		h.resolver.upsert(&resolvedWorkspace)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if got, want := opts.ReasoningEffort, "max"; got != want {
				t.Fatalf("StartOpts.ReasoningEffort = %q, want %q", got, want)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-agent-reasoning"), nil
		}

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "agent-reasoning-default",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		if got, want := session.Info().ReasoningEffort, "max"; got != want {
			t.Fatalf("session.Info().ReasoningEffort = %q, want %q", got, want)
		}
		if got, want := readMeta(t, session.MetaPath()).ReasoningEffort, "max"; got != want {
			t.Fatalf("meta.ReasoningEffort = %q, want %q", got, want)
		}
	})

	t.Run("Should default speed to normal and persist the negotiated outcome", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if got, want := opts.Speed, speedpkg.SpeedNormal; got != want {
				t.Fatalf("StartOpts.Speed = %q, want %q", got, want)
			}
			proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-speed-normal")
			proc.handle.setCaps(acp.Caps{SpeedResolution: &speedpkg.Resolution{
				Requested: speedpkg.SpeedNormal,
				Status:    speedpkg.ResolutionApplied,
			}})
			return proc, nil
		}

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		stopped := false
		t.Cleanup(func() {
			if !stopped {
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			}
		})

		info := session.Info()
		if got, want := info.Speed, speedpkg.SpeedNormal; got != want {
			t.Fatalf("session.Info().Speed = %q, want %q", got, want)
		}
		if info.SpeedResolution == nil || info.SpeedResolution.Status != speedpkg.ResolutionApplied {
			t.Fatalf("session.Info().SpeedResolution = %#v, want applied", info.SpeedResolution)
		}
		meta := readMeta(t, session.MetaPath())
		if got, want := meta.Speed, speedpkg.SpeedNormal; got != want {
			t.Fatalf("meta.Speed = %q, want %q", got, want)
		}
		if meta.SpeedResolution == nil || meta.SpeedResolution.Status != speedpkg.ResolutionApplied {
			t.Fatalf("meta.SpeedResolution = %#v, want applied", meta.SpeedResolution)
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped = true
		hydrated, err := h.manager.Status(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Status(stopped) error = %v", err)
		}
		if hydrated.Speed != speedpkg.SpeedNormal ||
			hydrated.SpeedResolution == nil ||
			hydrated.SpeedResolution.Status != speedpkg.ResolutionApplied {
			t.Fatalf("Status(stopped) speed fields = %q, %#v", hydrated.Speed, hydrated.SpeedResolution)
		}
	})

	t.Run("Should pass an explicit fast speed to the driver", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if got, want := opts.Speed, speedpkg.SpeedFast; got != want {
				t.Fatalf("StartOpts.Speed = %q, want %q", got, want)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-speed-fast"), nil
		}

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Speed:     speedpkg.SpeedFast,
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		if got, want := readMeta(t, session.MetaPath()).Speed, speedpkg.SpeedFast; got != want {
			t.Fatalf("meta.Speed = %q, want %q", got, want)
		}
	})

	t.Run("Should reject an invalid speed before driver startup", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Speed:     speedpkg.Speed("turbo"),
			Workspace: h.workspaceID,
		})
		if !errors.Is(err, ErrInvalidRuntimeOverride) {
			t.Fatalf("Create() error = %v, want ErrInvalidRuntimeOverride", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want 0", got)
		}
	})
}

type modelCatalogStub struct {
	models []modelcatalog.Model
	onList func(modelcatalog.ListOptions)
}

func (s modelCatalogStub) ListModels(
	_ context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	if s.onList != nil {
		s.onList(opts)
	}
	models := make([]modelcatalog.Model, 0, len(s.models))
	for _, model := range s.models {
		if opts.ProviderID == "" || model.ProviderID == opts.ProviderID {
			models = append(models, model)
		}
	}
	return models, nil
}

func (modelCatalogStub) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (modelCatalogStub) ListSourceStatus(
	context.Context,
	modelcatalog.StatusOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func TestCreateNotifiesSessionCreationBeforeImmediateExit(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-%d", sequence))
		proc.exit()
		return proc, nil
	}

	session := createSession(t, h)
	h.notifier.waitForStopped(t, session.ID)

	got := h.notifier.notificationOrder()
	want := []string{"created:" + session.ID, "stopped:" + session.ID}
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("notification order = %#v, want %#v", got, want)
	}

	meta := readMeta(t, session.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopCompleted {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopCompleted)
	}
}

func TestCreateWithWorkspacePathUsesResolveOrRegister(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve or register a default-profile workspace path through its profile layer", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		workspacePath := filepath.Join(t.TempDir(), "path-workspace")
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			t.Fatalf("MkdirAll(path workspace) error = %v", err)
		}

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:     "coder",
			Name:          "path-session",
			WorkspacePath: workspacePath,
		})
		if err != nil {
			t.Fatalf("Create(workspace path) error = %v", err)
		}
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		if got := len(h.resolver.resolveCalls); got != 1 {
			t.Fatalf("resolver Resolve() calls = %d, want 1", got)
		}
		if got, want := h.resolver.resolveCalls[0], session.Info().WorkspaceID; got != want {
			t.Fatalf("resolver Resolve() arg = %q, want %q", got, want)
		}
		if got := len(h.resolver.resolveOrRegisterCalls); got != 1 {
			t.Fatalf("resolver ResolveOrRegisterForProfile() calls = %d, want 1", got)
		}
		if got, want := h.resolver.resolveOrRegisterCalls[0], normalizeResolverPath(workspacePath); got != want {
			t.Fatalf("resolver ResolveOrRegister() arg = %q, want %q", got, want)
		}
		if got := h.resolver.profileRegisterNames; !slices.Equal(got, []string{"default"}) {
			t.Fatalf("resolver profile registrations = %#v, want default", got)
		}
		if got, want := session.Info().Workspace, normalizeResolverPath(workspacePath); got != want {
			t.Fatalf("session workspace = %q, want %q", got, want)
		}
		if !strings.HasPrefix(session.Info().WorkspaceID, "ws-auto-") {
			t.Fatalf("session workspace id = %q, want ws-auto-*", session.Info().WorkspaceID)
		}
		if meta := readMeta(t, session.MetaPath()); meta.WorkspaceID != session.Info().WorkspaceID {
			t.Fatalf("meta workspace id = %q, want %q", meta.WorkspaceID, session.Info().WorkspaceID)
		}
	})

	t.Run("Should resolve or register a profile workspace path without a second lookup", func(t *testing.T) {
		t.Parallel()

		const profileID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		h := newHarness(t, WithProfileNameResolver(profileNameResolverMap{
			profileID: "marketing",
		}))
		workspacePath := filepath.Join(t.TempDir(), "profile-path-workspace")
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			t.Fatalf("MkdirAll(profile path workspace) error = %v", err)
		}

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			ProfileID:     profileID,
			AgentName:     "coder",
			Name:          "profile-path-session",
			WorkspacePath: workspacePath,
		})
		if err != nil {
			t.Fatalf("Create(profile workspace path) error = %v", err)
		}
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		if got := len(h.resolver.resolveCalls); got != 1 {
			t.Fatalf("resolver Resolve() calls = %d, want only the lifecycle lookup", got)
		}
		if got := len(h.resolver.resolveOrRegisterCalls); got != 1 {
			t.Fatalf("resolver ResolveOrRegisterForProfile() calls = %d, want 1", got)
		}
		if got := len(h.resolver.profileRegisterNames); got != 1 {
			t.Fatalf("resolver profile registrations = %d, want 1", got)
		}
		if got, want := h.resolver.profileRegisterNames[0], "marketing"; got != want {
			t.Fatalf("resolved profile name = %q, want %q", got, want)
		}
	})
}

func TestCreateUsesProfileWorkspaceAgents(t *testing.T) {
	t.Parallel()

	t.Run("Should create a session with an agent from the default profile layer", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.resolver.mu.Lock()
		profileWorkspace := h.resolver.byRef[h.workspaceID]
		profileWorkspace.Agents = append(profileWorkspace.Agents, compozyconfig.AgentDef{
			Name:     "marketing",
			Provider: "claude",
			Prompt:   "You are a marketing assistant.",
		})
		h.resolver.byProfile[sessionDefaultProfileName] = profileWorkspace
		h.resolver.mu.Unlock()

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "marketing",
			Name:      "marketing-session",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create(default profile agent) error = %v", err)
		}
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		h.resolver.mu.Lock()
		profileNames := append([]string(nil), h.resolver.profileResolveNames...)
		h.resolver.mu.Unlock()
		if len(profileNames) == 0 || profileNames[0] != sessionDefaultProfileName {
			t.Fatalf(
				"ResolveForProfile() names = %v, want first lookup for %q",
				profileNames,
				sessionDefaultProfileName,
			)
		}
		if got := session.Info().AgentName; got != "marketing" {
			t.Fatalf("session agent = %q, want marketing", got)
		}
	})
}

func TestJoinNetworkPeerHandlesNoOpConditionsAndCapabilityProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should no-op for nil session blank channel and missing lifecycle", func(t *testing.T) {
		t.Parallel()

		capabilities := []NetworkPeerCapability{{
			ID:      "review-pr",
			Summary: "Review pull requests",
		}}

		tests := []struct {
			name             string
			session          *Session
			installLifecycle bool
		}{
			{
				name:             "Should no-op when session is nil",
				session:          nil,
				installLifecycle: true,
			},
			{
				name: "Should no-op when channel is blank",
				session: &Session{
					ID:                   "sess-no-channel",
					AgentName:            "Coder",
					NetworkParticipation: participation.LocalSpec(),
				},
				installLifecycle: true,
			},
			{
				name: "Should no-op when lifecycle is missing",
				session: &Session{
					ID:                   "sess-no-lifecycle",
					AgentName:            "Coder",
					NetworkParticipation: testLiveParticipation("ws-test", "builders"),
				},
				installLifecycle: false,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				h := newHarness(t)
				lifecycle := newFakeNetworkPeerLifecycle()
				if tc.installLifecycle {
					h.manager.SetNetworkPeerLifecycle(lifecycle)
				}

				if err := h.manager.joinNetworkPeer(testutil.Context(t), tc.session, capabilities); err != nil {
					t.Fatalf("joinNetworkPeer() error = %v", err)
				}
				if got := lifecycle.joinCount(); got != 0 {
					t.Fatalf("joinNetworkPeer() join count = %d, want 0", got)
				}
			})
		}
	})

	t.Run("Should forward identity channel and capability-aware payload", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		lifecycle := newFakeNetworkPeerLifecycle()
		h.manager.SetNetworkPeerLifecycle(lifecycle)

		session := &Session{
			ID:                   "sess-capabilities",
			AgentName:            "Coder",
			NetworkParticipation: testLiveParticipation("ws-test", "builders"),
		}
		capabilities := []NetworkPeerCapability{{
			ID:      "review-pr",
			Summary: "Review pull requests",
		}}

		if err := h.manager.joinNetworkPeer(testutil.Context(t), session, capabilities); err != nil {
			t.Fatalf("joinNetworkPeer() error = %v", err)
		}

		if got := lifecycle.joinCount(); got != 1 {
			t.Fatalf("joinNetworkPeer() join count = %d, want 1", got)
		}
		call := lifecycle.joinCall(0)
		if got, want := call.sessionID, "sess-capabilities"; got != want {
			t.Fatalf("join session_id = %q, want %q", got, want)
		}
		if got, want := call.peerID, "coder.sess-capabilities"; got != want {
			t.Fatalf("join peer_id = %q, want %q", got, want)
		}
		if got, want := call.channel, "builders"; got != want {
			t.Fatalf("join channel = %q, want %q", got, want)
		}
		if !reflect.DeepEqual(call.capabilities, capabilities) {
			t.Fatalf("join capabilities = %#v, want %#v", call.capabilities, capabilities)
		}
	})

	t.Run("Should rebind an active peer and restore its durable participation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		lifecycle := newFakeNetworkPeerLifecycle()
		h.manager.SetNetworkPeerLifecycle(lifecycle)
		active := createLiveNetworkSession(t, h)
		taskSpec := testLiveParticipation(h.workspaceID, "lifecycle-cadence")

		if err := h.manager.BindNetworkPeer(
			testutil.Context(t),
			active.ID,
			taskSpec,
			"task_run:run-lifecycle",
		); err != nil {
			t.Fatalf("BindNetworkPeer() error = %v", err)
		}
		if got, want := lifecycle.joinCall(1).channel, "lifecycle-cadence"; got != want {
			t.Fatalf("bound peer channel = %q, want %q", got, want)
		}
		if got, want := active.Info().NetworkParticipation.ChannelID, "builders"; got != want {
			t.Fatalf("durable session channel = %q, want %q", got, want)
		}

		if err := h.manager.RestoreNetworkPeer(testutil.Context(t), active.ID); err != nil {
			t.Fatalf("RestoreNetworkPeer() error = %v", err)
		}
		if got, want := lifecycle.joinCall(2).channel, "builders"; got != want {
			t.Fatalf("restored peer channel = %q, want %q", got, want)
		}
	})
}

func TestListAndGet(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	first := createSession(t, h)
	second := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, first.ID)
		reportSessionStop(t, h, second.ID)
	})

	list := h.manager.List()
	if len(list) != 2 {
		t.Fatalf("List() = %d sessions, want 2", len(list))
	}
	if list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("List() ids = [%s %s], want [%s %s]", list[0].ID, list[1].ID, first.ID, second.ID)
	}
	if _, ok := h.manager.Get("missing"); ok {
		t.Fatal("Get(missing) = found, want missing")
	}
}

func TestCreateDoesNotEnforceSessionCap(t *testing.T) {
	t.Parallel()

	t.Run("Should allow more sessions than the configured cap", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		const total = 12
		for range total {
			session := createSession(t, h)
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
					!errors.Is(err, ErrSessionNotFound) {
					t.Errorf("Stop(%q) error = %v", session.ID, err)
				}
			})
		}
		if list := h.manager.List(); len(list) != total {
			t.Fatalf("List() = %d sessions, want %d", len(list), total)
		}
	})
}

func TestCreatePassesMergedMCPServers(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	skillRegistry := newFakeSkillRegistry()
	h.cfg.MCPServers = []compozyconfig.MCPServer{
		{Name: "global", Command: "global-command"},
	}
	h.cfg.Providers["claude"] = compozyconfig.ProviderConfig{
		Command: "provider-command",
		MCPServers: []compozyconfig.MCPServer{
			{Name: "base", Command: "base-command", Args: []string{"--base"}},
			{Name: "override", Command: "provider-override"},
		},
	}
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are helpful.",
			MCPServers: []compozyconfig.MCPServer{
				{Name: "override", Command: "agent-override", Args: []string{"--agent"}},
				{Name: "extra", Command: "extra-command"},
			},
		}},
	})
	skillRegistry.setSkills(h.workspaceID, []*skillspkg.Skill{
		{
			Enabled: true,
			Source:  skillspkg.SourceUser,
			Meta:    skillspkg.SkillMeta{Name: "skill-mcp"},
			MCPServers: []skillspkg.MCPServerDecl{
				{Name: "override", Command: "skill-override", Args: []string{"--skill"}},
				{Name: "skill-extra", Command: "skill-extra-command"},
			},
		},
	})
	h.manager = newManagerWithHarness(
		t,
		h,
		WithSkillRegistry(skillRegistry),
		WithMCPResolver(skillspkg.NewMCPResolver(compozyconfig.SkillsConfig{}, nil)),
		WithLogger(slog.New(logs)),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	got := h.driver.startCalls[0].MCPServers
	if len(got) != 5 {
		t.Fatalf("start MCPServers = %#v, want 5 entries", got)
	}
	if got[0].Name != "global" || got[0].Command != "global-command" {
		t.Fatalf("global MCP server = %#v", got[0])
	}
	if got[1].Name != "base" || got[1].Command != "base-command" {
		t.Fatalf("base MCP server = %#v", got[1])
	}
	if got[2].Name != "override" || got[2].Command != "skill-override" {
		t.Fatalf("override MCP server = %#v", got[2])
	}
	if got[3].Name != "extra" || got[3].Command != "extra-command" {
		t.Fatalf("extra MCP server = %#v", got[3])
	}
	if got[4].Name != "skill-extra" || got[4].Command != "skill-extra-command" {
		t.Fatalf("skill-extra MCP server = %#v", got[4])
	}
	if got := skillRegistry.callCount(); got != 1 {
		t.Fatalf("skill registry call count = %d, want 1", got)
	}
	if got := skillRegistry.call(0).ID; got != h.workspaceID {
		t.Fatalf("skill registry workspace id = %q, want %q", got, h.workspaceID)
	}
	record, ok := logs.FindByMessage("session.mcp.hosted_mcp_unavailable")
	if !ok {
		t.Fatalf("logs = %#v, want hosted MCP unavailable diagnostic", logs.Records())
	}
	if record.Level != slog.LevelWarn {
		t.Fatalf("hosted MCP unavailable log level = %s, want WARN", record.Level)
	}
	if got, want := record.Attrs["reason"], "hosted_mcp_launcher_unavailable"; got != want {
		t.Fatalf("hosted MCP unavailable reason = %q, want %q", got, want)
	}
	if got, want := record.Attrs["configured_mcp_servers"], "4"; got != want {
		t.Fatalf("hosted MCP unavailable configured_mcp_servers = %q, want %q", got, want)
	}
}

func TestCreateInjectsOnlyHostedMCPServerWhenLauncherConfigured(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.cfg.MCPServers = []compozyconfig.MCPServer{
		{
			Name:      "remote-http",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       "https://example.test/mcp",
		},
		{
			Name:    "legacy-stdio",
			Command: "legacy-command",
		},
	}
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are helpful.",
			MCPServers: []compozyconfig.MCPServer{
				{Name: "agent-stdio", Command: "agent-command"},
			},
		}},
	})
	hosted := &recordingHostedMCPLauncher{
		server: compozyconfig.MCPServer{
			Name:      "compozy-hosted-tools",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   "/bin/compozy",
			Args:      []string{"tool", "mcp", "--session", "sess-1", "--bind-nonce", "nonce"},
		},
	}
	h.manager = newManagerWithHarness(t, h, WithHostedMCPLauncher(hosted))

	session := createSession(t, h)
	if got, want := len(h.driver.startCalls), 1; got != want {
		t.Fatalf("start calls = %d, want %d", got, want)
	}
	got := h.driver.startCalls[0].MCPServers
	if len(got) != 1 {
		t.Fatalf("start MCPServers = %#v, want one hosted entry", got)
	}
	if got[0].Name != "compozy-hosted-tools" || got[0].Command != "/bin/compozy" {
		t.Fatalf("hosted MCP server = %#v, want Compozy hosted stdio entry", got[0])
	}

	requests := hosted.launchRequests()
	if len(requests) != 1 {
		t.Fatalf("hosted launch requests = %#v, want one launch request", requests)
	}
	if requests[0].SessionID != session.ID || requests[0].ProfileID != store.DefaultProfileID ||
		requests[0].WorkspaceID != h.workspaceID ||
		requests[0].AgentName != "coder" {
		t.Fatalf("hosted launch request = %#v, want profile/session/workspace/agent scope", requests[0])
	}
	if armed := hosted.armedSessionIDs(); !slices.Equal(armed, []string{session.ID}) {
		t.Fatalf("hosted armed sessions = %#v, want [%q]", armed, session.ID)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if releases := hosted.releaseSessionIDs(); !slices.Contains(releases, session.ID) {
		t.Fatalf("hosted releases = %#v, want released session %q", releases, session.ID)
	}
}

func TestCreateRequiresHostedMCPForRestrictedTools(t *testing.T) {
	t.Parallel()

	t.Run("Should fail before provider start when hosted launcher is unavailable", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:            "coder",
			Name:                 "restricted",
			Workspace:            h.workspaceID,
			AllowedToolsOverride: []string{toolspkg.ToolIDTaskRunClaimNext.String()},
		})
		if !errors.Is(err, ErrHostedMCPUnavailable) {
			t.Fatalf("Create() error = %v, want %v", err, ErrHostedMCPUnavailable)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("provider start calls = %d, want 0", got)
		}
	})

	t.Run("Should fail before provider start when provider disables session MCP", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      h.workspaceID,
				RootDir: h.workspace,
				Name:    h.workspaceName,
			},
			Config: h.cfg,
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "openclaw",
				Prompt:   "You are helpful.",
			}},
		})
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:            "coder",
			Name:                 "restricted",
			Workspace:            h.workspaceID,
			AllowedToolsOverride: []string{toolspkg.ToolIDTaskRunClaimNext.String()},
		})
		if !errors.Is(err, ErrHostedMCPUnavailable) {
			t.Fatalf("Create() error = %v, want %v", err, ErrHostedMCPUnavailable)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("provider start calls = %d, want 0", got)
		}
	})

	t.Run("Should fail before provider start when a verdict-only session requires tools", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:            "coder",
			Name:                 "restricted-verdict",
			Workspace:            h.workspaceID,
			RuntimeMode:          RuntimeModeVerdictOnly,
			AllowedToolsOverride: []string{toolspkg.ToolIDTaskRunClaimNext.String()},
		})
		if !errors.Is(err, ErrHostedMCPUnavailable) {
			t.Fatalf("Create() error = %v, want %v", err, ErrHostedMCPUnavailable)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("provider start calls = %d, want 0", got)
		}
	})
}

func TestCreateOmitsMCPServersForVerdictOnlyRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should withhold hosted MCP capabilities from verdict-only sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		hosted := &recordingHostedMCPLauncher{
			server: compozyconfig.MCPServer{
				Name:      "compozy-hosted-tools",
				Transport: compozyconfig.MCPServerTransportStdio,
				Command:   "/bin/compozy",
			},
		}
		h.manager = newManagerWithHarness(t, h, WithHostedMCPLauncher(hosted))

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:   "coder",
			Name:        "verdict-only",
			Workspace:   h.workspaceID,
			RuntimeMode: RuntimeModeVerdictOnly,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		if got := h.driver.startCalls[0].MCPServers; len(got) != 0 {
			t.Fatalf("start MCPServers = %#v, want none for verdict-only runtime", got)
		}
		if got := hosted.launchRequests(); len(got) != 0 {
			t.Fatalf("hosted launch requests = %#v, want none for verdict-only runtime", got)
		}
	})
}

func TestCreateSkipsHostedMCPWhenProviderDisablesSessionMCP(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: "openclaw",
			Prompt:   "You are helpful.",
		}},
	})
	hosted := &recordingHostedMCPLauncher{
		server: compozyconfig.MCPServer{
			Name:      "compozy-hosted-tools",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   "/bin/compozy",
		},
	}
	h.manager = newManagerWithHarness(t, h, WithHostedMCPLauncher(hosted), WithLogger(slog.New(logs)))

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	got := h.driver.startCalls[0]
	if got.Command != "openclaw acp" {
		t.Fatalf("start command = %q, want openclaw acp", got.Command)
	}
	if len(got.MCPServers) != 0 {
		t.Fatalf("start MCPServers = %#v, want none for provider without session MCP support", got.MCPServers)
	}
	if requests := hosted.launchRequests(); len(requests) != 0 {
		t.Fatalf("hosted launch requests = %#v, want none", requests)
	}
	record, ok := logs.FindByMessage("session.mcp.skipped")
	if !ok {
		t.Fatalf("logs = %#v, want session MCP skipped diagnostic", logs.Records())
	}
	if record.Level != slog.LevelInfo {
		t.Fatalf("session MCP skipped log level = %s, want INFO", record.Level)
	}
	if got, want := record.Attrs["reason"], "provider_session_mcp_disabled"; got != want {
		t.Fatalf("session MCP skipped reason = %q, want %q", got, want)
	}
	if got, want := record.Attrs["resolved_provider"], "openclaw"; got != want {
		t.Fatalf("session MCP skipped provider = %q, want %q", got, want)
	}
}

func TestCreateBlocksMarketplaceSkillMCPServersWithoutConsent(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	skillRegistry := newFakeSkillRegistry()
	skillRegistry.setSkills(h.workspaceID, []*skillspkg.Skill{
		{
			Enabled: true,
			Source:  skillspkg.SourceMarketplace,
			Meta:    skillspkg.SkillMeta{Name: "market-skill"},
			MCPServers: []skillspkg.MCPServerDecl{
				{Name: "market-extra", Command: "market-extra-command"},
			},
		},
	})
	h.manager = newManagerWithHarness(
		t,
		h,
		WithSkillRegistry(skillRegistry),
		WithMCPResolver(skillspkg.NewMCPResolver(compozyconfig.SkillsConfig{}, nil)),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := h.driver.startCalls[0].MCPServers; len(got) != 0 {
		t.Fatalf("start MCPServers = %#v, want marketplace skill MCP blocked", got)
	}
}

func TestCreateWithoutChannelDoesNotAppendBundledNetworkSkill(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptAssembler(nil))
	networkSkill, err := skillbundled.LoadResource(testBundledCompozySkillName, testBundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", testBundledCompozySkillName, testBundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant." {
		t.Fatalf("start system prompt = %q, want raw agent prompt", got)
	}
	if strings.Contains(h.driver.startCalls[0].SystemPrompt, networkSkill) {
		t.Fatalf("start system prompt unexpectedly contains bundled network skill")
	}
}

func TestCreateWithChannelInjectsNetworkSessionEnv(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptAssembler(nil))

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	env := h.driver.startCalls[0].Env
	if got, ok := lookupEnvValue(env, "COMPOZY_SESSION_ID"); !ok || got != session.ID {
		t.Fatalf("COMPOZY_SESSION_ID = %q, %v, want %q", got, ok, session.ID)
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_AGENT"); !ok || got != "coder" {
		t.Fatalf("COMPOZY_AGENT = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_AGENT_NAME"); !ok || got != "coder" {
		t.Fatalf("COMPOZY_AGENT_NAME = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_SESSION_CHANNEL"); !ok || got != "builders" {
		t.Fatalf("COMPOZY_SESSION_CHANNEL = %q, %v, want %q", got, ok, "builders")
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_PEER_ID"); !ok || got != "coder."+session.ID {
		t.Fatalf("COMPOZY_PEER_ID = %q, %v, want %q", got, ok, "coder."+session.ID)
	}
}

func TestCreateWithoutChannelOmitsNetworkChannelEnv(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptAssembler(nil))

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	env := h.driver.startCalls[0].Env
	if got, ok := lookupEnvValue(env, "COMPOZY_SESSION_ID"); !ok || got != session.ID {
		t.Fatalf("COMPOZY_SESSION_ID = %q, %v, want %q", got, ok, session.ID)
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_AGENT"); !ok || got != "coder" {
		t.Fatalf("COMPOZY_AGENT = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_AGENT_NAME"); !ok || got != "coder" {
		t.Fatalf("COMPOZY_AGENT_NAME = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_SESSION_CHANNEL"); ok {
		t.Fatalf("COMPOZY_SESSION_CHANNEL = %q, want unset", got)
	}
	if got, ok := lookupEnvValue(env, "COMPOZY_PEER_ID"); ok {
		t.Fatalf("COMPOZY_PEER_ID = %q, want unset", got)
	}
}

func TestACPDriverAdapterErrorPaths(t *testing.T) {
	t.Parallel()

	adapter := NewACPDriverAdapter(acp.New())
	if _, err := adapter.Prompt(testutil.Context(t), &AgentProcess{}, acp.PromptRequest{}); err == nil {
		t.Fatal("Prompt(unsupported process) error = nil, want non-nil")
	}
	if err := adapter.Stop(testutil.Context(t), &AgentProcess{}); err == nil {
		t.Fatal("Stop(unsupported process) error = nil, want non-nil")
	}
}

func TestManagerOptionsPreserveExplicitQueryStore(t *testing.T) {
	t.Parallel()

	t.Run("Should start and join the default query store runtime", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown(cleanup) error = %v", err)
			}
		})
		if manager.queryStoreRuntime == nil || manager.queryStoreRuntime.done == nil {
			t.Fatal("default query store runtime = not started")
		}
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		select {
		case <-manager.queryStoreRuntime.done:
		default:
			t.Fatal("default query store runtime = not joined")
		}
	})

	t.Run("Should keep explicit query store regardless of WithStore order", func(t *testing.T) {
		t.Parallel()

		called := ""
		queryOpener := func(context.Context, store.SessionDBOwner, string) (EventReadCloser, error) {
			called = "query"
			return &stubRecorder{}, nil
		}
		storeOpener := func(context.Context, store.SessionDBOwner, string) (EventRecorder, error) {
			called = "store"
			return &stubRecorder{}, nil
		}
		h := newHarness(t, WithQueryStore(queryOpener), WithStore(storeOpener))

		recorder, err := h.manager.openQueryStore(
			testutil.Context(t),
			testSessionDBOwner("sess-query-order", h.workspaceID),
			filepath.Join(t.TempDir(), "events.db"),
		)
		if err != nil {
			t.Fatalf("openQueryStore() error = %v", err)
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("recorder.Close() error = %v", err)
		}
		if called != "query" {
			t.Fatalf("openQueryStore() called %q opener, want query", called)
		}
	})
}
