package cli

import (
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/spf13/cobra"
)

func newAutomationJobsCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw     string
		workspaceRef string
		sourceRaw    string
		loopName     string
		search       string
		cursor       string
		enabled      bool
		limit        int
	)

	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage automation jobs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseAutomationJobListQuery(
				cmd.Context(),
				client,
				scopeRaw,
				workspaceRef,
				sourceRaw,
				optionalAutomationEnabled(cmd, enabled),
				loopName,
				search,
				cursor,
				limit,
			)
			if err != nil {
				return err
			}

			page, err := client.ListAutomationJobs(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationJobListBundle(page))
		},
	}
	cmd.Flags().StringVar(&scopeRaw, automationScopeKey, "", "Filter by scope: global or workspace")
	cmd.Flags().StringVar(&workspaceRef, "workspace", "", "Filter by workspace path, name, or ID")
	cmd.Flags().
		StringVar(&sourceRaw, automationSourceKey, "", "Filter by definition source: config, package, or dynamic")
	cmd.Flags().BoolVar(
		&enabled,
		automationEnabledKey,
		false,
		"Filter by enabled state; use --enabled=false for disabled jobs",
	)
	cmd.Flags().StringVar(&loopName, "loop", "", "Filter by loop target name")
	cmd.Flags().StringVar(&search, "query", "", "Search jobs by name, agent, prompt, scope, source, or schedule")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue after this automation job cursor")
	cmd.Flags().IntVar(&limit, "limit", automationpkg.DefaultListLimit, "Maximum jobs per page")

	cmd.AddCommand(newAutomationJobsCreateCommand(deps))
	cmd.AddCommand(newAutomationJobsGetCommand(deps))
	cmd.AddCommand(newAutomationJobsUpdateCommand(deps))
	cmd.AddCommand(newAutomationJobsDeleteCommand(deps))
	cmd.AddCommand(newAutomationJobsTriggerCommand(deps))
	cmd.AddCommand(newAutomationJobsHistoryCommand(deps))
	return cmd
}

func newAutomationTriggersCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw     string
		workspaceRef string
		eventRaw     string
		sourceRaw    string
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
				cmd.Context(),
				client,
				scopeRaw,
				workspaceRef,
				eventRaw,
				sourceRaw,
				optionalAutomationEnabled(cmd, enabled),
				loopName,
				search,
				cursor,
				limit,
			)
			if err != nil {
				return err
			}

			page, err := client.ListAutomationTriggers(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationTriggerListBundle(page))
		},
	}
	cmd.Flags().StringVar(&scopeRaw, automationScopeKey, "", "Filter by scope: global or workspace")
	cmd.Flags().StringVar(&workspaceRef, "workspace", "", "Filter by workspace path, name, or ID")
	cmd.Flags().StringVar(&eventRaw, automationEventKey, "", "Filter by activation event")
	cmd.Flags().
		StringVar(&sourceRaw, automationSourceKey, "", "Filter by definition source: config, package, or dynamic")
	cmd.Flags().BoolVar(
		&enabled,
		automationEnabledKey,
		false,
		"Filter by enabled state; use --enabled=false for disabled triggers",
	)
	cmd.Flags().StringVar(&loopName, "loop", "", "Filter by loop target name")
	cmd.Flags().StringVar(&search, "query", "", "Search triggers by definition or filter fields")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue after this automation trigger cursor")
	cmd.Flags().IntVar(&limit, "limit", automationpkg.DefaultListLimit, "Maximum triggers per page")

	cmd.AddCommand(newAutomationTriggersCreateCommand(deps))
	cmd.AddCommand(newAutomationTriggersGetCommand(deps))
	cmd.AddCommand(newAutomationTriggersUpdateCommand(deps))
	cmd.AddCommand(newAutomationTriggersDeleteCommand(deps))
	cmd.AddCommand(newAutomationTriggersHistoryCommand(deps))
	return cmd
}

func optionalAutomationEnabled(cmd *cobra.Command, value bool) *bool {
	if !cmd.Flags().Changed(automationEnabledKey) {
		return nil
	}
	return &value
}
