package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) resolveScopedSkills(
	c *gin.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]*skills.Skill, error) {
	if agentName != "" {
		projected, err := h.projectedAgentForSkillScope(c.Request.Context(), resolved, agentName)
		if err != nil {
			return nil, err
		}
		var skillList []*skills.Skill
		if projected == nil {
			skillList, err = h.SkillsRegistry.ForAgent(c.Request.Context(), resolved, agentName)
		} else {
			skillList, err = h.SkillsRegistry.ForAgentDef(c.Request.Context(), resolved, *projected)
		}
		if err != nil {
			return nil, mapSkillScopeError(err)
		}
		return skillList, nil
	}
	return h.SkillsRegistry.ForWorkspace(c.Request.Context(), resolved)
}

func (h *BaseHandlers) projectedAgentForSkillScope(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) (*compozyconfig.AgentDef, error) {
	target := compozyconfig.NormalizeAgentName(agentName)
	if resolved != nil {
		for _, agent := range resolved.Agents {
			if compozyconfig.NormalizeAgentName(agent.Name) == target {
				candidate := compozyconfig.CloneAgentDef(agent)
				return &candidate, nil
			}
		}
	}
	if h.AgentCatalog == nil {
		return nil, nil
	}
	var entries []AgentCatalogEntry
	var err error
	if resolved == nil {
		entries, err = h.AgentCatalog.ListAgents(ctx)
	} else {
		entries, err = h.AgentCatalog.ListAgentsForWorkspace(ctx, resolved)
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if compozyconfig.NormalizeAgentName(entry.Def.Name) == target {
			candidate := compozyconfig.CloneAgentDef(entry.Def)
			return &candidate, nil
		}
	}
	return nil, nil
}

func (h *BaseHandlers) resolveSkillScope(
	c *gin.Context,
) (*workspacepkg.ResolvedWorkspace, string, error) {
	workspace := strings.TrimSpace(c.Query("workspace"))
	agentName, err := skillAgentScope(c)
	if err != nil {
		return nil, "", err
	}
	return h.resolveProfiledSkillScope(c, workspace, agentName)
}

func (h *BaseHandlers) resolveSkillDetailScope(
	c *gin.Context,
) (*workspacepkg.ResolvedWorkspace, string, error) {
	if _, legacyWorkspace := c.GetQuery("workspace"); legacyWorkspace {
		return nil, "", fmt.Errorf("%w: workspace is not valid here; use canonical workspace_id", ErrSkillValidation)
	}
	workspaceID, hasWorkspaceID := c.GetQuery("workspace_id")
	workspaceID = strings.TrimSpace(workspaceID)
	if hasWorkspaceID && workspaceID == "" {
		return nil, "", fmt.Errorf("%w: workspace_id is required", ErrSkillValidation)
	}
	agentName, err := skillAgentScope(c)
	if err != nil {
		return nil, "", err
	}
	resolved, agentName, err := h.resolveProfiledSkillScope(c, workspaceID, agentName)
	if err != nil {
		return nil, "", err
	}
	if hasWorkspaceID && canonicalResolvedWorkspaceID(resolved) != workspaceID {
		return nil, "", fmt.Errorf("%w: workspace_id must be the canonical workspace id", ErrSkillValidation)
	}
	return resolved, agentName, nil
}

func (h *BaseHandlers) resolveProfiledSkillScope(
	c *gin.Context,
	workspaceRef string,
	agentName string,
) (*workspacepkg.ResolvedWorkspace, string, error) {
	selection, err := h.resolveProfileReadSelection(c)
	if err != nil {
		return nil, "", err
	}
	if selection.Scope.AllProfiles {
		return nil, "", fmt.Errorf("%w: skills require exactly one profile", ErrSkillValidation)
	}
	profileName := strings.TrimSpace(selection.ProfileName)
	if profileName == "" {
		profileName, err = h.agentResourceProfileNameForScope(c.Request.Context(), selection.Scope)
		if err != nil {
			return nil, "", err
		}
	}
	if workspaceRef == "" {
		if profileName == "" || profileName == compozyconfig.DefaultProfileDirName {
			return nil, agentName, nil
		}
		return h.profileOnlySkillScope(selection.Scope.ProfileID, profileName), agentName, nil
	}
	if h.Workspaces == nil {
		return nil, "", errors.New("workspace resolver is not configured")
	}
	resolved, err := resolveWorkspaceAgentProfile(c.Request.Context(), h.Workspaces, workspaceRef, profileName)
	if err != nil {
		return nil, "", err
	}
	return &resolved, agentName, nil
}

func (h *BaseHandlers) profileOnlySkillScope(profileID string, profileName string) *workspacepkg.ResolvedWorkspace {
	return &workspacepkg.ResolvedWorkspace{
		ProfileID:   strings.TrimSpace(profileID),
		ProfileName: strings.TrimSpace(profileName),
		ProfileRoot: filepath.Join(h.HomePaths.ProfilesDir, strings.TrimSpace(profileName)),
	}
}

func mapSkillScopeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, skills.ErrAgentNotFound):
		return fmt.Errorf("%w: %v", ErrSkillNotFound, err)
	case errors.Is(err, skills.ErrAgentLocalInvalid):
		return fmt.Errorf("%w: %v", ErrSkillUnprocessable, err)
	default:
		return err
	}
}

func skillAgentScope(c *gin.Context) (string, error) {
	agentName, hasAgent := c.GetQuery("for_agent")
	agentName = strings.TrimSpace(agentName)
	if hasAgent && agentName == "" {
		return "", fmt.Errorf("%w: for_agent is required", ErrSkillValidation)
	}
	if agentName != "" {
		if err := compozyconfig.ValidateAgentName(agentName); err != nil {
			return "", fmt.Errorf("%w: %v", ErrSkillValidation, err)
		}
	}
	return agentName, nil
}
