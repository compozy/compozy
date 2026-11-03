package speccycle

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

// Suite owns the published resource inventory, agent kinds and portable payload contract.
func TestEmbeddedResources(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve the four published agent kinds and the stays-on level", func(t *testing.T) {
		t.Parallel()
		for name, kind := range map[string]string{"orchestrator": "always_on", "spec_conductor": "on_demand", "code_implementer": "on_demand", "reviewer": "on_demand"} {
			data, err := fs.ReadFile(FS(), "agents/"+name+"/AGENT.md")
			if err != nil {
				t.Fatal(err)
			}
			agent, err := compozyconfig.ParseAgentDef(data)
			if err != nil {
				t.Fatal(err)
			}
			if agent.Name != name || agent.Kind != kind {
				t.Fatalf("agent = %s/%s, want %s/%s", agent.Name, agent.Kind, name, kind)
			}
			// #1200: shipped stays-on agents run unattended at auto, which degrades
			// to approve-edits on a provider without a reviewer mode.
			if kind == "stays_on" && agent.Permissions != string(compozyconfig.PermissionModeAuto) {
				t.Fatalf("%s permissions = %q, want auto", name, agent.Permissions)
			}
			// A built-in reads as part of the app: an icon without its plate, never a generated face.
			appearance, err := agent.ResolveAppearance()
			if err != nil || appearance["avatar"] != "icon" || appearance["background"] != "none" {
				t.Fatalf("%s appearance = %#v, %v", name, appearance, err)
			}
		}
	})
	t.Run("Should publish exactly four agents two skills and no executable surfaces", func(t *testing.T) {
		t.Parallel()
		manifest, err := extensionpkg.LoadManifest(".")
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Resources.Tools) != 0 || len(manifest.Resources.Loops) != 0 ||
			slices.Contains(manifest.Capabilities.Provides, "tool.provider") {
			t.Fatal("resource-only extension declares executable surfaces")
		}
		for dir, want := range map[string][]string{"agents": {"code_implementer", "orchestrator", "reviewer", "spec_conductor"}, "skills": {"cy-create-spec", "git-rebase"}} {
			entries, err := fs.ReadDir(FS(), dir)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			if !slices.Equal(names, want) {
				t.Fatalf("%s = %v, want %v", dir, names, want)
			}
		}
		if _, err := fs.Stat(FS(), "loops"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("loops stat = %v, want absent", err)
		}
	})
	t.Run("Should keep shipped agents and skills portable", func(t *testing.T) {
		t.Parallel()
		err := fs.WalkDir(FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasPrefix(path, "agents/") && !strings.HasPrefix(path, "skills/") {
				return nil
			}
			data, err := fs.ReadFile(FS(), path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"CodeRabbit", "Greptile", "React Doctor", "make gate", "gpt-", "claude-", "gh pr"} {
				if strings.Contains(string(data), forbidden) {
					t.Errorf("%s contains project-specific %q", path, forbidden)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}
