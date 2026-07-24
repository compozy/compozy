package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	sessionConfigModelID           = "model"
	sessionConfigEffortID          = "effort"
	sessionConfigReasoningEffortID = "reasoning_effort"
	claudeFableModelAlias          = "fable"
	claudeFableModelID             = "claude-fable-5"
	claudeAutoModeID               = "auto"
	reasoningEffortMax             = "max"
	reasoningEffortUltra           = "ultra"
)

// configureSession applies the model, reasoning, and mode configuration to a new
// or resumed session and reports the model the runtime accepted, which can differ
// from the requested one when an inherited model falls back.
func (c *clientImpl) configureSession(
	ctx context.Context,
	sessionID acp.SessionId,
	requestedModel string,
	options []acp.SessionConfigOption,
	modes *acp.SessionModeState,
) (string, error) {
	effectiveModel, err := c.configureSessionModel(ctx, sessionID, requestedModel, options)
	if err != nil {
		return "", err
	}
	if err := c.configureSessionReasoning(ctx, sessionID, options); err != nil {
		return "", err
	}
	if err := c.configureSessionMode(ctx, sessionID, effectiveModel, modes); err != nil {
		return "", err
	}
	return effectiveModel, nil
}

func (c *clientImpl) configureSessionModel(
	ctx context.Context,
	sessionID acp.SessionId,
	requestedModel string,
	options []acp.SessionConfigOption,
) (string, error) {
	modelInput := firstNonEmpty(requestedModel, c.cfg.Model)
	effectiveModel := resolveModel(c.spec, modelInput)
	leaveModelToRuntime := strings.EqualFold(strings.TrimSpace(modelInput), "auto")

	modelOption := findSessionSelectOption(
		options,
		acp.SessionConfigOptionCategoryModel,
		sessionConfigModelID,
	)
	if modelOption != nil {
		return c.configureAdvertisedSessionModel(
			ctx,
			sessionID,
			modelOption,
			effectiveModel,
			leaveModelToRuntime,
		)
	}
	if leaveModelToRuntime || strings.TrimSpace(effectiveModel) == "" {
		return effectiveModel, nil
	}
	if requiresAdvertisedSessionModel(c.spec) && !c.usesLegacyCodexACP() {
		return "", wrapSessionSetupError(
			SessionSetupStageSetModel,
			fmt.Errorf(
				"%s did not advertise an ACP model option; cannot resolve requested model %q",
				c.spec.DisplayName,
				effectiveModel,
			),
		)
	}
	if c.spec.UsesBootstrapModel {
		return effectiveModel, nil
	}
	if err := c.setSessionConfigValue(
		ctx,
		sessionID,
		acp.SessionConfigId(sessionConfigModelID),
		effectiveModel,
		SessionSetupStageSetModel,
		"set ACP session model",
	); err != nil {
		return "", err
	}
	return effectiveModel, nil
}

func (c *clientImpl) configureAdvertisedSessionModel(
	ctx context.Context,
	sessionID acp.SessionId,
	modelOption *acp.SessionConfigOptionSelect,
	effectiveModel string,
	leaveModelToRuntime bool,
) (string, error) {
	if leaveModelToRuntime {
		return string(modelOption.CurrentValue), nil
	}
	resolvedModel, err := resolveSessionSelectValue(modelOption, effectiveModel, "model")
	if err != nil {
		fallback, ok := c.inheritedModelFallback(modelOption, effectiveModel)
		if !ok {
			if c.spec.ID == model.IDECursor {
				err = fmt.Errorf(
					"%w; Cursor CLI --list-models entries can differ from ACP model IDs; "+
						"use an ACP name or value from the valid choices",
					err,
				)
			}
			return "", wrapSessionSetupError(SessionSetupStageSetModel, err)
		}
		resolvedModel = fallback
	}
	if err := c.setSessionConfigValue(
		ctx,
		sessionID,
		modelOption.Id,
		resolvedModel,
		SessionSetupStageSetModel,
		"set ACP session model",
	); err != nil {
		return "", err
	}
	return resolvedModel, nil
}

