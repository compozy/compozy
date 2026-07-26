package daemon

import (
	"context"

	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeConfigHookToolsDeclarationsKey  = "declarations"
	nativeConfigHookToolsDeletedKey       = "deleted"
	nativeConfigHookToolsDiffKey          = "diff"
	nativeConfigHookToolsEventsKey        = "events"
	nativeAppliedValue                    = "applied"
	nativeConfigHookToolsHookKey          = "hook"
	nativeConfigHookToolsLifecycleKey     = "lifecycle"
	nativeConfigHookToolsHooksKey         = "hooks"
	nativeConfigHookToolsNameKey          = "name"
	nativeConfigHookToolsNextActionKey    = "next_action"
	nativeConfigHookToolsPathKey          = "path"
	nativeConfigHookToolsRedactedKey      = "redacted"
	nativeConfigHookToolsRunsKey          = "runs"
	nativeConfigHookToolsScopeKey         = "scope"
	nativeConfigHookToolsTargetKey        = "target"
	nativeConfigHookToolsTokenKey         = "token"
	nativeConfigHookToolsValueKey         = "value"
	nativeConfigHookToolsWorkspaceRootKey = "workspace_root"
)

const (
	hookActionDisabled = "disabled"
	hookActionEnabled  = "enabled"
	hookEnabledKey     = "enabled"
)

func (n *daemonNativeTools) configShow(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	cfg, workspaceRoot, err := n.loadNativeConfig(input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	configMap := aghconfig.RedactedConfigMap(&cfg)
	return structuredResult(map[string]any{
		nativeConfigHookToolsScopeKey:         nativeScopeForWorkspace(workspaceRoot),
		nativeConfigHookToolsWorkspaceRootKey: workspaceRoot,
		nativeConfigHookToolsRedactedKey:      true,
		"config":                              configMap,
	}, "config")
}

func (n *daemonNativeTools) configList(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	cfg, workspaceRoot, err := n.loadNativeConfig(input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	entries := aghconfig.FlattenConfigEntries(aghconfig.RedactedConfigMap(&cfg))
	return structuredResult(map[string]any{
		nativeConfigHookToolsScopeKey:         nativeScopeForWorkspace(workspaceRoot),
		nativeConfigHookToolsWorkspaceRootKey: workspaceRoot,
		nativeConfigHookToolsRedactedKey:      true,
		"entries":                             entries,
	}, fmt.Sprintf("%d config entries", len(entries)))
}

func (n *daemonNativeTools) configGet(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configGetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	cfg, workspaceRoot, err := n.loadNativeConfig(input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	entries := aghconfig.FlattenConfigEntries(aghconfig.RedactedConfigMap(&cfg))
	entry, ok := aghconfig.EntryByPath(entries, input.Path)
	if !ok {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			req.ToolID,
			fmt.Sprintf("config path %q not found", strings.TrimSpace(input.Path)),
			toolspkg.ErrToolNotFound,
			toolspkg.ReasonToolUnknown,
		)
	}
	return structuredResult(map[string]any{
		nativeConfigHookToolsScopeKey:         nativeScopeForWorkspace(workspaceRoot),
		nativeConfigHookToolsWorkspaceRootKey: workspaceRoot,
		"entry":                               entry,
	}, entry.Path)
}

func (n *daemonNativeTools) configDiff(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceRoot, err := nativeOptionalWorkspaceRoot(input.WorkspaceRoot)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	var beforeCfg aghconfig.Config
	if workspaceRoot == "" {
		beforeCfg = aghconfig.DefaultWithHome(n.deps.HomePaths)
	} else {
		beforeCfg, err = aghconfig.LoadForHome(n.deps.HomePaths)
		if err != nil {
			return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
		}
	}
	loadOptions := []aghconfig.LoadOption{}
	if workspaceRoot != "" {
		loadOptions = append(loadOptions, aghconfig.WithWorkspaceRoot(workspaceRoot))
	}
	afterCfg, err := aghconfig.LoadForHome(n.deps.HomePaths, loadOptions...)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigValidationError(req.ToolID, err)
	}
	before := aghconfig.FlattenConfigEntries(aghconfig.RedactedConfigMap(&beforeCfg))
	after := aghconfig.FlattenConfigEntries(aghconfig.RedactedConfigMap(&afterCfg))
	diff := aghconfig.DiffConfigEntries(before, after)
	return structuredResult(map[string]any{
		nativeConfigHookToolsScopeKey:         nativeScopeForWorkspace(workspaceRoot),
		nativeConfigHookToolsWorkspaceRootKey: workspaceRoot,
		nativeConfigHookToolsRedactedKey:      true,
		nativeConfigHookToolsDiffKey:          diff,
	}, fmt.Sprintf("%d config differences", len(diff)))
}

func (n *daemonNativeTools) configPath(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input configPathInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	scope, err := nativeWriteScope(input.Scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	globalConfig, err := aghconfig.ResolveConfigWriteTarget(n.deps.HomePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	globalMCP, err := aghconfig.ResolveMCPSidecarWriteTarget(n.deps.HomePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
	}
	selected := globalConfig
	record := map[string]any{
		"home_dir":                    n.deps.HomePaths.HomeDir,
		"global_config":               globalConfig.Path(),
		"global_mcp_json":             globalMCP.Path(),
		nativeConfigHookToolsScopeKey: string(scope),
		"selected_config_target":      selected.Path(),
	}
	if scope == aghconfig.WriteScopeWorkspace || strings.TrimSpace(input.WorkspaceRoot) != "" {
		workspaceRoot, err := nativeRequiredWorkspaceRoot(input.WorkspaceRoot)
		if err != nil {
			return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
		}
		workspaceConfig, err := aghconfig.ResolveConfigWriteTarget(
			n.deps.HomePaths,
			workspaceRoot,
			aghconfig.WriteScopeWorkspace,
		)
		if err != nil {
			return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
		}
		workspaceMCP, err := aghconfig.ResolveMCPSidecarWriteTarget(
			n.deps.HomePaths,
			workspaceRoot,
			aghconfig.WriteScopeWorkspace,
		)
		if err != nil {
			return toolspkg.ToolResult{}, nativeConfigScopeError(req.ToolID, err)
		}
		record[nativeConfigHookToolsWorkspaceRootKey] = workspaceRoot
		record["workspace_config"] = workspaceConfig.Path()
		record["workspace_mcp_json"] = workspaceMCP.Path()
		if scope == aghconfig.WriteScopeWorkspace {
			selected = workspaceConfig
			record["selected_config_target"] = selected.Path()
		}
	}
	return structuredResult(record, fmt.Sprint(record["selected_config_target"]))
}
