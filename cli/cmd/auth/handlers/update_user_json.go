package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// UpdateUserJSON handles user update in JSON mode using the unified executor pattern
func UpdateUserJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)
	if len(args) == 0 {
		return outputJSONError("user ID is required")
	}
	userID := args[0]
	flags, err := parseUpdateUserFlags(cobraCmd)
	if err != nil {
		return err
	}
	if err := validateUpdateUserInput(flags); err != nil {
		return err
	}
	log.Debug("updating user in JSON mode",
		"user_id", userID,
		"email", flags.email,
		"name", flags.name,
		"role", flags.role)
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	req := buildUpdateUserRequest(flags)
	user, err := authClient.UpdateUser(ctx, userID, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to update user: %v", err))
	}
	return outputUpdateUserSuccess(user)
}

type updateUserFlags struct {
	email string
	name  string
	role  string
}

// parseUpdateUserFlags extracts and validates flags
func parseUpdateUserFlags(cobraCmd *cobra.Command) (*updateUserFlags, error) {
	email, err := cobraCmd.Flags().GetString("email")
	if err != nil {
		return nil, fmt.Errorf("failed to get email flag: %w", err)
	}
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return nil, fmt.Errorf("failed to get name flag: %w", err)
	}
	role, err := cobraCmd.Flags().GetString("role")
	if err != nil {
		return nil, fmt.Errorf("failed to get role flag: %w", err)
	}
	return &updateUserFlags{
		email: email,
		name:  name,
		role:  role,
	}, nil
}

// validateUpdateUserInput validates the input for user update
func validateUpdateUserInput(flags *updateUserFlags) error {
	if flags.role != "" && flags.role != roleAdmin && flags.role != roleUser {
		return outputJSONError(fmt.Sprintf("invalid role: must be '%s' or '%s'", roleAdmin, roleUser))
	}
	if flags.email == "" && flags.name == "" && flags.role == "" {
		return outputJSONError("at least one field (email, name, or role) must be specified for update")
	}
	return nil
}

// buildUpdateUserRequest creates the update request from flags
func buildUpdateUserRequest(flags *updateUserFlags) api.UpdateUserRequest {
	req := api.UpdateUserRequest{}
	if flags.email != "" {
		req.Email = &flags.email
	}
	if flags.name != "" {
		req.Name = &flags.name
	}
	if flags.role != "" {
		req.Role = &flags.role
	}
	return req
}

// outputUpdateUserSuccess outputs the successful user update response
func outputUpdateUserSuccess(user *api.UserInfo) error {
	response := map[string]any{
		"user":    user,
		"message": "User updated successfully",
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}
