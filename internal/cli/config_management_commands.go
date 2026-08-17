package cli

import (
	"errors"
	"fmt"

	"strings"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/spf13/cobra"
)

func newConfigPathCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw      string
		workspaceRoot string
	)
	cmd := &cobra.Command{
		Use:   configPathKey,
		Short: "Show resolved CompozyOS config paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope, err := parseWriteScope(scopeRaw)
			if err != nil {
				return err
			}
			homeWorkspace := ""
			if scope == compozyconfig.WriteScopeWorkspace || strings.TrimSpace(workspaceRoot) != "" {
				homeWorkspace, err = resolveConfigWorkspaceRoot(cmd, deps, workspaceRoot)
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
			globalMCP, err := compozyconfig.ResolveMCPSidecarWriteTarget(homePaths, "", compozyconfig.WriteScopeGlobal)
			if err != nil {
				return err
			}
			selected, err := compozyconfig.ResolveConfigWriteTarget(homePaths, "", compozyconfig.WriteScopeGlobal)
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
			if scope == compozyconfig.WriteScopeWorkspace || strings.TrimSpace(workspaceRoot) != "" {
				workspace := homeWorkspace
				workspaceConfig, err := compozyconfig.ResolveConfigWriteTarget(
					homePaths,
					workspace,
					compozyconfig.WriteScopeWorkspace,
				)
				if err != nil {
					return err
				}
				workspaceMCP, err := compozyconfig.ResolveMCPSidecarWriteTarget(
					homePaths,
					workspace,
					compozyconfig.WriteScopeWorkspace,
				)
				if err != nil {
					return err
				}
				record.WorkspaceRoot = workspace
				record.WorkspaceConfig = workspaceConfig.Path()
				record.WorkspaceMCPJSON = workspaceMCP.Path()
				if scope == compozyconfig.WriteScopeWorkspace {
					record.SelectedConfigTarget = workspaceConfig.Path()
				}
			}
			return writeCommandOutput(cmd, configPathBundle(record))
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, configScopeKey, string(compozyconfig.WriteScopeGlobal), "Path scope: global or workspace")
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace binding (ID, name, or path)")
	return cmd
}

func newConfigValidateCommand(deps commandDeps) *cobra.Command {
	return newConfigValidateCommandNamed(deps, cliValidateVerb)
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
		Short: "Validate CompozyOS configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, resolved, err := resolveContextualConfigWorkspaceRoot(cmd, deps, workspaceRoot)
			if err != nil {
				return err
			}
			var dotenvReport *compozyconfig.DotEnvRepairReport
			if repairEnv {
				if !resolved {
					workspace, err = resolveConfigWorkspaceRoot(cmd, deps, workspaceRoot)
					if err != nil {
						return err
					}
					resolved = true
				}
				report, err := compozyconfig.RepairDotEnvFile(compozyconfig.WorkspaceDotEnvFile(workspace))
				dotenvReport = &report
				if err != nil {
					return err
				}
			}
			homeWorkspace := workspace
			if !resolved {
				homeWorkspace, err = currentWorkingDirectory(deps)
				if err != nil {
					return err
				}
			}
			homePaths, err := deps.resolveHomeForWorkspace(homeWorkspace)
			if err != nil {
				return err
			}
			loadOptions, err := configDisplayLoadOptions(homePaths, workspace, resolved, deps.getenv)
			if err != nil {
				return err
			}
			if _, err := compozyconfig.LoadForHome(homePaths, loadOptions...); err != nil {
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
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace overlay (ID, name, or path)")
	cmd.Flags().BoolVar(&repairEnv, "repair-env", false, "Repair a structured workspace .env before validating")
	return cmd
}

func configValidationErrors(err error) []configValidationError {
	record := configValidationError{
		Code:    "config.invalid",
		Message: err.Error(),
	}
	if fileErr, ok := errors.AsType[compozyconfig.FileError](err); ok {
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
	if validationErr, ok := errors.AsType[compozyconfig.ValidationError](err); ok {
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
			homePaths, target, workspace, err := configWriteTarget(cmd, deps, scopeRaw, workspaceRoot)
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
			loadOptions, err := configDisplayLoadOptions(
				homePaths,
				workspace,
				workspace != "",
				deps.getenv,
			)
			if err != nil {
				return err
			}
			if _, err := compozyconfig.LoadForHome(homePaths, loadOptions...); err != nil {
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
		StringVar(&scopeRaw, configScopeKey, string(compozyconfig.WriteScopeGlobal), "Edit scope: global or workspace")
	cmd.Flags().
		StringVar(&workspaceRoot, "workspace", "", "Override workspace binding (ID, name, or path)")
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
				Scope:            string(compozyconfig.WriteScopeGlobal),
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
