package cli

import (
	"errors"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
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
			workspaceID, err := resolveRequiredSuggestionWorkspaceID(cmd, client, workspaceRef)
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
	cmd.PersistentFlags().StringVar(&workspaceRef, "workspace", "", "Workspace path, name, or ID")
	mustMarkPersistentFlagRequired(cmd, "workspace")
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
			workspaceID, err := resolveRequiredSuggestionWorkspaceID(cmd, client, *workspaceRef)
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
			workspaceID, err := resolveRequiredSuggestionWorkspaceID(cmd, client, *workspaceRef)
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

func resolveRequiredSuggestionWorkspaceID(
	cmd *cobra.Command,
	client DaemonClient,
	workspaceRef string,
) (string, error) {
	if strings.TrimSpace(workspaceRef) == "" {
		return "", errors.New("cli: --workspace is required")
	}
	return resolveAutomationWorkspaceID(cmd.Context(), client, workspaceRef)
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
