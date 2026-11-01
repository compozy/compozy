package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newAgentInstanceCommand(deps commandDeps) *cobra.Command {
	parent := &cobra.Command{Use: "instance", Short: "Manage stays-on agent instances"}
	for _, action := range []string{
		"create",
		agentListKey,
		"show",
		"pause",
		"resume",
		workspaceArchiveAction,
		agentInstanceAdoptAction,
		"ensure-main",
		"reports",
		agentInstanceCheckAction,
		agentInstanceAwayAction,
		agentInstanceMarkSeenAction,
		"check-ins",
	} {
		parent.AddCommand(newAgentInstanceAction(deps, action))
	}
	checks := &cobra.Command{Use: "checks", Short: "Turn proactive checks on or off"}
	for _, action := range []string{"on", agentInstanceChecksOff} {
		checks.AddCommand(newAgentInstanceAction(deps, action))
	}
	parent.AddCommand(checks, newAgentInstanceOccurrenceCommand(deps))
	return parent
}

func newAgentInstanceAction(deps commandDeps, action string) *cobra.Command {
	var options agentInstanceOptions
	cmd := &cobra.Command{
		Use:   action + " <agent-or-instance-id>",
		Short: "Manage agent instances: " + action,
		Args:  exactOneNonBlankArg(),
	}
	if action == agentListKey {
		cmd.Use = agentListKey
		cmd.Args = cobra.NoArgs
	}
	cmd.Flags().String(workspaceSkillSource, "", "Workspace ID, name, or path")
	cmd.Flags().BoolVar(&options.global, "global", false, "Use the global scope")
	cmd.MarkFlagsMutuallyExclusive("global", workspaceSkillSource)
	if action == agentListKey || action == "reports" || action == "check-ins" {
		if action != "check-ins" {
			cmd.Flags().StringVar(&options.state, "state", "", "Filter by state")
		}
		cmd.Flags().StringVar(&options.cursor, "cursor", "", "Continue from next_cursor")
		cmd.Flags().IntVar(&options.limit, listLimitField, 50, "Page size (1-100)")
	}
	if action == agentListKey {
		cmd.Flags().BoolVar(&options.all, "all-workspaces", false, "Include every workspace")
		cmd.Flags().StringVar(&options.agent, "agent", "", "Filter by agent name")
		cmd.MarkFlagsMutuallyExclusive("all-workspaces", "global", workspaceSkillSource)
	}
	if action == agentInstanceCheckAction {
		cmd.Flags().BoolVar(&options.noWait, "no-wait", false, "Return the occurrence ID without waiting")
		cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Inspect gates without running a check")
	}
	if action == agentInstanceAdoptAction {
		cmd.Flags().StringVar(&options.sessionID, "session", "", "Root session to adopt")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return options.run(cmd, deps, action, args)
	}
	return cmd
}

