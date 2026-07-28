package cli

import "github.com/spf13/cobra"

func newAutomationTriggersCreateCommand(deps commandDeps) *cobra.Command {
	var (
		name               string
		scopeRaw           string
		eventRaw           string
		workspaceRef       string
		agentName          string
		prompt             string
		retryRaw           string
		filterFlags        []string
		enabled            bool
		webhookID          string
		endpointSlug       string
		webhookSecretValue string
	)

	cmd := &cobra.Command{
		Use:   automationCreateKey,
		Short: "Create an automation trigger",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request, err := buildAutomationTriggerCreateRequest(
				cmd,
				client,
				automationTriggerCommandInput{
					Name:               name,
					ScopeRaw:           scopeRaw,
					EventRaw:           eventRaw,
					WorkspaceRef:       workspaceRef,
					AgentName:          agentName,
					Prompt:             prompt,
					RetryRaw:           retryRaw,
					FilterFlags:        filterFlags,
					Enabled:            enabled,
					WebhookID:          webhookID,
					EndpointSlug:       endpointSlug,
					WebhookSecretValue: webhookSecretValue,
				},
			)
			if err != nil {
				return err
			}

			created, err := client.CreateAutomationTrigger(cmd.Context(), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationTriggerBundle(created))
		},
	}
	bindAutomationTriggerCreateFlags(
		cmd,
		&name,
		&scopeRaw,
		&eventRaw,
		&workspaceRef,
		&agentName,
		&prompt,
		&retryRaw,
		&filterFlags,
		&enabled,
		&webhookID,
		&endpointSlug,
		&webhookSecretValue,
	)
	return cmd
}

func bindAutomationTriggerCreateFlags(
	cmd *cobra.Command,
	name *string,
	scopeRaw *string,
	eventRaw *string,
	workspaceRef *string,
	agentName *string,
	prompt *string,
	retryRaw *string,
	filterFlags *[]string,
	enabled *bool,
	webhookID *string,
	endpointSlug *string,
	webhookSecretValue *string,
) {
	cmd.Flags().StringVar(name, automationNameKey, "", "Trigger name")
	cmd.Flags().StringVar(scopeRaw, automationScopeKey, "", "Trigger scope: global or workspace")
	cmd.Flags().StringVar(eventRaw, automationEventKey, "", "Trigger event name")
	cmd.Flags().
		StringVar(workspaceRef, "workspace", "", "Workspace path, name, or ID (required when --scope=workspace)")
	cmd.Flags().StringVar(agentName, "agent", "", "Agent definition name")
	cmd.Flags().StringVar(prompt, automationPromptKey, "", "Prompt template body")
	cmd.Flags().
		StringArrayVar(filterFlags, "filter", nil, "Exact-match filter(s): key=value or comma-separated key=value pairs")
	cmd.Flags().
		StringVar(retryRaw, automationRetryKey, "", `Retry policy: "none", "backoff", or "backoff:<max_retries>:<base_delay>"`)
	cmd.Flags().BoolVar(enabled, automationEnabledKey, false, "Create the trigger enabled or disabled")
	cmd.Flags().StringVar(webhookID, "webhook-id", "", "Stable webhook identifier override for webhook events")
	cmd.Flags().StringVar(endpointSlug, "endpoint-slug", "", "Public endpoint slug for webhook events")
	cmd.Flags().StringVar(webhookSecretValue, "webhook-secret-value", "", "Write-only webhook secret value")
	mustMarkFlagRequired(cmd, automationNameKey)
	mustMarkFlagRequired(cmd, automationScopeKey)
	mustMarkFlagRequired(cmd, automationEventKey)
	mustMarkFlagRequired(cmd, "agent")
	mustMarkFlagRequired(cmd, automationPromptKey)
}

func newAutomationTriggersGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationGetIDValue,
		Short: "Show one automation trigger",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			trigger, err := client.GetAutomationTrigger(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationTriggerBundle(trigger))
		},
	}
}

func newAutomationTriggersUpdateCommand(deps commandDeps) *cobra.Command {
	var (
		name               string
		prompt             string
		eventRaw           string
		filterFlags        []string
		retryRaw           string
		enabled            bool
		webhookID          string
		endpointSlug       string
		webhookSecretValue string
	)

	cmd := &cobra.Command{
		Use:   automationUpdateIDValue,
		Short: "Update an automation trigger",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request, err := buildAutomationTriggerUpdateRequest(
				cmd,
				client,
				automationTriggerCommandInput{
					Name:               name,
					EventRaw:           eventRaw,
					Prompt:             prompt,
					RetryRaw:           retryRaw,
					FilterFlags:        filterFlags,
					Enabled:            enabled,
					WebhookID:          webhookID,
					EndpointSlug:       endpointSlug,
					WebhookSecretValue: webhookSecretValue,
				},
			)
			if err != nil {
				return err
			}

			updated, err := client.UpdateAutomationTrigger(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationTriggerBundle(updated))
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "Update the trigger name")
	cmd.Flags().StringVar(&prompt, automationPromptKey, "", "Update the prompt template body")
	cmd.Flags().StringVar(&eventRaw, automationEventKey, "", "Update the trigger event")
	cmd.Flags().
		StringArrayVar(&filterFlags, "filter", nil, "Replace filters with key=value entries")
	cmd.Flags().
		StringVar(&retryRaw, automationRetryKey, "", `Update retry policy: "none", "backoff", or "backoff:<max_retries>:<base_delay>"`)
	cmd.Flags().BoolVar(&enabled, automationEnabledKey, false, "Update the enabled state")
	cmd.Flags().StringVar(&webhookID, "webhook-id", "", "Update the stable webhook identifier")
	cmd.Flags().StringVar(&endpointSlug, "endpoint-slug", "", "Update the webhook endpoint slug")
	cmd.Flags().
		StringVar(&webhookSecretValue, "webhook-secret-value", "", "Write-only webhook secret value")
	return cmd
}

func newAutomationTriggersDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationDeleteIDValue,
		Short: "Delete an automation trigger",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			current, err := client.GetAutomationTrigger(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := client.DeleteAutomationTrigger(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationTriggerBundle(current))
		},
	}
}

func newAutomationTriggersHistoryCommand(deps commandDeps) *cobra.Command {
	var (
		statusRaw string
		sinceRaw  string
		untilRaw  string
		last      int
	)

	cmd := &cobra.Command{
		Use:   automationHistoryIDValue,
		Short: "Show run history for one automation trigger",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseAutomationRunListQuery(statusRaw, sinceRaw, untilRaw, last, deps.now)
			if err != nil {
				return err
			}

			runs, err := client.AutomationTriggerRuns(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationRunListBundle(runs))
		},
	}
	cmd.Flags().StringVar(&statusRaw, automationStatusKey, "", "Filter by run status")
	cmd.Flags().
		StringVar(&sinceRaw, "since", "", "Show runs since an RFC3339 timestamp or relative duration")
	cmd.Flags().
		StringVar(&untilRaw, "until", "", "Show runs until an RFC3339 timestamp or relative duration")
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N runs")
	return cmd
}
