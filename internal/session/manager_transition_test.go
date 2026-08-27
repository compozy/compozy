package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/modelcatalog"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var errRecordingCatalogMissingSession = errors.New("recording catalog missing session")

func TestManagerLifecycleCatalogTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should pass prompt ACP options through live runtime configuration", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })

		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.ACPOptions = []acp.SessionConfigOptionSelection{{ID: "thinking", BoolValue: new(true)}}
		process, err := h.manager.ensurePromptRuntime(
			testutil.Context(t),
			active,
			&selection,
			snapshot.process,
		)
		if err != nil {
			t.Fatalf("ensurePromptRuntime() error = %v", err)
		}
		if process != snapshot.process {
			t.Fatal("ensurePromptRuntime() replaced a session-config process, want live reconfiguration")
		}
		configs := driver.configSnapshots()
		if len(configs) != 1 || len(configs[0].ACPOptions) != 1 ||
			configs[0].ACPOptions[0].ID != "thinking" || configs[0].ACPOptions[0].BoolValue == nil ||
			!*configs[0].ACPOptions[0].BoolValue {
			t.Fatalf("ConfigureRuntime() configs = %#v, want thinking=true", configs)
		}
		if got := active.Info().ACPOptions; len(got) != 1 || got[0].ID != "thinking" ||
			got[0].BoolValue == nil || !*got[0].BoolValue {
			t.Fatalf("active ACP options = %#v, want thinking=true", got)
		}
	})

	t.Run("Should replace Cursor process when a launch-bound ACP option changes", func(t *testing.T) {
		t.Parallel()

		const logicalModel = "opus-5"
		const regularTransport = "claude-opus-5-low"
		const thinkingTransport = "claude-opus-5-thinking-high"
		thinkingOff := false
		thinkingOn := true
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
			ProviderID:        cursorRuntimeProvider,
			ModelID:           logicalModel,
			AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
			ConfigOptions: []modelcatalog.ModelOptionDescriptor{{
				ID: "thinking", Kind: modelcatalog.ModelOptionKindBoolean, CurrentBool: &thinkingOff,
			}},
			TransportBindings: []modelcatalog.ModelTransportBinding{
				{
					TransportModelID: regularTransport,
					Thinking:         &thinkingOff,
					OptionSelections: []modelcatalog.ModelOptionSelection{{ID: "thinking", BoolValue: &thinkingOff}},
				},
				{
					TransportModelID: thinkingTransport,
					Thinking:         &thinkingOn,
					OptionSelections: []modelcatalog.ModelOptionSelection{{ID: "thinking", BoolValue: &thinkingOn}},
				},
			},
			Sources: []modelcatalog.SourceRef{
				{
					SourceID: modelcatalog.SourceKindProviderLiveID(
						cursorRuntimeProvider,
					),
					SourceKind: modelcatalog.SourceKindProviderLive,
				},
			},
		}}}))
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[index].Name == "coder" {
				resolvedWorkspace.Agents[index].Provider = cursorRuntimeProvider
				resolvedWorkspace.Agents[index].Model = logicalModel
			}
		}
		h.resolver.upsert(&resolvedWorkspace)

		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.ACPOptions = []acp.SessionConfigOptionSelection{{ID: "thinking", BoolValue: &thinkingOn}}
		replacement, err := h.manager.ensurePromptRuntime(
			testutil.Context(t),
			active,
			&selection,
			snapshot.process,
		)
		if err != nil {
			t.Fatalf("ensurePromptRuntime() error = %v", err)
		}
		if replacement == snapshot.process {
			t.Fatal("ensurePromptRuntime() reused process, want launch-bound replacement")
		}

		h.driver.mu.Lock()
		startCalls := append([]acp.StartOpts(nil), h.driver.startCalls...)
		stopCalls := h.driver.stopCalls
		h.driver.mu.Unlock()
		if len(startCalls) != 2 || startCalls[0].LaunchModelID != regularTransport ||
			startCalls[1].LaunchModelID != thinkingTransport {
			t.Fatalf("Cursor starts = %#v, want regular then thinking transport", startCalls)
		}
		if stopCalls != 1 {
			t.Fatalf("Cursor replacement stop calls = %d, want 1", stopCalls)
		}
		info := active.Info()
		if info.Model != logicalModel || len(info.ACPOptions) != 1 ||
			info.ACPOptions[0].BoolValue == nil || !*info.ACPOptions[0].BoolValue {
			t.Fatalf(
				"active runtime = model %q options %#v, want logical model with thinking=true",
				info.Model,
				info.ACPOptions,
			)
		}
	})

	t.Run("Should atomically resolve an orphaned permission and preserve the first winner", func(t *testing.T) {
		t.Parallel()

		_, restarted, catalog, sessionID := newOrphanResolutionHarness(
			t,
			store.PendingInteractionKindPermission,
			store.PendingInteractionPayload{Decisions: []string{"allow-once", "reject-once"}},
		)
		ctx := WithActingSession(testutil.Context(t), "sess-creator")
		result, err := restarted.ApprovePermission(ctx, sessionID, acp.ApproveRequest{
			RequestID: "request-orphan",
			Decision:  "allow-once",
		})
		if err != nil {
			t.Fatalf("ApprovePermission(orphan) error = %v", err)
		}
		if result.Outcome != store.PendingInteractionOutcomeResolvedAfterRestart ||
			result.ResolvedDecision != "allow-once" {
			t.Fatalf("ApprovePermission(orphan) = %#v", result)
		}
		assertOrphanResolutionQueue(t, catalog, sessionID, "Response: allow-once")
		resolved, err := catalog.ListPendingInteractions(
			ctx,
			sessionID,
			[]string{store.PendingInteractionStatusResolved},
		)
		if err != nil {
			t.Fatalf("ListPendingInteractions() error = %v", err)
		}
		if len(resolved) != 1 || resolved[0].ResolvedBy != "agent_session:sess-creator" {
			t.Fatalf("resolved interaction = %#v, want acting session", resolved)
		}

		duplicate, err := restarted.ApprovePermission(ctx, sessionID, acp.ApproveRequest{
			RequestID: "request-orphan",
			Decision:  "reject-once",
		})
		if err != nil {
			t.Fatalf("ApprovePermission(duplicate) error = %v", err)
		}
		if duplicate.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
			duplicate.ResolvedDecision != "allow-once" {
			t.Fatalf("ApprovePermission(duplicate) = %#v", duplicate)
		}
		assertOrphanResolutionQueue(t, catalog, sessionID, "Response: allow-once")
	})

	t.Run("Should report the durable winner when live permission resolution races", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		h := newHarness(t)
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown(initial manager) error = %v", err)
		}
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, h.manager)
		target := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, target.ID) })

		createdAt := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
		if _, err := catalog.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
			InteractionID:     "interaction-race",
			SessionID:         target.ID,
			Kind:              store.PendingInteractionKindPermission,
			ProviderRequestID: "request-race",
			TurnID:            "turn-race",
			Title:             "Allow the command?",
			Payload:           store.PendingInteractionPayload{Decisions: []string{"allow-once", "reject-once"}},
			CreatedAt:         createdAt,
		}); err != nil {
			t.Fatalf("CreatePendingInteraction() error = %v", err)
		}
		h.driver.approveHook = func(*fakeProcess, acp.ApproveRequest) error {
			if _, err := catalog.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
				InteractionID: "interaction-race",
				Status:        store.PendingInteractionStatusResolved,
				Resolution:    "reject-once",
				ResolvedBy:    "operator:competing-client",
				At:            createdAt.Add(time.Second),
			}); err != nil {
				return err
			}
			return acp.ErrPendingPermissionConflict
		}

		result, err := h.manager.ApprovePermission(ctx, target.ID, acp.ApproveRequest{
			RequestID: "request-race",
			Decision:  "allow-once",
		})
		if err != nil {
			t.Fatalf("ApprovePermission(race) error = %v", err)
		}
		if result.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
			result.ResolvedDecision != "reject-once" {
			t.Fatalf("ApprovePermission(race) = %#v, want durable winner", result)
		}
	})

	t.Run("Should atomically resolve an orphaned clarification with its validated choice", func(t *testing.T) {
		t.Parallel()

		h, restarted, catalog, sessionID := newOrphanResolutionHarness(
			t,
			store.PendingInteractionKindClarify,
			store.PendingInteractionPayload{Choices: []string{"Fast", "Safe"}},
		)
		ctx := WithActingSession(testutil.Context(t), "sess-creator")
		choice := 1
		result, err := restarted.ResolveOrphanedClarification(
			ctx,
			toolspkg.Scope{
				WorkspaceID: h.workspaceID,
				SessionID:   sessionID,
				AgentName:   "coder",
				Operator:    true,
			},
			"request-orphan",
			toolspkg.ClarifyAnswerRequest{ChoiceIndex: &choice},
		)
		if err != nil {
			t.Fatalf("ResolveOrphanedClarification() error = %v", err)
		}
		if result.Outcome != store.PendingInteractionOutcomeResolvedAfterRestart ||
			result.Choice == nil || *result.Choice != choice || result.ResolvedAnswer != "Safe" {
			t.Fatalf("ResolveOrphanedClarification() = %#v", result)
		}
		assertOrphanResolutionQueue(t, catalog, sessionID, "Response: Safe")
		resolved, err := catalog.ListPendingInteractions(
			ctx,
			sessionID,
			[]string{store.PendingInteractionStatusResolved},
		)
		if err != nil {
			t.Fatalf("ListPendingInteractions() error = %v", err)
		}
		if len(resolved) != 1 || resolved[0].ResolvedBy != "agent_session:sess-creator" {
			t.Fatalf("resolved interaction = %#v, want acting session", resolved)
		}

		other := 0
		duplicate, err := restarted.ResolveOrphanedClarification(
			ctx,
			toolspkg.Scope{
				WorkspaceID: h.workspaceID,
				SessionID:   sessionID,
				AgentName:   "coder",
				Operator:    true,
			},
			"request-orphan",
			toolspkg.ClarifyAnswerRequest{ChoiceIndex: &other},
		)
		if err != nil {
			t.Fatalf("ResolveOrphanedClarification(duplicate) error = %v", err)
		}
		if duplicate.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
			duplicate.Choice == nil || *duplicate.Choice != choice || duplicate.ResolvedAnswer != "Safe" {
			t.Fatalf("ResolveOrphanedClarification(duplicate) = %#v", duplicate)
		}
	})

	t.Run("Should hydrate durable attention badges in a fresh manager", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		h := newHarness(t)
		cleanupTestManager(t, h.manager)
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		baseAt := time.Date(2026, 8, 15, 19, 0, 0, 0, time.UTC)
		register := func(id string) {
			t.Helper()
			if err := catalog.RegisterSession(ctx, store.SessionInfo{
				ID: id, Name: id, AgentName: "coder", ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
				SessionType: string(SessionTypeUser), State: string(StateActive),
				RuntimeStatus: store.SessionRuntimeReady, CreatedAt: baseAt, UpdatedAt: baseAt,
			}); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", id, err)
			}
		}
		for index, testCase := range []struct {
			id    string
			kind  string
			badge Badge
		}{
			{id: "sess-restart-permission", kind: store.PendingInteractionKindPermission, badge: BadgeWaitingForAuth},
			{id: "sess-restart-clarify", kind: store.PendingInteractionKindClarify, badge: BadgeWaitingForInput},
		} {
			register(testCase.id)
			if _, err := catalog.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
				InteractionID: "interaction-" + testCase.id, SessionID: testCase.id,
				Kind: testCase.kind, ProviderRequestID: "request-" + testCase.id,
				Title: "Restart interaction", CreatedAt: baseAt.Add(time.Duration(index+1) * time.Second),
			}); err != nil {
				t.Fatalf("CreatePendingInteraction(%q) error = %v", testCase.id, err)
			}
		}
		register("sess-restart-done")
		if _, err := catalog.MarkSessionSettled(
			ctx,
			"sess-restart-done",
			false,
			baseAt.Add(3*time.Second),
		); err != nil {
			t.Fatalf("MarkSessionSettled(done) error = %v", err)
		}

		restarted := newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, restarted)
		for _, testCase := range []struct {
			id    string
			badge Badge
		}{
			{id: "sess-restart-permission", badge: BadgeWaitingForAuth},
			{id: "sess-restart-clarify", badge: BadgeWaitingForInput},
			{id: "sess-restart-done", badge: BadgeDone},
		} {
			target := &Session{ID: testCase.id, State: StateActive}
			if err := restarted.hydrateSessionAttention(ctx, target); err != nil {
				t.Fatalf("hydrateSessionAttention(%q) error = %v", testCase.id, err)
			}
			if got := BadgeForInfo(target.Info()); got != testCase.badge {
				t.Fatalf("BadgeForInfo(%q) = %q, want %q", testCase.id, got, testCase.badge)
			}
		}
		for _, transition := range []store.PendingInteractionTransition{
			{
				InteractionID: "interaction-sess-restart-permission",
				Status:        store.PendingInteractionStatusResolved,
				Resolution:    "allow-once",
				ResolvedBy:    "operator:test",
				At:            baseAt.Add(4 * time.Second),
			},
			{
				InteractionID: "interaction-sess-restart-clarify",
				Status:        store.PendingInteractionStatusTimedOut,
				At:            baseAt.Add(4 * time.Second),
			},
		} {
			if _, err := catalog.TransitionPendingInteraction(ctx, transition); err != nil {
				t.Fatalf("TransitionPendingInteraction(%q) error = %v", transition.InteractionID, err)
			}
			targetID := strings.TrimPrefix(transition.InteractionID, "interaction-")
			target := &Session{ID: targetID, State: StateActive}
			if err := restarted.hydrateSessionAttention(ctx, target); err != nil {
				t.Fatalf("hydrateSessionAttention(%q after clear) error = %v", targetID, err)
			}
			if got := BadgeForInfo(target.Info()); got != BadgeIdle {
				t.Fatalf("BadgeForInfo(%q after clear) = %q, want %q", targetID, got, BadgeIdle)
			}
		}

		if _, err := catalog.MarkSessionSeen(ctx, "sess-restart-done", baseAt.Add(5*time.Second)); err != nil {
			t.Fatalf("MarkSessionSeen() error = %v", err)
		}
		seen := &Session{ID: "sess-restart-done", State: StateActive}
		if err := restarted.hydrateSessionAttention(ctx, seen); err != nil {
			t.Fatalf("hydrateSessionAttention(seen) error = %v", err)
		}
		if got := BadgeForInfo(seen.Info()); got != BadgeIdle {
			t.Fatalf("BadgeForInfo(seen restart) = %q, want %q", got, BadgeIdle)
		}
	})

	t.Run("Should keep memory meta and catalog consistent on stopping transition", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)

		if err := h.manager.RequestStopWithCause(testutil.Context(t), session.ID, CauseUserRequested, ""); err != nil {
			t.Fatalf("RequestStopWithCause() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		if got, want := session.Info().State, StateStopping; got != want {
			t.Fatalf("memory state after request stop = %q, want %q", got, want)
		}
		stoppingMeta := readMeta(t, session.MetaPath())
		if stoppingMeta.State != string(StateStopping) {
			t.Fatalf("meta state after request stop = %q, want %q", stoppingMeta.State, StateStopping)
		}
		stoppingCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after request stop", session.ID)
		}
		if stoppingCatalog.State != string(StateStopping) {
			t.Fatalf("catalog state after request stop = %q, want %q", stoppingCatalog.State, StateStopping)
		}
		stoppingAttention := stoppingCatalog.AttentionSnapshot()
		if stoppingAttention.AttentionRevision != 1 || stoppingAttention.AttentionChangedAt == nil ||
			!stoppingAttention.AttentionChangedAt.Equal(stoppingCatalog.UpdatedAt) {
			t.Fatalf("catalog attention after request stop = %#v, want revision 1 at updated_at", stoppingCatalog)
		}
		if stoppingCatalog.Provider != stoppingMeta.Provider || stoppingCatalog.Model != stoppingMeta.Model ||
			stoppingCatalog.ReasoningEffort != stoppingMeta.ReasoningEffort || stoppingCatalog.Speed != stoppingMeta.Speed ||
			stoppingCatalog.RuntimeStatus != stoppingMeta.RuntimeStatus ||
			stoppingCatalog.RuntimeTransition != stoppingMeta.RuntimeTransition ||
			stoppingCatalog.RuntimeFailure != store.SessionRuntimeFailureValue(stoppingMeta.RuntimeFailure) {
			t.Fatalf(
				"catalog runtime after request stop = %#v, want metadata runtime %#v",
				stoppingCatalog,
				stoppingMeta,
			)
		}
	})

	t.Run("Should leave metadata as recovery source when stopping catalog update fails", func(t *testing.T) {
		t.Parallel()

		updateErr := errors.New("catalog update failed")
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		catalog.setUpdateErr(updateErr)

		err := h.manager.RequestStopWithCause(testutil.Context(t), session.ID, CauseUserRequested, "")
		if !errors.Is(err, updateErr) {
			t.Fatalf("RequestStopWithCause() error = %v, want wrapped catalog error", err)
		}
		t.Cleanup(func() {
			catalog.setUpdateErr(nil)
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		stoppingMeta := readMeta(t, session.MetaPath())
		if stoppingMeta.State != string(StateStopping) {
			t.Fatalf("meta state after catalog failure = %q, want %q", stoppingMeta.State, StateStopping)
		}
		activeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after failed update", session.ID)
		}
		if activeCatalog.State != string(StateActive) {
			t.Fatalf("catalog state after failed update = %q, want %q", activeCatalog.State, StateActive)
		}

		catalog.setUpdateErr(nil)
		_, reconcileErr := catalog.ReconcileSessions(testutil.Context(t), []store.SessionInfo{{
			ID:          session.ID,
			WorkspaceID: stoppingMeta.WorkspaceID,
			State:       stoppingMeta.State,
			UpdatedAt:   stoppingMeta.UpdatedAt,
		}})
		if reconcileErr != nil {
			t.Fatalf("ReconcileSessions() error = %v", reconcileErr)
		}
		reconciledCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after reconcile", session.ID)
		}
		if reconciledCatalog.State != string(StateStopping) {
			t.Fatalf("catalog state after reconcile = %q, want %q", reconciledCatalog.State, StateStopping)
		}
	})

	t.Run("Should keep memory meta and catalog consistent on active and stopped transitions", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)

		activeInfo := session.Info()
		if activeInfo.State != StateActive {
			t.Fatalf("memory state after create = %q, want %q", activeInfo.State, StateActive)
		}
		activeMeta := readMeta(t, session.MetaPath())
		if activeMeta.State != string(StateActive) {
			t.Fatalf("meta state after create = %q, want %q", activeMeta.State, StateActive)
		}
		activeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after create", session.ID)
		}
		if activeCatalog.State != string(StateActive) {
			t.Fatalf("catalog state after create = %q, want %q", activeCatalog.State, StateActive)
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stoppedMeta := readMeta(t, session.MetaPath())
		if stoppedMeta.State != string(StateStopped) {
			t.Fatalf("meta state after stop = %q, want %q", stoppedMeta.State, StateStopped)
		}
		stoppedCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after stop", session.ID)
		}
		if stoppedCatalog.State != string(StateStopped) {
			t.Fatalf("catalog state after stop = %q, want %q", stoppedCatalog.State, StateStopped)
		}
		stoppedAttention := stoppedCatalog.AttentionSnapshot()
		if stoppedAttention.AttentionRevision == 0 || stoppedAttention.AttentionChangedAt == nil {
			t.Fatalf("catalog attention after stop = %#v, want durable lifecycle edge", stoppedCatalog)
		}
	})

	t.Run("Should register stopped startup failure when create fails before active registration", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		catalog.requireExistingUpdates()
		h := newHarness(t, WithSessionCatalog(catalog))
		startErr := errors.New("driver start failed")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, startErr
		}

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "failed",
			Workspace: h.workspaceID,
		})
		if err == nil {
			t.Fatal("Create() error = nil, want startup failure")
		}
		if errors.Is(err, errRecordingCatalogMissingSession) {
			t.Fatalf("Create() error = %v, want failed start to register catalog row", err)
		}

		failedMeta := readMeta(t, store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "sess-1")))
		assertStartupFailureMeta(t, failedMeta)
		failedCatalog, ok := catalog.get("sess-1")
		if !ok {
			t.Fatal("catalog missing failed create session sess-1")
		}
		assertStartupFailureCatalog(t, failedCatalog)
	})

	t.Run("Should restore catalog projection from metadata after resume start failure", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		beforeMeta := readMeta(t, session.MetaPath())
		beforeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing stopped session %q before resume", session.ID)
		}
		resumeErr := errors.New("resume failed")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, resumeErr
		}

		_, err := h.manager.Resume(testutil.Context(t), session.ID)
		if !errors.Is(err, resumeErr) {
			t.Fatalf("Resume() error = %v, want wrapped resume failure", err)
		}
		afterMeta := readMeta(t, session.MetaPath())
		afterCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after failed resume", session.ID)
		}
		assertRestoredResumeMeta(t, afterMeta, beforeMeta)
		assertRestoredResumeCatalog(t, afterCatalog, beforeCatalog)
	})

	t.Run("Should reject archive after a resume transition begins", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		stopped := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		archiveReadStarted := make(chan struct{})
		releaseArchiveRead := make(chan struct{})
		var archiveReadOnce sync.Once
		catalog.archiveReadHook = func() {
			archiveReadOnce.Do(func() { close(archiveReadStarted) })
			<-releaseArchiveRead
		}
		resumeResult := make(chan error, 1)
		go func() {
			_, resumeErr := h.manager.Resume(testutil.Context(t), stopped.ID)
			resumeResult <- resumeErr
		}()
		<-archiveReadStarted

		_, archiveErr := h.manager.Archive(testutil.Context(t), h.workspaceID, stopped.ID)
		if !errors.Is(archiveErr, ErrSessionArchiveRequiresStopped) {
			t.Fatalf("Archive() error = %v, want ErrSessionArchiveRequiresStopped", archiveErr)
		}
		close(releaseArchiveRead)
		if resumeErr := <-resumeResult; resumeErr != nil {
			t.Fatalf("Resume() error = %v", resumeErr)
		}
		if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
			t.Fatalf("Stop(resumed) error = %v", err)
		}
	})

	t.Run("Should synchronize a successful attach CAS into the active session", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		attachedAt := active.Info().UpdatedAt.Add(time.Minute)
		attach, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        attachedAt,
			TTL:        time.Minute,
		})
		if err != nil {
			t.Fatalf("AttachSession() error = %v", err)
		}
		info := active.Info()
		if info.AttachedTo != attach.AttachedTo || info.AttachExpiresAt == nil ||
			!info.AttachExpiresAt.Equal(attach.AttachExpiresAt) || !info.UpdatedAt.Equal(attachedAt) {
			t.Fatalf("active info after attach = %#v, want synchronized lock %#v", info, attach)
		}
		if AttachableForInfo(info, attachedAt) {
			t.Fatal("AttachableForInfo(attached) = true, want locked")
		}
		healthByID, err := h.manager.SessionHealthForPage(testutil.Context(t), []*Info{info})
		if err != nil {
			t.Fatalf("SessionHealthForPage() error = %v", err)
		}
		if healthByID[active.ID].Attachable {
			t.Fatalf("session health after attach = %#v, want not attachable", healthByID[active.ID])
		}
	})

	t.Run("Should leave active memory unchanged when attach CAS fails", func(t *testing.T) {
		t.Parallel()

		attachErr := errors.New("attach CAS failed")
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		before := active.Info()
		catalog.setAttachErr(attachErr)
		_, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        before.UpdatedAt.Add(time.Minute),
			TTL:        time.Minute,
		})
		if !errors.Is(err, attachErr) {
			t.Fatalf("AttachSession() error = %v, want wrapped CAS failure", err)
		}
		after := active.Info()
		if after.AttachedTo != before.AttachedTo || after.AttachExpiresAt != nil ||
			!after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("active info after failed CAS = %#v, want unchanged %#v", after, before)
		}
	})

	t.Run("Should persist selected runtime without changing the active effective runtime", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		before := active.Info()
		updated, err := h.manager.SetRuntimeSelection(
			testutil.Context(t),
			active.ID,
			RuntimeSelection{Provider: "claude", Model: "claude-fable-5", ReasoningEffort: "max"},
			0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		if updated.SelectedRuntime == nil || updated.SelectedRuntime.Provider != "claude" ||
			updated.SelectedRuntime.Model != "claude-fable-5" ||
			updated.SelectedRuntime.ReasoningEffort != "max" || updated.RuntimeSelectionRevision != 1 {
			t.Fatalf("SetRuntimeSelection() info = %#v, want Claude selection at revision 1", updated)
		}
		if updated.Provider != before.Provider || updated.Model != before.Model ||
			updated.ReasoningEffort != before.ReasoningEffort || updated.RuntimeStatus != before.RuntimeStatus {
			t.Fatalf("effective runtime after selection = %#v, want unchanged %#v", updated, before)
		}
		meta := readMeta(t, active.MetaPath())
		selectedRuntime, selectionRevision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelection)
		catalogInfo, ok := catalog.get(active.ID)
		if !ok || selectedRuntime == nil || catalogInfo.SelectedRuntime == nil ||
			selectionRevision != 1 || catalogInfo.RuntimeSelectionRevision != 1 {
			t.Fatalf(
				"durable selection meta = %#v catalog = %#v, want revision 1",
				selectedRuntime,
				catalogInfo.SelectedRuntime,
			)
		}
		_, err = h.manager.ClearRuntimeSelection(testutil.Context(t), active.ID, 0)
		if !errors.Is(err, ErrRuntimeSelectionConflict) {
			t.Fatalf("ClearRuntimeSelection(stale revision) error = %v, want revision conflict", err)
		}
		cleared, err := h.manager.ClearRuntimeSelection(testutil.Context(t), active.ID, 1)
		if err != nil {
			t.Fatalf("ClearRuntimeSelection(active) error = %v", err)
		}
		if cleared.State != StateActive || cleared.SelectedRuntime != nil ||
			cleared.RuntimeSelectionRevision != 2 {
			t.Fatalf("ClearRuntimeSelection(active) info = %#v, want active clear at revision 2", cleared)
		}
		clearedMeta := readMeta(t, active.MetaPath())
		clearedSelection, clearedRevision := store.SessionRuntimeSelectionStateValues(clearedMeta.RuntimeSelection)
		clearedCatalog, ok := catalog.get(active.ID)
		if !ok || clearedSelection != nil || clearedCatalog.SelectedRuntime != nil ||
			clearedRevision != 2 || clearedCatalog.RuntimeSelectionRevision != 2 {
			t.Fatalf(
				"cleared active selection meta = %#v catalog = %#v, want revision 2",
				clearedSelection,
				clearedCatalog.SelectedRuntime,
			)
		}
	})

	t.Run("Should persist only an advertised Cursor runtime selection", func(t *testing.T) {
		t.Parallel()

		const advertisedModel = "grok-4.5"
		const transportModel = "cursor-grok-4.5-high"
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
			ProviderID:             "cursor",
			ModelID:                advertisedModel,
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
				resolvedWorkspace.Agents[index].Model = ""
			}
		}
		h.resolver.upsert(&resolvedWorkspace)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-cursor-runtime-selection")
			proc.handle.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
				ID:             "model",
				Category:       "model",
				Kind:           acp.SessionConfigOptionKindSelect,
				CurrentValueID: transportModel,
				Values:         []acp.SessionConfigOptionValue{{Value: transportModel}},
			}}})
			return proc, nil
		}
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		_, err = h.manager.SetRuntimeSelection(
			testutil.Context(t),
			active.ID,
			RuntimeSelection{Provider: "cursor", Model: "cursor-grok-4.5-high"},
			0,
		)
		negotiationErr, ok := errors.AsType[*acp.NegotiationError](err)
		if !ok || negotiationErr.Code != acp.NegotiationCodeModelUnavailable {
			t.Fatalf("SetRuntimeSelection(alias) error = %v, want model_unavailable NegotiationError", err)
		}
		rejectedMeta := readMeta(t, active.MetaPath())
		rejectedSelection, rejectedRevision := store.SessionRuntimeSelectionStateValues(rejectedMeta.RuntimeSelection)
		if rejectedSelection != nil || rejectedRevision != 0 {
			t.Fatalf(
				"rejected Cursor selection = %#v at revision %d, want no persisted selection",
				rejectedSelection,
				rejectedRevision,
			)
		}

		updated, err := h.manager.SetRuntimeSelection(
			testutil.Context(t),
			active.ID,
			RuntimeSelection{Provider: "cursor", Model: advertisedModel},
			0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(advertised model) error = %v", err)
		}
		if updated.SelectedRuntime == nil || updated.SelectedRuntime.Model != advertisedModel ||
			updated.RuntimeSelectionRevision != 1 {
			t.Fatalf(
				"SetRuntimeSelection(advertised model) info = %#v, want exact persisted model at revision 1",
				updated,
			)
		}
	})

	t.Run("Should update selected runtime while stopped without starting ACP", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		h.driver.mu.Lock()
		startsBefore := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		updated, err := h.manager.SetRuntimeSelection(
			testutil.Context(t),
			active.ID,
			RuntimeSelection{Provider: "claude", Model: "claude-fable-5", ReasoningEffort: "max"},
			0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(stopped) error = %v", err)
		}
		if updated.State != StateStopped || updated.SelectedRuntime == nil ||
			updated.SelectedRuntime.Model != "claude-fable-5" || updated.RuntimeSelectionRevision != 1 {
			t.Fatalf("SetRuntimeSelection(stopped) info = %#v, want stopped Claude selection at revision 1", updated)
		}
		cleared, err := h.manager.ClearRuntimeSelection(testutil.Context(t), active.ID, 1)
		if err != nil {
			t.Fatalf("ClearRuntimeSelection(stopped) error = %v", err)
		}
		if cleared.State != StateStopped || cleared.SelectedRuntime != nil ||
			cleared.RuntimeSelectionRevision != 2 {
			t.Fatalf("ClearRuntimeSelection(stopped) info = %#v, want stopped clear at revision 2", cleared)
		}
		clearedMeta := readMeta(t, active.MetaPath())
		clearedSelection, clearedRevision := store.SessionRuntimeSelectionStateValues(clearedMeta.RuntimeSelection)
		clearedCatalog, ok := catalog.get(active.ID)
		if !ok || clearedSelection != nil || clearedCatalog.SelectedRuntime != nil ||
			clearedRevision != 2 || clearedCatalog.RuntimeSelectionRevision != 2 {
			t.Fatalf(
				"cleared stopped selection meta = %#v catalog = %#v, want revision 2",
				clearedSelection,
				clearedCatalog.SelectedRuntime,
			)
		}
		h.driver.mu.Lock()
		startsAfter := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if got := startsAfter; got != startsBefore {
			t.Fatalf("driver starts after stopped selection = %d, want %d", got, startsBefore)
		}
	})

	t.Run("Should rename an active user session across memory metadata catalog and events", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		events, cancel, err := h.manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		updated, err := h.manager.Rename(
			testutil.Context(t),
			h.workspaceID,
			active.ID,
			"  Checkout\n  webhook   retries  ",
		)
		if err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		const want = "Checkout webhook retries"
		catalogInfo, ok := catalog.get(active.ID)
		if updated.Name != want || active.Info().Name != want ||
			readMeta(t, active.MetaPath()).Name != want || !ok || catalogInfo.Name != want {
			t.Fatalf("renamed identity = info %q memory %q catalog %#v", updated.Name, active.Info().Name, catalogInfo)
		}
		select {
		case event := <-events:
			if event.Kind != CatalogEventUpserted ||
				event.WorkspaceID != h.workspaceID ||
				event.SessionID != active.ID {
				t.Fatalf("rename catalog event = %#v, want upserted %s/%s", event, h.workspaceID, active.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("rename catalog event was not published")
		}
	})

	t.Run("Should rename a stopped archived session without starting ACP or losing archive state", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if _, err := h.manager.Archive(testutil.Context(t), h.workspaceID, active.ID); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		h.driver.mu.Lock()
		startsBefore := len(h.driver.startCalls)
		h.driver.mu.Unlock()

		updated, err := h.manager.Rename(testutil.Context(t), h.workspaceID, active.ID, "Archived checkout")
		if err != nil {
			t.Fatalf("Rename(stopped) error = %v", err)
		}
		if updated.Name != "Archived checkout" || updated.ArchivedAt == nil {
			t.Fatalf("Rename(stopped) = %#v, want renamed archived session", updated)
		}
		catalogInfo, ok := catalog.get(active.ID)
		if !ok || catalogInfo.Name != updated.Name || catalogInfo.ArchivedAtValue() == nil {
			t.Fatalf("archived catalog after rename = %#v", catalogInfo)
		}
		h.driver.mu.Lock()
		startsAfter := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if startsAfter != startsBefore {
			t.Fatalf("driver starts after stopped rename = %d, want %d", startsAfter, startsBefore)
		}
	})

	t.Run("Should reject invalid manual names managed sessions and foreign workspaces", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		for _, test := range []struct {
			name        string
			workspaceID string
			value       string
			want        error
		}{
			{name: "empty", workspaceID: h.workspaceID, value: " \n\t ", want: ErrValidation},
			{name: "too long", workspaceID: h.workspaceID, value: strings.Repeat("界", SessionNameMaxRunes+1), want: ErrValidation},
			{name: "foreign workspace", workspaceID: "ws-foreign", value: "Private", want: ErrSessionNotFound},
		} {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				_, err := h.manager.Rename(testutil.Context(t), test.workspaceID, active.ID, test.value)
				if !errors.Is(err, test.want) {
					t.Fatalf("Rename() error = %v, want %v", err, test.want)
				}
			})
		}

		managed, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("Create(system) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, managed.ID) })
		_, err = h.manager.Rename(
			testutil.Context(t),
			h.workspaceID,
			managed.ID,
			"Operator name",
		)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Rename(system) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should roll back an active rename when catalog persistence fails", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		before := active.Info()
		persistErr := errors.New("catalog rename unavailable")
		catalog.registerHook = func(_ context.Context, info store.SessionInfo) error {
			if info.Name == "New name" {
				return persistErr
			}
			return nil
		}

		_, err := h.manager.Rename(testutil.Context(t), h.workspaceID, active.ID, "New name")
		if !errors.Is(err, persistErr) {
			t.Fatalf("Rename() error = %v, want catalog failure", err)
		}
		memoryName := active.Info().Name
		metaName := readMeta(t, active.MetaPath()).Name
		if memoryName != before.Name || metaName != before.Name {
			t.Fatalf("failed rename changed identity: memory=%q meta=%q", memoryName, metaName)
		}
	})
}

