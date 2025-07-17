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

// CreateUserJSON handles user creation in JSON mode using the unified executor pattern
func CreateUserJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)

	// Parse flags
	email, err := cobraCmd.Flags().GetString("email")
	if err != nil {
		return fmt.Errorf("failed to get email flag: %w", err)
	}
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	role, err := cobraCmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}

	// Validate role
	if role != api.RoleAdmin && role != api.RoleUser {
		return outputJSONError(fmt.Sprintf("invalid role: must be '%s' or '%s'", api.RoleAdmin, api.RoleUser))
	}

	log.Debug("creating user in JSON mode",
		"email", email,
		"name", name,
		"role", role)

	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}

	// Create the user
	req := api.CreateUserRequest{
		Email: email,
		Name:  name,
		Role:  role,
	}

	user, err := authClient.CreateUser(ctx, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to create user: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"data":    user,
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
