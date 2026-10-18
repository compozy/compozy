package automation

import (
	"context"

	"strings"

	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// persistenceContext gives cancellation-independent finalization a bounded lifetime.
func persistenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return nil, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func cloneRun(run *Run) *Run {
	if run == nil {
		return nil
	}

	cloned := *run
	if run.ScheduledAt != nil {
		cloned.ScheduledAt = new(*run.ScheduledAt)
	}
	if run.StartedAt != nil {
		cloned.StartedAt = new(*run.StartedAt)
	}
	if run.EndedAt != nil {
		cloned.EndedAt = new(*run.EndedAt)
	}
	if run.DeliveryErrorAt != nil {
		cloned.DeliveryErrorAt = new(*run.DeliveryErrorAt)
	}
	cloned.Metadata = cloneJSONMap(run.Metadata)
	return &cloned
}

func hookSchedulePayload(schedule *ScheduleSpec) *hookspkg.AutomationSchedulePayload {
	if schedule == nil {
		return nil
	}
	return &hookspkg.AutomationSchedulePayload{
		Mode:     string(schedule.Mode),
		Expr:     strings.TrimSpace(schedule.Expr),
		Interval: strings.TrimSpace(schedule.Interval),
		Time:     strings.TrimSpace(schedule.Time),
	}
}

func runDurationMilliseconds(run Run) int64 {
	if run.StartedAt == nil || run.EndedAt == nil {
		return 0
	}
	return run.EndedAt.UTC().Sub(run.StartedAt.UTC()).Milliseconds()
}

func cloneJSONMap(source map[string]any) map[string]any {
	return cloneAnyMap(source)
}

func nestedPath(path string, field string) string {
	trimmedPath := strings.TrimSpace(path)
	trimmedField := strings.TrimSpace(field)
	switch {
	case trimmedPath == "":
		return trimmedField
	case trimmedField == "":
		return trimmedPath
	default:
		return trimmedPath + "." + trimmedField
	}
}
