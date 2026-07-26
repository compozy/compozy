package daemon

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestResourceAgentCatalogLookupReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should keep catalog records immutable after resolve mutation", func(t *testing.T) {
		t.Parallel()

		workspaceID := "ws-refac"
		catalog := newResourceCatalog(cloneAgentDef)
		catalog.Replace(1, []resources.Record[aghconfig.AgentDef]{{
			ID:      "workspace:coder",
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspaceID},
			Version: 1,
			Spec: aghconfig.AgentDef{
				Name:   "coder",
				Prompt: "workspace prompt",
				Tools:  []string{toolspkg.ToolIDToolInfo.String()},
			},
		}})
		resolved := &workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: workspaceID}}
		dependency := agentCatalogDependency(catalog)

		first, err := dependency.ResolveAgent("coder", resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(first) error = %v", err)
		}
		first.Tools[0] = "corrupted"

		second, err := dependency.ResolveAgent("coder", resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(second) error = %v", err)
		}
		if got, want := second.Tools[0], toolspkg.ToolIDToolInfo.String(); got != want {
			t.Fatalf("ResolveAgent(second).Tools[0] = %q, want %q", got, want)
		}
	})
}
