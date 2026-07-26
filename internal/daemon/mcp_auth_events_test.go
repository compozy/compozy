package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/store"
)

func TestDaemonMCPAuthNotifierWritesOnlyRedactedLifecycleOutcome(t *testing.T) {
	t.Parallel()

	t.Run("Should write only redacted lifecycle outcome fields", func(t *testing.T) {
		t.Parallel()

		writtenAt := time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC)
		writer := &recordingMCPAuthEventWriter{}
		notifier := &daemonMCPAuthNotifier{
			writer: writer,
			now:    func() time.Time { return writtenAt },
		}
		notifier.NotifyMCPAuth(context.Background(), mcpauth.LifecycleOutcome{
			Action: mcpauth.LifecycleExchange,
			Target: mcpauth.Target{
				Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear",
			},
			Outcome:    "failure",
			ErrorClass: "expired",
		})

		if len(writer.summaries) != 1 {
			t.Fatalf("written event count = %d, want 1", len(writer.summaries))
		}
		summary := writer.summaries[0]
		if summary.Type != eventspkg.MCPAuthExchange || summary.Outcome != string(eventspkg.OutcomeFailure) ||
			!summary.Timestamp.Equal(writtenAt) {
			t.Fatalf("event summary = %#v", summary)
		}
		var payload map[string]string
		if err := json.Unmarshal(summary.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event content) error = %v", err)
		}
		want := map[string]string{
			"server_name":  "linear",
			"scope":        "workspace",
			"workspace_id": "workspace-a",
			"outcome":      "failure",
			"error_class":  "expired",
		}
		if len(payload) != len(want) {
			t.Fatalf("event payload = %#v, want exactly %#v", payload, want)
		}
		for key, value := range want {
			if payload[key] != value {
				t.Fatalf("event payload[%q] = %q, want %q", key, payload[key], value)
			}
		}
		serialized := string(summary.Content) + summary.Summary
		for _, forbidden := range []string{
			"authorization_code",
			"code_verifier",
			"redirect_url",
			"access_token",
			"refresh_token",
			"client_secret",
			"state",
		} {
			if strings.Contains(serialized, forbidden) {
				t.Fatalf("event summary leaked forbidden field %q: %s", forbidden, serialized)
			}
		}
	})
}

type recordingMCPAuthEventWriter struct {
	summaries []store.EventSummary
}

func (w *recordingMCPAuthEventWriter) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	w.summaries = append(w.summaries, summary)
	return nil
}
