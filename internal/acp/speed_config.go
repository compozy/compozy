package acp

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	acpsdk "github.com/coder/acp-go-sdk"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

const (
	speedConfigCategory = "model_config"
	speedConfigKey      = "speed"
)

type speedConfigMatch struct {
	option    SessionConfigOption
	selection SessionConfigOptionSelection
}

func (d *Driver) applySessionSpeed(
	ctx context.Context,
	process *AgentProcess,
	requested speedpkg.Speed,
) (bool, error) {
	if requested == "" || ctx == nil || process == nil || process.conn == nil {
		return false, nil
	}
	parsed, err := speedpkg.Parse(string(requested))
	if err != nil {
		return false, err
	}

	match, reason := matchSpeedConfig(parsed, process.CapsSnapshot().ConfigOptions)
	if match == nil {
		process.setSpeedResolution(&speedpkg.Resolution{
			Requested: parsed,
			Status:    speedpkg.ResolutionUnsupported,
			Reason:    reason,
		})
		return false, nil
	}
	if match.alreadyApplied() {
		process.setSpeedResolution(&speedpkg.Resolution{
			Requested: parsed,
			Status:    speedpkg.ResolutionApplied,
		})
		return false, nil
	}

	request, err := match.request(acpsdk.SessionId(process.SessionID))
	if err != nil {
		return true, fmt.Errorf("acp: build speed config request: %w", err)
	}
	response, err := acpsdk.SendRequest[acpsdk.SetSessionConfigOptionResponse](
		process.conn,
		ctx,
		acpsdk.AgentMethodSessionSetConfigOption,
		request,
	)
	if err != nil {
		process.setSpeedResolution(&speedpkg.Resolution{
			Requested: parsed,
			Status:    speedpkg.ResolutionRejected,
			Reason:    speedpkg.ReasonProviderRejected,
		})
		return true, rejectedSpeedNegotiation(parsed, match.option.ID, err)
	}
	options := sessionConfigOptionsFromSDK(response.ConfigOptions)
	process.setConfigOptions(options)
	if !match.confirmed(options) {
		process.setSpeedResolution(&speedpkg.Resolution{
			Requested: parsed,
			Status:    speedpkg.ResolutionRejected,
			Reason:    speedpkg.ReasonProviderRejected,
		})
		return true, rejectedSpeedNegotiation(
			parsed,
			match.option.ID,
			fmt.Errorf("provider response did not confirm the requested speed"),
		)
	}
	process.setSpeedResolution(&speedpkg.Resolution{
		Requested: parsed,
		Status:    speedpkg.ResolutionApplied,
	})
	return true, nil
}

func rejectedSpeedNegotiation(requested speedpkg.Speed, optionID string, cause error) error {
	return newNegotiationError(
		NegotiationCodeSpeedRejected,
		speedConfigKey,
		string(requested),
		optionID,
		nil,
		cause,
	)
}

func matchSpeedConfig(
	requested speedpkg.Speed,
	options []SessionConfigOption,
) (*speedConfigMatch, speedpkg.ResolutionReason) {
	candidates := make([]SessionConfigOption, 0, 1)
	for _, option := range options {
		if isSpeedConfigCandidate(option) {
			candidates = append(candidates, option)
		}
	}
	switch len(candidates) {
	case 0:
		return nil, speedpkg.ReasonCapabilityAbsent
	case 1:
	default:
		return nil, speedpkg.ReasonCapabilityAmbiguous
	}

	option := candidates[0]
	if strings.TrimSpace(option.ID) == "" {
		return nil, speedpkg.ReasonValueAmbiguous
	}
	if countConfigOptionID(options, option.ID) != 1 {
		return nil, speedpkg.ReasonCapabilityAmbiguous
	}
	switch option.Kind {
	case SessionConfigOptionKindBoolean:
		return &speedConfigMatch{
			option: option,
			selection: SessionConfigOptionSelection{
				ID:        option.ID,
				BoolValue: new(requested == speedpkg.SpeedFast),
			},
		}, ""
	case SessionConfigOptionKindSelect:
		target, ok := selectSpeedTarget(requested, option.Values)
		if !ok {
			return nil, speedpkg.ReasonValueAmbiguous
		}
		return &speedConfigMatch{
			option: option,
			selection: SessionConfigOptionSelection{
				ID:      option.ID,
				ValueID: target,
			},
		}, ""
	default:
		return nil, speedpkg.ReasonValueAmbiguous
	}
}

func isSpeedConfigCandidate(option SessionConfigOption) bool {
	if normalizeSpeedToken(option.Category) != normalizeSpeedToken(speedConfigCategory) {
		return false
	}
	return isSpeedToken(normalizeSpeedToken(option.ID)) ||
		isSpeedToken(normalizeSpeedToken(option.Label))
}

func isSpeedToken(value string) bool {
	switch value {
	case speedConfigKey, string(speedpkg.SpeedFast), "fastmode":
		return true
	default:
		return false
	}
}

func countConfigOptionID(options []SessionConfigOption, id string) int {
	count := 0
	for _, option := range options {
		if strings.TrimSpace(option.ID) == strings.TrimSpace(id) {
			count++
		}
	}
	return count
}

func selectSpeedTarget(
	requested speedpkg.Speed,
	values []SessionConfigOptionValue,
) (string, bool) {
	targets := map[speedpkg.Speed][]string{
		speedpkg.SpeedNormal: nil,
		speedpkg.SpeedFast:   nil,
	}
	valueCounts := make(map[string]int, len(values))
	for _, value := range values {
		valueID := strings.TrimSpace(value.Value)
		valueCounts[valueID]++
		semantic, ok := classifySpeedValue(value)
		if !ok {
			return "", false
		}
		if semantic != "" {
			targets[semantic] = append(targets[semantic], valueID)
		}
	}
	if len(targets[speedpkg.SpeedNormal]) != 1 || len(targets[speedpkg.SpeedFast]) != 1 {
		return "", false
	}
	target := targets[requested][0]
	return target, target != "" && valueCounts[target] == 1
}

func classifySpeedValue(value SessionConfigOptionValue) (speedpkg.Speed, bool) {
	valueSemantic := speedValueSemantic(normalizeSpeedToken(value.Value))
	labelSemantic := speedValueSemantic(normalizeSpeedToken(value.Label))
	if valueSemantic != "" && labelSemantic != "" && valueSemantic != labelSemantic {
		return "", false
	}
	if valueSemantic != "" {
		return valueSemantic, true
	}
	return labelSemantic, true
}

func speedValueSemantic(value string) speedpkg.Speed {
	switch value {
	case string(speedpkg.SpeedNormal), "standard", "off", "disabled", booleanFalseText:
		return speedpkg.SpeedNormal
	case string(speedpkg.SpeedFast), "on", "enabled", booleanTrueText:
		return speedpkg.SpeedFast
	default:
		return ""
	}
}

func normalizeSpeedToken(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func (m speedConfigMatch) alreadyApplied() bool {
	return m.selection.matches(m.option)
}

func (m speedConfigMatch) request(
	sessionID acpsdk.SessionId,
) (setSessionConfigOptionWireRequest, error) {
	request, err := m.selection.request(sessionID)
	if err != nil {
		return setSessionConfigOptionWireRequest{}, err
	}
	return request, nil
}

func (m speedConfigMatch) confirmed(options []SessionConfigOption) bool {
	if countConfigOptionID(options, m.option.ID) != 1 {
		return false
	}
	for _, option := range options {
		if strings.TrimSpace(option.ID) != strings.TrimSpace(m.option.ID) {
			continue
		}
		return m.selection.matches(option)
	}
	return false
}