type runtimeConfigRecordingDriver struct {
	*fakeDriver
	configMu sync.Mutex
	configs  []acp.RuntimeConfig
}

func (d *runtimeConfigRecordingDriver) ConfigureRuntime(
	_ context.Context,
	_ *AgentProcess,
	config acp.RuntimeConfig,
) error {
	d.configMu.Lock()
	defer d.configMu.Unlock()
	config.ACPOptions = acp.CloneSessionConfigOptionSelections(config.ACPOptions)
	d.configs = append(d.configs, config)
	return nil
}

func (d *runtimeConfigRecordingDriver) configSnapshots() []acp.RuntimeConfig {
	d.configMu.Lock()
	defer d.configMu.Unlock()
	result := append([]acp.RuntimeConfig(nil), d.configs...)
	for index := range result {
		result[index].ACPOptions = acp.CloneSessionConfigOptionSelections(result[index].ACPOptions)
	}
	return result
}

func newOrphanResolutionHarness(
	t *testing.T,
	kind string,
	payload store.PendingInteractionPayload,
) (*harness, *Manager, *globaldb.GlobalDB, string) {
	t.Helper()

	ctx := testutil.Context(t)
	h := newHarness(t)
	cleanupTestManager(t, h.manager)
	target := createSession(t, h)
	if err := h.manager.Stop(ctx, target.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	catalog := openManagerInputQueueStore(t)
	registerManagerInputQueueWorkspace(t, catalog, h)
	registerManagerInputQueueSession(t, catalog, h, target)
	if _, err := catalog.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
		InteractionID:     "interaction-orphan",
		SessionID:         target.ID,
		Kind:              kind,
		ProviderRequestID: "request-orphan",
		TurnID:            "turn-orphan",
		Title:             "Which path?",
		Payload:           payload,
		CreatedAt:         time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreatePendingInteraction() error = %v", err)
	}
	restarted := newManagerWithHarness(
		t,
		h,
		WithSessionCatalog(catalog),
		WithSessionInputQueueStore(catalog),
	)
	cleanupTestManager(t, restarted)
	orphaned, err := restarted.RecoverPendingInteractions(ctx, target.ID)
	if err != nil {
		t.Fatalf("RecoverPendingInteractions() error = %v", err)
	}
	if orphaned != 1 {
		t.Fatalf("RecoverPendingInteractions() = %d, want 1", orphaned)
	}
	return h, restarted, catalog, target.ID
}

