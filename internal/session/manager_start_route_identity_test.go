package session

import (
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
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

			session, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder",
				Workspace: h.workspaceID,
			})
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
