package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/admission"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/sandbox"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	skillbundled "github.com/compozy/agh/skills"
)

func testLiveParticipation(workspaceID, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     strings.TrimSpace(workspaceID),
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       strings.TrimSpace(channelID),
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}
}

func testLocalParticipation() participation.Spec {
	return participation.LocalSpec()
}

func testLocalParticipationPtr() *participation.Spec {
	return participation.CloneSpec(testLocalParticipation())
}

func testLiveParticipationPtr(workspaceID, channelID string) *participation.Spec {
	spec := testLiveParticipation(workspaceID, channelID)
	return &spec
}

type recordingSessionParticipationResolver struct {
	inner        participation.Resolver
	calls        int
	observations []participation.ResolvedObservation
}

func (r *recordingSessionParticipationResolver) ObserveParticipationResolved(
	_ context.Context,
	observation participation.ResolvedObservation,
) error {
	r.observations = append(r.observations, observation)
	return nil
}

func (r *recordingSessionParticipationResolver) Resolve(
	ctx context.Context,
	in participation.ResolveInput,
) (participation.Spec, error) {
	r.calls++
	return r.inner.Resolve(ctx, in)
}

func newTestSessionParticipationResolver(t *testing.T, available bool) participation.Resolver {
	t.Helper()
	defaults := testLiveParticipation("ws-test", "builders").Bounds
	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: defaults,
		Limits: participation.Limits{
			MaxWakes:          16,
			MaxWakeWallTime:   "2m",
			MaxTotalWallTime:  "10m",
			MaxInputTokens:    65536,
			MaxOutputTokens:   65536,
			MaxWakeDepth:      16,
			MinCoalesceWindow: "100ms",
			MaxCoalesceWindow: "5s",
		},
		Availability: func(context.Context) (bool, error) {
			return available, nil
		},
		ChannelExists: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
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
	if got := len(h.resolver.resolveOrRegisterCalls); got != 0 {
		t.Fatalf("resolver ResolveOrRegister() calls = %d, want 0", got)
	}
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
		if got, want := decoded.RequestID, clarifyEvent.Request.RequestID; got != want {
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

		source <- acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           "turn-admitted",
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
}

func TestCreateAppliesRuntimeModelOverride(t *testing.T) {
	t.Parallel()

	const cursorModel = "grok-4.5[effort=high,fast=true]"
	canonicalCursorCatalog := modelCatalogFunc(func(
		_ context.Context,
		_ modelcatalog.ListOptions,
	) ([]modelcatalog.Model, error) {
		return []modelcatalog.Model{{ProviderID: "cursor", ModelID: cursorModel}}, nil
	})

	t.Run("Should reject an explicit model alias before driver or storage startup", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithModelCatalog(canonicalCursorCatalog))
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Provider:  "cursor",
			Model:     "cursor-grok-4.5-high",
			Workspace: h.workspaceID,
		})
		if !errors.Is(err, ErrInvalidRuntimeOverride) {
			t.Fatalf("Create() error = %v, want ErrInvalidRuntimeOverride", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want 0", got)
		}
		entries, readErr := os.ReadDir(h.homePaths.SessionsDir)
		if readErr != nil {
			t.Fatalf("ReadDir(sessions) error = %v", readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("session storage entries = %d, want 0", len(entries))
		}
	})

	t.Run("Should recommend only available explicit models", func(t *testing.T) {
		t.Parallel()

		catalog := modelCatalogFunc(func(
			_ context.Context,
			_ modelcatalog.ListOptions,
		) ([]modelcatalog.Model, error) {
			return []modelcatalog.Model{
				{ProviderID: "cursor", ModelID: "available-model", Available: new(true)},
				{ProviderID: "cursor", ModelID: "unavailable-model", Available: new(false)},
			}, nil
		})
		h := newHarness(t, WithModelCatalog(catalog))
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Provider:  "cursor",
			Model:     "missing-model",
			Workspace: h.workspaceID,
		})
		if !errors.Is(err, ErrInvalidRuntimeOverride) {
			t.Fatalf("Create() error = %v, want ErrInvalidRuntimeOverride", err)
		}
		if !strings.Contains(err.Error(), "choose one of: available-model") {
			t.Fatalf("Create() error = %v, want available-model recommendation", err)
		}
		if strings.Contains(err.Error(), "unavailable-model") {
			t.Fatalf("Create() error = %v, want unavailable model omitted", err)
		}
	})

	t.Run("Should report when a provider has no available explicit models", func(t *testing.T) {
		t.Parallel()

		catalog := modelCatalogFunc(func(
			_ context.Context,
			_ modelcatalog.ListOptions,
		) ([]modelcatalog.Model, error) {
			return []modelcatalog.Model{
				{ProviderID: "cursor", ModelID: "unavailable-model", Available: new(false)},
			}, nil
		})
		h := newHarness(t, WithModelCatalog(catalog))
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Provider:  "cursor",
			Model:     "unavailable-model",
			Workspace: h.workspaceID,
		})
		if !errors.Is(err, ErrInvalidRuntimeOverride) {
			t.Fatalf("Create() error = %v, want ErrInvalidRuntimeOverride", err)
		}
		if !strings.Contains(err.Error(), "has no available explicit models") {
			t.Fatalf("Create() error = %v, want no available explicit models", err)
		}
	})

	t.Run("Should accept and persist the exact catalog descriptor", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithModelCatalog(canonicalCursorCatalog))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Provider:  "cursor",
			Model:     cursorModel,
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
		if got := readMeta(t, session.MetaPath()).Model; got != cursorModel {
			t.Fatalf("meta.Model = %q, want %q", got, cursorModel)
		}
	})

	t.Run("Should preserve native default startup and persist the provider current model", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
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
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if opts.PreferredModel != "" {
				t.Fatalf("StartOpts.PreferredModel = %q, want provider-native default", opts.PreferredModel)
			}
			proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-cursor-native")
			proc.handle.Caps.ConfigOptions = []acp.SessionConfigOption{{
				ID:      "model",
				Kind:    acp.SessionConfigOptionKindSelect,
				Current: cursorModel,
				Values:  []acp.SessionConfigOptionValue{{Value: cursorModel}},
			}}
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
		if got := session.Info().Model; got != cursorModel {
			t.Fatalf("session.Info().Model = %q, want %q", got, cursorModel)
		}
		if got := readMeta(t, session.MetaPath()).Model; got != cursorModel {
			t.Fatalf("meta.Model = %q, want %q", got, cursorModel)
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
	waitForCondition(t, "stop notification after immediate exit", func() bool {
		return h.notifier.stoppedCount() == 1
	})

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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := len(h.resolver.resolveCalls); got != 1 {
		t.Fatalf("resolver Resolve() calls = %d, want 1", got)
	}
	if got, want := h.resolver.resolveCalls[0], session.Info().WorkspaceID; got != want {
		t.Fatalf("resolver Resolve() arg = %q, want %q", got, want)
	}
	if got := len(h.resolver.resolveOrRegisterCalls); got != 1 {
		t.Fatalf("resolver ResolveOrRegister() calls = %d, want 1", got)
	}
	if got, want := h.resolver.resolveOrRegisterCalls[0], normalizeResolverPath(workspacePath); got != want {
		t.Fatalf("resolver ResolveOrRegister() arg = %q, want %q", got, want)
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
}

func TestStopTransitionsToStoppedAndNotifies(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) after Stop() = found, want missing", session.ID)
	}
	if got := h.notifier.stoppedCount(); got != 1 {
		t.Fatalf("stopped notifications = %d, want 1", got)
	}
	meta := readMeta(t, session.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}
	if got := session.Info().StopReason; got != store.StopUserCanceled {
		t.Fatalf("session.Info().StopReason = %q, want %q", got, store.StopUserCanceled)
	}

	events := readStoredEvents(t, session)
	stopEvent := storedEventByType(t, events, EventTypeSessionStopped)
	stopPayload := decodeStoredEventPayload(t, stopEvent)
	if got, want := stopPayload["stop_reason"], string(store.StopUserCanceled); got != want {
		t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
	}
}

func TestResumeLoadsMetaAndPassesStoredACPSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Errorf("Stop(resumed) error = %v", err)
		}
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := resumed.Info().ACPSessionID; got != originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want %q", got, originalACP)
	}
	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if got := resumed.Info().StopReason; got != "" {
		t.Fatalf("resumed stop reason = %q, want empty", got)
	}
	if got := resumed.Info().StopDetail; got != "" {
		t.Fatalf("resumed stop detail = %q, want empty", got)
	}
}

func TestCreateAndResumePreserveNetworkParticipation(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	wantSpec := testLiveParticipation(h.workspaceID, "builders")
	wantOwnerKey := "session:" + session.ID
	if got := session.Info().NetworkParticipation; got != wantSpec {
		t.Fatalf("Create() participation = %#v, want %#v", got, wantSpec)
	}
	if got := session.Info().NetworkOwnerKey; got != wantOwnerKey {
		t.Fatalf("Create() owner key = %q, want %q", got, wantOwnerKey)
	}
	if meta := readMeta(t, session.MetaPath()); meta.NetworkSpecSnapshot() != wantSpec ||
		meta.NetworkOwnerKeySnapshot() != wantOwnerKey {
		t.Fatalf(
			"meta network identity = (%#v, %q), want (%#v, %q)",
			meta.NetworkSpecSnapshot(),
			meta.NetworkOwnerKeySnapshot(),
			wantSpec,
			wantOwnerKey,
		)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stopped, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(stopped) error = %v", err)
	}
	if got := stopped.NetworkParticipation; got != wantSpec {
		t.Fatalf("Status(stopped).NetworkParticipation = %#v, want %#v", got, wantSpec)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := resumed.Info().NetworkParticipation; got != wantSpec {
		t.Fatalf("Resume() participation = %#v, want %#v", got, wantSpec)
	}
	if got := resumed.Info().NetworkOwnerKey; got != wantOwnerKey {
		t.Fatalf("Resume() owner key = %q, want immutable %q", got, wantOwnerKey)
	}
	if meta := readMeta(t, resumed.MetaPath()); meta.NetworkSpecSnapshot() != wantSpec ||
		meta.NetworkOwnerKeySnapshot() != wantOwnerKey {
		t.Fatalf(
			"resumed meta network identity = (%#v, %q), want (%#v, %q)",
			meta.NetworkSpecSnapshot(),
			meta.NetworkOwnerKeySnapshot(),
			wantSpec,
			wantOwnerKey,
		)
	}
}

func TestCreateParticipationResolvesBeforeWritesAndResumeReusesSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a live request once and reuse the persisted snapshot on resume", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		resolver := &recordingSessionParticipationResolver{
			inner: newTestSessionParticipationResolver(t, true),
		}
		h.manager = newManagerWithHarness(t, h, WithParticipationResolver(resolver))
		live := participation.ModeLive
		named := participation.StrategyNamed
		channelID := "builders"
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &channelID,
			},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got, want := resolver.calls, 1; got != want {
			t.Fatalf("resolver calls after create = %d, want %d", got, want)
		}
		createdSpec := session.Info().NetworkParticipation
		if got, want := createdSpec.Source, participation.SourceExplicitRequest; got != want {
			t.Fatalf("created source = %q, want %q", got, want)
		}
		if got, want := readMeta(t, session.MetaPath()).NetworkSpecSnapshot(), createdSpec; got != want {
			t.Fatalf("persisted snapshot = %#v, want %#v", got, want)
		}
		if got, want := len(resolver.observations), 1; got != want {
			t.Fatalf("resolved observations after create = %d, want %d", got, want)
		}
		observation := resolver.observations[0]
		if observation.Owner.ID != session.ID || observation.Owner.WorkspaceID != h.workspaceID ||
			observation.Spec != createdSpec {
			t.Fatalf("resolved observation = %#v, want committed session snapshot", observation)
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Errorf("Stop(resumed) error = %v", err)
			}
		})
		if got, want := resolver.calls, 1; got != want {
			t.Fatalf("resolver calls after resume = %d, want %d", got, want)
		}
		if got, want := resumed.Info().NetworkParticipation, createdSpec; got != want {
			t.Fatalf("resumed snapshot = %#v, want %#v", got, want)
		}
		if got, want := len(resolver.observations), 1; got != want {
			t.Fatalf("resolved observations after resume = %d, want %d", got, want)
		}
	})

	t.Run("Should not observe a resolved snapshot when session creation rolls back", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		resolver := &recordingSessionParticipationResolver{
			inner: newTestSessionParticipationResolver(t, true),
		}
		h.manager = newManagerWithHarness(t, h, WithParticipationResolver(resolver))
		startErr := errors.New("provider start failed after participation resolution")
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
			return nil, startErr
		}
		live := participation.ModeLive
		named := participation.StrategyNamed
		channelID := "builders"
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &channelID,
			},
		})
		if !errors.Is(err, startErr) {
			t.Fatalf("Create() error = %v, want %v", err, startErr)
		}
		if got := len(resolver.observations); got != 0 {
			t.Fatalf("resolved observations after rollback = %d, want 0", got)
		}
	})

	t.Run("Should reject unavailable live participation before session state exists", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.manager = newManagerWithHarness(
			t,
			h,
			WithParticipationResolver(newTestSessionParticipationResolver(t, false)),
		)
		live := participation.ModeLive
		named := participation.StrategyNamed
		channelID := "builders"
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &named,
				ChannelID:       &channelID,
			},
		})
		if !errors.Is(err, participation.ErrUnavailable) {
			t.Fatalf("Create() error = %v, want %v", err, participation.ErrUnavailable)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver starts = %d, want 0", got)
		}
		entries, readErr := os.ReadDir(h.homePaths.SessionsDir)
		if readErr != nil {
			t.Fatalf("ReadDir(sessions) error = %v", readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("session artifacts = %#v, want none", entries)
		}
	})
}

func TestCreateResumeAndStopInvokeLateBoundNetworkPeerLifecycle(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	lifecycle := newFakeNetworkPeerLifecycle()
	h.manager.SetNetworkPeerLifecycle(lifecycle)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got := lifecycle.joinCount(); got != 1 {
		t.Fatalf("join calls after Create() = %d, want 1", got)
	}
	firstJoin := lifecycle.joinCall(0)
	if got, want := firstJoin.sessionID, session.ID; got != want {
		t.Fatalf("first join session_id = %q, want %q", got, want)
	}
	if got, want := firstJoin.peerID, "coder."+session.ID; got != want {
		t.Fatalf("first join peer_id = %q, want %q", got, want)
	}
	if got, want := firstJoin.channel, "builders"; got != want {
		t.Fatalf("first join channel = %q, want %q", got, want)
	}
	if firstJoin.capabilities == nil {
		t.Fatal("first join capabilities = nil, want deterministic empty projection")
	}
	if got := len(firstJoin.capabilities); got != 0 {
		t.Fatalf("first join capabilities len = %d, want 0", got)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if got := lifecycle.leaveCount(); got != 1 {
		t.Fatalf("leave calls after Stop() = %d, want 1", got)
	}
	if got, want := lifecycle.leaveCall(0), session.ID; got != want {
		t.Fatalf("first leave session_id = %q, want %q", got, want)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if got := lifecycle.joinCount(); got != 2 {
		t.Fatalf("join calls after Resume() = %d, want 2", got)
	}
	secondJoin := lifecycle.joinCall(1)
	if got, want := secondJoin.sessionID, resumed.ID; got != want {
		t.Fatalf("second join session_id = %q, want %q", got, want)
	}
	if got, want := secondJoin.peerID, "coder."+resumed.ID; got != want {
		t.Fatalf("second join peer_id = %q, want %q", got, want)
	}
	if got, want := secondJoin.channel, "builders"; got != want {
		t.Fatalf("second join channel = %q, want %q", got, want)
	}
	if secondJoin.capabilities == nil {
		t.Fatal("second join capabilities = nil, want deterministic empty projection")
	}
	if got := len(secondJoin.capabilities); got != 0 {
		t.Fatalf("second join capabilities len = %d, want 0", got)
	}

	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("Stop(resumed) error = %v", err)
	}
	if got := lifecycle.leaveCount(); got != 2 {
		t.Fatalf("leave calls after resumed Stop() = %d, want 2", got)
	}
	if got, want := lifecycle.leaveCall(1), resumed.ID; got != want {
		t.Fatalf("second leave session_id = %q, want %q", got, want)
	}
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
}

func TestResumeRepairsIncompleteStartAndStartsFreshACPClient(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateStarting)
	meta.StopReason = nil
	meta.StopDetail = ""
	meta.ACPSessionID = stringPointer(originalACP)
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(incomplete start) error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != "" {
		t.Fatalf("resume start ResumeSessionID = %q, want empty for repaired start", got)
	}
	if got := resumed.Info().ACPSessionID; got == "" || got == originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want fresh ACP session id distinct from %q", got, originalACP)
	}
	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if got := resumed.Info().StopReason; got != "" {
		t.Fatalf("resumed stop reason = %q, want empty", got)
	}
	if got := resumed.Info().StopDetail; got != "" {
		t.Fatalf("resumed stop detail = %q, want empty", got)
	}
}

