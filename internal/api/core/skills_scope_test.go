package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type projectedSkillAgentCatalog struct {
	entry AgentCatalogEntry
}

func (c projectedSkillAgentCatalog) ListAgents(context.Context) ([]AgentCatalogEntry, error) {
	return []AgentCatalogEntry{c.entry}, nil
}

func (c projectedSkillAgentCatalog) ListAgentsForWorkspace(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
) ([]AgentCatalogEntry, error) {
	return []AgentCatalogEntry{c.entry}, nil
}

func (c projectedSkillAgentCatalog) GetAgent(context.Context, string) (AgentCatalogEntry, error) {
	return c.entry, nil
}

func TestResolveScopedSkillsUsesProjectedAgentSourcePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	agentDir := filepath.Join(root, "agents", "extension-agent")
	if err := os.MkdirAll(filepath.Join(agentDir, "skills", "sentinel"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	agentPath := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte("---\nname: extension-agent\n---\nagent\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(agent) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(agentDir, "skills", "sentinel", "SKILL.md"),
		[]byte("---\nname: sentinel\ndescription: projected\n---\nbody\n"),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile(skill) error = %v", err)
	}

	resolved := &workspacepkg.ResolvedWorkspace{
		ProfileID: "profile-work", ProfileName: "work",
		Workspace: workspacepkg.Workspace{ID: "workspace-test", RootDir: root},
	}
	handlers := &BaseHandlers{
		SkillsRegistry: skills.NewRegistry(skills.RegistryConfig{}),
		AgentCatalog: projectedSkillAgentCatalog{entry: AgentCatalogEntry{
			Def: compozyconfig.AgentDef{Name: "extension-agent", SourcePath: agentPath},
		}},
	}

	request := httptest.NewRequestWithContext(
		context.Background(), "GET", "/api/skills?for_agent=extension-agent", http.NoBody,
	)
	got, err := handlers.resolveScopedSkills(&gin.Context{Request: request}, resolved, "extension-agent")
	if err != nil {
		t.Fatalf("resolveScopedSkills() error = %v", err)
	}
	if len(got) != 1 || got[0].Meta.Name != "sentinel" || got[0].Source != skills.SourceAgentLocal {
		t.Fatalf("resolveScopedSkills() = %#v, want projected Agent-local sentinel", got)
	}
}
