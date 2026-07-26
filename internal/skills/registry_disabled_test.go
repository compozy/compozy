package skills

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestRegistryForWorkspaceDisabledOverlay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		withWorkspaceSkill bool
	}{
		{name: "Should disable inherited global skills when workspace has no local skills"},
		{name: "Should disable inherited global skills after workspace skill merge", withWorkspaceSkill: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			userDir := filepath.Join(root, "user")
			workspace := filepath.Join(root, "workspace")

			writeSkillFile(
				t,
				userDir,
				filepath.Join("global", skillFileName),
				skillWithDescription("global", "Global skill"),
			)

			resolved := &workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID: "ws-disabled-" + strings.ReplaceAll(tt.name, " ", "-"),
				},
				Config: aghconfig.Config{
					Skills: aghconfig.SkillsConfig{
						DisabledSkills: []string{"global"},
					},
				},
			}
			if tt.withWorkspaceSkill {
				writeSkillFile(
					t,
					filepath.Join(workspace, ".agh", "skills"),
					filepath.Join("local", skillFileName),
					skillWithDescription("local", "Workspace skill"),
				)
				resolved.RootDir = workspace
				resolved.Skills = []workspacepkg.SkillPath{
					resolvedSkillPath(filepath.Join(workspace, ".agh", "skills", "local"), "workspace"),
				}
			}

			registry := newTestRegistry(t, RegistryConfig{
				UserSkillsDir: userDir,
			})
			if err := registry.LoadAll(context.Background()); err != nil {
				t.Fatalf("LoadAll() error = %v", err)
			}

			got, err := registry.ForWorkspace(context.Background(), resolved)
			if err != nil {
				t.Fatalf("ForWorkspace() error = %v", err)
			}

			global := findSkill(t, got, "global")
			if global.Enabled {
				t.Fatalf("global Enabled = true, want workspace disabled overlay applied to %#v", got)
			}
			if tt.withWorkspaceSkill {
				local := findSkill(t, got, "local")
				if !local.Enabled {
					t.Fatal("local Enabled = false, want unrelated workspace skill to stay enabled")
				}
			}
		})
	}
}
