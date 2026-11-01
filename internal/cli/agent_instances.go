package cli

import (
	"errors"
	"fmt"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
	"net/url"
	"strconv"
	"strings"
)

func newAgentInstanceCommand(deps commandDeps) *cobra.Command {
	parent := &cobra.Command{Use: "instance", Short: "Manage stays-on agent instances"}
	for _, action := range []string{"create", "list", "show", "pause", "resume", "archive", "adopt", "ensure-main", "reports"} {
		parent.AddCommand(newAgentInstanceAction(deps, action))
	}
	return parent
}

func newAgentInstanceAction(deps commandDeps, action string) *cobra.Command {
	var global, all bool
	var state, cursor, sessionID, agent string
	var limit int
	cmd := &cobra.Command{Use: action + " <agent-or-instance-id>", Short: "Manage agent instances: " + action, Args: exactOneNonBlankArg()}
	if action == "list" {
		cmd.Use = "list"
		cmd.Args = cobra.NoArgs
	}
	cmd.Flags().String("workspace", "", "Workspace ID, name, or path")
	cmd.Flags().BoolVar(&global, "global", false, "Use the global scope")
	cmd.MarkFlagsMutuallyExclusive("global", "workspace")
	if action == "list" || action == "reports" {
		cmd.Flags().StringVar(&state, "state", "", "Filter by state")
		cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from next_cursor")
		cmd.Flags().IntVar(&limit, "limit", 50, "Page size (1-100)")
	}
	if action == "list" {
		cmd.Flags().BoolVar(&all, "all-workspaces", false, "Include every workspace")
		cmd.Flags().StringVar(&agent, "agent", "", "Filter by agent name")
		cmd.MarkFlagsMutuallyExclusive("all-workspaces", "global", "workspace")
	}
	if action == "adopt" {
		cmd.Flags().StringVar(&sessionID, "session", "", "Root session to adopt")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		base, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		client, ok := base.(agentInstancesClient)
		if !ok {
			return errors.New("cli: agent instances unavailable")
		}
		workspace := "@user"
		if !global && !all && action != "adopt" {
			query, err := agentQueryFromCommand(cmd, deps, base)
			if err != nil {
				return err
			}
			workspace = query.Workspace
		}
		query := url.Values{"workspace": {workspace}}
		if action == "list" || action == "reports" {
			if limit < 1 || limit > 100 {
				return errors.New("cli: --limit must be from 1 to 100")
			}
			query.Set("limit", strconv.Itoa(limit))
			query.Set("state", state)
			query.Set("cursor", cursor)
		}
		if action == "list" {
			query.Set("all_workspaces", strconv.FormatBool(all))
			query.Set("agent", strings.TrimSpace(agent))
			result, err := client.ListAgentInstances(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeInstanceOutput(cmd, result, instanceListText(result))
		}
		target := strings.TrimSpace(args[0])
		if action == "create" {
			result, err := client.CreateAgentInstance(cmd.Context(), contract.AgentInstanceCreateRequest{Agent: target, Workspace: workspace})
			if err != nil {
				return err
			}
			return writeInstanceOutput(cmd, result, instanceText(result.Instance))
		}
		if action == "adopt" {
			if strings.TrimSpace(sessionID) == "" {
				return errors.New("cli: --session is required")
			}
			result, err := client.AdoptAgentInstance(cmd.Context(), contract.AgentInstanceAdoptRequest{Agent: target, SessionID: strings.TrimSpace(sessionID)})
			if err != nil {
				return err
			}
			return writeInstanceOutput(cmd, result, instanceText(result.Instance))
		}
		id, err := resolveInstanceID(cmd, client, target, workspace)
		if err != nil {
			return err
		}
		if action == "reports" {
			result, err := client.AgentInstanceReports(cmd.Context(), id, query)
			if err != nil {
				return err
			}
			var text strings.Builder
			for _, report := range result.Reports {
				fmt.Fprintf(&text, "%s\t%s\t%s\n", report.ID, report.Kind, report.State)
			}
			return writeInstanceOutput(cmd, result, text.String())
		}
		result, err := client.MutateAgentInstance(cmd.Context(), id, action)
		if err != nil {
			return err
		}
		if action == "archive" {
			return writeInstanceOutput(cmd, map[string]string{"id": id}, "Instance archived. Its sessions stay in history.\n")
		}
		return writeInstanceOutput(cmd, result, instanceText(result.Instance))
	}
	return cmd
}

func resolveInstanceID(cmd *cobra.Command, client agentInstancesClient, target, workspace string) (string, error) {
	if strings.HasPrefix(target, "ainst-") {
		return target, nil
	}
	query := url.Values{"workspace": {workspace}, "agent": {target}, "limit": {"100"}}
	for {
		result, err := client.ListAgentInstances(cmd.Context(), query)
		if err != nil {
			return "", err
		}
		for _, instance := range result.Instances {
			if instance.AgentName == target {
				return instance.ID, nil
			}
		}
		if result.NextCursor == "" {
			return "", fmt.Errorf("cli: instance for agent %q not found", target)
		}
		query.Set("cursor", result.NextCursor)
	}
}
func writeInstanceOutput(cmd *cobra.Command, value any, human string) error {
	return writeCommandOutput(cmd, outputBundle{jsonValue: value,
		human: func() (string, error) { return human, nil },
		json:  func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, value) },
		jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, value) },
	})
}
func instanceText(instance contract.AgentInstancePayload) string {
	return fmt.Sprintf("%s\t%s\t%s\nMain session: %s\n", instance.ID, instance.AgentName, instance.State, instance.MainSessionID)
}
func instanceListText(page contract.AgentInstanceListResponse) string {
	var text strings.Builder
	for _, instance := range page.Instances {
		text.WriteString(instanceText(instance))
	}
	if page.NextCursor != "" {
		fmt.Fprintf(&text, "Next cursor: %s\n", page.NextCursor)
	}
	return text.String()
}
