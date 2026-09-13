package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

type extensionCLIScope struct{ scope, workspaceID, profile string }

func resolveExtensionCLIScope(
	cmd *cobra.Command, deps commandDeps, scope, workspaceRef string,
) (extensionCLIScope, error) {
	scope, workspaceRef = strings.TrimSpace(scope), strings.TrimSpace(workspaceRef)
	if scope != "" && scope != "global" && scope != "workspace" {
		return extensionCLIScope{}, errors.New("cli: extension scope must be global or workspace")
	}
	if scope == "global" && workspaceRef != "" {
		return extensionCLIScope{}, errors.New("cli: --scope global cannot select --workspace")
	}
	profile := ""
	if cmd.Flags().Lookup(profileFlagName) != nil {
		selected, err := requestedProfileName(cmd)
		if err != nil {
			return extensionCLIScope{}, err
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
			return extensionCLIScope{}, err
		}
		if !running {
			return extensionCLIScope{}, errors.New("cli: workspace extension operations require a running daemon")
		}
		workspaceID, err = resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
		if err != nil {
			return extensionCLIScope{}, err
		}
		scope = "workspace"
	}
	return extensionCLIScope{scope: scope, workspaceID: workspaceID, profile: profile}, nil
}
