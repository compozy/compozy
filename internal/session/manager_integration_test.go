//go:build integration

package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	skillbundled "github.com/compozy/agh/skills"
)

func TestManagerIntegrationFullLifecycle(t *testing.T) {
	h := newHarness(t)

	sessionCWD := filepath.Join(h.workspace, "nested-session-cwd")
	if err := os.MkdirAll(sessionCWD, 0o755); err != nil {
		t.Fatalf("MkdirAll(session CWD) error = %v", err)
	}
	canonicalSessionCWD := resolveIntegrationWorkspaceRoot(t, sessionCWD)
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "session",
		Workspace: h.workspaceID,
		CWD:       sessionCWD,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := h.driver.startCalls[0].Cwd; got != canonicalSessionCWD {
		t.Fatalf("Create() CWD = %q, want %q", got, canonicalSessionCWD)
	}
	firstPrompt, err := h.manager.Prompt(testutil.Context(t), session.ID, "first")
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	firstEvents := collectEvents(t, firstPrompt)
	if len(firstEvents) != 2 {
		t.Fatalf("first prompt events = %d, want 2", len(firstEvents))
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if got := h.driver.startCalls[1].Cwd; got != canonicalSessionCWD {
		t.Fatalf("Resume() CWD = %q, want persisted %q", got, canonicalSessionCWD)
	}

	secondPrompt, err := h.manager.Prompt(testutil.Context(t), resumed.ID, "second")
	if err != nil {
		t.Fatalf("Prompt(second) error = %v", err)
	}
	secondEvents := collectEvents(t, secondPrompt)
	if len(secondEvents) != 2 {
		t.Fatalf("second prompt events = %d, want 2", len(secondEvents))
	}

	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("final Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(testutil.Context(t), resumed.ID, resumed.DBPath())
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("stored events = %d, want 10", len(events))
	}
	if !containsEventType(events, acp.EventTypeAgentMessage) || !containsEventType(events, acp.EventTypeDone) {
		t.Fatalf("stored events missing expected types: %#v", events)
	}
	if got := countEventType(events, EventTypeSessionStopped); got != 2 {
		t.Fatalf("stored %q events = %d, want 2", EventTypeSessionStopped, got)
	}
	if got := countEventType(events, eventspkg.TranscriptMarkerCreated); got != 2 {
		t.Fatalf("stored %q events = %d, want 2", eventspkg.TranscriptMarkerCreated, got)
	}

	meta := readMeta(t, resumed.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
}

func TestManagerIntegrationCapabilityAwareJoinCarriesCatalogAcrossCreateResumeAndStop(t *testing.T) {
	h := newHarness(t)
	lifecycle := newFakeNetworkPeerLifecycle()
	h.manager.SetNetworkPeerLifecycle(lifecycle)

	resolvedSandbox, err := h.cfg.ResolveSandbox(h.cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	capabilityAgent := aghconfig.AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "You are a coding assistant.",
		Capabilities: &aghconfig.CapabilityCatalog{
			Capabilities: []aghconfig.CapabilityDef{{
				ID:      "review-pr",
				Summary: "Review pull requests",
				Outcome: "Deliver actionable pull request feedback",
				Version: "1.0.0",
				ContextNeeded: []string{
					"Pull request diff",
					"Acceptance criteria",
				},
				ArtifactsExpected: []string{
					"Review summary",
				},
				Requirements: []string{
					"workspace-write",
					"review-guidelines",
				},
			}},
		},
	}
	if err := capabilityAgent.Validate(); err != nil {
		t.Fatalf("capabilityAgent.Validate() error = %v", err)
	}
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
			capabilityAgent,
		},
		Sandbox: resolvedSandbox,
	})

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
		t.Fatalf("join count after Create() = %d, want 1", got)
	}
	firstJoin := lifecycle.joinCall(0)
	if got, want := firstJoin.sessionID, session.ID; got != want {
		t.Fatalf("first join session_id = %q, want %q", got, want)
	}
	if got, want := firstJoin.peerID, "coder."+session.ID; got != want {
		t.Fatalf("first join peer_id = %q, want %q", got, want)
	}
	wantDigest, err := aghconfig.CanonicalCapabilityDigest(aghconfig.CapabilityDef{
		ID:      "review-pr",
		Summary: "Review pull requests",
		Outcome: "Deliver actionable pull request feedback",
		Version: "1.0.0",
		ContextNeeded: []string{
			"Pull request diff",
			"Acceptance criteria",
		},
		ArtifactsExpected: []string{
			"Review summary",
		},
		Requirements: []string{
			"workspace-write",
			"review-guidelines",
		},
	})
	if err != nil {
		t.Fatalf("CanonicalCapabilityDigest() error = %v", err)
	}
	wantCapabilities := []NetworkPeerCapability{{
		ID:                "review-pr",
		Summary:           "Review pull requests",
		Outcome:           "Deliver actionable pull request feedback",
		Version:           "1.0.0",
		Digest:            wantDigest,
		ContextNeeded:     []string{"Pull request diff", "Acceptance criteria"},
		ArtifactsExpected: []string{"Review summary"},
		Requirements:      []string{"review-guidelines", "workspace-write"},
	}}
	if !reflect.DeepEqual(firstJoin.capabilities, wantCapabilities) {
		t.Fatalf("first join capabilities = %#v, want %#v", firstJoin.capabilities, wantCapabilities)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if got := lifecycle.leaveCount(); got != 1 {
		t.Fatalf("leave count after Stop() = %d, want 1", got)
	}
	if got, want := lifecycle.leaveCall(0), session.ID; got != want {
		t.Fatalf("leave session_id = %q, want %q", got, want)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if got := lifecycle.joinCount(); got != 2 {
		t.Fatalf("join count after Resume() = %d, want 2", got)
	}
	secondJoin := lifecycle.joinCall(1)
	if got, want := secondJoin.sessionID, resumed.ID; got != want {
		t.Fatalf("second join session_id = %q, want %q", got, want)
	}
	if got, want := secondJoin.peerID, "coder."+resumed.ID; got != want {
		t.Fatalf("second join peer_id = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(secondJoin.capabilities, wantCapabilities) {
		t.Fatalf("second join capabilities = %#v, want %#v", secondJoin.capabilities, wantCapabilities)
	}

	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("final Stop() error = %v", err)
	}
	if got := lifecycle.leaveCount(); got != 2 {
		t.Fatalf("leave count after resumed Stop() = %d, want 2", got)
	}
}

func TestManagerIntegrationCapabilityAwareJoinKeepsMissingCatalogProjectionEmpty(t *testing.T) {
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
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := lifecycle.joinCount(); got != 1 {
		t.Fatalf("join count after Create() = %d, want 1", got)
	}
	join := lifecycle.joinCall(0)
	if join.capabilities == nil {
		t.Fatal("join capabilities = nil, want deterministic empty projection")
	}
	if got := len(join.capabilities); got != 0 {
		t.Fatalf("join capabilities len = %d, want 0", got)
	}
}

func TestManagerIntegrationCapabilityProjectionDoesNotAliasSourceCatalog(t *testing.T) {
	h := newHarness(t)

	resolvedSandbox, err := h.cfg.ResolveSandbox(h.cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	capabilityAgent := aghconfig.AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   "You are a coding assistant.",
		Capabilities: &aghconfig.CapabilityCatalog{
			Capabilities: []aghconfig.CapabilityDef{{
				ID:               "review-pr",
				Summary:          "Review pull requests",
				Outcome:          "Deliver actionable pull request feedback",
				Version:          "1.0.0",
				ContextNeeded:    []string{"Pull request diff", "Acceptance criteria"},
				ExecutionOutline: []string{"inspect", "comment"},
				Requirements:     []string{"workspace-write", "review-guidelines"},
			}},
		},
	}
	if err := capabilityAgent.Validate(); err != nil {
		t.Fatalf("capabilityAgent.Validate() error = %v", err)
	}
	sourceBefore := capabilityAgent.Capabilities.Clone()

	h.manager.SetNetworkPeerLifecycle(&mutatingNetworkPeerLifecycle{})
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
			capabilityAgent,
		},
		Sandbox: resolvedSandbox,
	})

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

	if !reflect.DeepEqual(capabilityAgent.Capabilities, sourceBefore) {
		t.Fatalf(
			"source capability catalog mutated through join projection:\nbefore=%#v\nafter=%#v",
			sourceBefore,
			capabilityAgent.Capabilities,
		)
	}
}