// inheritedModelFallback resolves a model the runtime does not advertise down to
// that runtime's own current default. A workspace, task-rule, or agent default is
// not a statement about which runtime the session lands on, so a cross-runtime
// value must not fail the session. An explicitly pinned model stays a hard error:
// running a model other than the one requested is worse than failing.
//
// ModelExplicit only distinguishes a --model flag from everything else, while
// RuntimeForTask gives type and id task rules authority over that flag. Such a
// rule is therefore correctable here even though it outranks the flag upstream.
func (c *clientImpl) inheritedModelFallback(
	option *acp.SessionConfigOptionSelect,
	requested string,
) (string, bool) {
	if c.cfg.ModelExplicit || option == nil {
		return "", false
	}
	// A current value the runtime does not list among its own options cannot be a
	// safer choice than the request, so leave the original error in place.
	current := strings.TrimSpace(string(option.CurrentValue))
	if current == "" || strings.EqualFold(current, strings.TrimSpace(requested)) {
		return "", false
	}
	if c.logger != nil {
		c.logger.Warn(
			"inherited model is not available on this runtime; falling back to the runtime default",
			"runtime", c.spec.ID,
			"requested_model", requested,
			"resolved_model", current,
		)
	}
	return current, true
}

func (c *clientImpl) configureSessionReasoning(
	ctx context.Context,
	sessionID acp.SessionId,
	options []acp.SessionConfigOption,
) error {
	reasoning := strings.TrimSpace(c.cfg.ReasoningEffort)
	if reasoning == "" {
		return nil
	}
	reasoningOption := findSessionSelectOption(
		options,
		acp.SessionConfigOptionCategoryThoughtLevel,
		sessionConfigReasoningEffortID,
		sessionConfigEffortID,
	)
	if reasoningOption == nil {
		if c.spec.ID != model.IDECodex || c.usesLegacyCodexACP() {
			return nil
		}
		return wrapSessionSetupError(
			SessionSetupStageSetReasoning,
			fmt.Errorf(
				"%s did not advertise an ACP reasoning option; cannot apply reasoning effort %q",
				c.spec.DisplayName,
				reasoning,
			),
		)
	}
	resolvedReasoning, err := resolveSessionSelectValue(reasoningOption, reasoning, "reasoning effort")
	if err != nil {
		return wrapSessionSetupError(SessionSetupStageSetReasoning, err)
	}
	return c.setSessionConfigValue(
		ctx,
		sessionID,
		reasoningOption.Id,
		resolvedReasoning,
		SessionSetupStageSetReasoning,
		"set ACP session reasoning effort",
	)
}

func (c *clientImpl) configureSessionMode(
	ctx context.Context,
	sessionID acp.SessionId,
	effectiveModel string,
	modes *acp.SessionModeState,
) error {
	modeID := sessionModeForModelAccess(c.spec, effectiveModel, c.cfg.AccessMode)
	if modeID == "" {
		return nil
	}
	if isClaudeFableModel(c.spec, effectiveModel) && c.cfg.AccessMode == model.AccessModeFull && c.logger != nil {
		c.logger.Info(
			"using Claude auto permission mode required by Fable 5",
			"requested_access_mode", c.cfg.AccessMode,
			"effective_mode", modeID,
			"model", effectiveModel,
		)
	}
	shouldSet, err := c.shouldSetSessionMode(modes, modeID)
	if err != nil {
		return err
	}
	if !shouldSet {
		return nil
	}
	if _, err := c.conn.SetSessionMode(ctx, acp.SetSessionModeRequest{
		SessionId: sessionID,
		ModeId:    acp.SessionModeId(modeID),
	}); err != nil {
		return c.wrapACPSetupErrorWithDiagnostics(
			ctx,
			SessionSetupStageSetMode,
			"set ACP session mode",
			err,
		)
	}
	return nil
}

func (c *clientImpl) shouldSetSessionMode(modes *acp.SessionModeState, modeID string) (bool, error) {
	if modes != nil && !sessionModeAvailable(modes, modeID) {
		if c.usesLegacyCodexACP() {
			// Legacy Codex ACP adapters configure access at process bootstrap and do not
			// advertise the current session-mode surface.
			return false, nil
		}
		return false, wrapSessionSetupError(
			SessionSetupStageSetMode,
			fmt.Errorf(
				"ACP session mode %q is not available; valid modes: %s",
				modeID,
				strings.Join(sessionModeIDs(modes), ", "),
			),
		)
	}
	if c.spec.ID != model.IDECodex || modes != nil {
		return true, nil
	}
	if c.usesLegacyCodexACP() {
		return false, nil
	}
	return false, wrapSessionSetupError(
		SessionSetupStageSetMode,
		fmt.Errorf(
			"%s did not advertise ACP session modes; cannot apply mode %q",
			c.spec.DisplayName,
			modeID,
		),
	)
}

