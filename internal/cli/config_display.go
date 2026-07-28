package cli

import (
	"fmt"
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
	loadOptions := []compozyconfig.LoadOption{}
	if resolved {
		loadOptions = append(loadOptions, compozyconfig.WithWorkspaceRoot(workspace))
	}
	cfg, err := compozyconfig.LoadForHome(homePaths, loadOptions...)
	if err != nil {
		return compozyconfig.Config{}, "", err
	}
	return cfg, workspace, nil
}

func configWriteTarget(
	cmd *cobra.Command,
	deps commandDeps,
	scopeRaw string,
	workspaceRoot string,
) (compozyconfig.HomePaths, compozyconfig.WriteTarget, string, error) {
	scope, err := parseWriteScope(scopeRaw)
	if err != nil {
		return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
	}
	var workspace string
	if scope == compozyconfig.WriteScopeWorkspace {
		workspace, err = resolveConfigWorkspaceRoot(cmd, deps, workspaceRoot)
		if err != nil {
			return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
		}
	} else {
		var resolved bool
		workspace, resolved, err = resolveContextualConfigWorkspaceRoot(cmd, deps, workspaceRoot)
		if err != nil {
			return compozyconfig.HomePaths{}, compozyconfig.WriteTarget{}, "", err
		}
		if !resolved {
			workspace, err = currentWorkingDirectory(deps)
		}
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
