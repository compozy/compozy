package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type projectedSkillAgentCatalog struct {
	globalEntry       AgentCatalogEntry
	defaultEntry      *AgentCatalogEntry
	profiles          map[string]AgentCatalogEntry
	workspaceProfiles map[string]AgentCatalogEntry
	globalErr         error
	workspaceErr      error
}

func (c projectedSkillAgentCatalog) ListAgents(context.Context) ([]AgentCatalogEntry, error) {
	if c.globalErr != nil {
		return nil, c.globalErr
	}
	if c.defaultEntry != nil {
		return []AgentCatalogEntry{*c.defaultEntry}, nil
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
	if entry, ok := c.workspaceProfiles[resolved.ID+"@pf:"+resolved.ProfileName]; ok {
		return []AgentCatalogEntry{entry}, nil
	}
	if entry, ok := c.profiles[resolved.ProfileID]; ok {
		return []AgentCatalogEntry{entry}, nil
	}
	if c.defaultEntry != nil {
		return []AgentCatalogEntry{*c.defaultEntry}, nil
	}
	return []AgentCatalogEntry{c.globalEntry}, nil
}

func (c projectedSkillAgentCatalog) GetAgent(context.Context, string) (AgentCatalogEntry, error) {
	return c.globalEntry, nil
}

func TestResolveScopedSkillsUsesProjectedAgentSourcePath(t *testing.T) {
	t.Parallel()

	globalAgent := projectedSkillAgent(t, "global winner", "global body")
	defaultAgent := projectedSkillAgent(t, "default Profile winner", "default Profile body")
	workAgent := projectedSkillAgent(t, "work Profile winner", "work Profile body")
	workspaceProfileAgent := projectedSkillAgent(t, "Workspace and Profile winner", "Workspace and Profile body")
	tests := []struct {
		name     string
		resolved *workspacepkg.ResolvedWorkspace
		catalog  projectedSkillAgentCatalog
		want     projectedSkillAgentFixture
	}{
		{
			name: "Should use the global Agent when it is the only candidate",
			catalog: projectedSkillAgentCatalog{
				globalEntry: globalAgent.entry,
			},
			want: globalAgent,
		},
		{
			name: "Should let the default Profile Agent override the global Agent",
			catalog: projectedSkillAgentCatalog{
				globalEntry:  globalAgent.entry,
				defaultEntry: &defaultAgent.entry,
			},
			want: defaultAgent,
		},
		{
			name: "Should let a non-default Profile Agent override the default Profile Agent",
			resolved: &workspacepkg.ResolvedWorkspace{
				ProfileID: "profile-work", ProfileName: "work", ProfileRoot: t.TempDir(),
			},
			catalog: projectedSkillAgentCatalog{
				globalEntry:  globalAgent.entry,
				defaultEntry: &defaultAgent.entry,
				profiles: map[string]AgentCatalogEntry{
					"profile-work": workAgent.entry,
				},
			},
			want: workAgent,
		},
		{
			name: "Should let a Workspace and Profile Agent override every lower layer",
			resolved: &workspacepkg.ResolvedWorkspace{
				ProfileID: "profile-work", ProfileName: "work",
				Workspace: workspacepkg.Workspace{ID: "workspace-test", RootDir: t.TempDir()},
			},
			catalog: projectedSkillAgentCatalog{
				globalEntry:  globalAgent.entry,
				defaultEntry: &defaultAgent.entry,
				profiles: map[string]AgentCatalogEntry{
					"profile-work": workAgent.entry,
				},
				workspaceProfiles: map[string]AgentCatalogEntry{
					"workspace-test@pf:work": workspaceProfileAgent.entry,
				},
			},
			want: workspaceProfileAgent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handlers := &BaseHandlers{
				SkillsRegistry: skills.NewRegistry(skills.RegistryConfig{}),
				AgentCatalog:   tt.catalog,
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
			if len(got) != 1 {
				t.Fatalf("len(resolveScopedSkills()) = %d, want 1 (%#v)", len(got), got)
			}
			if got[0].Meta.Name != "sentinel" || got[0].Meta.Description != tt.want.description ||
				got[0].Source != skills.SourceAgentLocal || got[0].FilePath != tt.want.skillPath {
				t.Fatalf(
					"resolveScopedSkills() = %#v, want %q from %q",
					got,
					tt.want.description,
					tt.want.skillPath,
				)
			}
			content, err := handlers.SkillsRegistry.LoadContent(t.Context(), got[0])
			if err != nil {
				t.Fatalf("SkillsRegistry.LoadContent() error = %v", err)
			}
			if got := strings.TrimSpace(content); got != tt.want.body {
				t.Fatalf("SkillsRegistry.LoadContent() = %q, want %q", got, tt.want.body)
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

type projectedSkillAgentFixture struct {
	entry       AgentCatalogEntry
	description string
	body        string
	skillPath   string
}

func projectedSkillAgent(t *testing.T, description string, body string) projectedSkillAgentFixture {
	t.Helper()
	agentDir := filepath.Join(t.TempDir(), "agents", "extension-agent")
	if err := os.MkdirAll(filepath.Join(agentDir, "skills", "sentinel"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	agentPath := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte("---\nname: extension-agent\n---\nagent\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(agent) error = %v", err)
	}
	skillPath := filepath.Join(agentDir, "skills", "sentinel", "SKILL.md")
	skill := "---\nname: sentinel\ndescription: " + description + "\n---\n" + body + "\n"
	if err := os.WriteFile(
		skillPath,
		[]byte(skill),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile(skill) error = %v", err)
	}
	return projectedSkillAgentFixture{
		entry:       AgentCatalogEntry{Def: compozyconfig.AgentDef{Name: "extension-agent", SourcePath: agentPath}},
		description: description,
		body:        body,
		skillPath:   skillPath,
	}
}
