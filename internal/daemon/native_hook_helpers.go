package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	hookspkg "github.com/compozy/agh/internal/hooks"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func hookCatalogFilter(
	id toolspkg.ToolID,
	input hooksListInput,
	scope toolspkg.Scope,
) (hookspkg.CatalogFilter, error) {
	workspaceID, err := nativeCallerWorkspaceInput(id, "workspace_id", input.WorkspaceID, scope)
	if err != nil {
		return hookspkg.CatalogFilter{}, err
	}
	filter := hookspkg.CatalogFilter{
		WorkspaceID:   workspaceID,
		WorkspaceRoot: strings.TrimSpace(input.WorkspaceRoot),
		AgentName:     strings.TrimSpace(input.Agent),
	}
	if event := strings.TrimSpace(input.Event); event != "" {
		parsed := hookspkg.HookEvent(event)
		if err := parsed.Validate(); err != nil {
			return hookspkg.CatalogFilter{}, err
		}
		filter.Event = parsed
	}
	if source := strings.TrimSpace(input.Source); source != "" {
		var parsed hookspkg.HookSource
		if err := parsed.UnmarshalText([]byte(source)); err != nil {
			return hookspkg.CatalogFilter{}, err
		}
		filter.Source = &parsed
	}
	if mode := strings.TrimSpace(input.Mode); mode != "" {
		parsed := hookspkg.HookMode(mode)
		if err := parsed.Validate(); err != nil {
			return hookspkg.CatalogFilter{}, err
		}
		filter.Mode = parsed
	}
	return filter, nil
}

func (n *daemonNativeTools) rejectImmutableHookIfPresent(
	ctx context.Context,
	id toolspkg.ToolID,
	name string,
) error {
	if n.deps.Observer == nil {
		return nil
	}
	entries, err := n.deps.Observer.QueryHookCatalog(ctx, hookspkg.CatalogFilter{})
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(name)
	for _, entry := range entries {
		if entry.Name != trimmed {
			continue
		}
		if entry.Source != hookspkg.HookSourceConfig {
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeDenied,
				id,
				fmt.Sprintf("hook %q source %q is immutable by tools", trimmed, entry.Source.String()),
				toolspkg.ErrToolDenied,
				toolspkg.ReasonHookSourceImmutable,
			)
		}
		return nativeHookValidationError(id, fmt.Errorf("hook %q already exists", trimmed))
	}
	return nil
}

func (n *daemonNativeTools) deleteHookDeclaration(
	ctx context.Context,
	id toolspkg.ToolID,
	target aghconfig.WriteTarget,
	workspaceRoot string,
	name string,
) (bool, error) {
	deleted := false
	if _, err := aghconfig.EditConfigOverlay(
		n.deps.HomePaths,
		workspaceRoot,
		target,
		func(editor *aghconfig.OverlayEditor) error {
			var err error
			deleted, err = editor.DeleteArrayTableItem(
				[]string{nativeConfigHookToolsHooksKey, nativeConfigHookToolsDeclarationsKey},
				nativeConfigHookToolsNameKey,
				name,
			)
			return err
		},
	); err != nil {
		return false, nativeHookValidationError(id, err)
	}
	if !deleted {
		if err := n.rejectImmutableHookIfPresent(ctx, id, name); err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (n *daemonNativeTools) setHookEnabled(
	ctx context.Context,
	req toolspkg.CallRequest,
	enabled bool,
) (toolspkg.ToolResult, error) {
	var input hookNameMutationInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, input.Scope, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	decls, err := aghconfig.OverlayHookDeclarations(target)
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
	decl, ok := findHookDecl(decls, input.Name)
	if !ok {
		if err := n.rejectImmutableHookIfPresent(ctx, req.ToolID, input.Name); err != nil {
			return toolspkg.ToolResult{}, err
		}
		return toolspkg.ToolResult{}, nativeHookNotFoundError(req.ToolID, input.Name)
	}
	decl.Enabled = new(enabled)
	if hookEnvMapContainsSecret(decl.Env) {
		return toolspkg.ToolResult{}, nativeHookSecretError(req.ToolID)
	}
	if _, err := hookspkg.CanonicalizeHookDecl(decl); err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
	if _, err := aghconfig.EditConfigOverlay(
		n.deps.HomePaths,
		workspaceRoot,
		target,
		func(editor *aghconfig.OverlayEditor) error {
			return editor.UpsertArrayTableItem(
				[]string{nativeConfigHookToolsHooksKey, nativeConfigHookToolsDeclarationsKey},
				nativeConfigHookToolsNameKey,
				decl.Name,
				aghconfig.HookDeclarationOverlayValues(decl),
			)
		},
	); err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
	action := hookActionDisabled
	if enabled {
		action = hookActionEnabled
	}
	return nativeHookMutationResult(decl, action, target)
}

func nativeHookMutableSource(id toolspkg.ToolID, source *string) error {
	if source == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*source)
	if trimmed == "" || trimmed == hookspkg.HookSourceConfig.String() {
		return nil
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("hook source %q is immutable by tools", trimmed),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonHookSourceImmutable,
	)
}

func nativeHookSecretError(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		"hook executor env contains a forbidden secret-bearing input",
		toolspkg.ErrToolDenied,
		toolspkg.ReasonHookSecretInputForbidden,
	)
}

