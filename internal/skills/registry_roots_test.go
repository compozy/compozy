package skills

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestWorkspaceResolvedSkillRoots(t *testing.T) {
	t.Parallel()

	t.Run("Should scope every root by the registered workspace ID", func(t *testing.T) {
		t.Parallel()

		resolved := &workspacepkg.ResolvedWorkspace{
			Workspace:   workspacepkg.Workspace{ID: "ws-registered", RootDir: t.TempDir()},
			WorkspaceID: "runtime-workspace-identity",
		}
		roots := workspaceResolvedSkillRoots(resolved)
		if len(roots) == 0 {
			t.Fatal("workspaceResolvedSkillRoots() returned no roots")
		}
		for _, root := range roots {
			if got, want := root.ResourceScope.ID, "ws-registered"; got != want {
				t.Errorf("root %q scope ID = %q, want %q", root.Dir, got, want)
			}
			if got, want := root.WorkspaceID, "ws-registered"; got != want {
				t.Errorf("root %q workspace ID = %q, want %q", root.Dir, got, want)
			}
		}
	})
}

func testGlobalSkillRoots(dir string) []compozyconfig.SkillRootSpec {
	return []compozyconfig.SkillRootSpec{{
		Dir:           dir,
		SourceSlug:    compozyconfig.SkillSourceCompozy,
		Kind:          compozyconfig.RootKindBuiltin,
		ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}}
}
