package core

import (
	"net/http"
	"os"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondAgentWithEnvironment(c *gin.Context, entry *AgentCatalogEntry, cfg *config.Config) {
	payload := AgentPayloadFromEntryWithConfig(entry, cfg)
	if runtime := payload.EffectiveRuntime; runtime != nil && len(runtime.SessionEnv) != 0 {
		statuses, err := providers.SessionEnvStatuses(c.Request.Context(), sessionEnvFromPayload(runtime.SessionEnv),
			&providers.ProbeEnv{LookupEnv: os.LookupEnv, Vault: h.Vault})
		if err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
		runtime.SessionEnvStatus = sessionEnvStatusPayloads(statuses)
	}
	c.JSON(http.StatusOK, contract.AgentResponse{Agent: payload})
}

// SessionEnvToPayload projects reference-only declarations to the public contract.
func SessionEnvToPayload(env config.SessionEnv) map[string]contract.SessionEnvBindingPayload {
	if env == nil {
		return nil
	}
	result := make(map[string]contract.SessionEnvBindingPayload, len(env))
	for name, binding := range env {
		result[name] = contract.SessionEnvBindingPayload{Ref: binding.Ref, Required: binding.Required}
	}
	return result
}

func sessionEnvStatusPayloads(
	statuses []providers.CredentialStatus,
) []contract.SettingsProviderCredentialStatusPayload {
	result := make([]contract.SettingsProviderCredentialStatusPayload, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, contract.SettingsProviderCredentialStatusPayload{
			Name: status.Name, TargetEnv: status.TargetEnv, SecretRef: status.SecretRef,
			Required: status.Required, Present: status.Present, Source: status.Source,
		})
	}
	return result
}

func sessionEnvFromPayload(env map[string]contract.SessionEnvBindingPayload) config.SessionEnv {
	if env == nil {
		return nil
	}
	result := make(config.SessionEnv, len(env))
	for name, binding := range env {
		result[name] = config.SessionEnvBinding{Ref: binding.Ref, Required: binding.Required}
	}
	return result
}
