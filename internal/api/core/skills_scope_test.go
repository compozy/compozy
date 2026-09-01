package core

import (
	"context"
	"errors"
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
	globalEntry  AgentCatalogEntry
	profile      map[string]AgentCatalogEntry
	globalErr    error
	workspaceErr error
}

func (c projectedSkillAgentCatalog) ListAgents(context.Context) ([]AgentCatalogEntry, error) {
	if c.globalErr != nil {
		return nil, c.globalErr
	}
	return []AgentCatalogEntry{c.globalEntry}, nil
}

func (c projectedSkillAgentCatalog) ListAgentsForWorkspace(
	_ context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]AgentCatalogEntry, error) {
	if c.workspaceErr != nil {
		return nil, c.workspaceErr
	}
	entry, ok := c.profile[resolved.ProfileID]
	if !ok {
		return nil, nil
	}
	return []AgentCatalogEntry{entry}, nil
}

func (c projectedSkillAgentCatalog) GetAgent(context.Context, string) (AgentCatalogEntry, error) {
	return c.globalEntry, nil
}

func TestResolveScopedSkillsUsesProjectedAgentSourcePath(t *testing.T) {
	t.Parallel()

	globalAgent := projectedSkillAgent(t, "global")
	defaultAgent := projectedSkillAgent(t, "default")
	workAgent := projectedSkillAgent(t, "work")
	catalog := projectedSkillAgentCatalog{
		globalEntry: AgentCatalogEntry{Def: globalAgent},
		profile: map[string]AgentCatalogEntry{
			"profile-default": {Def: defaultAgent},
			"profile-work":    {Def: workAgent},
		},
	}
	tests := []struct {
		name        string
		resolved    *workspacepkg.ResolvedWorkspace
		description string
	}{
		{name: "Should resolve a global Agent without a Workspace", description: "global"},
		{
			name: "Should preserve a non-default Profile without a Workspace",
			resolved: &workspacepkg.ResolvedWorkspace{
				ProfileID: "profile-work", ProfileName: "work", ProfileRoot: t.TempDir(),
			},
			description: "work",
		},
		{
			name: "Should preserve the Workspace and Profile projection",
			resolved: &workspacepkg.ResolvedWorkspace{
				ProfileID: "profile-work", ProfileName: "work",
				Workspace: workspacepkg.Workspace{ID: "workspace-test", RootDir: t.TempDir()},
			},
			description: "work",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handlers := &BaseHandlers{
				SkillsRegistry: skills.NewRegistry(skills.RegistryConfig{}),
				AgentCatalog:   catalog,
			}
			request := httptest.NewRequestWithContext(
				t.Context(), "GET", "/api/skills?for_agent=extension-agent", http.NoBody,
			)
			got, err := handlers.resolveScopedSkills(
				&gin.Context{Request: request}, tt.resolved, "extension-agent",
			)
			if err != nil {
				t.Fatalf("resolveScopedSkills() error = %v", err)
			}
			if len(got) != 1 || got[0].Meta.Name != "sentinel" ||
				got[0].Meta.Description != tt.description || got[0].Source != skills.SourceAgentLocal {
				t.Fatalf("resolveScopedSkills() = %#v, want %q Agent-local sentinel", got, tt.description)
			}
		})
	}

	t.Run("Should return catalog errors", func(t *testing.T) {
		t.Parallel()
		catalogErr := errors.New("catalog unavailable")
		cases := []struct {
			name     string
			resolved *workspacepkg.ResolvedWorkspace
			catalog  projectedSkillAgentCatalog
		}{
			{name: "global", catalog: projectedSkillAgentCatalog{globalErr: catalogErr}},
			{
				name: "Workspace",
				resolved: &workspacepkg.ResolvedWorkspace{
					ProfileID: "profile-work",
					Workspace: workspacepkg.Workspace{ID: "workspace-test", RootDir: t.TempDir()},
				},
				catalog: projectedSkillAgentCatalog{workspaceErr: catalogErr},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				handlers := &BaseHandlers{
					SkillsRegistry: skills.NewRegistry(skills.RegistryConfig{}),
					AgentCatalog:   tc.catalog,
				}
				request := httptest.NewRequestWithContext(
					t.Context(), "GET", "/api/skills?for_agent=extension-agent", http.NoBody,
				)
				_, err := handlers.resolveScopedSkills(
					&gin.Context{Request: request}, tc.resolved, "extension-agent",
				)
				if !errors.Is(err, catalogErr) {
					t.Fatalf("resolveScopedSkills() error = %v, want %v", err, catalogErr)
				}
			})
		}
	})
}

func TestProfileOnlySkillScopePreservesProfileIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should retain both Profile ID and name", func(t *testing.T) {
		t.Parallel()
		handlers := &BaseHandlers{HomePaths: compozyconfig.HomePaths{ProfilesDir: t.TempDir()}}
		resolved := handlers.profileOnlySkillScope("profile-work", "work")
		if resolved.ProfileID != "profile-work" || resolved.ProfileName != "work" {
			t.Fatalf(
				"profileOnlySkillScope() identity = %q/%q, want profile-work/work",
				resolved.ProfileID,
				resolved.ProfileName,
			)
		}
	})
}

func projectedSkillAgent(t *testing.T, description string) compozyconfig.AgentDef {
	t.Helper()
	agentDir := filepath.Join(t.TempDir(), "agents", "extension-agent")
	if err := os.MkdirAll(filepath.Join(agentDir, "skills", "sentinel"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	agentPath := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte("---\nname: extension-agent\n---\nagent\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(agent) error = %v", err)
	}
	skill := "---\nname: sentinel\ndescription: " + description + "\n---\nbody\n"
	if err := os.WriteFile(
		filepath.Join(agentDir, "skills", "sentinel", "SKILL.md"),
		[]byte(skill),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile(skill) error = %v", err)
	}
	return compozyconfig.AgentDef{Name: "extension-agent", SourcePath: agentPath}
}
