package cli

import (
	"errors"
	"fmt"

	"strings"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/spf13/cobra"
)

func newConfigPathCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw      string
		workspaceRoot string
	)
	cmd := &cobra.Command{
		Use:   configPathKey,
		Short: "Show resolved AGH config paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope, err := parseWriteScope(scopeRaw)
			if err != nil {
				return err
			}
			homeWorkspace := ""
			if scope == aghconfig.WriteScopeWorkspace || strings.TrimSpace(workspaceRoot) != "" {
				homeWorkspace, err = resolveConfigWorkspaceRoot(deps, workspaceRoot)
				if err != nil {
					return err
				}
			} else {
				homeWorkspace, err = currentWorkingDirectory(deps)
				if err != nil {
					return err
				}
			}
			homePaths, err := deps.resolveHomeForWorkspace(homeWorkspace)
			if err != nil {
				return err
			}
			globalMCP, err := aghconfig.ResolveMCPSidecarWriteTarget(homePaths, "", aghconfig.WriteScopeGlobal)
			if err != nil {
				return err
			}
			selected, err := aghconfig.ResolveConfigWriteTarget(homePaths, "", aghconfig.WriteScopeGlobal)
			if err != nil {
				return err
			}
			record := configPathRecord{
				HomeDir:              homePaths.HomeDir,
				GlobalConfig:         homePaths.ConfigFile,
				GlobalMCPJSON:        globalMCP.Path(),
				Scope:                string(scope),
				Managed:              detectManagedState(deps).Managed,
				Manager:              detectManagedState(deps).Manager,
				SelectedConfigTarget: selected.Path(),
			}
			if scope == aghconfig.WriteScopeWorkspace || strings.TrimSpace(workspaceRoot) != "" {
				workspace := homeWorkspace
				workspaceConfig, err := aghconfig.ResolveConfigWriteTarget(
					homePaths,
					workspace,
					aghconfig.WriteScopeWorkspace,
				)
				if err != nil {
					return err
				}
				workspaceMCP, err := aghconfig.ResolveMCPSidecarWriteTarget(
					homePaths,
					workspace,
					aghconfig.WriteScopeWorkspace,
				)
				if err != nil {
					return err
				}
				record.WorkspaceRoot = workspace
				record.WorkspaceConfig = workspaceConfig.Path()
				record.WorkspaceMCPJSON = workspaceMCP.Path()
				if scope == aghconfig.WriteScopeWorkspace {
					record.SelectedConfigTarget = workspaceConfig.Path()
				}
			}
			return writeCommandOutput(cmd, configPathBundle(record))
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, configScopeKey, string(aghconfig.WriteScopeGlobal), "Path scope: global or workspace")
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root for workspace-scoped paths")
	return cmd
}

func newConfigValidateCommand(deps commandDeps) *cobra.Command {
	return newConfigValidateCommandNamed(deps, "validate")
}

func newConfigCheckCommand(deps commandDeps) *cobra.Command {
	cmd := newConfigValidateCommandNamed(deps, "check")
	cmd.Short = "Alias for config validate"
	return cmd
}

func newConfigValidateCommandNamed(deps commandDeps, name string) *cobra.Command {
	var workspaceRoot string
	var repairEnv bool
	cmd := &cobra.Command{
		Use:   name,
		Short: "Validate AGH configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := resolveOptionalConfigWorkspaceRoot(workspaceRoot)
			if err != nil {
				return err
			}
			homeWorkspace := workspace
			if homeWorkspace == "" {
				homeWorkspace, err = currentWorkingDirectory(deps)
				if err != nil {
					return err
				}
			}
			homePaths, err := deps.resolveHomeForWorkspace(homeWorkspace)
			if err != nil {
				return err
			}
			var dotenvReport *aghconfig.DotEnvRepairReport
			if repairEnv {
				if workspace == "" {
					workspace, err = currentWorkingDirectory(deps)
					if err != nil {
						return err
					}
				}
				report, err := aghconfig.RepairDotEnvFile(aghconfig.WorkspaceDotEnvFile(workspace))
				dotenvReport = &report
				if err != nil {
					return err
				}
			}
			loadOptions := []aghconfig.LoadOption{}
			if workspace != "" {
				loadOptions = append(loadOptions, aghconfig.WithWorkspaceRoot(workspace))
			}
			if _, err := aghconfig.LoadForHome(homePaths, loadOptions...); err != nil {
				record := configValidateRecord{
					Status:        configInvalidKey,
					Scope:         scopeForWorkspace(workspace),
					WorkspaceRoot: workspace,
					ConfigFile:    homePaths.ConfigFile,
					Redacted:      true,
					Errors:        configValidationErrors(err),
					DotEnv:        dotenvReport,
				}
				if writeErr := writeCommandOutput(cmd, configValidateBundle(record)); writeErr != nil {
					return writeErr
				}
				return configValidationFailedError{err: err}
			}
			return writeCommandOutput(cmd, configValidateBundle(configValidateRecord{
				Status:        configValidKey,
				Scope:         scopeForWorkspace(workspace),
				WorkspaceRoot: workspace,
				ConfigFile:    homePaths.ConfigFile,
				Redacted:      true,
				DotEnv:        dotenvReport,
			}))
		},
	}
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root whose overlay should be validated")
	cmd.Flags().BoolVar(&repairEnv, "repair-env", false, "Repair a structured workspace .env before validating")
	return cmd
}

