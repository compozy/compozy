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
	profileResolutionDefault    = configDefaultKey
)

type profileResolution struct {
	Profile       contract.Profile
	Source        string
	Note          string
	WorkspaceID   string
	WorkspaceName string
}

type profileResolutionContextKey struct{}
type historicalProfileReadContextKey struct{}

func resolveCommandProfile(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileResolutionClient,
	workspaces workspaceLookupClient,
) (profileResolution, error) {
	return resolveCommandProfileWithArchived(ctx, cmd, deps, profiles, workspaces, false)
}

func resolveHistoricalCommandProfile(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileResolutionClient,
	workspaces workspaceLookupClient,
) (profileResolution, error) {
	return resolveCommandProfileWithArchived(ctx, cmd, deps, profiles, workspaces, true)
}

func resolveCommandProfileWithArchived(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileResolutionClient,
	workspaces workspaceLookupClient,
	allowArchived bool,
) (profileResolution, error) {
	if allowArchived {
		recordHistoricalProfileRead(cmd)
	}
	if resolution, ok := commandProfileResolution(cmd); ok {
		return resolution, nil
	}
	if _, err := requestedProfileName(cmd); err != nil {
		return profileResolution{}, err
	}
	if cmd != nil && cmd.Flags().Lookup(allWorkspacesFlagName) != nil {
		allWorkspaces, err := cmd.Flags().GetBool(allWorkspacesFlagName)
		if err != nil {
			return profileResolution{}, fmt.Errorf("cli: read all-workspaces flag: %w", err)
		}
		if allWorkspaces {
			return resolveProfileForWorkspaceWithArchived(
				ctx,
				cmd,
				deps,
				profiles,
				workspaceResolution{},
				false,
				allowArchived,
			)
		}
	}
	workspaceRef := ""
	if cmd != nil && cmd.Flags().Lookup(workspaceFlagName) != nil {
		var err error
		workspaceRef, err = commandWorkspaceFlag(cmd)
		if err != nil {
			return profileResolution{}, fmt.Errorf("cli: read profile workspace: %w", err)
		}
	}
	workspace, hasWorkspace, err := resolveContextualCommandWorkspace(ctx, cmd, deps, workspaces, workspaceRef)
	if err != nil {
		return profileResolution{}, err
	}
	return resolveProfileForWorkspaceWithArchived(ctx, cmd, deps, profiles, workspace, hasWorkspace, allowArchived)
}

func resolveProfileForWorkspace(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileResolutionClient,
	workspace workspaceResolution,
	hasWorkspace bool,
) (profileResolution, error) {
	return resolveProfileForWorkspaceWithArchived(ctx, cmd, deps, profiles, workspace, hasWorkspace, false)
}

func resolveProfileForWorkspaceWithArchived(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileResolutionClient,
	workspace workspaceResolution,
	hasWorkspace bool,
	allowArchived bool,
) (profileResolution, error) {
	if resolution, ok := commandProfileResolution(cmd); ok {
		return resolution, nil
	}
	flag, err := requestedProfileName(cmd)
	if err != nil {
		return profileResolution{}, err
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
		return resolveExplicitProfile(cmd, byName, explicit, profileResolutionFlag, base, allowArchived)
	}
	if explicit := strings.TrimSpace(deps.getenv(profileEnvName)); explicit != "" {
		return resolveExplicitProfile(cmd, byName, explicit, profileResolutionEnv, base, allowArchived)
	}
	selections, err := profiles.ListProfileSelections(ctx)
	if err != nil {
		return profileResolution{}, err
	}
	if remembered, found := rememberedProfile(selections, base.WorkspaceID); found {
		selected, exists := byName[remembered]
		if exists && selected.State == authoredContextActiveKey {
			base.Profile, base.Source = selected, profileResolutionRemembered
			recordProfileResolution(cmd, base)
			return base, nil
		}
		base.Note = "archived_remembered_fallback"
	}
	selected, found := byName[configDefaultKey]
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
	profiles, ok := client.(profileResolutionClient)
	if !ok {
		return nil
	}
	_, err := resolveProfileForWorkspaceWithArchived(
		ctx, cmd, deps, profiles, workspace, true, historicalProfileReadAllowed(cmd),
	)
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
	case configDaemonKey, doctorCommandKey, updateUpdateKey:
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
	allowArchived bool,
) (profileResolution, error) {
	selected, found := profiles[name]
	if !found {
		return profileResolution{}, newProfileSelectionError(
			"profile_not_found", fmt.Sprintf("profile %q was not found", name), "run compozy profile list",
		)
	}
	if selected.State == "archived" && !allowArchived {
		return profileResolution{}, newProfileSelectionError(
			"profile_archived", fmt.Sprintf("profile %q is archived", name), "run compozy profile unarchive "+name,
		)
	}
	base.Profile, base.Source = selected, source
	recordProfileResolution(cmd, base)
	return base, nil
}

func rememberedProfile(selections []contract.ProfileSelection, workspaceID string) (string, bool) {
	wantedScope := contract.ProfileSelectionScopeGlobal
	if workspaceID != "" {
		wantedScope = contract.ProfileSelectionScopeWorkspace
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

func requestedProfileName(cmd *cobra.Command) (string, error) {
	flag, err := commandProfileFlag(cmd)
	if err != nil {
		return "", fmt.Errorf("cli: read profile flag: %w", err)
	}
	if commandProfileFlagIsBlank(cmd, flag) {
		return "", newProfileSelectionError(
			"profile_name_invalid", "--profile requires a profile name", "pass --profile <name>",
		)
	}
	return flag, nil
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

func recordHistoricalProfileRead(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	cmd.SetContext(context.WithValue(cmd.Context(), historicalProfileReadContextKey{}, true))
}

func historicalProfileReadAllowed(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Context() == nil {
		return false
	}
	allowed, ok := cmd.Context().Value(historicalProfileReadContextKey{}).(bool)
	return ok && allowed
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
		return contract.ProfileSelection{Scope: layoutProfileScopeGlobal, Profile: name}
	}
	return contract.ProfileSelection{Scope: workspaceSkillSource, WorkspaceID: resolution.ID, Profile: name}
}

func resolveProfileWorkspaceLens(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
) (workspaceResolution, bool, error) {
	return resolveContextualCommandWorkspace(ctx, cmd, deps, client, "")
}