func TestResumePreservesCrashStopClassificationFromRepairedMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if got := resumed.Info().StopReason; got != store.StopAgentCrashed {
		t.Fatalf("resumed stop reason = %q, want %q", got, store.StopAgentCrashed)
	}
	if got := resumed.Info().StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("resumed stop detail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}

	repaired := readMeta(t, resumed.MetaPath())
	if repaired.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *repaired.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *repaired.StopReason, store.StopAgentCrashed)
	}
	if got := repaired.StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("meta.StopDetail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}
}

func TestResumeFallsBackToFreshStartWhenStoredACPSessionIsMissing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(missing ACP session) error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := h.driver.startCalls[2].ResumeSessionID; got != "" {
		t.Fatalf("fallback resume start ResumeSessionID = %q, want empty", got)
	}
	if got := resumed.Info().ACPSessionID; got == "" || got == originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want fresh ACP session id distinct from %q", got, originalACP)
	}
	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if meta := readMeta(t, session.MetaPath()); meta.State != string(StateActive) {
		t.Fatalf("meta state after fallback resume = %q, want %q", meta.State, StateActive)
	}
}

func TestResumeMissingACPStateFallbackPreservesRecoveredCrashClassification(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(missing ACP state after crash repair) error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := h.driver.startCalls[2].ResumeSessionID; got != "" {
		t.Fatalf("fallback resume start ResumeSessionID = %q, want empty", got)
	}
	if got := resumed.Info().StopReason; got != store.StopAgentCrashed {
		t.Fatalf("resumed StopReason = %q, want %q", got, store.StopAgentCrashed)
	}
	if got := resumed.Info().StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("resumed StopDetail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}
}

func TestResumeMissingACPStateFallbackLogsAtInfoLevel(t *testing.T) {
	t.Parallel()

	logs := newCaptureLogHandler()
	h := newHarness(t, WithLogger(slog.New(logs)))
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}

	record, ok := logs.FindByMessage("session.resume.context_replay_fallback")
	if !ok {
		t.Fatalf("missing context_replay_fallback log: %#v", logs.Records())
	}
	if got, want := record.Level, slog.LevelInfo; got != want {
		t.Fatalf("fallback log level = %v, want %v", got, want)
	}
	assertCapturedLogAttr(t, record, "session_id", session.ID)
	assertCapturedLogAttr(t, record, "agent_name", "coder")
	assertCapturedLogAttr(t, record, "provider", "claude")
	assertCapturedLogAttr(t, record, "phase", "resume")
	assertCapturedLogAttr(t, record, "fallback_reason", "load_session_resource_missing")
}

func TestResumeFailureRestoresStoppedMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	metaBefore := readMeta(t, session.MetaPath())
	h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
		return nil, errors.New("start failed")
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(generic failure) error = nil, want non-nil")
	}

	metaAfter := readMeta(t, session.MetaPath())
	if got := metaAfter.State; got != string(StateStopped) {
		t.Fatalf("meta state after failed resume = %q, want %q", got, StateStopped)
	}
	if got := derefString(metaAfter.ACPSessionID); got != originalACP {
		t.Fatalf("meta ACPSessionID after failed resume = %q, want %q", got, originalACP)
	}
	assertOptionalStopReasonEqual(t, metaAfter.StopReason, metaBefore.StopReason)
	if got := metaAfter.StopDetail; got != metaBefore.StopDetail {
		t.Fatalf("meta stop detail after failed resume = %q, want %q", got, metaBefore.StopDetail)
	}
}

func TestResumeReplayFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should inject the checkpoint summary before the persisted transcript", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		checkpoint := "<agh_checkpoint_summary>\n## Goal\nPreserve the cobalt decision.\n</agh_checkpoint_summary>"
		h.manager = newManagerWithHarness(
			t,
			h,
			WithPromptAssembler(&resumeContextPromptAssembler{checkpoint: checkpoint}),
		)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "local-only-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})
		events, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		got := h.driver.promptCalls[0].Message
		checkpointIndex := strings.Index(got, "<agh_checkpoint_summary>")
		replayIndex := strings.Index(got, resumeReplayOpenTag)
		if checkpointIndex < 0 || replayIndex < 0 || checkpointIndex >= replayIndex {
			t.Fatalf("resume prompt checkpoint/replay order invalid:\n%s", got)
		}
	})

	t.Run("Should stage a pruned replay when session load is unsupported", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "local-only-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		eventsBeforeResume := readStoredEvents(t, session)
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf(
					"%w: agent %q does not support session/load",
					acp.ErrAgentDoesNotSupportSession,
					opts.AgentName,
				)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(load unsupported) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		firstPrompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue after restart")
		if err != nil {
			t.Fatalf("Prompt(first resumed turn) error = %v", err)
		}
		collectEvents(t, firstPrompt)
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[0].Message, eventsBeforeResume)

		secondPrompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue again")
		if err != nil {
			t.Fatalf("Prompt(second resumed turn) error = %v", err)
		}
		collectEvents(t, secondPrompt)
		if strings.Contains(h.driver.promptCalls[1].Message, "<agh_context_replay>") {
			t.Fatalf("second resumed prompt contains replay block: %q", h.driver.promptCalls[1].Message)
		}
		assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 1)
	})

	t.Run("Should stage a pruned replay when the stored ACP session is stale", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "stale-session-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		eventsBeforeResume := readStoredEvents(t, session)
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf(
					"%w: load session %q for %q: %w",
					acp.ErrLoadSessionFailed,
					opts.ResumeSessionID,
					opts.AgentName,
					&acpsdk.RequestError{Code: -32002, Message: "Resource not found"},
				)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(stale ACP session) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		prompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue after stale load")
		if err != nil {
			t.Fatalf("Prompt(resumed) error = %v", err)
		}
		collectEvents(t, prompt)
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[0].Message, eventsBeforeResume)
		assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 1)
	})

	t.Run("Should not stage replay when session load succeeds", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "loaded-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(load succeeds) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		prompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue loaded session")
		if err != nil {
			t.Fatalf("Prompt(resumed) error = %v", err)
		}
		collectEvents(t, prompt)
		if strings.Contains(h.driver.promptCalls[0].Message, "<agh_context_replay>") {
			t.Fatalf("successful load prompt contains replay block: %q", h.driver.promptCalls[0].Message)
		}
		assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 0)
	})

	t.Run("Should retain replay until prompt delivery is fully prepared", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, session, "retry-context")
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		eventsBeforeResume := readStoredEvents(t, session)
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume(load unsupported) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return nil, errors.New("prompt dispatch rejected")
		}
		if _, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "first attempt"); err == nil ||
			!strings.Contains(err.Error(), "prompt dispatch rejected") {
			t.Fatalf("Prompt(rejected) error = %v, want dispatch rejection", err)
		}
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[0].Message, eventsBeforeResume)
		if replay := h.manager.pendingResumeReplay(resumed.ID); replay == "" {
			t.Fatal("pending resume replay = empty after rejected dispatch")
		}

		h.driver.promptHook = nil
		prepareErr := errors.New("delivery preparation failed")
		if _, err := h.manager.PromptWithOpts(testutil.Context(t), resumed.ID, PromptOpts{
			Message: "delivery attempt",
			PrepareDelivery: func(context.Context, PromptDelivery) error {
				return prepareErr
			},
		}); !errors.Is(err, prepareErr) {
			t.Fatalf("PromptWithOpts(delivery failure) error = %v, want %v", err, prepareErr)
		}
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[1].Message, eventsBeforeResume)
		if replay := h.manager.pendingResumeReplay(resumed.ID); replay == "" {
			t.Fatal("pending resume replay = empty after delivery preparation failure")
		}

		retry, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "retry accepted")
		if err != nil {
			t.Fatalf("Prompt(retry) error = %v", err)
		}
		collectEvents(t, retry)
		assertResumeReplayEqualsPrunedEvents(t, h.driver.promptCalls[2].Message, eventsBeforeResume)
		if replay := h.manager.pendingResumeReplay(resumed.ID); replay != "" {
			t.Fatalf("pending resume replay = %q, want consumed after accepted dispatch", replay)
		}
	})

	t.Run("Should isolate replay to the resumed session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		localSession := createSession(t, h)
		recordResumeReplayFixture(t, h.manager, localSession, "local-session-secret")
		if err := h.manager.Stop(testutil.Context(t), localSession.ID); err != nil {
			t.Fatalf("Stop(local) error = %v", err)
		}

		foreignRoot := filepath.Join(h.homePaths.HomeDir, "foreign-workspace")
		if err := os.MkdirAll(foreignRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(foreign workspace) error = %v", err)
		}
		foreignWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(primary workspace) error = %v", err)
		}
		foreignWorkspace.ID = "ws-foreign"
		foreignWorkspace.WorkspaceID = "ws-foreign"
		foreignWorkspace.RootDir = foreignRoot
		foreignWorkspace.Name = "foreign-workspace"
		h.resolver.upsert(&foreignWorkspace)

		foreignSession, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: "ws-foreign",
		})
		if err != nil {
			t.Fatalf("Create(foreign session) error = %v", err)
		}
		recordResumeReplayFixture(t, h.manager, foreignSession, "foreign-session-secret")
		if err := h.manager.Stop(testutil.Context(t), foreignSession.ID); err != nil {
			t.Fatalf("Stop(foreign) error = %v", err)
		}

		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}

		resumed, err := h.manager.Resume(testutil.Context(t), localSession.ID)
		if err != nil {
			t.Fatalf("Resume(local) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		prompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "continue isolated session")
		if err != nil {
			t.Fatalf("Prompt(resumed) error = %v", err)
		}
		collectEvents(t, prompt)
		replay := resumeReplayMessagesFromPrompt(t, h.driver.promptCalls[0].Message)
		encoded, err := json.Marshal(replay)
		if err != nil {
			t.Fatalf("json.Marshal(replay) error = %v", err)
		}
		if !strings.Contains(string(encoded), "local-session-secret") {
			t.Fatalf("replay = %s, want local session context", encoded)
		}
		if strings.Contains(string(encoded), "foreign-session-secret") {
			t.Fatalf("replay = %s, contains foreign session context", encoded)
		}
	})

	t.Run("Should release fallback resources when marker persistence fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopCallsBeforeResume := h.driver.stopCalls

		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: unsupported", acp.ErrAgentDoesNotSupportSession)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}
		openCalls := 0
		var fallbackRecorder *markerFailingRecorder
		h.manager = newManagerWithHarness(t, h, WithStore(func(
			ctx context.Context,
			sessionID string,
			path string,
		) (EventRecorder, error) {
			openCalls++
			recorder, err := sessiondb.OpenSessionDB(ctx, sessionID, path)
			if err != nil {
				return nil, err
			}
			if openCalls == 2 {
				fallbackRecorder = &markerFailingRecorder{
					EventRecorder: recorder,
					failErr:       errors.New("marker write failed"),
				}
				return fallbackRecorder, nil
			}
			return recorder, nil
		}))

		if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil ||
			!strings.Contains(err.Error(), "marker write failed") {
			t.Fatalf("Resume(marker failure) error = %v, want marker write failure", err)
		}
		if fallbackRecorder == nil {
			t.Fatal("fallback recorder = nil, want opened fallback recorder")
		}
		if got, want := fallbackRecorder.closeCalls, 1; got != want {
			t.Fatalf("fallback recorder close calls = %d, want %d", got, want)
		}
		if got, want := h.driver.stopCalls, stopCallsBeforeResume+1; got != want {
			t.Fatalf("driver stop calls = %d, want %d", got, want)
		}
		if _, ok := h.manager.Get(session.ID); ok {
			t.Fatalf("Get(%q) found failed fallback session", session.ID)
		}
		if replay := h.manager.pendingResumeReplay(session.ID); replay != "" {
			t.Fatalf("pending resume replay = %q, want empty after failed start", replay)
		}
		if meta := readMeta(t, session.MetaPath()); meta.State != string(StateStopped) {
			t.Fatalf("meta state after marker failure = %q, want %q", meta.State, StateStopped)
		}
	})
}

func TestActivateAndWatchUpdatesStateAndStartsWatcher(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	sessionDir := filepath.Join(h.homePaths.SessionsDir, "sess-helper")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}

	dbPath := store.SessionDBFile(sessionDir)
	recorder, err := sessiondb.OpenSessionDB(testutil.Context(t), "sess-helper", dbPath)
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}

	session := &Session{
		ID:                   "sess-helper",
		Name:                 "helper",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		Workspace:            h.workspace,
		NetworkParticipation: testLocalParticipation(),
		Type:                 SessionTypeUser,
		State:                StateStarting,
		CreatedAt:            time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		sessionDir:           sessionDir,
		metaPath:             store.SessionMetaFile(sessionDir),
		dbPath:               dbPath,
		recorder:             recorder,
	}

	if err := h.manager.reserveStart(testutil.Context(t), session.ID, h.workspaceID); err != nil {
		t.Fatalf("reserveStart() error = %v", err)
	}

	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName: "coder",
		Command:   "fake-agent",
		Cwd:       h.workspace,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := h.manager.activateAndWatch(
		testutil.Context(t),
		session,
		proc,
		false,
		aghconfig.ResolvedAgent{Name: "coder"},
		[]NetworkPeerCapability{},
		hookspkg.HookSessionPostCreate,
		false,
	); err != nil {
		t.Fatalf("activateAndWatch() error = %v", err)
	}

	if got := session.Info().State; got != StateActive {
		t.Fatalf("session state = %q, want %q", got, StateActive)
	}
	if got := session.Info().ACPSessionID; got != proc.SessionID {
		t.Fatalf("session ACPSessionID = %q, want %q", got, proc.SessionID)
	}
	if got, ok := h.manager.Get(session.ID); !ok || got != session {
		t.Fatalf("Get(%q) = (%v, %v), want active session", session.ID, got, ok)
	}
	if got := h.notifier.createdCount(); got != 1 {
		t.Fatalf("created notifications = %d, want 1", got)
	}
	if meta := readMeta(t, session.MetaPath()); meta.State != string(StateActive) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateActive)
	}

	h.driver.lastProcess().exit()
	waitForCondition(t, "session watcher finalization", func() bool {
		_, ok := h.manager.Get(session.ID)
		return !ok && h.notifier.stoppedCount() == 1
	})
}

