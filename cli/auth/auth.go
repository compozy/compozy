package auth

import (
	"context"
	"fmt"

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

// GenerateCmd returns the key generation command
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
	AddModeFlags(cmd)

	return cmd
}

// ListCmd returns the key listing command
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Long:  "List all API keys for the authenticated user",
		RunE:  runList,
	}

	// Add flags
	cmd.Flags().String("sort", "created", "Sort by field: created, name, last_used")
	cmd.Flags().String("filter", "", "Filter keys by name or prefix")
	cmd.Flags().Int("page", 1, "Page number for pagination")
	cmd.Flags().Int("limit", 50, "Number of items per page")
	AddModeFlags(cmd)

	return cmd
}

// Wrapper functions for list handlers
func listJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runListJSON(ctx, cmd, client)
}

func listTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runListTUI(ctx, cmd, client)
}

// runList handles the key listing command execution using the new executor pattern
func runList(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: listJSONHandler,
		TUI:  listTUIHandler,
	}, args)
}

// RevokeCmd returns the key revocation command
func RevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke [key-id]",
		Short: "Revoke an API key",
		Long:  "Revoke an API key by ID. In TUI mode, select from a list. In JSON mode, provide the key ID as an argument.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRevoke,
	}

	// Add flags
	cmd.Flags().Bool("force", false, "Skip confirmation prompt (JSON mode only)")
	AddModeFlags(cmd)

	return cmd
}

// Wrapper functions for revoke handlers
func revokeJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	// In JSON mode, require key ID as argument
	if len(args) == 0 {
		return fmt.Errorf("key ID required in JSON mode")
	}
	return runRevokeJSON(ctx, cmd, client, args[0])
}

func revokeTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runRevokeTUI(ctx, cmd, client)
}

// runRevoke handles the key revocation command execution using the new executor pattern
func runRevoke(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: revokeJSONHandler,
		TUI:  revokeTUIHandler,
	}, args)
}

// CreateUserCmd returns the user creation command
func CreateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-user",
		Short: "Create a new user (admin only)",
		Long:  "Create a new user with specified email, name, and role. This command requires admin privileges.",
		RunE:  runCreateUser,
	}

	// Add flags
	cmd.Flags().String("email", "", "Email address for the new user (required)")
	cmd.Flags().String("name", "", "Name of the new user")
	cmd.Flags().String("role", "user", "Role for the new user: admin or user")
	if err := cmd.MarkFlagRequired("email"); err != nil {
		panic(fmt.Sprintf("failed to mark email flag as required: %v", err))
	}
	AddModeFlags(cmd)

	return cmd
}

// Wrapper functions for create-user handlers
func createUserJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runCreateUserJSON(ctx, cmd, client)
}

func createUserTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runCreateUserTUI(ctx, cmd, client)
}

// runCreateUser handles the user creation command execution using the new executor pattern
func runCreateUser(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: createUserJSONHandler,
		TUI:  createUserTUIHandler,
	}, args)
}

// ListUsersCmd returns the user listing command
func ListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-users",
		Short: "List all users (admin only)",
		Long:  "List all users in the system. This command requires admin privileges.",
		RunE:  runListUsers,
	}

	// Add flags
	cmd.Flags().String("role", "", "Filter by role: admin or user")
	cmd.Flags().String("sort", "created", "Sort by field: created, name, email, role")
	cmd.Flags().String("filter", "", "Filter users by name or email")
	cmd.Flags().Bool("active", false, "Show only active users (with API keys)")
	AddModeFlags(cmd)

	return cmd
}

// Wrapper functions for list-users handlers
func listUsersJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runListUsersJSON(ctx, cmd, client)
}

func listUsersTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runListUsersTUI(ctx, cmd, client)
}

// runListUsers handles the user listing command execution using the new executor pattern
func runListUsers(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: listUsersJSONHandler,
		TUI:  listUsersTUIHandler,
	}, args)
}

// Wrapper functions to adapt existing handlers to new signature
func generateJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runGenerateJSON(ctx, cmd, client)
}

func generateTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, _ []string) error {
	return runGenerateTUI(ctx, cmd, client)
}

// runGenerate handles the key generation command execution using the new executor pattern
func runGenerate(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: generateJSONHandler,
		TUI:  generateTUIHandler,
	}, args)
}

// UpdateUserCmd returns the user update command
func UpdateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-user [user-id]",
		Short: "Update a user (admin only)",
		Long:  "Update user information such as email, name, or role. This command requires admin privileges.",
		Args:  cobra.ExactArgs(1),
		RunE:  runUpdateUser,
	}

	// Add flags
	cmd.Flags().String("email", "", "New email address for the user")
	cmd.Flags().String("name", "", "New name for the user")
	cmd.Flags().String("role", "", "New role for the user: admin or user")
	AddModeFlags(cmd)

	return cmd
}

// Wrapper functions for update-user handlers
func updateUserJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	// In JSON mode, require user ID as argument
	if len(args) == 0 {
		return fmt.Errorf("user ID required in JSON mode")
	}
	return runUpdateUserJSON(ctx, cmd, client, args[0])
}

func updateUserTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	// In TUI mode, also require user ID as argument for update-user
	if len(args) == 0 {
		return fmt.Errorf("user ID required")
	}
	return runUpdateUserTUI(ctx, cmd, client, args[0])
}

// runUpdateUser handles the user update command execution using the new executor pattern
func runUpdateUser(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: updateUserJSONHandler,
		TUI:  updateUserTUIHandler,
	}, args)
}

// DeleteUserCmd returns the user deletion command
func DeleteUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-user [user-id]",
		Short: "Delete a user (admin only)",
		Long: "Delete a user and optionally their associated API keys. " +
			"This command requires admin privileges and cannot be undone.",
		Args: cobra.ExactArgs(1),
		RunE: runDeleteUser,
	}

	// Add flags
	cmd.Flags().Bool("force", false, "Skip confirmation prompt")
	cmd.Flags().Bool("cascade", false, "Also delete user's API keys")
	AddModeFlags(cmd)

	return cmd
}

// Wrapper functions for delete-user handlers
func deleteUserJSONHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	// In JSON mode, require user ID as argument
	if len(args) == 0 {
		return fmt.Errorf("user ID required in JSON mode")
	}
	return runDeleteUserJSON(ctx, cmd, client, args[0])
}

func deleteUserTUIHandler(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error {
	// In TUI mode, also require user ID as argument for delete-user
	if len(args) == 0 {
		return fmt.Errorf("user ID required")
	}
	return runDeleteUserTUI(ctx, cmd, client, args[0])
}

// runDeleteUser handles the user deletion command execution using the new executor pattern
func runDeleteUser(cmd *cobra.Command, args []string) error {
	return ExecuteCommand(cmd, ModeHandlers{
		JSON: deleteUserJSONHandler,
		TUI:  deleteUserTUIHandler,
	}, args)
}