func resolveInstanceID(cmd *cobra.Command, client agentInstancesClient, target, workspace string) (string, error) {
	if strings.HasPrefix(target, "ainst-") {
		return target, nil
	}
	query := url.Values{workspaceSkillSource: {workspace}, "agent": {target}, listLimitField: {"100"}}
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
func instanceActionText(instance contract.AgentInstancePayload, action string) string {
	label, scope := instance.Label, instance.Scope
	if label == "" {
		label = instance.AgentName
	}
	if scope == "" {
		scope = "Global"
	}
	switch action {
	case "create", agentInstanceAdoptAction:
		return fmt.Sprintf("%s is stays-on in %s\nMain session: %s\n", label, scope, instance.MainSessionID)
	case "pause":
		return fmt.Sprintf("%s paused in %s\n", label, scope)
	case "resume":
		return fmt.Sprintf("%s resumed in %s\n", label, scope)
	case workspaceArchiveAction:
		return fmt.Sprintf("%s archived in %s. Its sessions stay in history.\n", label, scope)
	case "on", agentInstanceChecksOff:
		return fmt.Sprintf("Checks %s for %s in %s\n", action, label, scope)
	case "ensure-main":
		return fmt.Sprintf("Main session: %s\n", instance.MainSessionID)
	default:
		return instanceText(instance)
	}
}
func instanceText(instance contract.AgentInstancePayload) string {
	return fmt.Sprintf(
		"%s\t%s\t%s\nMain session: %s\n",
		instance.ID,
		instance.AgentName,
		instance.State,
		instance.MainSessionID,
	)
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

type agentInstanceOptions struct {
	global, all, dryRun, noWait     bool
	state, cursor, sessionID, agent string
	limit                           int
}

func (options agentInstanceOptions) run(cmd *cobra.Command, deps commandDeps, action string, args []string) error {
	base, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	client, ok := base.(agentInstancesClient)
	if !ok {
		return errors.New("cli: agent instances unavailable")
	}
	query, err := options.query(cmd, deps, base, action)
	if err != nil {
		return err
	}
	workspace := query.Get(workspaceSkillSource)
	if action == agentListKey {
		query.Set("all_workspaces", strconv.FormatBool(options.all))
		query.Set("agent", strings.TrimSpace(options.agent))
		result, err := client.ListAgentInstances(cmd.Context(), query)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, instanceListText(result))
	}
	target := strings.TrimSpace(args[0])
	if action == "create" || action == agentInstanceAdoptAction {
		return options.createOrAdopt(cmd, client, action, target, workspace)
	}
	id, err := resolveInstanceID(cmd, client, target, workspace)
	if err != nil {
		return err
	}

	if action == agentInstanceCheckAction || action == "on" || action == agentInstanceChecksOff {
		return options.check(cmd, base, action, id)
	}
	if action == agentInstanceAwayAction || action == agentInstanceMarkSeenAction || action == "check-ins" {
		return writeInstanceRead(cmd, base, action, id, query)
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
	return mutateAgentInstance(cmd, client, action, id)
}

func (options agentInstanceOptions) query(
	cmd *cobra.Command,
	deps commandDeps,
	base workspaceLookupClient,
	action string,
) (url.Values, error) {
	workspace := "@user"
	if !options.global && !options.all && action != agentInstanceAdoptAction {
		query, err := agentQueryFromCommand(cmd, deps, base)
		if err != nil {
			return nil, err
		}
		workspace = query.Workspace
	}
	query := url.Values{workspaceSkillSource: {workspace}}
	if action == agentListKey || action == "reports" || action == "check-ins" {
		if options.limit < 1 || options.limit > 100 {
			return nil, errors.New("cli: --limit must be from 1 to 100")
		}
		query.Set(listLimitField, strconv.Itoa(options.limit))
		if action != "check-ins" {
			query.Set("state", options.state)
		}
		query.Set("cursor", options.cursor)
	}
	return query, nil
}

func (options agentInstanceOptions) check(cmd *cobra.Command, base any, action, id string) error {
	checks, ok := base.(agentInstanceChecksClient)
	if !ok {
		return errors.New("cli: instance checks unavailable")
	}
	if action == agentInstanceCheckAction {
		result, err := checks.CheckAgentInstance(cmd.Context(), id, options.dryRun)
		if err != nil {
			return err
		}
		if !options.noWait && result.OccurrenceID != "" {
			reader, ok := base.(agentInstanceOccurrenceClient)
			if !ok {
				return errors.New("cli: instance occurrences unavailable")
			}
			return waitInstanceOccurrence(cmd, reader, id, result.OccurrenceID, defaultPollInterval)
		}
		return writeInstanceOutput(cmd, result, fmt.Sprintf("%s%s\n", result.Gate, result.OccurrenceID))
	}
	result, err := checks.SetAgentInstanceChecks(cmd.Context(), id, action == "on")
	if err != nil {
		return err
	}
	return writeInstanceOutput(cmd, result, instanceActionText(result.Instance, action))
}

func (options agentInstanceOptions) createOrAdopt(
	cmd *cobra.Command,
	client agentInstancesClient,
	action, target, workspace string,
) error {
	if action == "create" {
		result, err := client.CreateAgentInstance(
			cmd.Context(),
			contract.AgentInstanceCreateRequest{Agent: target, Workspace: workspace},
		)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, instanceActionText(result.Instance, action))
	}
	if strings.TrimSpace(options.sessionID) == "" {
		return errors.New("cli: --session is required")
	}
	result, err := client.AdoptAgentInstance(
		cmd.Context(),
		contract.AgentInstanceAdoptRequest{Agent: target, SessionID: strings.TrimSpace(options.sessionID)},
	)
	if err != nil {
		return err
	}
	return writeInstanceOutput(cmd, result, instanceActionText(result.Instance, action))
}

func mutateAgentInstance(cmd *cobra.Command, client agentInstancesClient, action, id string) error {
	var err error
	var before contract.AgentInstanceResponse
	if action == workspaceArchiveAction {
		before, err = client.MutateAgentInstance(cmd.Context(), id, "show")
		if err != nil {
			return err
		}
	}
	result, err := client.MutateAgentInstance(cmd.Context(), id, action)
	if err != nil {
		return err
	}
	if action == workspaceArchiveAction {
		return writeInstanceOutput(cmd, map[string]string{"id": id}, instanceActionText(before.Instance, action))
	}
	return writeInstanceOutput(cmd, result, instanceActionText(result.Instance, action))
}

const (
	agentInstanceAdoptAction    = "adopt"
	agentInstanceChecksOff      = "off"
	agentInstanceAwayAction     = "away"
	agentInstanceMarkSeenAction = "mark-seen"
)

const agentInstanceCheckAction = "check"
