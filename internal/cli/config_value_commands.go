package cli

import (
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/spf13/cobra"
)

func newConfigCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   configConfigKey,
		Short: "Inspect and mutate CompozyOS configuration",
	}
	cmd.AddCommand(newConfigShowCommand(deps))
	cmd.AddCommand(newConfigListCommand(deps))
	cmd.AddCommand(newConfigGetCommand(deps))
	cmd.AddCommand(newConfigSetCommand(deps))
	cmd.AddCommand(newConfigUnsetCommand(deps))
	cmd.AddCommand(newConfigPathCommand(deps))
	cmd.AddCommand(newConfigValidateCommand(deps))
	cmd.AddCommand(newConfigCheckCommand(deps))
	cmd.AddCommand(newConfigEditCommand(deps))
	cmd.AddCommand(newConfigReloadCommand(deps))
	cmd.AddCommand(newConfigApplyHistoryCommand(deps))
	return cmd
}

func newConfigUnsetCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw      string
		workspaceRoot string
	)
	cmd := &cobra.Command{
		Use:   "unset <path>",
		Short: "Remove one config value through the validated config writer",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigUnsetCommand(cmd, deps, scopeRaw, workspaceRoot, args[0])
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, configScopeKey, "", "Write scope: user, profile, or workspace (defaults to active owner)")
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace binding (ID, name, or path)")
	return cmd
}

func runConfigUnsetCommand(
	cmd *cobra.Command,
	deps commandDeps,
	scopeRaw string,
	workspaceRoot string,
	rawPath string,
) error {
	if err := requireUnmanagedForMutation(deps, "unset config values"); err != nil {
		return err
	}
	homePaths, target, workspace, err := configWriteTarget(cmd, deps, scopeRaw, workspaceRoot)
	if err != nil {
		return err
	}
	path, _, _, err := configMutationPath(rawPath)
	if err != nil {
		return err
	}
	if err := prepareConfigMutationTarget(target, path); err != nil {
		return err
	}
	if record, err := maybeUnsetWorkspaceSkillSourceViaDaemon(
		cmd, deps, target, workspaceRoot, path,
	); err != nil {
		return err
	} else if record != nil {
		return writeCommandOutput(cmd, configUnsetBundle(*record))
	}
	deleted := false
	if _, err := compozyconfig.EditConfigOverlay(
		homePaths,
		workspace,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			deleted = editor.HasPath(path)
			return editor.Delete(path)
		},
	); err != nil {
		return err
	}
	lifecycle := classifyConfigSetLifecycle(path)
	base := configSetRecordForLocalWrite(path, nil, target, false, lifecycle)
	reloaded, err := maybeReloadConfigAfterLocalWrite(cmd.Context(), deps, target, base)
	if err != nil {
		return err
	}
	if reloaded != nil {
		base = *reloaded
	}
	return writeCommandOutput(cmd, configUnsetBundle(configUnsetRecord{
		Path:             base.Path,
		Scope:            base.Scope,
		Target:           base.Target,
		Deleted:          deleted,
		Lifecycle:        base.Lifecycle,
		ApplyRecordID:    base.ApplyRecordID,
		Applied:          base.Applied,
		ActiveGeneration: base.ActiveGeneration,
		ActiveConfigHash: base.ActiveConfigHash,
		NextAction:       base.NextAction,
		RestartRequired:  base.RestartRequired,
		RestartScope:     base.RestartScope,
	}))
}

func newConfigShowCommand(deps commandDeps) *cobra.Command {
	var workspaceRoot string
	cmd := &cobra.Command{
		Use:   configShowKey,
		Short: "Show the redacted effective config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, resolvedWorkspace, err := loadConfigForDisplay(cmd, deps, workspaceRoot)
			if err != nil {
				return err
			}
			configMap := redactedConfigMap(&cfg)
			entries := flattenConfigEntries(configMap)
			record := configShowRecord{
				Scope:         scopeForWorkspace(resolvedWorkspace),
				WorkspaceRoot: resolvedWorkspace,
				Redacted:      true,
				Config:        configMap,
			}
			return writeCommandOutput(cmd, configShowBundle(record, entries))
		},
	}
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace overlay (ID, name, or path)")
	return cmd
}

func newConfigListCommand(deps commandDeps) *cobra.Command {
	var workspaceRoot string
	cmd := &cobra.Command{
		Use:   configListKey,
		Short: "List redacted effective config values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, resolvedWorkspace, err := loadConfigForDisplay(cmd, deps, workspaceRoot)
			if err != nil {
				return err
			}
			record := configListRecord{
				Scope:         scopeForWorkspace(resolvedWorkspace),
				WorkspaceRoot: resolvedWorkspace,
				Redacted:      true,
				Entries:       flattenConfigEntries(redactedConfigMap(&cfg)),
			}
			return writeCommandOutput(cmd, configListBundle(record))
		},
	}
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace overlay (ID, name, or path)")
	return cmd
}

func newConfigGetCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw      string
		workspaceRoot string
	)
	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Get one redacted effective config value",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfigForDisplayScope(cmd, deps, scopeRaw, workspaceRoot)
			if err != nil {
				return err
			}
			path := strings.TrimSpace(args[0])
			if path == configWindowManagerKey {
				discovery, discoveryErr := windowManagerConfigDiscovery(cfg.WindowManager)
				if discoveryErr != nil {
					return discoveryErr
				}
				return writeCommandOutput(cmd, configValueBundle(configValueRecord{
					Path: path, Value: discovery, Redacted: false,
				}))
			}
			configMap := redactedConfigMap(&cfg)
			for _, entry := range flattenConfigEntries(configMap) {
				if entry.Path == path {
					return writeCommandOutput(cmd, configValueBundle(configValueRecord(entry)))
				}
			}
			if value, found := configValueAtPath(configMap, path); found {
				return writeCommandOutput(cmd, configValueBundle(configValueRecord{
					Path: path, Value: value, Redacted: configValueContainsRedaction(value),
				}))
			}
			return fmt.Errorf("cli: config path %q not found", path)
		},
	}
	cmd.Flags().StringVar(&scopeRaw, configScopeKey, "", "Read scope: user, profile, or workspace (defaults to effective context)")
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace overlay (ID, name, or path)")
	return cmd
}

func newConfigSetCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw      string
		workspaceRoot string
	)
	cmd := &cobra.Command{
		Use:   "set <path> <value>",
		Short: "Set one config value through the validated config writer",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSetCommand(cmd, deps, scopeRaw, workspaceRoot, args)
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, configScopeKey, "", "Write scope: user, profile, or workspace (defaults to active owner)")
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace binding (ID, name, or path)")
	return cmd
}

func runConfigSetCommand(
	cmd *cobra.Command,
	deps commandDeps,
	scopeRaw string,
	workspaceRoot string,
	args []string,
) error {
	if err := requireUnmanagedForMutation(deps, "set config values"); err != nil {
		return err
	}
	homePaths, target, workspace, err := configWriteTarget(cmd, deps, scopeRaw, workspaceRoot)
	if err != nil {
		return err
	}
	path, kind, redacted, err := configMutationPath(args[0])
	if err != nil {
		return err
	}
	if err := prepareConfigMutationTarget(target, path); err != nil {
		return err
	}
	lifecycle := classifyConfigSetLifecycle(path)
	value, err := parseConfigSetValue(kind, args[1])
	if err != nil {
		return err
	}
	if err := validateSkillSourceConfigValue(target.Scope(), path, value); err != nil {
		return err
	}
	if kind == configSetLoopInput {
		return runLoopInputConfigSet(cmd, deps, target, workspaceRoot, path, value)
	}
	liveRecord, err := maybeApplyConfigSetViaDaemon(
		cmd, deps, target, workspaceRoot, path, value, redacted,
	)
	if err != nil {
		return err
	}
	if liveRecord != nil {
		return writeCommandOutput(cmd, configSetBundle(*liveRecord))
	}
	if _, err := compozyconfig.EditConfigOverlay(
		homePaths,
		workspace,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if kind == configSetTable || kind == configSetLoopInput {
				table, ok := value.(map[string]any)
				if ok {
					return editor.SetTable(path, table)
				}
				if kind == configSetTable {
					return fmt.Errorf("cli: config path %q requires an object", strings.Join(path, "."))
				}
			}
			return editor.SetValue(path, value)
		},
	); err != nil {
		return err
	}
	record := configSetRecordForLocalWrite(path, value, target, redacted, lifecycle)
	winner, overridden, err := compozyconfig.WriteOverride(homePaths, workspace, target, path)
	if err != nil {
		return err
	}
	if overridden {
		record.Status = configStatusOverridden
		record.WinningLayer = winner
		record.Applied = false
		record.NextAction = "saved but not applied — " + winner + " wins"
	}
	reloadRecord, err := maybeReloadConfigAfterLocalWrite(cmd.Context(), deps, target, record)
	if err != nil {
		return err
	}
	if reloadRecord != nil {
		record = *reloadRecord
	}
	return writeCommandOutput(cmd, configSetBundle(record))
}

func validateSkillSourceConfigValue(scope compozyconfig.WriteScope, path []string, value any) error {
	if len(path) != 2 || path[0] != configSkillsKey ||
		(path[1] != "sources" && path[1] != "custom_sources") {
		return nil
	}
	values, ok := value.([]string)
	if !ok {
		return fmt.Errorf("cli: config set %q expects a string slice payload, got %T", strings.Join(path, "."), value)
	}
	settings := compozyconfig.SkillsConfig{}
	if path[1] == "sources" {
		settings.Sources = append([]string(nil), values...)
	} else {
		settings.CustomSources = append([]string(nil), values...)
	}
	return settings.ValidateForScope(scope)
}

func runLoopInputConfigSet(
	cmd *cobra.Command,
	deps commandDeps,
	target compozyconfig.WriteTarget,
	workspaceRef string,
	path []string,
	value any,
) error {
	client, err := loopClientFromDeps(deps)
	if err != nil {
		return err
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return err
	}
	scope := contract.LoopInputDefaultsScope(target.Scope())
	response, err := client.PutLoopInputDefault(
		cmd.Context(), resolution.ID, path[2], path[3],
		contract.PutLoopInputDefaultRequest{Scope: scope, Value: value},
	)
	if err != nil {
		return fmt.Errorf("cli: set Loop input default: %w", err)
	}
	lifecycle := classifyConfigSetLifecycle(path)
	record := configSetRecordForLocalWrite(path, response.Value, target, false, lifecycle)
	record.Applied = true
	return writeCommandOutput(cmd, configSetBundle(record))
}

func configSetRecordForLocalWrite(
	path []string,
	value any,
	target compozyconfig.WriteTarget,
	redacted bool,
	lifecycle configMutationLifecycle,
) configSetRecord {
	outputValue := value
	if redacted {
		outputValue = compozyconfig.RedactedValue()
	}
	return configSetRecord{
		Path:            strings.Join(path, "."),
		Value:           outputValue,
		Scope:           string(target.Scope()),
		Target:          target.Path(),
		Redacted:        redacted,
		Lifecycle:       lifecycle.Lifecycle,
		Applied:         lifecycle.Applied,
		NextAction:      lifecycle.NextAction,
		RestartRequired: lifecycle.RestartRequired,
		RestartScope:    lifecycle.RestartScope,
	}
}
