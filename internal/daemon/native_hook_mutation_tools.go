package daemon

import (
	"context"

	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) hooksCreate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hookMutationInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeHookMutableSource(req.ToolID, input.Source); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if hookEnvContainsSecret(input.Env) {
		return toolspkg.ToolResult{}, nativeHookSecretError(req.ToolID)
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, input.Scope, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if existing, err := aghconfig.OverlayHookDeclarations(target); err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	} else if _, ok := findHookDecl(existing, input.Name); ok {
		return toolspkg.ToolResult{}, nativeHookValidationError(
			req.ToolID,
			fmt.Errorf("hook %q already exists in %s", strings.TrimSpace(input.Name), target.Kind()),
		)
	}
	if err := n.rejectImmutableHookIfPresent(ctx, req.ToolID, input.Name); err != nil {
		return toolspkg.ToolResult{}, err
	}
	decl, err := input.newDecl()
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
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
	return nativeHookMutationResult(decl, "created", target)
}

func (n *daemonNativeTools) hooksUpdate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hookMutationInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeHookMutableSource(req.ToolID, input.Source); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if hookEnvContainsSecret(input.Env) {
		return toolspkg.ToolResult{}, nativeHookSecretError(req.ToolID)
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, input.Scope, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	decls, err := aghconfig.OverlayHookDeclarations(target)
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
	current, ok := findHookDecl(decls, input.Name)
	if !ok {
		if err := n.rejectImmutableHookIfPresent(ctx, req.ToolID, input.Name); err != nil {
			return toolspkg.ToolResult{}, err
		}
		return toolspkg.ToolResult{}, nativeHookNotFoundError(req.ToolID, input.Name)
	}
	decl, err := input.apply(current)
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
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
	return nativeHookMutationResult(decl, "updated", target)
}

func (n *daemonNativeTools) hooksDelete(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hookNameMutationInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, workspaceRoot, err := n.nativeConfigWriteTarget(req.ToolID, input.Scope, input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if deleted, err := n.deleteHookDeclaration(ctx, req.ToolID, target, workspaceRoot, input.Name); err != nil {
		return toolspkg.ToolResult{}, err
	} else if !deleted {
		return toolspkg.ToolResult{}, nativeHookNotFoundError(req.ToolID, input.Name)
	}
	payload := map[string]any{
		nativeConfigHookToolsNameKey:   strings.TrimSpace(input.Name),
		"action":                       nativeConfigHookToolsDeletedKey,
		nativeConfigHookToolsScopeKey:  string(target.Scope()),
		nativeConfigHookToolsTargetKey: target.Path(),
	}
	if err := addNativeConfigLifecycleFields(payload, "hooks.declarations"); err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
	return structuredResult(payload, strings.TrimSpace(input.Name))
}

func (n *daemonNativeTools) hooksEnable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.setHookEnabled(ctx, req, true)
}

func (n *daemonNativeTools) hooksDisable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.setHookEnabled(ctx, req, false)
}
