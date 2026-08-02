package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestBundledBuiltinSessionFallback(t *testing.T) {
	t.Run("Should create a dream session without a materialized agent definition", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		configureBundledBuiltinFallbackWorkspace(t, h)
		created, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: compozyconfig.BuiltinDreamingCuratorAgentName,
			Provider:  "claude",
			Name:      "bundled-dream",
			Workspace: h.workspaceID,
			Type:      SessionTypeDream,
		})
		if err != nil {
			t.Fatalf("Create(dream) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Fatalf("Stop(%q) error = %v", created.ID, err)
			}
		})

		if got, want := created.AgentName, compozyconfig.BuiltinDreamingCuratorAgentName; got != want {
			t.Fatalf("Create().AgentName = %q, want %q", got, want)
		}
		if got, want := created.Type, SessionTypeDream; got != want {
			t.Fatalf("Create().Type = %q, want %q", got, want)
		}
		if got := h.driver.startCalls[0].SystemPrompt; !strings.Contains(
			got,
			mustBuiltinAgentDef(t, compozyconfig.BuiltinDreamingCuratorAgentName).Prompt,
		) {
			t.Fatalf("startCalls[0].SystemPrompt = %q, want bundled dream prompt", got)
		}
	})

	t.Run("Should reject a builtin identity for a mismatched session type", func(t *testing.T) {
		t.Parallel()

		if _, ok := builtinSessionAgentDef(compozyconfig.BuiltinDreamingCuratorAgentName, SessionTypeUser); ok {
			t.Fatal("builtinSessionAgentDef(dreaming-curator, user) ok = true, want false")
		}
		if _, ok := builtinSessionAgentDef(compozyconfig.BuiltinCoordinatorAgentName, SessionTypeDream); ok {
			t.Fatal("builtinSessionAgentDef(coordinator, dream) ok = true, want false")
		}
	})

	t.Run("Should resolve normalized builtin identities for their session type", func(t *testing.T) {
		t.Parallel()

		def, ok := builtinSessionAgentDef("  DREAMING-CURATOR  ", SessionTypeDream)
		if !ok {
			t.Fatal("builtinSessionAgentDef(normalized dreaming-curator, dream) ok = false, want true")
		}
		if got, want := def.Name, compozyconfig.BuiltinDreamingCuratorAgentName; got != want {
			t.Fatalf("builtinSessionAgentDef().Name = %q, want %q", got, want)
		}
	})

	t.Run("Should create a coordinator session without a materialized agent definition", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		configureBundledBuiltinFallbackWorkspace(t, h)

		created := createBundledCoordinatorSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Fatalf("Stop(%q) error = %v", created.ID, err)
			}
		})

		if got, want := created.AgentName, compozyconfig.BuiltinCoordinatorAgentName; got != want {
			t.Fatalf("Create().AgentName = %q, want %q", got, want)
		}
		if got, want := created.Type, SessionTypeCoordinator; got != want {
			t.Fatalf("Create().Type = %q, want %q", got, want)
		}
		if got, want := len(h.driver.startCalls), 1; got != want {
			t.Fatalf("len(startCalls) = %d, want %d", got, want)
		}
		if got, want := h.driver.startCalls[0].AgentName, compozyconfig.BuiltinCoordinatorAgentName; got != want {
			t.Fatalf("startCalls[0].AgentName = %q, want %q", got, want)
		}
		if got := h.driver.startCalls[0].SystemPrompt; !strings.Contains(
			got,
			mustBuiltinAgentDef(t, compozyconfig.BuiltinCoordinatorAgentName).Prompt,
		) {
			t.Fatalf("startCalls[0].SystemPrompt = %q, want bundled coordinator prompt", got)
		}
	})

	t.Run("Should resume a coordinator session without a materialized agent definition", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		configureBundledBuiltinFallbackWorkspace(t, h)

		ctx := testutil.Context(t)
		created := createBundledCoordinatorSession(t, h)
		if err := h.manager.Stop(ctx, created.ID); err != nil {
			t.Fatalf("Stop(%q) error = %v", created.ID, err)
		}

		resumed, err := h.manager.Resume(ctx, created.ID)
		if err != nil {
			t.Fatalf("Resume(%q) error = %v", created.ID, err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(%q) error = %v", resumed.ID, err)
			}
		})

		if got, want := resumed.AgentName, compozyconfig.BuiltinCoordinatorAgentName; got != want {
			t.Fatalf("Resume().AgentName = %q, want %q", got, want)
		}
		if got, want := len(h.driver.startCalls), 2; got != want {
			t.Fatalf("len(startCalls) after resume = %d, want %d", got, want)
		}
		if got := h.driver.startCalls[1].SystemPrompt; !strings.Contains(
			got,
			mustBuiltinAgentDef(t, compozyconfig.BuiltinCoordinatorAgentName).Prompt,
		) {
			t.Fatalf("startCalls[1].SystemPrompt = %q, want bundled coordinator prompt", got)
		}
	})

	t.Run(
		"Should validate stopped coordinator infrastructure without a materialized agent definition",
		func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			configureBundledBuiltinFallbackWorkspace(t, h)

			meta := validResumeMeta(h, "sess-coordinator-fallback")
			meta.AgentName = compozyconfig.BuiltinCoordinatorAgentName
			meta.Provider = "claude"
			meta.SessionType = string(SessionTypeCoordinator)
			writeResumeEventStore(t, h.homePaths, meta.ID, []byte("not-empty"))

			errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
			if len(errs) != 0 {
				t.Fatalf("validateInfrastructure() errors = %#v, want none", errs)
			}
		},
	)
}

func createBundledCoordinatorSession(t *testing.T, h *harness) *Session {
	t.Helper()

	created, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    compozyconfig.BuiltinCoordinatorAgentName,
		Provider:                     "claude",
		Name:                         "bundled-coordinator",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "coord-fallback-test"),
		Lineage: &store.SessionLineage{
			SpawnRole: string(SessionTypeCoordinator),
			TTLExpiresAt: func() *time.Time {
				ttl := h.manager.now().UTC().Add(time.Hour)
				return &ttl
			}(),
		},
		Type: SessionTypeCoordinator,
	})
	if err != nil {
		t.Fatalf("Create(coordinator) error = %v", err)
	}
	return created
}

func configureBundledBuiltinFallbackWorkspace(t *testing.T, h *harness) {
	t.Helper()

	resolved, err := h.resolver.Resolve(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
	}
	resolved.Config.Defaults.Provider = "claude"
	h.cfg.Defaults.Provider = "claude"
	h.resolver.upsert(&resolved)
	if _, err := resolveWorkspaceAgent(
		compozyconfig.BuiltinCoordinatorAgentName,
		&resolved,
	); !errors.Is(
		err,
		workspacepkg.ErrAgentNotAvailable,
	) {
		t.Fatalf("resolveWorkspaceAgent(coordinator) error = %v, want ErrAgentNotAvailable", err)
	}
}

func mustBuiltinAgentDef(t *testing.T, name string) compozyconfig.AgentDef {
	t.Helper()
	def, ok := compozyconfig.BuiltinAgentDef(name)
	if !ok {
		t.Fatalf("BuiltinAgentDef(%q) ok = false, want true", name)
	}
	return def
}
