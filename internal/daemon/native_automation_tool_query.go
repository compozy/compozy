package daemon

import (
	"fmt"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type automationJobHistoryInput struct {
	JobID string `json:"job_id"`
	automationRunQueryInput
}

type automationTriggerHistoryInput struct {
	TriggerID string `json:"trigger_id"`
	automationRunQueryInput
}

type automationRunQueryInput struct {
	JobID     string `json:"job_id,omitempty"`
	TriggerID string `json:"trigger_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Since     string `json:"since,omitempty"`
	Until     string `json:"until,omitempty"`
	Limit     int    `json:"limit,omitempty"`
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
