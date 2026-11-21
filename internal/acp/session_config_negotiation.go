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
		modeID = triageReadOnlyMode(process.permissionModeProvider(), modes)
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
		err := d.applySessionConfigOption(ctx, process, SessionConfigOptionSelection{ID: option.ID, ValueID: modeID})
		if err != nil && isConfigRejectionCause(err) {
			// A provider refusal of its own advertised mode is the same structured rejection
			// as any other config option, so callers can map it to actionable guidance.
			err = fmt.Errorf("%w: apply config option %q: %w", ErrSessionConfigOptionRejected, option.ID, err)
		}
		return true, err
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
	waitForCatalog bool,
) (bool, error) {
	if ctx == nil || process == nil || process.conn == nil {
		return false, nil
	}
	modelID := strings.TrimSpace(preferredModel)
	if modelID == "" {
		return false, nil
	}

	option, err := d.awaitSessionModelOption(ctx, process, modelID, waitForCatalog)
	if err != nil {
		return false, err
	}
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

// awaitSessionModelOption reads the model option from the current snapshot. During the
// fresh startup bind the provider may still be publishing its catalog in a later
// config_option_update, so that caller waits for a newer revision; runtime switches and
// inspection pass waitForCatalog false and reject against the snapshot they have.
func (d *Driver) awaitSessionModelOption(
	ctx context.Context,
	process *AgentProcess,
	modelID string,
	waitForCatalog bool,
) (SessionConfigOption, error) {
	options := process.CapsSnapshot().ConfigOptions
	option, ok := ModelConfigOption(options)
	if !ok {
		return SessionConfigOption{}, ValidateModelConfigValue(options, modelID)
	}
	if configOptionAllowsValue(option, modelID) {
		return option, nil
	}
	if !waitForCatalog || option.ReadOnly {
		// Discovery-only options never gain writable values after session/new.
		return SessionConfigOption{}, ValidateModelConfigValue(options, modelID)
	}

	budget := defaultModelCatalogBudget
	if d != nil && d.modelCatalogBudget > 0 {
		budget = d.modelCatalogBudget
	}
	deadline := time.Now().Add(budget)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// Capture the revision before reading the snapshot: an update that lands
		// between the two is then already newer than the revision we wait on, so the
		// wait returns instead of blocking for the whole budget.
		since, _ := process.configRevisionAndChange()
		options = process.CapsSnapshot().ConfigOptions
		if announced, found := ModelConfigOption(options); found &&
			configOptionAllowsValue(announced, modelID) {
			return announced, nil
		}
		if err := waitForConfigRevision(ctx, process, since, remaining); err != nil {
			if errors.Is(err, errConfigSettleTimeout) {
				break
			}
			return SessionConfigOption{}, err
		}
	}

	// The budget elapsed without an announcement: the rejection is permanent, so
	// report it against the freshest catalog the provider did publish.
	options = process.CapsSnapshot().ConfigOptions
	option, ok = ModelConfigOption(options)
	if ok && configOptionAllowsValue(option, modelID) {
		return option, nil
	}
	return SessionConfigOption{}, ValidateModelConfigValue(options, modelID)
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

// applySessionReasoningEffort rejects unadvertised intent before requesting provider
// acknowledgement; the shared confirmation wait covers a late config_option_update.
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

	option, ok := findReasoningConfigOption(process.CapsSnapshot().ConfigOptions)
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
	if strings.TrimSpace(option.CurrentValueID) == effortID {
		return false, nil
	}
	if err := d.applySessionConfigOption(ctx, process, SessionConfigOptionSelection{
		ID:      option.ID,
		ValueID: effortID,
	}); err != nil {
		if errors.Is(err, errConfigOptionUnconfirmed) {
			// The value is advertised but the provider never confirmed it: report the
			// cause without claiming the value is absent from the choices.
			return true, newNegotiationError(
				NegotiationCodeReasoningEffortUnsupported,
				"reasoning effort",
				effortID,
				option.ID,
				nil,
				err,
			)
		}
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

// applySessionConfigOption applies one selection and confirms it through the shared
// wait, without replacing a concurrent provider observation.
func (d *Driver) applySessionConfigOption(
	ctx context.Context,
	process *AgentProcess,
	selection SessionConfigOptionSelection,
) error {
	// The confirmation wait lives here, in the path every selection uses, so model,
	// generic options and reasoning effort all survive a late config_option_update.
	apply := func(ctx context.Context) error {
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
		process.setConfigOptionsResponse(sessionConfigOptionsFromSDK(response.ConfigOptions), revision)
		return nil
	}
	confirmed := func() bool {
		option, ok := findConfigOptionByID(process.CapsSnapshot().ConfigOptions, selection.ID)
		return ok && selection.matches(option)
	}
	if err := d.applyConfigOption(ctx, process, apply, confirmed); err != nil {
		if errors.Is(err, errConfigOptionUnconfirmed) {
			return fmt.Errorf(
				"acp: provider did not confirm config option %q: %w",
				selection.ID,
				err,
			)
		}
		return err
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
		return []string{"acceptEdits", "accept-edits", "workspace-write", "accept_edits", "auto_edit"}
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
