package cli

import (
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/spf13/cobra"
)

func newAutomationJobsCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw     string
		workspaceRef string
		sourceRaw    string
		targetRaw    string
		loopName     string
		search       string
		cursor       string
		enabled      bool
		limit        int
	)

	cmd := &cobra.Command{
		Use:   automationJobsKey,
		Short: "Manage automation jobs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseAutomationJobListQuery(
				cmd,
				deps,
				client,
				scopeRaw,
				workspaceRef,
				sourceRaw,
				targetRaw,
				optionalAutomationEnabled(cmd, enabled),
				loopName,
				search,
				cursor,
				limit,
			)
			if err != nil {
				return err
			}

			query.AllWorkspaces = wantsAllWorkspaces(cmd)
			page, err := client.ListAutomationJobs(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeProjectLabeledOutput(cmd, client, query.AllWorkspaces,
				func(labels ...map[string]string) outputBundle {
					return automationJobListBundle(page, deps.now, labels...)
				})
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, automationScopeKey, "", "Filter by scope: user (aliases: @user, global) or workspace")
	cmd.Flags().
		StringVar(&workspaceRef, "workspace", "", "Override workspace filter (ID, name, or path)")
	cmd.Flags().
		StringVar(&sourceRaw, automationSourceKey, "", "Filter by definition source: config, package, or dynamic")
	cmd.Flags().BoolVar(
		&enabled,
		automationEnabledKey,
		false,
		"Filter by enabled state; use --enabled=false for disabled jobs",
	)
	cmd.Flags().StringVar(&targetRaw, "target", "", "Filter by target: agent, loop, or task")
	cmd.Flags().StringVar(&loopName, "loop", "", "Filter by loop target name")
	cmd.Flags().StringVar(&search, "query", "", "Search jobs by name, agent, prompt, scope, source, or schedule")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue after this automation job cursor")
	cmd.Flags().IntVar(&limit, "limit", automationpkg.DefaultListLimit, "Maximum jobs per page")

	cmd.AddCommand(
		newAutomationJobsCreateCommand(deps), newAutomationJobsGetCommand(deps),
		newAutomationJobsUpdateCommand(deps), newAutomationJobsDeleteCommand(deps),
		newAutomationJobsTriggerCommand(deps), newAutomationJobsHistoryCommand(deps),
	)
	addAllWorkspacesFlag(cmd)
	configureProfileReadCommand(cmd, deps)
	return cmd
}

func newAutomationTriggersCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw     string
		workspaceRef string
		eventRaw     string
		sourceRaw    string
		targetRaw    string
		loopName     string
		search       string
		cursor       string
		enabled      bool
		limit        int
	)

	cmd := &cobra.Command{
		Use:   "triggers",
		Short: "Manage automation triggers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseAutomationTriggerListQuery(
				cmd,
				deps,
				client,
				scopeRaw,
				workspaceRef,
				eventRaw,
				sourceRaw,
				targetRaw,
				optionalAutomationEnabled(cmd, enabled),
				loopName,
				search,
				cursor,
				limit,
			)
			if err != nil {
				return err
			}

			query.AllWorkspaces = wantsAllWorkspaces(cmd)
			page, err := client.ListAutomationTriggers(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeProjectLabeledOutput(cmd, client, query.AllWorkspaces,
				func(labels ...map[string]string) outputBundle {
					return automationTriggerListBundle(page, deps.now, labels...)
				})
		},
	}
	cmd.Flags().
		StringVar(&scopeRaw, automationScopeKey, "", "Filter by scope: user (aliases: @user, global) or workspace")
	cmd.Flags().
		StringVar(&workspaceRef, "workspace", "", "Override workspace filter (ID, name, or path)")
	cmd.Flags().StringVar(&eventRaw, automationEventKey, "", "Filter by activation event")
	cmd.Flags().
		StringVar(&sourceRaw, automationSourceKey, "", "Filter by definition source: config, package, or dynamic")
	cmd.Flags().BoolVar(
		&enabled,
		automationEnabledKey,
		false,
		"Filter by enabled state; use --enabled=false for disabled triggers",
	)
	cmd.Flags().StringVar(&targetRaw, "target", "", "Filter by target: agent, loop, or task")
	cmd.Flags().StringVar(&loopName, "loop", "", "Filter by loop target name")
	cmd.Flags().StringVar(&search, "query", "", "Search triggers by definition or filter fields")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue after this automation trigger cursor")
	cmd.Flags().IntVar(&limit, "limit", automationpkg.DefaultListLimit, "Maximum triggers per page")

	cmd.AddCommand(
		newAutomationTriggersCreateCommand(deps), newAutomationTriggersGetCommand(deps),
		newAutomationTriggersUpdateCommand(deps), newAutomationTriggersDeleteCommand(deps),
		newAutomationTriggersHistoryCommand(deps),
	)
	addAllWorkspacesFlag(cmd)
	configureProfileReadCommand(cmd, deps)
	return cmd
}

func optionalAutomationEnabled(cmd *cobra.Command, value bool) *bool {
	if !cmd.Flags().Changed(automationEnabledKey) {
		return nil
	}
	return &value
}
