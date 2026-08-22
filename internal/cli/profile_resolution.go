package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	profileResolutionFlag       = "flag"
	profileResolutionEnv        = "env"
	profileResolutionRemembered = "remembered"
	profileResolutionSession    = "session"
	profileResolutionDefault    = "default"
)

type profileResolution struct {
	Profile       contract.Profile
	Source        string
	Note          string
	WorkspaceID   string
	WorkspaceName string
}

type profileResolutionContextKey struct{}

func resolveCommandProfile(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileClientAPI,
	workspaces workspaceLookupClient,
) (profileResolution, error) {
	if resolution, ok := commandProfileResolution(cmd); ok {
		return resolution, nil
	}
	flag, err := commandProfileFlag(cmd)
	if err != nil {
		return profileResolution{}, fmt.Errorf("cli: read profile flag: %w", err)
	}
	if commandProfileFlagIsBlank(cmd, flag) {
		return profileResolution{}, newProfileSelectionError(
			"profile_name_invalid", "--profile requires a profile name", "pass --profile <name>",
		)
	}
	workspace, hasWorkspace, err := resolveContextualCommandWorkspace(ctx, cmd, deps, workspaces, "")
	if err != nil {
		return profileResolution{}, err
	}
	return resolveProfileForWorkspace(ctx, cmd, deps, profiles, workspace, hasWorkspace)
}

func resolveProfileForWorkspace(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileClientAPI,
	workspace workspaceResolution,
	hasWorkspace bool,
) (profileResolution, error) {
	if resolution, ok := commandProfileResolution(cmd); ok {
		return resolution, nil
	}
	flag, err := commandProfileFlag(cmd)
	if err != nil {
		return profileResolution{}, fmt.Errorf("cli: read profile flag: %w", err)
	}
	if commandProfileFlagIsBlank(cmd, flag) {
		return profileResolution{}, newProfileSelectionError(
			"profile_name_invalid", "--profile requires a profile name", "pass --profile <name>",
		)
	}
	items, err := profiles.ListProfiles(ctx)
	if err != nil {
		return profileResolution{}, err
	}
	byName := make(map[string]contract.Profile, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}
	base := profileResolution{}
	if hasWorkspace {
		base.WorkspaceID = workspace.ID
		base.WorkspaceName = workspace.Detail.Workspace.Name
	}
	if explicit := strings.TrimSpace(flag); explicit != "" {
		return resolveExplicitProfile(cmd, byName, explicit, profileResolutionFlag, base)
	}
	if explicit := strings.TrimSpace(deps.getenv(profileEnvName)); explicit != "" {
		return resolveExplicitProfile(cmd, byName, explicit, profileResolutionEnv, base)
	}
	selections, err := profiles.ListProfileSelections(ctx)
	if err != nil {
		return profileResolution{}, err
	}
	if remembered, found := rememberedProfile(selections, base.WorkspaceID); found {
		selected, exists := byName[remembered]
		if exists && selected.State == "active" {
			base.Profile, base.Source = selected, profileResolutionRemembered
			recordProfileResolution(cmd, base)
			return base, nil
		}
		base.Note = "archived_remembered_fallback"
	}
	selected, found := byName["default"]
	if !found {
		return profileResolution{}, newProfileSelectionError(
			"profile_not_found", "default profile was not found", "start the daemon and run compozy profile list",
		)
	}
	base.Profile, base.Source = selected, profileResolutionDefault
	recordProfileResolution(cmd, base)
	return base, nil
}

func resolveProfileAtWorkspaceBoundary(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	workspace workspaceResolution,
) error {
	if selection, ok := commandProfileReadSelection(cmd); ok && selection.AllProfiles {
		return nil
	}
	if profileSelectionExemptCommand(cmd) {
		return nil
	}
	profiles, ok := client.(profileClientAPI)
	if !ok {
		return nil
	}
	_, err := resolveProfileForWorkspace(ctx, cmd, deps, profiles, workspace, true)
	return err
}

func profileSelectionExemptCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "daemon", "doctor", "update":
		return true
	default:
		return false
	}
}

func resolveExplicitProfile(
	cmd *cobra.Command,
	profiles map[string]contract.Profile,
	name, source string,
	base profileResolution,
) (profileResolution, error) {
	selected, found := profiles[name]
	if !found {
		return profileResolution{}, newProfileSelectionError(
			"profile_not_found", fmt.Sprintf("profile %q was not found", name), "run compozy profile list",
		)
	}
	if selected.State == "archived" {
		return profileResolution{}, newProfileSelectionError(
			"profile_archived", fmt.Sprintf("profile %q is archived", name), "run compozy profile unarchive "+name,
		)
	}
	base.Profile, base.Source = selected, source
	recordProfileResolution(cmd, base)
	return base, nil
}

func rememberedProfile(selections []contract.ProfileSelection, workspaceID string) (string, bool) {
	wantedScope := "global"
	if workspaceID != "" {
		wantedScope = "workspace"
	}
	for _, selection := range selections {
		if selection.Scope == wantedScope && selection.WorkspaceID == workspaceID {
			return selection.Profile, true
		}
	}
	return "", false
}

func commandProfileFlagIsBlank(cmd *cobra.Command, value string) bool {
	if cmd == nil || strings.TrimSpace(value) != "" {
		return false
	}
	flag := cmd.Flags().Lookup(profileFlagName)
	return flag != nil && flag.Changed
}

func recordProfileResolution(cmd *cobra.Command, resolution profileResolution) {
	if cmd == nil || strings.TrimSpace(resolution.Source) == "" {
		return
	}
	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	cmd.SetContext(context.WithValue(parent, profileResolutionContextKey{}, resolution))
}

func commandProfileResolution(cmd *cobra.Command) (profileResolution, bool) {
	if cmd == nil || cmd.Context() == nil {
		return profileResolution{}, false
	}
	resolution, ok := cmd.Context().Value(profileResolutionContextKey{}).(profileResolution)
	return resolution, ok && strings.TrimSpace(resolution.Source) != ""
}

func profileSelectionLens(resolution workspaceResolution, hasWorkspace bool, name string) contract.ProfileSelection {
	if !hasWorkspace {
		return contract.ProfileSelection{Scope: "global", Profile: name}
	}
	return contract.ProfileSelection{Scope: "workspace", WorkspaceID: resolution.ID, Profile: name}
}

func resolveProfileWorkspaceLens(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
) (workspaceResolution, bool, error) {
	return resolveContextualCommandWorkspace(ctx, cmd, deps, client, "")
}
