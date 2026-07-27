package cli

import (
	"sort"

	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func loadConfigForDisplay(deps commandDeps, workspaceRoot string) (compozyconfig.Config, string, error) {
	workspace, err := resolveOptionalConfigWorkspaceRoot(workspaceRoot)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	homeWorkspace := workspace
	if homeWorkspace == "" {
		homeWorkspace, err = currentWorkingDirectory(deps)
		if err != nil {
			return compozyconfig.Config{}, "", err
		}
	}
	homePaths, err := deps.resolveHomeForWorkspace(homeWorkspace)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	loadOptions := []compozyconfig.LoadOption{}
	if workspace != "" {
		loadOptions = append(loadOptions, compozyconfig.WithWorkspaceRoot(workspace))
	}
	cfg, err := compozyconfig.LoadForHome(homePaths, loadOptions...)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	return cfg, workspace, nil
}

func configWriteTarget(
	deps commandDeps,
	scopeRaw string,
	workspaceRoot string,
) (compozyconfig.HomePaths, compozyconfig.WriteTarget, string, error) {
	scope, err := parseWriteScope(scopeRaw)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	workspace := ""
	if scope == compozyconfig.WriteScopeWorkspace {
		workspace, err = resolveConfigWorkspaceRoot(deps, workspaceRoot)
		if err != nil {
			return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
		}
	} else {
		workspace, err = currentWorkingDirectory(deps)
		if err != nil {
			return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
		}
	}
	homePaths, err := deps.resolveHomeForWorkspace(workspace)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	writeWorkspace := ""
	if scope == compozyconfig.WriteScopeWorkspace {
		writeWorkspace = workspace
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, writeWorkspace, scope)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	return homePaths, target, writeWorkspace, nil
}

func parseWriteScope(raw string) (compozyconfig.WriteScope, error) {
	scope := compozyconfig.WriteScope(strings.ToLower(strings.TrimSpace(raw)))
	if scope == "" {
		scope = compozyconfig.WriteScopeGlobal
	}
	if err := scope.Validate(); err != nil {
		return "", err
	}
	return scope, nil
}

func resolveConfigWorkspaceRoot(deps commandDeps, raw string) (string, error) {
	if strings.TrimSpace(raw) != "" {
		return compozyconfig.ResolvePath(raw)
	}
	return currentWorkingDirectory(deps)
}

func resolveOptionalConfigWorkspaceRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return compozyconfig.ResolvePath(raw)
}

func scopeForWorkspace(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return string(compozyconfig.WriteScopeGlobal)
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
