package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	settingspkg "github.com/compozy/agh/internal/settings"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) configSet(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configSetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	policy, err := nativeConfigPathPolicy(req.ToolID, input.Path)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	value, err := aghconfig.NormalizeToolConfigValue(policy.Kind, input.Value)
	if err != nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			"config value is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigValidationFailed,
		)
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, input.Scope, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := aghconfig.ValidateConfigWriteScope(target.Scope(), policy.Segments); err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	path := strings.Join(policy.Segments, ".")
	applyService, err := n.nativeConfigApplyService(req.ToolID, target, path)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := aghconfig.EditConfigOverlay(
		n.deps.HomePaths,
		workspaceRoot,
		target,
		func(editor *aghconfig.OverlayEditor) error {
			if policy.Kind == aghconfig.ConfigValueTable {
				table, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("daemon: config path %q requires an object", path)
				}
				return editor.SetTable(policy.Segments, table)
			}
			return editor.SetValue(policy.Segments, value)
		},
	); err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	outputValue := value
	if policy.Redacted {
		outputValue = aghconfig.RedactedValue()
	}
	payload := map[string]any{
		nativeConfigHookToolsPathKey:     path,
		nativeConfigHookToolsValueKey:    outputValue,
		nativeConfigHookToolsScopeKey:    string(target.Scope()),
		nativeConfigHookToolsTargetKey:   target.Path(),
		nativeConfigHookToolsRedactedKey: policy.Redacted,
	}
	if err := applyNativeConfigLifecycle(ctx, req.ToolID, applyService, payload, path); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(payload, path)
}

func (n *daemonNativeTools) configUnset(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configUnsetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	policy, err := nativeConfigPathPolicy(req.ToolID, input.Path)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, input.Scope, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := aghconfig.ValidateConfigWriteScope(target.Scope(), policy.Segments); err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	path := strings.Join(policy.Segments, ".")
	applyService, err := n.nativeConfigApplyService(req.ToolID, target, path)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	deleted := false
	if _, err := aghconfig.EditConfigOverlay(
		n.deps.HomePaths,
		workspaceRoot,
		target,
		func(editor *aghconfig.OverlayEditor) error {
			deleted = editor.HasPath(policy.Segments)
			return editor.Delete(policy.Segments)
		},
	); err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	payload := map[string]any{
		nativeConfigHookToolsPathKey:    path,
		nativeConfigHookToolsDeletedKey: deleted,
		nativeConfigHookToolsScopeKey:   string(target.Scope()),
		nativeConfigHookToolsTargetKey:  target.Path(),
	}
	if err := applyNativeConfigLifecycle(ctx, req.ToolID, applyService, payload, path); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(payload, path)
}

func (n *daemonNativeTools) nativeConfigApplyService(
	id toolspkg.ToolID,
	target aghconfig.WriteTarget,
	path string,
) (core.SettingsService, error) {
	rule, err := lifecycle.ClassifyPath(path)
	if err != nil {
		return nil, nativeConfigValidationError(id, err)
	}
	if target.Scope() != aghconfig.WriteScopeGlobal || rule.Lifecycle == lifecycle.RestartRequired {
		return nil, nil
	}
	service := n.settingsService()
	if service == nil {
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			"settings apply service is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	return service, nil
}

func applyNativeConfigLifecycle(
	ctx context.Context,
	id toolspkg.ToolID,
	service core.SettingsService,
	payload map[string]any,
	path string,
) error {
	if service == nil {
		if err := addNativeConfigLifecycleFields(payload, path); err != nil {
			return nativeConfigValidationError(id, err)
		}
		return nil
	}
	result, err := service.Reload(settingspkg.WithMutationSource(ctx, id.String()))
	if err != nil {
		return nativeHTTPStatusToolError(id, err, core.StatusForSettingsError(err))
	}
	addNativeConfigApplyFields(payload, core.SettingsApplyResponseFromResult(result))
	return nil
}

func addNativeConfigApplyFields(payload map[string]any, apply contract.SettingsApplyResponse) {
	payload[nativeAppliedValue] = apply.Applied
	payload[nativeConfigHookToolsLifecycleKey] = apply.Lifecycle
	payload["apply_record_id"] = apply.ApplyRecordID
	payload["active_generation"] = apply.ActiveGeneration
	payload["active_config_hash"] = apply.ActiveConfigHash
	payload[nativeConfigHookToolsNextActionKey] = apply.NextAction
	if apply.RestartRequired {
		payload["restart_required"] = true
	}
	if apply.RestartScope != "" {
		payload["restart_scope"] = apply.RestartScope
	}
	if len(apply.Warnings) > 0 {
		payload["warnings"] = apply.Warnings
	}
	if len(apply.PartialFailures) > 0 {
		payload["partial_failures"] = apply.PartialFailures
	}
	if apply.Skipped {
		payload["skipped"] = true
		payload["skipped_reason"] = apply.SkippedReason
	}
}
