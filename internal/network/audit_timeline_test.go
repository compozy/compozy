package network

import (
	"testing"
	"time"
)

func TestNetworkAuditContractTimelineExtensions(t *testing.T) {
	t.Parallel()

	t.Run("Should keep extension metadata with the persisted timeline body", func(t *testing.T) {
		t.Parallel()

		envelope := testSayAuditEnvelope(t)
		envelope.Ext = ExtensionMap{
			"coordination": mustRawJSON(t, map[string]any{"task_id": "task-1", "run_id": "run-1"}),
		}

		entry, ok, err := normalizeTimelineMessageEntry(
			"sess-audit",
			AuditDirectionReceived,
			envelope,
			time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("normalizeTimelineMessageEntry() error = %v", err)
		}
		if !ok {
			t.Fatal("normalizeTimelineMessageEntry() ok = false, want true")
		}
		if got, want := string(entry.ExtJSON), `{"coordination":{"run_id":"run-1","task_id":"task-1"}}`; got != want {
			t.Fatalf("entry.ExtJSON = %q, want %q", got, want)
		}
	})
}