func assertOrphanResolutionQueue(
	t *testing.T,
	queueStore store.SessionInputQueueStore,
	sessionID string,
	wantText string,
) {
	t.Helper()

	entries, err := queueStore.ListPendingSessionInputs(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("ListPendingSessionInputs() error = %v", err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Text, wantText) {
		t.Fatalf("queued orphan resolution = %#v, want one entry containing %q", entries, wantText)
	}
}

type recordingSessionCatalog struct {
	mu              sync.Mutex
	sessions        map[string]store.SessionInfo
	registerHook    func(context.Context, store.SessionInfo) error
	updateErr       error
	updateHook      func(store.SessionStateUpdate) error
	deleteErr       error
	deleteHook      func(context.Context, string) error
	attachErr       error
	archiveReadHook func()
	strictUpdates   bool
}

func newRecordingSessionCatalog() *recordingSessionCatalog {
	return &recordingSessionCatalog{sessions: make(map[string]store.SessionInfo)}
}

func (c *recordingSessionCatalog) RegisterSession(ctx context.Context, session store.SessionInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.registerHook != nil {
		if err := c.registerHook(ctx, session); err != nil {
			return err
		}
	}
	c.sessions[session.ID] = session
	return nil
}

func (c *recordingSessionCatalog) UpdateSessionState(_ context.Context, update store.SessionStateUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.updateHook != nil {
		if err := c.updateHook(update); err != nil {
			return err
		}
	}
	if c.updateErr != nil {
		return c.updateErr
	}
	current, ok := c.sessions[update.ID]
	if c.strictUpdates && !ok {
		return errRecordingCatalogMissingSession
	}
	current.State = update.State
	current.ACPSessionID = update.ACPSessionID
	if update.RuntimeSet {
		current.Provider = update.Provider
		current.Model = update.Model
		current.ReasoningEffort = update.ReasoningEffort
		current.Speed = update.Speed
		current.SpeedResolution = speedpkg.CloneResolution(update.SpeedResolution)
		current.RuntimeStatus = update.RuntimeStatus
		current.RuntimeTransition = update.RuntimeTransition
		current.RuntimeFailure = update.RuntimeFailure
	}
	if update.SelectedRuntimeSet {
		current.SelectedRuntime = store.CloneSessionRuntimeSelection(update.SelectedRuntime)
		current.RuntimeSelectionRevision = update.RuntimeSelectionRevision
	}
	if update.StopReasonSet {
		current.StopReason = store.StopReason("")
		if update.StopReason != nil {
			current.StopReason = store.StopReason(*update.StopReason)
		}
		current.StopDetail = update.StopDetail
	}
	if update.FailureSet {
		current.Failure = store.CloneSessionFailure(update.Failure)
	}
	if update.AttentionTransition {
		attention := current.AttentionSnapshot()
		attention.AttentionRevision++
		changedAt := update.UpdatedAt.UTC()
		attention.AttentionChangedAt = &changedAt
		current.Attention = &attention
	}
	current.Liveness = store.CloneSessionLivenessMeta(update.Liveness)
	current.Sandbox = cloneSessionSandboxMeta(update.Sandbox)
	current.UpdatedAt = update.UpdatedAt
	c.sessions[update.ID] = current
	return nil
}

func (c *recordingSessionCatalog) DeleteSession(ctx context.Context, id string) error {
	if c.deleteHook != nil {
		if err := c.deleteHook(ctx, id); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deleteErr != nil {
		return c.deleteErr
	}
	if _, ok := c.sessions[id]; !ok {
		return store.ErrSessionNotFound
	}
	delete(c.sessions, id)
	return nil
}

func (c *recordingSessionCatalog) SessionArchivedAt(
	_ context.Context,
	workspaceID string,
	sessionID string,
) (*time.Time, error) {
	if c.archiveReadHook != nil {
		c.archiveReadHook()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.sessions[sessionID]
	if !ok || current.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	return cloneTimePointer(current.ArchivedAtValue()), nil
}

func (c *recordingSessionCatalog) SetSessionArchived(
	_ context.Context,
	workspaceID string,
	sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.sessions[sessionID]
	if !ok || current.WorkspaceID != workspaceID {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if archived && current.State != string(StateStopped) {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionArchiveRequiresStopped, sessionID)
	}
	if archived == (current.ArchivedAtValue() != nil) {
		return current, nil
	}
	if archived {
		now := time.Now().UTC()
		current.SetArchivedAt(&now)
	} else {
		current.SetArchivedAt(nil)
	}
	c.sessions[sessionID] = current
	return current, nil
}
func (c *recordingSessionCatalog) ListSessions(
	_ context.Context,
	query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]store.SessionInfo, 0, len(c.sessions))
	for _, session := range c.sessions {
		if query.WorkspaceID != "" && session.WorkspaceID != query.WorkspaceID {
			continue
		}
		out = append(out, session)
	}
	return out, nil
}

func (c *recordingSessionCatalog) AttachSession(
	_ context.Context,
	req store.SessionAttachRequest,
) (store.SessionAttach, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attachErr != nil {
		return store.SessionAttach{}, c.attachErr
	}
	normalized := req.Normalize()
	current, ok := c.sessions[normalized.SessionID]
	if !ok {
		return store.SessionAttach{}, store.ErrSessionNotFound
	}
	if current.State != string(StateActive) {
		return store.SessionAttach{}, store.ErrSessionNotAttachable
	}
	if current.AttachedTo != "" && current.AttachExpiresAtValue() != nil &&
		current.AttachExpiresAtValue().After(normalized.Now) {
		return store.SessionAttach{}, store.ErrSessionAttachLocked
	}
	expiresAt := normalized.Now.Add(normalized.TTL).UTC()
	current.AttachedTo = normalized.AttachedTo
	current.SetAttachExpiresAt(&expiresAt)
	current.UpdatedAt = normalized.Now
	c.sessions[normalized.SessionID] = current
	return store.SessionAttach{
		SessionID:       normalized.SessionID,
		AttachedTo:      normalized.AttachedTo,
		AttachedAt:      normalized.Now,
		AttachExpiresAt: expiresAt,
	}, nil
}

func (c *recordingSessionCatalog) ReconcileSessions(
	_ context.Context,
	sessions []store.SessionInfo,
) (store.ReconcileResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, session := range sessions {
		c.sessions[session.ID] = session
	}
	indexed := make([]string, 0, len(sessions))
	for _, session := range sessions {
		indexed = append(indexed, session.ID)
	}
	return store.ReconcileResult{Indexed: indexed}, nil
}

func (c *recordingSessionCatalog) get(id string) (store.SessionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[id]
	return session, ok
}

func (c *recordingSessionCatalog) setUpdateErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateErr = err
}

