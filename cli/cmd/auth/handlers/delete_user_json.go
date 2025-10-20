package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// DeleteUserJSON handles user deletion in JSON mode using the unified executor pattern

func DeleteUserJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)
	userID, err := resolveUserID(args)
	if err != nil {
		return outputJSONError(err.Error())
	}
	options, err := parseDeleteUserFlags(cobraCmd)
	if err != nil {
		return err
	}
	options.userID = userID
	log.Debug("deleting user in JSON mode",
		"user_id", options.userID,
		"force", options.force,
		"cascade", options.cascade)
	if err := ensureForceDeletion(options.force); err != nil {
		return outputJSONError(err.Error())
	}
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	if err := authClient.DeleteUser(ctx, options.userID); err != nil {
		return outputJSONError(fmt.Sprintf("failed to delete user: %v", err))
	}
	return writeDeleteResponse(options.userID, options.cascade)
}

type deleteUserOptions struct {
	userID  string
	force   bool
	cascade bool
}

func resolveUserID(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("user ID required")
	}
	id := strings.TrimSpace(args[0])
	if id == "" {
		return "", fmt.Errorf("user ID cannot be empty or whitespace")
	}
	return id, nil
}

func parseDeleteUserFlags(cobraCmd *cobra.Command) (*deleteUserOptions, error) {
	force, err := cobraCmd.Flags().GetBool("force")
	if err != nil {
		return nil, fmt.Errorf("failed to get force flag: %w", err)
	}
	cascade, err := cobraCmd.Flags().GetBool("cascade")
	if err != nil {
		return nil, fmt.Errorf("failed to get cascade flag: %w", err)
	}
	return &deleteUserOptions{
		force:   force,
		cascade: cascade,
	}, nil
}

func ensureForceDeletion(force bool) error {
	if force {
		return nil
	}
	return fmt.Errorf("user deletion requires --force flag in JSON mode")
}

func writeDeleteResponse(userID string, cascade bool) error {
	response := map[string]any{
		"data": map[string]any{
			"user_id": userID,
			"deleted": time.Now().Format(time.RFC3339),
			"cascade": cascade,
		},
		"message": "Success",
	}
	return writeJSONResponse(response)
}