func TestResumeFailsWhenWorkspaceCannotBeResolved(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.resolver.resolveErr = workspacepkg.ErrWorkspaceNotFound
	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(missing workspace) error = nil, want non-nil")
	} else if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
		t.Fatalf("Resume(missing workspace) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestActivateAndWatchRollsBackOnMetaWriteFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sessionDir := filepath.Join(t.TempDir(), "session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}
	blockingPath := filepath.Join(sessionDir, "blocked-parent")
	if err := os.WriteFile(blockingPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile(blockingPath) error = %v", err)
	}

	recorder, err := h.manager.openStore(testutil.Context(t), "sess-rollback", filepath.Join(sessionDir, "events.db"))
	if err != nil {
		t.Fatalf("openStore() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = recorder.Close(closeCtx)
	})

	session := &Session{
		ID:          "sess-rollback",
		AgentName:   "coder",
		WorkspaceID: h.workspaceID,
		Workspace:   h.workspace,
		Type:        SessionTypeUser,
		State:       StateStarting,
		CreatedAt:   time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		sessionDir:  sessionDir,
		metaPath:    filepath.Join(blockingPath, "session.json"),
		dbPath:      filepath.Join(sessionDir, "events.db"),
		recorder:    recorder,
	}

	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName: "coder",
		Command:   "fake-agent",
		Cwd:       h.workspace,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := h.manager.reserveStart(testutil.Context(t), session.ID, session.WorkspaceID); err != nil {
		t.Fatalf("reserveStart() error = %v", err)
	}

	if err := h.manager.activateAndWatch(
		testutil.Context(t),
		session,
		proc,
		false,
		aghconfig.ResolvedAgent{Name: "coder"},
		[]NetworkPeerCapability{},
		hookspkg.HookSessionPostCreate,
		false,
	); err == nil {
		t.Fatal("activateAndWatch() error = nil, want non-nil")
	}
	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) = active session, want rollback", session.ID)
	}
	if got := session.Info().State; got != StateStarting {
		t.Fatalf("session state after rollback = %q, want %q", got, StateStarting)
	}
	if got := session.processHandle(); got != nil {
		t.Fatalf("session process after rollback = %#v, want nil", got)
	}
	if h.driver.stopCalls != 1 {
		t.Fatalf("driver stop calls = %d, want 1", h.driver.stopCalls)
	}
}

func TestCleanupFailedStartRemovesSessionDir(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	recorder := &fakeEventRecorder{}
	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{AgentName: "coder"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	sessionDir := filepath.Join(t.TempDir(), "failed-session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}

	if err := h.manager.cleanupFailedStart(sessionDir, recorder, proc); err != nil {
		t.Fatalf("cleanupFailedStart(with dir) error = %v", err)
	}
	if h.driver.stopCalls != 1 {
		t.Fatalf("driver stop calls = %d, want 1", h.driver.stopCalls)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder close calls = %d, want 1", recorder.closeCalls)
	}
	if _, err := os.Stat(sessionDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(sessionDir) error = %v, want os.ErrNotExist", err)
	}
}

func TestPumpPromptReturnsWhenContextIsCanceledWhileWaitingForSource(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	source := make(chan acp.AgentEvent)
	out := make(chan acp.AgentEvent)
	ctx, cancel := context.WithCancel(testutil.Context(t))

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.manager.pumpPrompt(
			ctx,
			ctx,
			nil,
			newPromptTurnDispatchState(nil, "turn-1", TurnSourceUser, ""),
			source,
			nil,
			out,
			nil,
			nil,
		)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pumpPrompt() did not return after context cancellation")
	}

	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("pumpPrompt() output channel remained open after cancellation")
		}
	default:
		t.Fatal("pumpPrompt() did not close output channel")
	}
}

func TestPumpPromptDrainsRuntimeEventsAfterTurnDone(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	source := make(chan acp.AgentEvent)
	runtimeEvents := make(chan acp.AgentEvent, 1)
	out := make(chan acp.AgentEvent)
	done := make(chan struct{})
	ctx := testutil.Context(t)

	go func() {
		defer close(done)
		h.manager.pumpPrompt(
			ctx,
			ctx,
			session,
			newPromptTurnDispatchState(session, "turn-runtime-drain", TurnSourceUser, ""),
			source,
			runtimeEvents,
			out,
			nil,
			nil,
		)
	}()

	source <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "turn-runtime-drain"}
	first := receivePromptEvent(t, out)
	if first.Type != acp.EventTypeDone {
		t.Fatalf("first event type = %q, want %q", first.Type, acp.EventTypeDone)
	}

	runtimeEvents <- acp.AgentEvent{Type: acp.EventTypeRuntimeProgress, TurnID: "turn-runtime-drain"}
	close(runtimeEvents)
	second := receivePromptEvent(t, out)
	if second.Type != acp.EventTypeRuntimeProgress {
		t.Fatalf("second event type = %q, want %q", second.Type, acp.EventTypeRuntimeProgress)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pumpPrompt() did not return after draining runtime events")
	}
}

func TestNextPromptPumpEventPrioritizesReadyRuntimeEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer a ready runtime event over a ready source event", func(t *testing.T) {
		t.Parallel()

		source := make(chan acp.AgentEvent, 1)
		runtimeEvents := make(chan acp.AgentEvent, 1)
		loop := &promptPumpLoopState{source: source, runtime: runtimeEvents}

		source <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: "turn-runtime-priority"}
		runtimeEvents <- acp.AgentEvent{
			Type:   acp.EventTypeRuntimeWarning,
			TurnID: "turn-runtime-priority",
		}

		event, runtimeEvent, ok := nextPromptPumpEvent(testutil.Context(t), loop)
		if !ok {
			t.Fatal("nextPromptPumpEvent() ok = false, want true")
		}
		if !runtimeEvent {
			t.Fatal("nextPromptPumpEvent() runtimeEvent = false, want true")
		}
		if got, want := event.Type, acp.EventTypeRuntimeWarning; got != want {
			t.Fatalf("nextPromptPumpEvent() type = %q, want %q", got, want)
		}

		select {
		case remaining := <-source:
			if got, want := remaining.Type, acp.EventTypeError; got != want {
				t.Fatalf("remaining source event type = %q, want %q", got, want)
			}
		default:
			t.Fatal("source event drained unexpectedly")
		}
	})

	t.Run("Should probe source closure after a prioritized runtime event", func(t *testing.T) {
		t.Parallel()

		source := make(chan acp.AgentEvent)
		runtimeEvents := make(chan acp.AgentEvent, 2)
		loop := &promptPumpLoopState{source: source, runtime: runtimeEvents}

		runtimeEvents <- acp.AgentEvent{
			Type:   acp.EventTypeRuntimeProgress,
			TurnID: "turn-runtime-priority",
		}
		runtimeEvents <- acp.AgentEvent{
			Type:   acp.EventTypeRuntimeProgress,
			TurnID: "turn-runtime-priority",
		}

		event, runtimeEvent, ok := nextPromptPumpEvent(testutil.Context(t), loop)
		if !ok {
			t.Fatal("nextPromptPumpEvent() ok = false, want true")
		}
		if !runtimeEvent {
			t.Fatal("nextPromptPumpEvent() runtimeEvent = false, want true")
		}
		if got, want := event.Type, acp.EventTypeRuntimeProgress; got != want {
			t.Fatalf("nextPromptPumpEvent() type = %q, want %q", got, want)
		}

		close(source)
		event, runtimeEvent, ok = nextPromptPumpEvent(testutil.Context(t), loop)
		if !ok {
			t.Fatal("nextPromptPumpEvent() ok = false, want buffered runtime event after source closure")
		}
		if !runtimeEvent {
			t.Fatal("nextPromptPumpEvent() runtimeEvent = false, want true")
		}
		if got, want := event.Type, acp.EventTypeRuntimeProgress; got != want {
			t.Fatalf("nextPromptPumpEvent() type = %q, want %q", got, want)
		}
		if loop.source != nil {
			t.Fatal("loop source = non-nil, want closed source observed before draining more runtime events")
		}
	})
}

func receivePromptEvent(t *testing.T, events <-chan acp.AgentEvent) acp.AgentEvent {
	t.Helper()

	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("prompt output channel closed before expected event")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for prompt event")
		return acp.AgentEvent{}
	}
}

func TestCleanupFailedStartWithoutSessionDirSkipsRemoval(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	recorder := &fakeEventRecorder{}
	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{AgentName: "coder"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := h.manager.cleanupFailedStart("", recorder, proc); err != nil {
		t.Fatalf("cleanupFailedStart(without dir) error = %v", err)
	}
	if h.driver.stopCalls != 1 {
		t.Fatalf("driver stop calls = %d, want 1", h.driver.stopCalls)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder close calls = %d, want 1", recorder.closeCalls)
	}
}

func TestPromptStreamsToRecorderAndNotifier(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
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
		t.Fatalf("Prompt() events = %d, want 2", len(events))
	}
	if events[0].Type != acp.EventTypeAgentMessage {
		t.Fatalf("first event type = %q, want %q", events[0].Type, acp.EventTypeAgentMessage)
	}
	if events[1].Type != acp.EventTypeDone {
		t.Fatalf("second event type = %q, want %q", events[1].Type, acp.EventTypeDone)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("stored events = %d, want 3", len(stored))
	}
	if got := stored[0].Type; got != acp.EventTypeUserMessage {
		t.Fatalf("first stored event type = %q, want %q", got, acp.EventTypeUserMessage)
	}
	if got := h.notifier.eventCount(session.ID); got != 3 {
		t.Fatalf("notifier events = %d, want 3", got)
	}
}

func TestPromptPersistenceFailureStopsSessionBeforeLiveDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a failed single event out of live reload and turn completion", func(t *testing.T) {
		t.Parallel()

		recordErr := errors.New("projection failed token=super-secret")
		recorder := &failingSinglePromptRecorder{failErr: recordErr}
		h := newHarness(t, WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return recorder, nil
		}))
		turnEnds := 0
		h.manager.SetTurnEndNotifier(func(string) { turnEnds++ })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			go func() {
				defer close(events)
				events <- acp.AgentEvent{
					Type:       acp.EventTypeToolCall,
					SessionID:  proc.handle.SessionID,
					TurnID:     req.TurnID,
					ToolCallID: "call-failed",
					Title:      "Read",
				}
			}()
			return events, nil
		}
		session := createSession(t, h)
		t.Cleanup(func() {
			if _, active := h.manager.Get(session.ID); active {
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
					!errors.Is(err, ErrSessionNotFound) {
					t.Errorf("Stop() cleanup error = %v", err)
				}
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if delivered := collectEvents(t, eventsCh); len(delivered) != 0 {
			t.Fatalf("Prompt() delivered = %#v, want no unpersisted event", delivered)
		}
		if got := countAgentEvents(h.notifier.eventsForSession(session.ID), acp.EventTypeToolCall); got != 0 {
			t.Fatalf("tool call notifier events = %d, want zero", got)
		}
		if turnEnds != 0 {
			t.Fatalf("turn end notifications = %d, want zero", turnEnds)
		}
		waitForCondition(t, "session stopped after single persistence failure", func() bool {
			_, active := h.manager.Get(session.ID)
			return !active
		})
		meta := readMeta(t, session.MetaPath())
		if meta.Failure == nil || meta.Failure.Kind != store.FailureTransport {
			t.Fatalf("stopped session failure = %#v, want transport failure", meta.Failure)
		}
		if strings.Contains(meta.Failure.Summary, "super-secret") {
			t.Fatalf("stopped session failure leaked secret: %q", meta.Failure.Summary)
		}
		reloaded, queryErr := recorder.Query(testutil.Context(t), store.EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query(reload) error = %v", queryErr)
		}
		if got := countEventType(reloaded, acp.EventTypeToolCall); got != 0 {
			t.Fatalf("reloaded tool_call rows = %d, want zero", got)
		}
	})
}

func TestPromptDeadlineDeliversRuntimeWarningBeforeError(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithSessionSupervision(aghconfig.SessionSupervisionConfig{
		ActivityHeartbeatInterval: time.Hour,
		ProgressNotifyInterval:    0,
		InactivityWarningAfter:    0,
		InactivityTimeout:         0,
		TimeoutCancelGrace:        2 * time.Second,
		PromptDeadline:            20 * time.Millisecond,
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	source := make(chan acp.AgentEvent, 1)
	var promptTurnID string
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		promptTurnID = req.TurnID
		return source, nil
	}
	h.driver.cancelHook = func(proc *fakeProcess) error {
		source <- acp.AgentEvent{
			Type:      acp.EventTypeError,
			SessionID: proc.handle.SessionID,
			TurnID:    promptTurnID,
			Timestamp: time.Now().UTC(),
			Error:     `{"code":-32603,"message":"Internal error","data":{"error":"context deadline exceeded"}}`,
		}
		close(source)
		return nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "long running")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if got, want := len(events), 2; got != want {
		t.Fatalf("Prompt() events = %d, want %d", got, want)
	}
	if got, want := events[0].Type, acp.EventTypeRuntimeWarning; got != want {
		t.Fatalf("Prompt() first event type = %q, want %q", got, want)
	}
	if got, want := events[1].Type, acp.EventTypeError; got != want {
		t.Fatalf("Prompt() second event type = %q, want %q", got, want)
	}
	if events[0].Runtime == nil || events[0].Runtime.DeadlineAt == nil {
		t.Fatalf("Prompt() first runtime = %#v, want deadline payload", events[0].Runtime)
	}
	if got := h.driver.cancelCalls; got != 1 {
		t.Fatalf("driver cancel calls = %d, want 1", got)
	}
	if got := h.driver.stopCalls; got != 1 {
		t.Fatalf("driver stop calls = %d, want 1", got)
	}
}

func TestPromptFatalProcessFailureStopsSessionAndAllowsFreshResumeFallback(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID
	t.Cleanup(func() {
		if _, ok := h.manager.Get(session.ID); ok {
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		}
	})

	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent, 1)
		go func() {
			defer close(events)
			events <- acp.AgentEvent{
				Type:      acp.EventTypeError,
				SessionID: originalACP,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Error:     `{"code":-32603,"message":"Internal error: The Claude Agent process exited unexpectedly. Please start a new session."}`,
				Failure: &store.SessionFailure{
					Kind:    store.FailureProcess,
					Summary: `{"code":-32603,"message":"Internal error: The Claude Agent process exited unexpectedly. Please start a new session."}`,
				},
			}
		}()
		return events, nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if got, want := len(events), 1; got != want {
		t.Fatalf("Prompt() events = %d, want %d", got, want)
	}
	if got, want := events[0].Type, acp.EventTypeError; got != want {
		t.Fatalf("Prompt() first event type = %q, want %q", got, want)
	}
	if events[0].Failure == nil || events[0].Failure.Kind != store.FailureProcess {
		t.Fatalf("Prompt() failure = %#v, want process_exit", events[0].Failure)
	}

	waitForCondition(t, "session stopped after fatal prompt failure", func() bool {
		_, ok := h.manager.Get(session.ID)
		return !ok && h.notifier.stoppedCount() == 1
	})

	if got := h.driver.stopCalls; got != 1 {
		t.Fatalf("driver stop calls = %d, want 1", got)
	}

	meta := readMeta(t, session.MetaPath())
	if got := meta.State; got != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", got, StateStopped)
	}
	if meta.StopReason == nil || *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %#v, want %q", meta.StopReason, store.StopAgentCrashed)
	}
	if meta.Failure == nil || meta.Failure.Kind != store.FailureProcess {
		t.Fatalf("meta.Failure = %#v, want process_exit", meta.Failure)
	}

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := h.driver.startCalls[2].ResumeSessionID; got != "" {
		t.Fatalf("fallback resume start ResumeSessionID = %q, want empty", got)
	}
	if got := resumed.Info().ACPSessionID; got == "" || got == originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want fresh ACP session id distinct from %q", got, originalACP)
	}
}

func TestPromptGenericFailureKeepsSessionActive(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if _, ok := h.manager.Get(session.ID); ok {
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		}
	})

	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent, 1)
		go func() {
			defer close(events)
			events <- acp.AgentEvent{
				Type:      acp.EventTypeError,
				SessionID: session.Info().ACPSessionID,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Error:     `{"code":-32603,"message":"Internal error","data":{"details":"Tool invocation failed"}}`,
				Failure: &store.SessionFailure{
					Kind:    store.FailurePrompt,
					Summary: `{"code":-32603,"message":"Internal error","data":{"details":"Tool invocation failed"}}`,
				},
			}
		}()
		return events, nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if got, want := len(events), 1; got != want {
		t.Fatalf("Prompt() events = %d, want %d", got, want)
	}

	if _, ok := h.manager.Get(session.ID); !ok {
		t.Fatal("session removed after generic prompt failure, want still active")
	}
	if got := h.driver.stopCalls; got != 0 {
		t.Fatalf("driver stop calls = %d, want 0", got)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(active) error = %v", err)
	}
	if resumed != session {
		t.Fatalf("Resume(active) returned %p, want %p", resumed, session)
	}
}

func TestCancelPrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should cancel driver prompt for an active prompting session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		})

		promptEvents := make(chan acp.AgentEvent)
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return promptEvents, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		waitForCondition(t, "session prompting", func() bool {
			return session.IsPrompting()
		})

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
		if got := len(h.driver.interruptScopes); got != 1 {
			t.Fatalf("driver interrupt calls = %d, want 1", got)
		}
		if got := h.driver.interruptScopes[0]; got.SessionID != session.ID || got.TurnID == "" {
			t.Fatalf("driver interrupt scope = %#v, want session and turn", got)
		}

		close(promptEvents)
		_ = collectEvents(t, eventsCh)
	})

	t.Run("Should cancel prompt setup before driver prompt is registered", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			session.clearCurrentPromptCancel()
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		promptCtx, cancelPrompt := context.WithCancel(testutil.Context(t))
		session.setCurrentTurnID("turn-setup")
		session.setCurrentTurnSource(TurnSourceUser)
		session.setCurrentPromptCancel(cancelPrompt)

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		select {
		case <-promptCtx.Done():
		default:
			t.Fatal("prompt setup context is still active after CancelPrompt()")
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
		if got := len(h.driver.interruptScopes); got != 1 {
			t.Fatalf("driver interrupt calls = %d, want 1", got)
		}
	})

	t.Run("Should no-op when a prompting session loses its process handle", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		})

		session.setCurrentTurnSource(TurnSourceUser)
		session.clearProcess(time.Now().UTC())

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should ignore cancel errors once the process is already done", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		})

		session.setCurrentTurnSource(TurnSourceUser)
		h.driver.cancelHook = func(_ *fakeProcess) error {
			return errors.New("test: cancel after process exit")
		}
		h.driver.lastProcess().exit()

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
	})

	t.Run("Should no-op for an active session without a prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		})

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should no-op for a known stopped session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should return ErrSessionNotFound for an unknown session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		err := h.manager.CancelPrompt(testutil.Context(t), "missing")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("CancelPrompt(missing) error = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestPromptPersistsUserMessageBeforeDriverPrompt(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	var storedBeforePrompt []store.SessionEvent
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if err != nil {
			return nil, err
		}
		storedBeforePrompt = events

		ch := make(chan acp.AgentEvent)
		close(ch)
		return ch, nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	for event := range eventsCh {
		_ = event
	}

	if len(storedBeforePrompt) != 1 {
		t.Fatalf("storedBeforePrompt = %d events, want 1", len(storedBeforePrompt))
	}
	if got := storedBeforePrompt[0].Type; got != acp.EventTypeUserMessage {
		t.Fatalf("storedBeforePrompt[0].Type = %q, want %q", got, acp.EventTypeUserMessage)
	}
	if !strings.Contains(storedBeforePrompt[0].Content, `"text":"remember me"`) {
		t.Fatalf("stored user_message content = %s", storedBeforePrompt[0].Content)
	}
}

func TestApplyAutomaticSessionTitleOwnsGeneratedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should include the ellipsis inside the eight-word and sixty-four-rune bounds", func(t *testing.T) {
		t.Parallel()

		got := normalizeAutomaticSessionTitle(
			"aaaaaaa bbbbbbb ccccccc ddddddd eeeeeee fffffff ggggggg hhhhhhhh ninth",
		)
		if words := strings.Fields(got); len(words) != automaticSessionTitleMaxWords {
			t.Fatalf("normalized title words = %d, want %d; title=%q", len(words), automaticSessionTitleMaxWords, got)
		}
		if runes := utf8.RuneCountInString(got); runes != automaticSessionTitleMaxRunes {
			t.Fatalf("normalized title runes = %d, want %d; title=%q", runes, automaticSessionTitleMaxRunes, got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Fatalf("normalized title = %q, want ellipsis", got)
		}
	})

	t.Run("Should persist the first generated title as durable session identity", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})
		catalogEvents, cancelCatalogEvents, err := h.manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancelCatalogEvents()

		const wantTitle = "Checkout webhook retries"
		applied, err := h.manager.ApplyAutomaticSessionTitle(testutil.Context(t), session.ID, wantTitle)
		if err != nil {
			t.Fatalf("ApplyAutomaticSessionTitle() error = %v", err)
		}
		if !applied {
			t.Fatal("ApplyAutomaticSessionTitle() applied = false, want true")
		}
		reapplied, err := h.manager.ApplyAutomaticSessionTitle(
			testutil.Context(t),
			session.ID,
			"Later title must lose",
		)
		if err != nil {
			t.Fatalf("ApplyAutomaticSessionTitle(second) error = %v", err)
		}
		if reapplied {
			t.Fatal("ApplyAutomaticSessionTitle(second) applied = true, want false")
		}
		if got := session.Info().Name; got != wantTitle {
			t.Fatalf("session title = %q, want %q", got, wantTitle)
		}
		if got := readMeta(t, session.MetaPath()).Name; got != wantTitle {
			t.Fatalf("persisted session title = %q, want %q", got, wantTitle)
		}
		catalogSessions, err := catalog.ListSessions(testutil.Context(t), store.SessionListQuery{
			WorkspaceID: h.workspaceID,
			SessionType: string(SessionTypeUser),
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(catalogSessions) != 1 || catalogSessions[0].Name != wantTitle {
			t.Fatalf("catalog sessions = %#v, want one session titled %q", catalogSessions, wantTitle)
		}
		select {
		case event := <-catalogEvents:
			if event.Kind != CatalogEventUpserted || event.WorkspaceID != h.workspaceID ||
				event.SessionID != session.ID {
				t.Fatalf("catalog title event = %#v, want workspace-scoped upsert", event)
			}
		default:
			t.Fatal("catalog title event was not published")
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		if got := resumed.Info().Name; got != wantTitle {
			t.Fatalf("resumed session title = %q, want %q", got, wantTitle)
		}
	})

	t.Run("Should preserve explicit and internal session identities", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			sessionName string
			sessionType Type
		}{
			{
				name:        "Should preserve an explicit user title",
				sessionName: "Release review",
				sessionType: SessionTypeUser,
			},
			{name: "Should leave a system session unnamed", sessionType: SessionTypeSystem},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				h := newHarness(t)
				session, err := h.manager.Create(testutil.Context(t), CreateOpts{
					AgentName: "coder",
					Name:      test.sessionName,
					Workspace: h.workspaceID,
					Type:      test.sessionType,
				})
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				t.Cleanup(func() {
					if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil &&
						!errors.Is(stopErr, ErrSessionNotFound) {
						t.Errorf("Stop() error = %v", stopErr)
					}
				})

				applied, err := h.manager.ApplyAutomaticSessionTitle(
					testutil.Context(t),
					session.ID,
					"Ignore internal bookkeeping",
				)
				if err != nil {
					t.Fatalf("ApplyAutomaticSessionTitle() error = %v", err)
				}
				if applied {
					t.Fatal("ApplyAutomaticSessionTitle() applied = true, want explicit/internal identity preserved")
				}

				if got := session.Info().Name; got != test.sessionName {
					t.Fatalf("session title = %q, want preserved %q", got, test.sessionName)
				}
			})
		}
	})
}

func TestPromptRejectsConcurrentUserPromptWithoutPersistingSecondInput(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
	})

	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent)
		go func() {
			defer close(events)
			if req.TurnID != "turn-1" {
				return
			}

			close(firstPromptEntered)
			<-releaseFirstPrompt

			ts := time.Now().UTC()
			events <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				TurnID:    req.TurnID,
				Timestamp: ts,
				Text:      "first prompt complete",
			}
			events <- acp.AgentEvent{
				Type:             acp.EventTypeDone,
				TurnID:           req.TurnID,
				Timestamp:        ts,
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}
		}()
		return events, nil
	}

	firstEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "first prompt")
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	<-firstPromptEntered

	_, err = h.manager.Prompt(testutil.Context(t), session.ID, "second prompt")
	if !errors.Is(err, ErrPromptInProgress) {
		t.Fatalf("Prompt(second) error = %v, want ErrPromptInProgress", err)
	}
	if got, want := len(h.driver.promptCalls), 1; got != want {
		t.Fatalf("len(promptCalls) while first prompt active = %d, want %d", got, want)
	}

	storedWhileBusy, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(while busy) error = %v", err)
	}
	if got, want := countEventType(storedWhileBusy, acp.EventTypeUserMessage), 1; got != want {
		t.Fatalf("countEventType(user_message) while busy = %d, want %d", got, want)
	}

	releaseOnce.Do(func() {
		close(releaseFirstPrompt)
	})
	_ = collectEvents(t, firstEvents)

	storedAfterRelease, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(after release) error = %v", err)
	}
	if got, want := countEventType(storedAfterRelease, acp.EventTypeUserMessage), 1; got != want {
		t.Fatalf("countEventType(user_message) after release = %d, want %d", got, want)
	}

	thirdEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "third prompt")
	if err != nil {
		t.Fatalf("Prompt(third) error = %v", err)
	}
	_ = collectEvents(t, thirdEvents)

	if got, want := len(h.driver.promptCalls), 2; got != want {
		t.Fatalf("len(promptCalls) after recovery = %d, want %d", got, want)
	}
}

func TestPromptAugmenterPreservesStoredUserMessageAndAugmentsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		message string,
	) (string, error) {
		return "MEMORY RECALL\n\n" + message, nil
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("stored events = 0, want at least one event")
	}
	if got := h.driver.promptCalls[0].Message; got != "MEMORY RECALL\n\nremember me" {
		t.Fatalf("driver prompt message = %q, want augmented content", got)
	}
	if !strings.Contains(stored[0].Content, `"text":"remember me"`) {
		t.Fatalf("stored user_message content = %s, want original message", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "MEMORY RECALL") {
		t.Fatalf("stored user_message content = %s, want no augmentation block", stored[0].Content)
	}
}

func TestPromptNetworkAugmenterPreservesStoredUserMessageAndAugmentsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		message string,
	) (string, error) {
		return message + "\n\nNETWORK AUGMENT", nil
	}))
	session := createLiveNetworkSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.PromptNetwork(
		testutil.Context(t),
		session.ID,
		"network message",
		acp.PromptNetworkMeta{
			MessageID: "msg-1",
			Kind:      "direct",
			Channel:   "builders",
			From:      "ops.peer",
		},
	)
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("stored events = 0, want at least one event")
	}
	if got := h.driver.promptCalls[0].Message; got != "network message\n\nNETWORK AUGMENT" {
		t.Fatalf("driver prompt message = %q, want augmented network content", got)
	}
	if got := h.driver.promptCalls[0].Meta.TurnSource; got != acp.PromptTurnSourceNetwork {
		t.Fatalf("driver turn source = %q, want %q", got, acp.PromptTurnSourceNetwork)
	}
	if !strings.Contains(stored[0].Content, `"text":"network message"`) {
		t.Fatalf("stored user_message content = %s, want original network message", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "NETWORK AUGMENT") {
		t.Fatalf("stored user_message content = %s, want no augmentation block", stored[0].Content)
	}
}

func TestPromptAugmenterPropagatesFailureAndSkipsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		_ string,
	) (string, error) {
		return "", errors.New("boom")
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me"); err == nil {
		t.Fatal("Prompt() error = nil, want augmentation failure")
	} else if !strings.Contains(err.Error(), "augment prompt input") {
		t.Fatalf("Prompt() error = %v, want augmentation error context", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(driver.promptCalls) = %d, want 0 after augmentation failure", got)
	}
}

func TestPromptAugmenterPropagatesCancellationAndSkipsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		_ string,
	) (string, error) {
		return "", context.Canceled
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Prompt() error = %v, want context.Canceled", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(driver.promptCalls) = %d, want 0 after canceled augmentation", got)
	}
}

func TestPromptWithOptsTracksTurnSourceAndClearsAfterPrompt(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createLiveNetworkSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	seenSources := make([]TurnSource, 0, 2)
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		seenSources = append(seenSources, session.CurrentTurnSource())

		ch := make(chan acp.AgentEvent)
		close(ch)
		return ch, nil
	}

	firstEvents, err := h.manager.PromptWithOpts(testutil.Context(t), session.ID, PromptOpts{
		Message: "user prompt",
	})
	if err != nil {
		t.Fatalf("PromptWithOpts(user) error = %v", err)
	}
	_ = collectEvents(t, firstEvents)

	secondEvents, err := h.manager.PromptNetwork(testutil.Context(t), session.ID, "network prompt")
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	_ = collectEvents(t, secondEvents)

	if got, want := len(h.driver.promptCalls), 2; got != want {
		t.Fatalf("len(promptCalls) = %d, want %d", got, want)
	}
	if got, want := h.driver.promptCalls[0].Meta.TurnSource, acp.PromptTurnSourceUser; got != want {
		t.Fatalf("promptCalls[0].Meta.TurnSource = %q, want %q", got, want)
	}
	networkMeta := acp.PromptNetworkMeta{
		MessageID: "msg-1",
		Kind:      "direct",
		Channel:   "builders",
		From:      "ops.peer",
	}
	thirdEvents, err := h.manager.PromptNetwork(testutil.Context(t), session.ID, "network prompt", networkMeta)
	if err != nil {
		t.Fatalf("PromptNetwork(with meta) error = %v", err)
	}
	_ = collectEvents(t, thirdEvents)
	if got, want := h.driver.promptCalls[2].Meta.TurnSource, acp.PromptTurnSourceNetwork; got != want {
		t.Fatalf("promptCalls[2].Meta.TurnSource = %q, want %q", got, want)
	}
	if h.driver.promptCalls[2].Meta.Network == nil {
		t.Fatal("promptCalls[2].Meta.Network = nil, want populated metadata")
	}
	if got, want := h.driver.promptCalls[2].Meta.Network.MessageID, networkMeta.MessageID; got != want {
		t.Fatalf("promptCalls[2].Meta.Network.MessageID = %q, want %q", got, want)
	}
	if !slices.Equal(seenSources, []TurnSource{TurnSourceUser, TurnSourceNetwork, TurnSourceNetwork}) {
		t.Fatalf(
			"seen turn sources = %#v, want %#v",
			seenSources,
			[]TurnSource{TurnSourceUser, TurnSourceNetwork, TurnSourceNetwork},
		)
	}
	if got := session.CurrentTurnSource(); got != "" {
		t.Fatalf("CurrentTurnSource() after prompts = %q, want empty", got)
	}
}

