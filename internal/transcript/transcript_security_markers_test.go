package transcript

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

func TestTranscriptRedactsSecretsAcrossDisplaySurfaces(t *testing.T) {
	t.Parallel()

	runtimeSecret := "sk-transcript-display-secret-123456"
	unregisteredProviderSecret := "sk-ant-api03-abcdefghijklmnopqrstuv"
	cleanup := diagnostics.RegisterDynamicSecret(runtimeSecret)
	t.Cleanup(cleanup)

	t.Run("Should redact live event payloads before UI projection", func(t *testing.T) {
		t.Parallel()

		leaks := []string{
			runtimeSecret,
			unregisteredProviderSecret,
			"text-secret",
			"compozy_claim_live_123",
			"bearer-secret",
			"failure-secret",
			"runtime-secret",
			"raw-secret",
			"raw-private-camel",
			"tool-name-secret",
			"tool-input-secret",
			"tool-credential-secret",
			"attachment-name-secret",
			"compozy_claim_attachment_123",
		}
		event := (acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: "sess-redact",
			TurnID:    "turn-redact",
			Timestamp: time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC),
			Text:      "assistant stdout " + runtimeSecret + " token=text-secret compozy_claim_live_123",
			Error:     "Bearer bearer-secret",
			Failure: &store.SessionFailure{
				Kind:    store.FailurePrompt,
				Summary: "secret_binding=failure-secret",
			},
			Runtime: &acp.RuntimeActivity{
				LastActivityDetail: "token=runtime-secret",
			},
			Raw: json.RawMessage(
				`{"access_token":"raw-secret","privateKey":"raw-private-camel","note":"` + runtimeSecret + `","opaque":"` + unregisteredProviderSecret + `"}`,
			),
		}).WithTool(
			"token=tool-name-secret",
			json.RawMessage(
				`{"apiKey":"tool-input-secret","credential":"tool-credential-secret","command":"echo ok"}`,
			),
			false,
		).WithAttachments([]acp.EventAttachment{{
			ID:       "att-redact",
			Name:     "token=attachment-name-secret compozy_claim_attachment_123.png",
			MIMEType: "image/png",
		}})

		assertNoDisplayLeaks(t, RedactAgentEvent(event), leaks)
		assertNoDisplayLeaks(t, UIAgentEventPayloadFromEvent(event), leaks)
	})

	t.Run("Should redact stored tool output before transcript and chat replay", func(t *testing.T) {
		t.Parallel()

		leaks := []string{
			runtimeSecret,
			"stdout-secret",
			"stderr-secret",
			"content-secret",
			"raw-binding",
			"raw-secret",
			"input-secret",
			"compozy_claim_tool_123",
		}
		payload, err := MarshalAgentEvent(acp.AgentEvent{
			Type:      acp.EventTypeToolResult,
			SessionID: "sess-redact",
			TurnID:    "turn-redact",
			Timestamp: time.Date(2026, 5, 19, 10, 0, 1, 0, time.UTC),
			Title:     "Bash",
			Raw: json.RawMessage(`{
				"sessionUpdate":"tool_call_update",
				"status":"completed",
				"rawOutput":{
					"stdout":"runtime ` + runtimeSecret + ` token=stdout-secret compozy_claim_tool_123",
					"stderr":"Bearer stderr-secret",
					"content":"secret_binding=raw-binding",
					"api_key":"raw-secret"
				},
				"content":[{"type":"content","content":{"type":"text","text":"token=content-secret ` + runtimeSecret + `"}}],
				"_meta":{"claudeCode":{"toolName":"Bash"}},
				"rawInput":{"api_key":"input-secret","command":"echo ok"}
			}`),
		})
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		assertNoDisplayLeaks(t, payload, leaks)

		events := []store.SessionEvent{{
			ID:        "ev-redact-tool",
			SessionID: "sess-redact",
			TurnID:    "turn-redact",
			Sequence:  1,
			Type:      acp.EventTypeToolResult,
			Content:   payload,
			Timestamp: time.Date(2026, 5, 19, 10, 0, 1, 0, time.UTC),
		}}
		transcriptMessages, err := Assemble(events)
		if err != nil {
			t.Fatalf("Assemble() error = %v", err)
		}
		assertNoDisplayLeaks(t, transcriptMessages, leaks)

		uiMessages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		assertNoDisplayLeaks(t, uiMessages, leaks)
	})
}

