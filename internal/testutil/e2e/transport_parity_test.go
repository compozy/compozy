package e2e

import (
	"net/http"
	"strings"
	"testing"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestRuntimeHarnessTransportClientsReuseSharedSurfaces(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{}
	udsClient := &http.Client{}
	harness := &RuntimeHarness{
		Config: compozyconfig.Config{
			Daemon: compozyconfig.DaemonConfig{Socket: "/tmp/compozy-transport.sock"},
		},
		HTTPBaseURL: "http://127.0.0.1:4317",
		HTTPClient:  httpClient,
		UDSBaseURL:  "http://unix",
		UDSClient:   udsClient,
		CLI:         &CLIClient{},
	}

	clients, err := harness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	if got, want := clients.HTTPBaseURL, harness.HTTPBaseURL; got != want {
		t.Fatalf("clients.HTTPBaseURL = %q, want %q", got, want)
	}
	if clients.HTTPClient != httpClient {
		t.Fatal("clients.HTTPClient did not reuse the shared runtime HTTP client")
	}
	if got, want := clients.UDSBaseURL, harness.UDSBaseURL; got != want {
		t.Fatalf("clients.UDSBaseURL = %q, want %q", got, want)
	}
	if clients.UDSClient != udsClient {
		t.Fatal("clients.UDSClient did not reuse the shared runtime UDS client")
	}
	if clients.CLI != harness.CLI {
		t.Fatal("clients.CLI did not reuse the shared runtime CLI helper")
	}
}

func TestRuntimeHarnessTransportClientsRejectBlankSocketPath(t *testing.T) {
	t.Parallel()

	harness := &RuntimeHarness{
		Config: compozyconfig.Config{},
	}

	_, err := harness.TransportClients()
	if err == nil {
		t.Fatal("TransportClients() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "daemon socket path is required") {
		t.Fatalf("TransportClients() error = %q, want daemon socket path failure", err)
	}
}

func TestRuntimeHarnessPromptSessionWithEventsInvokesCallback(t *testing.T) {
	t.Parallel()

	server := newHarnessTestServer(t)
	defer server.Close()

	harness := &RuntimeHarness{
		WorkspaceID: "ws-1",
		UDSBaseURL:  server.URL,
		UDSClient:   server.Client(),
	}

	seen := make([]SSEEvent, 0, 2)
	records, err := harness.PromptSessionWithEvents(
		testContext(t),
		"sess-1",
		"hello world",
		func(record SSEEvent) error {
			seen = append(seen, record)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("PromptSessionWithEvents() error = %v", err)
	}

	if got, want := len(records), 2; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if got, want := len(seen), len(records); got != want {
		t.Fatalf("len(seen) = %d, want %d", got, want)
	}
	if got, want := seen[0].Event, "agent_message"; got != want {
		t.Fatalf("seen[0].Event = %q, want %q", got, want)
	}
	if got, want := seen[1].Event, "done"; got != want {
		t.Fatalf("seen[1].Event = %q, want %q", got, want)
	}
}

func TestValidateUDSApprovalResponseChecksCanonicalPayload(t *testing.T) {
	t.Parallel()

	body := []byte(`{"outcome":"applied","request_id":"req-1","decision":"allow-once"}`)
	if err := ValidateUDSApprovalResponse(http.StatusOK, body); err != nil {
		t.Fatalf("ValidateUDSApprovalResponse() error = %v", err)
	}

	err := ValidateUDSApprovalResponse(http.StatusBadGateway, body)
	if err == nil {
		t.Fatal("ValidateUDSApprovalResponse() error = nil, want status failure")
	}
	if !strings.Contains(err.Error(), "want 200") {
		t.Fatalf("ValidateUDSApprovalResponse() error = %q, want status mismatch", err)
	}

	err = ValidateUDSApprovalResponse(http.StatusOK, []byte(`{"status":"pending"}`))
	if err == nil {
		t.Fatal("ValidateUDSApprovalResponse(pending) error = nil, want payload failure")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("ValidateUDSApprovalResponse(pending) error = %q, want payload mismatch", err)
	}
}

func TestValidateWebhookRunProjectionUsesNarrowProjection(t *testing.T) {
	t.Parallel()

	delivery := compozycontract.WebhookDeliveryPayload{
		Matched: 1,
		Runs: []compozycontract.RunPayload{{
			ID:        "run-1",
			TriggerID: "trg-1",
			SessionID: "sess-1",
			Status:    "completed",
			Attempt:   1,
			Error:     "daemon-truth detail ignored here",
		}},
	}

	httpProjection := compozycontract.RunPayload{
		ID:        "run-1",
		TriggerID: "trg-1",
		SessionID: "sess-1",
		Status:    "completed",
		Attempt:   77,
		Error:     "HTTP projection detail should not matter",
	}
	udsProjection := compozycontract.RunPayload{
		ID:        "run-1",
		TriggerID: "trg-1",
		SessionID: "sess-1",
		Status:    "completed",
		Attempt:   99,
		Error:     "UDS projection detail should not matter",
	}

	if err := ValidateWebhookRunProjection(delivery, httpProjection, udsProjection); err != nil {
		t.Fatalf("ValidateWebhookRunProjection() error = %v", err)
	}

	mismatch := compozycontract.RunPayload{
		ID:        "run-1",
		TriggerID: "trg-1",
		SessionID: "sess-2",
		Status:    "completed",
	}
	err := ValidateWebhookRunProjection(delivery, mismatch)
	if err == nil {
		t.Fatal("ValidateWebhookRunProjection() error = nil, want mismatch")
	}
	if !strings.Contains(err.Error(), "run projection 0") {
		t.Fatalf("ValidateWebhookRunProjection() error = %q, want projection mismatch", err)
	}
}

func TestPermissionPayloadHelpersAndTextDeltaDetection(t *testing.T) {
	t.Parallel()

	records := []SSEEvent{
		{
			Data: []byte(`{"type":"data-compozy-permission","data":{"request_id":"req-1"}}`),
		},
		{
			Event: "permission",
			Data:  []byte(`{"type":"data-compozy-permission","data":{"request_id":"req-1","decision":"allow-always"}}`),
		},
		{
			Data: []byte(`{"type":"text-delta","delta":"allow-always"}`),
		},
	}

	payloads := PermissionPayloads(records)
	if got, want := len(payloads), 2; got != want {
		t.Fatalf("len(payloads) = %d, want %d", got, want)
	}
	if got, want := payloads[0].RequestID, "req-1"; got != want {
		t.Fatalf("payloads[0].RequestID = %q, want %q", got, want)
	}
	if got, want := payloads[1].Decision, "allow-always"; got != want {
		t.Fatalf("payloads[1].Decision = %q, want %q", got, want)
	}
	if !RecordsContainTextDelta(records, "allow-always") {
		t.Fatal("RecordsContainTextDelta() = false, want true")
	}
}
