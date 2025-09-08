package webhook

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric/noop"
)

func TestMetrics_Init(t *testing.T) {
	t.Run("Should init without panic", func(_ *testing.T) {
		m := NewMetrics(context.Background(), noop.NewMeterProvider().Meter("test"))
		m.OnReceived(context.Background(), "slug", "wf")
		m.ObserveOverall(context.Background(), "slug", "wf", time.Millisecond)
	})
}
