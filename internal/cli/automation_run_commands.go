package cli

import (
	"context"
	"errors"
	"fmt"

	"strconv"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/spf13/cobra"
)

func newAutomationRunsCommand(deps commandDeps) *cobra.Command {
	var (
		jobID     string
		triggerID string
		statusRaw string
		sinceRaw  string
		untilRaw  string
		last      int
	)

	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Inspect automation run history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseAutomationRunListQuery(statusRaw, sinceRaw, untilRaw, last, deps.now)
			if err != nil {
				return err
			}
			query.JobID = strings.TrimSpace(jobID)
			query.TriggerID = strings.TrimSpace(triggerID)

			runs, err := client.ListAutomationRuns(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationRunListBundle(runs))
		},
	}
	cmd.Flags().StringVar(&jobID, "job-id", "", "Filter by automation job ID")
	cmd.Flags().StringVar(&triggerID, "trigger-id", "", "Filter by automation trigger ID")
	cmd.Flags().StringVar(&statusRaw, automationStatusKey, "", "Filter by run status")
	cmd.Flags().
		StringVar(&sinceRaw, "since", "", "Show runs since an RFC3339 timestamp or relative duration")
	cmd.Flags().
		StringVar(&untilRaw, "until", "", "Show runs until an RFC3339 timestamp or relative duration")
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N runs")
	cmd.AddCommand(newAutomationRunsGetCommand(deps))
	return cmd
}

func newAutomationRunsGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationGetIDValue,
		Short: "Show one automation run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			run, err := client.GetAutomationRun(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationRunBundle(run))
		},
	}
}

func parseAutomationRunListQuery(
	statusRaw string,
	sinceRaw string,
	untilRaw string,
	last int,
	now func() time.Time,
) (AutomationRunQuery, error) {
	query := AutomationRunQuery{}
	if err := validateAutomationLast(last); err != nil {
		return AutomationRunQuery{}, err
	}
	query.Limit = last

	status, err := parseOptionalAutomationRunStatus(statusRaw)
	if err != nil {
		return AutomationRunQuery{}, err
	}
	query.Status = status

	since, err := parseAutomationOptionalTimeFlag(sinceRaw, "since", now)
	if err != nil {
		return AutomationRunQuery{}, err
	}
	query.Since = since

	until, err := parseAutomationOptionalTimeFlag(untilRaw, "until", now)
	if err != nil {
		return AutomationRunQuery{}, err
	}
	query.Until = until
	return query, nil
}

func resolveAutomationScopeWorkspace(
	ctx context.Context,
	client DaemonClient,
	rawScope string,
	workspaceRef string,
) (automationpkg.Scope, string, error) {
	scope, err := parseRequiredAutomationScope(rawScope)
	if err != nil {
		return "", "", err
	}

	trimmedWorkspace := strings.TrimSpace(workspaceRef)
	switch scope {
	case automationpkg.AutomationScopeGlobal:
		if trimmedWorkspace != "" {
			return "", "", errors.New("cli: --workspace must be empty when --scope is global")
		}
		return scope, "", nil
	case automationpkg.AutomationScopeWorkspace:
		if trimmedWorkspace == "" {
			return "", "", errors.New("cli: --workspace is required when --scope is workspace")
		}
		workspaceID, err := resolveAutomationWorkspaceID(ctx, client, trimmedWorkspace)
		if err != nil {
			return "", "", err
		}
		return scope, workspaceID, nil
	default:
		return "", "", fmt.Errorf("cli: unsupported automation scope %q", scope)
	}
}

func resolveAutomationWorkspaceID(
	ctx context.Context,
	client DaemonClient,
	ref string,
) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", nil
	}
	detail, err := client.GetWorkspace(ctx, trimmed)
	if err != nil {
		return "", fmt.Errorf("cli: resolve workspace %q: %w", trimmed, err)
	}
	id := strings.TrimSpace(detail.Workspace.ID)
	if id == "" {
		return "", fmt.Errorf("cli: resolve workspace %q: missing workspace id", trimmed)
	}
	return id, nil
}

