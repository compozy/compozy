package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/skills"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestActiveSkillsForHookDeclarations(t *testing.T) {
	t.Parallel()

	t.Run("Should skip invalid agent local skill layers during hook rebuild", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		agentDir := filepath.Join(root, ".agh", "agents", "broken-agent")
		if err := os.MkdirAll(filepath.Join(agentDir, "skills", "broken"), 0o755); err != nil {
			t.Fatalf("MkdirAll(agent skills) error = %v", err)
		}
		agentPath := filepath.Join(agentDir, "AGENT.md")
		if err := os.WriteFile(agentPath, []byte("name: broken-agent\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(AGENT.md) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(agentDir, "skills", "broken", "SKILL.md"),
			[]byte("not-frontmatter"),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile(SKILL.md) error = %v", err)
		}

		registry := skills.NewRegistry(skills.RegistryConfig{})
		resolved := &workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-hooks", RootDir: root},
			Agents: []aghconfig.AgentDef{{
				Name:       "broken-agent",
				SourcePath: agentPath,
			}},
		}

		activeSkills, err := activeSkillsForHookDeclarations(
			context.Background(),
			registry,
			resolved,
			"broken-agent",
			discardLogger(),
		)
		if err != nil {
			t.Fatalf("activeSkillsForHookDeclarations() error = %v", err)
		}
		if len(activeSkills) != 0 {
			t.Fatalf("len(activeSkills) = %d, want 0 for invalid agent-local layer", len(activeSkills))
		}
	})
}
