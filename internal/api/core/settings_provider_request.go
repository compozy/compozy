package core

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/modelcatalog"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

func parsePutSettingsProviderRequest(c *gin.Context) (settingspkg.CollectionItemPutRequest, error) {
	var body contract.PutSettingsProviderRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode provider settings request: %w", err),
		)
	}
	if providerSettingsPayloadEmpty(body.Settings) && len(body.Secrets) == 0 {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			errors.New("provider.settings is required"),
		)
	}
	req, err := parseSettingsCollectionRequest(c, settingspkg.CollectionProviders)
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	settings := settingspkg.ProviderSettings{
		Command:         strings.TrimSpace(body.Settings.Command),
		DisplayName:     strings.TrimSpace(body.Settings.DisplayName),
		Models:          providerModelsFromPayload(body.Settings.Models),
		ModelsSet:       body.Settings.Models != nil,
		Harness:         compozyconfig.ProviderHarness(strings.TrimSpace(body.Settings.Harness)),
		RuntimeProvider: strings.TrimSpace(body.Settings.RuntimeProvider),
		Transport:       strings.TrimSpace(body.Settings.Transport),
		BaseURL:         strings.TrimSpace(body.Settings.BaseURL),
		AuthMode:        compozyconfig.ProviderAuthMode(strings.TrimSpace(body.Settings.AuthMode)),
		EnvPolicy:       compozyconfig.ProviderEnvPolicy(strings.TrimSpace(body.Settings.EnvPolicy)),
		HomePolicy:      compozyconfig.ProviderHomePolicy(strings.TrimSpace(body.Settings.HomePolicy)),
		AuthStatusCmd:   strings.TrimSpace(body.Settings.AuthStatusCmd),
		AuthLoginCmd:    trimmedOptionalString(body.Settings.AuthLoginCmd),
		AuthLoginCmdSet: body.Settings.AuthLoginCmd != nil,
		CredentialSlots: providerCredentialSlotsFromPayload(body.Settings.CredentialSlots),
	}
	return settingspkg.CollectionItemPutRequest{
		CollectionRequest:     req,
		Name:                  name,
		Provider:              &settings,
		ProviderModelCuration: providerModelCurationFromPayload(name, body.ModelCuration),
		ProviderSecrets:       providerSecretWritesFromPayload(body.Secrets),
	}, nil
}

func providerModelCurationFromPayload(
	providerID string,
	payload *contract.ProviderModelCurationRequest,
) *settingspkg.ProviderModelCurationRequest {
	if payload == nil {
		return nil
	}
	return &settingspkg.ProviderModelCurationRequest{
		ProviderID:             strings.TrimSpace(providerID),
		ModelID:                strings.TrimSpace(payload.ModelID),
		Hidden:                 cloneBoolPtr(payload.Hidden),
		Featured:               cloneBoolPtr(payload.Featured),
		Deprecated:             cloneBoolPtr(payload.Deprecated),
		DefaultReasoningEffort: cloneReasoningEffortPtr(payload.DefaultReasoningEffort),
	}
}

func cloneReasoningEffortPtr(value *modelcatalog.ReasoningEffort) *modelcatalog.ReasoningEffort {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func providerSettingsPayloadEmpty(payload contract.SettingsProviderWritePayload) bool {
	return strings.TrimSpace(payload.Command) == "" &&
		strings.TrimSpace(payload.DisplayName) == "" &&
		payload.Models == nil &&
		strings.TrimSpace(payload.Harness) == "" &&
		strings.TrimSpace(payload.RuntimeProvider) == "" &&
		strings.TrimSpace(payload.Transport) == "" &&
		strings.TrimSpace(payload.BaseURL) == "" &&
		strings.TrimSpace(payload.AuthMode) == "" &&
		strings.TrimSpace(payload.EnvPolicy) == "" &&
		strings.TrimSpace(payload.HomePolicy) == "" &&
		strings.TrimSpace(payload.AuthStatusCmd) == "" &&
		payload.AuthLoginCmd == nil &&
		len(payload.CredentialSlots) == 0
}

func trimmedOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func providerCredentialSlotsFromPayload(
	payloads []contract.SettingsProviderCredentialSlotPayload,
) []compozyconfig.ProviderCredentialSlot {
	if len(payloads) == 0 {
		return nil
	}
	slots := make([]compozyconfig.ProviderCredentialSlot, 0, len(payloads))
	for _, payload := range payloads {
		slots = append(slots, compozyconfig.ProviderCredentialSlot{
			Name:      strings.TrimSpace(payload.Name),
			TargetEnv: strings.TrimSpace(payload.TargetEnv),
			SecretRef: strings.TrimSpace(payload.SecretRef),
			Kind:      strings.TrimSpace(payload.Kind),
			Required:  payload.Required,
		})
	}
	return slots
}

func providerSecretWritesFromPayload(
	payloads []contract.SettingsProviderSecretWritePayload,
) []settingspkg.ProviderSecretWrite {
	if len(payloads) == 0 {
		return nil
	}
	secrets := make([]settingspkg.ProviderSecretWrite, 0, len(payloads))
	for _, payload := range payloads {
		secrets = append(secrets, settingspkg.ProviderSecretWrite{
			Name:      strings.TrimSpace(payload.Name),
			SecretRef: strings.TrimSpace(payload.SecretRef),
			Kind:      strings.TrimSpace(payload.Kind),
			Value:     payload.Value,
		})
	}
	return secrets
}
