package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

var testCmdPaletteProfileLens = cmdpalette.ScopedProfileLens(cmdpalette.DefaultProfileLensID, "default")

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

		bindings, aliases, err := resolver.Bindings(t.Context(), testCmdPaletteProfileLens, "workspace-a")
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
		bindings, aliases, err = resolver.Bindings(t.Context(), testCmdPaletteProfileLens, "workspace-a")
		if err != nil {
			t.Fatalf("Bindings(second) error = %v", err)
		}
		enabled, err := resolver.PersonalizationEnabled(t.Context(), testCmdPaletteProfileLens, "workspace-a")
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

	t.Run("Should wrap workspace resolution failures", func(t *testing.T) {
		t.Parallel()
		want := errors.New("workspace missing")
		resolver := &cmdPaletteBindingsResolver{
			workspaces: &cmdPaletteWorkspaceResolverStub{err: want},
			loadGlobal: func() (compozyconfig.Config, error) {
				return compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()}), nil
			},
			catalog: func() cmdpalette.BindableCatalog {
				return cmdPaletteBindableCatalogStub{}
			},
		}
		_, _, err := resolver.Bindings(t.Context(), testCmdPaletteProfileLens, "workspace-a")
		if !errors.Is(err, want) {
			t.Fatalf("Bindings() error = %v, want wrapped %v", err, want)
		}
	})

	t.Run("Should wrap global config loader failures", func(t *testing.T) {
		t.Parallel()
		want := errors.New("global config missing")
		resolver := &cmdPaletteBindingsResolver{
			workspaces: &cmdPaletteWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "workspace-a", RootDir: t.TempDir()},
				WorkspaceID: "workspace-a",
			}},
			loadGlobal: func() (compozyconfig.Config, error) {
				return compozyconfig.Config{}, want
			},
			catalog: func() cmdpalette.BindableCatalog {
				return cmdPaletteBindableCatalogStub{}
			},
		}
		_, _, err := resolver.Bindings(t.Context(), testCmdPaletteProfileLens, "workspace-a")
		if !errors.Is(err, want) {
			t.Fatalf("Bindings() error = %v, want wrapped %v", err, want)
		}
	})

	t.Run("Should wrap invalid workspace overlay validation", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		configDir := filepath.Join(root, compozyconfig.DirName)
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		configPath := filepath.Join(configDir, compozyconfig.ConfigName)
		configData := []byte("[cmd_palette.aliases]\n\"session.new\" = \"bad alias\"\n")
		if err := os.WriteFile(configPath, configData, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		resolver := &cmdPaletteBindingsResolver{
			workspaces: &cmdPaletteWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "workspace-a", RootDir: root},
				WorkspaceID: "workspace-a",
			}},
			loadGlobal: func() (compozyconfig.Config, error) {
				return compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()}), nil
			},
			catalog: func() cmdpalette.BindableCatalog {
				return cmdPaletteBindableCatalogStub{}
			},
		}
		_, _, err := resolver.Bindings(t.Context(), testCmdPaletteProfileLens, "workspace-a")
		if err == nil {
			t.Fatal("Bindings() error = nil, want overlay validation failure")
		}
		validationErr, ok := errors.AsType[compozyconfig.ValidationError](err)
		if !ok || validationErr.Path == "" {
			t.Fatalf("Bindings() error = %v, want wrapped ValidationError", err)
		}
	})
}

type cmdPaletteWorkspaceResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
	err      error
}

func (s *cmdPaletteWorkspaceResolverStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s.err != nil {
		return workspacepkg.ResolvedWorkspace{}, s.err
	}
	return s.resolved, nil
}

type cmdPaletteBindableCatalogStub struct {
	ids []cmdpalette.CommandID
}

func (s cmdPaletteBindableCatalogStub) BindableIDs(
	context.Context,
	cmdpalette.ProfileLens,
	cmdpalette.WorkspaceID,
) ([]cmdpalette.CommandID, error) {
	return append([]cmdpalette.CommandID(nil), s.ids...), nil
}
