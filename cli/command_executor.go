package cli

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/cli/tui/models"
	cliutils "github.com/compozy/compozy/cli/utils"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// CommandExecutor handles common setup and execution patterns for CLI commands.
// It eliminates boilerplate code by providing a single place for:
// - Configuration loading
// - API client creation
// - Output mode detection
// - Context cancellation
// - Error handling
type CommandExecutor struct {
	client *APIClient
	config *config.Config
	mode   models.Mode
}

// HandlerFunc defines the signature for command handlers.
// Args parameter is optional - handlers can ignore it if not needed.
type HandlerFunc func(ctx context.Context, cmd *cobra.Command, client *APIClient, args []string) error

// ModeHandlers contains handlers for different execution modes.
type ModeHandlers struct {
	JSON HandlerFunc
	TUI  HandlerFunc
}

// NewCommandExecutor creates a new command executor with all necessary setup.
// This function handles all the boilerplate that would otherwise be duplicated
// across every CLI command.
func NewCommandExecutor(cmd *cobra.Command) (*CommandExecutor, error) {
	ctx := cmd.Context()
	log := logger.FromContext(ctx)

	// Load configuration using the excellent pkg/config system
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create API client with proper configuration
	client, err := NewAPIClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	// Detect execution mode
	mode := DetectOutputMode(cmd)
	log.Debug("detected output mode", "mode", mode)

	return &CommandExecutor{
		client: client,
		config: cfg,
		mode:   mode,
	}, nil
}

// Execute runs the appropriate handler based on the detected mode.
// This eliminates the mode switching boilerplate that would be repeated
// in every command.
func (e *CommandExecutor) Execute(ctx context.Context, cmd *cobra.Command, handlers ModeHandlers, args []string) error {
	// Create a cancellable context for the command execution
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle context cancellation gracefully
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	switch e.mode {
	case models.ModeJSON:
		if handlers.JSON == nil {
			return fmt.Errorf("JSON mode handler not implemented")
		}
		return handlers.JSON(ctx, cmd, e.client, args)
	case models.ModeTUI:
		if handlers.TUI == nil {
			return fmt.Errorf("TUI mode handler not implemented")
		}
		return handlers.TUI(ctx, cmd, e.client, args)
	default:
		return fmt.Errorf("unsupported mode: %s", e.mode)
	}
}

// GetClient returns the configured API client.
func (e *CommandExecutor) GetClient() *APIClient {
	return e.client
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
// This is the main entry point that eliminates all boilerplate from command handlers.
func ExecuteCommand(cmd *cobra.Command, handlers ModeHandlers, args []string) error {
	executor, err := NewCommandExecutor(cmd)
	if err != nil {
		return err
	}

	return executor.Execute(cmd.Context(), cmd, handlers, args)
}

// ExecuteWithContext is similar to ExecuteCommand but uses a provided context.
// This is useful for commands that need to set up their own context with specific values.
func ExecuteWithContext(ctx context.Context, cmd *cobra.Command, handlers ModeHandlers, args []string) error {
	executor, err := NewCommandExecutor(cmd)
	if err != nil {
		return err
	}

	return executor.Execute(ctx, cmd, handlers, args)
}

// ValidateRequiredFlags checks that all required flags are present and valid.
// This is a helper function that can be used by commands to validate their inputs.
func ValidateRequiredFlags(cmd *cobra.Command, required []string) error {
	for _, flag := range required {
		if !cmd.Flags().Changed(flag) {
			return cliutils.NewCliError("MISSING_FLAG", fmt.Sprintf("required flag '%s' not specified", flag))
		}

		// Check if the flag value is empty
		if value, err := cmd.Flags().GetString(flag); err == nil && value == "" {
			return cliutils.NewCliError("EMPTY_FLAG", fmt.Sprintf("required flag '%s' cannot be empty", flag))
		}
	}
	return nil
}

// SetupGlobalFlags adds common global flags to a command.
// This should be called by the root command to set up global flags.
func SetupGlobalFlags(cmd *cobra.Command) {
	AddGlobalOutputFlags(cmd)

	// Add global configuration flags
	cmd.PersistentFlags().String("config", "", "Path to configuration file")
	cmd.PersistentFlags().String("server-url", "", "Compozy server URL")
	cmd.PersistentFlags().String("api-key", "", "API key for authentication")
	cmd.PersistentFlags().Duration("timeout", 0, "Request timeout duration")
}

// GetGlobalConfig extracts global configuration from command flags.
// This is used to override configuration values with command-line flags.
func GetGlobalConfig(cmd *cobra.Command) map[string]any {
	config := make(map[string]any)

	if configFile, err := cmd.Flags().GetString("config"); err == nil && configFile != "" {
		config["config_file"] = configFile
	}

	if serverURL, err := cmd.Flags().GetString("server-url"); err == nil && serverURL != "" {
		config["server.host"] = serverURL
	}

	if apiKey, err := cmd.Flags().GetString("api-key"); err == nil && apiKey != "" {
		config["cli.api_key"] = apiKey
	}

	if timeout, err := cmd.Flags().GetDuration("timeout"); err == nil && timeout > 0 {
		config["cli.timeout"] = timeout
	}

	return config
}

// HandleCommonErrors provides consistent error handling across all commands.
func HandleCommonErrors(err error, mode models.Mode) error {
	if err == nil {
		return nil
	}

	cliErr := categorizeError(err)

	if cliErr != nil {
		cliutils.OutputError(cliErr, mode)
		return cliErr
	}

	// For unknown errors, still format them based on mode
	cliutils.OutputError(err, mode)
	return err
}

// categorizeError converts errors to structured CLI errors
func categorizeError(err error) *cliutils.CliError {
	switch {
	case err == context.Canceled:
		return cliutils.NewCliError("OPERATION_CANCELED", "Operation was canceled by user")
	case err == context.DeadlineExceeded:
		return cliutils.NewCliError("OPERATION_TIMEOUT", "Operation timed out")
	case cliutils.IsNetworkError(err):
		return cliutils.NewCliError("NETWORK_ERROR", "Network connection failed", err.Error())
	case cliutils.IsAuthError(err):
		return cliutils.NewCliError("AUTH_ERROR", "Authentication failed", err.Error())
	default:
		if apiErr, ok := err.(*APIError); ok && apiErr != nil {
			return cliutils.NewCliError("API_ERROR", apiErr.Message, apiErr.Details)
		}
		return nil
	}
}