func parseRequiredAutomationScope(raw string) (automationpkg.Scope, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("cli: --scope is required")
	}
	return parseOptionalAutomationScope(raw)
}

func parseOptionalAutomationScope(raw string) (automationpkg.Scope, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	scope := automationpkg.Scope(trimmed)
	if err := scope.Validate(automationScopeKey); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return scope, nil
}

func parseOptionalAutomationSource(raw string) (automationpkg.JobSource, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	source := automationpkg.JobSource(trimmed)
	if err := source.Validate(automationSourceKey); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return source, nil
}

func parseOptionalAutomationRunStatus(raw string) (automationpkg.RunStatus, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	status := automationpkg.RunStatus(trimmed)
	if err := status.Validate(automationStatusKey); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return status, nil
}

func parseAutomationRetryFlag(raw string) (*automationpkg.RetryConfig, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	lower := strings.ToLower(trimmed)
	var cfg automationpkg.RetryConfig
	switch {
	case lower == string(automationpkg.RetryStrategyNone):
		cfg = automationpkg.DefaultRetryConfig()
	case lower == string(automationpkg.RetryStrategyBackoff):
		cfg = automationpkg.DefaultBackoffRetryConfig()
	case strings.HasPrefix(lower, "backoff:"):
		payload := strings.TrimSpace(trimmed[len("backoff:"):])
		parts := strings.Split(payload, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf(
				`cli: invalid retry value %q: use "none", "backoff", or "backoff:<max_retries>:<base_delay>"`,
				trimmed,
			)
		}
		maxRetries, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("cli: invalid retry max_retries %q: %w", parts[0], err)
		}
		cfg = automationpkg.RetryConfig{
			Strategy:   automationpkg.RetryStrategyBackoff,
			MaxRetries: maxRetries,
			BaseDelay:  strings.TrimSpace(parts[1]),
		}
	default:
		return nil, fmt.Errorf(
			`cli: invalid retry value %q: use "none", "backoff", or "backoff:<max_retries>:<base_delay>"`,
			trimmed,
		)
	}

	if err := cfg.Validate(automationRetryKey); err != nil {
		return nil, fmt.Errorf("cli: %w", err)
	}
	return &cfg, nil
}

func parseAutomationFilterFlags(flags []string) (map[string]string, error) {
	if len(flags) == 0 {
		return nil, nil
	}

	filter := make(map[string]string)
	for _, rawFlag := range flags {
		for entry := range strings.SplitSeq(rawFlag, ",") {
			trimmedEntry := strings.TrimSpace(entry)
			if trimmedEntry == "" {
				continue
			}
			key, value, ok := strings.Cut(trimmedEntry, "=")
			if !ok {
				return nil, fmt.Errorf("cli: invalid filter %q: use key=value", trimmedEntry)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" {
				return nil, fmt.Errorf("cli: invalid filter %q: key is required", trimmedEntry)
			}
			if value == "" {
				return nil, fmt.Errorf("cli: invalid filter %q: value is required", trimmedEntry)
			}
			filter[key] = value
		}
	}
	if len(filter) == 0 {
		return nil, nil
	}
	if err := automationpkg.ValidateTriggerFilter(filter, "filter"); err != nil {
		return nil, fmt.Errorf("cli: %w", err)
	}
	return filter, nil
}

func parseAutomationOptionalTimeFlag(
	raw string,
	flagName string,
	now func() time.Time,
) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, nil
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("cli: invalid --%s value %q", flagName, value)
	}
	if duration < 0 {
		return time.Time{}, fmt.Errorf("cli: relative --%s must be positive: %q", flagName, value)
	}

	if now != nil {
		return now().UTC().Add(-duration), nil
	}
	return time.Now().UTC().Add(-duration), nil
}

func validateAutomationLast(last int) error {
	if last < 0 {
		return fmt.Errorf("cli: --last must be zero or positive: %d", last)
	}
	return nil
}
