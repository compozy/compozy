package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// CreateUserJSON handles user creation in JSON mode using the unified executor pattern

func CreateUserJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	options, err := parseCreateUserFlags(cobraCmd)
	if err != nil {
		return outputJSONError(err.Error())
	}
	if err := validateCreateUserRole(options.role); err != nil {
		return outputJSONError(err.Error())
	}
	log.Debug("creating user in JSON mode",
		"email", options.email,
		"name", options.name,
		"role", options.role)
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	user, err := authClient.CreateUser(ctx, api.CreateUserRequest{
		Email: options.email,
		Name:  options.name,
		Role:  options.role,
	})
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to create user: %v", err))
	}
	return outputJSONResponse(map[string]any{
		"data":    user,
		"message": "Success",
	})
}

type createUserOptions struct {
	email string
	name  string
	role  string
}

func parseCreateUserFlags(cobraCmd *cobra.Command) (*createUserOptions, error) {
	email, err := cobraCmd.Flags().GetString("email")
	if err != nil {
		return nil, fmt.Errorf("failed to get email flag: %w", err)
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	at := strings.IndexByte(email, '@')
	dot := strings.LastIndexByte(email, '.')
	if at <= 0 || dot < at+2 || dot == len(email)-1 {
		return nil, fmt.Errorf("email must be a valid address")
	}
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return nil, fmt.Errorf("failed to get name flag: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	role, err := cobraCmd.Flags().GetString("role")
	if err != nil {
		return nil, fmt.Errorf("failed to get role flag: %w", err)
	}
	return &createUserOptions{
		email: email,
		name:  name,
		role:  role,
	}, nil
}

func validateCreateUserRole(role string) error {
	if role == api.RoleAdmin || role == api.RoleUser {
		return nil
	}
	return fmt.Errorf("invalid role: must be '%s' or '%s'", api.RoleAdmin, api.RoleUser)
}
