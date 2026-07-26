// Package usage owns the shared public Network usage query and response projection.
package usage

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

var (
	ErrUnknownField = errors.New("unknown_field")
	queryFields     = map[string]struct{}{
		"channel":    {},
		"cursor":     {},
		"limit":      {},
		"owner_id":   {},
		"owner_kind": {},
		"run_id":     {},
	}
)

// ParseQuery strictly parses the shared public usage filter contract.
func ParseQuery(workspaceID string, values url.Values) (store.NetworkUsageQuery, error) {
	for field, fieldValues := range values {
		if _, ok := queryFields[field]; !ok {
			return store.NetworkUsageQuery{}, fmt.Errorf("%w: %s", ErrUnknownField, field)
		}
		if len(fieldValues) != 1 {
			return store.NetworkUsageQuery{}, fmt.Errorf(
				"%w: query field %s must appear once",
				store.ErrNetworkUsageQueryInvalid,
				field,
			)
		}
	}
	query := store.NetworkUsageQuery{
		WorkspaceID: strings.TrimSpace(workspaceID),
		Channel:     strings.TrimSpace(values.Get("channel")),
		RunID:       strings.TrimSpace(values.Get("run_id")),
		Cursor:      strings.TrimSpace(values.Get("cursor")),
	}
	ownerKind := strings.TrimSpace(values.Get("owner_kind"))
	ownerID := strings.TrimSpace(values.Get("owner_id"))
	if ownerKind != "" || ownerID != "" {
		query.Owner = &participation.OwnerRef{
			WorkspaceID: query.WorkspaceID,
			Kind:        participation.OwnerKind(ownerKind),
			ID:          ownerID,
		}
	}
	if rawLimit, exists := values["limit"]; exists {
		query.LimitSet = true
		limit, err := strconv.Atoi(strings.TrimSpace(rawLimit[0]))
		if err != nil {
			return store.NetworkUsageQuery{}, fmt.Errorf(
				"%w: limit must be an integer",
				store.ErrNetworkUsageLimitInvalid,
			)
		}
		query.Limit = limit
	}
	normalized, _, err := store.NormalizeNetworkUsageQuery(query)
	if err != nil {
		return store.NetworkUsageQuery{}, err
	}
	return normalized, nil
}

// ResponseFromReport validates JSON-safe counters and projects the shared response.
func ResponseFromReport(
	workspaceID string,
	report store.NetworkUsageReport,
) (contract.NetworkUsageResponse, error) {
	if err := validateReportIntegers(report); err != nil {
		return contract.NetworkUsageResponse{}, err
	}
	details := make([]contract.NetworkUsageDetailPayload, 0, len(report.Details))
	for _, detail := range report.Details {
		owner, err := participation.OwnerRefFromKey(detail.WorkspaceID, detail.OwnerKey)
		if err != nil {
			return contract.NetworkUsageResponse{}, fmt.Errorf("network usage: project detail owner: %w", err)
		}
		details = append(details, contract.NetworkUsageDetailPayload{
			WakeID:    detail.WakeID,
			TaskRunID: detail.TaskRunID,
			ParticipationStatus: participation.Status{
				Owner:         owner,
				Available:     true,
				Participating: true,
			},
			WorkspaceID:     detail.WorkspaceID,
			Channel:         detail.Channel,
			RootID:          detail.RootID,
			Depth:           detail.Depth,
			State:           detail.State,
			UsageState:      detail.UsageState,
			ChargedWallTime: detail.ChargedWallTime.String(),
			InputTokens:     detail.InputTokens,
			OutputTokens:    detail.OutputTokens,
			ReservedAt:      detail.ReservedAt,
			SettledAt:       cloneOptionalTime(detail.SettledAt),
			Reason:          detail.Reason,
		})
	}
	response := contract.NetworkUsageResponse{
		WorkspaceID: strings.TrimSpace(workspaceID),
		Details:     details,
		Total: contract.NetworkUsageSummaryPayload{
			WakeCount:            report.Total.WakeCount,
			ReservedWakeCount:    report.Total.ReservedWakeCount,
			ActualWakeCount:      report.Total.ActualWakeCount,
			UnavailableWakeCount: report.Total.UnavailableWakeCount,
			ChargedWallTime:      report.Total.ChargedWallTime.String(),
			InputTokens:          report.Total.InputTokens,
			OutputTokens:         report.Total.OutputTokens,
		},
		NextCursor: report.NextCursor,
	}
	if report.Budget != nil {
		owner, err := participation.OwnerRefFromKey(workspaceID, report.Budget.OwnerKey)
		if err != nil {
			return contract.NetworkUsageResponse{}, fmt.Errorf("network usage: project budget owner: %w", err)
		}
		response.Budget = &contract.NetworkBudgetUsagePayload{
			ParticipationStatus: participation.Status{
				Owner:         owner,
				Available:     true,
				Participating: true,
			},
			WakesUsed:        report.Budget.WakesUsed,
			WallTimeUsed:     report.Budget.WallTimeUsed.String(),
			InputTokensUsed:  report.Budget.InputTokensUsed,
			OutputTokensUsed: report.Budget.OutputTokensUsed,
			ExhaustedReason:  report.Budget.ExhaustedReason,
			UpdatedAt:        report.Budget.UpdatedAt,
		}
	}
	return response, nil
}

func validateReportIntegers(report store.NetworkUsageReport) error {
	for _, detail := range report.Details {
		if err := store.ValidateJavaScriptSafeInteger("details.input_tokens", detail.InputTokens); err != nil {
			return err
		}
		if err := store.ValidateJavaScriptSafeInteger("details.output_tokens", detail.OutputTokens); err != nil {
			return err
		}
	}
	if err := store.ValidateJavaScriptSafeInteger("total.input_tokens", report.Total.InputTokens); err != nil {
		return err
	}
	if err := store.ValidateJavaScriptSafeInteger("total.output_tokens", report.Total.OutputTokens); err != nil {
		return err
	}
	if report.Budget == nil {
		return nil
	}
	if err := store.ValidateJavaScriptSafeInteger(
		"budget.input_tokens_used",
		report.Budget.InputTokensUsed,
	); err != nil {
		return err
	}
	return store.ValidateJavaScriptSafeInteger("budget.output_tokens_used", report.Budget.OutputTokensUsed)
}

func cloneOptionalTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