func TestPromptNetworkRejectsMultipleMetadataValues(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	_, err := h.manager.PromptNetwork(
		testutil.Context(t),
		session.ID,
		"network prompt",
		acp.PromptNetworkMeta{MessageID: "msg-1", Kind: "direct"},
		acp.PromptNetworkMeta{MessageID: "msg-2", Kind: "direct"},
	)
	if err == nil {
		t.Fatal("PromptNetwork(multiple meta) error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "at most one metadata value") {
		t.Fatalf("PromptNetwork(multiple meta) error = %v, want multiple-metadata validation", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(promptCalls) = %d, want 0 when PromptNetwork validation fails", got)
	}
}

func TestPromptWithOptsRejectsSyntheticTurnSourceUntilDedicatedPathExists(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	_, err := h.manager.PromptWithOpts(testutil.Context(t), session.ID, PromptOpts{
		Message:    "synthetic prompt",
		TurnSource: TurnSourceSynthetic,
	})
	if err == nil {
		t.Fatal("PromptWithOpts(synthetic) error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "dedicated synthetic submission path") {
		t.Fatalf("PromptWithOpts(synthetic) error = %v, want dedicated-path validation", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(promptCalls) = %d, want 0 when synthetic validation fails", got)
	}
}

func TestPromptSyntheticPersistsDedicatedEventAndMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "system-session",
		Workspace: h.workspaceID,
		Type:      SessionTypeSystem,
	})
	if err != nil {
		t.Fatalf("Create(system) error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	eventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskID:               "task-1",
			TaskRunID:            "run-1",
			ClaimTokenHash:       "sha256:claim-1",
			CoordinatorSessionID: "sess-coordinator-1",
			Reason:               "task_run_completed",
			Summary:              "background work finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	if got := len(h.driver.promptCalls); got != 1 {
		t.Fatalf("len(promptCalls) = %d, want 1", got)
	}
	if got := h.driver.promptCalls[0].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
		t.Fatalf("promptCalls[0].Meta.TurnSource = %q, want %q", got, acp.PromptTurnSourceSynthetic)
	}
	if h.driver.promptCalls[0].Meta.Synthetic == nil {
		t.Fatal("promptCalls[0].Meta.Synthetic = nil, want metadata")
	}
	if got, want := h.driver.promptCalls[0].Meta.Synthetic.TaskRunID, "run-1"; got != want {
		t.Fatalf("promptCalls[0].Meta.Synthetic.TaskRunID = %q, want %q", got, want)
	}
	if got, want := h.driver.promptCalls[0].Meta.Synthetic.ClaimTokenHash, "sha256:claim-1"; got != want {
		t.Fatalf("promptCalls[0].Meta.Synthetic.ClaimTokenHash = %q, want %q", got, want)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("stored events = 0, want persisted synthetic event")
	}
	if got := stored[0].Type; got != acp.EventTypeSyntheticReentry {
		t.Fatalf("stored[0].Type = %q, want %q", got, acp.EventTypeSyntheticReentry)
	}
	if got := countEventType(stored, acp.EventTypeUserMessage); got != 0 {
		t.Fatalf("countEventType(user_message) = %d, want 0 for synthetic-only prompt", got)
	}

	payload := decodeStoredEventPayload(t, stored[0])
	if got, want := payload["type"], acp.EventTypeSyntheticReentry; got != want {
		t.Fatalf("stored synthetic payload type = %v, want %q", got, want)
	}
	if got, want := payload["text"], "synthetic wake-up"; got != want {
		t.Fatalf("stored synthetic payload text = %v, want %q", got, want)
	}
	syntheticPayload, ok := payload["synthetic"].(map[string]any)
	if !ok {
		t.Fatalf("stored synthetic payload metadata = %#v, want object", payload["synthetic"])
	}
	if got, want := syntheticPayload["task_run_id"], "run-1"; got != want {
		t.Fatalf("stored synthetic task_run_id = %v, want %q", got, want)
	}
	if got, want := syntheticPayload["reason"], "task_run_completed"; got != want {
		t.Fatalf("stored synthetic reason = %v, want %q", got, want)
	}
	if got, want := syntheticPayload["summary"], "background work finished"; got != want {
		t.Fatalf("stored synthetic summary = %v, want %q", got, want)
	}
	notified := h.notifier.eventsForSession(session.ID)
	if len(notified) == 0 {
		t.Fatal("notified events = 0, want synthetic observe notification")
	}
	if got, want := notified[0].SchedulerReason, "task_run_completed"; got != want {
		t.Fatalf("notified[0].SchedulerReason = %q, want %q", got, want)
	}
	if got, want := notified[0].ClaimTokenHash, "sha256:claim-1"; got != want {
		t.Fatalf("notified[0].ClaimTokenHash = %q, want %q", got, want)
	}
	if got, want := notified[0].CoordinatorSessionID, "sess-coordinator-1"; got != want {
		t.Fatalf("notified[0].CoordinatorSessionID = %q, want %q", got, want)
	}
}

func TestPromptSyntheticRejectsMissingWakeupMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	_, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
		},
	})
	if err == nil {
		t.Fatal("PromptSynthetic() error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "requires a reason") {
		t.Fatalf("PromptSynthetic() error = %v, want missing-reason validation", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(promptCalls) = %d, want 0 after validation failure", got)
	}
}

func TestPromptSyntheticHeartbeatWakeOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should reject busy synthetic prompt without queueing when requested", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.TurnID == "turn-1" {
				close(firstPromptEntered)
				events := make(chan acp.AgentEvent)
				go func() {
					<-releaseFirstPrompt
					close(events)
				}()
				return events, nil
			}
			events := make(chan acp.AgentEvent)
			close(events)
			return events, nil
		}

		userEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		<-firstPromptEntered

		_, err = h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "heartbeat wake",
			Metadata: acp.PromptSyntheticMeta{
				Reason:      "agent_heartbeat_wake",
				WakeEventID: "hwe-busy",
			},
			SkipIfBusy: true,
		})
		if !errors.Is(err, ErrPromptInProgress) {
			t.Fatalf("PromptSynthetic(skip busy) error = %v, want ErrPromptInProgress", err)
		}
		if got, want := len(h.driver.promptCalls), 1; got != want {
			t.Fatalf("len(promptCalls) = %d, want %d", got, want)
		}

		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
		_ = collectEvents(t, userEvents)
	})

	t.Run("Should preserve heartbeat metadata and supplied prompt turn id", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		eventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "heartbeat wake",
			TurnID:  "turn-heartbeat",
			Metadata: acp.PromptSyntheticMeta{
				Reason:           "agent_heartbeat_wake",
				Summary:          "policy summary",
				WakeEventID:      "hwe-heartbeat",
				PolicySnapshotID: "hb-heartbeat",
				PolicyDigest:     "sha256:policy",
				ConfigDigest:     "sha256:config",
			},
			SkipIfBusy: true,
		})
		if err != nil {
			t.Fatalf("PromptSynthetic(heartbeat) error = %v", err)
		}
		_ = collectEvents(t, eventsCh)

		if got, want := h.driver.promptCalls[0].TurnID, "turn-heartbeat"; got != want {
			t.Fatalf("synthetic turn id = %q, want %q", got, want)
		}
		if got, want := h.driver.promptCalls[0].Meta.Synthetic.WakeEventID, "hwe-heartbeat"; got != want {
			t.Fatalf("synthetic wake_event_id = %q, want %q", got, want)
		}

		stored := readStoredEvents(t, session)
		payload := decodeStoredEventPayload(t, stored[0])
		syntheticPayload, ok := payload["synthetic"].(map[string]any)
		if !ok {
			t.Fatalf("stored synthetic metadata = %#v, want object", payload["synthetic"])
		}
		if got, want := syntheticPayload["wake_event_id"], "hwe-heartbeat"; got != want {
			t.Fatalf("stored wake_event_id = %v, want %q", got, want)
		}
		if got, want := syntheticPayload["policy_snapshot_id"], "hb-heartbeat"; got != want {
			t.Fatalf("stored policy_snapshot_id = %v, want %q", got, want)
		}
		if got, want := syntheticPayload["policy_digest"], "sha256:policy"; got != want {
			t.Fatalf("stored policy_digest = %v, want %q", got, want)
		}
	})
}

func TestPromptSyntheticQueuesBehindActiveTurnAndPreservesStoredOrder(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		if req.TurnID == "turn-1" {
			close(firstPromptEntered)
			events := make(chan acp.AgentEvent)
			go func() {
				<-releaseFirstPrompt
				close(events)
			}()
			return events, nil
		}

		events := make(chan acp.AgentEvent)
		close(events)
		return events, nil
	}

	userEventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	<-firstPromptEntered

	syntheticEventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic prompt",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-2",
			Reason:    "task_run_completed",
			Summary:   "queued behind user turn",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}

	if got := len(h.driver.promptCalls); got != 1 {
		t.Fatalf("len(promptCalls) before releasing active turn = %d, want 1", got)
	}

	close(releaseFirstPrompt)
	_ = collectEvents(t, userEventsCh)
	_ = collectEvents(t, syntheticEventsCh)

	if got := len(h.driver.promptCalls); got != 2 {
		t.Fatalf("len(promptCalls) after draining synthetic queue = %d, want 2", got)
	}
	if got := h.driver.promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
		t.Fatalf("promptCalls[1].Meta.TurnSource = %q, want %q", got, acp.PromptTurnSourceSynthetic)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) < 2 {
		t.Fatalf("stored events = %d, want at least the user and synthetic input events", len(stored))
	}
	if got := stored[0].Type; got != acp.EventTypeUserMessage {
		t.Fatalf("stored[0].Type = %q, want %q", got, acp.EventTypeUserMessage)
	}
	if got := stored[1].Type; got != acp.EventTypeSyntheticReentry {
		t.Fatalf("stored[1].Type = %q, want %q", got, acp.EventTypeSyntheticReentry)
	}
}

func TestPromptSyntheticInterruptsAgentWaitingTurnWhenRequested(t *testing.T) {
	t.Run("Should interrupt agent waiting when synthetic prompt requests it", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("failed to stop session %s: %v", session.ID, err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.cancelHook = func(*fakeProcess) error {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
			return nil
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.Message == "user prompt" {
				req.ActivityReporter(acp.PromptActivityReport{
					Kind:   runtimeActivityKindAgentWaiting,
					Detail: "waiting for detached work",
				})
				close(firstPromptEntered)
				events := make(chan acp.AgentEvent)
				go func() {
					<-releaseFirstPrompt
					close(events)
				}()
				return events, nil
			}
			events := make(chan acp.AgentEvent)
			close(events)
			return events, nil
		}

		userEventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		<-firstPromptEntered
		waitForCondition(t, "agent waiting activity", func() bool {
			info := session.Info()
			return info.Liveness != nil &&
				info.Liveness.Activity != nil &&
				info.Liveness.Activity.LastActivityKind == runtimeActivityKindAgentWaiting
		})

		syntheticEventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "detached completion",
			Metadata: acp.PromptSyntheticMeta{
				TaskRunID: "run-detached",
				Reason:    "task_run_completed",
				Summary:   "detached work completed",
			},
			InterruptIfAgentWaiting: true,
		})
		if err != nil {
			t.Fatalf("PromptSynthetic(interrupt waiting) error = %v", err)
		}
		_ = collectEvents(t, userEventsCh)
		_ = collectEvents(t, syntheticEventsCh)

		promptCalls := managerPromptCalls(h)
		if got, want := len(promptCalls), 2; got != want {
			t.Fatalf("len(promptCalls) = %d, want %d", got, want)
		}
		if got, want := promptCalls[1].Message, "detached completion"; got != want {
			t.Fatalf("synthetic prompt message = %q, want %q", got, want)
		}
		if got := promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
			t.Fatalf("synthetic turn source = %q, want %q", got, acp.PromptTurnSourceSynthetic)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("cancel calls = %d, want 1", got)
		}
	})

	t.Run("Should bound cancel when interrupting agent waiting for synthetic prompt", func(t *testing.T) {
		t.Parallel()

		supervision := aghconfig.DefaultSessionSupervisionConfig()
		supervision.TimeoutCancelGrace = 20 * time.Millisecond
		h := newHarness(t, WithSessionSupervision(supervision))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("failed to stop session %s: %v", session.ID, err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.cancelWithContextHook = func(ctx context.Context, _ *fakeProcess) error {
			timer := time.NewTimer(5 * supervision.TimeoutCancelGrace)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return errors.New("cancel prompt did not receive bounded context")
			}
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.Message == "user prompt" {
				req.ActivityReporter(acp.PromptActivityReport{
					Kind:   runtimeActivityKindAgentWaiting,
					Detail: "waiting for detached work",
				})
				close(firstPromptEntered)
				events := make(chan acp.AgentEvent)
				go func() {
					<-releaseFirstPrompt
					close(events)
				}()
				return events, nil
			}
			events := make(chan acp.AgentEvent)
			close(events)
			return events, nil
		}

		userEventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		<-firstPromptEntered
		waitForCondition(t, "agent waiting activity", func() bool {
			info := session.Info()
			return info.Liveness != nil &&
				info.Liveness.Activity != nil &&
				info.Liveness.Activity.LastActivityKind == runtimeActivityKindAgentWaiting
		})

		syntheticEventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "detached completion",
			Metadata: acp.PromptSyntheticMeta{
				TaskRunID: "run-detached",
				Reason:    "task_run_completed",
				Summary:   "detached work completed",
			},
			InterruptIfAgentWaiting: true,
		})
		if err != nil {
			t.Fatalf("PromptSynthetic(interrupt waiting) error = %v", err)
		}
		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
		_ = collectEvents(t, userEventsCh)
		_ = collectEvents(t, syntheticEventsCh)
	})
}

func TestApprovePermissionRoutesToActiveSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	var (
		gotReq sessionApproveCapture
		called bool
	)
	h.driver.approveHook = func(proc *fakeProcess, req acp.ApproveRequest) error {
		called = true
		gotReq = sessionApproveCapture{
			SessionID: proc.handle.SessionID,
			RequestID: req.RequestID,
			TurnID:    req.TurnID,
			Decision:  req.Decision,
		}
		return nil
	}

	err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
		RequestID: "req-1",
		TurnID:    "turn-1",
		Decision:  "allow-once",
	})
	if err != nil {
		t.Fatalf("ApprovePermission() error = %v", err)
	}
	if !called {
		t.Fatal("ApprovePermission() did not reach the active session process")
	}
	if gotReq.RequestID != "req-1" || gotReq.TurnID != "turn-1" || gotReq.Decision != "allow-once" {
		t.Fatalf("approve request = %#v", gotReq)
	}
}

func TestApprovePermissionReturnsNotActiveForStoppedSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
		RequestID: "req-1",
		Decision:  "allow-once",
	})
	if !errors.Is(err, ErrSessionNotActive) {
		t.Fatalf("ApprovePermission(stopped) error = %v, want ErrSessionNotActive", err)
	}
}

func TestApprovePermissionMapsPendingLookupErrors(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	testCases := []struct {
		name    string
		hookErr error
		wantErr error
	}{
		{
			name:    "ShouldMapNotFound",
			hookErr: acp.ErrPendingPermissionNotFound,
			wantErr: ErrPendingPermissionNotFound,
		},
		{
			name:    "ShouldMapConflict",
			hookErr: acp.ErrPendingPermissionConflict,
			wantErr: ErrPendingPermissionConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			session := createSession(t, h)
			t.Cleanup(func() {
				_ = h.manager.Stop(testutil.Context(t), session.ID)
			})

			h.driver.approveHook = func(*fakeProcess, acp.ApproveRequest) error {
				return tc.hookErr
			}
			err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
				RequestID: "req-1",
				Decision:  "allow-once",
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ApprovePermission() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestAgentCrashTransitionsToStoppedAndNotifies(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	h.driver.lastProcess().crash(errors.New("boom"), "stderr trace")

	waitForCondition(t, "session stopped after crash", func() bool {
		_, ok := h.manager.Get(session.ID)
		return !ok && h.notifier.stoppedCount() == 1
	})

	meta := readMeta(t, session.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopAgentCrashed)
	}

	events := readStoredEvents(t, session)
	if !containsEventType(events, acp.EventTypeError) {
		t.Fatalf("stored events missing crash error: %#v", events)
	}
	stopEvent := storedEventByType(t, events, EventTypeSessionStopped)
	stopPayload := decodeStoredEventPayload(t, stopEvent)
	if got, want := stopPayload["stop_reason"], string(store.StopAgentCrashed); got != want {
		t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
	}
}

func TestProcessExitDuringActivePromptPersistsAgentCrashedStopReason(t *testing.T) {
	t.Parallel()

	t.Run("Should persist StopAgentCrashed when process exits during active prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		source := make(chan acp.AgentEvent)
		promptStarted := make(chan struct{})
		var closePromptStarted sync.Once

		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			closePromptStarted.Do(func() {
				close(promptStarted)
			})
			return source, nil
		}

		_, err := h.manager.Prompt(testutil.Context(t), session.ID, "run a long command")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		select {
		case <-promptStarted:
		case <-time.After(time.Second):
			t.Fatal("prompt hook did not start")
		}

		h.driver.lastProcess().exit()

		waitForCondition(t, "session stopped after clean process exit during prompt", func() bool {
			meta := readMeta(t, session.MetaPath())
			return meta.State == string(StateStopped) && meta.StopReason != nil
		})
		close(source)

		meta := readMeta(t, session.MetaPath())
		if got, want := *meta.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("meta.StopReason = %q, want %q", got, want)
		}
		if meta.Failure == nil || meta.Failure.Kind != store.FailureProcess {
			t.Fatalf("meta.Failure = %#v, want process_exit", meta.Failure)
		}

		events := readStoredEvents(t, session)
		if !containsEventType(events, acp.EventTypeError) {
			t.Fatalf("stored events missing process-exit error: %#v", events)
		}
		stopEvent := storedEventByType(t, events, EventTypeSessionStopped)
		stopPayload := decodeStoredEventPayload(t, stopEvent)
		if got, want := stopPayload["stop_reason"], string(store.StopAgentCrashed); got != want {
			t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
		}
	})
}

