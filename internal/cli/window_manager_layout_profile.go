package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/spf13/cobra"
)

const (
	layoutProfileCommandKey     = "layout-profile"
	layoutProfileScopeFlag      = "scope"
	layoutProfileSpecFile       = "file"
	layoutProfileListKey        = "list"
	layoutProfileIDArg          = "profile-id"
	layoutProfileVersionFlag    = "expected-version"
	layoutProfileWorkspaceIDKey = "workspace_id"
	layoutProfileScopeWorkspace = "workspace"
	layoutProfileScopeGlobal    = "global"
)

// newLayoutProfileCommand exposes the saved-layout resources the web editor
// manages, so an agent can list, read, write, and delete them over the same
// routes and the same scope rules.
func newLayoutProfileCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   layoutProfileCommandKey,
		Short: "List, read, save, and delete saved layouts",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newLayoutProfileListCommand(deps))
	cmd.AddCommand(newLayoutProfileGetCommand(deps))
	cmd.AddCommand(newLayoutProfilePutCommand(deps))
	cmd.AddCommand(newLayoutProfileDeleteCommand(deps))
	return cmd
}

func newLayoutProfileListCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   layoutProfileListKey,
		Short: "List saved layouts visible to a workspace, global scope included",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			records, err := listLayoutProfiles(cmd, deps, workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				windowManagerJSONBundle("window_layout_profiles", "Saved layouts", records),
			)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	return cmd
}

func newLayoutProfileGetCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "get <profile-id>",
		Short: "Get one saved layout",
		Long: "Get one saved layout.\n\n" +
			"There is no per-profile read route, so this filters the visible list; " +
			"the list is complete and unpaginated, so the result is the same record " +
			"the daemon would return.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileID, err := requiredWindowManagerFlag(args[0], layoutProfileIDArg)
			if err != nil {
				return err
			}
			records, err := listLayoutProfiles(cmd, deps, workspace)
			if err != nil {
				return err
			}
			for index := range records.Records {
				if records.Records[index].ID != profileID {
					continue
				}
				return writeCommandOutput(cmd, windowManagerJSONBundle(
					"window_layout_profile",
					"Saved layout",
					contract.ResourceResponse{Record: records.Records[index]},
				))
			}
			return newWindowManagerCLIValidationError(
				windowManagerCLIValidationInvalidValue,
				layoutProfileIDArg,
				fmt.Errorf("cli: no saved layout %q is visible to this workspace", profileID),
			)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	return cmd
}

func newLayoutProfilePutCommand(deps commandDeps) *cobra.Command {
	var workspace, scope, file string
	var expectedVersion int64
	cmd := &cobra.Command{
		Use:   "put <profile-id>",
		Short: "Create or replace one saved layout",
		Long: "Create or replace one saved layout.\n\n" +
			"--expected-version 0 creates; a non-zero value replaces that exact version " +
			"and fails if the record moved underneath you.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileID, err := requiredWindowManagerFlag(args[0], layoutProfileIDArg)
			if err != nil {
				return err
			}
			workspaceID, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			resourceScope, err := layoutProfileScope(scope, workspaceID)
			if err != nil {
				return err
			}
			spec, err := readLayoutProfileSpec(cmd, file)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.PutWindowManagerLayoutProfile(
				cmd.Context(),
				workspaceID,
				profileID,
				contract.PutResourceRequest{
					Scope:           resourceScope,
					ExpectedVersion: expectedVersion,
					Spec:            spec,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				windowManagerJSONBundle("window_layout_profile", "Saved layout", response),
			)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().
		StringVar(&scope, layoutProfileScopeFlag, layoutProfileScopeWorkspace, "Visibility scope: workspace or global")
	cmd.Flags().
		StringVar(&file, layoutProfileSpecFile, "", "Layout profile spec file or - for stdin")
	cmd.Flags().
		Int64Var(&expectedVersion, layoutProfileVersionFlag, 0, "Version this write replaces; 0 creates")
	return cmd
}

func newLayoutProfileDeleteCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var expectedVersion int64
	cmd := &cobra.Command{
		Use:   "delete <profile-id>",
		Short: "Delete one saved layout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileID, err := requiredWindowManagerFlag(args[0], layoutProfileIDArg)
			if err != nil {
				return err
			}
			workspaceID, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed(layoutProfileVersionFlag) {
				return newWindowManagerCLIValidationError(
					windowManagerCLIValidationRequired,
					layoutProfileVersionFlag,
					fmt.Errorf("cli: --%s is required", layoutProfileVersionFlag),
				)
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeleteWindowManagerLayoutProfile(
				cmd.Context(),
				workspaceID,
				profileID,
				contract.DeleteResourceRequest{ExpectedVersion: expectedVersion},
			); err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerJSONBundle(
				"window_layout_profile_deleted",
				"Saved layout deleted",
				map[string]any{"id": profileID, layoutProfileWorkspaceIDKey: workspaceID},
			))
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().Int64Var(&expectedVersion, layoutProfileVersionFlag, 0, "Version this delete removes")
	return cmd
}

func listLayoutProfiles(
	cmd *cobra.Command,
	deps commandDeps,
	workspace string,
) (contract.ResourcesResponse, error) {
	workspaceID, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
	if err != nil {
		return contract.ResourcesResponse{}, err
	}
	client, err := windowManagerClientFromDeps(deps)
	if err != nil {
		return contract.ResourcesResponse{}, err
	}
	return client.ListWindowManagerLayoutProfiles(cmd.Context(), workspaceID)
}

func layoutProfileScope(scope string, workspaceID string) (resources.ResourceScope, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case layoutProfileScopeWorkspace:
		return resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   workspaceID,
		}, nil
	case layoutProfileScopeGlobal:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}, nil
	default:
		return resources.ResourceScope{}, newWindowManagerCLIValidationError(
			windowManagerCLIValidationUnsupported,
			layoutProfileScopeFlag,
			fmt.Errorf(
				"cli: --%s must be %s or %s",
				layoutProfileScopeFlag,
				layoutProfileScopeWorkspace,
				layoutProfileScopeGlobal,
			),
		)
	}
}

// readLayoutProfileSpec keeps the spec opaque on the way through: the daemon's
// resource codec owns what a `window_layout` spec may contain, so re-validating
// its shape here would fork that rule into a second place.
func readLayoutProfileSpec(cmd *cobra.Command, file string) (json.RawMessage, error) {
	raw, err := readWindowManagerJSONFile(cmd, file, "layout profile spec")
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := decodeStrictWindowManagerJSON(raw, &object); err != nil {
		return nil, fmt.Errorf("cli: decode layout profile spec: %w", err)
	}
	if object == nil {
		return nil, newWindowManagerCLIValidationError(
			windowManagerCLIValidationInvalidValue,
			layoutProfileSpecFile,
			fmt.Errorf("cli: --%s must contain a JSON object", layoutProfileSpecFile),
		)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("cli: compact layout profile spec: %w", err)
	}
	return json.RawMessage(compact.Bytes()), nil
}
