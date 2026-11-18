package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	extensionRestoreUse    = "restore <name>"
	extensionRemovedUse    = "removed"
	extensionRestoreKey    = "extension_restore"
	extensionRemovedKey    = "removed"
	extensionRemovedName   = "NAME"
	extensionRemovedAtName = "REMOVED_AT"
	extensionRemovedByName = "REMOVED_BY"
	extensionRestoredLabel = "Restored %s %s (%s, %s)."
)

// newExtensionRestoreCommand restores a permanently removed built-in. Built-ins have no workspace
// scope, so restore is always global; --global is accepted to mirror `remove --permanent --global`.
func newExtensionRestoreCommand(deps commandDeps) *cobra.Command {
	var global bool
	command := &cobra.Command{
		Use:   extensionRestoreUse,
		Short: "Restore a permanently removed built-in extension at its bundled version",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := exactOneNonBlankArg()(cmd, args); err != nil {
				return withCommandExitCode(2, err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			baseClient, err := requireExtensionDaemonClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			if err := prepareExtensionProfileRead(cmd, deps, baseClient); err != nil {
				return err
			}
			item, restoreErr := restoreExtensionBuiltin(cmd.Context(), baseClient, args[0])
			if restoreErr != nil {
				return restoreErr
			}
			return writeCommandOutput(cmd, extensionRestoreBundle(item))
		},
	}
	command.Flags().BoolVar(
		&global,
		"global",
		false,
		"Accepted for symmetry with remove --permanent --global; built-ins are always global",
	)
	return command
}

// newExtensionRemovedCommand lists the built-ins that are permanently removed and restorable.
func newExtensionRemovedCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   extensionRemovedUse,
		Short: "List built-in extensions that were removed permanently",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			baseClient, err := requireExtensionDaemonClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			if err := prepareExtensionProfileRead(cmd, deps, baseClient); err != nil {
				return err
			}
			items, listErr := baseClient.ListRemovedExtensionBuiltins(cmd.Context())
			if listErr != nil {
				return withCommandExitCode(1, listErr)
			}
			return writeCommandOutput(cmd, extensionRemovedBundle(items))
		},
	}
}

// restoreExtensionBuiltin forwards restore to the daemon. Every daemon refusal exits 1, as the
// extension DX contract specifies for not-removed, existing-copy, and failed-restore conflicts.
func restoreExtensionBuiltin(
	ctx context.Context,
	client extensionCommandClient,
	name string,
) (ExtensionRestoreRecord, error) {
	item, err := client.RestoreExtensionBuiltin(ctx, name)
	if err != nil {
		return ExtensionRestoreRecord{}, withCommandExitCode(1, err)
	}
	return item, nil
}

// extensionRestoreBundle renders the restore result. JSON is the payload object itself.
func extensionRestoreBundle(item ExtensionRestoreRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, item)
		},
		human: func() (string, error) {
			enabled := daemonDisabledKey
			if item.Enabled {
				enabled = automationEnabledKey
			}
			return fmt.Sprintf(extensionRestoredLabel, item.Name, item.Version, item.Source, enabled), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				extensionRestoreKey,
				[]string{automationNameKey, automationStatusKey, automationSourceKey, versionKey, "enabled",
					extensionMarketplacePathKey},
				[]string{
					item.Name,
					item.Status,
					item.Source,
					item.Version,
					fmt.Sprintf("%t", item.Enabled),
					item.Path,
				},
			), nil
		},
	}
}

// extensionRemovedBundle renders the removed-built-in listing. An empty listing is `{"removed":[]}`.
func extensionRemovedBundle(items []ExtensionRemovedBuiltinRecord) outputBundle {
	if items == nil {
		items = []contract.ExtensionRemovedBuiltinPayload{}
	}
	return listBundle(
		contract.ExtensionRemovedBuiltinsResponse{Removed: items},
		items,
		"Removed built-ins",
		[]string{extensionRemovedName, extensionRemovedAtName, extensionRemovedByName},
		extensionRemovedKey,
		[]string{automationNameKey, "removed_at", "removed_by"},
		func(item ExtensionRemovedBuiltinRecord) []string {
			return []string{stringOrDash(item.Name), stringOrDash(item.RemovedAt), stringOrDash(item.RemovedBy)}
		},
		func(item ExtensionRemovedBuiltinRecord) []string {
			return []string{strings.TrimSpace(item.Name), item.RemovedAt, item.RemovedBy}
		},
	)
}
