package cli

import (
	"fmt"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/spf13/cobra"
)

func newConfigCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   configConfigKey,
		Short: "Inspect and mutate AGH configuration",
	}
	cmd.AddCommand(newConfigShowCommand(deps))
	cmd.AddCommand(newConfigListCommand(deps))
	cmd.AddCommand(newConfigGetCommand(deps))
	cmd.AddCommand(newConfigSetCommand(deps))
	cmd.AddCommand(newConfigPathCommand(deps))
	cmd.AddCommand(newConfigValidateCommand(deps))
	cmd.AddCommand(newConfigCheckCommand(deps))
	cmd.AddCommand(newConfigEditCommand(deps))
	cmd.AddCommand(newConfigReloadCommand(deps))
	cmd.AddCommand(newConfigApplyHistoryCommand(deps))
	return cmd
}

func newConfigShowCommand(deps commandDeps) *cobra.Command {
	var workspaceRoot string
	cmd := &cobra.Command{
		Use:   configShowKey,
		Short: "Show the redacted effective config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, resolvedWorkspace, err := loadConfigForDisplay(deps, workspaceRoot)
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
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root whose overlay should be included")
	return cmd
}

func newConfigListCommand(deps commandDeps) *cobra.Command {
	var workspaceRoot string
	cmd := &cobra.Command{
		Use:   configListKey,
		Short: "List redacted effective config values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, resolvedWorkspace, err := loadConfigForDisplay(deps, workspaceRoot)
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
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root whose overlay should be included")
	return cmd
}

func newConfigGetCommand(deps commandDeps) *cobra.Command {
	var workspaceRoot string
	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Get one redacted effective config value",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfigForDisplay(deps, workspaceRoot)
			if err != nil {
				return err
			}
			path := strings.TrimSpace(args[0])
			for _, entry := range flattenConfigEntries(redactedConfigMap(&cfg)) {
				if entry.Path == path {
					return writeCommandOutput(cmd, configValueBundle(configValueRecord(entry)))
				}
			}
			return fmt.Errorf("cli: config path %q not found", path)
		},
	}
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root whose overlay should be included")
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
		StringVar(&scopeRaw, configScopeKey, string(aghconfig.WriteScopeGlobal), "Write scope: global or workspace")
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root for workspace-scoped writes")
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
	homePaths, target, workspace, err := configWriteTarget(deps, scopeRaw, workspaceRoot)
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
	liveRecord, err := maybeApplyConfigSetViaDaemon(cmd.Context(), deps, target, path, value, redacted)
	if err != nil {
		return err
	}
	if liveRecord != nil {
		return writeCommandOutput(cmd, configSetBundle(*liveRecord))
	}
	if _, err := aghconfig.EditConfigOverlay(homePaths, workspace, target, func(editor *aghconfig.OverlayEditor) error {
		if kind == configSetTable {
			table, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("cli: config path %q requires an object", strings.Join(path, "."))
			}
			return editor.SetTable(path, table)
		}
		return editor.SetValue(path, value)
	}); err != nil {
		return err
	}
	record := configSetRecordForLocalWrite(path, value, target, redacted, lifecycle)
	reloadRecord, err := maybeReloadConfigAfterLocalWrite(cmd.Context(), deps, target, record)
	if err != nil {
		return err
	}
	if reloadRecord != nil {
		record = *reloadRecord
	}
	return writeCommandOutput(cmd, configSetBundle(record))
}

func configSetRecordForLocalWrite(
	path []string,
	value any,
	target aghconfig.WriteTarget,
	redacted bool,
	lifecycle configMutationLifecycle,
) configSetRecord {
	outputValue := value
	if redacted {
		outputValue = aghconfig.RedactedValue()
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
