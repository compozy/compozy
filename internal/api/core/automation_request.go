package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"strconv"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/gin-gonic/gin"
)

// ParseAutomationRunQuery parses the shared automation run list filters.
func ParseAutomationRunQuery(c *gin.Context) (automationpkg.RunQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return automationpkg.RunQuery{}, err
	}
	since, err := ParseOptionalTime(c.Query("since"))
	if err != nil {
		return automationpkg.RunQuery{}, err
	}
	until, err := ParseOptionalTime(c.Query("until"))
	if err != nil {
		return automationpkg.RunQuery{}, err
	}

	query := automationpkg.RunQuery{
		JobID:     strings.TrimSpace(c.Query("job_id")),
		TriggerID: strings.TrimSpace(c.Query("trigger_id")),
		Since:     since,
		Until:     until,
		Limit:     limit,
	}
	if rawStatus := strings.TrimSpace(c.Query("status")); rawStatus != "" {
		query.Status = automationpkg.RunStatus(rawStatus)
		if err := query.Status.Validate("status"); err != nil {
			return automationpkg.RunQuery{}, err
		}
	}
	return query, nil
}

func webhookRequestFromHTTP(c *gin.Context, scope automationpkg.Scope) (automationpkg.WebhookRequest, error) {
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	if err := scope.Validate("scope"); err != nil {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(err)
	}
	if err := automationpkg.ValidateScopeBinding(scope, workspaceID, "webhook", "workspace_id"); err != nil {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(err)
	}

	endpoint := strings.TrimSpace(c.Param("endpoint"))
	if _, err := automationpkg.ParseWebhookEndpoint(endpoint); err != nil {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(err)
	}

	timestamp, err := parseWebhookTimestampHeader(c.GetHeader(WebhookTimestampHeader))
	if err != nil {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(err)
	}

	signature := strings.TrimSpace(c.GetHeader(WebhookSignatureHeader))
	if signature == "" {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(
			errors.New("webhook signature header is required"),
		)
	}
	deliveryID := strings.TrimSpace(c.GetHeader(WebhookDeliveryIDHeader))
	if deliveryID == "" {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(
			errors.New("webhook delivery id header is required"),
		)
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookPayloadSize)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		if maxBytesErr, ok := errors.AsType[*http.MaxBytesError](err); ok && maxBytesErr != nil {
			return automationpkg.WebhookRequest{}, NewAutomationValidationError(
				fmt.Errorf("webhook request body exceeds %d bytes: %w", maxWebhookPayloadSize, err),
			)
		}
		return automationpkg.WebhookRequest{}, fmt.Errorf("%s: read webhook request body: %w", c.FullPath(), err)
	}

	request := automationpkg.WebhookRequest{
		Scope:       scope,
		WorkspaceID: workspaceID,
		Endpoint:    endpoint,
		DeliveryID:  deliveryID,
		Timestamp:   timestamp,
		Signature:   signature,
		Payload:     payload,
		Data:        decodeWebhookPayloadData(payload),
	}
	if err := request.Validate("webhook"); err != nil {
		return automationpkg.WebhookRequest{}, NewAutomationValidationError(err)
	}
	return request, nil
}

func parseWebhookTimestampHeader(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, errors.New("webhook timestamp header is required")
	}

	if parsed, err := ParseOptionalTime(value); err == nil {
		return parsed, nil
	}

	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid webhook timestamp %q", value)
	}
	return time.Unix(seconds, 0).UTC(), nil
}

func decodeWebhookPayloadData(payload []byte) map[string]any {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil
	}

	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	return data
}
