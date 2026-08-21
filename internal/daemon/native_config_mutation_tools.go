package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) configSet(
	ctx context.Context,
	scope toolspkg.Scope,
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
	value, err := compozyconfig.NormalizeToolConfigValue(policy.Kind, input.Value)
	if err != nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			"config value is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigValidationFailed,
		)
	}
	workspaceRoot, err := n.nativeWorkspaceRoot(ctx, req.ToolID, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, scope, input.Scope, workspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := compozyconfig.ValidateConfigWriteScope(target.Scope(), policy.Segments); err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	path := strings.Join(policy.Segments, ".")
	if policy.Kind == compozyconfig.ConfigValueLoopInput {
		return n.setNativeLoopInputDefault(ctx, scope, req.ToolID, target, workspaceRoot, policy.Segments, value)
	}
	applyService, err := n.nativeConfigApplyService(req.ToolID, target, path)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := compozyconfig.EditConfigOverlay(
		n.deps.HomePaths,
		workspaceRoot,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if policy.Kind == compozyconfig.ConfigValueTable ||
				policy.Kind == compozyconfig.ConfigValueLoopInput {
				table, ok := value.(map[string]any)
				if ok {
					return editor.SetTable(policy.Segments, table)
				}
				if policy.Kind == compozyconfig.ConfigValueTable {
					return fmt.Errorf("daemon: config path %q requires an object", path)
				}
			}
			return editor.SetValue(policy.Segments, value)
		},
	); err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	outputValue := value
	if policy.Redacted {
		outputValue = compozyconfig.RedactedValue()
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

func (n *daemonNativeTools) setNativeLoopInputDefault(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	target compozyconfig.WriteTarget,
	workspaceRoot string,
	path []string,
	value any,
) (toolspkg.ToolResult, error) {
	workspaceID, err := n.nativeLoopWorkspaceID(ctx, id, workspaceRoot, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	inputScope := contract.LoopInputDefaultsScope(target.Scope())
	service := n.loopService()
	if service == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			"Loop input-default service is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	response, err := service.PutLoopInputDefault(
		ctx,
		workspaceID,
		scope.ProfileID,
		path[2],
		path[3],
		contract.PutLoopInputDefaultRequest{Scope: inputScope, Value: value},
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(id, err)
	}
	payload := map[string]any{
		nativeConfigHookToolsPathKey:     strings.Join(path, "."),
		nativeConfigHookToolsValueKey:    response.Value,
		nativeConfigHookToolsScopeKey:    string(target.Scope()),
		nativeConfigHookToolsTargetKey:   target.Path(),
		nativeConfigHookToolsRedactedKey: false,
		nativeAppliedValue:               true,
	}
	if err := addNativeConfigLifecycleFields(payload, strings.Join(path, ".")); err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(id, err)
	}
	return structuredResult(payload, strings.Join(path, "."))
}

func (n *daemonNativeTools) configUnset(
	ctx context.Context,
	scope toolspkg.Scope,
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
	workspaceRoot, err := n.nativeWorkspaceRoot(ctx, req.ToolID, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, scope, input.Scope, workspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := compozyconfig.ValidateConfigWriteScope(target.Scope(), policy.Segments); err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	path := strings.Join(policy.Segments, ".")
	applyService, err := n.nativeConfigApplyService(req.ToolID, target, path)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	deleted := false
	if _, err := compozyconfig.EditConfigOverlay(
		n.deps.HomePaths,
		workspaceRoot,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
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
	target compozyconfig.WriteTarget,
	path string,
) (core.SettingsService, error) {
	rule, err := lifecycle.ClassifyPath(path)
	if err != nil {
		return nil, nativeConfigValidationError(id, err)
	}
	if target.Scope() != compozyconfig.WriteScopeUser || rule.Lifecycle == lifecycle.RestartRequired {
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
