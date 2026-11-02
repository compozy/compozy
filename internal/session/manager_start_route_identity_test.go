package session

import (
	"context"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/testutil"
)

func TestAcceptedRouteIdentityIgnoresRuntimeAdapterRewrite(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should keep the accepted route config-derived when the Cursor adapter injects the transport model",
		func(t *testing.T) {
			t.Parallel()

			const cursorModel = "grok-4.5"
			const cursorTransport = "cursor-grok-4.5-high"
			h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
				ProviderID:             cursorRuntimeProvider,
				ModelID:                cursorModel,
				AvailabilityState:      modelcatalog.AvailabilityStateAvailableLive,
				DefaultReasoningEffort: new(modelcatalog.ReasoningEffortHigh),
				TransportBindings: []modelcatalog.ModelTransportBinding{{
					TransportModelID: cursorTransport,
					ReasoningEffort:  new(modelcatalog.ReasoningEffortHigh),
				}},
				Sources: []modelcatalog.SourceRef{{
					SourceID:   modelcatalog.SourceKindProviderLiveID(cursorRuntimeProvider),
					SourceKind: modelcatalog.SourceKindProviderLive,
				}},
			}}}))
			resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
			}
			for index := range resolved.Agents {
				if resolved.Agents[index].Name == "coder" {
					resolved.Agents[index].Provider = cursorRuntimeProvider
					resolved.Agents[index].Model = cursorModel
					resolved.Agents[index].ReasoningEffort = "high"
				}
			}
			h.resolver.upsert(&resolved)

			session, err := h.manager.Create(testutil.Context(t), new(CreateOpts{
				AgentName: "coder",
				Workspace: h.workspaceID,
			}))
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			t.Cleanup(func() { reportSessionStop(t, h, session.ID) })

			if got := h.driver.startCalls[0].Command; !strings.Contains(got, "--model "+cursorTransport) {
				t.Fatalf("start command = %q, want the Cursor transport binding", got)
			}
			record := readMeta(t, session.MetaPath()).AcceptedRoute
			if record == nil {
				t.Fatal("accepted route = nil, want the accepted binding")
			}
			if got, want := record.CommandFingerprint, compozyconfig.CommandFingerprint(
				"cursor-agent acp",
			); got != want {
				t.Fatalf("accepted command fingerprint = %q, want the adapter-free configuration command", got)
			}
		},
	)
}

func TestAgentPreStartHookProviderRouting(t *testing.T) {
	t.Parallel()

	t.Run("Should apply the hook once per launch without changing the inherited command", func(t *testing.T) {
		t.Parallel()

		dispatcher := &spyHookDispatcher{
			dispatchAgentPreStartFn: func(
				_ context.Context,
				payload hookspkg.AgentPreStartPayload,
			) (hookspkg.AgentPreStartPayload, error) {
				payload.Args = append([]string{payload.Command}, payload.Args...)
				payload.Command = "hook-wrapper"
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		cleanupTestManager(t, h.manager)
		created, err := h.manager.CreateAccepted(t.Context(), new(CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		}))
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		events, err := h.manager.Prompt(t.Context(), created.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)

		child, err := h.manager.Spawn(t.Context(), SpawnOpts{
			ParentSessionID: created.ID,
			AgentName:       "coder",
			TTL:             time.Minute,
			ChainOwner:      ChainOwnerCaller,
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}
		if got := len(h.driver.startCalls); got != 2 {
			t.Fatalf("driver start calls = %d, want 2", got)
		}
		for index, start := range h.driver.startCalls {
			if got, want := start.Command, "hook-wrapper test-acp-driver"; got != want {
				t.Fatalf("start %d command = %q, want %q", index, got, want)
			}
			if start.ProviderConfig == nil || start.ProviderConfig.CommandIsBuiltin {
				t.Fatalf("start %d provider config = %#v, want a non-builtin hook command", index, start.ProviderConfig)
			}
		}
		parent, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatalf("Get(%q) did not find the parent", created.ID)
		}
		for _, session := range []*Session{parent, child} {
			if got, want := session.providerRoutingSnapshot().Command, "test-acp-driver"; got != want {
				t.Fatalf("session %q routing command = %q, want %q", session.ID, got, want)
			}
			record := readMeta(t, session.MetaPath()).AcceptedRoute
			if record == nil || record.CommandFingerprint != providerCommandFingerprint("test-acp-driver") {
				t.Fatalf("session %q accepted route = %#v, want the configured command fingerprint", session.ID, record)
			}
		}
	})
}