type mutatingNetworkPeerLifecycle struct{}

func (m *mutatingNetworkPeerLifecycle) JoinChannel(_ context.Context, join NetworkPeerJoin) error {
	if len(join.Capabilities) == 0 {
		return nil
	}

	join.Capabilities[0].Summary = "mutated summary"
	join.Capabilities[0].Digest = "sha256:mutated"
	if len(join.Capabilities[0].ContextNeeded) > 0 {
		join.Capabilities[0].ContextNeeded[0] = "mutated context"
	}
	if len(join.Capabilities[0].ExecutionOutline) > 0 {
		join.Capabilities[0].ExecutionOutline[0] = "mutated execution outline"
	}
	if len(join.Capabilities[0].Requirements) > 0 {
		join.Capabilities[0].Requirements[0] = "mutated requirement"
	}

	return nil
}

func (m *mutatingNetworkPeerLifecycle) LeaveChannel(context.Context, string) error {
	return nil
}

func TestManagerIntegrationUsesRealSQLitePerSessionDB(t *testing.T) {
	h := newHarness(t)

	session := createSession(t, h)
	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "persist")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	recorder, ok := session.recorderHandle().(*sessiondb.SessionDB)
	if !ok {
		t.Fatalf("recorder = %T, want *sessiondb.SessionDB", session.recorderHandle())
	}
	if got, want := recorder.Path(), session.DBPath(); got != want {
		t.Fatalf("SessionDB.Path() = %q, want %q", got, want)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(testutil.Context(t), session.ID, session.DBPath())
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("Query(reopen) returned 0 events, want persisted rows")
	}
}

