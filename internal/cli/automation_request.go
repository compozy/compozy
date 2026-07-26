package cli

import (
	"errors"
	"fmt"

	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/spf13/cobra"
)

func buildAutomationTriggerCreateRequest(
	cmd *cobra.Command,
	client DaemonClient,
	input automationTriggerCommandInput,
) (AutomationTriggerCreateRequest, error) {
	scope, workspaceID, err := resolveAutomationScopeWorkspace(
		cmd.Context(),
		client,
		input.ScopeRaw,
		input.WorkspaceRef,
	)
	if err != nil {
		return AutomationTriggerCreateRequest{}, err
	}
	retry, err := parseAutomationRetryFlag(input.RetryRaw)
	if err != nil {
		return AutomationTriggerCreateRequest{}, err
	}
	filter, err := parseAutomationFilterFlags(input.FilterFlags)
	if err != nil {
		return AutomationTriggerCreateRequest{}, err
	}

	request := AutomationTriggerCreateRequest{
		Scope:              scope,
		Name:               strings.TrimSpace(input.Name),
		AgentName:          strings.TrimSpace(input.AgentName),
		WorkspaceID:        workspaceID,
		Prompt:             strings.TrimSpace(input.Prompt),
		Event:              strings.TrimSpace(input.EventRaw),
		Filter:             filter,
		WebhookID:          strings.TrimSpace(input.WebhookID),
		EndpointSlug:       strings.TrimSpace(input.EndpointSlug),
		WebhookSecretValue: input.WebhookSecretValue,
	}
	if cmd.Flags().Changed(automationEnabledKey) {
		request.Enabled = new(input.Enabled)
	}
	if retry != nil {
		request.Retry = retry
	}
	return request, nil
}

func buildAutomationTriggerUpdateRequest(
	cmd *cobra.Command,
	_ DaemonClient,
	input automationTriggerCommandInput,
) (AutomationTriggerUpdateRequest, error) {
	request := AutomationTriggerUpdateRequest{}
	if cmd.Flags().Changed(automationNameKey) {
		request.Name = new(strings.TrimSpace(input.Name))
	}
	if cmd.Flags().Changed(automationPromptKey) {
		request.Prompt = new(strings.TrimSpace(input.Prompt))
	}
	if cmd.Flags().Changed(automationEventKey) {
		request.Event = new(strings.TrimSpace(input.EventRaw))
	}
	if cmd.Flags().Changed("filter") {
		filter, err := parseAutomationFilterFlags(input.FilterFlags)
		if err != nil {
			return AutomationTriggerUpdateRequest{}, err
		}
		request.Filter = filter
	}
	if cmd.Flags().Changed(automationRetryKey) {
		retry, err := parseAutomationRetryFlag(input.RetryRaw)
		if err != nil {
			return AutomationTriggerUpdateRequest{}, err
		}
		if retry == nil {
			return AutomationTriggerUpdateRequest{}, errors.New(
				`cli: --retry requires "none", "backoff", or "backoff:<max_retries>:<base_delay>"`,
			)
		}
		request.Retry = retry
	}
	if cmd.Flags().Changed(automationEnabledKey) {
		request.Enabled = new(input.Enabled)
	}
	if cmd.Flags().Changed("webhook-id") {
		request.WebhookID = new(strings.TrimSpace(input.WebhookID))
	}
	if cmd.Flags().Changed("endpoint-slug") {
		request.EndpointSlug = new(strings.TrimSpace(input.EndpointSlug))
	}
	if cmd.Flags().Changed("webhook-secret-value") {
		request.WebhookSecretValue = new(input.WebhookSecretValue)
	}
	if !request.HasChanges() {
		return AutomationTriggerUpdateRequest{}, errors.New(
			"cli: automation trigger update requires at least one change flag",
		)
	}
	return request, nil
}

func formatAutomationSchedule(spec *automationpkg.ScheduleSpec) string {
	if spec == nil {
		return ""
	}
	switch spec.Mode {
	case automationpkg.ScheduleModeEvery:
		return "every:" + strings.TrimSpace(spec.Interval)
	case automationpkg.ScheduleModeAt:
		return "at:" + strings.TrimSpace(spec.Time)
	default:
		return "cron:" + strings.TrimSpace(spec.Expr)
	}
}

func formatAutomationRetry(cfg automationpkg.RetryConfig) string {
	switch cfg.Strategy {
	case automationpkg.RetryStrategyBackoff:
		return fmt.Sprintf("backoff:%d:%s", cfg.MaxRetries, strings.TrimSpace(cfg.BaseDelay))
	default:
		return string(automationpkg.RetryStrategyNone)
	}
}

func formatAutomationFireLimit(cfg automationpkg.FireLimitConfig) string {
	return fmt.Sprintf("%d/%s", cfg.Max, strings.TrimSpace(cfg.Window))
}

func displayTriggerEndpoint(item TriggerRecord) string {
	if !strings.EqualFold(strings.TrimSpace(item.Event), "webhook") {
		return ""
	}
	endpoint, err := automationpkg.FormatWebhookEndpoint(item.EndpointSlug, item.WebhookID)
	if err != nil {
		return ""
	}
	if item.Scope == automationpkg.AutomationScopeWorkspace &&
		strings.TrimSpace(item.WorkspaceID) != "" {
		return "/api/webhooks/workspaces/" + strings.TrimSpace(item.WorkspaceID) + "/" + endpoint
	}
	return "/api/webhooks/global/" + endpoint
}

func displayRunTarget(item RunRecord) string {
	switch {
	case strings.TrimSpace(item.JobID) != "":
		return "job:" + strings.TrimSpace(item.JobID)
	case strings.TrimSpace(item.TriggerID) != "":
		return "trigger:" + strings.TrimSpace(item.TriggerID)
	default:
		return ""
	}
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}
