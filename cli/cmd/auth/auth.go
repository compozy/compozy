package auth

import (
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/cmd/auth/handlers"
	"github.com/spf13/cobra"
)

// Cmd returns the auth command group
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication and API key management",
		Long:  "Commands for managing API keys and authentication",
	}
	cmd.AddCommand(
		BootstrapCmd(),
		GenerateCmd(),
		ListCmd(),
		RevokeCmd(),
		CreateUserCmd(),
		ListUsersCmd(),
		UpdateUserCmd(),
		DeleteUserCmd(),
	)
	return cmd
}

// GenerateCmd returns the key generation command using the new unified executor pattern
func GenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new API key",
		Long:  "Generate a new API key for authenticating with the Compozy API",
		RunE:  runGenerate,
	}
	cmd.Flags().String("name", "", "Name/description for the API key")
	cmd.Flags().String("description", "", "Detailed description of the API key usage")
	cmd.Flags().String("expires", "", "Expiration date for the key (YYYY-MM-DD format)")
	return cmd
}

// runGenerate handles the key generation command execution using the unified executor pattern
func runGenerate(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.GenerateJSON,
		TUI:  handlers.GenerateTUI,
	}, args)
}

// ListCmd returns the key listing command
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Long:  "List all API keys with optional filtering and sorting",
		RunE:  runList,
	}
	cmd.Flags().String("sort", "created", "Sort by field (created, name, expires)")
	cmd.Flags().String("filter", "", "Filter keys by name or description")
	cmd.Flags().Int("page", 1, "Page number for pagination")
	cmd.Flags().Int("limit", 20, "Number of keys per page")
	return cmd
}

// runList handles the key listing command execution
func runList(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.ListJSON,
		TUI:  handlers.ListTUI,
	}, args)
}

// RevokeCmd returns the key revocation command
func RevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke [key-id]",
		Short: "Revoke an API key",
		Long:  "Revoke an API key by ID",
		RunE:  runRevoke,
	}
	cmd.Flags().Bool("force", false, "Force revocation without confirmation")
	return cmd
}

// runRevoke handles the key revocation command execution
func runRevoke(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.RevokeJSON,
		TUI:  handlers.RevokeTUI,
	}, args)
}

// CreateUserCmd returns the user creation command
func CreateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-user",
		Short: "Create a new user",
		Long:  "Create a new user with specified email, name, and role",
		RunE:  runCreateUser,
	}
	cmd.Flags().String("email", "", "User email address")
	cmd.Flags().String("name", "", "User full name")
	cmd.Flags().String("role", "user", "User role (user or admin)")
	return cmd
}

// runCreateUser handles the user creation command execution
func runCreateUser(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.CreateUserJSON,
		TUI:  handlers.CreateUserTUI,
	}, args)
}

// ListUsersCmd returns the user listing command
func ListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-users",
		Short: "List all users",
		Long:  "List all users with optional filtering and sorting",
		RunE:  runListUsers,
	}
	cmd.Flags().String("sort", "created", "Sort by field (created, name, email, role)")
	cmd.Flags().String("filter", "", "Filter users by name or email")
	cmd.Flags().String("role", "", "Filter by role (user or admin)")
	cmd.Flags().Bool("active", false, "Show only active users")
	cmd.Flags().Int("page", 1, "Page number for pagination")
	cmd.Flags().Int("limit", 20, "Number of users per page")
	return cmd
}

// runListUsers handles the user listing command execution
func runListUsers(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.ListUsersJSON,
		TUI:  handlers.ListUsersTUI,
	}, args)
}

// UpdateUserCmd returns the user update command
func UpdateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-user [user-id]",
		Short: "Update an existing user",
		Long:  "Update an existing user's email, name, or role",
		RunE:  runUpdateUser,
	}
	cmd.Flags().String("email", "", "New user email address")
	cmd.Flags().String("name", "", "New user full name")
	cmd.Flags().String("role", "", "New user role (user or admin)")
	return cmd
}

// runUpdateUser handles the user update command execution
func runUpdateUser(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.UpdateUserJSON,
		TUI:  handlers.UpdateUserTUI,
	}, args)
}

// DeleteUserCmd returns the user deletion command
func DeleteUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-user [user-id]",
		Short: "Delete a user",
		Long:  "Delete a user by ID",
		RunE:  runDeleteUser,
	}
	cmd.Flags().Bool("force", false, "Force deletion without confirmation")
	cmd.Flags().Bool("cascade", false, "Cascade delete related resources")
	return cmd
}

// runDeleteUser handles the user deletion command execution
func runDeleteUser(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
	}, cmd.ModeHandlers{
		JSON: handlers.DeleteUserJSON,
		TUI:  handlers.DeleteUserTUI,
	}, args)
}
