package cli

import (
	"errors"

	"strings"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/spf13/cobra"
)

const (
	sessionTypeValue = "Type"
)

const (
	extensionTypeKey = "type"
)

const (
	extensionCapabilitiesValue = "Capabilities"
	extensionEnabledValue      = "Enabled"
	extensionHealthValue       = "Health"
	extensionCapabilitiesKey   = "capabilities"
	extensionEnabledKey        = "enabled"
	extensionExtensionKey      = "extension"
	extensionHealthKey         = "health"
	extensionListKey           = "list"
	extensionSearchQueryValue  = "search <query>"
	cliUseEnableName           = "enable <name>"
	cliUseDisableName          = "disable <name>"
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
		Short: "Manage AGH extensions",
	}

	cmd.AddCommand(newExtensionSearchCommand(deps))
	cmd.AddCommand(newExtensionListCommand(deps))
	cmd.AddCommand(newExtensionInstallCommand(deps))
	cmd.AddCommand(newExtensionRemoveCommand(deps))
	cmd.AddCommand(newExtensionUpdateCommand(deps))
	cmd.AddCommand(newExtensionEnableCommand(deps))
	cmd.AddCommand(newExtensionDisableCommand(deps))
	cmd.AddCommand(newExtensionStatusCommand(deps))
	cmd.AddCommand(newExtensionProvenanceCommand(deps))
	return cmd
}

func newExtensionSearchCommand(deps commandDeps) *cobra.Command {
	limit := defaultExtensionRegistrySearchLimit

	cmd := &cobra.Command{
		Use:   extensionSearchQueryValue,
		Short: "Search marketplace extensions",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := searchExtensions(cmd.Context(), deps, args[0], limit)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSearchBundle(results))
		},
	}
	cmd.Flags().
		IntVar(&limit, "limit", defaultExtensionRegistrySearchLimit, "Maximum number of extension registry results to return")
	return cmd
}

func newExtensionListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   extensionListKey,
		Short: "List installed extensions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := loadExtensionRecords(cmd.Context(), deps)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionListBundle(items))
		},
	}
}

func newExtensionInstallCommand(deps commandDeps) *cobra.Command {
	var sourceFilter string
	var version string
	var asset string
	var allowUnverified bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "install <path-or-slug>",
		Short: "Install a local extension or download one from a registry",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			prepared, isLocalPath, err := prepareLocalExtensionInstallIfPresent(args[0])
			if err != nil {
				return err
			}
			if err := confirmExtensionUnverifiedInstall(cmd, allowUnverified, yes); err != nil {
				return err
			}
			if isLocalPath {
				if strings.TrimSpace(sourceFilter) != "" || strings.TrimSpace(version) != "" ||
					strings.TrimSpace(asset) != "" {
					return errors.New("cli: --from, --version, and --asset are only supported for registry installs")
				}

				item, err := installExtension(cmd.Context(), deps, prepared, allowUnverified)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, extensionBundle(item))
			}

			item, err := installMarketplaceExtension(
				cmd.Context(),
				deps,
				args[0],
				sourceFilter,
				version,
				asset,
				allowUnverified,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBundle(item))
		},
	}
	cmd.Flags().StringVar(&sourceFilter, "from", "", "Only use one configured extension registry source")
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
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed extension from disk and the registry",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := removeInstalledExtension(cmd.Context(), deps, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionRemoveBundle(item))
		},
	}
}

func newExtensionUpdateCommand(deps commandDeps) *cobra.Command {
	var updateAll bool
	var checkOnly bool
	var version string
	var allowUnverified bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Check for or install updates for marketplace extensions",
		Args: func(_ *cobra.Command, args []string) error {
			if updateAll && len(args) > 0 {
				return errors.New("cli: update accepts either an extension name or --all, not both")
			}
			if !updateAll && len(args) != 1 {
				return errors.New("cli: update requires an extension name unless --all is set")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmExtensionUnverifiedInstall(cmd, allowUnverified && !checkOnly, yes); err != nil {
				return err
			}
			items, err := updateMarketplaceExtensions(
				cmd.Context(),
				deps,
				args,
				updateAll,
				checkOnly,
				version,
				allowUnverified,
			)
			if err != nil {
				return err
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
	return cmd
}

func newExtensionEnableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliUseEnableName,
		Short: "Enable an installed extension",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := mutateExtensionEnabled(cmd.Context(), deps, args[0], true)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBundle(item))
		},
	}
}

func newExtensionDisableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliUseDisableName,
		Short: "Disable an installed extension",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := mutateExtensionEnabled(cmd.Context(), deps, args[0], false)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBundle(item))
		},
	}
}

func newExtensionStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show extension runtime status",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := extensionStatus(cmd.Context(), deps, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionBundle(item))
		},
	}
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
