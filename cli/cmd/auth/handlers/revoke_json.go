package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// RevokeJSON handles key revocation in JSON mode using the unified executor pattern
func RevokeJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)

	// In JSON mode, require key ID as argument
	if len(args) == 0 {
		return outputJSONError("key ID required in JSON mode")
	}
	keyID := args[0]

	// Get force flag
	force, err := cobraCmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}

	log.Debug("revoking API key in JSON mode",
		"key_id", keyID,
		"force", force)

	// If not forced, we should show a warning (in a real implementation,
	// we'd show affected resources)
	if !force {
		return outputJSONError("revocation requires --force flag in JSON mode")
	}

	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}

	// Revoke the key
	err = authClient.RevokeKey(ctx, keyID)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to revoke API key: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"data": map[string]any{
			"key_id":  keyID,
			"revoked": time.Now().Format(time.RFC3339),
		},
		"message": "Success",
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}
