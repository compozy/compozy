package toolcontext

import "context"

type ctxKey string

const (
	plannerDisabledKey ctxKey = "toolcontext.planner_disabled"
)

func DisablePlannerTools(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, plannerDisabledKey, true)
}

func PlannerToolsDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, ok := ctx.Value(plannerDisabledKey).(bool)
	if !ok {
		return false
	}
	return disabled
}
