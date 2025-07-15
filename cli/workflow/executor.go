package workflow

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/cli/auth"
	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// CommandExecutor handles common setup and execution patterns for workflow commands.
// It eliminates boilerplate code by providing a single place for:
// - Configuration loading
// - API key handling
// - Client creation
// - Mode detection
// - Error handling
type CommandExecutor struct {
	client *Client
	config *config.Config
	mode   models.Mode
}

// HandlerFunc defines the signature for workflow command handlers.
// Args parameter is optional - handlers can ignore it if not needed.
type HandlerFunc func(ctx context.Context, cmd *cobra.Command, client *Client, args []string) error

// ModeHandlers contains handlers for different execution modes.
type ModeHandlers struct {
	JSON HandlerFunc
	TUI  HandlerFunc
}

// NewCommandExecutor creates a new command executor with all necessary setup.
// This function handles all the boilerplate that was previously duplicated
// across every workflow command.
func NewCommandExecutor(cmd *cobra.Command) (*CommandExecutor, error) {
	ctx := cmd.Context()
	log := logger.FromContext(ctx)

	// Load configuration using the excellent pkg/config system
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get API key from CLI config (supports env vars, config files, and flags)
	apiKey := string(cfg.CLI.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf(
			"API key is required (set CLI.APIKey in config file or COMPOZY_API_KEY environment variable)",
		)
	}

	// Create workflow client with proper configuration
	client, err := NewClient(cfg, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow client: %w", err)
	}

	// Detect execution mode
	mode := auth.DetectMode(cmd)
	log.Debug("detected mode", "mode", mode)

	return &CommandExecutor{
		client: client,
		config: cfg,
		mode:   mode,
	}, nil
}

// Execute runs the appropriate handler based on the detected mode.
// This eliminates the mode switching boilerplate that was repeated
// in every command.
func (e *CommandExecutor) Execute(ctx context.Context, cmd *cobra.Command, handlers ModeHandlers, args []string) error {
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

// GetClient returns the configured workflow client.
func (e *CommandExecutor) GetClient() *Client {
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
