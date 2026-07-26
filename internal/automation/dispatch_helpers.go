package automation

import (
	"context"

	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func persistenceContext(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	if ctx.Err() == nil {
		return ctx
	}
	return context.WithoutCancel(ctx)
}

func timePointer(value time.Time) *time.Time {
	timestamp := value
	return &timestamp
}

func cloneRun(run *Run) *Run {
	if run == nil {
		return nil
	}

	cloned := *run
	if run.ScheduledAt != nil {
		scheduledAt := *run.ScheduledAt
		cloned.ScheduledAt = &scheduledAt
	}
	if run.StartedAt != nil {
		startedAt := *run.StartedAt
		cloned.StartedAt = &startedAt
	}
	if run.EndedAt != nil {
		endedAt := *run.EndedAt
		cloned.EndedAt = &endedAt
	}
	if run.DeliveryErrorAt != nil {
		deliveryErrorAt := *run.DeliveryErrorAt
		cloned.DeliveryErrorAt = &deliveryErrorAt
	}
	cloned.Metadata = cloneJSONMap(run.Metadata)
	cloned.NetworkParticipation = cloneParticipationRequest(run.NetworkParticipation)
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
