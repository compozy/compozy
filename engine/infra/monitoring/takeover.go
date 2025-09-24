package monitoring

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/interceptor"
)

const (
	TakeoverOutcomeStarted        = "started"
	TakeoverOutcomeTerminated     = "terminated"
	TakeoverOutcomeTerminateError = "terminate_error"
	TakeoverOutcomeError          = "error"
)

func RecordDispatcherTakeover(ctx context.Context, dispatcherID string, duration time.Duration, outcome string) {
	interceptor.RecordDispatcherTakeover(ctx, dispatcherID, duration, outcome)
}
