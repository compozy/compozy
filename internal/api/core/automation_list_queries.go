package core

import (
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/gin-gonic/gin"
)

// ParseAutomationJobListQuery parses the shared automation job list filters.
func ParseAutomationJobListQuery(c *gin.Context) (automationpkg.JobListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return automationpkg.JobListQuery{}, err
	}
	enabled, err := parseOptionalBoolPointer(c.Query("enabled"))
	if err != nil {
		return automationpkg.JobListQuery{}, err
	}

	query := automationpkg.JobListQuery{
		WorkspaceID: strings.TrimSpace(c.Query("workspace_id")),
		LoopName:    strings.TrimSpace(c.Query("loop")),
		Enabled:     enabled,
		Search:      strings.TrimSpace(c.Query("q")),
		Cursor:      strings.TrimSpace(c.Query("cursor")),
		Limit:       limit,
	}
	if query.Limit == 0 {
		query.Limit = automationpkg.DefaultListLimit
	}
	if rawScope := strings.TrimSpace(c.Query("scope")); rawScope != "" {
		query.Scope = automationpkg.Scope(rawScope)
		if err := query.Scope.Validate("scope"); err != nil {
			return automationpkg.JobListQuery{}, err
		}
	}
	if rawSource := strings.TrimSpace(c.Query("source")); rawSource != "" {
		query.Source = automationpkg.JobSource(rawSource)
		if err := query.Source.Validate("source"); err != nil {
			return automationpkg.JobListQuery{}, err
		}
	}
	if err := automationpkg.ValidateJobListQuery(query); err != nil {
		return automationpkg.JobListQuery{}, err
	}
	return query, nil
}

// ParseAutomationTriggerListQuery parses the shared automation trigger list filters.
func ParseAutomationTriggerListQuery(c *gin.Context) (automationpkg.TriggerListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return automationpkg.TriggerListQuery{}, err
	}
	enabled, err := parseOptionalBoolPointer(c.Query("enabled"))
	if err != nil {
		return automationpkg.TriggerListQuery{}, err
	}

	query := automationpkg.TriggerListQuery{
		WorkspaceID: strings.TrimSpace(c.Query("workspace_id")),
		Event:       strings.TrimSpace(c.Query("event")),
		LoopName:    strings.TrimSpace(c.Query("loop")),
		Enabled:     enabled,
		Search:      strings.TrimSpace(c.Query("q")),
		Cursor:      strings.TrimSpace(c.Query("cursor")),
		Limit:       limit,
	}
	if query.Limit == 0 {
		query.Limit = automationpkg.DefaultListLimit
	}
	if rawScope := strings.TrimSpace(c.Query("scope")); rawScope != "" {
		query.Scope = automationpkg.Scope(rawScope)
		if err := query.Scope.Validate("scope"); err != nil {
			return automationpkg.TriggerListQuery{}, err
		}
	}
	if rawSource := strings.TrimSpace(c.Query("source")); rawSource != "" {
		query.Source = automationpkg.JobSource(rawSource)
		if err := query.Source.Validate("source"); err != nil {
			return automationpkg.TriggerListQuery{}, err
		}
	}
	if err := automationpkg.ValidateTriggerListQuery(query); err != nil {
		return automationpkg.TriggerListQuery{}, err
	}
	return query, nil
}
