package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

	caps := process.CapsSnapshot()
	option, configurable := findConfigOptionByID(caps.ConfigOptions, sessionConfigModeKey)
	configurable = configurable && option.Kind == SessionConfigOptionKindSelect
	modes := caps.SupportedModes
	if configurable {
		modes = configOptionChoices(option)
	}
	modeID := preferredSessionMode(modes, permissions, process.toolGateway != nil,
		process.permissionModeProvider(), process.reviewerMode)
	if process.executionPolicy == ExecutionPolicyTriage {
		modeID = triageReadOnlyMode(process.providerName, modes)
		if modeID == "" {
			return false, ErrTriageUnsupported
		}
	}
	if modeID == "" {
		return false, nil
	}
	if configurable && strings.TrimSpace(option.CurrentValueID) == modeID {
		return false, nil
	}
	// Providers that advertise session modes receive session/set_mode; the mode
	// config option is the transport only for triage or when the mode is not one
	// of the advertised session modes.
	if configurable && (process.executionPolicy == ExecutionPolicyTriage ||
		!advertisesSessionMode(caps.SupportedModes, modeID)) {
		return true, d.applySessionConfigOption(
			ctx,
			process,
			SessionConfigOptionSelection{ID: option.ID, ValueID: modeID},
		)
	}

	_, err := process.sendControlRequest[acpsdk.SetSessionModeResponse](ctx,
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

func advertisesSessionMode(modes []string, modeID string) bool {
	for _, mode := range modes {
		if strings.TrimSpace(mode) == modeID {
			return true
		}
	}
	return false
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
	if err := ValidateModelConfigValue(caps.ConfigOptions, modelID); err != nil {
		return false, err
	}
	option, _ := ModelConfigOption(caps.ConfigOptions)
	if option.ReadOnly {
		return false, newNegotiationError(
			NegotiationCodeModelUnavailable,
			sessionConfigModelKey,
			modelID,
			option.ID,
			configOptionChoices(option),
			errModelConfigOptionReadOnly,
		)
	}
	if err := d.applySessionConfigOption(ctx, process, SessionConfigOptionSelection{
		ID:      option.ID,
		ValueID: modelID,
	}); err != nil {
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

// ValidateModelConfigValue verifies that modelID is an exact advertised ACP model value.
func ValidateModelConfigValue(options []SessionConfigOption, modelID string) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil
	}
	option, ok := ModelConfigOption(options)
	if !ok {
		return newNegotiationError(
			NegotiationCodeModelUnavailable,
			sessionConfigModelKey,
			modelID,
			"",
			nil,
			errModelConfigOptionRequired,
		)
	}
	if configOptionAllowsValue(option, modelID) {
		return nil
	}
	return newNegotiationError(
		NegotiationCodeModelUnavailable,
		sessionConfigModelKey,
		modelID,
		option.ID,
		configOptionChoices(option),
		nil,
	)
}

// Providers may apply a selected value and announce it in a config_option_update
// that arrives after the set response returns, so one readback is not proof of
// rejection. These bounds keep the confirmation wait short and deterministic.
const (
	reasoningEffortConfirmAttempts = 4
	reasoningEffortConfirmBackoff  = 50 * time.Millisecond
)

// applySessionReasoningEffort rejects unadvertised intent before requesting provider
// acknowledgement, and retries an unconfirmed apply while the provider snapshot settles.
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

	requested := false
	optionID := ""
	var unconfirmed error
	for attempt := range reasoningEffortConfirmAttempts {
		if attempt > 0 {
			if err := waitReasoningEffortConfirmation(ctx, attempt); err != nil {
				return requested, err
			}
		}
		option, ok := findReasoningConfigOption(process.CapsSnapshot().ConfigOptions)
		if !ok {
			return requested, newNegotiationError(
				NegotiationCodeReasoningOptionMissing,
				"reasoning effort",
				effortID,
				"",
				nil,
				nil,
			)
		}
		optionID = option.ID
		if !configOptionAllowsValue(option, effortID) {
			return requested, newNegotiationError(
				NegotiationCodeReasoningEffortUnsupported,
				"reasoning effort",
				effortID,
				option.ID,
				configOptionChoices(option),
				nil,
			)
		}
		if strings.TrimSpace(option.CurrentValueID) == effortID {
			return requested, nil
		}
		requested = true
		err := d.applySessionConfigOption(ctx, process, SessionConfigOptionSelection{
			ID:      option.ID,
			ValueID: effortID,
		})
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, errConfigOptionUnconfirmed) {
			return true, newNegotiationError(
				NegotiationCodeReasoningEffortUnsupported,
				"reasoning effort",
				effortID,
				option.ID,
				configOptionChoices(option),
				err,
			)
		}
		unconfirmed = err
	}
	// The provider advertises the value but never confirmed it across the retry
	// window: report the unconfirmed cause without claiming the value is absent
	// from the advertised choices.
	return true, newNegotiationError(
		NegotiationCodeReasoningEffortUnsupported,
		"reasoning effort",
		effortID,
		optionID,
		nil,
		unconfirmed,
	)
}

func waitReasoningEffortConfirmation(ctx context.Context, attempt int) error {
	timer := time.NewTimer(reasoningEffortConfirmBackoff << (attempt - 1))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("acp: wait for reasoning effort confirmation: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// applySessionConfigOption requires confirmation without replacing a concurrent provider observation.
func (d *Driver) applySessionConfigOption(
	ctx context.Context,
	process *AgentProcess,
	selection SessionConfigOptionSelection,
) error {
	request, err := selection.request(acpsdk.SessionId(process.SessionID))
	if err != nil {
		return err
	}
	response, err := process.sendControlRequest[acpsdk.SetSessionConfigOptionResponse](ctx,
		acpsdk.AgentMethodSessionSetConfigOption,
		request,
	)
	if err != nil {
		return err
	}
	revision := process.configRevisionAfterResponse()
	options := process.setConfigOptionsResponse(sessionConfigOptionsFromSDK(response.ConfigOptions), revision)
	confirmed, ok := findConfigOptionByID(options, selection.ID)
	if !ok || !selection.matches(confirmed) {
		return fmt.Errorf(
			"acp: provider did not confirm config option %q: %w",
			selection.ID,
			errConfigOptionUnconfirmed,
		)
	}
	return nil
}

func preferredSessionMode(
	supported []string,
	permissions compozyconfig.PermissionMode,
	toolGatewayEnabled bool,
	provider, reviewerMode string,
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

	if permissions == compozyconfig.PermissionModeAuto {
		if reviewerMode != "" {
			if matched, ok := lookup[strings.ToLower(strings.TrimSpace(reviewerMode))]; ok &&
				!compozyconfig.IsUnrestrictedACPOption(provider, sessionConfigModeKey, matched) {
				return matched
			}
		}
		permissions = compozyconfig.PermissionModeApproveEdits
	}
	if permissions == compozyconfig.PermissionModeApproveAll ||
		permissions == compozyconfig.PermissionModeApproveEdits {
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
	case compozyconfig.PermissionModeApproveEdits, compozyconfig.PermissionModeAuto:
		return []string{"acceptEdits", "workspace-write", "accept_edits", "auto_edit"}
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