func configValidationErrors(err error) []configValidationError {
	record := configValidationError{
		Code:    "config.invalid",
		Message: err.Error(),
	}
	if fileErr, ok := errors.AsType[aghconfig.FileError](err); ok {
		record.File = fileErr.Path
		switch fileErr.Op {
		case "decode":
			record.Code = "config.decode"
		case configReadKey:
			record.Code = "config.read"
		default:
			record.Code = "config.file"
		}
	}
	if parseErr, ok := errors.AsType[burnttoml.ParseError](err); ok {
		record.Code = "config.parse"
		record.Line = parseErr.Position.Line
		record.Column = parseErr.Position.Col
		record.Message = parseErr.Message
	}
	if validationErr, ok := errors.AsType[aghconfig.ValidationError](err); ok {
		record.Code = "config.validation"
		record.Path = validationErr.Path
		record.Message = validationErr.Error()
	}
	return []configValidationError{record}
}

func newConfigEditCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw      string
		workspaceRoot string
	)
	cmd := &cobra.Command{
		Use:   configEditKey,
		Short: "Open the selected config overlay in $VISUAL or $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireUnmanagedForMutation(deps, "edit config"); err != nil {
				return err
			}
			homePaths, target, workspace, err := configWriteTarget(deps, scopeRaw, workspaceRoot)
			if err != nil {
				return err
			}
			if err := ensureWriteTargetParent(target); err != nil {
				return err
			}
			if err := ensureEditableConfigFile(target); err != nil {
				return err
			}
			if err := runConfigEditor(cmd, deps, target.Path()); err != nil {
				return err
			}
			loadOptions := []aghconfig.LoadOption{}
			if workspace != "" {
				loadOptions = append(loadOptions, aghconfig.WithWorkspaceRoot(workspace))
			}
			if _, err := aghconfig.LoadForHome(homePaths, loadOptions...); err != nil {
				return fmt.Errorf("cli: edited config failed validation: %w", err)
			}
			return writeCommandOutput(cmd, configSetBundle(configSetRecord{
				Path:            "",
				Value:           "edited",
				Scope:           string(target.Scope()),
				Target:          target.Path(),
				Lifecycle:       string(contract.SettingsApplyLifecycleRestartRequired),
				NextAction:      string(contract.SettingsApplyNextActionRestartDaemon),
				RestartRequired: true,
				RestartScope:    configDaemonKey,
			}))
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, configScopeKey, string(aghconfig.WriteScopeGlobal), "Edit scope: global or workspace")
	cmd.Flags().StringVar(&workspaceRoot, "workspace", "", "Workspace root for workspace-scoped edits")
	return cmd
}

func newConfigReloadCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   configReloadCommandName,
		Short: "Reconcile config.toml with the daemon active generation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return fmt.Errorf("cli: create daemon client for config reload: %w", err)
			}
			result, err := client.ReloadSettings(cmd.Context())
			if err != nil {
				return fmt.Errorf("cli: reload config via daemon settings surface: %w", err)
			}
			return writeCommandOutput(cmd, configSetBundle(configSetRecord{
				Path:             "config.toml",
				Value:            configReloadCommandName,
				Scope:            string(aghconfig.WriteScopeGlobal),
				Target:           configDaemonKey,
				Lifecycle:        string(result.Lifecycle),
				ApplyRecordID:    result.ApplyRecordID,
				Applied:          result.Applied,
				ActiveGeneration: result.ActiveGeneration,
				ActiveConfigHash: result.ActiveConfigHash,
				NextAction:       string(result.NextAction),
				RestartRequired:  result.RestartRequired,
				RestartScope:     result.RestartScope,
			}))
		},
	}
	return cmd
}

func newConfigApplyHistoryCommand(deps commandDeps) *cobra.Command {
	var status string
	var actor string
	var limit int
	cmd := &cobra.Command{
		Use:   "apply-history",
		Short: "List persisted config apply records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return fmt.Errorf("cli: create daemon client for config apply-history: %w", err)
			}
			history, err := client.ListSettingsApplyRecords(cmd.Context(), SettingsApplyHistoryQuery{
				Status: status,
				Actor:  actor,
				Limit:  limit,
			})
			if err != nil {
				return fmt.Errorf("cli: list config apply-history: %w", err)
			}
			return writeCommandOutput(cmd, configApplyHistoryBundle(history))
		},
	}
	cmd.Flags().StringVar(&status, configStatusKey, "", "Filter by apply status")
	cmd.Flags().StringVar(&actor, "actor", "", "Filter by apply actor")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum records to return")
	return cmd
}
