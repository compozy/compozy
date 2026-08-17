package cli

import (
	"errors"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
)

const (
	sessionTypeValue = "Type"
)

const (
	extensionTypeKey = "type"
)

const (
	extensionCapabilitiesValue      = "Capabilities"
	extensionEnabledValue           = "Enabled"
	extensionHealthValue            = "Health"
	extensionCapabilitiesKey        = "capabilities"
	extensionEnabledKey             = "enabled"
	extensionExtensionKey           = "extension"
	extensionHealthKey              = "health"
	extensionMissingEnvKey          = "missing_env"
	extensionListKey                = "list"
	extensionSearchQueryValue       = "search <query>"
	extensionUpdateValue            = "Update"
	extensionUpdateAvailable        = "available"
	extensionRuntimeUnknown         = "unknown"
	extensionDevVerb                = "dev"
	extensionReloadVerb             = "reload"
	extensionConfirmNetworkFlagName = "confirm-network-requirement"
	extensionAgentPluginValue       = "agent plugin"
	extensionMCPKind                = "mcp"
	extensionMCPServerKind          = "mcp_server"
	extensionSkillKind              = "skill"
	cliRemoveNameUse                = "remove <name>"
	cliUseEnableName                = "enable <name>"
	cliUseDisableName               = "disable <name>"
)

type preparedExtensionInstall struct {
	Path     string
	Manifest *extensionpkg.Manifest
	Checksum string
}

type localExtensionRegistry interface {
	Install(manifest *extensionpkg.Manifest, path string, checksum string, opts ...extensionpkg.InstallOption) error
	List() ([]extensionpkg.ExtensionInfo, error)
	Get(name string) (*extensionpkg.ExtensionInfo, error)
	Enable(name string) error
	Disable(name string) error
	Uninstall(name string) error
}

func newExtensionCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   extensionExtensionKey,
		Short: "Manage CompozyOS extensions",
	}

	cmd.AddCommand(newExtensionSearchCommand(deps))
	cmd.AddCommand(newExtensionCommandsCommand(deps))
	cmd.AddCommand(newExtensionExecCommand(deps))
	cmd.AddCommand(newExtensionInitCommand())
	cmd.AddCommand(newExtensionBuildCommand(deps))
	cmd.AddCommand(newExtensionValidateCommand(deps))
	cmd.AddCommand(newExtensionDevCommand(deps))
	cmd.AddCommand(newExtensionReloadCommand(deps))
	cmd.AddCommand(newExtensionLogsCommand(deps))
	cmd.AddCommand(newExtensionListCommand(deps))
	cmd.AddCommand(newExtensionInstallCommand(deps))
	cmd.AddCommand(newExtensionRemoveCommand(deps))
	cmd.AddCommand(newExtensionUpdateCommand(deps))
	cmd.AddCommand(newExtensionEnableCommand(deps))
	cmd.AddCommand(newExtensionDisableCommand(deps))
	cmd.AddCommand(newExtensionStatusCommand(deps))
	cmd.AddCommand(newExtensionInventoryCommand(deps))
	cmd.AddCommand(newExtensionPreviewCommand(deps))
	cmd.AddCommand(newExtensionProvenanceCommand(deps))
	cmd.AddCommand(newExtensionSecretsCommand(deps))
	cmd.AddCommand(newExtensionPublishCommand(deps))
	return cmd
}

func newExtensionSearchCommand(deps commandDeps) *cobra.Command {
	limit := defaultExtensionRegistrySearchLimit
	sources := "curated,github"
	cursor := ""

	cmd := &cobra.Command{
		Use:   extensionSearchQueryValue,
		Short: "Search curated and GitHub extensions",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := searchExtensionsPage(
				cmd.Context(), deps, args[0], strings.Split(sources, ","), limit, cursor,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSearchBundle(result))
		},
	}
	cmd.Flags().
		IntVar(&limit, "limit", defaultExtensionRegistrySearchLimit, "Maximum number of extension registry results to return")
	cmd.Flags().StringVar(&sources, "sources", "curated,github", "Comma-separated discovery sources: curated,github")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue a stable extension search page")
	return cmd
}

func newExtensionListCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	command := &cobra.Command{
		Use:   extensionListKey,
		Short: "List installed extensions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := loadExtensionRecords(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionListBundle(items))
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return command
}

func newExtensionInstallCommand(deps commandDeps) *cobra.Command {
	var version string
	var asset string
	var allowUnverified bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "install <source>",
		Short: "Install an extension from curated, GitHub, git, or a local path",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := parseExtensionInstallPlan(args[0], version, asset, allowUnverified)
			if err != nil {
				return err
			}
			if err := confirmExtensionUnverifiedInstall(cmd, allowUnverified, yes); err != nil {
				return err
			}
			item, err := executeExtensionInstallPlan(cmd.Context(), deps, plan)
			if err != nil {
				return err
			}
			report := extensionInstallValidationReport(deps, plan, item)
			return writeCommandOutput(cmd, extensionInstallSuccessBundle(item, report))
		},
	}
	cmd.Flags().StringVar(&version, versionKey, "", "Install a specific registry version")
	cmd.Flags().StringVar(&asset, "asset", "", "Select a specific registry asset when multiple archives exist")
	cmd.Flags().BoolVar(
		&allowUnverified,
		"allow-unverified",
		false,
		"Allow install when the extension checksum is not registry-verified",
	)
	cmd.Flags().BoolVar(&yes, yesFlagName, false, "Skip confirmation when using --allow-unverified")
	return cmd
}

func newExtensionRemoveCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var global bool
	command := &cobra.Command{
		Use:   cliRemoveNameUse,
		Short: "Remove an installed extension from disk and the registry",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseClient, err := requireExtensionDaemonClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			client, supportsDev := baseClient.(extensionDevClient)
			if global || !supportsDev {
				item, removeErr := baseClient.RemoveExtension(cmd.Context(), args[0])
				if removeErr != nil {
					return removeErr
				}
				return writeCommandOutput(cmd, extensionRemoveBundle(item))
			}
			workspace, err := resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			item, err := client.RemoveDevExtension(cmd.Context(), workspace, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionRemoveBundle(item))
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	command.Flags().BoolVar(&global, "global", false, "Remove the published global installation")
	return command
}

func newExtensionUpdateCommand(deps commandDeps) *cobra.Command {
	var updateAll bool
	var checkOnly bool
	var version string
	var allowUnverified bool
	var yes bool
	var confirmNetworkDigest string

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Check for or install extension updates",
		Args: func(_ *cobra.Command, args []string) error {
			if updateAll && len(args) > 0 {
				return errors.New("cli: update accepts either an extension name or --all, not both")
			}
			if !updateAll && len(args) != 1 {
				return errors.New("cli: update requires an extension name unless --all is set")
			}
			if updateAll && strings.TrimSpace(confirmNetworkDigest) != "" {
				return fmt.Errorf(
					"cli: --%s applies only to a single extension update",
					extensionConfirmNetworkFlagName,
				)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmExtensionUnverifiedInstall(cmd, allowUnverified && !checkOnly, yes); err != nil {
				return err
			}
			items, err := updateMarketplaceExtensions(cmd.Context(), deps, extensionUpdateOptions{
				Names: args, All: updateAll, CheckOnly: checkOnly, Version: version,
				AllowUnverified: allowUnverified, ConfirmNetworkDigest: confirmNetworkDigest,
			})
			if err != nil {
				return err
			}
			if !checkOnly {
				portable := make([]extensionPortableUpdateOutput, 0, len(items))
				for _, item := range items {
					presentation, ok := extensionPortableUpdatePresentation(deps, item)
					if ok {
						portable = append(portable, presentation)
					}
				}
				if len(portable) > 0 {
					return writeCommandOutput(cmd, extensionUpdatesSuccessBundle(items, portable))
				}
			}
			if err := writeCommandOutput(cmd, extensionUpdateBundle(items)); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&updateAll, "all", false, "Update every installed marketplace extension")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates without installing them")
	cmd.Flags().StringVar(&version, versionKey, "", "Update to a specific registry version")
	cmd.Flags().BoolVar(
		&allowUnverified,
		"allow-unverified",
		false,
		"Allow update when the extension checksum is not registry-verified",
	)
	cmd.Flags().BoolVar(&yes, yesFlagName, false, "Skip confirmation when using --allow-unverified")
	cmd.Flags().StringVar(
		&confirmNetworkDigest,
		extensionConfirmNetworkFlagName,
		"",
		"Confirm the candidate extension network requirement digest",
	)
	return cmd
}

func newExtensionEnableCommand(deps commandDeps) *cobra.Command {
	var confirmNetworkDigest string
	command := &cobra.Command{
		Use:   cliUseEnableName,
		Short: "Enable an installed extension",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := enableExtension(cmd.Context(), deps, args[0], EnableExtensionRequest{
				ConfirmNetworkDigest: strings.TrimSpace(confirmNetworkDigest),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionEnableBundle(result))
		},
	}
	command.Flags().StringVar(
		&confirmNetworkDigest,
		extensionConfirmNetworkFlagName,
		"",
		"Confirm the current extension network requirement digest",
	)
	return command
}

func newExtensionDisableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliUseDisableName,
		Short: "Disable an installed extension",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := disableExtension(cmd.Context(), deps, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBundle(item))
		},
	}
}

func newExtensionStatusCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	command := &cobra.Command{
		Use:   "status <name>",
		Short: "Show extension runtime status",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := extensionStatus(cmd, deps, workspaceRef, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBundle(item))
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return command
}

func newExtensionProvenanceCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "provenance <name>",
		Short: "Show extension provenance and trust report",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := extensionProvenance(cmd.Context(), deps, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionProvenanceBundle(item))
		},
	}
}
