package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const allProfilesFlagName = "all-profiles"

type profileReadSelection struct {
	AllProfiles bool
	Profile     string
}

type profileReadSelectionContextKey struct{}

func configureProfileReadCommand(cmd *cobra.Command, deps commandDeps) {
	if cmd == nil || cmd.RunE == nil {
		return
	}
	var allProfiles bool
	cmd.Flags().BoolVar(
		&allProfiles,
		allProfilesFlagName,
		false,
		"Read across every profile and include owner labels",
	)
	original := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		profiles := optionalProfileResolutionClient(client)
		if err := prepareProfileReadSelection(cmd, deps, profiles, client, allProfiles); err != nil {
			return err
		}
		return original(cmd, args)
	}
}

// configureProfileMutationCommand resolves the selected profile before a
// mutating request is sent. The transport then stamps the same selection on
// the request query, keeping reads and writes in one profile context.
func configureProfileMutationCommand(cmd *cobra.Command, deps commandDeps) {
	configureSingleProfileCommand(cmd, deps)
}

// configureSingleProfileReadCommand resolves exactly one selected profile for
// reads that do not define an aggregate form.
func configureSingleProfileReadCommand(cmd *cobra.Command, deps commandDeps) {
	configureSingleProfileCommand(cmd, deps)
}

func configureSingleProfileCommand(cmd *cobra.Command, deps commandDeps) {
	if cmd == nil || cmd.RunE == nil {
		return
	}
	original := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		profiles := optionalProfileResolutionClient(client)
		if err := prepareProfileReadSelection(cmd, deps, profiles, client, false); err != nil {
			return err
		}
		return original(cmd, args)
	}
}

func commandProfileSelectionRequested(cmd *cobra.Command, deps commandDeps) (bool, error) {
	flag, err := commandProfileFlag(cmd)
	if err != nil {
		return false, fmt.Errorf("cli: read profile flag: %w", err)
	}
	return strings.TrimSpace(flag) != "" || strings.TrimSpace(deps.getenv(profileEnvName)) != "", nil
}

func prepareProfileReadSelection(
	cmd *cobra.Command,
	deps commandDeps,
	profiles profileResolutionClient,
	workspaces workspaceLookupClient,
	allProfiles bool,
) error {
	if allProfiles {
		if cmd.Flags().Changed(profileFlagName) {
			return newProfileSelectionError(
				"profile_selection_conflict",
				"--profile and --all-profiles cannot be used together",
				"choose one profile or use --all-profiles",
			)
		}
		recordProfileReadSelection(cmd, profileReadSelection{AllProfiles: true})
		return nil
	}
	if handled, err := prepareAgentProfileSelection(cmd, deps); handled || err != nil {
		return err
	}
	if profiles == nil {
		flag, err := commandProfileFlag(cmd)
		if err != nil {
			return fmt.Errorf("cli: read profile flag: %w", err)
		}
		if strings.TrimSpace(flag) != "" || strings.TrimSpace(deps.getenv(profileEnvName)) != "" {
			return newProfileSelectionError(
				"profile_unavailable",
				"profile client is unavailable",
				"update the CompozyOS client and retry",
			)
		}
		recordProfileResolution(cmd, profileResolution{
			Profile: contract.Profile{Name: configDefaultKey}, Source: profileResolutionDefault,
		})
		recordProfileReadSelection(cmd, profileReadSelection{Profile: configDefaultKey})
		return nil
	}
	resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, workspaces)
	if err != nil {
		return fmt.Errorf("cli: resolve profile read scope: %w", err)
	}
	recordProfileReadSelection(cmd, profileReadSelection{Profile: resolution.Profile.Name})
	return nil
}

func prepareAgentProfileSelection(cmd *cobra.Command, deps commandDeps) (bool, error) {
	credentials := agentCredentialsFromEnv(deps)
	if strings.TrimSpace(credentials.SessionID) == "" || strings.TrimSpace(credentials.AgentName) == "" {
		return false, nil
	}
	name, err := requestedProfileName(cmd)
	if err != nil {
		return true, err
	}
	source := profileResolutionFlag
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSpace(deps.getenv(profileEnvName))
		source = profileResolutionEnv
	}
	if strings.TrimSpace(name) == "" {
		return true, nil
	}
	recordProfileResolution(cmd, profileResolution{
		Profile: contract.Profile{Name: name},
		Source:  source,
	})
	recordProfileReadSelection(cmd, profileReadSelection{Profile: name})
	return true, nil
}

func recordProfileReadSelection(cmd *cobra.Command, selection profileReadSelection) {
	if cmd == nil {
		return
	}
	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	cmd.SetContext(context.WithValue(parent, profileReadSelectionContextKey{}, selection))
}

func commandProfileReadSelection(cmd *cobra.Command) (profileReadSelection, bool) {
	if cmd == nil || cmd.Context() == nil {
		return profileReadSelection{}, false
	}
	selection, ok := cmd.Context().Value(profileReadSelectionContextKey{}).(profileReadSelection)
	return selection, ok
}

func profileQueryValues(ctx context.Context, query url.Values) url.Values {
	values := cloneQueryValues(query)
	if ctx == nil || values.Has(profileFlagName) || values.Has("all_profiles") {
		return values
	}
	if selection, ok := ctx.Value(profileReadSelectionContextKey{}).(profileReadSelection); ok {
		if selection.AllProfiles {
			values.Set("all_profiles", "true")
			return values
		}
		if name := strings.TrimSpace(selection.Profile); name != "" {
			values.Set(profileFlagName, name)
		}
		return values
	}
	if resolution, ok := ctx.Value(profileResolutionContextKey{}).(profileResolution); ok {
		if name := strings.TrimSpace(resolution.Profile.Name); name != "" {
			values.Set(profileFlagName, name)
		}
	}
	return values
}

func cloneQueryValues(query url.Values) url.Values {
	values := make(url.Values, len(query)+1)
	for key, entries := range query {
		values[key] = append([]string(nil), entries...)
	}
	return values
}
