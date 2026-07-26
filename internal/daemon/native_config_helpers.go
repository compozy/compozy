package daemon

import (
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) loadNativeConfig(workspaceRootRaw string) (aghconfig.Config, string, error) {
	workspaceRoot, err := nativeOptionalWorkspaceRoot(workspaceRootRaw)
	if err != nil {
		return aghconfig.Config{}, "", err
	}
	loadOptions := []aghconfig.LoadOption{}
	if workspaceRoot != "" {
		loadOptions = append(loadOptions, aghconfig.WithWorkspaceRoot(workspaceRoot))
	}
	cfg, err := aghconfig.LoadForHome(n.deps.HomePaths, loadOptions...)
	if err != nil {
		return aghconfig.Config{}, "", err
	}
	return cfg, workspaceRoot, nil
}

func (n *daemonNativeTools) nativeConfigWriteTarget(
	id toolspkg.ToolID,
	scopeRaw string,
	workspaceRootRaw string,
) (aghconfig.WriteTarget, string, error) {
	scope, err := nativeWriteScope(scopeRaw)
	if err != nil {
		return aghconfig.WriteTarget{}, "", nativeConfigScopeError(id, err)
	}
	workspaceRoot := ""
	if scope == aghconfig.WriteScopeWorkspace {
		workspaceRoot, err = nativeRequiredWorkspaceRoot(workspaceRootRaw)
		if err != nil {
			return aghconfig.WriteTarget{}, "", nativeConfigScopeError(id, err)
		}
	}
	target, err := aghconfig.ResolveConfigWriteTarget(n.deps.HomePaths, workspaceRoot, scope)
	if err != nil {
		return aghconfig.WriteTarget{}, "", nativeConfigScopeError(id, err)
	}
	return target, workspaceRoot, nil
}

func nativeConfigPathPolicy(id toolspkg.ToolID, raw string) (aghconfig.PathPolicy, error) {
	path, err := aghconfig.ParseDottedConfigPath(raw)
	if err != nil {
		return aghconfig.PathPolicy{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"config path is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigPathForbidden,
		)
	}
	policy, err := aghconfig.ClassifyToolConfigPath(path)
	if err != nil {
		return aghconfig.PathPolicy{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"config path is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigPathForbidden,
		)
	}
	if policy.Denial == aghconfig.ConfigPathAllowed {
		return policy, nil
	}
	return aghconfig.PathPolicy{}, toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("config path %q is not mutable by tools", strings.TrimSpace(raw)),
		toolspkg.ErrToolDenied,
		nativeConfigDenialReason(policy.Denial),
	)
}

func nativeConfigDenialReason(denial aghconfig.PathDenial) toolspkg.ReasonCode {
	switch denial {
	case aghconfig.ConfigPathSecretForbidden:
		return toolspkg.ReasonConfigSecretPathForbidden
	case aghconfig.ConfigPathTrustForbidden:
		return toolspkg.ReasonConfigTrustRootForbidden
	default:
		return toolspkg.ReasonConfigPathForbidden
	}
}

func nativeConfigValidationError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"config write validation failed",
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonConfigValidationFailed,
	)
}

func nativeConfigScopeError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		"config scope is not allowed",
		fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
		toolspkg.ReasonConfigScopeNotAllowed,
	)
}

func nativeWriteScope(raw string) (aghconfig.WriteScope, error) {
	scope := aghconfig.WriteScope(strings.ToLower(strings.TrimSpace(raw)))
	if scope == "" {
		scope = aghconfig.WriteScopeGlobal
	}
	if err := scope.Validate(); err != nil {
		return "", err
	}
	return scope, nil
}

func nativeOptionalWorkspaceRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return aghconfig.ResolvePath(raw)
}

func nativeRequiredWorkspaceRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("workspace_root is required for workspace scope")
	}
	return aghconfig.ResolvePath(raw)
}

func nativeScopeForWorkspace(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return string(aghconfig.WriteScopeGlobal)
	}
	return string(aghconfig.WriteScopeWorkspace)
}