func TestManagerIntegrationSyntheticPromptPersistsDedicatedEventsWithMixedHistory(t *testing.T) {
	h := newHarness(t)

	session := createLiveNetworkSession(t, h)
	userEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
	if err != nil {
		t.Fatalf("Prompt(user) error = %v", err)
	}
	_ = collectEvents(t, userEvents)

	networkEvents, err := h.manager.PromptNetwork(
		testutil.Context(t),
		session.ID,
		"network prompt",
		acp.PromptNetworkMeta{MessageID: "msg-1", Kind: "direct"},
	)
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	_ = collectEvents(t, networkEvents)

	syntheticEvents, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
			Reason:    "task_run_completed",
			Summary:   "background task finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, syntheticEvents)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	reopened, err := sessiondb.OpenSessionDB(testutil.Context(t), session.ID, session.DBPath())
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(testutil.Context(t)); err != nil {
			t.Fatalf("reopened.Close() error = %v", err)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopen) error = %v", err)
	}
	if got := countEventType(events, acp.EventTypeUserMessage); got != 2 {
		t.Fatalf("countEventType(user_message) = %d, want 2 for user+network input", got)
	}
	if got := countEventType(events, acp.EventTypeSyntheticReentry); got != 1 {
		t.Fatalf("countEventType(synthetic_reentry) = %d, want 1", got)
	}
	if !containsEventType(events, acp.EventTypeAgentMessage) || !containsEventType(events, acp.EventTypeDone) {
		t.Fatalf("mixed history missing runtime events: %#v", events)
	}
}

