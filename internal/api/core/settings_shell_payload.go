package core

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsShellSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.Shell == nil {
		return nil, errors.New("settings shell section is required")
	}
	return contract.SettingsShellResponse{
		SettingsUserSectionResponseMetaPayload: settingsUserSectionMetaPayload(envelope),
		Config:                                 settingsShellPayload(envelope.Shell.Config),
		Warnings:                               envelope.Shell.Warnings,
		LegacyScope: contract.SettingsShellLegacyScopePayload{
			Value:    contract.SettingsShellSessionScope(envelope.Shell.Config.Sessions.Scope),
			Explicit: envelope.Shell.Config.Sessions.ScopeExplicit,
			Revision: envelope.Shell.ScopeRevision,
		},
	}, nil
}

func settingsShellPayload(cfg compozyconfig.ShellConfig) contract.SettingsShellPayload {
	uiFont := cfg.Appearance.UIFont
	monoFont := cfg.Appearance.MonoFont
	return contract.SettingsShellPayload{
		Sessions: contract.SettingsShellSessionsPayload{
			GroupBy: contract.SettingsShellSessionGroupBy(cfg.Sessions.GroupBy),
			Sort:    contract.SettingsShellSessionSort(cfg.Sessions.Sort),
			Scope:   contract.SettingsShellSessionScope(cfg.Sessions.Scope),
		},
		Appearance: &contract.SettingsShellAppearancePayload{UIFont: &uiFont, MonoFont: &monoFont},
	}
}

func shellConfigFromPayload(payload contract.SettingsShellPayload) (compozyconfig.ShellConfig, error) {
	value := compozyconfig.ShellConfig{Sessions: compozyconfig.ShellSessionsConfig{
		GroupBy:         compozyconfig.ShellSessionGroupBy(payload.Sessions.GroupBy),
		GroupByExplicit: payload.Sessions.GroupBy != "",
		ScopeExplicit:   payload.Sessions.Scope != "",
		Sort:            compozyconfig.ShellSessionSort(payload.Sessions.Sort),
		Scope:           compozyconfig.ShellSessionScope(payload.Sessions.Scope),
	}}
	if appearance := payload.Appearance; appearance != nil {
		if appearance.UIFont != nil {
			value.Appearance.UIFont = *appearance.UIFont
			value.Appearance.UIFontExplicit = true
		}
		if appearance.MonoFont != nil {
			value.Appearance.MonoFont = *appearance.MonoFont
			value.Appearance.MonoFontExplicit = true
		}
	}
	validation := value
	if validation.Sessions.Sort == "" {
		validation.Sessions.Sort = compozyconfig.ShellSessionSortCreated
	}
	if validation.Sessions.Scope == "" {
		validation.Sessions.Scope = compozyconfig.ShellSessionScopeWorkspace
	}
	if err := validation.Validate(); err != nil {
		return compozyconfig.ShellConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}

func (h *BaseHandlers) updateSettingsShellSection(c *gin.Context, req settingspkg.SectionUpdateRequest) {
	result, ok := h.applySettingsSection(c, req, nil)
	if !ok {
		return
	}
	envelope, err := h.Settings.GetSection(c.Request.Context(), req.SectionRequest)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	if envelope.Shell == nil {
		h.respondError(c, http.StatusInternalServerError, errors.New("settings shell section is required"))
		return
	}
	c.JSON(http.StatusOK, contract.SettingsShellMutationResponse{
		SettingsApplyResponse: SettingsApplyResponseFromResult(result),
		Config:                settingsShellPayload(envelope.Shell.Config),
		LegacyScope: contract.SettingsShellLegacyScopePayload{
			Value:    contract.SettingsShellSessionScope(envelope.Shell.Config.Sessions.Scope),
			Explicit: envelope.Shell.Config.Sessions.ScopeExplicit,
			Revision: envelope.Shell.ScopeRevision,
		},
	})
}