func TestTranscriptRuntimeMarkers(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		turnID  string
		kind    string
		summary string
		want    string
	}{
		{
			name:    "Should render timeout marker from canonical marker event",
			turnID:  "turn-timeout",
			kind:    MarkerPromptTimeout,
			summary: "Runtime activity timed out (30 seconds idle).",
			want:    "Runtime activity timed out (30 seconds idle).",
		},
		{
			name:    "Should render unhealthy marker from canonical marker event",
			turnID:  "turn-unhealthy",
			kind:    MarkerSessionUnhealthy,
			summary: "Runtime health check failed; prompt may be stalled.",
			want:    "Runtime health check failed; prompt may be stalled.",
		},
		{
			name:    "Should render interrupted marker from canonical marker event",
			turnID:  "turn-interrupt",
			kind:    MarkerPromptInterrupted,
			summary: "operator interrupted the turn",
			want:    "operator interrupted the turn",
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			marker, err := NewMarker(
				test.kind,
				test.summary,
				timestamp.Add(time.Duration(index)*time.Second),
				map[string]any{"failure_count": 2},
			)
			if err != nil {
				t.Fatalf("NewMarker() error = %v", err)
			}
			event, err := marker.AgentEvent("sess-marker", test.turnID)
			if err != nil {
				t.Fatalf("AgentEvent() error = %v", err)
			}
			events := []store.SessionEvent{
				mustUIAgentSessionEvent(
					t,
					"ev-marker-"+test.turnID,
					int64(index+1),
					event.Timestamp,
					event,
				),
			}
			transcriptMessages, err := Assemble(events)
			if err != nil {
				t.Fatalf("Assemble() error = %v", err)
			}
			if got, want := len(transcriptMessages), 1; got != want {
				t.Fatalf("len(transcriptMessages) = %d, want %d; messages=%#v", got, want, transcriptMessages)
			}
			if got, want := transcriptMessages[0].Role, RoleSystem; got != want {
				t.Fatalf("transcript role = %q, want %q", got, want)
			}
			if got := transcriptMessages[0].Content; got != test.want {
				t.Fatalf("transcript content = %q, want %q", got, test.want)
			}

			uiMessages, err := ToUIMessages(events)
			if err != nil {
				t.Fatalf("ToUIMessages() error = %v", err)
			}
			if got, want := len(uiMessages), 1; got != want {
				t.Fatalf("len(uiMessages) = %d, want %d; messages=%#v", got, want, uiMessages)
			}
			if got, want := uiMessages[0].Role, UIRoleAssistant; got != want {
				t.Fatalf("UI role = %q, want %q", got, want)
			}
			if got, want := len(uiMessages[0].Parts), 1; got != want {
				t.Fatalf("UI marker parts = %d, want %d; parts=%#v", got, want, uiMessages[0].Parts)
			}
			markerPart := uiMessages[0].Parts[0]
			if got, want := markerPart.Type, uiPartDataEvent; got != want {
				t.Fatalf("UI marker part type = %q, want %q", got, want)
			}
			var payload struct {
				Type string `json:"type"`
				Raw  struct {
					Kind       string         `json:"kind"`
					OccurredAt string         `json:"occurred_at"`
					Summary    string         `json:"summary"`
					Evidence   map[string]any `json:"evidence"`
				} `json:"raw"`
			}
			if err := json.Unmarshal(markerPart.Data, &payload); err != nil {
				t.Fatalf("UI marker data payload = %q, unmarshal error = %v", markerPart.Data, err)
			}
			if got, want := payload.Type, event.Type; got != want {
				t.Fatalf("UI marker event type = %q, want %q", got, want)
			}
			if got, want := payload.Raw.Kind, test.kind; got != want {
				t.Fatalf("UI marker raw kind = %q, want %q", got, want)
			}
			if got, want := payload.Raw.Summary, test.want; got != want {
				t.Fatalf("UI marker raw summary = %q, want %q", got, want)
			}
			gotOccurred, err := time.Parse(time.RFC3339Nano, payload.Raw.OccurredAt)
			if err != nil {
				t.Fatalf("UI marker raw occurred_at = %q, parse error = %v", payload.Raw.OccurredAt, err)
			}
			if !gotOccurred.Equal(marker.OccurredAt) {
				t.Fatalf("UI marker raw occurred_at = %s, want %s", gotOccurred, marker.OccurredAt)
			}
			if got, want := payload.Raw.Evidence["failure_count"], float64(2); got != want {
				t.Fatalf(
					"UI marker raw evidence[failure_count] = %v, want %v; evidence=%#v",
					got,
					want,
					payload.Raw.Evidence,
				)
			}
		})
	}
}
