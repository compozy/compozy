package daemon

import (
	"context"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestStarvationSpawnerResolveAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should pick the lexicographically-first agent that covers the capabilities", func(t *testing.T) {
		t.Parallel()

		spawner := starvationSpawner{
			workspaces: &fakeSpawnWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{Agents: []aghconfig.AgentDef{
					spawnAgentDef("zeta-agent", "go", "sqlite"),
					spawnAgentDef("alpha-agent", "go", "sqlite"),
				}},
			},
			agents: reviewRouterAgentResolverStub{
				"zeta-agent":  spawnAgentDef("zeta-agent", "go", "sqlite"),
				"alpha-agent": spawnAgentDef("alpha-agent", "go", "sqlite"),
			},
		}

		name, ok, err := spawner.resolveAgent(context.Background(), "ws-1", []string{"go"})
		if err != nil {
			t.Fatalf("resolveAgent() error = %v", err)
		}
		if !ok {
			t.Fatal("resolveAgent() ok = false, want a capable agent")
		}
		if got, want := name, "alpha-agent"; got != want {
			t.Fatalf("resolved agent = %q, want lexicographically-first %q", got, want)
		}
	})

	t.Run("Should report unresolvable when no agent covers the capabilities", func(t *testing.T) {
		t.Parallel()

		spawner := starvationSpawner{
			workspaces: &fakeSpawnWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{Agents: []aghconfig.AgentDef{
					spawnAgentDef("docs-agent", "docs"),
				}},
			},
			agents: reviewRouterAgentResolverStub{"docs-agent": spawnAgentDef("docs-agent", "docs")},
		}

		name, ok, err := spawner.resolveAgent(context.Background(), "ws-1", []string{"go"})
		if err != nil {
			t.Fatalf("resolveAgent() error = %v", err)
		}
		if ok {
			t.Fatalf("resolveAgent() ok = true (agent %q); want unresolvable", name)
		}
	})

	t.Run("Should pick the lexicographically-first agent for an empty capability set", func(t *testing.T) {
		t.Parallel()

		spawner := starvationSpawner{
			workspaces: &fakeSpawnWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{Agents: []aghconfig.AgentDef{
					spawnAgentDef("zeta-agent"),
					spawnAgentDef("alpha-agent"),
				}},
			},
			agents: reviewRouterAgentResolverStub{
				"zeta-agent":  spawnAgentDef("zeta-agent"),
				"alpha-agent": spawnAgentDef("alpha-agent"),
			},
		}

		name, ok, err := spawner.resolveAgent(context.Background(), "ws-1", nil)
		if err != nil {
			t.Fatalf("resolveAgent(no caps) error = %v", err)
		}
		if !ok {
			t.Fatal("resolveAgent(no caps) ok = false, want an eligible agent")
		}
		if got, want := name, "alpha-agent"; got != want {
			t.Fatalf("resolved agent = %q, want lexicographically-first %q", got, want)
		}
	})
}

type fakeSpawnWorkspaceResolver struct {
	resolved workspacepkg.ResolvedWorkspace
	err      error
}

func (f *fakeSpawnWorkspaceResolver) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	if f.err != nil {
		return workspacepkg.ResolvedWorkspace{}, f.err
	}
	return f.resolved, nil
}

func spawnAgentDef(name string, capabilities ...string) aghconfig.AgentDef {
	defs := make([]aghconfig.CapabilityDef, 0, len(capabilities))
	for _, capability := range capabilities {
		defs = append(defs, aghconfig.CapabilityDef{ID: capability})
	}
	return aghconfig.AgentDef{
		Name:         name,
		Capabilities: &aghconfig.CapabilityCatalog{Capabilities: defs},
	}
}