func TestManagerIntegrationSyntheticQueuePreservesOrderingBehindActivePrompt(t *testing.T) {
	h := newHarness(t)

	session := createSession(t, h)

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		if req.TurnID == "turn-1" {
			close(firstPromptEntered)
			events := make(chan acp.AgentEvent)
			go func() {
				<-releaseFirstPrompt
				events <- acp.AgentEvent{
					Type:      acp.EventTypeDone,
					TurnID:    req.TurnID,
					Timestamp: time.Now().UTC(),
				}
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
		t.Fatalf("Prompt(user) error = %v", err)
	}
	<-firstPromptEntered

	syntheticEvents, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-2",
			Reason:    "task_run_completed",
			Summary:   "queued after user turn",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}

	close(releaseFirstPrompt)
	_ = collectEvents(t, userEvents)
	_ = collectEvents(t, syntheticEvents)

	events, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("stored events = %d, want at least user, done, and synthetic events", len(events))
	}

	userIndex := -1
	doneIndex := -1
	syntheticIndex := -1
	for i, event := range events {
		switch event.Type {
		case acp.EventTypeUserMessage:
			if userIndex < 0 {
				userIndex = i
			}
		case acp.EventTypeDone:
			if doneIndex < 0 {
				doneIndex = i
			}
		case acp.EventTypeSyntheticReentry:
			if syntheticIndex < 0 {
				syntheticIndex = i
			}
		}
	}
	if userIndex < 0 {
		t.Fatalf("stored events missing %q: %#v", acp.EventTypeUserMessage, events)
	}
	if doneIndex < 0 {
		t.Fatalf("stored events missing %q: %#v", acp.EventTypeDone, events)
	}
	if syntheticIndex < 0 {
		t.Fatalf("stored events missing %q: %#v", acp.EventTypeSyntheticReentry, events)
	}
	if !(userIndex < doneIndex && doneIndex < syntheticIndex) {
		t.Fatalf(
			"event order user=%d done=%d synthetic=%d, want user < done < synthetic",
			userIndex,
			doneIndex,
			syntheticIndex,
		)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("cleanup Stop() error = %v", err)
	}
}

func TestManagerIntegrationRemovePurgesSyntheticState(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*Manager, string)
	}{
		{
			name: "Should purge synthetic state on remove",
			remove: func(m *Manager, id string) {
				m.remove(id)
			},
		},
		{
			name: "Should purge synthetic state on removeActive",
			remove: func(m *Manager, id string) {
				m.removeActive(id)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventsCh := make(chan acp.AgentEvent, 1)
			finalizing := make(chan struct{})
			manager := &Manager{
				sessions: map[string]*Session{
					"sess-synth": {ID: "sess-synth"},
				},
				pending: map[string]sessionReservation{
					"sess-synth": {},
				},
				finalizing: map[string]*sessionFinalization{
					"sess-synth": {done: finalizing},
				},
				syntheticQueues: map[string][]queuedSyntheticPrompt{
					"sess-synth": {{
						request: promptRequest{target: "sess-synth", turnID: "turn-synth"},
						out:     eventsCh,
					}},
				},
				syntheticDispatching: map[string]bool{
					"sess-synth": true,
				},
			}

			tc.remove(manager, "sess-synth")

			manager.syntheticMu.Lock()
			if got := len(manager.syntheticQueues["sess-synth"]); got != 0 {
				manager.syntheticMu.Unlock()
				t.Fatalf("len(syntheticQueues[\"sess-synth\"]) = %d, want 0", got)
			}
			if manager.syntheticDispatching["sess-synth"] {
				manager.syntheticMu.Unlock()
				t.Fatal("syntheticDispatching[\"sess-synth\"] = true, want cleared")
			}
			manager.syntheticMu.Unlock()

			event, ok := <-eventsCh
			if !ok {
				t.Fatal("queued synthetic output closed without error event")
			}
			if got, want := event.Type, acp.EventTypeError; got != want {
				t.Fatalf("queued synthetic event type = %q, want %q", got, want)
			}
			if got, want := event.TurnID, "turn-synth"; got != want {
				t.Fatalf("queued synthetic event turn id = %q, want %q", got, want)
			}
			if !strings.Contains(event.Error, "synthetic prompt dropped") {
				t.Fatalf("queued synthetic error = %q, want drop summary", event.Error)
			}
			if _, ok := <-eventsCh; ok {
				t.Fatal("queued synthetic output channel left open after removal")
			}
		})
	}
}

