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
		StringVar(&flags.workspace, workspaceSkillSource, "", "Override workspace binding (ID, name, or path)")
	bindConfirmNetworkRequirementFlag(cmd, &flags.confirmNetworkRequirement)
	mustMarkFlagRequired(cmd, bundleExtensionKey)
	mustMarkFlagRequired(cmd, bundleBundleKey)
	mustMarkFlagRequired(cmd, bundleProfileKey)
}

func bundleActivationRequestFromFlags(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	flags bundleActivationFlags,
) (ActivateBundleRequest, error) {
	request := ActivateBundleRequest{
		ExtensionName:             strings.TrimSpace(flags.extensionName),
		BundleName:                strings.TrimSpace(flags.bundleName),
		ProfileName:               strings.TrimSpace(flags.profileName),
		Scope:                     strings.ToLower(strings.TrimSpace(flags.scope)),
		Workspace:                 strings.TrimSpace(flags.workspace),
		ConfirmNetworkRequirement: flags.confirmNetworkRequirement,
	}
	if request.Scope == bundleGlobalKey {
		return request, nil
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: request.Workspace},
	)
	if err != nil {
		return ActivateBundleRequest{}, err
	}
	request.Workspace = resolution.ID
	return request, nil
}
