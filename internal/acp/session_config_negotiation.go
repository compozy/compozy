package acp

import (
	"context"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	sessionConfigModelKey        = "model"
	sessionConfigModeKey         = "mode"
	sessionModeAgent             = "agent"
	sessionModeAsk               = "ask"
	sessionModeBypassPermissions = "bypassPermissions"
)

func (d *Driver) applySessionMode(
	ctx context.Context,
	process *AgentProcess,
	permissions compozyconfig.PermissionMode,
) (bool, error) {
	if ctx == nil || process == nil || process.conn == nil {
		return false, nil
	}

	modeID := preferredSessionMode(process.CapsSnapshot().SupportedModes, permissions, process.toolGateway != nil)
	if modeID == "" {
		return false, nil
	}
	if option, ok := findSelectConfigOption(process.CapsSnapshot().ConfigOptions, sessionConfigModeKey); ok &&
		strings.TrimSpace(option.Current) == modeID {
		return false, nil
	}

	_, err := acpsdk.SendRequest[acpsdk.SetSessionModeResponse](
		process.conn,
		ctx,
		acpsdk.AgentMethodSessionSetMode,
		acpsdk.SetSessionModeRequest{
			SessionId: acpsdk.SessionId(process.SessionID),
			ModeId:    acpsdk.SessionModeId(modeID),
		},
	)
	if err == nil {
		process.setConfigOptionCurrent(sessionConfigModeKey, modeID)
	}
	return true, err
}

func (d *Driver) applySessionModel(
	ctx context.Context,
	process *AgentProcess,
	preferredModel string,
) (bool, error) {
	if ctx == nil || process == nil || process.conn == nil {
		return false, nil
	}
	modelID := strings.TrimSpace(preferredModel)
	if modelID == "" {
		return false, nil
	}

	caps := process.CapsSnapshot()
	if option, ok := findModelConfigOption(caps.ConfigOptions); ok {
		if !configOptionAllowsValue(option, modelID) {
			return false, newNegotiationError(
				NegotiationCodeModelUnavailable,
				sessionConfigModelKey,
				modelID,
				option.ID,
				configOptionChoices(option),
				nil,
			)
		}
		if err := d.applySessionConfigOption(ctx, process, option.ID, modelID); err != nil {
			return true, newNegotiationError(
				NegotiationCodeModelUnavailable,
				sessionConfigModelKey,
				modelID,
				option.ID,
				configOptionChoices(option),
				err,
			)
		}
		return true, nil
	}

	return false, newNegotiationError(
		NegotiationCodeModelUnavailable,
		sessionConfigModelKey,
		modelID,
		"",
		nil,
		errModelConfigOptionRequired,
	)
}

func (d *Driver) applySessionReasoningEffort(
	ctx context.Context,
	process *AgentProcess,
	effort string,
) (bool, error) {
	if ctx == nil || process == nil || process.conn == nil {
		return false, nil
	}
	effortID := strings.TrimSpace(effort)
	if effortID == "" {
		return false, nil
	}

	caps := process.CapsSnapshot()
	option, ok := findReasoningConfigOption(caps.ConfigOptions)
	if !ok {
		return false, newNegotiationError(
			NegotiationCodeReasoningOptionMissing,
			"reasoning effort",
			effortID,
			"",
			nil,
			nil,
		)
	}
	if !configOptionAllowsValue(option, effortID) {
		return false, newNegotiationError(
			NegotiationCodeReasoningEffortUnsupported,
			"reasoning effort",
			effortID,
			option.ID,
			configOptionChoices(option),
			nil,
		)
	}
	if strings.TrimSpace(option.Current) == effortID {
		return false, nil
	}
	if err := d.applySessionConfigOption(ctx, process, option.ID, effortID); err != nil {
		return true, newNegotiationError(
			NegotiationCodeReasoningEffortUnsupported,
			"reasoning effort",
			effortID,
			option.ID,
			configOptionChoices(option),
			err,
		)
	}
	return true, nil
}

func (d *Driver) applySessionConfigOption(
	ctx context.Context,
	process *AgentProcess,
	optionID string,
	valueID string,
) error {
	response, err := acpsdk.SendRequest[acpsdk.SetSessionConfigOptionResponse](
		process.conn,
		ctx,
		acpsdk.AgentMethodSessionSetConfigOption,
		acpsdk.SetSessionConfigOptionRequest{
			ValueId: &acpsdk.SetSessionConfigOptionValueId{
				SessionId: acpsdk.SessionId(process.SessionID),
				ConfigId:  acpsdk.SessionConfigId(strings.TrimSpace(optionID)),
				Value:     acpsdk.SessionConfigValueId(strings.TrimSpace(valueID)),
			},
		},
	)
	if err != nil {
		return err
	}
	process.setConfigOptions(sessionConfigOptionsFromSDK(response.ConfigOptions))
	return nil
}

func preferredSessionMode(
	supported []string,
	permissions compozyconfig.PermissionMode,
	toolGatewayEnabled bool,
) string {
	if len(supported) == 0 {
		return ""
	}

	lookup := make(map[string]string, len(supported))
	for _, mode := range supported {
		trimmed := strings.TrimSpace(mode)
		if trimmed == "" {
			continue
		}
		lookup[strings.ToLower(trimmed)] = trimmed
	}

	if permissions == compozyconfig.PermissionModeApproveAll {
		for _, candidate := range sessionModeCandidates(permissions) {
			if matched, ok := lookup[strings.ToLower(candidate)]; ok {
				return matched
			}
		}
	}

	if toolGatewayEnabled {
		for _, candidate := range permissionGatewayModeCandidates() {
			if matched, ok := lookup[strings.ToLower(candidate)]; ok {
				return matched
			}
		}
	}

	for _, candidate := range sessionModeCandidates(permissions) {
		if matched, ok := lookup[strings.ToLower(candidate)]; ok {
			return matched
		}
	}
	return ""
}

func permissionGatewayModeCandidates() []string {
	return []string{
		clientDefaultKey,
		sessionModeAsk,
	}
}

func sessionModeCandidates(permissions compozyconfig.PermissionMode) []string {
	switch permissions {
	case compozyconfig.PermissionModeApproveAll:
		return []string{
			sessionModeAgent,
			"full-access",
			"full_access",
			sessionModeBypassPermissions,
			"bypass_permissions",
			"auto",
			"acceptEdits",
		}
	case compozyconfig.PermissionModeApproveReads, compozyconfig.PermissionModeDenyAll:
		return []string{
			"read-only",
			"read_only",
			"readOnly",
			EventTypePlan,
			sessionModeAsk,
		}
	default:
		return nil
	}
}
