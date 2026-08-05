package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

const skillScopeAction = "skill.scope"

func (h *BaseHandlers) resolveSkill(
	c *gin.Context,
	name string,
) (*skills.Skill, error) {
	resolved, agentName, err := h.resolveSkillScope(c)
	if err != nil {
		return nil, err
	}

	skillList, err := h.resolveScopedSkills(c, resolved, agentName)
	if err != nil {
		return nil, err
	}
	for _, skill := range skillList {
		if skill != nil && skill.Meta.Name == name {
			return skill, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", ErrSkillNotFound, name)
}

func (h *BaseHandlers) resolveScopedSkills(
	c *gin.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]*skills.Skill, error) {
	if agentName != "" {
		skillList, err := h.SkillsRegistry.ForAgent(c.Request.Context(), resolved, agentName)
		if err != nil {
			return nil, mapSkillScopeError(err)
		}
		return skillList, nil
	}
	if resolved != nil {
		return h.SkillsRegistry.ForWorkspace(c.Request.Context(), resolved)
	}
	return h.SkillsRegistry.ForWorkspace(c.Request.Context(), nil)
}

func (h *BaseHandlers) resolveSkillScope(
	c *gin.Context,
) (*workspacepkg.ResolvedWorkspace, string, error) {
	workspace := strings.TrimSpace(c.Query("workspace"))
	agentName, hasAgent := c.GetQuery("for_agent")
	agentName = strings.TrimSpace(agentName)
	if hasAgent && agentName == "" {
		return nil, "", fmt.Errorf("%w: for_agent is required", ErrSkillValidation)
	}
	if agentName != "" {
		if err := compozyconfig.ValidateAgentName(agentName); err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrSkillValidation, err)
		}
	}

	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, err := h.resolveAgentCallerForWorkspace(
			c.Request.Context(),
			credentials,
			skillScopeAction,
			workspace,
		)
		if err != nil {
			return nil, "", err
		}
		if agentName != "" && agentName != caller.Session.AgentName {
			return nil, "", fmt.Errorf(
				"%w: managed session agent %q cannot request skill scope for %q",
				agentidentity.ErrIdentityUnauthorized,
				caller.Session.AgentName,
				agentName,
			)
		}
		agentName = caller.Session.AgentName
		if workspace == "" {
			workspace = caller.Session.WorkspaceID
		}
	}

	if workspace == "" {
		return nil, agentName, nil
	}
	if h.Workspaces == nil {
		return nil, "", fmt.Errorf(
			"skills: workspace resolver is not configured: %w",
			workspacepkg.ErrWorkspaceResolverUnavailable,
		)
	}
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspace)
	if err != nil {
		return nil, "", err
	}
	return &resolved, agentName, nil
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
