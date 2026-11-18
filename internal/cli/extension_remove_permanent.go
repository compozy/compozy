package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// extensionPermanentQueryKey is the DELETE query flag that selects permanent removal of a built-in.
const extensionPermanentQueryKey = "permanent"

// validateExtensionPermanentRemoveFlags refuses permanent removal with a workspace override. A built-in
// has no workspace scope, so the combination is invalid usage (exit 2).
func validateExtensionPermanentRemoveFlags(permanent bool, workspaceRef string) error {
	if permanent && strings.TrimSpace(workspaceRef) != "" {
		return withCommandExitCode(2, errors.New(
			"cli: --permanent applies to the global built-in; do not combine it with --workspace",
		))
	}
	return nil
}

// removeExtensionPermanently runs permanent removal through the global route. Every daemon refusal
// (not found, not a built-in, conflict, denial) exits 1, as the extension DX contract specifies.
func removeExtensionPermanently(
	ctx context.Context,
	client extensionCommandClient,
	name string,
) (ManagedExtensionRemoveRecord, error) {
	item, err := client.RemovePermanentExtension(ctx, name)
	if err != nil {
		return ManagedExtensionRemoveRecord{}, withCommandExitCode(1, err)
	}
	return item, nil
}

// extensionPermanentRemoveHuman renders the two-line golden-path message from the DX contract.
func extensionPermanentRemoveHuman(item extensionRemoveItem) string {
	name := strings.TrimSpace(item.Name)
	return fmt.Sprintf(
		"Removed %s permanently. It will not return on restart or update.\nRestore it with: compozy extension restore %s",
		name,
		name,
	)
}
