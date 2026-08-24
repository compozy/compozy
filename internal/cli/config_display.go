package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"

	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

func loadConfigForDisplay(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRoot string,
) (compozyconfig.Config, string, error) {
	workspace, resolved, err := resolveContextualConfigWorkspaceRoot(cmd, deps, workspaceRoot)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	homeWorkspace := workspace
	if !resolved {
		homeWorkspace, err = currentWorkingDirectory(deps)
		if err != nil {
			return compozyconfig.Config{}, "", err
		}
	}
	homePaths, err := deps.resolveHomeForWorkspace(homeWorkspace)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	loadOptions, err := configDisplayCommandLoadOptions(cmd, deps, homePaths, workspace, resolved)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	cfg, err := compozyconfig.LoadForHome(homePaths, loadOptions...)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	return cfg, workspace, nil
}

func configDisplayCommandLoadOptions(
	cmd *cobra.Command,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	workspace string,
	resolved bool,
) ([]compozyconfig.LoadOption, error) {
	loadOptions, err := configDisplayLoadOptions(homePaths, workspace, resolved, deps.getenv)
	if err != nil {
		return nil, err
	}
	profileName, err := resolveConfigDisplayProfile(cmd, deps)
	if err != nil {
		return nil, err
	}
	if profileName != "" {
		loadOptions = append(loadOptions, compozyconfig.WithProfile(profileName))
	}
	return loadOptions, nil
}

func resolveConfigDisplayProfile(cmd *cobra.Command, deps commandDeps) (string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return configDefaultKey, nil
	}
	profiles, ok := client.(profileResolutionClient)
	if !ok {
		return configDefaultKey, nil
	}
	resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resolution.Profile.Name), nil
}

func configDisplayLoadOptions(
	homePaths compozyconfig.HomePaths,
	workspace string,
	resolved bool,
	getenv func(string) string,
) ([]compozyconfig.LoadOption, error) {
	if !resolved {
		return nil, nil
	}
	options := []compozyconfig.LoadOption{compozyconfig.WithWorkspaceRoot(workspace)}
	operatorHome, err := compozyconfig.ResolveOperatorHomeDirWithLookup(
		homePaths,
		func(key string) (string, bool) {
			if getenv == nil {
				return "", false
			}
			value := getenv(key)
			return value, strings.TrimSpace(value) != ""
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cli: resolve operator home for config display: %w", err)
	}
	workspacePath, err := comparableConfigDisplayPath(workspace)
	if err != nil {
		return nil, err
	}
	operatorPath, err := comparableConfigDisplayPath(operatorHome)
	if err != nil {
		return nil, err
	}
	if workspacePath == operatorPath {
		options = append(options, compozyconfig.WithoutWorkspaceOverlayFiles())
	}
	return options, nil
}

func comparableConfigDisplayPath(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("cli: resolve config display path %q: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func configWriteTarget(
	cmd *cobra.Command,
	deps commandDeps,
	scopeRaw string,
	workspaceRoot string,
) (compozyconfig.HomePaths, compozyconfig.WriteTarget, string, error) {
	scope, profileName, err := resolveConfigWriteScopeAndProfile(cmd, deps, scopeRaw)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	workspace, err := resolveConfigWriteWorkspace(cmd, deps, scope, scopeRaw, workspaceRoot)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	homePaths, err := deps.resolveHomeForWorkspace(workspace)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	writeWorkspace := ""
	if scope == compozyconfig.WriteScopeWorkspace {
		writeWorkspace = workspace
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, writeWorkspace, scope, profileName)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	return homePaths, target, workspace, nil
}

func resolveConfigWriteScopeAndProfile(
	cmd *cobra.Command,
	deps commandDeps,
	scopeRaw string,
) (compozyconfig.WriteScope, string, error) {
	requestedScope := strings.TrimSpace(scopeRaw)
	profileName := configDefaultKey
	var err error
	if requestedScope == "" {
		profileName, err = resolveConfigWriteProfile(cmd, deps)
		if err != nil {
			return "", "", err
		}
		requestedScope = string(compozyconfig.WriteScopeProfile)
		if profileName == "" || profileName == configDefaultKey {
			requestedScope = string(compozyconfig.WriteScopeUser)
		}
	}
	scope, err := parseWriteScope(requestedScope)
	if err != nil {
		return "", "", err
	}
	if scope != compozyconfig.WriteScopeProfile {
		return scope, profileName, nil
	}
	profileName, err = resolveConfigWriteProfile(cmd, deps)
	if err != nil {
		return "", "", err
	}
	if profileName == "" || profileName == configDefaultKey {
		return "", "", errors.New(
			"cli: the default profile writes user config; select a non-default profile or use --scope user",
		)
	}
	return scope, profileName, nil
}

func resolveConfigWriteWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	scope compozyconfig.WriteScope,
	scopeRaw string,
	workspaceRoot string,
) (string, error) {
	if scope == compozyconfig.WriteScopeWorkspace {
		return resolveConfigWorkspaceRoot(cmd, deps, workspaceRoot)
	}
	if scope == compozyconfig.WriteScopeUser && strings.TrimSpace(scopeRaw) != "" {
		if strings.TrimSpace(workspaceRoot) != "" {
			return resolveConfigWorkspaceRoot(cmd, deps, workspaceRoot)
		}
		return currentWorkingDirectory(deps)
	}
	workspace, resolved, err := resolveContextualConfigWorkspaceRoot(cmd, deps, workspaceRoot)
	if err != nil || resolved {
		return workspace, err
	}
	return currentWorkingDirectory(deps)
}

func resolveConfigWriteProfile(cmd *cobra.Command, deps commandDeps) (string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return "", err
	}
	profiles, ok := client.(profileResolutionClient)
	if !ok {
		flag, flagErr := commandProfileFlag(cmd)
		if flagErr != nil {
			return "", flagErr
		}
		if strings.TrimSpace(flag) != "" || strings.TrimSpace(deps.getenv(profileEnvName)) != "" {
			return "", newProfileSelectionError(
				"profile_unavailable", "profile client is unavailable", "start the daemon and retry",
			)
		}
		return configDefaultKey, nil
	}
	resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
	if err != nil {
		return "", err
	}
	return resolution.Profile.Name, nil
}

