package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

func runCreateUserJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags
	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return fmt.Errorf("failed to get email flag: %w", err)
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	role, err := cmd.Flags().GetString("role")
	if err != nil {
		return fmt.Errorf("failed to get role flag: %w", err)
	}

	// Validate role
	if role != roleAdmin && role != roleUser {
		return outputJSONError(fmt.Sprintf("invalid role: must be '%s' or '%s'", roleAdmin, roleUser))
	}

	log.Debug("creating user in JSON mode",
		"email", email,
		"name", name,
		"role", role)

	// Create the user
	req := CreateUserRequest{
		Email: email,
		Name:  name,
		Role:  role,
	}

	user, err := client.CreateUser(ctx, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to create user: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"user":    user,
		"message": "User created successfully",
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

// userFilters holds the parsed command line flags for user filtering
type userFilters struct {
	roleFilter string
	sortBy     string
	filterStr  string
	activeOnly bool
}

// parseListUsersFlags extracts and validates flags for user listing
func parseListUsersFlags(cmd *cobra.Command) (*userFilters, error) {
	roleFilter, err := cmd.Flags().GetString("role")
	if err != nil {
		return nil, fmt.Errorf("failed to get role flag: %w", err)
	}
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return nil, fmt.Errorf("failed to get sort flag: %w", err)
	}
	filterStr, err := cmd.Flags().GetString("filter")
	if err != nil {
		return nil, fmt.Errorf("failed to get filter flag: %w", err)
	}
	activeOnly, err := cmd.Flags().GetBool("active")
	if err != nil {
		return nil, fmt.Errorf("failed to get active flag: %w", err)
	}

	// Validate role filter
	if roleFilter != "" && roleFilter != roleAdmin && roleFilter != roleUser {
		return nil, fmt.Errorf("invalid role filter: %s (must be '%s' or '%s')", roleFilter, roleAdmin, roleUser)
	}

	// Validate sort field
	validSorts := []string{sortCreated, sortName, sortEmail, sortRole}
	validSort := false
	for _, valid := range validSorts {
		if sortBy == valid {
			validSort = true
			break
		}
	}
	if !validSort {
		return nil, fmt.Errorf("invalid sort field: %s (must be one of: %v)", sortBy, validSorts)
	}

	return &userFilters{
		roleFilter: roleFilter,
		sortBy:     sortBy,
		filterStr:  filterStr,
		activeOnly: activeOnly,
	}, nil
}

// filterAndSortUsers applies filtering and sorting to the user list
func filterAndSortUsers(users []models.UserInfo, filters *userFilters) []models.UserInfo {
	filtered := make([]models.UserInfo, 0, len(users))

	for _, user := range users {
		// Apply role filter
		if filters.roleFilter != "" && user.Role != filters.roleFilter {
			continue
		}

		// Apply text filter (name or email)
		if filters.filterStr != "" {
			if !contains(user.Name, filters.filterStr) && !contains(user.Email, filters.filterStr) {
				continue
			}
		}

		// TODO: Apply active filter when KeyCount field is available
		// For now, include all users when active filter is requested
		// This will be updated when API provides key count information
		_ = filters.activeOnly // Prevent unused variable warning

		filtered = append(filtered, user)
	}

	// Sort users
	sort.Slice(filtered, func(i, j int) bool {
		switch filters.sortBy {
		case sortName:
			return filtered[i].Name < filtered[j].Name
		case sortEmail:
			return filtered[i].Email < filtered[j].Email
		case sortRole:
			return filtered[i].Role < filtered[j].Role
		case sortCreated:
			return filtered[i].CreatedAt < filtered[j].CreatedAt
		default:
			return filtered[i].CreatedAt < filtered[j].CreatedAt
		}
	})

	return filtered
}

func runListUsersJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags
	filters, err := parseListUsersFlags(cmd)
	if err != nil {
		return err
	}

	log.Debug("listing users in JSON mode",
		"role", filters.roleFilter,
		"sort", filters.sortBy,
		"filter", filters.filterStr,
		"activeOnly", filters.activeOnly)

	// Get users from API
	users, err := client.ListUsers(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to list users: %v", err))
	}

	// Apply filters and sorting
	filteredUsers := filterAndSortUsers(users, filters)

	// Prepare response
	response := map[string]any{
		"users": filteredUsers,
		"total": len(filteredUsers),
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}

func runUpdateUserJSON(ctx context.Context, cmd *cobra.Command, client *Client, userID string) error {
	log := logger.FromContext(ctx)

	// Parse flags
	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to get email flag: %v", err))
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to get name flag: %v", err))
	}
	role, err := cmd.Flags().GetString("role")
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to get role flag: %v", err))
	}

	// Validate role if provided
	if role != "" && role != roleUser && role != roleAdmin {
		return outputJSONError(fmt.Sprintf("role must be '%s' or '%s'", roleUser, roleAdmin))
	}

	// Create update request with only specified fields
	req := UpdateUserRequest{}
	if email != "" {
		req.Email = &email
	}
	if name != "" {
		req.Name = &name
	}
	if role != "" {
		req.Role = &role
	}

	// Check if any fields were provided
	if req.Email == nil && req.Name == nil && req.Role == nil {
		return outputJSONError("at least one field (email, name, role) must be provided")
	}

	log.Debug("updating user in JSON mode", "user_id", userID)

	// Update the user
	user, err := client.UpdateUser(ctx, userID, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to update user: %v", err))
	}

	// Output JSON response
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

func runDeleteUserJSON(ctx context.Context, cmd *cobra.Command, client *Client, userID string) error {
	log := logger.FromContext(ctx)

	// Parse flags
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}
	cascade, err := cmd.Flags().GetBool("cascade")
	if err != nil {
		return fmt.Errorf("failed to get cascade flag: %w", err)
	}

	log.Debug("deleting user in JSON mode", "user_id", userID, "force", force, "cascade", cascade)

	// If not forced, show warning and require confirmation
	if !force {
		return outputJSONError("user deletion requires --force flag in JSON mode for safety")
	}

	// TODO: If cascade is enabled, also delete user's API keys
	// This would require additional API endpoint or client method
	if cascade {
		return outputJSONError("cascade deletion is not yet implemented")
	}

	// Delete the user
	err = client.DeleteUser(ctx, userID)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to delete user: %v", err))
	}

	// Output JSON response
	response := map[string]any{
		"message": "User deleted successfully",
		"user_id": userID,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}
