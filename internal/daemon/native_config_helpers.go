package daemon

import (
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) loadNativeConfig(workspaceRootRaw string) (compozyconfig.Config, string, error) {
	workspaceRoot, err := nativeOptionalWorkspaceRoot(workspaceRootRaw)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	loadOptions := []compozyconfig.LoadOption{}
	if workspaceRoot != "" {
		loadOptions = append(loadOptions, compozyconfig.WithWorkspaceRoot(workspaceRoot))
	}
	cfg, err := compozyconfig.LoadForHome(n.deps.HomePaths, loadOptions...)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	return cfg, workspaceRoot, nil
}

func (n *daemonNativeTools) nativeConfigWriteTarget(
	id toolspkg.ToolID,
	callerScope toolspkg.Scope,
	scopeRaw string,
	workspaceRootRaw string,
) (compozyconfig.WriteTarget, string, error) {
	if strings.TrimSpace(scopeRaw) == "" &&
		!callerScope.Operator &&
		strings.TrimSpace(callerScope.WorkspaceID) != "" {
		scopeRaw = string(compozyconfig.WriteScopeWorkspace)
	}
	scope, err := nativeWriteScope(scopeRaw)
	if err != nil {
		return compozyconfig.WriteTarget{}, "", nativeConfigScopeError(id, err)
	}
	workspaceRoot := ""
	if scope == compozyconfig.WriteScopeWorkspace {
		workspaceRoot, err = nativeRequiredWorkspaceRoot(workspaceRootRaw)
		if err != nil {
			return compozyconfig.WriteTarget{}, "", nativeConfigScopeError(id, err)
		}
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(n.deps.HomePaths, workspaceRoot, scope, "")
	if err != nil {
		return compozyconfig.WriteTarget{}, "", nativeConfigScopeError(id, err)
	}
	return target, workspaceRoot, nil
}

func nativeConfigPathPolicy(id toolspkg.ToolID, raw string) (compozyconfig.PathPolicy, error) {
	path, err := compozyconfig.ParseDottedConfigPath(raw)
	if err != nil {
		return compozyconfig.PathPolicy{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"config path is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigPathForbidden,
		)
	}
	policy, err := compozyconfig.ClassifyToolConfigPath(path)
	if err != nil {
		return compozyconfig.PathPolicy{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"config path is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonConfigPathForbidden,
		)
	}
	if policy.Denial == compozyconfig.ConfigPathAllowed {
		return policy, nil
	}
	return compozyconfig.PathPolicy{}, toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("config path %q is not mutable by tools", strings.TrimSpace(raw)),
		toolspkg.ErrToolDenied,
		nativeConfigDenialReason(policy.Denial),
	)
}

func nativeConfigDenialReason(denial compozyconfig.PathDenial) toolspkg.ReasonCode {
	switch denial {
	case compozyconfig.ConfigPathSecretForbidden:
		return toolspkg.ReasonConfigSecretPathForbidden
	case compozyconfig.ConfigPathTrustForbidden:
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

func nativeWriteScope(raw string) (compozyconfig.WriteScope, error) {
	scope := compozyconfig.WriteScope(strings.ToLower(strings.TrimSpace(raw)))
	if scope == "" {
		scope = compozyconfig.WriteScopeUser
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
	return compozyconfig.ResolvePath(raw)
}

func nativeRequiredWorkspaceRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("workspace is required for workspace scope")
	}
	return compozyconfig.ResolvePath(raw)
}

func nativeScopeForWorkspace(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return string(compozyconfig.WriteScopeUser)
	}
	return string(compozyconfig.WriteScopeWorkspace)
}
