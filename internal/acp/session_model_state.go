package acp

import (
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

// wireSessionSetupResponse includes the Hermes model extension that the
// generated ACP SDK does not model yet. The standard fields remain typed by
// the SDK so modern config options keep their full union representation.
type wireSessionSetupResponse struct {
	SessionID     acpsdk.SessionId             `json:"sessionId,omitempty"`
	ConfigOptions []acpsdk.SessionConfigOption `json:"configOptions,omitempty"`
	Modes         *acpsdk.SessionModeState     `json:"modes,omitempty"`
	Models        *wireSessionModelState       `json:"models,omitempty"`
}

type wireSessionModelState struct {
	CurrentModelID  string                 `json:"currentModelId"`
	AvailableModels []wireSessionModelInfo `json:"availableModels"`
}

type wireSessionModelInfo struct {
	ModelID     string `json:"modelId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func captureSessionSetupCaps(
	base Caps,
	response wireSessionSetupResponse,
) Caps {
	caps := captureCaps(base, response.Modes, response.ConfigOptions)
	if _, ok := ModelConfigOption(caps.ConfigOptions); ok {
		return caps
	}
	modelOption, ok := sessionModelConfigOption(response.Models)
	if !ok {
		return caps
	}
	caps.ConfigOptions = append(caps.ConfigOptions, modelOption)
	return caps
}

func sessionModelConfigOption(state *wireSessionModelState) (SessionConfigOption, bool) {
	if state == nil {
		return SessionConfigOption{}, false
	}

	values := make([]SessionConfigOptionValue, 0, len(state.AvailableModels))
	seen := make(map[string]struct{}, len(state.AvailableModels))
	for _, model := range state.AvailableModels {
		value := strings.TrimSpace(model.ModelID)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		label := strings.TrimSpace(model.Name)
		if label == "" {
			label = value
		}
		values = append(values, SessionConfigOptionValue{
			Value:       value,
			Label:       label,
			Description: strings.TrimSpace(model.Description),
		})
	}

	current := strings.TrimSpace(state.CurrentModelID)
	if current != "" {
		if _, exists := seen[current]; !exists {
			values = append(values, SessionConfigOptionValue{
				Value: current,
				Label: current,
			})
		}
	}
	if len(values) == 0 {
		return SessionConfigOption{}, false
	}
	if current == "" {
		current = values[0].Value
	}

	return SessionConfigOption{
		ID:             sessionConfigModelKey,
		Label:          "Model",
		Category:       string(acpsdk.SessionConfigOptionCategoryModel),
		Kind:           SessionConfigOptionKindSelect,
		ReadOnly:       true,
		CurrentValueID: current,
		Values:         values,
	}, true
}

func preserveSyntheticModelConfigOption(
	current []SessionConfigOption,
	updated []SessionConfigOption,
) []SessionConfigOption {
	next := CloneSessionConfigOptions(updated)
	if _, ok := ModelConfigOption(next); ok {
		return next
	}
	model, ok := ModelConfigOption(current)
	if !ok || !model.ReadOnly {
		return next
	}
	return append(next, cloneSessionConfigOption(model))
}