func requiresAdvertisedSessionModel(spec Spec) bool {
	return spec.ID == model.IDECursor || spec.ID == model.IDECodex
}

func (c *clientImpl) usesLegacyCodexACP() bool {
	return c.spec.ID == model.IDECodex && launcherUsesLegacyCodexACP(c.spec.primaryLauncher())
}

func (c *clientImpl) setSessionConfigValue(
	ctx context.Context,
	sessionID acp.SessionId,
	configID acp.SessionConfigId,
	value string,
	stage SessionSetupStage,
	operation string,
) error {
	_, err := c.conn.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{
			SessionId: sessionID,
			ConfigId:  configID,
			Value:     acp.SessionConfigValueId(value),
		},
	})
	if err != nil {
		return c.wrapACPSetupErrorWithDiagnostics(ctx, stage, operation, err)
	}
	return nil
}

func findSessionSelectOption(
	options []acp.SessionConfigOption,
	category acp.SessionConfigOptionCategory,
	ids ...string,
) *acp.SessionConfigOptionSelect {
	for _, id := range ids {
		for i := range options {
			option := options[i].Select
			if option != nil && strings.EqualFold(strings.TrimSpace(string(option.Id)), id) {
				return option
			}
		}
	}
	for i := range options {
		option := options[i].Select
		if option != nil && option.Category != nil && *option.Category == category {
			return option
		}
	}
	return nil
}

func resolveSessionSelectValue(
	option *acp.SessionConfigOptionSelect,
	requested string,
	label string,
) (string, error) {
	requested = strings.TrimSpace(requested)
	values := sessionSelectValues(option)
	for _, candidate := range values {
		if string(candidate.Value) == requested {
			return string(candidate.Value), nil
		}
	}
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(string(candidate.Value)), requested) {
			return string(candidate.Value), nil
		}
	}
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(candidate.Name), requested) {
			return string(candidate.Value), nil
		}
	}

	choices := make([]string, 0, len(values))
	for _, candidate := range values {
		choices = append(choices, fmt.Sprintf("%s (%s)", candidate.Name, candidate.Value))
	}
	slices.Sort(choices)
	return "", fmt.Errorf(
		"%s %q is not available; valid choices: %s",
		label,
		requested,
		strings.Join(choices, ", "),
	)
}

func sessionSelectValues(option *acp.SessionConfigOptionSelect) []acp.SessionConfigSelectOption {
	if option == nil {
		return nil
	}
	if option.Options.Ungrouped != nil {
		return append([]acp.SessionConfigSelectOption(nil), (*option.Options.Ungrouped)...)
	}
	if option.Options.Grouped == nil {
		return nil
	}
	var values []acp.SessionConfigSelectOption
	for _, group := range *option.Options.Grouped {
		values = append(values, group.Options...)
	}
	return values
}

func sessionModeForModelAccess(spec Spec, modelName string, accessMode string) string {
	if isClaudeFableModel(spec, modelName) {
		return claudeAutoModeID
	}
	return spec.sessionModeForAccess(accessMode)
}

func isClaudeFableModel(spec Spec, modelName string) bool {
	if spec.ID != model.IDEClaude {
		return false
	}
	_, ok := canonicalClaudeFableModel(modelName)
	return ok
}

func canonicalClaudeFableModel(modelName string) (string, bool) {
	trimmed := strings.TrimSpace(modelName)
	base, parameters, parameterized := strings.Cut(trimmed, "[")
	if provider, candidate, ok := strings.Cut(base, "/"); ok && strings.TrimSpace(provider) != "" {
		base = candidate
	}
	switch strings.ToLower(strings.TrimSpace(base)) {
	case claudeFableModelAlias, "fable-5", claudeFableModelID:
		if parameterized {
			return claudeFableModelID + "[" + parameters, true
		}
		return claudeFableModelID, true
	default:
		return "", false
	}
}

func sessionModeAvailable(modes *acp.SessionModeState, modeID string) bool {
	if modes == nil {
		return false
	}
	for _, available := range modes.AvailableModes {
		if string(available.Id) == modeID {
			return true
		}
	}
	return false
}

func sessionModeIDs(modes *acp.SessionModeState) []string {
	if modes == nil {
		return nil
	}
	ids := make([]string, 0, len(modes.AvailableModes))
	for _, available := range modes.AvailableModes {
		ids = append(ids, string(available.Id))
	}
	slices.Sort(ids)
	return ids
}