func TestStopAndProcessExitFinalizeOnlyOnce(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	proceed := make(chan struct{})
	h.driver.stopHook = func(proc *fakeProcess) error {
		proc.crash(errors.New("boom"), "stderr trace")
		<-proceed
		return nil
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(testutil.Context(t), session.ID)
	}()

	waitForCondition(t, "stop notification", func() bool {
		return h.notifier.stoppedCount() == 1
	})
	close(proceed)

	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if got := h.notifier.stoppedCount(); got != 1 {
		t.Fatalf("stopped notifications = %d, want 1", got)
	}

	reopened, err := sessiondb.OpenSessionDB(testutil.Context(t), session.ID, session.DBPath())
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		_ = reopened.Close(testutil.Context(t))
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopened) error = %v", err)
	}
	if got := countEventType(events, EventTypeSessionStopped); got != 1 {
		t.Fatalf("countEventType(session_stopped) = %d, want 1", got)
	}
	meta := readMeta(t, session.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}
}

func TestStopWaitsForProcessDoneAfterSuccessfulDriverStop(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	release := make(chan struct{})
	h.driver.stopHook = func(proc *fakeProcess) error {
		go func() {
			<-release
			proc.exit()
		}()
		return nil
	}

	stopCtx, cancel := context.WithTimeout(testutil.Context(t), defaultLifecycleTimeout)
	defer cancel()

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(stopCtx, session.ID)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("Stop() completed early with %v, want it blocked on process exit", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	waitForCondition(t, "stopped session finalization", func() bool {
		_, ok := h.manager.Get(session.ID)
		return !ok && readMeta(t, session.MetaPath()).State == string(StateStopped)
	})
}

func TestPromptSerializesSetupAgainstConcurrentStop(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	promptEntered := make(chan struct{})
	releasePrompt := make(chan struct{})
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		close(promptEntered)
		<-releasePrompt
		events := make(chan acp.AgentEvent)
		close(events)
		return events, nil
	}

	promptDone := make(chan error, 1)
	go func() {
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			promptDone <- err
			return
		}
		for event := range eventsCh {
			_ = event
		}
		promptDone <- nil
	}()

	<-promptEntered

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(testutil.Context(t), session.ID)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("Stop() returned before prompt setup finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releasePrompt)

	if err := <-promptDone; err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestWaitForPromptDrains(t *testing.T) {
	t.Run("Should wait for active prompt pump exit", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		promptEvents := make(chan acp.AgentEvent)
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return promptEvents, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		waitDone := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
			defer cancel()
			waitDone <- h.manager.WaitForPromptDrains(ctx)
		}()

		select {
		case err := <-waitDone:
			t.Fatalf("WaitForPromptDrains() returned before prompt source closed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		close(promptEvents)
		_ = collectEvents(t, eventsCh)

		if err := <-waitDone; err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
		}
	})

	t.Run("Should make shutdown wait for every tracked prompt drain", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		finishDrain := h.manager.trackPromptDrain()
		shutdownDone := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
			defer cancel()
			shutdownDone <- h.manager.Shutdown(ctx)
		}()

		select {
		case err := <-shutdownDone:
			t.Fatalf("Shutdown() returned before prompt drain completed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		finishDrain()
		if err := <-shutdownDone; err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
}

func TestNormalizeEventSetsTimestampOnlyWhenZero(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	now := h.manager.now()

	normalized := h.manager.normalizeEvent(session, "turn-1", acp.AgentEvent{})
	if normalized.Timestamp.IsZero() {
		t.Fatal("normalizeEvent() left zero timestamp")
	}

	explicit := time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC)
	preserved := h.manager.normalizeEvent(session, "turn-1", acp.AgentEvent{Timestamp: explicit})
	if !preserved.Timestamp.Equal(explicit) {
		t.Fatalf("normalizeEvent() timestamp = %v, want %v", preserved.Timestamp, explicit)
	}
	if normalized.Timestamp.Before(now) {
		t.Fatalf("normalizeEvent() timestamp = %v, want >= %v", normalized.Timestamp, now)
	}
}

func TestListAndGet(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	first := createSession(t, h)
	second := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), first.ID)
		_ = h.manager.Stop(testutil.Context(t), second.ID)
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

func TestConcurrentCreateStopGet(t *testing.T) {
	h := newHarness(t)

	done := make(chan struct{})
	var readers sync.WaitGroup
	readers.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
				_ = h.manager.List()
				for _, info := range h.manager.List() {
					h.manager.Get(info.ID)
				}
			}
		}
	})

	const total = 8
	var workers sync.WaitGroup
	for i := range total {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()

			session, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder",
				Name:      fmt.Sprintf("session-%d", index),
				Workspace: h.workspaceID,
			})
			if err != nil {
				t.Errorf("Create(%d) error = %v", index, err)
				return
			}
			if _, ok := h.manager.Get(session.ID); !ok {
				t.Errorf("Get(%q) = missing after Create()", session.ID)
			}
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		}(i)
	}

	workers.Wait()
	close(done)
	readers.Wait()

	if list := h.manager.List(); len(list) != 0 {
		t.Fatalf("List() after concurrent stop = %d, want 0", len(list))
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
	h.cfg.MCPServers = []aghconfig.MCPServer{
		{Name: "global", Command: "global-command"},
	}
	h.cfg.Providers["claude"] = aghconfig.ProviderConfig{
		Command: "provider-command",
		MCPServers: []aghconfig.MCPServer{
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
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are helpful.",
			MCPServers: []aghconfig.MCPServer{
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
		WithMCPResolver(skillspkg.NewMCPResolver(aghconfig.SkillsConfig{}, nil)),
		WithLogger(slog.New(logs)),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
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
	h.cfg.MCPServers = []aghconfig.MCPServer{
		{
			Name:      "remote-http",
			Transport: aghconfig.MCPServerTransportHTTP,
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
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are helpful.",
			MCPServers: []aghconfig.MCPServer{
				{Name: "agent-stdio", Command: "agent-command"},
			},
		}},
	})
	hosted := &recordingHostedMCPLauncher{
		server: aghconfig.MCPServer{
			Name:      "agh-hosted-tools",
			Transport: aghconfig.MCPServerTransportStdio,
			Command:   "/bin/agh",
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
	if got[0].Name != "agh-hosted-tools" || got[0].Command != "/bin/agh" {
		t.Fatalf("hosted MCP server = %#v, want AGH hosted stdio entry", got[0])
	}

	requests := hosted.launchRequests()
	if len(requests) != 1 {
		t.Fatalf("hosted launch requests = %#v, want one launch request", requests)
	}
	if requests[0].SessionID != session.ID || requests[0].WorkspaceID != h.workspaceID ||
		requests[0].AgentName != "coder" {
		t.Fatalf("hosted launch request = %#v, want session/workspace/agent scope", requests[0])
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if releases := hosted.releaseSessionIDs(); !slices.Contains(releases, session.ID) {
		t.Fatalf("hosted releases = %#v, want released session %q", releases, session.ID)
	}
}

func TestCreateOmitsMCPServersForVerdictOnlyRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should withhold hosted MCP capabilities from verdict-only sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		hosted := &recordingHostedMCPLauncher{
			server: aghconfig.MCPServer{
				Name:      "agh-hosted-tools",
				Transport: aghconfig.MCPServerTransportStdio,
				Command:   "/bin/agh",
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
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "openclaw",
			Prompt:   "You are helpful.",
		}},
	})
	hosted := &recordingHostedMCPLauncher{
		server: aghconfig.MCPServer{
			Name:      "agh-hosted-tools",
			Transport: aghconfig.MCPServerTransportStdio,
			Command:   "/bin/agh",
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

func TestResumePassesMergedSkillMCPServers(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	skillRegistry := newFakeSkillRegistry()
	skillRegistry.setSkills(h.workspaceID, []*skillspkg.Skill{
		{
			Enabled: true,
			Source:  skillspkg.SourceUser,
			Meta:    skillspkg.SkillMeta{Name: "resume-skill"},
			MCPServers: []skillspkg.MCPServerDecl{
				{Name: "resume-extra", Command: "resume-extra-command"},
			},
		},
	})
	h.manager = newManagerWithHarness(
		t,
		h,
		WithSkillRegistry(skillRegistry),
		WithMCPResolver(skillspkg.NewMCPResolver(aghconfig.SkillsConfig{}, nil)),
	)

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	got := h.driver.startCalls[1].MCPServers
	if len(got) != 1 {
		t.Fatalf("resume start MCPServers = %#v, want 1 entry", got)
	}
	if got[0].Name != "resume-extra" || got[0].Command != "resume-extra-command" {
		t.Fatalf("resume MCP server = %#v", got[0])
	}
	if got := skillRegistry.callCount(); got != 2 {
		t.Fatalf("skill registry call count after resume = %d, want 2", got)
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
		WithMCPResolver(skillspkg.NewMCPResolver(aghconfig.SkillsConfig{}, nil)),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := h.driver.startCalls[0].MCPServers; len(got) != 0 {
		t.Fatalf("start MCPServers = %#v, want marketplace skill MCP blocked", got)
	}
}

func TestCreateInvokesPromptAssemblerWhenConfigured(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	var (
		called         bool
		gotWorkspace   string
		gotAgentName   string
		gotAgentPrompt string
	)
	h.manager = newManagerWithHarness(
		t,
		h,
		WithPromptAssembler(
			promptAssemblerFunc(
				func(_ context.Context, agent aghconfig.AgentDef, workspace *workspacepkg.ResolvedWorkspace) (string, error) {
					called = true
					gotWorkspace = workspace.RootDir
					gotAgentName = agent.Name
					gotAgentPrompt = agent.Prompt
					return agent.Prompt + "\n\nmemory block", nil
				},
			),
		),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if !called {
		t.Fatal("Create() did not invoke the configured prompt assembler")
	}
	if gotWorkspace != h.workspace {
		t.Fatalf("assembler workspace = %q, want %q", gotWorkspace, h.workspace)
	}
	if gotAgentName != "coder" {
		t.Fatalf("assembler agent name = %q, want %q", gotAgentName, "coder")
	}
	if gotAgentPrompt != "You are a coding assistant." {
		t.Fatalf("assembler prompt = %q, want original agent prompt", gotAgentPrompt)
	}
	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant.\n\nmemory block" {
		t.Fatalf("start system prompt = %q, want assembled prompt", got)
	}
}

func TestCreateInvokesStartupPromptOverlayWhenConfigured(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	var (
		called       bool
		gotChannel   string
		gotType      Type
		gotWorkspace string
	)
	h.manager = newManagerWithHarness(
		t,
		h,
		WithPromptAssembler(nil),
		WithStartupPromptOverlay(
			startupPromptOverlayFunc(func(
				_ context.Context,
				startup StartupPromptContext,
				prompt string,
			) (string, error) {
				called = true
				gotChannel = startup.NetworkParticipation.ChannelID
				gotType = startup.SessionType
				gotWorkspace = startup.Workspace
				return prompt + "\n\noverlay block", nil
			}),
		),
	)

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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if !called {
		t.Fatal("Create() did not invoke the configured startup prompt overlay")
	}
	if gotChannel != "builders" {
		t.Fatalf("startup overlay channel = %q, want %q", gotChannel, "builders")
	}
	if gotType != SessionTypeUser {
		t.Fatalf("startup overlay session type = %q, want %q", gotType, SessionTypeUser)
	}
	if gotWorkspace != h.workspace {
		t.Fatalf("startup overlay workspace = %q, want %q", gotWorkspace, h.workspace)
	}
	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant.\n\noverlay block" {
		t.Fatalf("start system prompt = %q, want overlay output", got)
	}
}

func TestCreateWithChannelAppendsBundledNetworkSkillAfterPromptAssembly(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	networkSkill, err := skillbundled.LoadResource(testBundledAghSkillName, testBundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", testBundledAghSkillName, testBundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)

	h.manager = newManagerWithHarness(
		t,
		h,
		WithPromptAssembler(
			startupPromptAssemblerFunc(
				func(
					_ context.Context,
					startup StartupPromptContext,
					agent aghconfig.AgentDef,
					workspace *workspacepkg.ResolvedWorkspace,
				) (string, error) {
					if got, want := workspace.RootDir, h.workspace; got != want {
						t.Fatalf("assembler workspace = %q, want %q", got, want)
					}
					prompt := agent.Prompt + "\n\nmemory block"
					if startup.NetworkParticipation.Mode != participation.ModeLive {
						return prompt, nil
					}
					return prompt + "\n\n" + networkSkill, nil
				},
			),
		),
	)

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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	wantPrompt := "You are a coding assistant.\n\nmemory block\n\n" + networkSkill
	if got := h.driver.startCalls[0].SystemPrompt; got != wantPrompt {
		t.Fatalf("start system prompt = %q, want %q", got, wantPrompt)
	}
	if got := strings.Count(h.driver.startCalls[0].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("network skill occurrences = %d, want 1", got)
	}
}

func TestCreateUsesRawPromptWhenAssemblerIsNil(t *testing.T) {
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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant." {
		t.Fatalf("start system prompt = %q, want raw agent prompt", got)
	}
}

func TestCreateWithoutChannelDoesNotAppendBundledNetworkSkill(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptAssembler(nil))
	networkSkill, err := skillbundled.LoadResource(testBundledAghSkillName, testBundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", testBundledAghSkillName, testBundledNetworkReference, err)
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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant." {
		t.Fatalf("start system prompt = %q, want raw agent prompt", got)
	}
	if strings.Contains(h.driver.startCalls[0].SystemPrompt, networkSkill) {
		t.Fatalf("start system prompt unexpectedly contains bundled network skill")
	}
}

func TestResumeWithChannelReinjectsBundledNetworkSkillOnce(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	networkSkill, err := skillbundled.LoadResource(testBundledAghSkillName, testBundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", testBundledAghSkillName, testBundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := strings.Count(h.driver.startCalls[0].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("create prompt network skill occurrences = %d, want 1", got)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	wantPrompt := "You are a coding assistant.\n\n" + networkSkill
	if got := h.driver.startCalls[1].SystemPrompt; got != wantPrompt {
		t.Fatalf("resume system prompt = %q, want %q", got, wantPrompt)
	}
	if got := strings.Count(h.driver.startCalls[1].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("resume prompt network skill occurrences = %d, want 1", got)
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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	env := h.driver.startCalls[0].Env
	if got, ok := lookupEnvValue(env, "AGH_SESSION_ID"); !ok || got != session.ID {
		t.Fatalf("AGH_SESSION_ID = %q, %v, want %q", got, ok, session.ID)
	}
	if got, ok := lookupEnvValue(env, "AGH_AGENT"); !ok || got != "coder" {
		t.Fatalf("AGH_AGENT = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "AGH_AGENT_NAME"); !ok || got != "coder" {
		t.Fatalf("AGH_AGENT_NAME = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "AGH_SESSION_CHANNEL"); !ok || got != "builders" {
		t.Fatalf("AGH_SESSION_CHANNEL = %q, %v, want %q", got, ok, "builders")
	}
	if got, ok := lookupEnvValue(env, "AGH_PEER_ID"); !ok || got != "coder."+session.ID {
		t.Fatalf("AGH_PEER_ID = %q, %v, want %q", got, ok, "coder."+session.ID)
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
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	env := h.driver.startCalls[0].Env
	if got, ok := lookupEnvValue(env, "AGH_SESSION_ID"); !ok || got != session.ID {
		t.Fatalf("AGH_SESSION_ID = %q, %v, want %q", got, ok, session.ID)
	}
	if got, ok := lookupEnvValue(env, "AGH_AGENT"); !ok || got != "coder" {
		t.Fatalf("AGH_AGENT = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "AGH_AGENT_NAME"); !ok || got != "coder" {
		t.Fatalf("AGH_AGENT_NAME = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "AGH_SESSION_CHANNEL"); ok {
		t.Fatalf("AGH_SESSION_CHANNEL = %q, want unset", got)
	}
	if got, ok := lookupEnvValue(env, "AGH_PEER_ID"); ok {
		t.Fatalf("AGH_PEER_ID = %q, want unset", got)
	}
}

func TestResumeWithChannelReinjectsNetworkSessionEnv(t *testing.T) {
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
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	env := h.driver.startCalls[1].Env
	if got, ok := lookupEnvValue(env, "AGH_SESSION_ID"); !ok || got != resumed.ID {
		t.Fatalf("AGH_SESSION_ID = %q, %v, want %q", got, ok, resumed.ID)
	}
	if got, ok := lookupEnvValue(env, "AGH_AGENT"); !ok || got != "coder" {
		t.Fatalf("AGH_AGENT = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "AGH_AGENT_NAME"); !ok || got != "coder" {
		t.Fatalf("AGH_AGENT_NAME = %q, %v, want %q", got, ok, "coder")
	}
	if got, ok := lookupEnvValue(env, "AGH_SESSION_CHANNEL"); !ok || got != "builders" {
		t.Fatalf("AGH_SESSION_CHANNEL = %q, %v, want %q", got, ok, "builders")
	}
	if got, ok := lookupEnvValue(env, "AGH_PEER_ID"); !ok || got != "coder."+resumed.ID {
		t.Fatalf("AGH_PEER_ID = %q, %v, want %q", got, ok, "coder."+resumed.ID)
	}
}

func TestCreateAppliesDreamPermissionsOverride(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.cfg.Permissions.Mode = aghconfig.PermissionModeDenyAll
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []aghconfig.AgentDef{
			{
				Name:     aghconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
		},
	})
	h.manager = newManagerWithHarness(t, h)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
		Type:      SessionTypeDream,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := h.driver.startCalls[0].Permissions; got != aghconfig.PermissionModeApproveAll {
		t.Fatalf("start permissions = %q, want %q", got, aghconfig.PermissionModeApproveAll)
	}
}

func TestCreateUsesConfiguredPermissionsForUserSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.cfg.Permissions.Mode = aghconfig.PermissionModeDenyAll
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []aghconfig.AgentDef{
			{
				Name:     aghconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
		},
	})
	h.manager = newManagerWithHarness(t, h)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
		Type:      SessionTypeUser,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := h.driver.startCalls[0].Permissions; got != aghconfig.PermissionModeDenyAll {
		t.Fatalf("start permissions = %q, want %q", got, aghconfig.PermissionModeDenyAll)
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

type harness struct {
	manager       *Manager
	driver        *fakeDriver
	notifier      *fakeNotifier
	resolver      *fakeWorkspaceResolver
	sandbox       *sandbox.Registry
	cfg           aghconfig.Config
	homePaths     aghconfig.HomePaths
	workspace     string
	workspaceID   string
	workspaceName string
}

type modelCatalogFunc func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)

func (f modelCatalogFunc) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return f(ctx, opts)
}

func newHarness(t *testing.T, extraOpts ...Option) *harness {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	workspace := filepath.Join(homePaths.HomeDir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}

	h := &harness{
		driver:        newFakeDriver(),
		notifier:      newFakeNotifier(),
		sandbox:       newFakeSandboxRegistry(t),
		cfg:           aghconfig.DefaultWithHome(homePaths),
		homePaths:     homePaths,
		workspace:     workspace,
		workspaceID:   "ws-primary",
		workspaceName: "workspace",
	}
	resolvedSandbox, err := h.cfg.ResolveSandbox(h.cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	h.resolver = newFakeWorkspaceResolver(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []aghconfig.AgentDef{
			{
				Name:     aghconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
		},
		Sandbox: resolvedSandbox,
	})
	h.manager = newManagerWithHarness(t, h, extraOpts...)
	return h
}

func newManagerWithHarness(t *testing.T, h *harness, extraOpts ...Option) *Manager {
	t.Helper()

	opts := []Option{
		WithHomePaths(h.homePaths),
		WithDriver(h.driver),
		WithNotifier(h.notifier),
		WithPromptAssembler(
			startupPromptAssemblerFunc(
				func(_ context.Context, startup StartupPromptContext, agent aghconfig.AgentDef, _ *workspacepkg.ResolvedWorkspace) (string, error) {
					prompt := strings.TrimSpace(agent.Prompt)
					if startup.NetworkParticipation.Mode != participation.ModeLive {
						return prompt, nil
					}
					networkSkill, err := skillbundled.LoadResource(testBundledAghSkillName, testBundledNetworkReference)
					if err != nil {
						return "", err
					}
					return prompt + "\n\n" + strings.TrimSpace(networkSkill), nil
				},
			),
		),
		WithWorkspaceResolver(h.resolver),
		WithStore(func(ctx context.Context, sessionID string, path string) (EventRecorder, error) {
			return sessiondb.OpenSessionDB(ctx, sessionID, path)
		}),
		WithQueryStore(func(ctx context.Context, sessionID string, path string) (EventReadCloser, error) {
			return sessiondb.OpenSessionDBReadOnly(ctx, sessionID, path)
		}),
		func(manager *Manager) {
			manager.queryStoreExplicit = false
		},
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithSessionIDGenerator(sequentialIDGenerator("sess")),
		WithTurnIDGenerator(sequentialIDGenerator("turn")),
		WithSandboxRegistry(h.sandbox),
		WithSandboxIDGenerator(sequentialIDGenerator("env")),
	}
	opts = append(opts, extraOpts...)

	manager, err := NewManager(opts...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func TestManagerOptionsPreserveExplicitQueryStore(t *testing.T) {
	t.Parallel()

	t.Run("Should keep explicit query store regardless of WithStore order", func(t *testing.T) {
		t.Parallel()

		called := ""
		queryOpener := func(context.Context, string, string) (EventReadCloser, error) {
			called = "query"
			return &stubRecorder{}, nil
		}
		storeOpener := func(context.Context, string, string) (EventRecorder, error) {
			called = "store"
			return &stubRecorder{}, nil
		}
		h := newHarness(t, WithQueryStore(queryOpener), WithStore(storeOpener))

		recorder, err := h.manager.openQueryStore(
			testutil.Context(t),
			"sess-query-order",
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

func createSession(t *testing.T, h *harness) *Session {
	t.Helper()

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "session",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return session
}

func createLiveNetworkSession(t *testing.T, h *harness) *Session {
	t.Helper()

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "network-session",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create(live network session) error = %v", err)
	}
	return session
}

func readMeta(t *testing.T, path string) store.SessionMeta {
	t.Helper()

	meta, err := store.ReadSessionMeta(path)
	if err != nil {
		t.Fatalf("ReadSessionMeta(%q) error = %v", path, err)
	}
	return meta
}

func readStoredEvents(t *testing.T, session *Session) []store.SessionEvent {
	t.Helper()

	reopened, err := sessiondb.OpenSessionDB(testutil.Context(t), session.ID, session.DBPath())
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopened) error = %v", err)
	}
	return events
}

func storedEventByType(t *testing.T, events []store.SessionEvent, want string) store.SessionEvent {
	t.Helper()

	for _, event := range events {
		if event.Type == want {
			return event
		}
	}

	t.Fatalf("stored event type %q not found", want)
	return store.SessionEvent{}
}

func decodeStoredEventPayload(t *testing.T, event store.SessionEvent) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
		t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
	}
	return payload
}

func collectEvents(t *testing.T, eventsCh <-chan acp.AgentEvent) []acp.AgentEvent {
	t.Helper()

	events := make([]acp.AgentEvent, 0, 4)
	for event := range eventsCh {
		events = append(events, event)
	}
	return events
}

func containsEventType(events []store.SessionEvent, want string) bool {
	for _, event := range events {
		if event.Type == want {
			return true
		}
	}
	return false
}

func countEventType(events []store.SessionEvent, want string) int {
	count := 0
	for _, event := range events {
		if event.Type == want {
			count++
		}
	}
	return count
}

func waitForCondition(t *testing.T, label string, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}

func sequentialIDGenerator(prefix string) IDGenerator {
	var counter atomic.Int64
	return func() string {
		return fmt.Sprintf("%s-%d", prefix, counter.Add(1))
	}
}

type promptAssemblerFunc func(context.Context, aghconfig.AgentDef, *workspacepkg.ResolvedWorkspace) (string, error)

func (fn promptAssemblerFunc) Assemble(
	ctx context.Context,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, agent, workspace)
}

type resumeContextPromptAssembler struct {
	checkpoint string
}

func (a *resumeContextPromptAssembler) Assemble(
	_ context.Context,
	agent aghconfig.AgentDef,
	_ *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return agent.Prompt, nil
}

func (a *resumeContextPromptAssembler) ResumeContextSection(
	_ context.Context,
	_ StartupPromptContext,
) (string, error) {
	return a.checkpoint, nil
}

const (
	testBundledAghSkillName     = "agh"
	testBundledNetworkReference = "references/network.md"
)

type startupPromptAssemblerFunc func(
	context.Context,
	StartupPromptContext,
	aghconfig.AgentDef,
	*workspacepkg.ResolvedWorkspace,
) (string, error)

func (fn startupPromptAssemblerFunc) Assemble(
	ctx context.Context,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, StartupPromptContext{
		AgentName:   strings.TrimSpace(agent.Name),
		SessionType: SessionTypeUser,
	}, agent, workspace)
}

func (fn startupPromptAssemblerFunc) AssembleStartup(
	ctx context.Context,
	startup StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, startup, agent, workspace)
}

type startupPromptOverlayFunc func(context.Context, StartupPromptContext, string) (string, error)

func (fn startupPromptOverlayFunc) Apply(
	ctx context.Context,
	startup StartupPromptContext,
	prompt string,
) (string, error) {
	return fn(ctx, startup, prompt)
}

type fakeNotifier struct {
	mu             sync.Mutex
	created        []*Info
	stopped        []*Info
	events         map[string][]acp.AgentEvent
	order          []string
	finalizingHook func(context.Context, *Session)
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{
		events: make(map[string][]acp.AgentEvent),
	}
}

func (n *fakeNotifier) OnSessionCreated(_ context.Context, session *Session) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.created = append(n.created, session.Info())
	n.order = append(n.order, "created:"+session.ID)
}

func (n *fakeNotifier) OnSessionStopped(_ context.Context, session *Session) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stopped = append(n.stopped, session.Info())
	n.order = append(n.order, "stopped:"+session.ID)
}

func (n *fakeNotifier) OnSessionFinalizing(ctx context.Context, session *Session) {
	n.mu.Lock()
	hook := n.finalizingHook
	if hook != nil {
		n.order = append(n.order, "finalizing:"+session.ID)
	}
	n.mu.Unlock()
	if hook != nil {
		hook(ctx, session)
	}
}

func (n *fakeNotifier) OnAgentEvent(_ context.Context, sessionID string, event any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	agentEvent, ok := event.(acp.AgentEvent)
	if !ok {
		return
	}
	n.events[sessionID] = append(n.events[sessionID], agentEvent)
}

func (n *fakeNotifier) createdCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.created)
}

func (n *fakeNotifier) stoppedCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.stopped)
}

func (n *fakeNotifier) eventCount(sessionID string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.events[sessionID])
}

