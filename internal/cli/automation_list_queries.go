package cli

import (
	"fmt"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/spf13/cobra"
)

func parseAutomationJobListQuery(
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
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

	if resolution, ok, err := resolveWorkspaceOverrideOnly(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceRef,
	); err != nil {
		return AutomationJobQuery{}, err
	} else if ok {
		query.WorkspaceID = resolution.ID
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
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
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

	if resolution, ok, err := resolveWorkspaceOverrideOnly(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceRef,
	); err != nil {
		return AutomationTriggerQuery{}, err
	} else if ok {
		query.WorkspaceID = resolution.ID
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
