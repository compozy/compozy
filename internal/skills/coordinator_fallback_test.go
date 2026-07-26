package skills

import (
	"context"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestBundledCoordinatorFallback(t *testing.T) {
	t.Run(
		"Should return the base skill set for the bundled coordinator without a materialized agent",
		func(t *testing.T) {
			t.Parallel()

			registry := newTestRegistry(t, RegistryConfig{
				BundledFS: bundledSkillFS(map[string]string{
					"review": "Bundled review skill",
				}),
			})
			if err := registry.LoadAll(context.Background()); err != nil {
				t.Fatalf("LoadAll() error = %v", err)
			}

			resolved := &workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-coordinator",
					RootDir: t.TempDir(),
				},
				Agents: []aghconfig.AgentDef{{
					Name:     "coder",
					Provider: "claude",
					Prompt:   "Coder prompt.",
				}},
			}

			skills, err := registry.ForAgent(context.Background(), resolved, aghconfig.BuiltinCoordinatorAgentName)
			if err != nil {
				t.Fatalf("ForAgent(coordinator) error = %v", err)
			}
			if got, want := len(skills), 1; got != want {
				t.Fatalf("len(skills) = %d, want %d", got, want)
			}
			if got, want := skills[0].Meta.Name, "review"; got != want {
				t.Fatalf("skills[0].Meta.Name = %q, want %q", got, want)
			}
			if got, want := skills[0].Source, SourceBundled; got != want {
				t.Fatalf("skills[0].Source = %v, want %v", got, want)
			}
		},
	)

	t.Run(
		"Should build the prompt agent section for the bundled coordinator without a materialized agent",
		func(t *testing.T) {
			t.Parallel()

			registry := newTestRegistry(t, RegistryConfig{
				BundledFS: bundledSkillFS(map[string]string{
					"review": "Bundled review skill",
				}),
			})
			if err := registry.LoadAll(context.Background()); err != nil {
				t.Fatalf("LoadAll() error = %v", err)
			}

			provider := NewCatalogProvider(registry)
			resolved := &workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-coordinator",
					RootDir: t.TempDir(),
				},
			}

			section, err := provider.PromptAgentSection(
				context.Background(),
				mustBuiltinAgentDef(t, aghconfig.BuiltinCoordinatorAgentName),
				resolved,
			)
			if err != nil {
				t.Fatalf("PromptAgentSection() error = %v", err)
			}
			if !strings.Contains(section, `<skill name="review">`) {
				t.Fatalf("PromptAgentSection() = %q, want bundled review skill", section)
			}
		},
	)
}

func mustBuiltinAgentDef(t *testing.T, name string) aghconfig.AgentDef {
	t.Helper()
	def, ok := aghconfig.BuiltinAgentDef(name)
	if !ok {
		t.Fatalf("BuiltinAgentDef(%q) ok = false, want true", name)
	}
	return def
}
