package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGatewayDeviceAuditSink(t *testing.T) {
	t.Parallel()

	// Invariant: a device removal is durably recorded before the record disappears, with
	// redacted identity fields and the warning gateway.device family.
	// Owning layer: daemon gateway device event sink.
	// Canonical suite: this gateway device audit sink suite.
	t.Run("Should durably record the removed device identity", func(t *testing.T) {
		t.Parallel()
		writer := &gatewayAuditWriter{}
		removedAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
		sink := gatewayDeviceAuditSink{
			writer: writer,
			now:    func() time.Time { return removedAt.Add(time.Minute) },
		}
		if err := sink.RecordDeviceRemoved(testutil.Context(t), gateway.DeviceMutationEvent{
			Device: gateway.DeviceSession{
				ID: "dev_1", Name: "Laptop", ActorKind: gateway.ActorKindOperatorDevice,
				PairingOrigin: "local",
			},
			ActorKind: string(gateway.ActorKindOperatorDevice), ActorID: "dev_actor",
			RemovedAt: removedAt,
		}); err != nil {
			t.Fatalf("RecordDeviceRemoved() error = %v", err)
		}
		if len(writer.summaries) != 1 {
			t.Fatalf("event count = %d, want 1", len(writer.summaries))
		}
		summary := writer.summaries[0]
		if summary.Type != eventspkg.GatewayDeviceRemoved ||
			summary.Outcome != string(eventspkg.OutcomeWarning) ||
			summary.ActorID != "dev_actor" ||
			summary.ActorKind != string(gateway.ActorKindOperatorDevice) ||
			!summary.Timestamp.Equal(removedAt.Add(time.Minute)) {
			t.Fatalf("event summary = %#v", summary)
		}
		var payload struct {
			DeviceID      string `json:"device_id"`
			Name          string `json:"name"`
			ActorKind     string `json:"actor_kind"`
			PairingOrigin string `json:"pairing_origin"`
			RemovedAt     string `json:"removed_at"`
		}
		if err := json.Unmarshal(summary.Content, &payload); err != nil {
			t.Fatalf("decode event payload error = %v; content = %s", err, summary.Content)
		}
		if payload.DeviceID != "dev_1" || payload.Name != "Laptop" ||
			payload.ActorKind != string(gateway.ActorKindOperatorDevice) ||
			payload.PairingOrigin != "local" ||
			payload.RemovedAt != removedAt.Format(time.RFC3339Nano) {
			t.Fatalf("event payload = %#v", payload)
		}
	})

	t.Run("Should redact the removed device identity", func(t *testing.T) {
		t.Parallel()
		writer := &gatewayAuditWriter{}
		sink := gatewayDeviceAuditSink{writer: writer}
		if err := sink.RecordDeviceRemoved(testutil.Context(t), gateway.DeviceMutationEvent{
			Device: gateway.DeviceSession{
				ID: "dev_1", Name: "Laptop api_key=sk-device-secret",
			},
			ActorID:   "dev_actor token=sk-actor-secret",
			RemovedAt: time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RecordDeviceRemoved() error = %v", err)
		}
		content := string(writer.summaries[0].Content)
		if strings.Contains(content, "sk-device-secret") || strings.Contains(content, "sk-actor-secret") ||
			strings.Contains(writer.summaries[0].ActorID, "sk-actor-secret") {
			t.Fatalf("removal event leaked a secret: %s / %q", content, writer.summaries[0].ActorID)
		}
	})

	t.Run("Should fail the audit write when the ledger is unavailable", func(t *testing.T) {
		t.Parallel()
		sink := gatewayDeviceAuditSink{}
		err := sink.RecordDeviceRemoved(testutil.Context(t), gateway.DeviceMutationEvent{})
		if err == nil || !strings.Contains(err.Error(), "audit store is required") {
			t.Fatalf("RecordDeviceRemoved() error = %v, want a missing store failure", err)
		}
	})

	t.Run("Should bound the audit write after caller cancellation", func(t *testing.T) {
		t.Parallel()
		sink := gatewayDeviceAuditSink{
			writer: &blockingGatewayAuditWriter{}, timeout: 10 * time.Millisecond,
		}
		ctx, cancel := context.WithCancel(context.WithoutCancel(testutil.Context(t)))
		cancel()
		err := sink.RecordDeviceRemoved(ctx, gateway.DeviceMutationEvent{
			Device: gateway.DeviceSession{ID: "dev_1"}, RemovedAt: time.Now(),
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RecordDeviceRemoved() error = %v, want deadline exceeded", err)
		}
	})
}

var _ store.EventSummaryStore = (*gatewayAuditWriter)(nil)
