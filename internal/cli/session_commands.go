package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

type sessionCommandsClient interface {
	ListSessionCommands(context.Context, string) (SessionCommandsRecord, error)
}

func newSessionCommandsCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "commands <session-id>",
		Short:   "List commands available to a session",
		Args:    exactOneNonBlankArg(),
		Example: "  compozy session commands sess_1234 -o json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			catalogClient, ok := client.(sessionCommandsClient)
			if !ok {
				return errors.New("cli: session command catalog is unavailable")
			}
			record, err := catalogClient.ListSessionCommands(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionCommandsBundle(record))
		},
	}
}

func sessionCommandsBundle(record SessionCommandsRecord) outputBundle {
	return listBundle(
		record,
		record.Commands,
		"Session Commands",
		[]string{"COMMAND", "LANE", automationSourceHeader, "SCOPE", "DESCRIPTION"},
		"commands",
		[]string{"command", "lane", automationSourceKey, automationScopeKey, "description"},
		func(item SessionCommandRecord) []string {
			return []string{
				item.CanonicalToken,
				item.Lane,
				sessionCommandSourceLabel(item.Source.Kind, item.Source.ID),
				item.Source.Scope,
				item.Description,
			}
		},
		func(item SessionCommandRecord) []string {
			return []string{
				item.CanonicalToken,
				item.Lane,
				sessionCommandSourceLabel(item.Source.Kind, item.Source.ID),
				item.Source.Scope,
				item.Description,
			}
		},
	)
}

func sessionCommandSourceLabel(kind string, id string) string {
	if strings.TrimSpace(id) == "" {
		return kind
	}
	return kind + ":" + id
}
