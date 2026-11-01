package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
)

// SupersededBundledDiagnostics includes retained workspace-only and unattached packages absent from global inventory.
func (s *daemonExtensionService) SupersededBundledDiagnostics(ctx context.Context) ([]contract.DiagnosticItem, error) {
	infos, err := s.registry.List()
	if err != nil {
		return nil, err
	}
	profileNames := map[string]string{store.DefaultProfileID: "default"}
	if s.profiles != nil {
		profiles, err := s.profiles.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, profile := range profiles {
			profileNames[profile.ID] = profile.Name
		}
	}
	var items []contract.DiagnosticItem
	for _, info := range infos {
		successor := extensionpkg.SupersededBundledSuccessor(info.Name)
		if successor == "" || info.Source != extensionpkg.SourceBundled {
			continue
		}
		scopes, err := s.registry.Installations(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		if len(scopes) == 0 {
			scopes = []extensionpkg.Installation{{}}
		}
		for _, installation := range scopes {
			command, scope, err := supersededRemovalCommand(info.Name, installation.Scope, profileNames)
			if err != nil {
				return nil, err
			}
			id := "doctor.extension." + info.Name + ".superseded"
			if installation.Scope.ProfileID != "" || installation.Scope.WorkspaceID != "" {
				id += "." + installation.Scope.ProfileID + "." + installation.Scope.WorkspaceID
			}
			items = append(items, diagnostics.NewItem(diagnostics.ItemSpec{
				ID: id, Code: contract.CodeExtensionRuntimeUnavailable, Category: contract.CategoryExtension,
				Title: "Bundled extension is superseded",
				Message: fmt.Sprintf("Extension %q was replaced by %q; retained state needs operator migration. "+
					"After preserving inputs and credentials, use the suggested command to remove only its %s.",
					info.Name, successor, scope),
				Severity: contract.SeverityWarn, DataFreshness: contract.FreshnessLive,
			}, diagnostics.WithSuggestedCommand(command)))
		}
	}
	return items, nil
}

// Commands select one persisted attachment rather than implying global removal covers every workspace.
func supersededRemovalCommand(
	name string,
	scope extensionpkg.InstallationScope,
	profiles map[string]string,
) (string, string, error) {
	command := "compozy"
	profileLabel := ""
	if scope.ProfileID != "" {
		profile, ok := profiles[scope.ProfileID]
		if !ok {
			return "", "", fmt.Errorf("daemon: profile for superseded extension %q is unavailable", name)
		}
		command += " --profile " + profile
		profileLabel = " in profile " + profile
	}
	command += " extension remove " + name
	if scope.WorkspaceID == "" {
		return command + " --global", "global attachment or unattached registration" + profileLabel, nil
	}
	return command + " --workspace " + scope.WorkspaceID, "workspace attachment " + scope.WorkspaceID + profileLabel, nil
}
