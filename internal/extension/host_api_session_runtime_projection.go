package extensionpkg

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

const maxHostAPIRuntimeDiagnosticBytes = 2048

func hostAPISessionRuntimePayloadFromInfo(info *session.Info) apicontract.SessionRuntimePayload {
	if info == nil {
		return apicontract.SessionRuntimePayload{}
	}

	payload := apicontract.SessionRuntimePayload{
		Status:            info.RuntimeStatus,
		Transition:        info.RuntimeTransition,
		Failure:           diagnostics.RedactAndBound(info.RuntimeFailure, maxHostAPIRuntimeDiagnosticBytes),
		Selected:          apicontract.PromptRuntimeSelectionPayloadFromSelection(info.SelectedRuntime),
		SelectionRevision: info.RuntimeSelectionRevision,
		ACPSessionID:      strings.TrimSpace(info.ACPSessionID),
	}
	if hostAPIRuntimeHasEffectiveSelection(info) {
		payload.Effective = &apicontract.RuntimeSelectionPayload{
			Provider:        strings.TrimSpace(info.Provider),
			Model:           strings.TrimSpace(info.Model),
			ReasoningEffort: apicontract.ReasoningEffort(strings.TrimSpace(info.ReasoningEffort)),
			Speed:           info.Speed,
			SpeedResolution: speedpkg.CloneResolution(info.SpeedResolution),
		}
	}
	payload.ACPCaps = hostAPIACPCapsPayloadFromInfo(info.ACPCaps)
	return payload
}

func hostAPIRuntimeHasEffectiveSelection(info *session.Info) bool {
	if info == nil || strings.TrimSpace(info.Provider) == "" {
		return false
	}
	switch info.RuntimeStatus {
	case session.RuntimeStatusReady, session.RuntimeStatusReconfiguring, session.RuntimeStatusFailed:
		return true
	case session.RuntimeStatusUnbound:
		return info.RuntimeTransition != session.RuntimeTransitionNone
	default:
		return false
	}
}

func hostAPIACPCapsPayloadFromInfo(caps acp.Caps) *apicontract.ACPCapsPayload {
	if !caps.SupportsLoadSession && len(caps.SupportedModes) == 0 && len(caps.ConfigOptions) == 0 {
		return nil
	}
	return &apicontract.ACPCapsPayload{
		SupportsLoadSession: caps.SupportsLoadSession,
		SupportedModes:      append([]string(nil), caps.SupportedModes...),
		ConfigOptions:       hostAPISessionConfigOptionPayloads(caps.ConfigOptions),
	}
}

func hostAPISessionConfigOptionPayloads(
	options []acp.SessionConfigOption,
) []apicontract.SessionConfigOptionPayload {
	if len(options) == 0 {
		return nil
	}
	payloads := make([]apicontract.SessionConfigOptionPayload, 0, len(options))
	for _, option := range options {
		payloads = append(payloads, apicontract.SessionConfigOptionPayload{
			ID:          strings.TrimSpace(option.ID),
			Label:       strings.TrimSpace(option.Label),
			Description: strings.TrimSpace(option.Description),
			Kind:        string(option.Kind),
			Current:     strings.TrimSpace(option.Current),
			Values:      hostAPISessionConfigOptionValuePayloads(option.Values),
		})
	}
	return payloads
}

func hostAPISessionConfigOptionValuePayloads(
	values []acp.SessionConfigOptionValue,
) []apicontract.SessionConfigOptionValuePayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]apicontract.SessionConfigOptionValuePayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, apicontract.SessionConfigOptionValuePayload{
			Value:       strings.TrimSpace(value.Value),
			Label:       strings.TrimSpace(value.Label),
			Description: strings.TrimSpace(value.Description),
		})
	}
	return payloads
}