func parseWriteScope(raw string) (compozyconfig.WriteScope, error) {
	scope := compozyconfig.WriteScope(strings.ToLower(strings.TrimSpace(raw)))
	if scope == "" {
		scope = compozyconfig.WriteScopeUser
	}
	if err := scope.Validate(); err != nil {
		return "", err
	}
	return scope, nil
}

func resolveConfigWorkspaceRoot(cmd *cobra.Command, deps commandDeps, raw string) (string, error) {
	return resolveCommandWorkspaceRoot(cmd, deps, raw)
}

func resolveContextualConfigWorkspaceRoot(
	cmd *cobra.Command,
	deps commandDeps,
	raw string,
) (string, bool, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return "", false, err
	}
	resolution, ok, err := resolveContextualCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		raw,
	)
	if err != nil || !ok {
		return "", ok, err
	}
	if resolution.Root == "" {
		return "", false, fmt.Errorf("cli: resolved workspace %q has no root directory", resolution.ID)
	}
	return resolution.Root, true, nil
}

func scopeForWorkspace(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return string(compozyconfig.WriteScopeUser)
	}
	return string(compozyconfig.WriteScopeWorkspace)
}

func redactedConfigMap(cfg *compozyconfig.Config) map[string]any {
	return compozyconfig.RedactedConfigMap(cfg)
}

func flattenConfigEntries(configMap map[string]any) []configEntry {
	entries := make([]configEntry, 0)
	flattenConfigValue(&entries, "", configMap, false)
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func configValueAtPath(configMap map[string]any, path string) (any, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}
	var current any = configMap
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func configValueContainsRedaction(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == compozyconfig.RedactedValue()
	case []any:
		return slices.ContainsFunc(typed, configValueContainsRedaction)
	case map[string]any:
		for _, item := range typed {
			if configValueContainsRedaction(item) {
				return true
			}
		}
	}
	return false
}
