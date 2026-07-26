package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

const (
	transportParityPermissionKey = "permission"
)

const transportParityEventAgentMessage = "agent_message"

// TransportClients exposes the public transport clients derived from one shared
// runtime harness. Tests use this to keep HTTP, UDS, and CLI reads pointed at
// the same daemon instance.
type TransportClients struct {
	HTTPBaseURL string
	HTTPClient  *http.Client

	UDSBaseURL string
	UDSClient  *http.Client

	CLI *CLIClient
}

// TransportClients returns HTTP, UDS, and CLI transport clients for the
// current shared runtime harness.
func (h *RuntimeHarness) TransportClients() (TransportClients, error) {
	if h == nil {
		return TransportClients{}, errors.New("runtime harness is required")
	}

	if strings.TrimSpace(h.Config.Daemon.Socket) == "" {
		return TransportClients{}, errors.New("runtime harness daemon socket path is required")
	}

	return TransportClients{
		HTTPBaseURL: h.HTTPBaseURL,
		HTTPClient:  h.HTTPClient,
		UDSBaseURL:  h.UDSBaseURL,
		UDSClient:   h.UDSClient,
		CLI:         h.CLI,
	}, nil
}

// RunProjectionSummary keeps only the projection fields transport parity tests
// care about, so they do not duplicate daemon-truth assertions already covered
// by the composition-root runtime lane.
type RunProjectionSummary struct {
	ID        string
	TriggerID string
	SessionID string
	Status    automationpkg.RunStatus
}

// PermissionStreamPayload is the narrow streamed approval payload used by
// transport parity tests.
type PermissionStreamPayload struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision,omitempty"`
}

// SummarizeRunProjection extracts the narrow automation-run projection that
// should stay stable across HTTP, UDS, and CLI reads.
func SummarizeRunProjection(run aghcontract.RunPayload) RunProjectionSummary {
	return RunProjectionSummary{
		ID:        strings.TrimSpace(run.ID),
		TriggerID: strings.TrimSpace(run.TriggerID),
		SessionID: strings.TrimSpace(run.SessionID),
		Status:    run.Status,
	}
}

// ValidateWebhookRunProjection confirms that transport-specific reads agree
// with the run returned by HTTP webhook ingress, while staying intentionally
// narrow about which fields define parity.
func ValidateWebhookRunProjection(
	delivery aghcontract.WebhookDeliveryPayload,
	projections ...aghcontract.RunPayload,
) error {
	if delivery.Matched != 1 {
		return fmt.Errorf("webhook delivery matched %d runs, want 1", delivery.Matched)
	}
	if len(delivery.Runs) != 1 {
		return fmt.Errorf("webhook delivery returned %d runs, want 1", len(delivery.Runs))
	}

	expected := SummarizeRunProjection(delivery.Runs[0])
	if expected.ID == "" {
		return errors.New("webhook delivery run id is required")
	}

	for idx, projection := range projections {
		got := SummarizeRunProjection(projection)
		if got != expected {
			return fmt.Errorf("run projection %d = %#v, want %#v", idx, got, expected)
		}
	}
	return nil
}

// PermissionPayloadFromSSE extracts one approval payload from a streamed SSE
// record when present.
func PermissionPayloadFromSSE(record SSEEvent) (PermissionStreamPayload, bool) {
	if semanticSSEEvent(record) != transportParityPermissionKey || len(record.Data) == 0 {
		return PermissionStreamPayload{}, false
	}

	var envelope struct {
		Type string                  `json:"type"`
		Data PermissionStreamPayload `json:"data"`
	}
	if err := json.Unmarshal(record.Data, &envelope); err != nil || envelope.Type != "data-agh-permission" {
		return PermissionStreamPayload{}, false
	}
	return envelope.Data, true
}

// PermissionPayloads collects all streamed approval payloads from one SSE run.
func PermissionPayloads(records []SSEEvent) []PermissionStreamPayload {
	payloads := make([]PermissionStreamPayload, 0, len(records))
	for _, record := range records {
		if payload, ok := PermissionPayloadFromSSE(record); ok {
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

// RecordsContainTextDelta reports whether one streamed SSE result contains the
// expected assistant text delta.
func RecordsContainTextDelta(records []SSEEvent, want string) bool {
	for _, record := range records {
		if semanticSSEEvent(record) != transportParityEventAgentMessage || len(record.Data) == 0 {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			continue
		}
		if payload["type"] == "text-delta" && payload["delta"] == want {
			return true
		}
	}
	return false
}

func semanticSSEEvent(record SSEEvent) string {
	if trimmed := strings.TrimSpace(record.Event); trimmed != "" {
		return trimmed
	}
	return inferSSEEventName(record.Data)
}

// ValidateUDSApprovalResponse checks the UDS approval transport contract.
func ValidateUDSApprovalResponse(statusCode int, body []byte) error {
	trimmedBody := string(bytes.TrimSpace(body))
	if statusCode != http.StatusOK {
		return fmt.Errorf(
			"UDS approve status = %d, want %d; body=%s",
			statusCode,
			http.StatusOK,
			trimmedBody,
		)
	}
	var payload aghcontract.SessionApprovalResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode UDS approval response: %w", err)
	}
	if payload.Status != "approved" {
		return fmt.Errorf("UDS approve status payload = %q, want approved", payload.Status)
	}
	return nil
}