func TestManagerIntegrationSyntheticQueueStateTransitions(t *testing.T) {
	t.Run("Should requeue a claimed synthetic prompt before clearing dispatch", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			syntheticQueues: map[string][]queuedSyntheticPrompt{
				"sess-synth": {{
					request: promptRequest{turnID: "turn-queued"},
				}},
			},
			syntheticDispatching: map[string]bool{
				"sess-synth": true,
			},
		}
		claimed := queuedSyntheticPrompt{request: promptRequest{turnID: "turn-claimed"}}

		manager.requeueSyntheticPromptFrontAndFinishDispatch("sess-synth", claimed)

		manager.syntheticMu.Lock()
		defer manager.syntheticMu.Unlock()

		if manager.syntheticDispatching["sess-synth"] {
			t.Fatal("syntheticDispatching[\"sess-synth\"] = true, want cleared")
		}
		queue := manager.syntheticQueues["sess-synth"]
		if got, want := len(queue), 2; got != want {
			t.Fatalf("len(syntheticQueues[\"sess-synth\"]) = %d, want %d", got, want)
		}
		if got, want := queue[0].request.turnID, "turn-claimed"; got != want {
			t.Fatalf("queue[0].request.turnID = %q, want %q", got, want)
		}
		if got, want := queue[1].request.turnID, "turn-queued"; got != want {
			t.Fatalf("queue[1].request.turnID = %q, want %q", got, want)
		}
	})

	t.Run("Should drain queued synthetic prompts while clearing dispatch", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			syntheticQueues: map[string][]queuedSyntheticPrompt{
				"sess-synth": {
					{request: promptRequest{turnID: "turn-1"}},
					{request: promptRequest{turnID: "turn-2"}},
				},
			},
			syntheticDispatching: map[string]bool{
				"sess-synth": true,
			},
		}

		drained := manager.finishQueuedSyntheticDispatchAndDrain("sess-synth")

		manager.syntheticMu.Lock()
		defer manager.syntheticMu.Unlock()

		if manager.syntheticDispatching["sess-synth"] {
			t.Fatal("syntheticDispatching[\"sess-synth\"] = true, want cleared")
		}
		if got := len(manager.syntheticQueues["sess-synth"]); got != 0 {
			t.Fatalf("len(syntheticQueues[\"sess-synth\"]) = %d, want 0", got)
		}
		if got, want := len(drained), 2; got != want {
			t.Fatalf("len(drained) = %d, want %d", got, want)
		}
		if got, want := drained[0].request.turnID, "turn-1"; got != want {
			t.Fatalf("drained[0].request.turnID = %q, want %q", got, want)
		}
		if got, want := drained[1].request.turnID, "turn-2"; got != want {
			t.Fatalf("drained[1].request.turnID = %q, want %q", got, want)
		}
	})
}

