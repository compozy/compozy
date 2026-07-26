package cli

import (
	"context"
	"fmt"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
)

func parseAutomationJobListQuery(
	ctx context.Context,
	client DaemonClient,
	scopeRaw string,
	workspaceRef string,
	sourceRaw string,
	enabled *bool,
	loopName string,
	search string,
	cursor string,
	limit int,
) (AutomationJobQuery, error) {
	query := AutomationJobQuery{}
	if err := validateAutomationListLimit(limit); err != nil {
		return AutomationJobQuery{}, err
	}
	query.Limit = limit

	scope, err := parseOptionalAutomationScope(scopeRaw)
	if err != nil {
		return AutomationJobQuery{}, err
	}
	query.Scope = scope

	if trimmed := strings.TrimSpace(workspaceRef); trimmed != "" {
		workspaceID, err := resolveAutomationWorkspaceID(ctx, client, trimmed)
		if err != nil {
			return AutomationJobQuery{}, err
		}
		query.WorkspaceID = workspaceID
	}

	source, err := parseOptionalAutomationSource(sourceRaw)
	if err != nil {
		return AutomationJobQuery{}, err
	}
	query.Source = source
	query.Enabled = enabled
	query.LoopName = strings.TrimSpace(loopName)
	query.Search = strings.TrimSpace(search)
	query.Cursor = strings.TrimSpace(cursor)
	if err := automationpkg.ValidateJobListQuery(query); err != nil {
		return AutomationJobQuery{}, fmt.Errorf("cli: invalid automation job list query: %w", err)
	}
	return query, nil
}

func parseAutomationTriggerListQuery(
	ctx context.Context,
	client DaemonClient,
	scopeRaw string,
	workspaceRef string,
	eventRaw string,
	sourceRaw string,
	enabled *bool,
	loopName string,
	search string,
	cursor string,
	limit int,
) (AutomationTriggerQuery, error) {
	query := AutomationTriggerQuery{
		Event: strings.TrimSpace(eventRaw),
	}
	if err := validateAutomationListLimit(limit); err != nil {
		return AutomationTriggerQuery{}, err
	}
	query.Limit = limit

	scope, err := parseOptionalAutomationScope(scopeRaw)
	if err != nil {
		return AutomationTriggerQuery{}, err
	}
	query.Scope = scope

	if trimmed := strings.TrimSpace(workspaceRef); trimmed != "" {
		workspaceID, err := resolveAutomationWorkspaceID(ctx, client, trimmed)
		if err != nil {
			return AutomationTriggerQuery{}, err
		}
		query.WorkspaceID = workspaceID
	}

	source, err := parseOptionalAutomationSource(sourceRaw)
	if err != nil {
		return AutomationTriggerQuery{}, err
	}
	query.Source = source
	query.Enabled = enabled
	query.LoopName = strings.TrimSpace(loopName)
	query.Search = strings.TrimSpace(search)
	query.Cursor = strings.TrimSpace(cursor)
	if err := automationpkg.ValidateTriggerListQuery(query); err != nil {
		return AutomationTriggerQuery{}, fmt.Errorf("cli: invalid automation trigger list query: %w", err)
	}
	return query, nil
}

func validateAutomationListLimit(limit int) error {
	if limit < 0 || limit > automationpkg.MaxListLimit {
		return fmt.Errorf(
			"cli: --limit must be between 0 and %d: %d",
			automationpkg.MaxListLimit,
			limit,
		)
	}
	return nil
}
