package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/skills"
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

	resolved, agentName, err := h.resolveSkillDetailScope(c)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}
	skillList, err := h.resolveScopedSkills(c, resolved, agentName)
	if err != nil {
		h.respondError(c, StatusForSkillError(err), err)
		return
	}
	skill := findSkillByName(skillList, name)
	if skill == nil {
		err = fmt.Errorf("%w: %q", ErrSkillNotFound, name)
		h.respondError(c, StatusForSkillError(err), err)
		return
	}

	payload := SkillPayloadFromSkill(skill)
	emptyExposures := []contract.SkillExposurePayload{}
	payload.Exposures = &emptyExposures
	resourceKind := skill.ResourceScope.Normalize().Kind
	if resourceKind == "user" || resourceKind == workspaceScopeValue {
		manager, managerErr := h.newSkillExposureManager(resolved)
		if managerErr != nil {
			h.respondError(c, http.StatusServiceUnavailable, managerErr)
			return
		}
		exposureCtx, contextErr := h.skillExposureInspectionContext(c, skill)
		if contextErr != nil {
			h.respondError(c, StatusForSkillError(contextErr), contextErr)
			return
		}
		states, exposureErr := manager.Exposures(exposureCtx, skill)
		if exposureErr != nil {
			h.respondError(c, http.StatusInternalServerError, exposureErr)
			return
		}
		exposures := SkillExposurePayloadsFromDomain(states)
		payload.Exposures = &exposures
	}
	c.JSON(http.StatusOK, contract.SkillResponse{Skill: payload})
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
