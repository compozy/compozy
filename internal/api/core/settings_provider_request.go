package core

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/modelcatalog"
	settingspkg "github.com/compozy/agh/internal/settings"
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
		Harness:         aghconfig.ProviderHarness(strings.TrimSpace(body.Settings.Harness)),
		RuntimeProvider: strings.TrimSpace(body.Settings.RuntimeProvider),
		Transport:       strings.TrimSpace(body.Settings.Transport),
		BaseURL:         strings.TrimSpace(body.Settings.BaseURL),
		AuthMode:        aghconfig.ProviderAuthMode(strings.TrimSpace(body.Settings.AuthMode)),
		EnvPolicy:       aghconfig.ProviderEnvPolicy(strings.TrimSpace(body.Settings.EnvPolicy)),
		HomePolicy:      aghconfig.ProviderHomePolicy(strings.TrimSpace(body.Settings.HomePolicy)),
		AuthStatusCmd:   strings.TrimSpace(body.Settings.AuthStatusCmd),
		AuthLoginCmd:    strings.TrimSpace(body.Settings.AuthLoginCmd),
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

func providerSettingsPayloadEmpty(payload contract.SettingsProviderSettingsPayload) bool {
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
		strings.TrimSpace(payload.AuthLoginCmd) == "" &&
		len(payload.CredentialSlots) == 0
}

func providerCredentialSlotsFromPayload(
	payloads []contract.SettingsProviderCredentialSlotPayload,
) []aghconfig.ProviderCredentialSlot {
	if len(payloads) == 0 {
		return nil
	}
	slots := make([]aghconfig.ProviderCredentialSlot, 0, len(payloads))
	for _, payload := range payloads {
		slots = append(slots, aghconfig.ProviderCredentialSlot{
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