func nativeHookValidationError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"hook declaration validation failed",
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonHookValidationFailed,
	)
}

func nativeHookCatalogFilterError(id toolspkg.ToolID, err error) error {
	if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok && toolErr != nil {
		return err
	}
	return nativeHookValidationError(id, err)
}

func nativeHookNotFoundError(id toolspkg.ToolID, name string) error {
	trimmed := strings.TrimSpace(name)
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeNotFound,
		id,
		fmt.Sprintf("hook %q not found in target config overlay", trimmed),
		toolspkg.ErrToolNotFound,
		toolspkg.ReasonToolUnknown,
	)
}

func nativeHookMutationResult(
	decl hookspkg.HookDecl,
	action string,
	target aghconfig.WriteTarget,
) (toolspkg.ToolResult, error) {
	payload := map[string]any{
		nativeConfigHookToolsNameKey:   decl.Name,
		"action":                       action,
		nativeConfigHookToolsScopeKey:  string(target.Scope()),
		nativeConfigHookToolsTargetKey: target.Path(),
		nativeConfigHookToolsHookKey:   nativeHookDeclPayload(decl),
	}
	if err := addNativeConfigLifecycleFields(payload, "hooks.declarations"); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(payload, decl.Name)
}

func addNativeConfigLifecycleFields(payload map[string]any, path string) error {
	rule, err := lifecycle.ClassifyPath(path)
	if err != nil {
		return err
	}
	applied := rule.Lifecycle != lifecycle.RestartRequired
	status := lifecycle.StatusApplied
	if !applied {
		status = lifecycle.StatusBlocked
	}
	payload[nativeAppliedValue] = applied
	payload[nativeConfigHookToolsLifecycleKey] = string(rule.Lifecycle)
	payload[nativeConfigHookToolsNextActionKey] = string(lifecycle.NextActionForLifecycle(rule.Lifecycle, status))
	return nil
}

func nativeHookDeclPayload(decl hookspkg.HookDecl) map[string]any {
	payload := map[string]any{
		nativeConfigHookToolsNameKey: decl.Name,
		"event":                      decl.Event.String(),
		"source":                     decl.Source.String(),
		"mode":                       string(decl.Mode),
		"required":                   decl.Required,
		"priority":                   decl.Priority,
		"executor_kind":              string(decl.ExecutorKind),
		"command":                    decl.Command,
		"args":                       append([]string(nil), decl.Args...),
		"env":                        cloneStringMap(decl.Env),
		"secret_env":                 cloneStringMap(decl.SecretEnv),
		"matcher":                    decl.Matcher,
	}
	if decl.Enabled != nil {
		payload[hookEnabledKey] = *decl.Enabled
	} else {
		payload[hookEnabledKey] = true
	}
	if decl.Timeout > 0 {
		payload["timeout_ms"] = decl.Timeout.Milliseconds()
	}
	return payload
}

func findHookDecl(decls []hookspkg.HookDecl, name string) (hookspkg.HookDecl, bool) {
	trimmed := strings.TrimSpace(name)
	for _, decl := range decls {
		if strings.EqualFold(strings.TrimSpace(decl.Name), trimmed) {
			return decl, true
		}
	}
	return hookspkg.HookDecl{}, false
}

func hookEnvContainsSecret(env *map[string]string) bool {
	if env == nil {
		return false
	}
	return hookEnvMapContainsSecret(*env)
}

func hookEnvMapContainsSecret(env map[string]string) bool {
	for key, value := range env {
		if secretLikeText(key) || secretLikeText(value) {
			return true
		}
	}
	return false
}

func secretLikeText(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	needles := []string{
		"secret",
		nativeConfigHookToolsTokenKey,
		"password",
		"api_key",
		"apikey",
		"authorization",
		"bearer",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return strings.HasPrefix(lower, "sk-")
}

func cloneHookMatcher(src hookspkg.HookMatcher) hookspkg.HookMatcher {
	cloned := src
	if src.ToolReadOnly != nil {
		value := *src.ToolReadOnly
		cloned.ToolReadOnly = &value
	}
	if src.Autonomy != nil {
		value := *src.Autonomy
		cloned.Autonomy = &value
	}
	return cloned
}
