package auth

import (
	"context"

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

	// Add subcommands
	cmd.AddCommand(
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

	// Add flags
	cmd.Flags().String("name", "", "Name/description for the API key")
	cmd.Flags().String("description", "", "Detailed description of the API key usage")
	cmd.Flags().String("expires", "", "Expiration date for the key (YYYY-MM-DD format)")

	return cmd
}

// runGenerate handles the key generation command execution using the unified executor pattern
func runGenerate(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: generateJSONHandler,
		TUI:  generateTUIHandler,
	}, args)
}

// generateJSONHandler uses the new handlers with unified executor pattern
func generateJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.GenerateJSON(ctx, cobraCmd, executor, args)
}

// generateTUIHandler uses the new handlers with unified executor pattern
func generateTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.GenerateTUI(ctx, cobraCmd, executor, args)
}

// ListCmd returns the key listing command
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Long:  "List all API keys with optional filtering and sorting",
		RunE:  runList,
	}

	// Add flags
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
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: listJSONHandler,
		TUI:  listTUIHandler,
	}, args)
}

// listJSONHandler handles key listing in JSON mode
func listJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.ListJSON(ctx, cobraCmd, executor, args)
}

// listTUIHandler handles key listing in TUI mode
func listTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.ListTUI(ctx, cobraCmd, executor, args)
}

// RevokeCmd returns the key revocation command
func RevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke [key-id]",
		Short: "Revoke an API key",
		Long:  "Revoke an API key by ID",
		RunE:  runRevoke,
	}

	// Add flags
	cmd.Flags().Bool("force", false, "Force revocation without confirmation")

	return cmd
}

// runRevoke handles the key revocation command execution
func runRevoke(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: revokeJSONHandler,
		TUI:  revokeTUIHandler,
	}, args)
}

// revokeJSONHandler handles key revocation in JSON mode
func revokeJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.RevokeJSON(ctx, cobraCmd, executor, args)
}

// revokeTUIHandler handles key revocation in TUI mode
func revokeTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.RevokeTUI(ctx, cobraCmd, executor, args)
}

// CreateUserCmd returns the user creation command
func CreateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-user",
		Short: "Create a new user",
		Long:  "Create a new user with specified email, name, and role",
		RunE:  runCreateUser,
	}

	// Add flags
	cmd.Flags().String("email", "", "User email address")
	cmd.Flags().String("name", "", "User full name")
	cmd.Flags().String("role", "user", "User role (user or admin)")

	return cmd
}

// runCreateUser handles the user creation command execution
func runCreateUser(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: createUserJSONHandler,
		TUI:  createUserTUIHandler,
	}, args)
}

// createUserJSONHandler handles user creation in JSON mode
func createUserJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.CreateUserJSON(ctx, cobraCmd, executor, args)
}

// createUserTUIHandler handles user creation in TUI mode
func createUserTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.CreateUserTUI(ctx, cobraCmd, executor, args)
}

// ListUsersCmd returns the user listing command
func ListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-users",
		Short: "List all users",
		Long:  "List all users with optional filtering and sorting",
		RunE:  runListUsers,
	}

	// Add flags
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
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: listUsersJSONHandler,
		TUI:  listUsersTUIHandler,
	}, args)
}

// listUsersJSONHandler handles user listing in JSON mode
func listUsersJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.ListUsersJSON(ctx, cobraCmd, executor, args)
}

// listUsersTUIHandler handles user listing in TUI mode
func listUsersTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.ListUsersTUI(ctx, cobraCmd, executor, args)
}

// UpdateUserCmd returns the user update command
func UpdateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-user [user-id]",
		Short: "Update an existing user",
		Long:  "Update an existing user's email, name, or role",
		RunE:  runUpdateUser,
	}

	// Add flags
	cmd.Flags().String("email", "", "New user email address")
	cmd.Flags().String("name", "", "New user full name")
	cmd.Flags().String("role", "", "New user role (user or admin)")

	return cmd
}

// runUpdateUser handles the user update command execution
func runUpdateUser(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: updateUserJSONHandler,
		TUI:  updateUserTUIHandler,
	}, args)
}

// updateUserJSONHandler handles user update in JSON mode
func updateUserJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.UpdateUserJSON(ctx, cobraCmd, executor, args)
}

// updateUserTUIHandler handles user update in TUI mode
func updateUserTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.UpdateUserTUI(ctx, cobraCmd, executor, args)
}

// DeleteUserCmd returns the user deletion command
func DeleteUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-user [user-id]",
		Short: "Delete a user",
		Long:  "Delete a user by ID",
		RunE:  runDeleteUser,
	}

	// Add flags
	cmd.Flags().Bool("force", false, "Force deletion without confirmation")
	cmd.Flags().Bool("cascade", false, "Cascade delete related resources")

	return cmd
}

// runDeleteUser handles the user deletion command execution
func runDeleteUser(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: true,
		RequireAPI:  true,
	}, cmd.ModeHandlers{
		JSON: deleteUserJSONHandler,
		TUI:  deleteUserTUIHandler,
	}, args)
}

// deleteUserJSONHandler handles user deletion in JSON mode
func deleteUserJSONHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.DeleteUserJSON(ctx, cobraCmd, executor, args)
}

// deleteUserTUIHandler handles user deletion in TUI mode
func deleteUserTUIHandler(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	args []string,
) error {
	return handlers.DeleteUserTUI(ctx, cobraCmd, executor, args)
}