func TestResolveWorkspaceSessionAgentGuardsNilInputs(t *testing.T) {
	t.Run("Should reject a nil resolved workspace", func(t *testing.T) {
		t.Parallel()

		_, err := resolveWorkspaceSessionAgentForType("coder", "", "", nil, nil)
		if err == nil {
			t.Fatal("resolveWorkspaceSessionAgent(nil workspace) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "resolved workspace is required") {
			t.Fatalf("resolveWorkspaceSessionAgent(nil workspace) error = %v", err)
		}
	})

	t.Run("Should allow a nil agent resolver when a workspace is provided", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		resolvedWorkspace := &workspacepkg.ResolvedWorkspace{
			Config: aghconfig.DefaultWithHome(homePaths),
			Agents: []aghconfig.AgentDef{{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			}},
		}

		resolved, err := resolveWorkspaceSessionAgentForType("coder", "", "", resolvedWorkspace, nil)
		if err != nil {
			t.Fatalf("resolveWorkspaceSessionAgentForType(nil agent resolver) error = %v", err)
		}
		if got, want := resolved.Provider, "claude"; got != want {
			t.Fatalf("resolveWorkspaceSessionAgentForType(nil agent resolver) provider = %q, want %q", got, want)
		}
	})
}

func TestManagerIntegrationResumeWithChannelReinjectsBundledNetworkSkillBeforeACPStart(t *testing.T) {
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

	if got := h.driver.startCalls[1].SystemPrompt; !strings.Contains(got, networkSkill) {
		t.Fatalf("resume system prompt = %q, want bundled network skill content", got)
	}
	if got := strings.Count(h.driver.startCalls[1].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("resume prompt network skill occurrences = %d, want 1", got)
	}
}

func TestManagerIntegrationFullLifecycleHooksFireInOrder(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(entry string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, entry)
	}

	dispatcher := &spyHookDispatcher{
		dispatchSessionPreCreateFn: func(_ context.Context, payload hookspkg.SessionPreCreatePayload) (hookspkg.SessionPreCreatePayload, error) {
			record("session.pre_create")
			return payload, nil
		},
		dispatchPromptPostAssembleFn: func(_ context.Context, payload hookspkg.PromptPayload) (hookspkg.PromptPayload, error) {
			record("prompt.post_assemble")
			return payload, nil
		},
		dispatchAgentPreStartFn: func(_ context.Context, payload hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error) {
			record("agent.pre_start")
			return payload, nil
		},
		dispatchAgentSpawnedFn: func(_ context.Context, payload hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error) {
			record("agent.spawned")
			return payload, nil
		},
		dispatchSessionPostCreateFn: func(_ context.Context, payload hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePayload, error) {
			record("session.post_create")
			return payload, nil
		},
		dispatchInputPreSubmitFn: func(_ context.Context, payload hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
			record("input.pre_submit")
			return payload, nil
		},
		dispatchTurnStartFn: func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			record("turn.start")
			return payload, nil
		},
		dispatchTurnEndFn: func(_ context.Context, payload hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error) {
			record("turn.end")
			return payload, nil
		},
		dispatchMessageStartFn: func(_ context.Context, payload hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error) {
			record("message.start")
			return payload, nil
		},
		dispatchMessageDeltaFn: func(_ context.Context, payload hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error) {
			record("message.delta:" + payload.DeltaType)
			return payload, nil
		},
		dispatchMessageEndFn: func(_ context.Context, payload hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error) {
			record("message.end")
			return payload, nil
		},
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			record("event.pre_record:" + payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			record("event.post_record:" + payload.RecordType)
			return payload, nil
		},
		dispatchSessionPreStopFn: func(_ context.Context, payload hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error) {
			record("session.pre_stop")
			return payload, nil
		},
		dispatchAgentStoppedFn: func(_ context.Context, payload hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error) {
			record("agent.stopped")
			return payload, nil
		},
		dispatchSessionPostStopFn: func(_ context.Context, payload hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error) {
			record("session.post_stop")
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	session := createSession(t, h)
	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	want := []string{
		"session.pre_create",
		"prompt.post_assemble",
		"agent.pre_start",
		"agent.spawned",
		"session.post_create",
		"input.pre_submit",
		"turn.start",
		"event.pre_record:user_message",
		"event.post_record:user_message",
		"message.start",
		"message.delta:text",
		"event.pre_record:agent_message",
		"event.post_record:agent_message",
		"message.end",
		"event.pre_record:done",
		"event.post_record:done",
		"turn.end",
		"session.pre_stop",
		"event.pre_record:session_stopped",
		"event.post_record:session_stopped",
		"event.pre_record:transcript_marker.created",
		"event.post_record:transcript_marker.created",
		"agent.stopped",
		"session.post_stop",
	}

	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("hook order = %#v, want %#v", got, want)
	}
}

