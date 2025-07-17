package handlers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// ListTUI handles key listing in TUI mode using the unified executor pattern
func ListTUI(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	log.Debug("listing API keys in TUI mode")

	authClient := executor.GetAuthClient()
	if authClient == nil {
		return fmt.Errorf("auth client not available")
	}

	// Create table columns
	columns := []table.Column{
		{Title: "Prefix", Width: 20},
		{Title: "Created", Width: 20},
		{Title: "Last Used", Width: 20},
		{Title: "Usage Count", Width: 12},
	}

	// Create and run the TUI model
	m := models.NewListModel[api.KeyInfo](ctx, authClient, columns)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if there was an error
	if model, ok := finalModel.(*models.ListModel[api.KeyInfo]); ok {
		if model.Error() != nil {
			return model.Error()
		}
	}

	return nil
}
