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

// DeleteUserJSON handles user deletion in JSON mode using the unified executor pattern
func DeleteUserJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)

	// In JSON mode, require user ID as argument
	if len(args) == 0 {
		return outputJSONError("user ID required")
	}
	userID := args[0]

	// Parse flags
	force, err := cobraCmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}
	cascade, err := cobraCmd.Flags().GetBool("cascade")
	if err != nil {
		return fmt.Errorf("failed to get cascade flag: %w", err)
	}

	log.Debug("deleting user in JSON mode",
		"user_id", userID,
		"force", force,
		"cascade", cascade)

	// Force flag is required for user deletion in JSON mode
	if !force {
		return outputJSONError("user deletion requires --force flag in JSON mode")
	}

	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}

	// Delete the user
	err = authClient.DeleteUser(ctx, userID)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to delete user: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"message": "User deleted successfully",
		"user_id": userID,
		"deleted": time.Now().Format(time.RFC3339),
		"cascade": cascade,
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}
