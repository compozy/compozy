package handlers

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

const (
	sortName    = "name"
	sortEmail   = "email"
	sortRole    = "role"
	sortCreated = "created"
)

// userFilters holds the parsed command line flags for user filtering
type userFilters struct {
	roleFilter string
	sortBy     string
	filterStr  string
	activeOnly bool
}

// ListUsersJSON handles user listing in JSON mode using the unified executor pattern.
// All errors are converted to JSON format for consistent API responses.
func ListUsersJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	filters, err := parseListUsersFlags(cobraCmd)
	if err != nil {
		return outputJSONError(err.Error())
	}
	log.Debug("listing users in JSON mode",
		"role", filters.roleFilter,
		"sort", filters.sortBy,
		"filter", filters.filterStr,
		"activeOnly", filters.activeOnly)
	users, err := authClient.ListUsers(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to list users: %v", err))
	}
	filteredUsers := filterAndSortUsers(users, filters)
	response := buildUsersResponse(filteredUsers)
	return outputJSONResponse(response)
}

// parseListUsersFlags extracts and validates flags for user listing.
// Returns Go errors that are converted to JSON format by the caller.
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
	if roleFilter != "" && roleFilter != roleAdmin && roleFilter != roleUser {
		return nil, fmt.Errorf("invalid role filter: %s (must be '%s' or '%s')", roleFilter, roleAdmin, roleUser)
	}
	if !isValidSortField(sortBy) {
		return nil, fmt.Errorf("invalid sort field: %s (must be one of: %v)", sortBy, getValidSortFields())
	}
	return &userFilters{
		roleFilter: roleFilter,
		sortBy:     sortBy,
		filterStr:  filterStr,
		activeOnly: activeOnly,
	}, nil
}

// filterAndSortUsers applies filtering and sorting to the user list
func filterAndSortUsers(users []api.UserInfo, filters *userFilters) []api.UserInfo {
	filtered := make([]api.UserInfo, 0, len(users))
	for _, user := range users {
		if filters.roleFilter != "" && user.Role != filters.roleFilter {
			continue
		}

		if filters.filterStr != "" && !userMatchesTextFilter(&user, filters.filterStr) {
			continue
		}

		// TODO: Apply active filter - currently based on user activity heuristics
		// In the future, this will use KeyCount field when available from API
		if filters.activeOnly && !isUserActive(&user) {
			continue
		}

		filtered = append(filtered, user)
	}
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

// isUserActive determines if a user is considered active based on available data.
// This is a heuristic implementation until KeyCount field is available from API.
// Currently considers a user active if they have recent activity or are admin users.
func isUserActive(user *api.UserInfo) bool {
	if user.Role == roleAdmin {
		return true
	}
	// TODO: For regular users, we use creation time as a proxy for activity
	// This is a temporary heuristic until proper activity tracking is implemented
	// Users created within the last 30 days are considered active
	if user.CreatedAt != "" {
		return isRecentlyCreated(user.CreatedAt)
	}
	return false
}

// isRecentlyCreated checks if a user was created within the last 30 days
func isRecentlyCreated(_ string) bool {
	// TODO: This is a simplified check - in practice, you'd parse the timestamp
	// and compare with current time. For now, return true to avoid filtering
	// until proper activity tracking is implemented
	return true
}

// isValidSortField checks if the sort field is valid
func isValidSortField(sortBy string) bool {
	validSorts := map[string]bool{
		sortCreated: true,
		sortName:    true,
		sortEmail:   true,
		sortRole:    true,
	}
	return validSorts[sortBy]
}

// getValidSortFields returns the list of valid sort fields
func getValidSortFields() []string {
	return []string{sortCreated, sortName, sortEmail, sortRole}
}

// userMatchesTextFilter checks if a user matches the text filter
func userMatchesTextFilter(user *api.UserInfo, filter string) bool {
	return helpers.Contains(user.Name, filter) || helpers.Contains(user.Email, filter)
}

// buildUsersResponse constructs the JSON response for user listing
func buildUsersResponse(users []api.UserInfo) map[string]any {
	return map[string]any{
		"users": users,
		"total": len(users),
	}
}
