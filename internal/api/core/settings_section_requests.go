package core

import (
	"context"
	"errors"
	"fmt"

	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	extensionpkg "github.com/compozy/agh/internal/extension"

	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

func parseSettingsSectionRequest(
	c *gin.Context,
	section settingspkg.SectionName,
) (settingspkg.SectionRequest, error) {
	scope, workspaceID, err := parseSettingsScope(c.Query("scope"), c.Query("workspace_id"))
	if err != nil {
		return settingspkg.SectionRequest{}, err
	}
	agentName := strings.TrimSpace(c.Query("agent_name"))
	if agentName != "" {
		if section != settingspkg.SectionSkills {
			return settingspkg.SectionRequest{}, NewSettingsValidationError(
				errors.New("agent_name is only supported for skills"),
			)
		}
		if err := aghconfig.ValidateAgentName(agentName); err != nil {
			return settingspkg.SectionRequest{}, NewSettingsValidationError(err)
		}
	}
	return settingspkg.SectionRequest{
		Section:     section,
		Scope:       scope,
		WorkspaceID: workspaceID,
		AgentName:   agentName,
	}, nil
}

func parseSettingsCollectionRequest(
	c *gin.Context,
	collection settingspkg.CollectionName,
) (settingspkg.CollectionRequest, error) {
	scope, workspaceID, err := parseSettingsScope(c.Query("scope"), c.Query("workspace_id"))
	if err != nil {
		return settingspkg.CollectionRequest{}, err
	}
	return settingspkg.CollectionRequest{
		Collection:  collection,
		Scope:       scope,
		WorkspaceID: workspaceID,
	}, nil
}

func parseConfigApplyRecordFilter(c *gin.Context) (settingspkg.ApplyRecordFilter, error) {
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return settingspkg.ApplyRecordFilter{}, NewSettingsValidationError(
				fmt.Errorf("settings.apply.limit must be a positive integer: %w", err),
			)
		}
		if parsed <= 0 {
			return settingspkg.ApplyRecordFilter{}, NewSettingsValidationError(
				fmt.Errorf("settings.apply.limit must be positive: %d", parsed),
			)
		}
		limit = parsed
	}
	filter := settingspkg.ApplyRecordFilter{
		Actor: strings.TrimSpace(c.Query("actor")),
		Limit: limit,
	}
	rawStatus := strings.TrimSpace(c.Query("status"))
	if rawStatus != "" {
		switch rawStatus {
		case string(lifecycle.StatusPendingApply),
			string(lifecycle.StatusApplied),
			string(lifecycle.StatusBlocked),
			string(lifecycle.StatusFailed):
			filter.Status = lifecycle.Status(rawStatus)
		default:
			return settingspkg.ApplyRecordFilter{}, NewSettingsValidationError(
				fmt.Errorf("settings.apply.status must be one of %q, %q, %q, %q",
					lifecycle.StatusPendingApply,
					lifecycle.StatusApplied,
					lifecycle.StatusBlocked,
					lifecycle.StatusFailed,
				),
			)
		}
	}
	return filter, nil
}

func parseDeleteSettingsCollectionRequest(
	c *gin.Context,
	collection settingspkg.CollectionName,
) (settingspkg.CollectionItemDeleteRequest, error) {
	req, err := parseSettingsCollectionRequest(c, collection)
	if err != nil {
		return settingspkg.CollectionItemDeleteRequest{}, err
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		return settingspkg.CollectionItemDeleteRequest{}, err
	}
	target, err := parseSettingsTarget(c.Query("target"))
	if err != nil {
		return settingspkg.CollectionItemDeleteRequest{}, err
	}
	return settingspkg.CollectionItemDeleteRequest{
		CollectionRequest: req,
		Name:              name,
		Target:            target,
	}, nil
}

func parseUpdateSettingsGeneralRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *settingsGeneralUpdateConfigPayload `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode general settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(errors.New("general.config is required"))
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionGeneral)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	payload, err := body.Config.validatedPayload()
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := generalSettingsFromPayload(payload)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, General: &config}, nil
}

func parseUpdateSettingsMemoryRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsMemoryConfigPayload `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode memory settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(errors.New("memory.config is required"))
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionMemory)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := memoryConfigFromPayload(body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, Memory: &config}, nil
}

func (h *BaseHandlers) validateSettingsMemoryProvider(
	ctx context.Context,
	req settingspkg.SectionUpdateRequest,
) error {
	if req.Memory == nil {
		return nil
	}
	name := strings.TrimSpace(req.Memory.Provider.Name)
	if name == "" || name == memoryLocalProviderName {
		return nil
	}
	if h.MemoryProviders == nil {
		return NewSettingsValidationError(
			fmt.Errorf("memory.config.provider.name %q is not available", name),
		)
	}
	if _, err := h.MemoryProviders.Get(ctx, req.WorkspaceID, name); err != nil {
		if errors.Is(err, extensionpkg.ErrMemoryProviderNotFound) {
			return NewSettingsValidationError(
				fmt.Errorf("memory.config.provider.name %q is not available: %w", name, err),
			)
		}
		return fmt.Errorf("memory.config.provider.name %q lookup failed: %w", name, err)
	}
	return nil
}

func parseUpdateSettingsSkillsRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsSkillsConfigPayload `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode skills settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(errors.New("skills.config is required"))
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionSkills)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := skillsConfigFromPayload(*body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, Skills: &config}, nil
}

func parseUpdateSettingsAutomationRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsAutomationConfigPayload `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode automation settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errors.New("automation.config is required"),
		)
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionAutomation)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := automationSettingsFromPayload(*body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, Automation: &config}, nil
}

func parseUpdateSettingsNetworkRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsNetworkConfigPayload `json:"config"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode network settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(errors.New("network.config is required"))
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionNetwork)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := networkConfigFromPayload(*body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, Network: &config}, nil
}

func parseUpdateSettingsWindowManagerRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsWindowManagerConfigPayload `json:"config"`
	}
	if err := decodeStrictJSONBody(c, &body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode window-manager settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errors.New("window-manager.config is required"),
		)
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionWindowManager)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := windowManagerConfigFromPayload(*body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, WindowManager: &config}, nil
}

func parseUpdateSettingsObservabilityRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsObservabilityConfigPayload `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode observability settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errors.New("observability.config is required"),
		)
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionObservability)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := observabilityConfigFromPayload(*body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, Observability: &config}, nil
}

func parseUpdateSettingsHooksExtensionsRequest(c *gin.Context) (settingspkg.SectionUpdateRequest, error) {
	var body struct {
		Config *contract.SettingsExtensionsConfigPayload `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode hooks-extensions settings request: %w", err),
		)
	}
	if body.Config == nil {
		return settingspkg.SectionUpdateRequest{}, NewSettingsValidationError(
			errors.New("hooks-extensions.config is required"),
		)
	}
	req, err := parseSettingsSectionRequest(c, settingspkg.SectionHooksExtensions)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	config, err := extensionsConfigFromPayload(*body.Config)
	if err != nil {
		return settingspkg.SectionUpdateRequest{}, err
	}
	return settingspkg.SectionUpdateRequest{SectionRequest: req, HooksExtensions: &config}, nil
}
