package cmd

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// getConfigFromContext retrieves the configuration from the command context
func getConfigFromContext(cmd *cobra.Command) (*config.Config, error) {
	ctx := cmd.Context()
	if ctx == nil {
		return nil, fmt.Errorf("failed to get context from command")
	}

	// Debug the context to see what's available
	if ctx.Value(helpers.ConfigKey) == nil {
		return nil, fmt.Errorf("configuration not found in context - context keys may not be properly set")
	}

	cfg, ok := ctx.Value(helpers.ConfigKey).(*config.Config)
	if !ok {
		return nil, fmt.Errorf("configuration found in context but has wrong type: %T", ctx.Value(helpers.ConfigKey))
	}
	if cfg == nil {
		return nil, fmt.Errorf("configuration found in context but is nil")
	}
	return cfg, nil
}

// CommandExecutor handles common setup and execution patterns for CLI commands.
// It eliminates boilerplate code by providing a single place for:
// - Configuration loading
// - Client creation (auth)
// - Mode detection
// - Context cancellation
// - Error handling
type CommandExecutor struct {
	config *config.Config
	mode   models.Mode

	// Clients - only populated as needed
	authClient api.AuthClient
}

// HandlerFunc defines the signature for command handlers.
type HandlerFunc func(ctx context.Context, cmd *cobra.Command, executor *CommandExecutor, args []string) error

// ModeHandlers contains handlers for different execution modes.
type ModeHandlers struct {
	JSON HandlerFunc
	TUI  HandlerFunc
}

// ExecutorOptions allows customization of the command executor
type ExecutorOptions struct {
	RequireAuth bool
}

// NewCommandExecutor creates a new command executor with all necessary setup.
func NewCommandExecutor(cmd *cobra.Command, opts ExecutorOptions) (*CommandExecutor, error) {
	ctx := cmd.Context()
	log := logger.FromContext(ctx)

	// Get configuration from context (processed by root command with all sources including CLI flags)
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration from context: %w", err)
	}

	// Detect execution mode
	mode := helpers.DetectMode(cmd)
	log.Debug("detected execution mode", "mode", mode)

	executor := &CommandExecutor{
		config: cfg,
		mode:   mode,
	}

	// Initialize auth client if required
	if opts.RequireAuth {
		apiKey := getAPIKeyFromConfigOrFlag(cmd, cfg)

		// Only require API key if authentication is enabled
		if cfg.Server.Auth.Enabled && apiKey == "" {
			return nil, fmt.Errorf(
				"API key is required (set CLI.APIKey in config file, COMPOZY_API_KEY environment variable, or use --api-key flag)",
			)
		}

		authClient, err := api.NewAuthClient(cfg, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create auth client: %w", err)
		}
		executor.authClient = authClient
	}

	return executor, nil
}

// getAPIKeyFromConfigOrFlag retrieves the API key from --api-key flag or config
func getAPIKeyFromConfigOrFlag(cmd *cobra.Command, cfg *config.Config) string {
	// Check for --api-key flag first (highest priority)
	if flagValue, err := cmd.Flags().GetString("api-key"); err == nil && flagValue != "" {
		return flagValue
	}

	// Fall back to config value
	return string(cfg.CLI.APIKey)
}

// Execute runs the appropriate handler based on the detected mode.
func (e *CommandExecutor) Execute(ctx context.Context, cmd *cobra.Command, handlers ModeHandlers, args []string) error {
	// Create a cancellable context for the command execution
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	switch e.mode {
	case models.ModeJSON:
		if handlers.JSON == nil {
			return fmt.Errorf("JSON mode handler not implemented")
		}
		return handlers.JSON(ctx, cmd, e, args)
	case models.ModeTUI:
		if handlers.TUI == nil {
			return fmt.Errorf("TUI mode handler not implemented")
		}
		return handlers.TUI(ctx, cmd, e, args)
	default:
		return fmt.Errorf("unsupported mode: %s", e.mode)
	}
}

// GetAuthClient returns the configured auth client.
func (e *CommandExecutor) GetAuthClient() api.AuthClient {
	return e.authClient
}

// GetConfig returns the loaded configuration.
func (e *CommandExecutor) GetConfig() *config.Config {
	return e.config
}

// GetMode returns the detected execution mode.
func (e *CommandExecutor) GetMode() models.Mode {
	return e.mode
}

// ExecuteCommand is a convenience function that combines executor creation and execution.
func ExecuteCommand(cmd *cobra.Command, opts ExecutorOptions, handlers ModeHandlers, args []string) error {
	executor, err := NewCommandExecutor(cmd, opts)
	if err != nil {
		return HandleCommonErrors(err, helpers.DetectMode(cmd))
	}

	return HandleCommonErrors(executor.Execute(cmd.Context(), cmd, handlers, args), executor.GetMode())
}

// ExecuteWithContext is similar to ExecuteCommand but uses a provided context.
func ExecuteWithContext(
	ctx context.Context,
	cmd *cobra.Command,
	opts ExecutorOptions,
	handlers ModeHandlers,
	args []string,
) error {
	executor, err := NewCommandExecutor(cmd, opts)
	if err != nil {
		return HandleCommonErrors(err, helpers.DetectMode(cmd))
	}

	return HandleCommonErrors(executor.Execute(ctx, cmd, handlers, args), executor.GetMode())
}

// ValidateRequiredFlags checks that all required flags are present and valid.
func ValidateRequiredFlags(cmd *cobra.Command, required []string) error {
	for _, flag := range required {
		if !cmd.Flags().Changed(flag) {
			return helpers.NewCliError("MISSING_FLAG", fmt.Sprintf("required flag '%s' not specified", flag))
		}

		// Check if the flag value is empty
		if value, err := cmd.Flags().GetString(flag); err == nil && value == "" {
			return helpers.NewCliError("EMPTY_FLAG", fmt.Sprintf("required flag '%s' cannot be empty", flag))
		}
	}
	return nil
}

// HandleCommonErrors provides consistent error handling across all commands.
func HandleCommonErrors(err error, mode models.Mode) error {
	if err == nil {
		return nil
	}

	cliErr := categorizeError(err)

	if cliErr != nil {
		helpers.OutputError(cliErr, mode)
		return cliErr
	}

	// For unknown errors, still format them based on mode
	helpers.OutputError(err, mode)
	return err
}

// categorizeError converts errors to structured CLI errors
func categorizeError(err error) *helpers.CliError {
	switch {
	case err == context.Canceled:
		return helpers.NewCliError("OPERATION_CANCELED", "Operation was canceled by user")
	case err == context.DeadlineExceeded:
		return helpers.NewCliError("OPERATION_TIMEOUT", "Operation timed out")
	case helpers.IsNetworkError(err):
		return helpers.NewCliError("NETWORK_ERROR", "Network connection failed", err.Error())
	case helpers.IsAuthError(err):
		return helpers.NewCliError("AUTH_ERROR", "Authentication failed", err.Error())
	default:
		return nil
	}
}
