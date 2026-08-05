package cli

import (
	"errors"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/spf13/cobra"
)

func newAutomationSuggestionsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var statusRaw string
	cmd := &cobra.Command{
		Use:   "suggestions",
		Short: "Review consent-first automation suggestions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := parseAutomationSuggestionStatus(statusRaw)
			if err != nil {
				return err
			}
			workspaceID, err := resolveSuggestionWorkspaceID(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.ListAutomationSuggestions(cmd.Context(), workspaceID, status)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationSuggestionListBundle(response))
		},
	}
	cmd.PersistentFlags().StringVar(&workspaceRef, "workspace", "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&statusRaw, automationStatusKey, string(automationpkg.SuggestionStatusPending),
		"Filter by status: pending, accepted, or dismissed")
	cmd.AddCommand(newAutomationSuggestionAcceptCommand(deps, &workspaceRef))
	cmd.AddCommand(newAutomationSuggestionDismissCommand(deps, &workspaceRef))
	return cmd
}

func newAutomationSuggestionAcceptCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return &cobra.Command{
		Use:   "accept <id>",
		Short: "Accept a suggestion and create its Job",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveSuggestionWorkspaceID(cmd, deps, client, *workspaceRef)
			if err != nil {
				return err
			}
			accepted, err := client.AcceptAutomationSuggestion(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationSuggestionAcceptanceBundle(&accepted))
		},
	}
}

func newAutomationSuggestionDismissCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return &cobra.Command{
		Use:   "dismiss <id>",
		Short: "Dismiss an automation suggestion",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveSuggestionWorkspaceID(cmd, deps, client, *workspaceRef)
			if err != nil {
				return err
			}
			dismissed, err := client.DismissAutomationSuggestion(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationSuggestionBundle(dismissed.Suggestion))
		},
	}
}

func resolveSuggestionWorkspaceID(
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	workspaceRef string,
) (string, error) {
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return "", err
	}
	return resolution.ID, nil
}

func parseAutomationSuggestionStatus(raw string) (automationpkg.SuggestionStatus, error) {
	status := automationpkg.SuggestionStatus(strings.TrimSpace(raw))
	if status == "" {
		return "", errors.New("cli: automation suggestion status is required")
	}
	if err := status.Validate("status"); err != nil {
		return "", err
	}
	return status, nil
}