func (n *fakeNotifier) eventsForSession(sessionID string) []acp.AgentEvent {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]acp.AgentEvent(nil), n.events[sessionID]...)
}

func (n *fakeNotifier) notificationOrder() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.order...)
}

type fakeNetworkPeerLifecycle struct {
	mu    sync.Mutex
	joins []fakeNetworkJoinCall
	left  []string
}

type fakeNetworkJoinCall struct {
	sessionID    string
	peerID       string
	channel      string
	capabilities []NetworkPeerCapability
}

func newFakeNetworkPeerLifecycle() *fakeNetworkPeerLifecycle {
	return &fakeNetworkPeerLifecycle{}
}

func (f *fakeNetworkPeerLifecycle) JoinChannel(
	_ context.Context,
	join NetworkPeerJoin,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.joins = append(f.joins, fakeNetworkJoinCall{
		sessionID:    join.SessionID,
		peerID:       join.PeerID,
		channel:      join.Channel,
		capabilities: cloneNetworkPeerCapabilities(join.Capabilities),
	})
	return nil
}

func (f *fakeNetworkPeerLifecycle) LeaveChannel(_ context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.left = append(f.left, sessionID)
	return nil
}

func (f *fakeNetworkPeerLifecycle) joinCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.joins)
}

func (f *fakeNetworkPeerLifecycle) joinCall(index int) fakeNetworkJoinCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.joins[index]
}

func (f *fakeNetworkPeerLifecycle) leaveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.left)
}

func (f *fakeNetworkPeerLifecycle) leaveCall(index int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.left[index]
}

type fakeEventRecorder struct {
	closeCalls int
}

type markerFailingRecorder struct {
	EventRecorder
	failErr    error
	closeCalls int
}

func (r *markerFailingRecorder) Record(ctx context.Context, event store.SessionEvent) error {
	if event.Type == events.TranscriptMarkerCreated {
		return r.failErr
	}
	return r.EventRecorder.Record(ctx, event)
}

func (r *markerFailingRecorder) Close(ctx context.Context) error {
	r.closeCalls++
	return r.EventRecorder.Close(ctx)
}

type failingSinglePromptRecorder struct {
	mu      sync.Mutex
	failErr error
	failed  bool
	events  []store.SessionEvent
}

func (r *failingSinglePromptRecorder) Record(ctx context.Context, event store.SessionEvent) error {
	_, err := r.RecordPersisted(ctx, event)
	return err
}

func (r *failingSinglePromptRecorder) RecordPersisted(
	_ context.Context,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failed || event.Type == acp.EventTypeToolCall {
		r.failed = true
		return store.SessionEvent{}, r.failErr
	}
	event.Sequence = int64(len(r.events) + 1)
	r.events = append(r.events, event)
	return event, nil
}

func (r *failingSinglePromptRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *failingSinglePromptRecorder) Query(
	context.Context,
	store.EventQuery,
) ([]store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]store.SessionEvent(nil), r.events...), nil
}

func (r *failingSinglePromptRecorder) History(
	context.Context,
	store.EventQuery,
) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *failingSinglePromptRecorder) Close(context.Context) error {
	return nil
}

func (r *fakeEventRecorder) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (r *fakeEventRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *fakeEventRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return nil, nil
}

func (r *fakeEventRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *fakeEventRecorder) Close(context.Context) error {
	r.closeCalls++
	return nil
}

type fakeDriver struct {
	mu                    sync.Mutex
	startCalls            []acp.StartOpts
	promptCalls           []acp.PromptRequest
	stopCalls             int
	cancelCalls           int
	processes             map[*AgentProcess]*fakeProcess
	lastProc              *fakeProcess
	promptHook            func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error)
	cancelHook            func(proc *fakeProcess) error
	cancelWithContextHook func(context.Context, *fakeProcess) error
	approveHook           func(proc *fakeProcess, req acp.ApproveRequest) error
	stopHook              func(proc *fakeProcess) error
	startHook             func(opts acp.StartOpts, sequence int) (*fakeProcess, error)
	interruptScopes       []toolruntime.InterruptScope
	interruptErr          error
	fallbackOnResume      bool
}

type fakeWorkspaceResolver struct {
	mu                     sync.Mutex
	byRef                  map[string]workspacepkg.ResolvedWorkspace
	byPath                 map[string]workspacepkg.ResolvedWorkspace
	resolveCalls           []string
	resolveOrRegisterCalls []string
	resolveErr             error
	resolveOrRegisterErr   error
	resolveHook            func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	resolveOrRegisterHook  func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	autoRegisterConfig     aghconfig.Config
	autoRegisterAgents     []aghconfig.AgentDef
	nextID                 int
}

type fakeSkillRegistry struct {
	mu                sync.Mutex
	skillsByWorkspace map[string][]*skillspkg.Skill
	calls             []workspacepkg.ResolvedWorkspace
	err               error
}

