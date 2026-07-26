package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCoordinatorRoleResolverReturnsBundledDefaultIdentity(t *testing.T) {
	t.Parallel()
	t.Run("Should return the bundled default identity", func(t *testing.T) {
		t.Parallel()

		cfg := defaultCoordinatorResolverConfig(t)
		resolver := coordinatorRoleResolverFor(newRoleResolver(&cfg, nil, nil))

		resolved, err := resolver.ResolveCoordinatorRole(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveCoordinatorRole() error = %v", err)
		}
		if got, want := resolved.AgentName, aghconfig.BuiltinCoordinatorAgentName; got != want {
			t.Fatalf("ResolveCoordinatorRole() AgentName = %q, want %q", got, want)
		}
		if resolved.Enabled {
			t.Fatal("ResolveCoordinatorRole() Enabled = true, want bundled default false")
		}
		if got, want := resolved.TTL, aghconfig.DefaultCoordinatorTTL; got != want {
			t.Fatalf("ResolveCoordinatorRole() TTL = %s, want %s", got, want)
		}
	})
}

func TestCoordinatorRoleResolverPrefersGlobalConfigOverBundledDefault(t *testing.T) {
	t.Parallel()
	t.Run("Should prefer global config over the bundled default", func(t *testing.T) {
		t.Parallel()

		cfg := defaultCoordinatorResolverConfig(t)
		cfg.Roles.Coordinator.Enabled = true
		cfg.Roles.Coordinator.Agent = aghconfig.BuiltinCoordinatorAgentName
		cfg.Roles.Coordinator.Provider = "codex"
		cfg.Roles.Coordinator.Model = "global-model"
		cfg.Roles.Coordinator.TTL = 4 * time.Hour
		cfg.Roles.Coordinator.MaxChildren = 4
		resolver := coordinatorRoleResolverFor(newRoleResolver(&cfg, nil, nil))

		resolved, err := resolver.ResolveCoordinatorRole(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveCoordinatorRole() error = %v", err)
		}
		if !resolved.Enabled {
			t.Fatal("ResolveCoordinatorRole() Enabled = false, want global true")
		}
		if got, want := resolved.AgentName, aghconfig.BuiltinCoordinatorAgentName; got != want {
			t.Fatalf("ResolveCoordinatorRole() AgentName = %q, want %q", got, want)
		}
		if got, want := resolved.Provider, "codex"; got != want {
			t.Fatalf("ResolveCoordinatorRole() Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "global-model"; got != want {
			t.Fatalf("ResolveCoordinatorRole() Model = %q, want %q", got, want)
		}
		if got, want := resolved.TTL, 4*time.Hour; got != want {
			t.Fatalf("ResolveCoordinatorRole() TTL = %s, want %s", got, want)
		}
		if got, want := resolved.MaxChildren, 4; got != want {
			t.Fatalf("ResolveCoordinatorRole() MaxChildren = %d, want %d", got, want)
		}
	})
}

func TestCoordinatorRoleResolverPrefersWorkspaceConfig(t *testing.T) {
	t.Parallel()
	t.Run("Should prefer workspace config", func(t *testing.T) {
		t.Parallel()

		global := defaultCoordinatorResolverConfig(t)
		global.Roles.Coordinator.Enabled = true
		global.Roles.Coordinator.Provider = "claude"
		global.Roles.Coordinator.Model = "global-model"
		global.Roles.Coordinator.TTL = 2 * time.Hour
		global.Roles.Coordinator.MaxChildren = 5

		workspaceCfg := defaultCoordinatorResolverConfig(t)
		workspaceCfg.Roles.Coordinator.Enabled = false
		workspaceCfg.Roles.Coordinator.Agent = aghconfig.BuiltinCoordinatorAgentName
		workspaceCfg.Roles.Coordinator.Provider = "codex"
		workspaceCfg.Roles.Coordinator.Model = "workspace-model"
		workspaceCfg.Roles.Coordinator.TTL = 3 * time.Hour
		workspaceCfg.Roles.Coordinator.MaxChildren = 2

		resolver := coordinatorRoleResolverFor(newRoleResolver(
			&global,
			&coordinatorWorkspaceResolverStub{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-1"},
					Config:    workspaceCfg,
				},
			},
			nil,
		))

		resolved, err := resolver.ResolveCoordinatorRole(context.Background(), "ws-1")
		if err != nil {
			t.Fatalf("ResolveCoordinatorRole(workspace) error = %v", err)
		}
		if resolved.Enabled {
			t.Fatal("ResolveCoordinatorRole() Enabled = true, want workspace false")
		}
		if got, want := resolved.AgentName, aghconfig.BuiltinCoordinatorAgentName; got != want {
			t.Fatalf("ResolveCoordinatorRole() AgentName = %q, want %q", got, want)
		}
		if got, want := resolved.Provider, "codex"; got != want {
			t.Fatalf("ResolveCoordinatorRole() Provider = %q, want %q", got, want)
		}
		if got, want := resolved.Model, "workspace-model"; got != want {
			t.Fatalf("ResolveCoordinatorRole() Model = %q, want %q", got, want)
		}
		if got, want := resolved.TTL, 3*time.Hour; got != want {
			t.Fatalf("ResolveCoordinatorRole() TTL = %s, want %s", got, want)
		}
		if got, want := resolved.MaxChildren, 2; got != want {
			t.Fatalf("ResolveCoordinatorRole() MaxChildren = %d, want %d", got, want)
		}
	})
}

func TestCoordinatorRoleResolverUsesAgentFallbackForProviderModel(t *testing.T) {
	t.Parallel()
	t.Run("Should use the catalog agent for provider and model fallback", func(t *testing.T) {
		t.Parallel()

		cfg := defaultCoordinatorResolverConfig(t)
		cfg.Defaults.Provider = "codex"
		cfg.Roles.Coordinator.Enabled = true
		cfg.Roles.Coordinator.Agent = "custom-coordinator"
		resolver := coordinatorRoleResolverFor(newRoleResolver(
			&cfg,
			nil,
			coordinatorAgentResolverStub{
				agent: aghconfig.AgentDef{
					Name:     "custom-coordinator",
					Provider: "claude",
					Model:    "agent-model",
					Prompt:   "agent fallback",
				},
			},
		))

		resolved, err := resolver.ResolveCoordinatorRole(context.Background(), "")
		if err != nil {
			t.Fatalf("ResolveCoordinatorRole() error = %v", err)
		}
		if got, want := resolved.Provider, "claude"; got != want {
			t.Fatalf("ResolveCoordinatorRole() Provider = %q, want agent fallback %q", got, want)
		}
		if got, want := resolved.Model, "agent-model"; got != want {
			t.Fatalf("ResolveCoordinatorRole() Model = %q, want agent fallback %q", got, want)
		}
	})
}

type coordinatorWorkspaceResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
	err      error
}

func (s *coordinatorWorkspaceResolverStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s.err != nil {
		return workspacepkg.ResolvedWorkspace{}, s.err
	}
	return s.resolved, nil
}

func (s *coordinatorWorkspaceResolverStub) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s.err != nil {
		return workspacepkg.ResolvedWorkspace{}, s.err
	}
	return s.resolved, nil
}

type coordinatorAgentResolverStub struct {
	agent aghconfig.AgentDef
	err   error
}

func (s coordinatorAgentResolverStub) ResolveAgent(
	string,
	*workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	if s.err != nil {
		return aghconfig.AgentDef{}, s.err
	}
	return s.agent, nil
}

func defaultCoordinatorResolverConfig(t *testing.T) aghconfig.Config {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return aghconfig.DefaultWithHome(homePaths)
}
