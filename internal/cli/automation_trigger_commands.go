package cli

import "github.com/spf13/cobra"

const automationTriggerEventHelp = "Trigger event: session.created, session.stopped, memory.consolidated, " +
	"hook.<hook_name>.completed, webhook, or ext.*"

func newAutomationTriggersCreateCommand(deps commandDeps) *cobra.Command {
	input := automationTriggerCommandInput{}

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
				deps,
				client,
				input,
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
	bindAutomationTriggerCreateFlags(cmd, deps, &input)
	return cmd
}

func bindAutomationTriggerCreateFlags(
	cmd *cobra.Command,
	deps commandDeps,
	input *automationTriggerCommandInput,
) {
	cmd.Flags().StringVar(&input.Name, automationNameKey, "", "Trigger name")
	cmd.Flags().StringVar(&input.ScopeRaw, automationScopeKey, "", "Trigger scope: global or workspace")
	cmd.Flags().StringVar(
		&input.EventRaw,
		automationEventKey,
		"",
		automationTriggerEventHelp,
	)
	cmd.Flags().
		StringVar(&input.WorkspaceRef, "workspace", "", "Override workspace binding (ID, name, or path)")
	bindAutomationCreateTargetFlags(cmd, &input.automationCreateTargetInput, true)
	cmd.Flags().StringArrayVar(
		&input.FilterFlags,
		"filter",
		nil,
		"Exact-match filter(s): key=value or comma-separated key=value pairs",
	)
	cmd.Flags().
		StringVar(&input.RetryRaw, automationRetryKey, "", `Retry policy: "none", "backoff", or "backoff:<max_retries>:<base_delay>"`)
	cmd.Flags().BoolVar(&input.Enabled, automationEnabledKey, false, "Create the trigger enabled or disabled")
	cmd.Flags().StringVar(&input.WebhookID, "webhook-id", "", "Stable webhook identifier override for webhook events")
	cmd.Flags().StringVar(&input.EndpointSlug, "endpoint-slug", "", "Public endpoint slug for webhook events")
	cmd.Flags().StringVar(&input.WebhookSecretValue, "webhook-secret-value", "", "Write-only webhook secret value")
	mustMarkFlagRequired(cmd, automationNameKey)
	mustMarkFlagRequired(cmd, automationScopeKey)
	mustMarkFlagRequired(cmd, automationEventKey)
	configureProfileMutationCommand(cmd, deps)
}

func newAutomationTriggersGetCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
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
	configureProfileReadCommand(cmd, deps)
	return cmd
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
				automationTriggerCommandInput{
					automationCreateTargetInput: automationCreateTargetInput{Prompt: prompt},
					Name:                        name,
					EventRaw:                    eventRaw,
					RetryRaw:                    retryRaw,
					FilterFlags:                 filterFlags,
					Enabled:                     enabled,
					WebhookID:                   webhookID,
					EndpointSlug:                endpointSlug,
					WebhookSecretValue:          webhookSecretValue,
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
	cmd.Flags().StringVar(&eventRaw, automationEventKey, "", automationTriggerEventHelp)
	cmd.Flags().
		StringArrayVar(&filterFlags, "filter", nil, "Replace filters with key=value entries")
	cmd.Flags().
		StringVar(&retryRaw, automationRetryKey, "", `Update retry policy: "none", "backoff", or "backoff:<max_retries>:<base_delay>"`)
	cmd.Flags().BoolVar(&enabled, automationEnabledKey, false, "Update the enabled state")
	cmd.Flags().StringVar(&webhookID, "webhook-id", "", "Update the stable webhook identifier")
	cmd.Flags().StringVar(&endpointSlug, "endpoint-slug", "", "Update the webhook endpoint slug")
	cmd.Flags().
		StringVar(&webhookSecretValue, "webhook-secret-value", "", "Write-only webhook secret value")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newAutomationTriggersDeleteCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
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
	configureProfileMutationCommand(cmd, deps)
	return cmd
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
	configureProfileReadCommand(cmd, deps)
	return cmd
}
