package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationmodel "github.com/compozy/compozy/internal/automation/model"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func automationSettingsFromPayload(
	payload contract.SettingsAutomationConfigPayload,
) (settingspkg.AutomationSettings, error) {
	config := compozyconfig.AutomationConfig{
		Enabled:           payload.Enabled,
		Timezone:          strings.TrimSpace(payload.Timezone),
		MaxConcurrentJobs: payload.MaxConcurrentJobs,
		DefaultFireLimit:  payload.DefaultFireLimit,
		Suggestions: compozyconfig.AutomationSuggestionsConfig{
			PendingCap: automationmodel.DefaultSuggestionPendingCap,
		},
	}
	if err := config.Validate(); err != nil {
		return settingspkg.AutomationSettings{}, NewSettingsValidationError(err)
	}
	return settingspkg.AutomationSettings{
		Enabled:           config.Enabled,
		Timezone:          config.Timezone,
		MaxConcurrentJobs: config.MaxConcurrentJobs,
		DefaultFireLimit:  config.DefaultFireLimit,
	}, nil
}