func TestManagerIntegrationSandboxNativeHooksLifecycleOrder(t *testing.T) {
	var (
		mu        sync.Mutex
		order     []string
		afterTo   = make(chan struct{})
		ready     = make(chan struct{})
		afterFrom = make(chan struct{})
	)
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, event)
	}
	waitFor := func(ctx context.Context, ch <-chan struct{}, label string) error {
		select {
		case <-ch:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return errors.New("timed out waiting for " + label)
		}
	}

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{
			{
				Name:         "env-prepare",
				Event:        hookspkg.HookSandboxPrepare,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-sync-before",
				Event:        hookspkg.HookSandboxSyncBefore,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-sync-after",
				Event:        hookspkg.HookSandboxSyncAfter,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-ready",
				Event:        hookspkg.HookSandboxReady,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "env-stop",
				Event:        hookspkg.HookSandboxStop,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		},
		map[string]hookspkg.Executor{
			"env-prepare": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxPreparePayload) (hookspkg.SandboxPreparePatch, error) {
					if payload.SandboxID == "" || payload.WorkspaceID == "" {
						return hookspkg.SandboxPreparePatch{}, errors.New("sandbox.prepare missing identity fields")
					}
					record("sandbox.prepare")
					return hookspkg.SandboxPreparePatch{}, nil
				},
			),
			"env-sync-before": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxSyncBeforePayload) (hookspkg.SandboxSyncBeforePatch, error) {
					if payload.SandboxID == "" || payload.Direction == "" || payload.Reason == "" {
						return hookspkg.SandboxSyncBeforePatch{}, errors.New(
							"sandbox.sync.before missing lifecycle fields",
						)
					}
					record("sandbox.sync.before:" + payload.Direction)
					return hookspkg.SandboxSyncBeforePatch{}, nil
				},
			),
			"env-sync-after": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxSyncAfterPayload) (hookspkg.SandboxSyncAfterPatch, error) {
					if payload.SandboxID == "" || payload.Direction == "" || payload.DurationMS < 0 {
						return hookspkg.SandboxSyncAfterPatch{}, errors.New(
							"sandbox.sync.after missing lifecycle fields",
						)
					}
					record("sandbox.sync.after:" + payload.Direction)
					switch payload.Direction {
					case string(sandbox.SyncDirectionToRuntime):
						close(afterTo)
					case string(sandbox.SyncDirectionFromRuntime):
						close(afterFrom)
					default:
						return hookspkg.SandboxSyncAfterPatch{}, errors.New(
							"unexpected sync direction " + payload.Direction,
						)
					}
					return hookspkg.SandboxSyncAfterPatch{}, nil
				},
			),
			"env-ready": hookspkg.NewTypedNativeExecutor(
				func(ctx context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxReadyPayload) (hookspkg.SandboxReadyPatch, error) {
					if err := waitFor(ctx, afterTo, "sandbox.sync.after:to_runtime"); err != nil {
						return hookspkg.SandboxReadyPatch{}, err
					}
					if payload.SandboxID == "" || payload.RuntimeRootDir == "" {
						return hookspkg.SandboxReadyPatch{}, errors.New("sandbox.ready missing runtime fields")
					}
					record("sandbox.ready")
					close(ready)
					return hookspkg.SandboxReadyPatch{}, nil
				},
			),
			"env-stop": hookspkg.NewTypedNativeExecutor(
				func(ctx context.Context, _ hookspkg.RegisteredHook, payload hookspkg.SandboxStopPayload) (hookspkg.SandboxStopPatch, error) {
					if err := waitFor(ctx, afterFrom, "sandbox.sync.after:from_runtime"); err != nil {
						return hookspkg.SandboxStopPatch{}, err
					}
					if payload.SandboxID == "" || payload.StopReason == "" {
						return hookspkg.SandboxStopPatch{}, errors.New("sandbox.stop missing stop fields")
					}
					record("sandbox.stop")
					return hookspkg.SandboxStopPatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(HookSet{Sandbox: hooks}))
	session := createSession(t, h)
	if err := waitFor(testutil.Context(t), ready, "sandbox.ready"); err != nil {
		t.Fatalf("waiting for sandbox.ready: %v", err)
	}
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	want := []string{
		"sandbox.prepare",
		"sandbox.sync.before:to_runtime",
		"sandbox.sync.after:to_runtime",
		"sandbox.ready",
		"sandbox.sync.before:from_runtime",
		"sandbox.sync.after:from_runtime",
		"sandbox.stop",
	}
	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("sandbox hook order = %#v, want %#v", got, want)
	}
}

