package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/skills"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

// ListSkills returns skills for the selected workspace or global scope.
func (h *BaseHandlers) ListSkills(c *gin.Context) {
	if h.SkillsRegistry == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: skills registry is not configured", h.transportName()),
		)
		return
	}

	resolved, agentName, err := h.resolveSkillScope(c)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	skillList, err := h.resolveScopedSkills(c, resolved, agentName)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SkillsResponse{Skills: SkillPayloadsFromSkills(skillList)})
}

// GetSkill returns one skill by name.
func (h *BaseHandlers) GetSkill(c *gin.Context) {
	if h.SkillsRegistry == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: skills registry is not configured", h.transportName()),
		)
		return
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: skill name is required", ErrSkillValidation))
		return
	}

	skill, err := h.resolveSkill(c, name)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SkillResponse{Skill: SkillPayloadFromSkill(skill)})
}

// GetSkillContent returns the explicit body for one skill.
func (h *BaseHandlers) GetSkillContent(c *gin.Context) {
	if h.SkillsRegistry == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: skills registry is not configured", h.transportName()),
		)
		return
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: skill name is required", ErrSkillValidation))
		return
	}

	skill, err := h.resolveSkill(c, name)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	content, err := h.SkillsRegistry.LoadContent(c.Request.Context(), skill)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, fmt.Errorf("load skill content %q: %w", name, err))
		return
	}

	c.JSON(http.StatusOK, contract.SkillContentResponse{Content: content})
}

// GetSkillShadows returns every declaration involved in resolving one skill.
func (h *BaseHandlers) GetSkillShadows(c *gin.Context) {
	if h.SkillsRegistry == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: skills registry is not configured", h.transportName()),
		)
		return
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: skill name is required", ErrSkillValidation))
		return
	}

	skill, err := h.resolveSkill(c, name)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	shadows, ok := skills.ShadowsForSkill(skill, h.Now())
	if !ok {
		h.respondError(c, StatusForSkillError(ErrSkillNotFound), fmt.Errorf("%w: %q", ErrSkillNotFound, name))
		return
	}

	c.JSON(http.StatusOK, SkillShadowsResponseFromDomain(shadows))
}

// EnableSkill enables a skill by name.
func (h *BaseHandlers) EnableSkill(c *gin.Context) {
	if h.SkillsRegistry == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: skills registry is not configured", h.transportName()),
		)
		return
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: skill name is required", ErrSkillValidation))
		return
	}

	resolved, agentName, err := h.resolveSkillScope(c)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	if agentName != "" {
		if err := h.SkillsRegistry.SetEnabledForAgent(name, resolved, agentName, true); err != nil {
			h.respondError(
				c,
				StatusForSkillError(mapSkillScopeError(err)),
				fmt.Errorf("enable skill %q: %w", name, err),
			)
			return
		}
		h.Logger.Info("skills: enable skill", "name", name, "agent_name", agentName)
		c.JSON(http.StatusOK, contract.SkillActionResponse{OK: true})
		return
	}

	skill, err := h.resolveSkill(c, name)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	if skill != nil && skill.Enabled {
		c.JSON(http.StatusOK, contract.SkillActionResponse{OK: true})
		return
	}

	if err := h.SkillsRegistry.SetEnabled(name, resolved, true); err != nil {
		h.respondError(c, http.StatusInternalServerError, fmt.Errorf("enable skill %q: %w", name, err))
		return
	}

	h.Logger.Info("skills: enable skill", "name", name)
	c.JSON(http.StatusOK, contract.SkillActionResponse{OK: true})
}

// DisableSkill disables a skill by name.
func (h *BaseHandlers) DisableSkill(c *gin.Context) {
	if h.SkillsRegistry == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: skills registry is not configured", h.transportName()),
		)
		return
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: skill name is required", ErrSkillValidation))
		return
	}

	resolved, agentName, err := h.resolveSkillScope(c)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	if agentName != "" {
		if err := h.SkillsRegistry.SetEnabledForAgent(name, resolved, agentName, false); err != nil {
			h.respondError(
				c,
				StatusForSkillError(mapSkillScopeError(err)),
				fmt.Errorf("disable skill %q: %w", name, err),
			)
			return
		}
		h.Logger.Info("skills: disable skill", "name", name, "agent_name", agentName)
		c.JSON(http.StatusOK, contract.SkillActionResponse{OK: true})
		return
	}

	skill, err := h.resolveSkill(c, name)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	if skill != nil && !skill.Enabled {
		c.JSON(http.StatusOK, contract.SkillActionResponse{OK: true})
		return
	}

	if err := h.SkillsRegistry.SetEnabled(name, resolved, false); err != nil {
		h.respondError(c, http.StatusInternalServerError, fmt.Errorf("disable skill %q: %w", name, err))
		return
	}

	h.Logger.Info("skills: disable skill", "name", name)
	c.JSON(http.StatusOK, contract.SkillActionResponse{OK: true})
}

