package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type bundleActivationFlags struct {
	extensionName             string
	bundleName                string
	profileName               string
	scope                     string
	workspace                 string
	confirmNetworkRequirement bool
}

func addBundleActivationFlags(cmd *cobra.Command, flags *bundleActivationFlags) {
	cmd.Flags().StringVar(&flags.extensionName, bundleExtensionKey, "", "Extension name")
	cmd.Flags().StringVar(&flags.bundleName, bundleBundleKey, "", "Bundle name")
	cmd.Flags().StringVar(&flags.profileName, bundleProfileKey, "", "Bundle profile name")
	cmd.Flags().
		StringVar(&flags.scope, automationScopeKey, bundleGlobalKey, "Activation scope: global or workspace")
	cmd.Flags().
		StringVar(&flags.workspace, workspaceSkillSource, "", "Workspace id, name, or path for workspace scope")
	bindConfirmNetworkRequirementFlag(cmd, &flags.confirmNetworkRequirement)
	mustMarkFlagRequired(cmd, bundleExtensionKey)
	mustMarkFlagRequired(cmd, bundleBundleKey)
	mustMarkFlagRequired(cmd, bundleProfileKey)
}

func bundleActivationRequestFromFlags(flags bundleActivationFlags) ActivateBundleRequest {
	return ActivateBundleRequest{
		ExtensionName:             strings.TrimSpace(flags.extensionName),
		BundleName:                strings.TrimSpace(flags.bundleName),
		ProfileName:               strings.TrimSpace(flags.profileName),
		Scope:                     strings.TrimSpace(flags.scope),
		Workspace:                 strings.TrimSpace(flags.workspace),
		ConfirmNetworkRequirement: flags.confirmNetworkRequirement,
	}
}
