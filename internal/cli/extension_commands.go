package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newExtensionCommandsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use:   "commands [extension]",
		Short: "List active extension-contributed commands",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 || (len(args) == 1 && strings.TrimSpace(args[0]) == "") {
				return fmt.Errorf("cli: commands accepts at most one extension name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireExtensionDaemonClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			filter := ""
			if len(args) == 1 {
				filter = strings.TrimSpace(args[0])
			}
			response, err := client.ListExtensionCommands(cmd.Context(), filter, workspaceID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionCommandsBundle(response))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return cmd
}

func extensionCommandsBundle(response ExtensionCommandsRecord) outputBundle {
	commands := slices.Clone(response.Commands)
	return outputBundle{
		jsonValue: commands,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLines(cmd, commands)
		},
		human: func() (string, error) {
			return renderExtensionCommandTree(response), nil
		},
		toon: func() (string, error) {
			rows := make([][]string, 0, len(commands))
			for _, command := range commands {
				rows = append(rows, []string{
					command.Extension, command.Verb, command.ToolID,
					string(command.RiskClass), formatBool(command.ApprovalRequired),
				})
			}
			return renderToonArray(
				"extension_commands",
				[]string{
					extensionExtensionKey,
					automationPathKey,
					toolOperatorToolIDKey,
					"risk_class",
					"approval_required",
				},
				rows,
			), nil
		},
	}
}

func renderExtensionCommandTree(response ExtensionCommandsRecord) string {
	groups := make(map[string]contract.ExtensionCommandGroupPayload, len(response.Groups))
	for _, group := range response.Groups {
		groups[group.Extension+"/"+group.Path] = group
	}
	rows := make([][]string, 0, len(response.Commands)+len(response.Groups))
	renderedGroups := make(map[string]struct{}, len(groups))
	for _, command := range response.Commands {
		parts := strings.Split(command.Verb, "/")
		path := command.Verb
		if len(parts) == 2 {
			key := command.Extension + "/" + parts[0]
			if _, rendered := renderedGroups[key]; !rendered {
				group := groups[key]
				rows = append(rows, []string{command.Extension, parts[0] + "/", "—", "—", group.Summary})
				renderedGroups[key] = struct{}{}
			}
			path = "  " + command.Verb
		}
		rows = append(rows, []string{
			command.Extension,
			path,
			command.ToolID,
			string(command.RiskClass),
			command.Summary,
		})
	}
	return renderHumanTable(
		"Extension Commands",
		[]string{"EXTENSION", "COMMAND", toolOperatorToolIDHeader, "RISK", "SUMMARY"},
		rows,
	)
}