// InstallSkillMarketplace installs one remote marketplace skill.
func (h *BaseHandlers) InstallSkillMarketplace(c *gin.Context) {
	var req contract.SkillMarketplaceInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("decode skill marketplace install request: %w", err))
		return
	}

	result, err := h.skillMarketplaceService().Install(
		c.Request.Context(),
		req.Slug,
		req.Version,
	)
	if err != nil {
		h.respondError(c, StatusForSkillMarketplaceError(err), err)
		return
	}
	if err := h.syncSkillsAfterMarketplaceMutation(c); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := skillmarketplace.VerifyInstallVisible(h.SkillsRegistry, result); err != nil {
		h.logSkillMarketplaceInstallVerificationFailure(result, err)
		h.respondError(c, StatusForSkillMarketplaceError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SkillMarketplaceInstallResponse{
		Skill: SkillMarketplaceInstallPayloadFromResult(result),
	})
}

// UpdateSkillMarketplace checks or applies updates for marketplace skills.
func (h *BaseHandlers) UpdateSkillMarketplace(c *gin.Context) {
	var req contract.SkillMarketplaceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("decode skill marketplace update request: %w", err))
		return
	}

	results, err := h.skillMarketplaceService().Update(c.Request.Context(), skillmarketplace.UpdateRequest{
		Name:      req.Name,
		All:       req.All,
		CheckOnly: req.CheckOnly,
	})
	if err != nil {
		h.respondError(c, StatusForSkillMarketplaceError(err), err)
		return
	}
	if !req.CheckOnly {
		if err := h.syncSkillsAfterMarketplaceMutation(c); err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
	}

	c.JSON(http.StatusOK, contract.SkillMarketplaceUpdateResponse{
		Skills: SkillMarketplaceUpdatePayloadsFromResults(results),
	})
}

// RemoveSkillMarketplace removes one installed marketplace skill.
func (h *BaseHandlers) RemoveSkillMarketplace(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%w: skill name is required", skillmarketplace.ErrValidation),
		)
		return
	}
	result, err := h.skillMarketplaceService().Remove(c.Request.Context(), name)
	if err != nil {
		h.respondError(c, StatusForSkillMarketplaceError(err), err)
		return
	}
	if err := h.syncSkillsAfterMarketplaceMutation(c); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, contract.SkillMarketplaceRemoveResponse{
		Skill: SkillMarketplaceRemovePayloadFromResult(result),
	})
}

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
		if err := aghconfig.ValidateAgentName(agentName); err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrSkillValidation, err)
		}
	}

	if workspace == "" {
		return nil, agentName, nil
	}
	if h.Workspaces == nil {
		return nil, "", errors.New("workspace resolver is not configured")
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

func (h *BaseHandlers) skillMarketplaceService() SkillMarketplaceService {
	if h.SkillMarketplace != nil {
		return h.SkillMarketplace
	}
	return skillmarketplace.NewService(
		h.HomePaths,
		h.Config.Skills,
		skillmarketplace.WithLogger(h.Logger),
		skillmarketplace.WithNow(h.Now),
	)
}

func (h *BaseHandlers) refreshSkillsAfterMarketplaceMutation(c *gin.Context) error {
	if h.SkillsRegistry == nil {
		return fmt.Errorf("%s: skills registry is not configured", h.transportName())
	}
	refresher, ok := h.SkillsRegistry.(SkillsRegistryRefresher)
	if !ok {
		return fmt.Errorf("%s: skills registry refresh is not configured", h.transportName())
	}
	if err := refresher.RefreshGlobal(c.Request.Context()); err != nil {
		return fmt.Errorf("refresh skills registry after marketplace mutation: %w", err)
	}
	return nil
}

func (h *BaseHandlers) syncSkillsAfterMarketplaceMutation(c *gin.Context) error {
	if h.SkillResources != nil {
		if err := h.SkillResources.SyncSkills(c.Request.Context()); err != nil {
			return fmt.Errorf("sync skill resources after marketplace mutation: %w", err)
		}
		return nil
	}
	return h.refreshSkillsAfterMarketplaceMutation(c)
}

func (h *BaseHandlers) logSkillMarketplaceInstallVerificationFailure(
	result skillmarketplace.InstallResult,
	err error,
) {
	h.Logger.Warn(
		"skills marketplace: installed skill is not discoverable",
		"name", result.Name,
		"source", result.Registry,
		"slug", result.Slug,
		"path", result.Path,
		"reason", err.Error(),
	)
}