func TestManagerIntegrationContextCompactionUsesPatchedParams(t *testing.T) {
	reason := "patched-reason"
	strategy := "patched-strategy"
	postSeen := make(chan hookspkg.ContextPostCompactPayload, 1)

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{
			{
				Name:         "context-pre",
				Event:        hookspkg.HookContextPreCompact,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			{
				Name:         "context-post",
				Event:        hookspkg.HookContextPostCompact,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		},
		map[string]hookspkg.Executor{
			"context-pre": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPatch, error) {
					return hookspkg.ContextPreCompactPatch{
						Reason:   &reason,
						Strategy: &strategy,
					}, nil
				},
			),
			"context-post": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPatch, error) {
					postSeen <- payload
					return hookspkg.ContextPostCompactPatch{}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	var seen hookspkg.ContextPreCompactPayload
	result, err := h.manager.runContextCompaction(
		testutil.Context(t),
		session,
		"turn-context",
		"manual",
		"noop",
		"",
		nil,
		func(_ context.Context, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			seen = payload
			return hookspkg.ContextPostCompactPayload{
				Summary: "after",
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("runContextCompaction() error = %v", err)
	}
	if seen.Reason != reason || seen.Strategy != strategy {
		t.Fatalf("compactor saw reason/strategy = %q/%q, want %q/%q", seen.Reason, seen.Strategy, reason, strategy)
	}
	if result.Reason != reason || result.Strategy != strategy {
		t.Fatalf("result reason/strategy = %q/%q, want %q/%q", result.Reason, result.Strategy, reason, strategy)
	}
	select {
	case payload := <-postSeen:
		if payload.Reason != reason || payload.Strategy != strategy {
			t.Fatalf(
				"post hook saw reason/strategy = %q/%q, want %q/%q",
				payload.Reason,
				payload.Strategy,
				reason,
				strategy,
			)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for context.post_compact hook")
	}
}

func TestManagerIntegrationPreStopRequiredHookErrorPreventsCleanStop(t *testing.T) {
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "required-pre-stop",
			Event:        hookspkg.HookSessionPreStop,
			Mode:         hookspkg.HookModeSync,
			Required:     true,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"required-pre-stop": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPatch, error) {
					return hookspkg.SessionPreStopPatch{}, errors.New("required hook failed")
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)

	err := h.manager.Stop(testutil.Context(t), session.ID)
	if err == nil {
		t.Fatal("Stop() error = nil, want required pre-stop hook failure")
	}
	if got := session.Info().State; got != StateActive {
		t.Fatalf("session state after failed Stop() = %q, want %q", got, StateActive)
	}
	if _, ok := h.manager.Get(session.ID); !ok {
		t.Fatalf("Get(%q) = missing, want active session after failed stop", session.ID)
	}

	h.manager.hooks = HookSet{}
	if cleanupErr := h.manager.Stop(testutil.Context(t), session.ID); cleanupErr != nil {
		t.Fatalf("cleanup Stop() error = %v", cleanupErr)
	}
}
