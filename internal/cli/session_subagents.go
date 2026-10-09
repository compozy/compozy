package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const subagentIDKey = "subagent_id"

func newSessionSubagentsCommand(deps commandDeps) *cobra.Command {
	query := SubagentListQuery{}
	cmd := &cobra.Command{
		Use: "subagents <session-id>", Short: "List a session's subagents", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if query.Limit < 1 || query.Limit > 200 {
				return errors.New("cli: --limit must be between 1 and 200")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			subagents, ok := client.(sessionSubagentsClient)
			if !ok {
				return errors.New("cli: subagents are unavailable")
			}
			result, err := subagents.ListSessionSubagents(cmd.Context(), strings.TrimSpace(args[0]), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				subagentOutput(result, func() (string, error) { return subagentTable(result, deps.now()) }),
			)
		},
	}
	cmd.PersistentFlags().
		String(workspaceSkillSource, "", "Override workspace (ID, name, or path); fast path for show/cancel "+
			"that avoids searching across workspaces")
	cmd.Flags().StringVar(&query.Origin, "origin", "", "Filter by delegated or provider_native origin")
	cmd.Flags().StringVar(&query.Status, automationStatusKey, "", "Filter by comma-separated statuses")
	cmd.Flags().IntVar(&query.Limit, "limit", 50, "Subagents per page (1-200)")
	cmd.Flags().StringVar(&query.Cursor, "cursor", "", "Continue from an opaque next_cursor")
	cmd.AddCommand(newSubagentShowCommand(deps), newSubagentCancelCommand(deps))
	return cmd
}

func newSubagentShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "show <subagent-id>", Short: "Show a subagent and its result", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := subagentCommandClient(cmd, deps)
			if err != nil {
				return err
			}
			row, err := client.GetSubagent(cmd.Context(), workspaceID, strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				subagentOutput(row, func() (string, error) { return subagentDetails(row, deps.now()), nil }),
			)
		},
	}
}

func newSubagentCancelCommand(deps commandDeps) *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use: "cancel <subagent-id>", Short: "Cancel a delegated subagent", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := subagentCommandClient(cmd, deps)
			if err != nil {
				return err
			}
			id := strings.TrimSpace(args[0])
			row, err := client.GetSubagent(cmd.Context(), workspaceID, id)
			if err != nil {
				return err
			}
			result, err := client.CancelSubagent(cmd.Context(), row.WorkspaceID, id, reason)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, subagentOutput(result, func() (string, error) {
				if result.Status != "cancel_requested" {
					return fmt.Sprintf("Subagent %s (%s) is %s.\n", id, row.Title, result.Status), nil
				}
				return fmt.Sprintf("Cancel requested for %s (%s).\n", id, row.Title), nil
			}))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for cancellation")
	return cmd
}

func subagentCommandClient(cmd *cobra.Command, deps commandDeps) (sessionSubagentsClient, string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	subagents, ok := client.(sessionSubagentsClient)
	if !ok {
		return nil, "", errors.New("cli: subagents are unavailable")
	}
	workspaceID, err := resolveWorkspaceFlagOverride(cmd, deps, client, false)
	return subagents, workspaceID, err
}

func subagentOutput(value any, human func() (string, error)) outputBundle {
	return outputBundle{jsonValue: value, human: human,
		json:  func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, value) },
		jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, value) },
		toon:  func() (string, error) { return subagentToon(value), nil },
	}
}

func subagentTable(page contract.SubagentListPayload, now time.Time) (string, error) {
	var output strings.Builder
	writer := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "ID\tTITLE\tRUNTIME\tSTATUS\tELAPSED"); err != nil {
		return "", err
	}
	for _, row := range page.Subagents {
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%s\t%s/%s\t%s\t%s\n",
			row.SubagentID,
			row.Title,
			row.Runtime.Provider,
			row.Runtime.Model,
			row.Status,
			subagentElapsed(row, now),
		); err != nil {
			return "", err
		}
	}
	if err := writer.Flush(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func subagentElapsed(row contract.SubagentPayload, now time.Time) string {
	if row.StartedAt == nil {
		return "—"
	}
	end := now
	if row.SettledAt != nil {
		end = *row.SettledAt
	}
	seconds := max(0, int(end.Sub(*row.StartedAt).Seconds()))
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds >= 3600 {
		return fmt.Sprintf("%dh %02dm", seconds/3600, seconds%3600/60)
	}
	return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
}

func subagentDetails(row contract.SubagentPayload, now time.Time) string {
	child := "—"
	if row.ChildSessionID != nil {
		child = *row.ChildSessionID
	}
	started := "—"
	if row.StartedAt != nil {
		started = row.StartedAt.Format("2006-01-02 15:04:05")
	}
	if row.SettledAt != nil {
		started += " · settled " + row.SettledAt.Format("2006-01-02 15:04:05") + " (" + subagentElapsed(row, now) + ")"
	}
	runtime := []string{}
	for _, value := range []string{
		row.Runtime.Provider, row.Runtime.Model, row.Runtime.ReasoningEffort, row.Runtime.Speed,
	} {
		if value != "" {
			runtime = append(runtime, value)
		}
	}
	output := fmt.Sprintf(
		"Subagent      %s\nTitle         %s\nParent        %s (turn %s)\nChild         %s\n"+
			"Runtime       %s\nStatus        %s (%s) · %s\nStarted       %s\n",
		row.SubagentID,
		row.Title,
		row.ParentSessionID,
		row.ParentTurnID,
		child,
		strings.Join(runtime, " · "),
		row.Status,
		strings.ReplaceAll(row.WorkState, "_", " "),
		row.Delivery,
		started,
	)
	if row.Result != nil {
		output += "\nResult\n" + *row.Result + "\n"
	}
	if row.Error != nil {
		output += "\nError\n" + *row.Error + "\n"
	}
	return output
}

func subagentToon(value any) string {
	switch row := value.(type) {
	case contract.SubagentCancelPayload:
		return renderToonObject(
			"subagent",
			[]string{subagentIDKey, automationStatusKey},
			[]string{row.SubagentID, row.Status},
		)
	case contract.SubagentPayload:
		result := ""
		if row.Result != nil {
			result = *row.Result
		}
		return renderToonObject(
			"subagent",
			[]string{subagentIDKey, "title", automationStatusKey, "result"},
			[]string{row.SubagentID, row.Title, row.Status, result},
		)
	case contract.SubagentListPayload:
		rows := make([][]string, 0, len(row.Subagents))
		for _, item := range row.Subagents {
			rows = append(
				rows,
				[]string{item.SubagentID, item.Title, item.Runtime.Provider + "/" + item.Runtime.Model, item.Status},
			)
		}
		return renderToonArray("subagents", []string{subagentIDKey, "title", "runtime", automationStatusKey}, rows)
	}
	return ""
}
