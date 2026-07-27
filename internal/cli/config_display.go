package cli

import (
	"sort"

	"strings"

	aghconfig "github.com/compozy/compozy/internal/config"
)

func loadConfigForDisplay(deps commandDeps, workspaceRoot string) (aghconfig.Config, string, error) {
	workspace, err := resolveOptionalConfigWorkspaceRoot(workspaceRoot)
	if err != nil {
		return aghconfig.Config{}, "", err
	}
	homeWorkspace := workspace
	if homeWorkspace == "" {
		homeWorkspace, err = currentWorkingDirectory(deps)
		if err != nil {
			return aghconfig.Config{}, "", err
		}
	}
	homePaths, err := deps.resolveHomeForWorkspace(homeWorkspace)
	if err != nil {
		return aghconfig.Config{}, "", err
	}
	loadOptions := []aghconfig.LoadOption{}
	if workspace != "" {
		loadOptions = append(loadOptions, aghconfig.WithWorkspaceRoot(workspace))
	}
	cfg, err := aghconfig.LoadForHome(homePaths, loadOptions...)
	if err != nil {
		return aghconfig.Config{}, "", err
	}
	return cfg, workspace, nil
}

func configWriteTarget(
	deps commandDeps,
	scopeRaw string,
	workspaceRoot string,
) (aghconfig.HomePaths, aghconfig.WriteTarget, string, error) {
	scope, err := parseWriteScope(scopeRaw)
	if err != nil {
		return aghconfig.HomePaths{}, aghconfig.WriteTarget{}, "", err
	}
	workspace := ""
	if scope == aghconfig.WriteScopeWorkspace {
		workspace, err = resolveConfigWorkspaceRoot(deps, workspaceRoot)
		if err != nil {
			return aghconfig.HomePaths{}, aghconfig.WriteTarget{}, "", err
		}
	} else {
		workspace, err = currentWorkingDirectory(deps)
		if err != nil {
			return aghconfig.HomePaths{}, aghconfig.WriteTarget{}, "", err
		}
	}
	homePaths, err := deps.resolveHomeForWorkspace(workspace)
	if err != nil {
		return aghconfig.HomePaths{}, aghconfig.WriteTarget{}, "", err
	}
	writeWorkspace := ""
	if scope == aghconfig.WriteScopeWorkspace {
		writeWorkspace = workspace
	}
	target, err := aghconfig.ResolveConfigWriteTarget(homePaths, writeWorkspace, scope)
	if err != nil {
		return aghconfig.HomePaths{}, aghconfig.WriteTarget{}, "", err
	}
	return homePaths, target, writeWorkspace, nil
}

func parseWriteScope(raw string) (aghconfig.WriteScope, error) {
	scope := aghconfig.WriteScope(strings.ToLower(strings.TrimSpace(raw)))
	if scope == "" {
		scope = aghconfig.WriteScopeGlobal
	}
	if err := scope.Validate(); err != nil {
		return "", err
	}
	return scope, nil
}

func resolveConfigWorkspaceRoot(deps commandDeps, raw string) (string, error) {
	if strings.TrimSpace(raw) != "" {
		return aghconfig.ResolvePath(raw)
	}
	return currentWorkingDirectory(deps)
}

func resolveOptionalConfigWorkspaceRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return aghconfig.ResolvePath(raw)
}

func scopeForWorkspace(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return string(aghconfig.WriteScopeGlobal)
	}
	return string(aghconfig.WriteScopeWorkspace)
}

func redactedConfigMap(cfg *aghconfig.Config) map[string]any {
	return aghconfig.RedactedConfigMap(cfg)
}

func flattenConfigEntries(configMap map[string]any) []configEntry {
	entries := make([]configEntry, 0)
	flattenConfigValue(&entries, "", configMap, false)
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}