func (c *recordingSessionCatalog) setAttachErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attachErr = err
}

func (c *recordingSessionCatalog) requireExistingUpdates() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.strictUpdates = true
}

func assertStartupFailureMeta(t *testing.T, meta store.SessionMeta) {
	t.Helper()

	if meta.State != string(StateStopped) {
		t.Fatalf("failed meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.Failure == nil {
		t.Fatal("failed meta failure = nil, want startup failure")
	}
	if got, want := meta.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed meta failure kind = %q, want %q", got, want)
	}
}

func assertStartupFailureCatalog(t *testing.T, info store.SessionInfo) {
	t.Helper()

	if info.State != string(StateStopped) {
		t.Fatalf("failed catalog state = %q, want %q", info.State, StateStopped)
	}
	if info.Failure == nil {
		t.Fatal("failed catalog failure = nil, want startup failure")
	}
	if got, want := info.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed catalog failure kind = %q, want %q", got, want)
	}
}

func assertRestoredResumeMeta(t *testing.T, after store.SessionMeta, before store.SessionMeta) {
	t.Helper()

	if after.State != before.State {
		t.Fatalf("restored meta state = %q, want %q", after.State, before.State)
	}
	if derefString(after.ACPSessionID) != derefString(before.ACPSessionID) {
		t.Fatalf(
			"restored meta acp session id = %q, want %q",
			derefString(after.ACPSessionID),
			derefString(before.ACPSessionID),
		)
	}
	if sessionMetaStopReason(after) != sessionMetaStopReason(before) {
		t.Fatalf(
			"restored meta stop reason = %q, want %q",
			sessionMetaStopReason(after),
			sessionMetaStopReason(before),
		)
	}
	assertSameSessionFailure(t, after.Failure, before.Failure, "restored meta")
}

