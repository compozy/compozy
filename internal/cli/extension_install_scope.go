package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

func selectExtensionInstallScope(
	cmd *cobra.Command, deps commandDeps, plan extensionInstallPlan, scope, workspaceRef string,
) (extensionInstallPlan, error) {
	scope, workspaceRef = strings.TrimSpace(scope), strings.TrimSpace(workspaceRef)
	if scope != "" && scope != "global" && scope != "workspace" {
		return extensionInstallPlan{}, errors.New("cli: extension scope must be global or workspace")
	}
	if scope == "global" && workspaceRef != "" {
		return extensionInstallPlan{}, errors.New("cli: --scope global cannot select --workspace")
	}
	profile := ""
	if cmd.Flags().Lookup(profileFlagName) != nil {
		selected, err := requestedProfileName(cmd)
		if err != nil {
			return extensionInstallPlan{}, err
		}
		profile = strings.TrimSpace(selected)
	}
	if profile == "" {
		profile = strings.TrimSpace(deps.getenv(profileEnvName))
	}
	workspaceID := ""
	if scope == "workspace" || workspaceRef != "" {
		client, running, err := daemonClientIfRunning(cmd.Context(), deps)
		if err != nil {
			return extensionInstallPlan{}, err
		}
		if !running {
			return extensionInstallPlan{}, errors.New("cli: scoped extension installation requires a running daemon")
		}
		workspaceID, err = resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
		if err != nil {
			return extensionInstallPlan{}, err
		}
		scope = "workspace"
	}
	for index := range plan.Attempts {
		plan.Attempts[index].Scope = scope
		plan.Attempts[index].WorkspaceID = workspaceID
		plan.Attempts[index].Profile = profile
	}
	return plan, nil
}