func newFakeWorkspaceResolver(resolved *workspacepkg.ResolvedWorkspace) *fakeWorkspaceResolver {
	r := &fakeWorkspaceResolver{
		byRef:              make(map[string]workspacepkg.ResolvedWorkspace),
		byPath:             make(map[string]workspacepkg.ResolvedWorkspace),
		autoRegisterConfig: resolved.Config,
		autoRegisterAgents: append([]aghconfig.AgentDef(nil), resolved.Agents...),
	}
	r.upsert(resolved)
	return r
}

func newFakeSkillRegistry() *fakeSkillRegistry {
	return &fakeSkillRegistry{
		skillsByWorkspace: make(map[string][]*skillspkg.Skill),
	}
}

func newFakeSandboxRegistry(t *testing.T) *sandbox.Registry {
	t.Helper()

	registry, err := sandbox.NewRegistry(fakeSandboxProvider{})
	if err != nil {
		t.Fatalf("NewRegistry(fake sandbox) error = %v", err)
	}
	return registry
}

type fakeSandboxProvider struct{}

func (fakeSandboxProvider) Backend() sandbox.Backend {
	return sandbox.BackendLocal
}

func (fakeSandboxProvider) Prepare(
	_ context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	state := sandbox.SessionState{
		SandboxID:             req.SandboxID,
		Backend:               sandbox.BackendLocal,
		Profile:               req.Sandbox.Profile,
		InstanceID:            strings.TrimSpace(req.InstanceID),
		State:                 "prepared",
		RuntimeRootDir:        req.LocalRootDir,
		RuntimeAdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
		ProviderState:         append(json.RawMessage(nil), req.ProviderState...),
		PreparedAt:            time.Now().UTC(),
	}
	return sandbox.Prepared{
		State:                 state,
		RuntimeRootDir:        req.LocalRootDir,
		RuntimeAdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
		Launch: sandbox.LaunchSpec{
			Command:        req.AgentCommand,
			Cwd:            req.LocalRootDir,
			AdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
			Env:            append([]string(nil), req.AgentEnv...),
		},
	}, nil
}

func (fakeSandboxProvider) SyncToRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	return sandbox.SyncResult{}, nil
}

func (fakeSandboxProvider) SyncFromRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	return sandbox.SyncResult{}, nil
}

func (fakeSandboxProvider) Destroy(context.Context, sandbox.SessionState) error {
	return nil
}

func (r *fakeSkillRegistry) ForWorkspace(
	_ context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]*skillspkg.Skill, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if resolved != nil {
		r.calls = append(r.calls, cloneResolvedWorkspaceForTests(resolved))
	}
	if r.err != nil {
		return nil, r.err
	}
	if resolved == nil {
		return nil, nil
	}

	skills := r.skillsByWorkspace[resolved.ID]
	return append([]*skillspkg.Skill(nil), skills...), nil
}

func (r *fakeSkillRegistry) ForAgentDef(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	_ aghconfig.AgentDef,
) ([]*skillspkg.Skill, error) {
	return r.ForWorkspace(ctx, resolved)
}

func (r *fakeSkillRegistry) setSkills(workspaceID string, skills []*skillspkg.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skillsByWorkspace[strings.TrimSpace(workspaceID)] = append([]*skillspkg.Skill(nil), skills...)
}

func (r *fakeSkillRegistry) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *fakeSkillRegistry) call(index int) workspacepkg.ResolvedWorkspace {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneResolvedWorkspaceForTests(&r.calls[index])
}

func (r *fakeWorkspaceResolver) Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ref := strings.TrimSpace(idOrPath)
	r.resolveCalls = append(r.resolveCalls, ref)
	if r.resolveHook != nil {
		return r.resolveHook(ctx, ref)
	}
	if r.resolveErr != nil {
		return workspacepkg.ResolvedWorkspace{}, r.resolveErr
	}
	if resolved, ok := r.byRef[ref]; ok {
		return cloneResolvedWorkspaceForTests(&resolved), nil
	}
	if resolved, ok := r.byPath[normalizeResolverPath(ref)]; ok {
		return cloneResolvedWorkspaceForTests(&resolved), nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *fakeWorkspaceResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	target := normalizeResolverPath(path)
	r.resolveOrRegisterCalls = append(r.resolveOrRegisterCalls, target)
	if r.resolveOrRegisterHook != nil {
		return r.resolveOrRegisterHook(ctx, target)
	}
	if r.resolveOrRegisterErr != nil {
		return workspacepkg.ResolvedWorkspace{}, r.resolveOrRegisterErr
	}
	if resolved, ok := r.byPath[target]; ok {
		return cloneResolvedWorkspaceForTests(&resolved), nil
	}

	r.nextID++
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      fmt.Sprintf("ws-auto-%d", r.nextID),
			RootDir: target,
			Name:    filepath.Base(target),
		},
		Config: r.autoRegisterConfig,
		Agents: append([]aghconfig.AgentDef(nil), r.autoRegisterAgents...),
	}
	r.upsert(&resolved)
	return cloneResolvedWorkspaceForTests(&resolved), nil
}

func (r *fakeWorkspaceResolver) upsert(resolved *workspacepkg.ResolvedWorkspace) {
	cloned := cloneResolvedWorkspaceForTests(resolved)
	if strings.TrimSpace(cloned.WorkspaceID) == "" {
		cloned.WorkspaceID = strings.TrimSpace(cloned.ID)
	}
	r.byRef[cloned.ID] = cloned
	if name := strings.TrimSpace(cloned.Name); name != "" {
		r.byRef[name] = cloned
	}
	if path := normalizeResolverPath(cloned.RootDir); path != "" {
		cloned.RootDir = path
		r.byPath[path] = cloned
	}
}

func normalizeResolverPath(path string) string {
	target := strings.TrimSpace(path)
	if target == "" {
		return ""
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return filepath.Clean(target)
	}
	return filepath.Clean(absPath)
}

func cloneResolvedWorkspaceForTests(src *workspacepkg.ResolvedWorkspace) workspacepkg.ResolvedWorkspace {
	if src == nil {
		return workspacepkg.ResolvedWorkspace{}
	}
	dst := *src
	dst.AdditionalDirs = append([]string(nil), src.AdditionalDirs...)
	dst.Agents = append([]aghconfig.AgentDef(nil), src.Agents...)
	dst.Skills = append([]workspacepkg.SkillPath(nil), src.Skills...)
	return dst
}

type recordingHostedMCPLauncher struct {
	mu       sync.Mutex
	server   aghconfig.MCPServer
	requests []HostedMCPLaunchRequest
	cancels  []string
	releases []string
}

var _ HostedMCPLauncher = (*recordingHostedMCPLauncher)(nil)

func (l *recordingHostedMCPLauncher) Launch(
	_ context.Context,
	req HostedMCPLaunchRequest,
) (aghconfig.MCPServer, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, req)
	return l.server, nil
}

func (l *recordingHostedMCPLauncher) CancelLaunch(sessionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancels = append(l.cancels, sessionID)
}

func (l *recordingHostedMCPLauncher) ReleaseSession(sessionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.releases = append(l.releases, sessionID)
}

func (l *recordingHostedMCPLauncher) launchRequests() []HostedMCPLaunchRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]HostedMCPLaunchRequest(nil), l.requests...)
}

func (l *recordingHostedMCPLauncher) releaseSessionIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.releases...)
}

func newFakeDriver() *fakeDriver {
	return &fakeDriver{
		processes: make(map[*AgentProcess]*fakeProcess),
	}
}

func (d *fakeDriver) Start(_ context.Context, opts acp.StartOpts) (*AgentProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	copied := opts
	copied.AdditionalDirs = append([]string(nil), opts.AdditionalDirs...)
	copied.Env = append([]string(nil), opts.Env...)
	copied.MCPServers = append([]aghconfig.MCPServer(nil), opts.MCPServers...)
	d.startCalls = append(d.startCalls, copied)

	sequence := len(d.startCalls)
	var proc *fakeProcess
	var err error
	if d.startHook != nil {
		proc, err = d.startHook(copied, sequence)
	} else {
		sessionID := fmt.Sprintf("acp-%d", sequence)
		if copied.ResumeSessionID != "" {
			if d.fallbackOnResume {
				sessionID = fmt.Sprintf("acp-new-%d", sequence)
			} else {
				sessionID = copied.ResumeSessionID
			}
		}
		proc = newFakeProcess(copied.AgentName, copied.Command, copied.Cwd, sessionID)
	}
	if err != nil {
		return nil, err
	}

	proc.handle.toolHost = copied.ToolHost
	proc.handle.approvePermissionFn = func(ctx context.Context, req acp.ApproveRequest) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		d.mu.Lock()
		hook := d.approveHook
		d.mu.Unlock()

		if hook != nil {
			return hook(proc, req)
		}
		return nil
	}

	d.processes[proc.handle] = proc
	d.lastProc = proc
	return proc.handle, nil
}

func (d *fakeDriver) Prompt(
	_ context.Context,
	proc *AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	fakeProc := d.processes[proc]
	d.promptCalls = append(d.promptCalls, req)
	hook := d.promptHook
	d.mu.Unlock()

	if fakeProc == nil {
		return nil, errors.New("test: unknown fake process")
	}
	if hook != nil {
		return hook(fakeProc, req)
	}

	totalTokens := int64(9)
	events := make(chan acp.AgentEvent, 2)
	go func() {
		defer close(events)
		ts := time.Now().UTC()
		events <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: fakeProc.handle.SessionID,
			TurnID:    req.TurnID,
			Timestamp: ts,
			Text:      "reply",
		}
		events <- acp.AgentEvent{
			Type:             acp.EventTypeDone,
			SessionID:        fakeProc.handle.SessionID,
			TurnID:           req.TurnID,
			Timestamp:        ts,
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
			Usage: &acp.TokenUsage{
				TurnID:      req.TurnID,
				TotalTokens: &totalTokens,
				Timestamp:   ts,
			},
		}
	}()
	return events, nil
}

func (d *fakeDriver) Cancel(ctx context.Context, proc *AgentProcess) error {
	d.mu.Lock()
	fakeProc := d.processes[proc]
	d.cancelCalls++
	hook := d.cancelHook
	contextHook := d.cancelWithContextHook
	d.mu.Unlock()

	if fakeProc == nil {
		return errors.New("test: unknown fake process")
	}
	if contextHook != nil {
		return contextHook(ctx, fakeProc)
	}
	if hook != nil {
		return hook(fakeProc)
	}
	return nil
}

func (d *fakeDriver) Interrupt(
	_ context.Context,
	sessionID string,
	turnID string,
) (toolruntime.InterruptReport, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.interruptScopes = append(d.interruptScopes, toolruntime.InterruptScope{
		SessionID: sessionID,
		TurnID:    turnID,
	})
	if d.interruptErr != nil {
		return toolruntime.InterruptReport{}, d.interruptErr
	}
	return toolruntime.InterruptReport{Matched: 1, Signaled: 1}, nil
}

func (d *fakeDriver) Stop(_ context.Context, proc *AgentProcess) error {
	d.mu.Lock()
	fakeProc := d.processes[proc]
	d.stopCalls++
	hook := d.stopHook
	d.mu.Unlock()

	if fakeProc == nil {
		return errors.New("test: unknown fake process")
	}
	if hook != nil {
		return hook(fakeProc)
	}
	fakeProc.exit()
	return nil
}

func (d *fakeDriver) lastProcess() *fakeProcess {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastProc
}

func lookupEnvValue(env []string, key string) (string, bool) {
	for _, entry := range env {
		existingKey, value, ok := strings.Cut(entry, "=")
		if ok && existingKey == key {
			return value, true
		}
	}
	return "", false
}

type fakeProcess struct {
	mu      sync.Mutex
	done    chan struct{}
	closed  bool
	waitErr error
	stderr  string
	health  subprocess.HealthState
	handle  *AgentProcess
}

type sessionApproveCapture struct {
	SessionID string
	RequestID string
	TurnID    string
	Decision  string
}

func newFakeProcess(agentName string, command string, cwd string, sessionID string) *fakeProcess {
	proc := &fakeProcess{
		done:   make(chan struct{}),
		health: subprocess.HealthState{Healthy: true},
	}
	proc.handle = &AgentProcess{
		PID:       1,
		AgentName: agentName,
		Command:   command,
		Cwd:       cwd,
		SessionID: sessionID,
		Caps: acp.Caps{
			SupportsLoadSession: true,
			SupportedModes:      []string{"chat"},
		},
		StartedAt: time.Now().UTC(),
		done:      proc.done,
		waitFn:    proc.wait,
		stderrFn:  proc.stderrOutput,
		healthStateFn: func() subprocess.HealthState {
			proc.mu.Lock()
			defer proc.mu.Unlock()
			return proc.health
		},
	}
	return proc
}

func (p *fakeProcess) wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

func (p *fakeProcess) stderrOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stderr
}

func (p *fakeProcess) setHealth(state subprocess.HealthState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health = state
}

func (p *fakeProcess) exit() {
	p.finish(nil, "")
}

func (p *fakeProcess) crash(err error, stderr string) {
	p.finish(err, stderr)
}

func (p *fakeProcess) finish(err error, stderr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.waitErr = err
	p.stderr = stderr
	if !p.closed {
		p.closed = true
		close(p.done)
	}
}

func recordResumeReplayFixture(t *testing.T, manager *Manager, session *Session, contextText string) {
	t.Helper()

	turnID := "turn-resume-replay"
	now := time.Now().UTC()
	toolInput, err := json.Marshal(map[string]string{
		"path": strings.Repeat("nested/path/", 80) + "fixture.txt",
	})
	if err != nil {
		t.Fatalf("json.Marshal(tool input) error = %v", err)
	}
	events := []acp.AgentEvent{
		{
			Type:      acp.EventTypeUserMessage,
			TurnID:    turnID,
			Timestamp: now,
			Text:      contextText,
		},
		{
			Type:      acp.EventTypeAgentMessage,
			TurnID:    turnID,
			Timestamp: now.Add(time.Millisecond),
			Text:      "Acknowledged " + contextText,
		},
		acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     turnID,
			ToolCallID: "tool-resume-replay",
			Timestamp:  now.Add(2 * time.Millisecond),
		}.WithTool("read", toolInput, false),
		acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			TurnID:     turnID,
			ToolCallID: "tool-resume-replay",
			Timestamp:  now.Add(3 * time.Millisecond),
			Text:       strings.Repeat("tool-noise-line\n", 80),
		}.WithTool("read", nil, false),
	}
	for _, event := range events {
		if err := manager.recordEvent(testutil.Context(t), session, event); err != nil {
			t.Fatalf("recordEvent(%q) error = %v", event.Type, err)
		}
	}
}

func assertResumeReplayEqualsPrunedEvents(
	t *testing.T,
	systemPrompt string,
	events []store.SessionEvent,
) {
	t.Helper()

	want, err := transcript.Assemble(events)
	if err != nil {
		t.Fatalf("transcript.Assemble(events) error = %v", err)
	}
	want = transcript.Prune(want, transcript.PruneOptions{Dedup: true})
	got := resumeReplayMessagesFromPrompt(t, systemPrompt)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resume replay = %#v, want pruned transcript %#v", got, want)
	}
}

func resumeReplayMessagesFromPrompt(t *testing.T, systemPrompt string) []transcript.Message {
	t.Helper()

	_, replayAndSuffix, ok := strings.Cut(systemPrompt, "<agh_context_replay>\n")
	if !ok {
		t.Fatalf("system prompt is missing replay start marker: %q", systemPrompt)
	}
	replayJSON, _, ok := strings.Cut(replayAndSuffix, "\n</agh_context_replay>")
	if !ok {
		t.Fatalf("system prompt is missing replay end marker: %q", systemPrompt)
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(replayJSON), &messages); err != nil {
		t.Fatalf("json.Unmarshal(resume replay) error = %v", err)
	}
	return messages
}

func assertContextRebuiltMarkerCount(t *testing.T, events []store.SessionEvent, want int) {
	t.Helper()

	messages, err := transcript.Assemble(events)
	if err != nil {
		t.Fatalf("transcript.Assemble(events) error = %v", err)
	}
	count := 0
	for _, message := range messages {
		if message.Role == transcript.RoleSystem && message.Content == "Context rebuilt from log." {
			count++
		}
	}
	if count != want {
		t.Fatalf("context rebuilt marker count = %d, want %d", count, want)
	}
}