func assertRestoredResumeCatalog(t *testing.T, after store.SessionInfo, before store.SessionInfo) {
	t.Helper()

	if after.State != before.State {
		t.Fatalf("restored catalog state = %q, want %q", after.State, before.State)
	}
	if derefString(after.ACPSessionID) != derefString(before.ACPSessionID) {
		t.Fatalf(
			"restored catalog acp session id = %q, want %q",
			derefString(after.ACPSessionID),
			derefString(before.ACPSessionID),
		)
	}
	if after.StopReason != before.StopReason {
		t.Fatalf("restored catalog stop reason = %q, want %q", after.StopReason, before.StopReason)
	}
	if after.StopDetail != before.StopDetail {
		t.Fatalf("restored catalog stop detail = %q, want %q", after.StopDetail, before.StopDetail)
	}
	assertSameSessionFailure(t, after.Failure, before.Failure, "restored catalog")
}

func assertSameSessionFailure(
	t *testing.T,
	after *store.SessionFailure,
	before *store.SessionFailure,
	label string,
) {
	t.Helper()

	if before == nil || after == nil {
		if before != after {
			t.Fatalf("%s failure = %#v, want %#v", label, after, before)
		}
		return
	}
	if after.Kind != before.Kind {
		t.Fatalf("%s failure kind = %q, want %q", label, after.Kind, before.Kind)
	}
	if after.Summary != before.Summary {
		t.Fatalf("%s failure summary = %q, want %q", label, after.Summary, before.Summary)
	}
}
