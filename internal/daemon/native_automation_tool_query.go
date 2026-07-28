package daemon

import (
	"errors"
	"fmt"
	"strings"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type automationJobHistoryInput struct {
	automationRunQueryInput
}

type automationTriggerHistoryInput struct {
	automationRunQueryInput
}

type automationRunQueryInput struct {
	JobID       string `json:"job_id,omitempty"`
	TriggerID   string `json:"trigger_id,omitempty"`
	WorkspaceID string `json:"workspace,omitempty"`
	Status      string `json:"status,omitempty"`
	Since       string `json:"since,omitempty"`
	Until       string `json:"until,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (i automationRunQueryInput) query(id toolspkg.ToolID) (automationpkg.RunQuery, error) {
	since, err := parseNativeAutomationOptionalRFC3339(id, "since", i.Since)
	if err != nil {
		return automationpkg.RunQuery{}, err
	}
	until, err := parseNativeAutomationOptionalRFC3339(id, "until", i.Until)
	if err != nil {
		return automationpkg.RunQuery{}, err
	}
	query := automationpkg.RunQuery{
		JobID:     strings.TrimSpace(i.JobID),
		TriggerID: strings.TrimSpace(i.TriggerID),
		Since:     since,
		Until:     until,
		Limit:     i.Limit,
	}
	if query.Limit < 0 {
		return automationpkg.RunQuery{}, nativeAutomationValidationError(
			id,
			fmt.Errorf("run limit must be zero or positive: %d", query.Limit),
		)
	}
	if !query.Until.IsZero() && !query.Since.IsZero() && query.Until.Before(query.Since) {
		return automationpkg.RunQuery{}, nativeAutomationValidationError(
			id,
			errors.New("run query until must not be before since"),
		)
	}
	if rawStatus := strings.TrimSpace(i.Status); rawStatus != "" {
		query.Status = automationpkg.RunStatus(rawStatus)
		if err := query.Status.Validate("status"); err != nil {
			return automationpkg.RunQuery{}, nativeAutomationValidationError(id, err)
		}
	}
	return query, nil
}

func parseNativeAutomationOptionalRFC3339(
	id toolspkg.ToolID,
	field string,
	raw string,
) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, nil
	}
	timestamp, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, nativeAutomationValidationError(
			id,
			fmt.Errorf("%s must be an RFC3339 timestamp: %w", field, err),
		)
	}
	return timestamp, nil
}
