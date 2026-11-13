package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

const (
	sessionConfigModelKey        = "model"
	sessionConfigModeKey         = "mode"
	sessionModeAgent             = "agent"
	sessionModeAsk               = "ask"
	sessionModeBypassPermissions = "bypassPermissions"
)

const (
	modelCatalogFirstBackoff = 250 * time.Millisecond
	modelCatalogMaxBackoff   = 2 * time.Second
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

	option, err := d.awaitSessionModelOption(ctx, process, modelID)
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

// awaitSessionModelOption reads the model option from the current snapshot and, when the
// advertised catalog does not carry the value yet, waits for the config_option_update that
// providers such as opencode publish only after session/new.
func (d *Driver) awaitSessionModelOption(
	ctx context.Context,
	process *AgentProcess,
	modelID string,
) (SessionConfigOption, error) {
	options := process.CapsSnapshot().ConfigOptions
	option, ok := ModelConfigOption(options)
	if !ok {
		return SessionConfigOption{}, ValidateModelConfigValue(options, modelID)
	}
	if configOptionAllowsValue(option, modelID) {
		return option, nil
	}
	if option.ReadOnly {
		// Discovery-only options never gain writable values after session/new.
		return SessionConfigOption{}, ValidateModelConfigValue(options, modelID)
	}

	budget := defaultModelCatalogWait
	if d != nil && d.modelCatalogWait > 0 {
		budget = d.modelCatalogWait
	}
	deadline := time.Now().Add(budget)
	backoff := modelCatalogFirstBackoff
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if backoff > remaining {
			backoff = remaining
		}
		if err := waitForCatalogUpdate(ctx, process, backoff); err != nil {
			return SessionConfigOption{}, err
		}
		options = process.CapsSnapshot().ConfigOptions
		if announced, found := ModelConfigOption(options); found &&
			configOptionAllowsValue(announced, modelID) {
			return announced, nil
		}
		if backoff < modelCatalogMaxBackoff {
			backoff = min(backoff*2, modelCatalogMaxBackoff)
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

func waitForCatalogUpdate(ctx context.Context, process *AgentProcess, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var exited <-chan struct{}
	if process != nil {
		exited = process.Done()
	}
	select {
	case <-timer.C:
		return nil
	case <-exited:
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("acp: wait for session model catalog: %w", err)
		}
		return catalogWaitProcessExitError(process)
	case <-ctx.Done():
		return fmt.Errorf("acp: wait for session model catalog: %w", ctx.Err())
	}
}

// catalogWaitProcessExitError reports a provider that died before it could publish
// the catalog. The announcement can never arrive, so the wait fails fast instead
// of spending the readiness budget and blaming the model with model_unavailable.
func catalogWaitProcessExitError(process *AgentProcess) error {
	cause := errProcessExited
	if waitErr := process.Wait(); waitErr != nil {
		cause = errors.Join(cause, waitErr)
	}
	return WrapFailure(
		store.FailureProcess,
		"ACP subprocess exited while waiting for the model catalog",
		fmt.Errorf("acp: wait for session model catalog: %w", cause),
	)
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

// applySessionReasoningEffort rejects unadvertised intent before requesting provider acknowledgement.
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
	if strings.TrimSpace(option.CurrentValueID) == effortID {
		return false, nil
	}
	if err := d.applySessionConfigOption(ctx, process, SessionConfigOptionSelection{
		ID:      option.ID,
		ValueID: effortID,
	}); err != nil {
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
