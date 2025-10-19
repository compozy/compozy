package webhook

import (
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric/noop"
)

func TestMetrics_Init(t *testing.T) {
	t.Run("Should init without panic", func(_ *testing.T) {
		m, err := NewMetrics(t.Context(), noop.NewMeterProvider().Meter("test"))
		if err != nil {
			t.Fatalf("NewMetrics failed: %v", err)
		}
		ctx := t.Context()
		m.OnReceived(ctx, "slug", "wf")
		m.ObserveOverall(ctx, "slug", "wf", time.Millisecond)
		m.RecordPayloadSize(ctx, "event.created", "slug", 512)
		m.ObserveEventOutcome(ctx, "event.created", time.Millisecond, "success")
		m.ObserveEventOutcome(ctx, "event.created", time.Millisecond, "error")
		m.IncrementQueueDepth(ctx)
		m.DecrementQueueDepth(ctx)
	})
}
