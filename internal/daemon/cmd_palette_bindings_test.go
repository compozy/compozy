package daemon

import (
	"context"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestCmdPaletteBindingsResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should reload global bindings aliases and personalization on every read", func(t *testing.T) {
		t.Parallel()

		global := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()})
		global.WindowManager.Shortcuts["session.new"] = windowmanager.ShortcutBinding{"meta+KeyN"}
		global.CmdPalette.Aliases["session.new"] = "new"
		root := t.TempDir()
		resolver := &cmdPaletteBindingsResolver{
			workspaces: &cmdPaletteWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "workspace-a", RootDir: root},
				WorkspaceID: "workspace-a",
			}},
			loadGlobal: func() (compozyconfig.Config, error) {
				return compozyconfig.CloneConfig(&global), nil
			},
			catalog: func() cmdpalette.BindableCatalog {
				ids := make([]cmdpalette.CommandID, 0, len(windowmanager.DefaultKeymap()))
				for id := range windowmanager.DefaultKeymap() {
					ids = append(ids, cmdpalette.CommandID(id))
				}
				return cmdPaletteBindableCatalogStub{ids: ids}
			},
		}

		bindings, aliases, err := resolver.Bindings(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Bindings(first) error = %v", err)
		}
		if !slices.Equal(bindings["session.new"], []string{"meta+KeyN"}) || aliases["session.new"] != "new" {
			t.Fatalf(
				"Bindings(first) = %q/%q, want current global values",
				bindings["session.new"],
				aliases["session.new"],
			)
		}

		global.WindowManager.Shortcuts["session.new"] = windowmanager.ShortcutBinding{"meta+alt+shift+KeyQ"}
		global.CmdPalette.Aliases["session.new"] = "create"
		global.CmdPalette.Personalization = false
		bindings, aliases, err = resolver.Bindings(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Bindings(second) error = %v", err)
		}
		enabled, err := resolver.PersonalizationEnabled(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("PersonalizationEnabled() error = %v", err)
		}
		if !slices.Equal(bindings["session.new"], []string{"meta+alt+shift+KeyQ"}) ||
			aliases["session.new"] != "create" || enabled {
			t.Fatalf(
				"live values = %q/%q/%v, want updated binding, alias, and disabled personalization",
				bindings["session.new"],
				aliases["session.new"],
				enabled,
			)
		}
	})
}

type cmdPaletteWorkspaceResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
}

func (s *cmdPaletteWorkspaceResolverStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return s.resolved, nil
}

type cmdPaletteBindableCatalogStub struct {
	ids []cmdpalette.CommandID
}

func (s cmdPaletteBindableCatalogStub) BindableIDs(
	context.Context,
	cmdpalette.WorkspaceID,
) ([]cmdpalette.CommandID, error) {
	return append([]cmdpalette.CommandID(nil), s.ids...), nil
}
